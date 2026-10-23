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
)

// The deviation from the implementation plan is PostgreSQL-only: it is a PATCH
// on its own, it reaches the repository on both create paths (a clone carries
// it), it never reaches ServiceNow, and the ServiceNow data source refuses it.

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
		mirror := &stubMirrorChangeRequestService{
			patchChangeRequest: func(context.Context, string, domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
				t.Error("mirror was called for a PATCH that only carried the deviation")
				return domain.PatchChangeRequestResponse{}, nil
			},
		}
		failures := &recordingSNWritebackFailures{}
		svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{}, mirror, NewSNWritebackDispatcher(failures))
		if _, err := svc.PatchChangeRequest(ctx, testUUID, domain.PatchChangeRequestRequest{ImplementationDeviation: &deviation}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		time.Sleep(150 * time.Millisecond) // the dispatch is asynchronous; give a wrongly-fired one time to show
		if got := failures.count(); got != 0 {
			t.Errorf("writeback failures = %d, want 0", got)
		}
	})
}

// A clone is a create carrying the source's deviation: PostgreSQL stores it on
// both create paths, and the ServiceNow-first create never sends it to ServiceNow.
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

	t.Run("servicenow-first", func(t *testing.T) {
		var toServiceNow, toPostgres domain.CreateChangeRequestRequest
		mirror := &stubMirrorChangeRequestService{
			createChangeRequest: func(_ context.Context, r domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
				toServiceNow = r
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
		if toServiceNow.ImplementationDeviation != nil {
			t.Errorf("ServiceNow was sent the deviation %q", *toServiceNow.ImplementationDeviation)
		}
		if toPostgres.ImplementationDeviation == nil || *toPostgres.ImplementationDeviation != deviation {
			t.Errorf("PostgreSQL saw %v, want the clone's deviation", toPostgres.ImplementationDeviation)
		}
	})
}

// The ServiceNow data source has no field for the deviation, so it refuses it
// rather than drop it: before anything is sent.
func TestServiceNowChangeRequestService_RefusesTheDeviation(t *testing.T) {
	svc := NewServiceNowChangeRequestService(nil)
	deviation := "<p>Something else was done.</p>"
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	create := validCreateChangeRequestRequest()
	create.ImplementationDeviation = &deviation
	_, err := svc.CreateChangeRequest(ctx, create)
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || ve.Msg != implementationDeviationNotOnServiceNowMsg {
		t.Errorf("create with a deviation: err = %v, want the ServiceNow refusal", err)
	}

	_, err = svc.PatchChangeRequest(ctx, testUUID, domain.PatchChangeRequestRequest{ImplementationDeviation: &deviation})
	if !errors.As(err, &ve) || ve.Msg != implementationDeviationNotOnServiceNowMsg {
		t.Errorf("PATCH with a deviation: err = %v, want the ServiceNow refusal", err)
	}
}
