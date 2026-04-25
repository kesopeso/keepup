package routes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository stores routes in PostgreSQL.
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository builds a PostgreSQL-backed route repository.
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// CreateRoute creates a route, owner member, and route-scoped tokens in one transaction.
func (r *PostgresRepository) CreateRoute(ctx context.Context, params CreateRouteRepoParams) (CreateRouteRepoResult, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CreateRouteRepoResult{}, fmt.Errorf("begin create route tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var route Route
	if err := tx.QueryRow(ctx, `
		INSERT INTO routes (
			code,
			name,
			description,
			password_hash,
			sharing_policy,
			status,
			max_tracking_members
		) VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7)
		RETURNING id, code, name, COALESCE(description, ''), password_hash IS NOT NULL, sharing_policy, status, max_tracking_members, created_at, closed_at
	`, params.Route.Code, params.Route.Name, params.Route.Description, params.Route.PasswordHash, params.Route.SharingPolicy, params.Route.Status, params.Route.MaxTrackingMembers).
		Scan(
			&route.ID,
			&route.Code,
			&route.Name,
			&route.Description,
			&route.HasPassword,
			&route.SharingPolicy,
			&route.Status,
			&route.MaxTrackingMembers,
			&route.CreatedAt,
			&route.ClosedAt,
		); err != nil {
		return CreateRouteRepoResult{}, mapDatabaseError(fmt.Errorf("insert route: %w", err))
	}

	var owner Member
	if err := tx.QueryRow(ctx, `
		INSERT INTO route_members (
			route_id,
			client_id,
			display_name,
			transport_mode,
			is_owner,
			status,
			color
		) VALUES ($1, $2, $3, $4, TRUE, $5, $6)
		RETURNING id, route_id, client_id, display_name, transport_mode, is_owner, status, color, joined_at, left_at
	`, route.ID, params.Owner.ClientID, params.Owner.DisplayName, params.Owner.TransportMode, params.Owner.Status, params.Owner.Color).
		Scan(
			&owner.ID,
			&owner.RouteID,
			&owner.ClientID,
			&owner.DisplayName,
			&owner.TransportMode,
			&owner.IsOwner,
			&owner.Status,
			&owner.Color,
			&owner.JoinedAt,
			&owner.LeftAt,
		); err != nil {
		return CreateRouteRepoResult{}, mapDatabaseError(fmt.Errorf("insert owner member: %w", err))
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO member_tokens (member_id, token_hash)
		VALUES ($1, $2)
	`, owner.ID, params.MemberTokenHash); err != nil {
		return CreateRouteRepoResult{}, mapDatabaseError(fmt.Errorf("insert member token: %w", err))
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO owner_tokens (route_id, member_id, token_hash)
		VALUES ($1, $2, $3)
	`, route.ID, owner.ID, params.OwnerTokenHash); err != nil {
		return CreateRouteRepoResult{}, mapDatabaseError(fmt.Errorf("insert owner token: %w", err))
	}

	if err := tx.Commit(ctx); err != nil {
		return CreateRouteRepoResult{}, fmt.Errorf("commit create route tx: %w", err)
	}

	return CreateRouteRepoResult{
		Route: route,
		Owner: owner,
	}, nil
}

// CreateMember creates a new member and member token in one transaction.
func (r *PostgresRepository) CreateMember(ctx context.Context, params CreateMemberRepoParams) (CreateMemberRepoResult, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CreateMemberRepoResult{}, fmt.Errorf("begin create member tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var member Member
	if err := tx.QueryRow(ctx, `
		INSERT INTO route_members (
			route_id,
			client_id,
			display_name,
			transport_mode,
			is_owner,
			status,
			color
		) VALUES ($1, $2, $3, $4, FALSE, $5, $6)
		RETURNING id, route_id, client_id, display_name, transport_mode, is_owner, status, color, joined_at, left_at
	`, params.RouteID, params.ClientID, params.DisplayName, params.TransportMode, params.Status, params.Color).
		Scan(
			&member.ID,
			&member.RouteID,
			&member.ClientID,
			&member.DisplayName,
			&member.TransportMode,
			&member.IsOwner,
			&member.Status,
			&member.Color,
			&member.JoinedAt,
			&member.LeftAt,
		); err != nil {
		return CreateMemberRepoResult{}, mapDatabaseError(fmt.Errorf("insert route member: %w", err))
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO member_tokens (member_id, token_hash)
		VALUES ($1, $2)
	`, member.ID, params.MemberTokenHash); err != nil {
		return CreateMemberRepoResult{}, mapDatabaseError(fmt.Errorf("insert route member token: %w", err))
	}

	if err := tx.Commit(ctx); err != nil {
		return CreateMemberRepoResult{}, fmt.Errorf("commit create member tx: %w", err)
	}

	return CreateMemberRepoResult{Member: member}, nil
}

// GetRouteByCode loads route access metadata plus password hash.
func (r *PostgresRepository) GetRouteByCode(ctx context.Context, code string) (Route, string, error) {
	var route Route
	var passwordHash string

	err := r.db.QueryRow(ctx, `
		SELECT id, code, name, COALESCE(description, ''), COALESCE(password_hash, ''), password_hash IS NOT NULL, sharing_policy, status, max_tracking_members, created_at, closed_at
		FROM routes
		WHERE code = $1
	`, code).Scan(
		&route.ID,
		&route.Code,
		&route.Name,
		&route.Description,
		&passwordHash,
		&route.HasPassword,
		&route.SharingPolicy,
		&route.Status,
		&route.MaxTrackingMembers,
		&route.CreatedAt,
		&route.ClosedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Route{}, "", ErrRouteNotFound
		}

		return Route{}, "", fmt.Errorf("get route by code: %w", err)
	}

	return route, passwordHash, nil
}

// GetAuthorizedMemberByTokenHash resolves a member token into route/member data.
func (r *PostgresRepository) GetAuthorizedMemberByTokenHash(ctx context.Context, tokenHash string) (AuthorizedMember, error) {
	var result AuthorizedMember
	err := r.db.QueryRow(ctx, `
		SELECT
			r.id,
			r.code,
			r.name,
			COALESCE(r.description, ''),
			r.password_hash IS NOT NULL,
			r.sharing_policy,
			r.status,
			r.max_tracking_members,
			r.created_at,
			r.closed_at,
			m.id,
			m.route_id,
			m.client_id,
			m.display_name,
			m.transport_mode,
			m.is_owner,
			m.status,
			m.color,
			m.joined_at,
			m.left_at
		FROM member_tokens mt
		INNER JOIN route_members m ON m.id = mt.member_id
		INNER JOIN routes r ON r.id = m.route_id
		WHERE mt.token_hash = $1 AND mt.revoked_at IS NULL
	`, tokenHash).Scan(
		&result.Route.ID,
		&result.Route.Code,
		&result.Route.Name,
		&result.Route.Description,
		&result.Route.HasPassword,
		&result.Route.SharingPolicy,
		&result.Route.Status,
		&result.Route.MaxTrackingMembers,
		&result.Route.CreatedAt,
		&result.Route.ClosedAt,
		&result.Member.ID,
		&result.Member.RouteID,
		&result.Member.ClientID,
		&result.Member.DisplayName,
		&result.Member.TransportMode,
		&result.Member.IsOwner,
		&result.Member.Status,
		&result.Member.Color,
		&result.Member.JoinedAt,
		&result.Member.LeftAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AuthorizedMember{}, ErrUnauthorized
		}

		return AuthorizedMember{}, fmt.Errorf("get authorized member: %w", err)
	}

	return result, nil
}

// GetAuthorizedOwnerByTokenHash resolves an owner token into route/member data.
func (r *PostgresRepository) GetAuthorizedOwnerByTokenHash(ctx context.Context, tokenHash string) (AuthorizedMember, error) {
	var result AuthorizedMember
	err := r.db.QueryRow(ctx, `
		SELECT
			r.id,
			r.code,
			r.name,
			COALESCE(r.description, ''),
			r.password_hash IS NOT NULL,
			r.sharing_policy,
			r.status,
			r.max_tracking_members,
			r.created_at,
			r.closed_at,
			m.id,
			m.route_id,
			m.client_id,
			m.display_name,
			m.transport_mode,
			m.is_owner,
			m.status,
			m.color,
			m.joined_at,
			m.left_at
		FROM owner_tokens ot
		INNER JOIN route_members m ON m.id = ot.member_id
		INNER JOIN routes r ON r.id = ot.route_id
		WHERE ot.token_hash = $1 AND ot.revoked_at IS NULL
	`, tokenHash).Scan(
		&result.Route.ID,
		&result.Route.Code,
		&result.Route.Name,
		&result.Route.Description,
		&result.Route.HasPassword,
		&result.Route.SharingPolicy,
		&result.Route.Status,
		&result.Route.MaxTrackingMembers,
		&result.Route.CreatedAt,
		&result.Route.ClosedAt,
		&result.Member.ID,
		&result.Member.RouteID,
		&result.Member.ClientID,
		&result.Member.DisplayName,
		&result.Member.TransportMode,
		&result.Member.IsOwner,
		&result.Member.Status,
		&result.Member.Color,
		&result.Member.JoinedAt,
		&result.Member.LeftAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AuthorizedMember{}, ErrUnauthorized
		}

		return AuthorizedMember{}, fmt.Errorf("get authorized owner: %w", err)
	}

	return result, nil
}

// GetMembersByRouteID loads the route member list.
func (r *PostgresRepository) GetMembersByRouteID(ctx context.Context, routeID string) ([]Member, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, route_id, client_id, display_name, transport_mode, is_owner, status, color, joined_at, left_at
		FROM route_members
		WHERE route_id = $1
		ORDER BY joined_at ASC
	`, routeID)
	if err != nil {
		return nil, fmt.Errorf("query route members: %w", err)
	}
	defer rows.Close()

	members := make([]Member, 0)
	for rows.Next() {
		var member Member
		if err := rows.Scan(
			&member.ID,
			&member.RouteID,
			&member.ClientID,
			&member.DisplayName,
			&member.TransportMode,
			&member.IsOwner,
			&member.Status,
			&member.Color,
			&member.JoinedAt,
			&member.LeftAt,
		); err != nil {
			return nil, fmt.Errorf("scan route member: %w", err)
		}

		members = append(members, member)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate route members: %w", err)
	}

	return members, nil
}

// CountTrackingMembers returns the number of active tracking members for a route.
func (r *PostgresRepository) CountMembersByRouteID(ctx context.Context, routeID string) (int, error) {
	var count int
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM route_members
		WHERE route_id = $1
	`, routeID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count route members: %w", err)
	}

	return count, nil
}

// CountTrackingMembers returns the number of active tracking members for a route.
func (r *PostgresRepository) CountTrackingMembers(ctx context.Context, routeID string) (int, error) {
	var count int
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM route_members
		WHERE route_id = $1 AND status = $2
	`, routeID, MemberStatusTracking).Scan(&count); err != nil {
		return 0, fmt.Errorf("count tracking members: %w", err)
	}

	return count, nil
}

// StartTrackingMember marks a member as tracking and opens a path segment.
func (r *PostgresRepository) StartTrackingMember(ctx context.Context, routeID, memberID string) (StartSharingRepoResult, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return StartSharingRepoResult{}, fmt.Errorf("begin start tracking tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var member Member
	if err := tx.QueryRow(ctx, `
		UPDATE route_members
		SET status = $3
		WHERE id = $1 AND route_id = $2
		RETURNING id, route_id, client_id, display_name, transport_mode, is_owner, status, color, joined_at, left_at
	`, memberID, routeID, MemberStatusTracking).Scan(
		&member.ID,
		&member.RouteID,
		&member.ClientID,
		&member.DisplayName,
		&member.TransportMode,
		&member.IsOwner,
		&member.Status,
		&member.Color,
		&member.JoinedAt,
		&member.LeftAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return StartSharingRepoResult{}, ErrUnauthorized
		}

		return StartSharingRepoResult{}, fmt.Errorf("update tracking member row: %w", err)
	}

	var segment PathSegment
	var startedAt time.Time
	if err := tx.QueryRow(ctx, `
		INSERT INTO path_segments (route_id, member_id)
		VALUES ($1, $2)
		RETURNING id, started_at, ended_at
	`, routeID, memberID).Scan(
		&segment.ID,
		&startedAt,
		&segment.EndedAt,
	); err != nil {
		return StartSharingRepoResult{}, fmt.Errorf("insert path segment: %w", err)
	}
	segment.StartedAt = &startedAt
	segment.Points = []RoutePoint{}

	if err := tx.Commit(ctx); err != nil {
		return StartSharingRepoResult{}, fmt.Errorf("commit start tracking tx: %w", err)
	}

	return StartSharingRepoResult{
		Member:  member,
		Segment: segment,
	}, nil
}

// StopTrackingMember marks a member as spectating and ends open path segments.
func (r *PostgresRepository) StopTrackingMember(ctx context.Context, routeID, memberID string) (Member, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Member{}, fmt.Errorf("begin stop tracking tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var member Member
	if err := tx.QueryRow(ctx, `
		UPDATE route_members
		SET status = $3
		WHERE id = $1 AND route_id = $2
		RETURNING id, route_id, client_id, display_name, transport_mode, is_owner, status, color, joined_at, left_at
	`, memberID, routeID, MemberStatusSpectating).Scan(
		&member.ID,
		&member.RouteID,
		&member.ClientID,
		&member.DisplayName,
		&member.TransportMode,
		&member.IsOwner,
		&member.Status,
		&member.Color,
		&member.JoinedAt,
		&member.LeftAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Member{}, ErrUnauthorized
		}

		return Member{}, fmt.Errorf("update spectating member row: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE path_segments
		SET ended_at = COALESCE(ended_at, NOW()), end_reason = COALESCE(end_reason, $3)
		WHERE route_id = $1 AND member_id = $2 AND ended_at IS NULL
	`, routeID, memberID, PathSegmentEndReasonStopped); err != nil {
		return Member{}, fmt.Errorf("end path segment: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Member{}, fmt.Errorf("commit stop tracking tx: %w", err)
	}

	return member, nil
}

// RecordPosition appends a point to the member's current open path segment.
func (r *PostgresRepository) RecordPosition(ctx context.Context, params RecordPositionRepoParams) (PositionUpdateResult, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PositionUpdateResult{}, fmt.Errorf("begin record position tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var segmentID string
	if err := tx.QueryRow(ctx, `
		SELECT id
		FROM path_segments
		WHERE route_id = $1 AND member_id = $2 AND ended_at IS NULL
		ORDER BY started_at DESC
		LIMIT 1
		FOR UPDATE
	`, params.RouteID, params.MemberID).Scan(&segmentID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PositionUpdateResult{}, ErrInvalidInput
		}

		return PositionUpdateResult{}, fmt.Errorf("load open path segment: %w", err)
	}

	var point RoutePoint
	var accuracy sql.NullFloat64
	var clientRecordedAt sql.NullTime
	if err := tx.QueryRow(ctx, `
		WITH next_seq AS (
			SELECT COALESCE(MAX(seq), 0) + 1 AS seq
			FROM position_points
			WHERE segment_id = $3
		)
		INSERT INTO position_points (
			route_id,
			member_id,
			segment_id,
			seq,
			client_recorded_at,
			location,
			latitude,
			longitude,
			accuracy_m,
			altitude_m,
			speed_mps,
			heading_deg,
			raw_payload
		)
		VALUES (
			$1,
			$2,
			$3,
			(SELECT seq FROM next_seq),
			$10,
			ST_SetSRID(ST_MakePoint($5, $4), 4326)::geography,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9,
			$11
		)
		RETURNING seq, latitude, longitude, accuracy_m, client_recorded_at, recorded_at
	`, params.RouteID, params.MemberID, segmentID, params.Latitude, params.Longitude, params.AccuracyM, params.AltitudeM, params.SpeedMPS, params.HeadingDeg, params.ClientRecordedAt, params.RawPayload).Scan(
		&point.Seq,
		&point.Latitude,
		&point.Longitude,
		&accuracy,
		&clientRecordedAt,
		&point.RecordedAt,
	); err != nil {
		return PositionUpdateResult{}, fmt.Errorf("insert position point: %w", err)
	}

	if accuracy.Valid {
		point.AccuracyM = &accuracy.Float64
	}
	if clientRecordedAt.Valid {
		point.ClientRecordedAt = &clientRecordedAt.Time
	}

	if err := tx.Commit(ctx); err != nil {
		return PositionUpdateResult{}, fmt.Errorf("commit record position tx: %w", err)
	}

	return PositionUpdateResult{
		RouteID:   params.RouteID,
		MemberID:  params.MemberID,
		SegmentID: segmentID,
		Point:     point,
	}, nil
}

// UpdateRoute mutates route name, description, and/or status.
func (r *PostgresRepository) UpdateRoute(ctx context.Context, routeID string, params UpdateRouteRepoParams) (Route, error) {
	var route Route

	err := r.db.QueryRow(ctx, `
		UPDATE routes
		SET
			name = COALESCE($2, name),
			description = COALESCE($3, description),
			status = COALESCE($4, status),
			closed_at = CASE
				WHEN $4 = 'closed' AND closed_at IS NULL THEN NOW()
				ELSE closed_at
			END,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, code, name, COALESCE(description, ''), password_hash IS NOT NULL, sharing_policy, status, max_tracking_members, created_at, closed_at
	`, routeID, params.Name, params.Description, params.Status).Scan(
		&route.ID,
		&route.Code,
		&route.Name,
		&route.Description,
		&route.HasPassword,
		&route.SharingPolicy,
		&route.Status,
		&route.MaxTrackingMembers,
		&route.CreatedAt,
		&route.ClosedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Route{}, ErrRouteNotFound
		}

		return Route{}, fmt.Errorf("update route row: %w", err)
	}

	return route, nil
}

// LeaveMember marks a route member as left.
func (r *PostgresRepository) LeaveMember(ctx context.Context, memberID string) (Member, error) {
	var member Member

	err := r.db.QueryRow(ctx, `
		UPDATE route_members
		SET status = $2, left_at = COALESCE(left_at, NOW())
		WHERE id = $1
		RETURNING id, route_id, client_id, display_name, transport_mode, is_owner, status, color, joined_at, left_at
	`, memberID, MemberStatusLeft).Scan(
		&member.ID,
		&member.RouteID,
		&member.ClientID,
		&member.DisplayName,
		&member.TransportMode,
		&member.IsOwner,
		&member.Status,
		&member.Color,
		&member.JoinedAt,
		&member.LeftAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Member{}, ErrUnauthorized
		}

		return Member{}, fmt.Errorf("leave member row: %w", err)
	}

	return member, nil
}

// DeleteRoute removes a route and all dependent records.
func (r *PostgresRepository) DeleteRoute(ctx context.Context, routeID string) error {
	commandTag, err := r.db.Exec(ctx, `
		DELETE FROM routes
		WHERE id = $1
	`, routeID)
	if err != nil {
		return fmt.Errorf("delete route row: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrRouteNotFound
	}

	return nil
}

func mapDatabaseError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "route_members_route_alias_unique_idx") {
			return ErrAliasTaken
		}
	}

	return err
}
