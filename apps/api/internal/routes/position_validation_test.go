package routes

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

func TestPositionValidation(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		change func(*PositionUpdateInput)
		want   string
	}{
		{"valid", func(*PositionUpdateInput) {}, ""},
		{"accuracy boundary", func(p *PositionUpdateInput) { *p.AccuracyM = 100 }, ""},
		{"accuracy missing", func(p *PositionUpdateInput) { p.AccuracyM = nil }, "accuracy_required"},
		{"accuracy excessive", func(p *PositionUpdateInput) { *p.AccuracyM = 100.01 }, "accuracy_too_low"},
		{"accuracy negative", func(p *PositionUpdateInput) { *p.AccuracyM = -1 }, "invalid_input"},
		{"accuracy infinity", func(p *PositionUpdateInput) { *p.AccuracyM = math.Inf(1) }, "invalid_input"},
		{"latitude nan", func(p *PositionUpdateInput) { p.Latitude = math.NaN() }, "invalid_input"},
		{"longitude outside range", func(p *PositionUpdateInput) { p.Longitude = 181 }, "invalid_input"},
		{"coordinate boundaries", func(p *PositionUpdateInput) { p.Latitude = -90; p.Longitude = 180 }, ""},
		{"timestamp missing", func(p *PositionUpdateInput) { p.ClientRecordedAt = nil }, "timestamp_required"},
		{"old boundary", func(p *PositionUpdateInput) { v := now.Add(-2 * time.Minute); p.ClientRecordedAt = &v }, ""},
		{"old", func(p *PositionUpdateInput) { v := now.Add(-2*time.Minute - time.Microsecond); p.ClientRecordedAt = &v }, "timestamp_too_old"},
		{"future boundary", func(p *PositionUpdateInput) { v := now.Add(30 * time.Second); p.ClientRecordedAt = &v }, ""},
		{"future", func(p *PositionUpdateInput) { v := now.Add(30*time.Second + time.Microsecond); p.ClientRecordedAt = &v }, "timestamp_in_future"},
		{"negative speed", func(p *PositionUpdateInput) { v := -1.0; p.SpeedMPS = &v }, "invalid_input"},
		{"infinite altitude", func(p *PositionUpdateInput) { v := math.Inf(1); p.AltitudeM = &v }, "invalid_input"},
		{"heading boundary", func(p *PositionUpdateInput) { v := 360.0; p.HeadingDeg = &v }, ""},
		{"heading outside range", func(p *PositionUpdateInput) { v := 360.01; p.HeadingDeg = &v }, "invalid_input"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accuracy := 10.0
			stamp := now
			input := PositionUpdateInput{Latitude: 46, Longitude: 14, AccuracyM: &accuracy, ClientRecordedAt: &stamp}
			tt.change(&input)
			called := false
			service := NewService(stubRepository{
				getAuthorizedMemberByTokenFn: func(context.Context, string) (AuthorizedMember, error) {
					return AuthorizedMember{Route: Route{ID: "r", Status: RouteStatusActive}, Member: Member{ID: "m", Status: MemberStatusTracking}}, nil
				},
				recordPositionFn: func(_ context.Context, p RecordPositionRepoParams) (PositionUpdateResult, error) {
					called = true
					return PositionUpdateResult{}, nil
				},
			}, 10)
			service.now = func() time.Time { return now }
			_, err := service.RecordPosition(context.Background(), "token", input)
			if tt.want == "" {
				if err != nil || !called {
					t.Fatalf("acceptance: called=%v err=%v", called, err)
				}
				return
			}
			if called {
				t.Fatal("rejected sample reached persistence")
			}
			if tt.want == "invalid_input" {
				if !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("got %v", err)
				}
				return
			}
			var rejection *PositionValidationError
			if !errors.As(err, &rejection) || rejection.Code != tt.want {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
}

func floatPointer(v float64) *float64    { return &v }
func timePointer(v time.Time) *time.Time { return &v }

func TestPositionHistory(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	tests := []struct {
		name     string
		delta    time.Duration
		distance float64
		want     string
	}{
		{"duplicate", 0, 0, "duplicate_timestamp"},
		{"out of order", -time.Second, 0, "out_of_order_timestamp"},
		{"stationary jitter", time.Millisecond, 19, ""},
		{"speed boundary", time.Second, 420, ""},
		{"impossible jump", time.Second, 420.01, "impossible_jump"},
		{"airplane", time.Second, 320, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePositionHistory(now.Add(tt.delta), now, tt.distance, 10, 10, 400)
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var rejection *PositionValidationError
			if !errors.As(err, &rejection) || rejection.Code != tt.want {
				t.Fatalf("got %v want %s", err, tt.want)
			}
		})
	}
}
