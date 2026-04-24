// Package routes owns route lifecycle behavior and data contracts.
package routes

import "time"

const (
	SharingPolicyEveryoneCanShare = "everyone_can_share"
	SharingPolicyJoinersViewOnly  = "joiners_can_view_only"

	RouteStatusActive = "active"
	RouteStatusClosed = "closed"

	MemberStatusTracking   = "tracking"
	MemberStatusSpectating = "spectating"
	MemberStatusStale      = "stale"
	MemberStatusOffline    = "offline"
	MemberStatusLeft       = "left"

	RoleOwner  = "owner"
	RoleMember = "member"
)

var validTransportModes = map[string]struct{}{
	"walking":  {},
	"bicycle":  {},
	"car":      {},
	"bus":      {},
	"train":    {},
	"boat":     {},
	"airplane": {},
}

var validSharingPolicies = map[string]struct{}{
	SharingPolicyEveryoneCanShare: {},
	SharingPolicyJoinersViewOnly:  {},
}

// Route stores public route data.
type Route struct {
	ID                 string     `json:"id"`
	Code               string     `json:"code"`
	Name               string     `json:"name"`
	Description        string     `json:"description"`
	HasPassword        bool       `json:"hasPassword"`
	SharingPolicy      string     `json:"sharingPolicy"`
	Status             string     `json:"status"`
	MaxTrackingMembers int        `json:"maxTrackingMembers"`
	CreatedAt          time.Time  `json:"createdAt"`
	ClosedAt           *time.Time `json:"closedAt"`
}

// Member stores route membership data.
type Member struct {
	ID            string     `json:"id"`
	RouteID       string     `json:"routeId"`
	ClientID      string     `json:"clientId"`
	DisplayName   string     `json:"displayName"`
	TransportMode string     `json:"transportMode"`
	IsOwner       bool       `json:"isOwner"`
	Status        string     `json:"status"`
	Color         string     `json:"color"`
	JoinedAt      time.Time  `json:"joinedAt"`
	LeftAt        *time.Time `json:"leftAt"`
}

// AuthorizedMember combines route and member data for token-authenticated requests.
type AuthorizedMember struct {
	Route  Route
	Member Member
}

// CreateRouteInput contains route creation request data.
type CreateRouteInput struct {
	ClientID      string
	DisplayName   string
	TransportMode string
	Name          string
	Description   string
	Password      string
	SharingPolicy string
}

// CreateRouteResult contains the create route response payload.
type CreateRouteResult struct {
	Route       Route  `json:"route"`
	Owner       Member `json:"owner"`
	MemberToken string `json:"memberToken"`
	OwnerToken  string `json:"ownerToken"`
}

// UpdateRouteInput contains mutable route fields.
type UpdateRouteInput struct {
	Name        string
	Description string
	Status      string
}

// AccessRouteResult contains non-sensitive route metadata for a join screen.
type AccessRouteResult struct {
	Code             string `json:"code"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Status           string `json:"status"`
	RequiresPassword bool   `json:"requiresPassword"`
	SharingPolicy    string `json:"sharingPolicy"`
}

// JoinRouteInput contains route join request data.
type JoinRouteInput struct {
	ClientID      string
	DisplayName   string
	TransportMode string
	Password      string
}

// JoinRouteResult contains the join route response payload.
type JoinRouteResult struct {
	Route       Route  `json:"route"`
	Member      Member `json:"member"`
	MemberToken string `json:"memberToken"`
}

// LeaveRouteResult contains the member state after leaving a route.
type LeaveRouteResult struct {
	Member Member `json:"member"`
}

// Snapshot contains the full route page bootstrap payload.
type Snapshot struct {
	Route   Route              `json:"route"`
	Members []SnapshotMember   `json:"members"`
	Viewer  ViewerCapabilities `json:"viewer"`
}

// SnapshotMember contains a member and their empty path history placeholder.
type SnapshotMember struct {
	ID            string        `json:"id"`
	DisplayName   string        `json:"displayName"`
	TransportMode string        `json:"transportMode"`
	Role          string        `json:"role"`
	Status        string        `json:"status"`
	Color         string        `json:"color"`
	JoinedAt      time.Time     `json:"joinedAt"`
	LeftAt        *time.Time    `json:"leftAt"`
	Paths         []PathSegment `json:"paths"`
}

// PathSegment is the historical path representation in snapshot responses.
type PathSegment struct {
	ID        string       `json:"id,omitempty"`
	StartedAt *time.Time   `json:"startedAt,omitempty"`
	EndedAt   *time.Time   `json:"endedAt,omitempty"`
	Points    []RoutePoint `json:"points"`
}

// RoutePoint is a historical route point representation.
type RoutePoint struct {
	Latitude         float64    `json:"latitude"`
	Longitude        float64    `json:"longitude"`
	AccuracyM        *float64   `json:"accuracyM,omitempty"`
	ClientRecordedAt *time.Time `json:"clientRecordedAt,omitempty"`
	RecordedAt       time.Time  `json:"recordedAt"`
}

// ViewerCapabilities contains caller-specific permission booleans.
type ViewerCapabilities struct {
	MemberID        string `json:"memberId"`
	Role            string `json:"role"`
	Status          string `json:"status"`
	CanStartSharing bool   `json:"canStartSharing"`
	CanStopSharing  bool   `json:"canStopSharing"`
	CanLeaveRoute   bool   `json:"canLeaveRoute"`
	CanCloseRoute   bool   `json:"canCloseRoute"`
	CanDeleteRoute  bool   `json:"canDeleteRoute"`
	CanEditRoute    bool   `json:"canEditRoute"`
}
