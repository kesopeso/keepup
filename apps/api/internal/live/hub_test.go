package live

import "testing"

func TestBroadcastDeliversToRouteSubscribers(t *testing.T) {
	t.Parallel()

	hub := NewHub()
	routeSubscriber := hub.Subscribe("route-1", "member-1")
	defer routeSubscriber.Close()
	otherRouteSubscriber := hub.Subscribe("route-2", "member-2")
	defer otherRouteSubscriber.Close()

	delivered := hub.Broadcast("route-1", Event{
		"type": "member_joined",
	})
	if delivered != 1 {
		t.Fatalf("Broadcast() delivered = %d, want 1", delivered)
	}

	event := <-routeSubscriber.Events()
	if event["type"] != "member_joined" {
		t.Fatalf("event type = %v, want member_joined", event["type"])
	}

	select {
	case event := <-otherRouteSubscriber.Events():
		t.Fatalf("unexpected event on other route: %#v", event)
	default:
	}
}

func TestClosedSubscriptionDoesNotReceiveBroadcasts(t *testing.T) {
	t.Parallel()

	hub := NewHub()
	subscription := hub.Subscribe("route-1", "member-1")
	subscription.Close()

	delivered := hub.Broadcast("route-1", Event{
		"type": "member_left",
	})
	if delivered != 0 {
		t.Fatalf("Broadcast() delivered = %d, want 0", delivered)
	}

	if count := hub.RouteConnectionCount("route-1"); count != 0 {
		t.Fatalf("RouteConnectionCount() = %d, want 0", count)
	}
}
