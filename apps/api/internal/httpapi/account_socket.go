package httpapi

import (
	"context"
	"keepup/apps/api/internal/routes"
)

type liveRouteCommands interface {
	StartSharing(context.Context, string, string) (routes.StartSharingResult, error)
	StopSharing(context.Context, string, string) (routes.StopSharingResult, error)
	RecordPosition(context.Context, string, routes.PositionUpdateInput) (routes.PositionUpdateResult, error)
	MarkMemberOnline(context.Context, string, string) (routes.Member, bool, error)
}

type accountSocketCommands struct {
	service *routes.AccountService
	bound   routes.LiveAccount
}

// Presence transitions are internal lifecycle work, not caller authorization.
// Account members store the same UUID in the legacy client column during staging.
type accountPresence struct{ RouteService }

func accountPresenceMember(m routes.Member) routes.Member {
	m.UserID = m.ClientID
	m.ClientID = ""
	return m
}
func (p accountPresence) MarkMemberOnline(ctx context.Context, routeID, memberID string) (routes.Member, bool, error) {
	m, changed, err := p.RouteService.MarkMemberOnline(ctx, routeID, memberID)
	return accountPresenceMember(m), changed, err
}
func (p accountPresence) MarkMemberStale(ctx context.Context, routeID, memberID string) (routes.Member, bool, error) {
	m, changed, err := p.RouteService.MarkMemberStale(ctx, routeID, memberID)
	return accountPresenceMember(m), changed, err
}
func (p accountPresence) MarkMemberOffline(ctx context.Context, routeID, memberID string) (routes.Member, bool, error) {
	m, changed, err := p.RouteService.MarkMemberOffline(ctx, routeID, memberID)
	return accountPresenceMember(m), changed, err
}

func (a *accountSocketCommands) StartSharing(ctx context.Context, _, _ string) (routes.StartSharingResult, error) {
	return a.service.StartSharing(ctx, a.bound)
}
func (a *accountSocketCommands) StopSharing(ctx context.Context, _, _ string) (routes.StopSharingResult, error) {
	return a.service.StopSharing(ctx, a.bound)
}
func (a *accountSocketCommands) RecordPosition(ctx context.Context, _ string, input routes.PositionUpdateInput) (routes.PositionUpdateResult, error) {
	return a.service.RecordPosition(ctx, a.bound, input)
}
func (a *accountSocketCommands) MarkMemberOnline(ctx context.Context, _, _ string) (routes.Member, bool, error) {
	return a.service.MarkOnline(ctx, a.bound)
}
