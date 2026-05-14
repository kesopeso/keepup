// Package live owns in-memory realtime route room coordination.
package live

import "sync"

const subscriptionEventBuffer = 32

// Hub tracks active WebSocket subscriptions by route.
type Hub struct {
	mu    sync.RWMutex
	rooms map[string]map[*Subscription]struct{}
}

// Event is one live route event ready to send to subscribed clients.
type Event map[string]any

// Subscription represents one live route connection.
type Subscription struct {
	hub      *Hub
	routeID  string
	memberID string
	events   chan Event
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
		events:   make(chan Event, subscriptionEventBuffer),
	}

	if h.rooms[routeID] == nil {
		h.rooms[routeID] = make(map[*Subscription]struct{})
	}
	h.rooms[routeID][subscription] = struct{}{}

	return subscription
}

// HasMemberConnection reports whether a member already has an active subscription.
func (h *Hub) HasMemberConnection(routeID, memberID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for subscription := range h.rooms[routeID] {
		if subscription.memberID == memberID && !subscription.closed {
			return true
		}
	}

	return false
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
	close(s.events)
	if len(room) == 0 {
		delete(s.hub.rooms, s.routeID)
	}
}

// Events returns the live event stream for this subscription.
func (s *Subscription) Events() <-chan Event {
	return s.events
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

// Broadcast publishes an event to active subscriptions in one route room.
func (h *Hub) Broadcast(routeID string, event Event) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	delivered := 0
	for subscription := range h.rooms[routeID] {
		select {
		case subscription.events <- event:
			delivered++
		default:
		}
	}

	return delivered
}
