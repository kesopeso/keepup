// Package live owns in-memory realtime route room coordination.
package live

import "sync"

// Hub tracks active WebSocket subscriptions by route.
type Hub struct {
	mu    sync.RWMutex
	rooms map[string]map[*Subscription]struct{}
}

// Subscription represents one live route connection.
type Subscription struct {
	hub      *Hub
	routeID  string
	memberID string
	closed   bool
}

// NewHub builds an empty live route hub.
func NewHub() *Hub {
	return &Hub{
		rooms: make(map[string]map[*Subscription]struct{}),
	}
}

// Subscribe registers one connection in a route room.
func (h *Hub) Subscribe(routeID, memberID string) *Subscription {
	h.mu.Lock()
	defer h.mu.Unlock()

	subscription := &Subscription{
		hub:      h,
		routeID:  routeID,
		memberID: memberID,
	}

	if h.rooms[routeID] == nil {
		h.rooms[routeID] = make(map[*Subscription]struct{})
	}
	h.rooms[routeID][subscription] = struct{}{}

	return subscription
}

// Close removes the subscription from its route room.
func (s *Subscription) Close() {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()

	if s.closed {
		return
	}

	s.closed = true
	room := s.hub.rooms[s.routeID]
	delete(room, s)
	if len(room) == 0 {
		delete(s.hub.rooms, s.routeID)
	}
}

// RouteID returns the subscribed route ID.
func (s *Subscription) RouteID() string {
	return s.routeID
}

// MemberID returns the subscribed member ID.
func (s *Subscription) MemberID() string {
	return s.memberID
}

// RouteConnectionCount returns the number of active subscriptions for a route.
func (h *Hub) RouteConnectionCount(routeID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.rooms[routeID])
}
