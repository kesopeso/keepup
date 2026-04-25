package routes

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	defaultCodeLength = 6
	maxCodeAttempts   = 5
)

var (
	// ErrRouteNotFound is returned when a route does not exist.
	ErrRouteNotFound = errors.New("route not found")
	// ErrAliasTaken is returned when a route alias already exists.
	ErrAliasTaken = errors.New("route alias already taken")
	// ErrInvalidPassword is returned when a route password does not match.
	ErrInvalidPassword = errors.New("invalid route password")
	// ErrUnauthorized is returned when a caller token is missing or invalid.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrInvalidInput is returned when a request body fails validation.
	ErrInvalidInput = errors.New("invalid input")
	// ErrRouteClosed is returned when a route no longer accepts live mutations.
	ErrRouteClosed = errors.New("route closed")
	// ErrSharingNotAllowed is returned when the route sharing policy forbids tracking.
	ErrSharingNotAllowed = errors.New("sharing not allowed")
	// ErrTrackingLimitReached is returned when all active tracking slots are occupied.
	ErrTrackingLimitReached = errors.New("tracking limit reached")
)

var palette = []string{
	"#22c55e",
	"#2563eb",
	"#f97316",
	"#e11d48",
	"#14b8a6",
	"#a855f7",
	"#f59e0b",
	"#06b6d4",
	"#84cc16",
	"#ef4444",
}

// Repository defines the route data access operations.
type Repository interface {
	CreateRoute(context.Context, CreateRouteRepoParams) (CreateRouteRepoResult, error)
	CreateMember(context.Context, CreateMemberRepoParams) (CreateMemberRepoResult, error)
	GetRouteByCode(context.Context, string) (Route, string, error)
	GetAuthorizedMemberByTokenHash(context.Context, string) (AuthorizedMember, error)
	GetAuthorizedOwnerByTokenHash(context.Context, string) (AuthorizedMember, error)
	GetMembersByRouteID(context.Context, string) ([]Member, error)
	CountMembersByRouteID(context.Context, string) (int, error)
	CountTrackingMembers(context.Context, string) (int, error)
	StartTrackingMember(context.Context, string, string) (StartSharingRepoResult, error)
	StopTrackingMember(context.Context, string, string) (Member, error)
	RecordPosition(context.Context, RecordPositionRepoParams) (PositionUpdateResult, error)
	UpdateRoute(context.Context, string, UpdateRouteRepoParams) (Route, error)
	LeaveMember(context.Context, string) (Member, error)
	DeleteRoute(context.Context, string) error
}

// Service coordinates route business logic.
type Service struct {
	defaultMaxTrackingMembers int
	repo                      Repository
}

// NewService builds the route service.
func NewService(repo Repository, defaultMaxTrackingMembers int) *Service {
	return &Service{
		defaultMaxTrackingMembers: defaultMaxTrackingMembers,
		repo:                      repo,
	}
}

// CreateRouteRepoParams contains repository route create data.
type CreateRouteRepoParams struct {
	Route           CreateRouteRepoRoute
	Owner           CreateRouteRepoOwner
	MemberTokenHash string
	OwnerTokenHash  string
}

// CreateRouteRepoRoute contains route persistence fields.
type CreateRouteRepoRoute struct {
	Code               string
	Name               string
	Description        string
	PasswordHash       string
	SharingPolicy      string
	Status             string
	MaxTrackingMembers int
}

// CreateRouteRepoOwner contains owner persistence fields.
type CreateRouteRepoOwner struct {
	ClientID      string
	DisplayName   string
	TransportMode string
	Status        string
	Color         string
}

// CreateRouteRepoResult contains created route and owner.
type CreateRouteRepoResult struct {
	Route Route
	Owner Member
}

// CreateMemberRepoParams contains route join persistence fields.
type CreateMemberRepoParams struct {
	RouteID         string
	ClientID        string
	DisplayName     string
	TransportMode   string
	Status          string
	Color           string
	MemberTokenHash string
}

// CreateMemberRepoResult contains the joined member.
type CreateMemberRepoResult struct {
	Member Member
}

// UpdateRouteRepoParams contains persistence fields for route updates.
type UpdateRouteRepoParams struct {
	Name        *string
	Description *string
	Status      *string
}

// StartSharingRepoResult contains the member and opened path segment.
type StartSharingRepoResult struct {
	Member  Member
	Segment PathSegment
}

// RecordPositionRepoParams contains persistence fields for one accepted point.
type RecordPositionRepoParams struct {
	RouteID          string
	MemberID         string
	Latitude         float64
	Longitude        float64
	AccuracyM        *float64
	AltitudeM        *float64
	SpeedMPS         *float64
	HeadingDeg       *float64
	ClientRecordedAt *time.Time
	RawPayload       json.RawMessage
}

// CreateRoute validates input and creates a route plus owner membership.
func (s *Service) CreateRoute(ctx context.Context, input CreateRouteInput) (CreateRouteResult, error) {
	normalized, err := normalizeCreateInput(input)
	if err != nil {
		return CreateRouteResult{}, err
	}

	passwordHash, err := hashPassword(normalized.Password)
	if err != nil {
		return CreateRouteResult{}, fmt.Errorf("create route: %w", err)
	}

	memberToken, memberTokenHash, err := newOpaqueToken()
	if err != nil {
		return CreateRouteResult{}, fmt.Errorf("create route member token: %w", err)
	}

	ownerToken, ownerTokenHash, err := newOpaqueToken()
	if err != nil {
		return CreateRouteResult{}, fmt.Errorf("create route owner token: %w", err)
	}

	var created CreateRouteRepoResult
	for attempt := 0; attempt < maxCodeAttempts; attempt++ {
		code, codeErr := newRouteCode(defaultCodeLength)
		if codeErr != nil {
			return CreateRouteResult{}, fmt.Errorf("create route code: %w", codeErr)
		}

		created, err = s.repo.CreateRoute(ctx, CreateRouteRepoParams{
			Route: CreateRouteRepoRoute{
				Code:               code,
				Name:               normalized.Name,
				Description:        normalized.Description,
				PasswordHash:       passwordHash,
				SharingPolicy:      normalized.SharingPolicy,
				Status:             RouteStatusActive,
				MaxTrackingMembers: s.defaultMaxTrackingMembers,
			},
			Owner: CreateRouteRepoOwner{
				ClientID:      normalized.ClientID,
				DisplayName:   normalized.DisplayName,
				TransportMode: normalized.TransportMode,
				Status:        MemberStatusSpectating,
				Color:         palette[0],
			},
			MemberTokenHash: memberTokenHash,
			OwnerTokenHash:  ownerTokenHash,
		})
		if err == nil {
			return CreateRouteResult{
				Route:       created.Route,
				Owner:       created.Owner,
				MemberToken: memberToken,
				OwnerToken:  ownerToken,
			}, nil
		}

		if !isCodeConflict(err) {
			return CreateRouteResult{}, fmt.Errorf("create route: %w", err)
		}
	}

	return CreateRouteResult{}, fmt.Errorf("create route: exhausted code generation attempts")
}

// AccessRoute returns join-screen metadata for a route.
func (s *Service) AccessRoute(ctx context.Context, code string) (AccessRouteResult, error) {
	route, _, err := s.repo.GetRouteByCode(ctx, normalizeCode(code))
	if err != nil {
		return AccessRouteResult{}, err
	}

	return AccessRouteResult{
		Code:             route.Code,
		Name:             route.Name,
		Description:      route.Description,
		Status:           route.Status,
		RequiresPassword: route.HasPassword,
		SharingPolicy:    route.SharingPolicy,
	}, nil
}

// JoinRoute creates a new route member and member token.
func (s *Service) JoinRoute(ctx context.Context, code string, input JoinRouteInput) (JoinRouteResult, error) {
	normalized, err := normalizeJoinInput(input)
	if err != nil {
		return JoinRouteResult{}, err
	}

	route, passwordHash, err := s.repo.GetRouteByCode(ctx, normalizeCode(code))
	if err != nil {
		return JoinRouteResult{}, err
	}

	if route.Status != RouteStatusActive {
		return JoinRouteResult{}, ErrRouteClosed
	}

	if route.HasPassword {
		if err := verifyPassword(passwordHash, normalized.Password); err != nil {
			return JoinRouteResult{}, ErrInvalidPassword
		}
	}

	memberCount, err := s.repo.CountMembersByRouteID(ctx, route.ID)
	if err != nil {
		return JoinRouteResult{}, fmt.Errorf("join route count all members: %w", err)
	}

	token, tokenHash, err := newOpaqueToken()
	if err != nil {
		return JoinRouteResult{}, fmt.Errorf("join route token: %w", err)
	}

	created, err := s.repo.CreateMember(ctx, CreateMemberRepoParams{
		RouteID:         route.ID,
		ClientID:        normalized.ClientID,
		DisplayName:     normalized.DisplayName,
		TransportMode:   normalized.TransportMode,
		Status:          MemberStatusSpectating,
		Color:           palette[(memberCount+1)%len(palette)],
		MemberTokenHash: tokenHash,
	})
	if err != nil {
		return JoinRouteResult{}, fmt.Errorf("join route: %w", err)
	}

	return JoinRouteResult{
		Route:       route,
		Member:      created.Member,
		MemberToken: token,
	}, nil
}

// UpdateRoute updates owner-managed route fields.
func (s *Service) UpdateRoute(ctx context.Context, code, ownerToken string, input UpdateRouteInput) (Route, error) {
	if strings.TrimSpace(ownerToken) == "" {
		return Route{}, ErrUnauthorized
	}

	authorized, err := s.repo.GetAuthorizedOwnerByTokenHash(ctx, tokenHash(ownerToken))
	if err != nil {
		return Route{}, err
	}

	if normalizeCode(code) != authorized.Route.Code {
		return Route{}, ErrUnauthorized
	}

	params := UpdateRouteRepoParams{}

	if name := strings.TrimSpace(input.Name); name != "" {
		params.Name = &name
	}

	description := strings.TrimSpace(input.Description)
	if input.Description != "" || params.Name != nil {
		params.Description = &description
	}

	if input.Status != "" {
		status := strings.TrimSpace(input.Status)
		if status != RouteStatusClosed {
			return Route{}, ErrInvalidInput
		}

		if authorized.Route.Status == RouteStatusClosed {
			return Route{}, ErrRouteClosed
		}

		params.Status = &status
	}

	if params.Name == nil && params.Description == nil && params.Status == nil {
		return Route{}, ErrInvalidInput
	}

	updated, err := s.repo.UpdateRoute(ctx, authorized.Route.ID, params)
	if err != nil {
		return Route{}, fmt.Errorf("update route: %w", err)
	}

	return updated, nil
}

// LeaveRoute marks the authenticated member as left.
func (s *Service) LeaveRoute(ctx context.Context, code, memberToken string) (LeaveRouteResult, error) {
	if strings.TrimSpace(memberToken) == "" {
		return LeaveRouteResult{}, ErrUnauthorized
	}

	authorized, err := s.repo.GetAuthorizedMemberByTokenHash(ctx, tokenHash(memberToken))
	if err != nil {
		return LeaveRouteResult{}, err
	}

	if normalizeCode(code) != authorized.Route.Code {
		return LeaveRouteResult{}, ErrUnauthorized
	}

	member, err := s.repo.LeaveMember(ctx, authorized.Member.ID)
	if err != nil {
		return LeaveRouteResult{}, fmt.Errorf("leave route: %w", err)
	}

	return LeaveRouteResult{Member: member}, nil
}

// StartSharing marks an authenticated member as actively sharing location.
func (s *Service) StartSharing(ctx context.Context, code, memberToken string) (StartSharingResult, error) {
	if strings.TrimSpace(memberToken) == "" {
		return StartSharingResult{}, ErrUnauthorized
	}

	authorized, err := s.repo.GetAuthorizedMemberByTokenHash(ctx, tokenHash(memberToken))
	if err != nil {
		return StartSharingResult{}, err
	}

	if normalizeCode(code) != authorized.Route.Code {
		return StartSharingResult{}, ErrUnauthorized
	}

	if authorized.Route.Status != RouteStatusActive {
		return StartSharingResult{}, ErrRouteClosed
	}

	if authorized.Member.Status == MemberStatusLeft || authorized.Member.Status == MemberStatusTracking {
		return StartSharingResult{}, ErrInvalidInput
	}

	if !authorized.Member.IsOwner && authorized.Route.SharingPolicy != SharingPolicyEveryoneCanShare {
		return StartSharingResult{}, ErrSharingNotAllowed
	}

	trackingCount, err := s.repo.CountTrackingMembers(ctx, authorized.Route.ID)
	if err != nil {
		return StartSharingResult{}, fmt.Errorf("start sharing count tracking members: %w", err)
	}

	if trackingCount >= authorized.Route.MaxTrackingMembers {
		return StartSharingResult{}, ErrTrackingLimitReached
	}

	result, err := s.repo.StartTrackingMember(ctx, authorized.Route.ID, authorized.Member.ID)
	if err != nil {
		return StartSharingResult{}, fmt.Errorf("start sharing: %w", err)
	}

	return StartSharingResult{
		Member:  result.Member,
		Segment: result.Segment,
	}, nil
}

// StopSharing returns an authenticated tracking member to spectator state.
func (s *Service) StopSharing(ctx context.Context, code, memberToken string) (StopSharingResult, error) {
	if strings.TrimSpace(memberToken) == "" {
		return StopSharingResult{}, ErrUnauthorized
	}

	authorized, err := s.repo.GetAuthorizedMemberByTokenHash(ctx, tokenHash(memberToken))
	if err != nil {
		return StopSharingResult{}, err
	}

	if normalizeCode(code) != authorized.Route.Code {
		return StopSharingResult{}, ErrUnauthorized
	}

	if authorized.Route.Status != RouteStatusActive {
		return StopSharingResult{}, ErrRouteClosed
	}

	if authorized.Member.Status != MemberStatusTracking {
		return StopSharingResult{}, ErrInvalidInput
	}

	member, err := s.repo.StopTrackingMember(ctx, authorized.Route.ID, authorized.Member.ID)
	if err != nil {
		return StopSharingResult{}, fmt.Errorf("stop sharing: %w", err)
	}

	return StopSharingResult{Member: member}, nil
}

// RecordPosition validates and persists one position update for an authenticated tracking member.
func (s *Service) RecordPosition(ctx context.Context, memberToken string, input PositionUpdateInput) (PositionUpdateResult, error) {
	if strings.TrimSpace(memberToken) == "" {
		return PositionUpdateResult{}, ErrUnauthorized
	}

	normalized, err := normalizePositionUpdateInput(input)
	if err != nil {
		return PositionUpdateResult{}, err
	}

	authorized, err := s.repo.GetAuthorizedMemberByTokenHash(ctx, tokenHash(memberToken))
	if err != nil {
		return PositionUpdateResult{}, err
	}

	if authorized.Route.Status != RouteStatusActive {
		return PositionUpdateResult{}, ErrRouteClosed
	}

	if authorized.Member.Status != MemberStatusTracking {
		return PositionUpdateResult{}, ErrInvalidInput
	}

	result, err := s.repo.RecordPosition(ctx, RecordPositionRepoParams{
		RouteID:          authorized.Route.ID,
		MemberID:         authorized.Member.ID,
		Latitude:         normalized.Latitude,
		Longitude:        normalized.Longitude,
		AccuracyM:        normalized.AccuracyM,
		AltitudeM:        normalized.AltitudeM,
		SpeedMPS:         normalized.SpeedMPS,
		HeadingDeg:       normalized.HeadingDeg,
		ClientRecordedAt: normalized.ClientRecordedAt,
		RawPayload:       normalized.RawPayload,
	})
	if err != nil {
		return PositionUpdateResult{}, fmt.Errorf("record position: %w", err)
	}

	return result, nil
}

// DeleteRoute removes a route owned by the authenticated owner token.
func (s *Service) DeleteRoute(ctx context.Context, code, ownerToken string) error {
	if strings.TrimSpace(ownerToken) == "" {
		return ErrUnauthorized
	}

	authorized, err := s.repo.GetAuthorizedOwnerByTokenHash(ctx, tokenHash(ownerToken))
	if err != nil {
		return err
	}

	if normalizeCode(code) != authorized.Route.Code {
		return ErrUnauthorized
	}

	if err := s.repo.DeleteRoute(ctx, authorized.Route.ID); err != nil {
		return fmt.Errorf("delete route: %w", err)
	}

	return nil
}

// AuthorizeMember resolves a member token into route/member data.
func (s *Service) AuthorizeMember(ctx context.Context, memberToken string) (AuthorizedMember, error) {
	if strings.TrimSpace(memberToken) == "" {
		return AuthorizedMember{}, ErrUnauthorized
	}

	authorized, err := s.repo.GetAuthorizedMemberByTokenHash(ctx, tokenHash(memberToken))
	if err != nil {
		return AuthorizedMember{}, err
	}

	return authorized, nil
}

// Snapshot returns the full route bootstrap payload for an authenticated member.
func (s *Service) Snapshot(ctx context.Context, code, memberToken string) (Snapshot, error) {
	if strings.TrimSpace(memberToken) == "" {
		return Snapshot{}, ErrUnauthorized
	}

	authorized, err := s.repo.GetAuthorizedMemberByTokenHash(ctx, tokenHash(memberToken))
	if err != nil {
		return Snapshot{}, err
	}

	if normalizeCode(code) != authorized.Route.Code {
		return Snapshot{}, ErrUnauthorized
	}

	members, err := s.repo.GetMembersByRouteID(ctx, authorized.Route.ID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("load snapshot members: %w", err)
	}

	trackingCount, err := s.repo.CountTrackingMembers(ctx, authorized.Route.ID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("load tracking count: %w", err)
	}

	snapshotMembers := make([]SnapshotMember, 0, len(members))
	for _, member := range members {
		role := RoleMember
		if member.IsOwner {
			role = RoleOwner
		}

		snapshotMembers = append(snapshotMembers, SnapshotMember{
			ID:            member.ID,
			DisplayName:   member.DisplayName,
			TransportMode: member.TransportMode,
			Role:          role,
			Status:        member.Status,
			Color:         member.Color,
			JoinedAt:      member.JoinedAt,
			LeftAt:        member.LeftAt,
			Paths:         []PathSegment{},
		})
	}

	return Snapshot{
		Route:   authorized.Route,
		Members: snapshotMembers,
		Viewer:  buildViewerCapabilities(authorized, trackingCount),
	}, nil
}

func buildViewerCapabilities(authorized AuthorizedMember, trackingCount int) ViewerCapabilities {
	role := RoleMember
	if authorized.Member.IsOwner {
		role = RoleOwner
	}

	canStartSharing := authorized.Route.Status == RouteStatusActive &&
		authorized.Member.Status != MemberStatusTracking &&
		trackingCount < authorized.Route.MaxTrackingMembers &&
		(authorized.Member.IsOwner || authorized.Route.SharingPolicy == SharingPolicyEveryoneCanShare)

	return ViewerCapabilities{
		MemberID:        authorized.Member.ID,
		Role:            role,
		Status:          authorized.Member.Status,
		CanStartSharing: canStartSharing,
		CanStopSharing:  authorized.Route.Status == RouteStatusActive && authorized.Member.Status == MemberStatusTracking,
		CanLeaveRoute:   authorized.Member.Status != MemberStatusLeft,
		CanCloseRoute:   authorized.Member.IsOwner && authorized.Route.Status == RouteStatusActive,
		CanDeleteRoute:  authorized.Member.IsOwner,
		CanEditRoute:    authorized.Member.IsOwner,
	}
}

func normalizeCreateInput(input CreateRouteInput) (CreateRouteInput, error) {
	normalized := CreateRouteInput{
		ClientID:      strings.TrimSpace(input.ClientID),
		DisplayName:   strings.TrimSpace(input.DisplayName),
		TransportMode: strings.ToLower(strings.TrimSpace(input.TransportMode)),
		Name:          strings.TrimSpace(input.Name),
		Description:   strings.TrimSpace(input.Description),
		Password:      input.Password,
		SharingPolicy: strings.TrimSpace(input.SharingPolicy),
	}

	if normalized.ClientID == "" || normalized.DisplayName == "" || normalized.Name == "" {
		return CreateRouteInput{}, ErrInvalidInput
	}

	if _, ok := validTransportModes[normalized.TransportMode]; !ok {
		return CreateRouteInput{}, ErrInvalidInput
	}

	if _, ok := validSharingPolicies[normalized.SharingPolicy]; !ok {
		return CreateRouteInput{}, ErrInvalidInput
	}

	return normalized, nil
}

func normalizeJoinInput(input JoinRouteInput) (JoinRouteInput, error) {
	normalized := JoinRouteInput{
		ClientID:      strings.TrimSpace(input.ClientID),
		DisplayName:   strings.TrimSpace(input.DisplayName),
		TransportMode: strings.ToLower(strings.TrimSpace(input.TransportMode)),
		Password:      input.Password,
	}

	if normalized.ClientID == "" || normalized.DisplayName == "" {
		return JoinRouteInput{}, ErrInvalidInput
	}

	if _, ok := validTransportModes[normalized.TransportMode]; !ok {
		return JoinRouteInput{}, ErrInvalidInput
	}

	return normalized, nil
}

func normalizePositionUpdateInput(input PositionUpdateInput) (PositionUpdateInput, error) {
	normalized := input
	if !isFiniteInRange(normalized.Latitude, -90, 90) ||
		!isFiniteInRange(normalized.Longitude, -180, 180) {
		return PositionUpdateInput{}, ErrInvalidInput
	}

	if !isNilOrFiniteAtLeast(normalized.AccuracyM, 0) ||
		!isNilOrFinite(normalized.AltitudeM) ||
		!isNilOrFiniteAtLeast(normalized.SpeedMPS, 0) ||
		!isNilOrFiniteInRange(normalized.HeadingDeg, 0, 360) {
		return PositionUpdateInput{}, ErrInvalidInput
	}

	if len(normalized.RawPayload) == 0 {
		normalized.RawPayload = []byte("{}")
	}

	return normalized, nil
}

func isFiniteInRange(value, minValue, maxValue float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= minValue && value <= maxValue
}

func isNilOrFinite(value *float64) bool {
	return value == nil || (!math.IsNaN(*value) && !math.IsInf(*value, 0))
}

func isNilOrFiniteAtLeast(value *float64, minValue float64) bool {
	return value == nil || (isNilOrFinite(value) && *value >= minValue)
}

func isNilOrFiniteInRange(value *float64, minValue, maxValue float64) bool {
	return value == nil || (isNilOrFinite(value) && *value >= minValue && *value < maxValue)
}

func normalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func hashPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return string(hashed), nil
}

func verifyPassword(hash, password string) error {
	if hash == "" || password == "" {
		return ErrInvalidPassword
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrInvalidPassword
	}

	return nil
}

func newOpaqueToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}

	token := hex.EncodeToString(raw)
	return token, tokenHash(token), nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newRouteCode(length int) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

	raw := make([]byte, length)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate route code: %w", err)
	}

	code := make([]byte, length)
	for index, value := range raw {
		code[index] = alphabet[int(value)%len(alphabet)]
	}

	return string(code), nil
}

func isCodeConflict(err error) bool {
	if errors.Is(err, ErrAliasTaken) {
		return false
	}

	return strings.Contains(strings.ToLower(err.Error()), "routes_code_key")
}
