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
	getRouteByCodeFn             func(context.Context, string) (Route, string, error)
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

func (s stubRepository) CountMembersByRouteID(ctx context.Context, routeID string) (int, error) {
	return s.countMembersByRouteIDFn(ctx, routeID)
}

func (s stubRepository) CountTrackingMembers(ctx context.Context, routeID string) (int, error) {
	return s.countTrackingMembersFn(ctx, routeID)
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
