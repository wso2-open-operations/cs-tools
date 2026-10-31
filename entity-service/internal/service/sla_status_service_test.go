// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

type stubSLAStatusRepo struct {
	search     func(ctx context.Context, p domain.Pagination, source string) ([]domain.SLAStatus, int, error)
	clockState func(ctx context.Context, workItemID, target, source string) (domain.SLAClockState, error)
}

func (s stubSLAStatusRepo) SearchActiveSLAStatuses(ctx context.Context, p domain.Pagination, source string) ([]domain.SLAStatus, int, error) {
	return s.search(ctx, p, source)
}

func (s stubSLAStatusRepo) GetClockState(ctx context.Context, workItemID, target, source string) (domain.SLAClockState, error) {
	return s.clockState(ctx, workItemID, target, source)
}

// restrictedAccess is an AccessService stub whose scope is never Unrestricted
// -- an authenticated-but-non-internal caller (e.g. a project_contact's own
// x-user-id-token), as opposed to no caller identity at all.
type restrictedAccess struct{}

func (restrictedAccess) ResolveScope(context.Context) (AccessScope, error) {
	return AccessScope{ProjectIDs: []string{"proj-1"}}, nil
}

// erroringAccess is an AccessService stub whose ResolveScope always fails --
// e.g. no verified identity on the request at all (see AccessService.
// ResolveScope's own doc comment for the unverified-identity case).
type erroringAccess struct{ err error }

func (e erroringAccess) ResolveScope(context.Context) (AccessScope, error) {
	return AccessScope{}, e.err
}

// TestSLAStatusService_SearchActiveSLAStatuses_RequiresInternalCaller is the
// regression guard for a real finding: this endpoint returns every active
// case's SLA data (case number, title, product, severity) in one bulk list
// with no per-project/per-case filtering of its own, so unlike every other
// Postgres-backed read, there is no scope short of "internal service" that's
// safe to hand this out under -- a caller with no token at all, or a
// correctly-authenticated but non-internal one, must both be refused before
// the repository is ever reached.
func TestSLAStatusService_SearchActiveSLAStatuses_RequiresInternalCaller(t *testing.T) {
	repo := stubSLAStatusRepo{search: func(context.Context, domain.Pagination, string) ([]domain.SLAStatus, int, error) {
		t.Fatal("repository must not be reached for a non-internal caller")
		return nil, 0, nil
	}}

	t.Run("no verified identity on the request is refused", func(t *testing.T) {
		_, err := NewSLAStatusService(repo, erroringAccess{err: &apierror.ServiceUnavailableError{Msg: "no verified identity"}}, nil, nil).
			SearchActiveSLAStatuses(context.Background(), domain.Pagination{}, "", false)
		if err == nil {
			t.Fatal("err = nil, want the AccessService error propagated")
		}
	})

	t.Run("an authenticated but non-internal caller is refused", func(t *testing.T) {
		_, err := NewSLAStatusService(repo, restrictedAccess{}, nil, nil).SearchActiveSLAStatuses(context.Background(), domain.Pagination{}, "", false)
		var fe *apierror.ForbiddenError
		if !errors.As(err, &fe) {
			t.Fatalf("err = %v, want *apierror.ForbiddenError", err)
		}
	})
}

// TestSLAStatusService_SearchActiveSLAStatuses proves the pagination default
// and cap are this endpoint's own (500/2000), not the generic 20/50 every
// other search uses — this is a machine-polling endpoint with one real
// caller, not a UI list.
func TestSLAStatusService_SearchActiveSLAStatuses(t *testing.T) {
	t.Run("no limit defaults to 500", func(t *testing.T) {
		var got domain.Pagination
		repo := stubSLAStatusRepo{search: func(_ context.Context, p domain.Pagination, _ string) ([]domain.SLAStatus, int, error) {
			got = p
			return nil, 0, nil
		}}
		if _, err := NewSLAStatusService(repo, alwaysUnrestrictedAccess{}, nil, nil).SearchActiveSLAStatuses(context.Background(), domain.Pagination{}, "", false); err != nil {
			t.Fatal(err)
		}
		if got.Limit != defaultSLAStatusLimit {
			t.Errorf("limit = %d, want %d", got.Limit, defaultSLAStatusLimit)
		}
	})

	t.Run("limit above 2000 is rejected before reaching the repository", func(t *testing.T) {
		repo := stubSLAStatusRepo{search: func(context.Context, domain.Pagination, string) ([]domain.SLAStatus, int, error) {
			t.Fatal("repository must not be reached for an invalid limit")
			return nil, 0, nil
		}}
		_, err := NewSLAStatusService(repo, alwaysUnrestrictedAccess{}, nil, nil).SearchActiveSLAStatuses(context.Background(), domain.Pagination{Limit: 2001}, "", false)
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("err = %v, want *apierror.ValidationError", err)
		}
	})

	t.Run("source=csm is normalized to the real enum label and reaches the repository", func(t *testing.T) {
		var gotSource string
		repo := stubSLAStatusRepo{search: func(_ context.Context, _ domain.Pagination, source string) ([]domain.SLAStatus, int, error) {
			gotSource = source
			return nil, 0, nil
		}}
		if _, err := NewSLAStatusService(repo, alwaysUnrestrictedAccess{}, nil, nil).SearchActiveSLAStatuses(context.Background(), domain.Pagination{}, "CSM", false); err != nil {
			t.Fatal(err)
		}
		if gotSource != "CSM" {
			t.Errorf("source reaching repository = %q, want %q", gotSource, "CSM")
		}
	})

	t.Run("an unrecognized source is rejected before reaching the repository", func(t *testing.T) {
		repo := stubSLAStatusRepo{search: func(context.Context, domain.Pagination, string) ([]domain.SLAStatus, int, error) {
			t.Fatal("repository must not be reached for an invalid source")
			return nil, 0, nil
		}}
		_, err := NewSLAStatusService(repo, alwaysUnrestrictedAccess{}, nil, nil).SearchActiveSLAStatuses(context.Background(), domain.Pagination{}, "bogus", false)
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("err = %v, want *apierror.ValidationError", err)
		}
	})

	t.Run("result is passed through with the normalized pagination echoed back", func(t *testing.T) {
		want := []domain.SLAStatus{{CaseID: "c1", ClockType: "response"}}
		repo := stubSLAStatusRepo{search: func(_ context.Context, p domain.Pagination, _ string) ([]domain.SLAStatus, int, error) {
			return want, 7, nil
		}}
		resp, err := NewSLAStatusService(repo, alwaysUnrestrictedAccess{}, nil, nil).SearchActiveSLAStatuses(context.Background(), domain.Pagination{Limit: 10, Offset: 20}, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Total != 7 || resp.Limit != 10 || resp.Offset != 20 || len(resp.Statuses) != 1 || resp.Statuses[0].CaseID != "c1" {
			t.Errorf("resp = %+v", resp)
		}
	})
}

const slaClockStateTestWorkItemID = "11111111-1111-1111-1111-111111111111"

// TestSLAStatusService_GetClockState_RequiresInternalCaller mirrors
// SearchActiveSLAStatuses's own scoping requirement -- see that test's own
// doc comment for why. GetClockState has the identical "no narrower scope
// to fall back to" shape.
func TestSLAStatusService_GetClockState_RequiresInternalCaller(t *testing.T) {
	repo := stubSLAStatusRepo{clockState: func(context.Context, string, string, string) (domain.SLAClockState, error) {
		t.Fatal("repository must not be reached for a non-internal caller")
		return domain.SLAClockState{}, nil
	}}
	_, err := NewSLAStatusService(repo, restrictedAccess{}, nil, nil).GetClockState(context.Background(), slaClockStateTestWorkItemID, "response", "")
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("err = %v, want *apierror.ForbiddenError", err)
	}
}

func TestSLAStatusService_GetClockState(t *testing.T) {
	t.Run("a malformed workItemId is rejected before reaching the repository", func(t *testing.T) {
		repo := stubSLAStatusRepo{clockState: func(context.Context, string, string, string) (domain.SLAClockState, error) {
			t.Fatal("repository must not be reached for an invalid workItemId")
			return domain.SLAClockState{}, nil
		}}
		_, err := NewSLAStatusService(repo, alwaysUnrestrictedAccess{}, nil, nil).GetClockState(context.Background(), "not-a-uuid", "response", "")
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("err = %v, want *apierror.ValidationError", err)
		}
	})

	t.Run("an unrecognized target is rejected before reaching the repository", func(t *testing.T) {
		repo := stubSLAStatusRepo{clockState: func(context.Context, string, string, string) (domain.SLAClockState, error) {
			t.Fatal("repository must not be reached for an invalid target")
			return domain.SLAClockState{}, nil
		}}
		_, err := NewSLAStatusService(repo, alwaysUnrestrictedAccess{}, nil, nil).GetClockState(context.Background(), slaClockStateTestWorkItemID, "bogus", "")
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("err = %v, want *apierror.ValidationError", err)
		}
	})

	t.Run("an empty target is rejected -- unlike source, it has no 'no filter' meaning here", func(t *testing.T) {
		repo := stubSLAStatusRepo{clockState: func(context.Context, string, string, string) (domain.SLAClockState, error) {
			t.Fatal("repository must not be reached for an empty target")
			return domain.SLAClockState{}, nil
		}}
		_, err := NewSLAStatusService(repo, alwaysUnrestrictedAccess{}, nil, nil).GetClockState(context.Background(), slaClockStateTestWorkItemID, "", "")
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("err = %v, want *apierror.ValidationError", err)
		}
	})

	t.Run("an unrecognized source is rejected before reaching the repository", func(t *testing.T) {
		repo := stubSLAStatusRepo{clockState: func(context.Context, string, string, string) (domain.SLAClockState, error) {
			t.Fatal("repository must not be reached for an invalid source")
			return domain.SLAClockState{}, nil
		}}
		_, err := NewSLAStatusService(repo, alwaysUnrestrictedAccess{}, nil, nil).GetClockState(context.Background(), slaClockStateTestWorkItemID, "response", "bogus")
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("err = %v, want *apierror.ValidationError", err)
		}
	})

	t.Run("target is normalized to the real enum label and reaches the repository", func(t *testing.T) {
		var gotTarget string
		repo := stubSLAStatusRepo{clockState: func(_ context.Context, _, target, _ string) (domain.SLAClockState, error) {
			gotTarget = target
			return domain.SLAClockState{}, nil
		}}
		if _, err := NewSLAStatusService(repo, alwaysUnrestrictedAccess{}, nil, nil).GetClockState(context.Background(), slaClockStateTestWorkItemID, "Response", ""); err != nil {
			t.Fatal(err)
		}
		if gotTarget != "RESPONSE" {
			t.Errorf("target reaching repository = %q, want %q", gotTarget, "RESPONSE")
		}
	})

	t.Run("result is passed through unchanged", func(t *testing.T) {
		want := domain.SLAClockState{Found: true, IsActive: true, Stage: "IN_PROGRESS"}
		repo := stubSLAStatusRepo{clockState: func(context.Context, string, string, string) (domain.SLAClockState, error) {
			return want, nil
		}}
		got, err := NewSLAStatusService(repo, alwaysUnrestrictedAccess{}, nil, nil).GetClockState(context.Background(), slaClockStateTestWorkItemID, "response", "csm")
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("got = %+v, want %+v", got, want)
		}
	})
}

// TestSLAStatusService_SearchActiveSLAStatuses_AdminTrueRequiresAdminRole
// confirms admin=true refuses a caller with no admin role, even one that
// would pass the plain internal-caller check admin=false uses.
func TestSLAStatusService_SearchActiveSLAStatuses_AdminTrueRequiresAdminRole(t *testing.T) {
	repo := stubSLAStatusRepo{search: func(context.Context, domain.Pagination, string) ([]domain.SLAStatus, int, error) {
		t.Fatal("repository must not be reached for a non-admin caller")
		return nil, 0, nil
	}}
	svc := NewSLAStatusService(repo, alwaysUnrestrictedAccess{}, []string{"cs_admin"}, nil)

	ctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "eng@wso2.com", Roles: []string{"cs_engineer"}})
	_, err := svc.SearchActiveSLAStatuses(ctx, domain.Pagination{}, "", true)
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("err = %v, want *apierror.ForbiddenError", err)
	}
}

// TestSLAStatusService_SearchActiveSLAStatuses_AdminTrueSucceedsForAdmin
// confirms a genuine admin reaches the same repository call admin=false
// makes.
func TestSLAStatusService_SearchActiveSLAStatuses_AdminTrueSucceedsForAdmin(t *testing.T) {
	want := []domain.SLAStatus{{CaseID: "case-1"}}
	repo := stubSLAStatusRepo{search: func(context.Context, domain.Pagination, string) ([]domain.SLAStatus, int, error) {
		return want, 1, nil
	}}
	svc := NewSLAStatusService(repo, alwaysUnrestrictedAccess{}, []string{"cs_admin"}, nil)

	got, err := svc.SearchActiveSLAStatuses(bannerAdminCtx(), domain.Pagination{}, "csm", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Statuses) != 1 || got.Statuses[0].CaseID != "case-1" {
		t.Errorf("got = %+v, want one status for case-1", got)
	}
}

// TestSLAStatusService_SearchActiveSLAStatuses_AdminFalseUnaffectedByAdminRoles
// is the regression guard for the whole reason admin became a query
// parameter on the existing endpoint rather than a second route: an
// ordinary internal caller (no admin role at all) making the plain,
// admin=false request -- exactly what csm-notification-service's own
// startup reconciliation does -- must see byte-identical behavior to
// before this parameter existed.
func TestSLAStatusService_SearchActiveSLAStatuses_AdminFalseUnaffectedByAdminRoles(t *testing.T) {
	want := []domain.SLAStatus{{CaseID: "case-1"}}
	repo := stubSLAStatusRepo{search: func(context.Context, domain.Pagination, string) ([]domain.SLAStatus, int, error) {
		return want, 1, nil
	}}
	svc := NewSLAStatusService(repo, alwaysUnrestrictedAccess{}, []string{"cs_admin"}, nil)

	got, err := svc.SearchActiveSLAStatuses(context.Background(), domain.Pagination{}, "", false)
	if err != nil {
		t.Fatalf("admin=false caller with no admin role was refused: %v", err)
	}
	if len(got.Statuses) != 1 || got.Statuses[0].CaseID != "case-1" {
		t.Errorf("got = %+v, want one status for case-1", got)
	}
}

// TestSLAStatusService_RequestRecalculation_RequiresAdmin confirms a
// non-admin caller is refused before the publisher is ever reached.
func TestSLAStatusService_RequestRecalculation_RequiresAdmin(t *testing.T) {
	pub := &mockEventPublisher{}
	svc := NewSLAStatusService(stubSLAStatusRepo{}, alwaysUnrestrictedAccess{}, []string{"cs_admin"}, pub)

	ctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "eng@wso2.com", Roles: []string{"cs_engineer"}})
	err := svc.RequestRecalculation(ctx)
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("err = %v, want *apierror.ForbiddenError", err)
	}
	if len(pub.calls) != 0 {
		t.Error("publisher was called for a non-admin caller")
	}
}

// TestSLAStatusService_RequestRecalculation_NoPublisherConfigured confirms
// a deployment with Event Hub publishing off fails loudly rather than
// silently doing nothing -- the one thing this whole action exists to do.
func TestSLAStatusService_RequestRecalculation_NoPublisherConfigured(t *testing.T) {
	svc := NewSLAStatusService(stubSLAStatusRepo{}, alwaysUnrestrictedAccess{}, []string{"cs_admin"}, nil)

	err := svc.RequestRecalculation(bannerAdminCtx())
	var unavailable *apierror.ServiceUnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("err = %v, want *apierror.ServiceUnavailableError", err)
	}
}

// TestSLAStatusService_RequestRecalculation_PublishesWithRequesterEmail
// confirms a genuine admin's request publishes
// TypeSLARecalculationRequested, carrying the caller's own email for the
// consumer's log line.
func TestSLAStatusService_RequestRecalculation_PublishesWithRequesterEmail(t *testing.T) {
	pub := &mockEventPublisher{}
	svc := NewSLAStatusService(stubSLAStatusRepo{}, alwaysUnrestrictedAccess{}, []string{"cs_admin"}, pub)

	if err := svc.RequestRecalculation(bannerAdminCtx()); err != nil {
		t.Fatal(err)
	}
	call, ok := findPublishCall(pub.calls, events.TypeSLARecalculationRequested)
	if !ok {
		t.Fatal("TypeSLARecalculationRequested was not published")
	}
	var payload events.SLARecalculationRequestedPayload
	if err := json.Unmarshal(call.payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.RequestedByEmail != "admin@wso2.com" {
		t.Errorf("RequestedByEmail = %q, want %q", payload.RequestedByEmail, "admin@wso2.com")
	}
	if payload.RequestedAt == "" {
		t.Error("RequestedAt is empty")
	}
}

// TestSLAStatusService_RequestRecalculation_PublishFailurePropagates
// confirms a publish failure is returned to the caller rather than
// swallowed -- unlike every best-effort publishCaseCreatedEvent-style call
// site, this admin action's whole point is telling the caller whether the
// request actually went out.
func TestSLAStatusService_RequestRecalculation_PublishFailurePropagates(t *testing.T) {
	pub := &mockEventPublisher{err: errors.New("event hub unreachable")}
	svc := NewSLAStatusService(stubSLAStatusRepo{}, alwaysUnrestrictedAccess{}, []string{"cs_admin"}, pub)

	if err := svc.RequestRecalculation(bannerAdminCtx()); err == nil {
		t.Fatal("err = nil, want the publisher's error propagated")
	}
}
