package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"keepup/apps/api/internal/auth"
	"keepup/apps/api/internal/live"
	"keepup/apps/api/internal/routes"
)

// Account sockets have one command loop and a serialized writer. Revocation may
// close them synchronously without racing command state or ordinary event writes.
type accountConnection struct {
	mu      sync.Mutex
	conn    *websocket.Conn
	bound   routes.LiveAccount
	routeID string
	closed  bool
}

func (c *accountConnection) write(ctx context.Context, event any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return context.Canceled
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return wsjson.Write(ctx, c.conn, event)
}

func (c *accountConnection) close(reason string, revoked bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	if revoked {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = wsjson.Write(ctx, c.conn, live.Event{"type": "session_revoked", "reason": reason})
		cancel()
	}
	_ = c.conn.Close(websocket.StatusPolicyViolation, reason)
}

func (s *Server) revokeAccountSockets(change auth.Revocation) {
	s.accountSockets.Range(func(_, value any) bool {
		c := value.(*accountConnection)
		if change.Matches(c.bound.Session.User.ID, c.bound.Session.ID) {
			c.close(change.Reason, true)
		}
		return true
	})
}

func (s *Server) closeAccountSockets(routeID, memberID string, closedRoute *routes.Route) {
	s.accountSockets.Range(func(_, value any) bool {
		c := value.(*accountConnection)
		if c.routeID == routeID && (memberID == "" || memberID == c.bound.MemberID) {
			reason := "access revoked"
			if closedRoute != nil {
				_ = c.write(context.Background(), live.Event{"type": "route_closed", "route": *closedRoute})
				reason = "route closed"
			}
			c.close(reason, false)
		}
		return true
	})
}

func (s *Server) rejectAccountSocket(ctx context.Context, c *accountConnection) {
	reason := s.accountRoutes.LiveRevocationReason(ctx, c.bound)
	if reason != "" {
		c.close(reason, true)
	} else {
		c.close("access revoked", false)
	}
}

func (s *Server) handleAccountWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer func() { _ = conn.CloseNow() }()
	conn.SetReadLimit(4096)
	first, err := readWebSocketAuth(r.Context(), conn, s.appConfig.WebSocketAuthTimeout, true)
	if err != nil {
		_ = conn.Close(websocket.StatusPolicyViolation, "authentication required")
		return
	}
	bound, err := s.accountRoutes.ConsumeSocketTicket(r.Context(), first.Ticket)
	if err == nil {
		bound, err = s.accountRoutes.OpenLive(r.Context(), bound)
	}
	if err != nil {
		_ = conn.Close(websocket.StatusPolicyViolation, "unauthorized")
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	defer func() {
		cancel()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if err := s.accountRoutes.DisconnectLive(cleanupCtx, bound); err != nil {
			s.logger.Error("disconnect account viewer", "error", err)
		}
	}()
	// Establish immutable registry metadata before publishing this connection.
	authorized, err := s.accountRoutes.AuthorizeLive(ctx, bound)
	if err != nil {
		_ = conn.Close(websocket.StatusPolicyViolation, "unauthorized")
		return
	}
	c := &accountConnection{conn: conn, bound: bound, routeID: authorized.Route.ID}
	s.accountSockets.Store(bound.ConnectionID, c)
	defer s.accountSockets.Delete(bound.ConnectionID)
	// Registration precedes the final authority check. Concurrent revocation
	// either sees the socket or makes this check fail.
	if _, err = s.accountRoutes.AuthorizeLive(ctx, bound); err != nil {
		s.rejectAccountSocket(ctx, c)
		return
	}
	subscription := s.liveHub.Subscribe(authorized.Route.ID, bound.MemberID)
	defer subscription.Close()
	viewer, err := s.accountRoutes.LiveViewer(ctx, bound)
	if err != nil {
		s.rejectAccountSocket(ctx, c)
		return
	}
	if err = c.write(ctx, live.Event{"type": "connection_established", "connectionId": bound.ConnectionID, "viewer": viewer,
		"route":  map[string]any{"id": authorized.Route.ID, "code": authorized.Route.Code, "status": authorized.Route.Status},
		"member": map[string]any{"id": bound.MemberID, "status": authorized.Member.Status}}); err != nil {
		return
	}
	if bound.PresenceRestored {
		s.broadcastLiveEvent(authorized.Route.ID, live.Event{"type": "member_back_online", "member": authorized.Member})
	}
	messages := make(chan json.RawMessage)
	readErrors := make(chan error, 1)
	go func() {
		for {
			var raw json.RawMessage
			if err := wsjson.Read(ctx, conn, &raw); err != nil {
				select {
				case readErrors <- err:
				case <-ctx.Done():
				}
				return
			}
			select {
			case messages <- raw:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		if err := runWebSocketHeartbeat(ctx, conn, s.appConfig.WebSocketPingInterval, s.appConfig.WebSocketPingTimeout); err != nil {
			select {
			case readErrors <- err:
			case <-ctx.Done():
			}
		}
	}()
	ticker := time.NewTicker(s.appConfig.WebSocketPingInterval)
	defer ticker.Stop()
	authorizedLive := func() bool {
		if _, err := s.accountRoutes.AuthorizeLive(ctx, bound); err != nil {
			s.rejectAccountSocket(ctx, c)
			return false
		}
		return true
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-readErrors:
			return
		case <-ticker.C:
			if !authorizedLive() {
				return
			}
		case event, ok := <-subscription.Events():
			if !ok || !authorizedLive() {
				return
			}
			if err = c.write(ctx, event); err != nil {
				return
			}
			viewer, err := s.accountRoutes.LiveViewer(ctx, bound)
			if err != nil {
				s.rejectAccountSocket(ctx, c)
				return
			}
			if err = c.write(ctx, live.Event{"type": "viewer_updated", "viewer": viewer}); err != nil {
				return
			}
		case raw := <-messages:
			if !authorizedLive() {
				return
			}
			if err = s.accountCommand(ctx, c, raw); err != nil {
				return
			}
		}
	}
}

func (s *Server) accountCommand(ctx context.Context, c *accountConnection, raw json.RawMessage) error {
	var message webSocketClientMessage
	if err := json.Unmarshal(raw, &message); err != nil {
		var envelope struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(raw, &envelope)
		if envelope.Type == "position_update" {
			return c.write(ctx, live.Event{"type": "position_rejected", "error": "invalid_input"})
		}
		return c.write(ctx, live.Event{"type": "message_rejected", "error": "invalid_input"})
	}
	bound := c.bound
	bound.SharingGeneration = message.SharingGeneration
	switch message.Type {
	case "start_sharing":
		result, err := s.accountRoutes.StartSharing(ctx, bound)
		if err != nil {
			return c.write(ctx, commandRejectedEvent(message, message.Type, err))
		}
		ack := commandAckEvent(message, message.Type)
		ack["sharingGeneration"] = result.SharingGeneration
		if err = c.write(ctx, ack); err != nil {
			return err
		}
		if result.Changed {
			s.broadcastLiveEvent(c.routeID, live.Event{"type": "member_started_sharing", "member": result.Member, "segment": result.Segment})
		}
	case "stop_sharing":
		result, err := s.accountRoutes.StopSharing(ctx, bound)
		if err != nil {
			return c.write(ctx, commandRejectedEvent(message, message.Type, err))
		}
		if err = c.write(ctx, commandAckEvent(message, message.Type)); err != nil {
			return err
		}
		s.broadcastLiveEvent(c.routeID, live.Event{"type": "member_stopped_sharing", "member": result.Member})
	case "position_update":
		input, err := positionUpdateInput(message, raw)
		var result routes.PositionUpdateResult
		if err == nil {
			result, err = s.accountRoutes.RecordPosition(ctx, bound, input)
		}
		if err != nil {
			if errors.Is(err, auth.ErrSession) {
				s.rejectAccountSocket(ctx, c)
				return err
			}
			s.logPositionRejection(err)
			return c.write(ctx, live.Event{"type": "position_rejected", "error": routeErrorReason(err)})
		}
		if result.RecoveredMember != nil {
			s.broadcastLiveEvent(c.routeID, live.Event{"type": "member_back_online", "member": result.RecoveredMember})
		}
		s.broadcastLiveEvent(c.routeID, live.Event{"type": "position_updated", "memberId": result.MemberID, "segmentId": result.SegmentID, "point": result.Point})
	default:
		return c.write(ctx, live.Event{"type": "message_rejected", "error": "unknown_message"})
	}
	return nil
}
