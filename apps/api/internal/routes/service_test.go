package routes

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubRepository struct {
	createMemberFn               func(context.Context, CreateMemberRepoParams) (CreateMemberRepoResult, error)
	createRouteFn                func(context.Context, CreateRouteRepoParams) (CreateRouteRepoResult, error)
	countMembersByRouteIDFn      func(context.Context, string) (int, error)
	countTrackingMembersFn       func(context.Context, string) (int, error)
	getAuthorizedMemberByTokenFn func(context.Context, string) (AuthorizedMember, error)
	getAuthorizedOwnerByTokenFn  func(context.Context, string) (AuthorizedMember, error)
	getMembersByRouteIDFn        func(context.Context, string) ([]Member, error)
	getPathSegmentsByRouteIDFn   func(context.Context, string) (map[string][]PathSegment, error)
	getRouteByCodeFn             func(context.Context, string) (Route, string, error)
	startTrackingMemberFn        func(context.Context, string, string) (StartSharingRepoResult, error)
	stopTrackingMemberFn         func(context.Context, string, string) (Member, error)
	recordPositionFn             func(context.Context, RecordPositionRepoParams) (PositionUpdateResult, error)
	updateRouteFn                func(context.Context, string, UpdateRouteRepoParams) (Route, error)
	leaveMemberFn                func(context.Context, string) (Member, error)
	deleteRouteFn                func(context.Context, string) error
}

func (s stubRepository) CreateRoute(ctx context.Context, params CreateRouteRepoParams) (CreateRouteRepoResult, error) {
	return s.createRouteFn(ctx, params)
}

func (s stubRepository) CreateMember(ctx context.Context, params CreateMemberRepoParams) (CreateMemberRepoResult, error) {
	return s.createMemberFn(ctx, params)
}

func (s stubRepository) GetRouteByCode(ctx context.Context, code string) (Route, string, error) {
	return s.getRouteByCodeFn(ctx, code)
}

func (s stubRepository) GetAuthorizedMemberByTokenHash(ctx context.Context, tokenHash string) (AuthorizedMember, error) {
	return s.getAuthorizedMemberByTokenFn(ctx, tokenHash)
}

func (s stubRepository) GetAuthorizedOwnerByTokenHash(ctx context.Context, tokenHash string) (AuthorizedMember, error) {
	return s.getAuthorizedOwnerByTokenFn(ctx, tokenHash)
}

func (s stubRepository) GetMembersByRouteID(ctx context.Context, routeID string) ([]Member, error) {
	return s.getMembersByRouteIDFn(ctx, routeID)
}

func (s stubRepository) GetPathSegmentsByRouteID(ctx context.Context, routeID string) (map[string][]PathSegment, error) {
	return s.getPathSegmentsByRouteIDFn(ctx, routeID)
}

func (s stubRepository) CountMembersByRouteID(ctx context.Context, routeID string) (int, error) {
	return s.countMembersByRouteIDFn(ctx, routeID)
}

func (s stubRepository) CountTrackingMembers(ctx context.Context, routeID string) (int, error) {
	return s.countTrackingMembersFn(ctx, routeID)
}

func (s stubRepository) StartTrackingMember(ctx context.Context, routeID, memberID string) (StartSharingRepoResult, error) {
	return s.startTrackingMemberFn(ctx, routeID, memberID)
}

func (s stubRepository) StopTrackingMember(ctx context.Context, routeID, memberID string) (Member, error) {
	return s.stopTrackingMemberFn(ctx, routeID, memberID)
}

func (s stubRepository) RecordPosition(ctx context.Context, params RecordPositionRepoParams) (PositionUpdateResult, error) {
	return s.recordPositionFn(ctx, params)
}

func (s stubRepository) UpdateRoute(ctx context.Context, routeID string, params UpdateRouteRepoParams) (Route, error) {
	return s.updateRouteFn(ctx, routeID, params)
}

func (s stubRepository) LeaveMember(ctx context.Context, memberID string) (Member, error) {
	return s.leaveMemberFn(ctx, memberID)
}

func (s stubRepository) DeleteRoute(ctx context.Context, routeID string) error {
	return s.deleteRouteFn(ctx, routeID)
}

func TestCreateRoute(t *testing.T) {
	t.Parallel()

	var captured CreateRouteRepoParams
	repo := stubRepository{
		createRouteFn: func(_ context.Context, params CreateRouteRepoParams) (CreateRouteRepoResult, error) {
			captured = params
			return CreateRouteRepoResult{
				Route: Route{
					ID:                 "route-1",
					Code:               params.Route.Code,
					Name:               params.Route.Name,
					Description:        params.Route.Description,
					HasPassword:        true,
					SharingPolicy:      params.Route.SharingPolicy,
					Status:             params.Route.Status,
					MaxTrackingMembers: params.Route.MaxTrackingMembers,
				},
				Owner: Member{
					ID:            "member-1",
					RouteID:       "route-1",
					DisplayName:   params.Owner.DisplayName,
					TransportMode: params.Owner.TransportMode,
					IsOwner:       true,
					Status:        params.Owner.Status,
					Color:         params.Owner.Color,
				},
			}, nil
		},
	}

	service := NewService(repo, 10)

	result, err := service.CreateRoute(context.Background(), CreateRouteInput{
		ClientID:      "client-1",
		DisplayName:   "Ana",
		TransportMode: "car",
		Name:          "Morning convoy",
		Description:   "Trip to work",
		Password:      "secret",
		SharingPolicy: SharingPolicyEveryoneCanShare,
	})
	if err != nil {
		t.Fatalf("CreateRoute() error = %v", err)
	}

	if result.Route.MaxTrackingMembers != 10 {
		t.Fatalf("CreateRoute() tracking limit = %d, want 10", result.Route.MaxTrackingMembers)
	}

	if captured.Route.PasswordHash == "" {
		t.Fatal("CreateRoute() password hash was empty")
	}

	if result.MemberToken == "" || result.OwnerToken == "" {
		t.Fatal("CreateRoute() tokens must not be empty")
	}
}

func TestJoinRoute(t *testing.T) {
	t.Parallel()

	route := Route{
		ID:                 "route-1",
		Code:               "K7P9QD",
		Name:               "Morning convoy",
		HasPassword:        true,
		SharingPolicy:      SharingPolicyEveryoneCanShare,
		Status:             RouteStatusActive,
		MaxTrackingMembers: 10,
	}

	repo := stubRepository{
		getRouteByCodeFn: func(_ context.Context, code string) (Route, string, error) {
			if code != "K7P9QD" {
				t.Fatalf("GetRouteByCode() code = %q, want K7P9QD", code)
			}

			hashed, err := hashPassword("secret")
			if err != nil {
				t.Fatalf("hashPassword() error = %v", err)
			}

			return route, hashed, nil
		},
		countMembersByRouteIDFn: func(_ context.Context, routeID string) (int, error) {
			if routeID != route.ID {
				t.Fatalf("CountMembersByRouteID() routeID = %q, want %q", routeID, route.ID)
			}

			return 1, nil
		},
		createMemberFn: func(_ context.Context, params CreateMemberRepoParams) (CreateMemberRepoResult, error) {
			if params.Color == "" {
				t.Fatal("CreateMember() color must not be empty")
			}

			return CreateMemberRepoResult{
				Member: Member{
					ID:            "member-2",
					RouteID:       params.RouteID,
					DisplayName:   params.DisplayName,
					TransportMode: params.TransportMode,
					Status:        params.Status,
					Color:         params.Color,
				},
			}, nil
		},
	}

	service := NewService(repo, 10)

	result, err := service.JoinRoute(context.Background(), "k7p9qd", JoinRouteInput{
		ClientID:      "client-2",
		DisplayName:   "Matej",
		TransportMode: "train",
		Password:      "secret",
	})
	if err != nil {
		t.Fatalf("JoinRoute() error = %v", err)
	}

	if result.MemberToken == "" {
		t.Fatal("JoinRoute() member token must not be empty")
	}

	if result.Member.DisplayName != "Matej" {
		t.Fatalf("JoinRoute() display name = %q, want Matej", result.Member.DisplayName)
	}
}

func TestJoinRouteRejectsWrongPassword(t *testing.T) {
	t.Parallel()

	hashed, err := hashPassword("secret")
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}

	service := NewService(stubRepository{
		getRouteByCodeFn: func(_ context.Context, _ string) (Route, string, error) {
			return Route{
				ID:          "route-1",
				Code:        "K7P9QD",
				Name:        "Protected route",
				HasPassword: true,
				Status:      RouteStatusActive,
			}, hashed, nil
		},
	}, 10)

	_, err = service.JoinRoute(context.Background(), "K7P9QD", JoinRouteInput{
		ClientID:      "client-2",
		DisplayName:   "Matej",
		TransportMode: "train",
		Password:      "wrong",
	})
	if !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("JoinRoute() error = %v, want ErrInvalidPassword", err)
	}
}

func TestSnapshot(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	authorized := AuthorizedMember{
		Route: Route{
			ID:                 "route-1",
			Code:               "K7P9QD",
			Name:               "Morning convoy",
			SharingPolicy:      SharingPolicyEveryoneCanShare,
			Status:             RouteStatusActive,
			MaxTrackingMembers: 10,
			CreatedAt:          now,
		},
		Member: Member{
			ID:            "member-1",
			RouteID:       "route-1",
			DisplayName:   "Ana",
			TransportMode: "car",
			IsOwner:       true,
			Status:        MemberStatusSpectating,
			Color:         palette[0],
			JoinedAt:      now,
		},
	}

	service := NewService(stubRepository{
		getAuthorizedMemberByTokenFn: func(_ context.Context, tokenHash string) (AuthorizedMember, error) {
			if tokenHash == "" {
				t.Fatal("GetAuthorizedMemberByTokenHash() token hash must not be empty")
			}

			return authorized, nil
		},
		getMembersByRouteIDFn: func(_ context.Context, routeID string) ([]Member, error) {
			if routeID != "route-1" {
				t.Fatalf("GetMembersByRouteID() routeID = %q, want route-1", routeID)
			}

			return []Member{
				authorized.Member,
				{
					ID:            "member-2",
					RouteID:       "route-1",
					DisplayName:   "Matej",
					TransportMode: "train",
					IsOwner:       false,
					Status:        MemberStatusTracking,
					Color:         palette[1],
					JoinedAt:      now,
				},
			}, nil
		},
		getPathSegmentsByRouteIDFn: func(_ context.Context, routeID string) (map[string][]PathSegment, error) {
			if routeID != "route-1" {
				t.Fatalf("GetPathSegmentsByRouteID() routeID = %q, want route-1", routeID)
			}

			recordedAt := now.Add(2 * time.Minute)
			return map[string][]PathSegment{
				"member-2": {
					{
						ID:        "segment-1",
						StartedAt: &now,
						Points: []RoutePoint{
							{
								Seq:        1,
								Latitude:   46.0569,
								Longitude:  14.5058,
								RecordedAt: recordedAt,
							},
						},
					},
				},
			}, nil
		},
		countMembersByRouteIDFn: func(_ context.Context, _ string) (int, error) {
			return 2, nil
		},
		countTrackingMembersFn: func(_ context.Context, _ string) (int, error) {
			return 1, nil
		},
	}, 10)

	snapshot, err := service.Snapshot(context.Background(), "K7P9QD", "raw-member-token")
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	if len(snapshot.Members) != 2 {
		t.Fatalf("Snapshot() members = %d, want 2", len(snapshot.Members))
	}

	if len(snapshot.Members[0].Paths) != 0 {
		t.Fatalf("Snapshot() owner paths = %d, want 0", len(snapshot.Members[0].Paths))
	}

	if len(snapshot.Members[1].Paths) != 1 {
		t.Fatalf("Snapshot() member paths = %d, want 1", len(snapshot.Members[1].Paths))
	}

	if got := snapshot.Members[1].Paths[0].Points[0].Latitude; got != 46.0569 {
		t.Fatalf("Snapshot() path point latitude = %f, want 46.0569", got)
	}

	if !snapshot.Viewer.CanStartSharing {
		t.Fatal("Snapshot() viewer should be allowed to start sharing")
	}

	if !snapshot.Viewer.CanCloseRoute {
		t.Fatal("Snapshot() owner should be allowed to close route")
	}
}

func TestUpdateRoute(t *testing.T) {
	t.Parallel()

	service := NewService(stubRepository{
		getAuthorizedOwnerByTokenFn: func(_ context.Context, _ string) (AuthorizedMember, error) {
			return AuthorizedMember{
				Route: Route{
					ID:            "route-1",
					Code:          "K7P9QD",
					Name:          "Old name",
					Description:   "Old description",
					Status:        RouteStatusActive,
					SharingPolicy: SharingPolicyEveryoneCanShare,
				},
				Member: Member{
					ID:      "member-1",
					IsOwner: true,
				},
			}, nil
		},
		updateRouteFn: func(_ context.Context, routeID string, params UpdateRouteRepoParams) (Route, error) {
			if routeID != "route-1" {
				t.Fatalf("UpdateRoute() routeID = %q, want route-1", routeID)
			}

			if params.Name == nil || *params.Name != "New name" {
				t.Fatal("UpdateRoute() expected updated name")
			}

			if params.Description == nil || *params.Description != "New description" {
				t.Fatal("UpdateRoute() expected updated description")
			}

			return Route{
				ID:            "route-1",
				Code:          "K7P9QD",
				Name:          "New name",
				Description:   "New description",
				Status:        RouteStatusActive,
				SharingPolicy: SharingPolicyEveryoneCanShare,
			}, nil
		},
	}, 10)

	updated, err := service.UpdateRoute(context.Background(), "K7P9QD", "owner-token", UpdateRouteInput{
		Name:        "New name",
		Description: "New description",
	})
	if err != nil {
		t.Fatalf("UpdateRoute() error = %v", err)
	}

	if updated.Name != "New name" {
		t.Fatalf("UpdateRoute() name = %q, want New name", updated.Name)
	}
}

func TestLeaveRoute(t *testing.T) {
	t.Parallel()

	service := NewService(stubRepository{
		getAuthorizedMemberByTokenFn: func(_ context.Context, _ string) (AuthorizedMember, error) {
			return AuthorizedMember{
				Route: Route{
					ID:   "route-1",
					Code: "K7P9QD",
				},
				Member: Member{
					ID:          "member-2",
					DisplayName: "Matej",
				},
			}, nil
		},
		leaveMemberFn: func(_ context.Context, memberID string) (Member, error) {
			if memberID != "member-2" {
				t.Fatalf("LeaveMember() memberID = %q, want member-2", memberID)
			}

			now := time.Now().UTC()
			return Member{
				ID:          "member-2",
				DisplayName: "Matej",
				Status:      MemberStatusLeft,
				LeftAt:      &now,
			}, nil
		},
	}, 10)

	result, err := service.LeaveRoute(context.Background(), "K7P9QD", "member-token")
	if err != nil {
		t.Fatalf("LeaveRoute() error = %v", err)
	}

	if result.Member.Status != MemberStatusLeft {
		t.Fatalf("LeaveRoute() status = %q, want %q", result.Member.Status, MemberStatusLeft)
	}
}

func TestStartSharing(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	authorized := AuthorizedMember{
		Route: Route{
			ID:                 "route-1",
			Code:               "K7P9QD",
			SharingPolicy:      SharingPolicyEveryoneCanShare,
			Status:             RouteStatusActive,
			MaxTrackingMembers: 10,
		},
		Member: Member{
			ID:      "member-2",
			RouteID: "route-1",
			Status:  MemberStatusSpectating,
		},
	}

	service := NewService(stubRepository{
		getAuthorizedMemberByTokenFn: func(_ context.Context, _ string) (AuthorizedMember, error) {
			return authorized, nil
		},
		countTrackingMembersFn: func(_ context.Context, routeID string) (int, error) {
			if routeID != "route-1" {
				t.Fatalf("CountTrackingMembers() routeID = %q, want route-1", routeID)
			}

			return 1, nil
		},
		startTrackingMemberFn: func(_ context.Context, routeID, memberID string) (StartSharingRepoResult, error) {
			if routeID != "route-1" || memberID != "member-2" {
				t.Fatalf("StartTrackingMember() got routeID=%q memberID=%q", routeID, memberID)
			}

			return StartSharingRepoResult{
				Member: Member{
					ID:      memberID,
					RouteID: routeID,
					Status:  MemberStatusTracking,
				},
				Segment: PathSegment{
					ID:        "segment-1",
					StartedAt: &now,
					Points:    []RoutePoint{},
				},
			}, nil
		},
	}, 10)

	result, err := service.StartSharing(context.Background(), "k7p9qd", "member-token")
	if err != nil {
		t.Fatalf("StartSharing() error = %v", err)
	}

	if result.Member.Status != MemberStatusTracking {
		t.Fatalf("StartSharing() status = %q, want %q", result.Member.Status, MemberStatusTracking)
	}

	if result.Segment.ID == "" {
		t.Fatal("StartSharing() segment ID must not be empty")
	}
}

func TestStartSharingRejectsTrackingLimit(t *testing.T) {
	t.Parallel()

	service := NewService(stubRepository{
		getAuthorizedMemberByTokenFn: func(_ context.Context, _ string) (AuthorizedMember, error) {
			return AuthorizedMember{
				Route: Route{
					ID:                 "route-1",
					Code:               "K7P9QD",
					SharingPolicy:      SharingPolicyEveryoneCanShare,
					Status:             RouteStatusActive,
					MaxTrackingMembers: 1,
				},
				Member: Member{
					ID:      "member-2",
					RouteID: "route-1",
					Status:  MemberStatusSpectating,
				},
			}, nil
		},
		countTrackingMembersFn: func(_ context.Context, _ string) (int, error) {
			return 1, nil
		},
	}, 10)

	_, err := service.StartSharing(context.Background(), "K7P9QD", "member-token")
	if !errors.Is(err, ErrTrackingLimitReached) {
		t.Fatalf("StartSharing() error = %v, want ErrTrackingLimitReached", err)
	}
}

func TestStartSharingRejectsRestrictedJoiner(t *testing.T) {
	t.Parallel()

	service := NewService(stubRepository{
		getAuthorizedMemberByTokenFn: func(_ context.Context, _ string) (AuthorizedMember, error) {
			return AuthorizedMember{
				Route: Route{
					ID:            "route-1",
					Code:          "K7P9QD",
					SharingPolicy: SharingPolicyJoinersViewOnly,
					Status:        RouteStatusActive,
				},
				Member: Member{
					ID:      "member-2",
					RouteID: "route-1",
					Status:  MemberStatusSpectating,
					IsOwner: false,
				},
			}, nil
		},
	}, 10)

	_, err := service.StartSharing(context.Background(), "K7P9QD", "member-token")
	if !errors.Is(err, ErrSharingNotAllowed) {
		t.Fatalf("StartSharing() error = %v, want ErrSharingNotAllowed", err)
	}
}

func TestStopSharing(t *testing.T) {
	t.Parallel()

	service := NewService(stubRepository{
		getAuthorizedMemberByTokenFn: func(_ context.Context, _ string) (AuthorizedMember, error) {
			return AuthorizedMember{
				Route: Route{
					ID:     "route-1",
					Code:   "K7P9QD",
					Status: RouteStatusActive,
				},
				Member: Member{
					ID:      "member-2",
					RouteID: "route-1",
					Status:  MemberStatusTracking,
				},
			}, nil
		},
		stopTrackingMemberFn: func(_ context.Context, routeID, memberID string) (Member, error) {
			if routeID != "route-1" || memberID != "member-2" {
				t.Fatalf("StopTrackingMember() got routeID=%q memberID=%q", routeID, memberID)
			}

			return Member{
				ID:      memberID,
				RouteID: routeID,
				Status:  MemberStatusSpectating,
			}, nil
		},
	}, 10)

	result, err := service.StopSharing(context.Background(), "K7P9QD", "member-token")
	if err != nil {
		t.Fatalf("StopSharing() error = %v", err)
	}

	if result.Member.Status != MemberStatusSpectating {
		t.Fatalf("StopSharing() status = %q, want %q", result.Member.Status, MemberStatusSpectating)
	}
}

func TestRecordPosition(t *testing.T) {
	t.Parallel()

	accuracy := 12.5
	clientRecordedAt := time.Now().UTC()
	service := NewService(stubRepository{
		getAuthorizedMemberByTokenFn: func(_ context.Context, _ string) (AuthorizedMember, error) {
			return AuthorizedMember{
				Route: Route{
					ID:     "route-1",
					Code:   "K7P9QD",
					Status: RouteStatusActive,
				},
				Member: Member{
					ID:      "member-2",
					RouteID: "route-1",
					Status:  MemberStatusTracking,
				},
			}, nil
		},
		recordPositionFn: func(_ context.Context, params RecordPositionRepoParams) (PositionUpdateResult, error) {
			if params.RouteID != "route-1" || params.MemberID != "member-2" {
				t.Fatalf("RecordPosition() got routeID=%q memberID=%q", params.RouteID, params.MemberID)
			}

			if params.Latitude != 46.0569 || params.Longitude != 14.5058 {
				t.Fatalf("RecordPosition() got coordinates=%f,%f", params.Latitude, params.Longitude)
			}

			if params.AccuracyM == nil || *params.AccuracyM != accuracy {
				t.Fatal("RecordPosition() expected accuracy")
			}

			if params.ClientRecordedAt == nil || !params.ClientRecordedAt.Equal(clientRecordedAt) {
				t.Fatal("RecordPosition() expected client recorded time")
			}

			if string(params.RawPayload) != "{}" {
				t.Fatalf("RecordPosition() raw payload = %s, want {}", string(params.RawPayload))
			}

			return PositionUpdateResult{
				RouteID:   params.RouteID,
				MemberID:  params.MemberID,
				SegmentID: "segment-1",
				Point: RoutePoint{
					Seq:              1,
					Latitude:         params.Latitude,
					Longitude:        params.Longitude,
					AccuracyM:        params.AccuracyM,
					ClientRecordedAt: params.ClientRecordedAt,
					RecordedAt:       time.Now().UTC(),
				},
			}, nil
		},
	}, 10)

	result, err := service.RecordPosition(context.Background(), "member-token", PositionUpdateInput{
		Latitude:         46.0569,
		Longitude:        14.5058,
		AccuracyM:        &accuracy,
		ClientRecordedAt: &clientRecordedAt,
	})
	if err != nil {
		t.Fatalf("RecordPosition() error = %v", err)
	}

	if result.SegmentID != "segment-1" || result.Point.Seq != 1 {
		t.Fatalf("RecordPosition() result segment/seq = %q/%d", result.SegmentID, result.Point.Seq)
	}
}

func TestRecordPositionRejectsSpectator(t *testing.T) {
	t.Parallel()

	service := NewService(stubRepository{
		getAuthorizedMemberByTokenFn: func(_ context.Context, _ string) (AuthorizedMember, error) {
			return AuthorizedMember{
				Route: Route{
					ID:     "route-1",
					Code:   "K7P9QD",
					Status: RouteStatusActive,
				},
				Member: Member{
					ID:      "member-2",
					RouteID: "route-1",
					Status:  MemberStatusSpectating,
				},
			}, nil
		},
		recordPositionFn: func(context.Context, RecordPositionRepoParams) (PositionUpdateResult, error) {
			t.Fatal("RecordPosition() repository call should not run for a spectator")
			return PositionUpdateResult{}, nil
		},
	}, 10)

	_, err := service.RecordPosition(context.Background(), "member-token", PositionUpdateInput{
		Latitude:  46.0569,
		Longitude: 14.5058,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("RecordPosition() error = %v, want ErrInvalidInput", err)
	}
}

func TestRecordPositionRejectsInvalidCoordinates(t *testing.T) {
	t.Parallel()

	service := NewService(stubRepository{
		getAuthorizedMemberByTokenFn: func(context.Context, string) (AuthorizedMember, error) {
			t.Fatal("RecordPosition() should validate coordinates before authorization")
			return AuthorizedMember{}, nil
		},
	}, 10)

	_, err := service.RecordPosition(context.Background(), "member-token", PositionUpdateInput{
		Latitude:  91,
		Longitude: 14.5058,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("RecordPosition() error = %v, want ErrInvalidInput", err)
	}
}

func TestDeleteRoute(t *testing.T) {
	t.Parallel()

	service := NewService(stubRepository{
		getAuthorizedOwnerByTokenFn: func(_ context.Context, _ string) (AuthorizedMember, error) {
			return AuthorizedMember{
				Route: Route{
					ID:   "route-1",
					Code: "K7P9QD",
				},
				Member: Member{IsOwner: true},
			}, nil
		},
		deleteRouteFn: func(_ context.Context, routeID string) error {
			if routeID != "route-1" {
				t.Fatalf("DeleteRoute() routeID = %q, want route-1", routeID)
			}

			return nil
		},
	}, 10)

	if err := service.DeleteRoute(context.Background(), "K7P9QD", "owner-token"); err != nil {
		t.Fatalf("DeleteRoute() error = %v", err)
	}
}
