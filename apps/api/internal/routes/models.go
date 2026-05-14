// Package routes owns route lifecycle behavior and data contracts.
package routes

import (
	"encoding/json"
	"time"
)

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

	PathSegmentEndReasonStopped      = "stopped"
	PathSegmentEndReasonDisconnected = "disconnected"
	PathSegmentEndReasonLeft         = "left"
	PathSegmentEndReasonRouteClosed  = "route_closed"
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

// StartSharingResult contains the member state and opened path segment.
type StartSharingResult struct {
	Member  Member      `json:"member"`
	Segment PathSegment `json:"segment"`
}

// StopSharingResult contains the member state after stopping tracking.
type StopSharingResult struct {
	Member Member `json:"member"`
}

// PositionUpdateInput contains one client-recorded location sample.
type PositionUpdateInput struct {
	Latitude         float64
	Longitude        float64
	AccuracyM        *float64
	AltitudeM        *float64
	SpeedMPS         *float64
	HeadingDeg       *float64
	ClientRecordedAt *time.Time
	RawPayload       json.RawMessage
}

// PositionUpdateResult contains the accepted and persisted route point.
type PositionUpdateResult struct {
	RouteID         string     `json:"-"`
	MemberID        string     `json:"memberId"`
	SegmentID       string     `json:"segmentId"`
	Point           RoutePoint `json:"point"`
	RecoveredMember *Member    `json:"member,omitempty"`
}

// Snapshot contains the full route page bootstrap payload.
type Snapshot struct {
	Route   Route              `json:"route"`
	Members []SnapshotMember   `json:"members"`
	Viewer  ViewerCapabilities `json:"viewer"`
}

// SnapshotMember contains a member and their persisted path history.
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
	Seq              int64      `json:"seq,omitempty"`
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
