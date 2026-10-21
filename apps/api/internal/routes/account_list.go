package routes

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"keepup/apps/api/internal/auth"
)

// ListRoutesInput uses mutually exclusive relationships with additive filters.
type ListRoutesInput struct {
	View                string
	Owned, Joined, Left bool
	Limit               int
	Cursor              string
}

// RouteSummary contains only fields safe for current and former members alike.
type RouteSummary struct {
	ID           string     `json:"id"`
	Code         string     `json:"code"`
	Name         string     `json:"name"`
	Status       string     `json:"status"`
	Relationship string     `json:"relationship"`
	HasPassword  bool       `json:"hasPassword"`
	CreatedAt    time.Time  `json:"createdAt"`
	ClosedAt     *time.Time `json:"closedAt"`
	Action       string     `json:"action"`
}

// ListRoutesResult supplies counts before pagination, across both lifecycle views.
type ListRoutesResult struct {
	Items      []RouteSummary `json:"items"`
	Counts     RouteCounts    `json:"counts"`
	NextCursor *string        `json:"nextCursor"`
}

// RouteCounts applies selected relationship filters but not the cursor or current view.
type RouteCounts struct {
	Active   int `json:"active"`
	Archived int `json:"archived"`
}

type routeCursor struct {
	UserID              string `json:"u"`
	View                string `json:"v"`
	Owned, Joined, Left bool
	At                  time.Time `json:"t"`
	ID                  string    `json:"i"`
}

func (s *AccountService) encodeCursor(cursor routeCursor) (string, error) {
	body, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, s.cursorKey)
	_, _ = mac.Write(body)
	return base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *AccountService) decodeCursor(raw, userID string, input ListRoutesInput) (routeCursor, error) {
	var c routeCursor
	parts := strings.Split(raw, ".")
	if len(parts) != 2 || len(raw) > 2048 {
		return c, ErrInvalidInput
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return c, ErrInvalidInput
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return c, ErrInvalidInput
	}
	mac := hmac.New(sha256.New, s.cursorKey)
	_, _ = mac.Write(body)
	if !hmac.Equal(signature, mac.Sum(nil)) || json.Unmarshal(body, &c) != nil || c.UserID != userID || c.View != input.View || c.Owned != input.Owned || c.Joined != input.Joined || c.Left != input.Left || c.ID == "" || c.At.IsZero() {
		return routeCursor{}, ErrInvalidInput
	}
	return c, nil
}

const accountRouteCategories = `WITH related AS (
    SELECT r.id,r.code,r.name,r.status,r.password_hash IS NOT NULL AS has_password,r.created_at,r.closed_at,
    CASE WHEN r.owner_user_id=$1 THEN 'owner'
         WHEN EXISTS(SELECT 1 FROM route_members m WHERE m.route_id=r.id AND m.user_id=$1 AND m.left_at IS NULL AND m.status<>'left') THEN 'joined'
         ELSE 'left' END AS relationship,
    EXISTS(SELECT 1 FROM route_members m WHERE m.route_id=r.id AND m.user_id=$1 AND m.left_at IS NULL AND m.status<>'left') AS current_member
    FROM routes r WHERE r.owner_user_id IS NOT NULL AND
        (r.owner_user_id=$1 OR EXISTS(SELECT 1 FROM route_members m WHERE m.route_id=r.id AND m.user_id=$1))
), selected AS (SELECT * FROM related WHERE
    (relationship='owner' AND $2) OR (relationship='joined' AND $3) OR (relationship='left' AND $4)) `

// ListRoutes uses keyset pagination and never accepts a cursor as route authority.
func (s *AccountService) ListRoutes(ctx context.Context, session auth.Session, input ListRoutesInput) (ListRoutesResult, error) {
	if (input.View != "active" && input.View != "archived") || input.Limit < 1 || input.Limit > 100 {
		return ListRoutesResult{}, ErrInvalidInput
	}
	var cursor routeCursor
	var err error
	if input.Cursor != "" {
		cursor, err = s.decodeCursor(input.Cursor, session.User.ID, input)
		if err != nil {
			return ListRoutesResult{}, err
		}
	}
	return accountOperation(ctx, s, session, "", false, false, func(tx pgx.Tx, _ *Service, _ accountRoute, user auth.User) (ListRoutesResult, error) {
		result := ListRoutesResult{Items: []RouteSummary{}}
		err := tx.QueryRow(ctx, accountRouteCategories+`SELECT count(*) FILTER(WHERE status='active'),count(*) FILTER(WHERE status='closed') FROM selected`, user.ID, input.Owned, input.Joined, input.Left).Scan(&result.Counts.Active, &result.Counts.Archived)
		if err != nil {
			return result, err
		}
		status, order := "active", "created_at"
		if input.View == "archived" {
			status, order = "closed", "closed_at"
		}
		var afterTime *time.Time
		var afterID *string
		if input.Cursor != "" {
			afterTime, afterID = &cursor.At, &cursor.ID
		}
		rows, err := tx.Query(ctx, accountRouteCategories+`SELECT id,code,name,status,relationship,has_password,created_at,closed_at,current_member FROM selected
            WHERE status=$5 AND ($6::timestamptz IS NULL OR (`+order+`,id)<($6,$7::uuid))
            ORDER BY `+order+` DESC,id DESC LIMIT $8`, user.ID, input.Owned, input.Joined, input.Left, status, afterTime, afterID, input.Limit+1)
		if err != nil {
			return result, err
		}
		defer rows.Close()
		for rows.Next() {
			var item RouteSummary
			var current bool
			if err = rows.Scan(&item.ID, &item.Code, &item.Name, &item.Status, &item.Relationship, &item.HasPassword, &item.CreatedAt, &item.ClosedAt, &current); err != nil {
				return result, err
			}
			item.Action = "rejoin"
			if current {
				item.Action = "open"
				if item.Status == RouteStatusClosed {
					item.Action = "view_archive"
				}
			}
			result.Items = append(result.Items, item)
		}
		if err = rows.Err(); err != nil {
			return result, err
		}
		if len(result.Items) > input.Limit {
			result.Items = result.Items[:input.Limit]
			last := result.Items[len(result.Items)-1]
			at := last.CreatedAt
			if input.View == "archived" {
				at = *last.ClosedAt
			}
			encoded, encodeErr := s.encodeCursor(routeCursor{UserID: user.ID, View: input.View, Owned: input.Owned, Joined: input.Joined, Left: input.Left, At: at, ID: last.ID})
			if encodeErr != nil {
				return result, encodeErr
			}
			result.NextCursor = &encoded
		}
		return result, nil
	})
}
