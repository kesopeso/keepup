package routes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Each integration test uses an isolated schema, never application tables.
func positionTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostGIS integration tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	for _, extension := range []string{"postgis", "pgcrypto"} {
		if _, err = admin.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS "+extension+" WITH SCHEMA public"); err != nil {
			t.Fatal(err)
		}
	}
	schema := fmt.Sprintf("gps_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	files, err := filepath.Glob("../../../../db/migrations/*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("migrations: %v", err)
	}
	for _, file := range files {
		sql, readErr := os.ReadFile(file)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, err = pool.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
	}
	return pool
}

func TestPostgresPositionValidation(t *testing.T) {
	pool := positionTestPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo := NewPostgresRepository(pool)
	service := NewService(repo, 10)
	created, err := service.CreateRoute(ctx, CreateRouteInput{Name: "GPS test", ClientID: "test", DisplayName: "Owner", TransportMode: "airplane", SharingPolicy: "everyone_can_share"})
	if err != nil {
		t.Fatal(err)
	}
	token := created.MemberToken
	sharing, err := service.StartSharing(ctx, created.Route.Code, token)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Add(-10 * time.Second).Truncate(time.Microsecond)
	sample := PositionUpdateInput{Latitude: 0, Longitude: 179.999, AccuracyM: floatPointer(10), ClientRecordedAt: timePointer(base)}
	first, err := service.RecordPosition(ctx, token, sample)
	if err != nil {
		t.Fatal(err)
	}
	if first.Point.Seq != 1 {
		t.Fatal("first sequence")
	}
	// Duplicate simultaneous deliveries are serialized even across separate service instances.
	sample.ClientRecordedAt = timePointer(base.Add(time.Second))
	services := []*Service{service, NewService(NewPostgresRepository(pool), 10)}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, s := range services {
		wg.Add(1)
		go func(s *Service) { defer wg.Done(); _, err := s.RecordPosition(ctx, token, sample); results <- err }(s)
	}
	wg.Wait()
	close(results)
	accepted, duplicates := 0, 0
	for err := range results {
		if err == nil {
			accepted++
		} else {
			assertPositionCode(t, err, "duplicate_timestamp")
			duplicates++
		}
	}
	if accepted != 1 || duplicates != 1 {
		t.Fatalf("accepted=%d duplicate=%d", accepted, duplicates)
	}
	if _, _, err = repo.MarkMemberStale(ctx, created.Route.ID, created.Owner.ID); err != nil {
		t.Fatal(err)
	}
	// Equivalent timezone and sub-microsecond values are duplicates after normalization.
	duplicate := sample
	duplicate.ClientRecordedAt = timePointer(sample.ClientRecordedAt.Add(100 * time.Nanosecond).In(time.FixedZone("offset", 7200)))
	_, err = NewService(NewPostgresRepository(pool), 10).RecordPosition(ctx, token, duplicate)
	assertPositionCode(t, err, "duplicate_timestamp")
	var status string
	var count int
	if err = pool.QueryRow(ctx, "SELECT status FROM route_members WHERE id=$1", created.Owner.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != MemberStatusStale {
		t.Fatal("rejection recovered stale member")
	}
	if err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM position_points").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("stored %d points", count)
	}
	jump := sample
	jump.Longitude = 0
	jump.ClientRecordedAt = timePointer(base.Add(2 * time.Second))
	_, err = service.RecordPosition(ctx, token, jump)
	assertPositionCode(t, err, "impossible_jump")
	old := sample
	old.ClientRecordedAt = timePointer(base)
	_, err = service.RecordPosition(ctx, token, old)
	assertPositionCode(t, err, "out_of_order_timestamp")
	// Crossing the antimeridian is a short geographic movement, not a jump around Earth.
	valid := sample
	valid.Longitude = -179.999
	valid.ClientRecordedAt = timePointer(base.Add(2 * time.Second))
	recovered, err := service.RecordPosition(ctx, token, valid)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Point.Seq != 3 || recovered.RecoveredMember == nil {
		t.Fatal("valid sample did not recover without sequence gap")
	}
	if _, err = service.StopSharing(ctx, created.Route.Code, token); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.RecordPosition(ctx, RecordPositionRepoParams{RouteID: created.Route.ID, MemberID: created.Owner.ID}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("stopped member accepted: %v", err)
	}
	if _, err = service.StartSharing(ctx, created.Route.Code, token); err != nil {
		t.Fatal(err)
	}
	// A new segment accepts a timestamp already used by a previous segment.
	reset, err := service.RecordPosition(ctx, token, sample)
	if err != nil {
		t.Fatal(err)
	}
	if reset.Point.Seq != 1 || reset.SegmentID == sharing.Segment.ID {
		t.Fatal("new segment baseline did not reset")
	}
	closed := RouteStatusClosed
	if _, err = repo.UpdateRoute(ctx, created.Route.ID, UpdateRouteRepoParams{Status: &closed}); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.RecordPosition(ctx, RecordPositionRepoParams{RouteID: created.Route.ID, MemberID: created.Owner.ID}); !errors.Is(err, ErrRouteClosed) {
		t.Fatalf("closed route accepted: %v", err)
	}
}

func assertPositionCode(t *testing.T, err error, code string) {
	t.Helper()
	var rejection *PositionValidationError
	if !errors.As(err, &rejection) || rejection.Code != code {
		t.Fatalf("got %v, want %s", err, code)
	}
}

func TestPostgresPositionConcurrentLifecycle(t *testing.T) {
	for _, action := range []string{"stop", "leave", "close"} {
		t.Run(action, func(t *testing.T) {
			pool := positionTestPool(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			repo := NewPostgresRepository(pool)
			service := NewService(repo, 10)
			created, err := service.CreateRoute(ctx, CreateRouteInput{Name: "Lifecycle", ClientID: "owner", DisplayName: "Owner", TransportMode: "walking", SharingPolicy: "everyone_can_share"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = service.StartSharing(ctx, created.Route.Code, created.MemberToken); err != nil {
				t.Fatal(err)
			}
			sample := PositionUpdateInput{Latitude: 46, Longitude: 14, AccuracyM: floatPointer(10), ClientRecordedAt: timePointer(time.Now())}
			start := make(chan struct{})
			recorded := make(chan error, 1)
			mutated := make(chan error, 1)
			go func() { <-start; _, err := service.RecordPosition(ctx, created.MemberToken, sample); recorded <- err }()
			go func() {
				<-start
				var err error
				switch action {
				case "stop":
					_, err = repo.StopTrackingMember(ctx, created.Route.ID, created.Owner.ID)
				case "leave":
					_, err = repo.LeaveMember(ctx, created.Owner.ID)
				case "close":
					status := RouteStatusClosed
					_, err = repo.UpdateRoute(ctx, created.Route.ID, UpdateRouteRepoParams{Status: &status})
				}
				mutated <- err
			}()
			close(start)
			if err = <-mutated; err != nil {
				t.Fatal(err)
			}
			err = <-recorded
			if err != nil && !errors.Is(err, ErrInvalidInput) && !errors.Is(err, ErrRouteClosed) && !errors.Is(err, ErrUnauthorized) {
				t.Fatal(err)
			}
			var open int
			if err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM path_segments WHERE ended_at IS NULL").Scan(&open); err != nil {
				t.Fatal(err)
			}
			if open != 0 {
				t.Fatal("lifecycle mutation left segment open")
			}
			if _, err = service.RecordPosition(ctx, created.MemberToken, sample); err == nil {
				t.Fatal("position accepted after lifecycle mutation")
			}
		})
	}
}
