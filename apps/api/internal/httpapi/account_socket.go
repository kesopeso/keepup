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
