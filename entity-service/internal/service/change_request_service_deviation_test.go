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
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The deviation from the implementation plan is PostgreSQL-only: it is a PATCH
// on its own, it reaches the repository on both create paths (a clone carries
// it), it is never mirrored to the previous system, and the data source that
// talks to that system directly refuses it.

func TestChangeRequestService_PatchChangeRequest_DeviationAloneIsAPatch(t *testing.T) {
	deviation := "<p>Restarted the nodes one by one instead of together.</p>"
	var got domain.PatchChangeRequestRequest
	repo := &stubChangeRequestRepo{
		patchChangeRequest: func(_ context.Context, id string, r domain.PatchChangeRequestRequest, _ string) (domain.ChangeRequest, error) {
			got = r
			return domain.ChangeRequest{SearchChangeRequestView: domain.SearchChangeRequestView{ID: id}}, nil
		},
	}
	svc := NewChangeRequestService(repo, stubUserRepo{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.PatchChangeRequest(ctx, testUUID, domain.PatchChangeRequestRequest{ImplementationDeviation: &deviation}); err != nil {
		t.Fatalf("a deviation-only PATCH was refused: %v", err)
	}
	if got.ImplementationDeviation == nil || *got.ImplementationDeviation != deviation {
		t.Fatalf("repository saw %v, want the request's deviation", got.ImplementationDeviation)
	}
}

func TestChangeRequestService_PatchChangeRequest_DeviationStaysOutOfTheMirror(t *testing.T) {
	deviation := "<p>Used the standby database.</p>"
	title := "new title"
	repo := &stubChangeRequestRepo{
		patchChangeRequest: func(_ context.Context, id string, _ domain.PatchChangeRequestRequest, _ string) (domain.ChangeRequest, error) {
			return domain.ChangeRequest{SearchChangeRequestView: domain.SearchChangeRequestView{ID: id}}, nil
		},
	}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	t.Run("with other fields: the deviation is stripped, the rest mirrored", func(t *testing.T) {
		called := make(chan domain.PatchChangeRequestRequest, 1)
		mirror := &stubMirrorChangeRequestService{
			patchChangeRequest: func(_ context.Context, _ string, r domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
				called <- r
				return domain.PatchChangeRequestResponse{}, nil
			},
		}
		svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{}, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
		if _, err := svc.PatchChangeRequest(ctx, testUUID, domain.PatchChangeRequestRequest{Title: &title, ImplementationDeviation: &deviation}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		select {
		case got := <-called:
			if got.Title == nil || *got.Title != title {
				t.Errorf("mirror title = %v, want %q", got.Title, title)
			}
			if got.ImplementationDeviation != nil {
				t.Errorf("mirror saw the PostgreSQL-only deviation: %q", *got.ImplementationDeviation)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("mirror.PatchChangeRequest was never called")
		}
	})

	t.Run("alone: nothing to mirror", func(t *testing.T) {
		var mirrored []domain.PatchChangeRequestRequest
		sentinel := make(chan struct{}, 1)
		mirror := &stubMirrorChangeRequestService{
			patchChangeRequest: func(_ context.Context, _ string, r domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
				mirrored = append(mirrored, r)
				if r.Title != nil && *r.Title == crMirrorSentinel {
					sentinel <- struct{}{}
				}
				return domain.PatchChangeRequestResponse{}, nil
			},
		}
		failures := &recordingSNWritebackFailures{}
		svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{}, mirror, NewSNWritebackDispatcher(failures))
		if _, err := svc.PatchChangeRequest(ctx, testUUID, domain.PatchChangeRequestRequest{ImplementationDeviation: &deviation}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		awaitMirroredWrites(ctx, t, svc, sentinel)
		if len(mirrored) != 1 {
			t.Errorf("the mirror received %d PATCHes, want only the sentinel: %+v", len(mirrored), mirrored)
		}
		if got := failures.count(); got != 0 {
			t.Errorf("writeback failures = %d, want 0", got)
		}
	})
}

// A clone is a create carrying the source's deviation: PostgreSQL stores it on
// both create paths, and the create that calls the previous system first never
// sends it there.
func TestChangeRequestService_CreateChangeRequest_CarriesTheDeviationToPostgresOnly(t *testing.T) {
	deviation := "<p>Rolled out to one region first.</p>"
	want := validCreateChangeRequestRequest()
	want.ImplementationDeviation = &deviation
	ok := func() domain.CreateChangeRequestResponse {
		resp := domain.CreateChangeRequestResponse{Message: "ok"}
		resp.ChangeRequest.ID = testUUID
		return resp
	}

	t.Run("plain", func(t *testing.T) {
		var got domain.CreateChangeRequestRequest
		repo := &stubChangeRequestRepo{
			createChangeRequest: func(_ context.Context, r domain.CreateChangeRequestRequest, _ string) (domain.CreateChangeRequestResponse, error) {
				got = r
				return ok(), nil
			},
		}
		svc := NewChangeRequestService(repo, stubUserRepo{})
		ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
		if _, err := svc.CreateChangeRequest(ctx, want); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ImplementationDeviation == nil || *got.ImplementationDeviation != deviation {
			t.Fatalf("repository saw %v, want the clone's deviation", got.ImplementationDeviation)
		}
	})

	t.Run("previous-system-first", func(t *testing.T) {
		var toPreviousSystem, toPostgres domain.CreateChangeRequestRequest
		mirror := &stubMirrorChangeRequestService{
			createChangeRequest: func(_ context.Context, r domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
				toPreviousSystem = r
				return ok(), nil
			},
		}
		repo := &stubChangeRequestRepo{
			createChangeRequestFromServiceNow: func(_ context.Context, r domain.CreateChangeRequestRequest, _, _, _ string) (domain.CreateChangeRequestResponse, error) {
				toPostgres = r
				return ok(), nil
			},
		}
		svc := NewChangeRequestServiceWithSNMirror(repo, stubUserRepo{}, mirror)
		if _, err := svc.CreateChangeRequest(context.Background(), want); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if toPreviousSystem.ImplementationDeviation != nil {
			t.Errorf("the previous system was sent the deviation %q", *toPreviousSystem.ImplementationDeviation)
		}
		if toPostgres.ImplementationDeviation == nil || *toPostgres.ImplementationDeviation != deviation {
			t.Errorf("PostgreSQL saw %v, want the clone's deviation", toPostgres.ImplementationDeviation)
		}
	})
}

// A customer's create carrying a deviation is refused (403) before the create
// that calls the previous system first calls it: a refusal after it would leave
// a record there with no PostgreSQL row. The route is internal-only; this is
// the second line.
func TestChangeRequestService_CreateChangeRequest_RefusesACustomersDeviationBeforeThePreviousSystem(t *testing.T) {
	deviation := "<p>Rolled out to one region first.</p>"
	req := validCreateChangeRequestRequest()
	req.ImplementationDeviation = &deviation
	mirror := &stubMirrorChangeRequestService{
		createChangeRequest: func(context.Context, domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
			t.Error("the previous system was called for a customer's create carrying a deviation")
			return domain.CreateChangeRequestResponse{}, nil
		},
	}
	repo := &stubChangeRequestRepo{
		createChangeRequestFromServiceNow: func(context.Context, domain.CreateChangeRequestRequest, string, string, string) (domain.CreateChangeRequestResponse, error) {
			t.Error("PostgreSQL was written for a customer's create carrying a deviation")
			return domain.CreateChangeRequestResponse{}, nil
		},
	}
	svc := NewChangeRequestServiceWithSNMirror(repo, stubUserRepo{}, mirror)
	customer := repository.WithCallerIdentity(contextWithUserIDToken(fakeJWTWithEmail(t, "dave.mendis@example.com")),
		repository.SearchScope{ProjectIDs: []string{"proj-1"}, ViewerEmail: "dave.mendis@example.com"})

	_, err := svc.CreateChangeRequest(customer, req)
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) || fe.Code != apierror.CodeChangeRequestForbidden {
		t.Fatalf("err = %v, want the 403 change_request_forbidden", err)
	}

	// Staff, and a customer whose create carries no (or a blank) deviation, reach
	// the previous system as before.
	called := 0
	mirror.createChangeRequest = func(context.Context, domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
		called++
		resp := domain.CreateChangeRequestResponse{Message: "ok"}
		resp.ChangeRequest.ID = testUUID
		return resp, nil
	}
	repo.createChangeRequestFromServiceNow = func(context.Context, domain.CreateChangeRequestRequest, string, string, string) (domain.CreateChangeRequestResponse, error) {
		return domain.CreateChangeRequestResponse{Message: "ok"}, nil
	}
	staff := repository.WithCallerIdentity(contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@wso2.com")),
		repository.SearchScope{Unrestricted: true, ViewerEmail: "jane.doe@wso2.com"})
	if _, err := svc.CreateChangeRequest(staff, req); err != nil {
		t.Errorf("staff create with a deviation: %v", err)
	}
	blank := "<p><br></p>"
	req.ImplementationDeviation = &blank
	if _, err := svc.CreateChangeRequest(customer, req); err != nil {
		t.Errorf("customer create with a blank deviation: %v", err)
	}
	if called != 2 {
		t.Errorf("the previous system was called %d times, want 2", called)
	}
}
