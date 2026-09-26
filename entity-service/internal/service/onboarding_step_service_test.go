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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type recordingStepRepo struct {
	fakeStepRepo
	searchReq domain.SearchOnboardingStepsRequest
}

func (r *recordingStepRepo) Search(_ context.Context, req domain.SearchOnboardingStepsRequest) ([]domain.OnboardingStep, int, error) {
	r.searchReq = req
	return nil, 0, nil
}

func validStepReq() domain.UpsertOnboardingStepRequest {
	return domain.UpsertOnboardingStepRequest{
		MembershipSfID:  " a0e1 ",
		Step:            "identity",
		Status:          "succeeded",
		EventType:       "CREATED",
		EventModifiedOn: time.Date(2026, 9, 18, 6, 37, 7, 0, time.UTC),
		Email:           " Jane@Acme.com ",
	}
}

func TestOnboardingStepService_UpsertNormalizes(t *testing.T) {
	repo := &recordingStepRepo{}
	svc := NewOnboardingStepService(repo, alwaysUnrestrictedAccess{})
	req := validStepReq()
	stale := "boom"
	req.LastError = &stale
	req.ProjectID = sampleStr(" ")
	if _, err := svc.Upsert(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	got := repo.upserts[0]
	if got.MembershipSfID != "a0e1" || got.Step != domain.OnboardingStepIdentity || got.Status != domain.OnboardingStepSucceeded ||
		got.Email != "jane@acme.com" || got.UpdatedBy != onboardingStepDefaultActor {
		t.Errorf("normalized = %+v", got)
	}
	if got.LastError != nil {
		t.Error("lastError must be cleared on a non-FAILED write")
	}
	if got.ProjectID != nil {
		t.Error("blank projectId must become nil")
	}

	req = validStepReq()
	req.Status = "FAILED"
	long := strings.Repeat("x", 1500)
	req.LastError = &long
	if _, err := svc.Upsert(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if got := repo.upserts[1]; got.LastError == nil || len(*got.LastError) != maxOnboardingStepErrorChars {
		t.Errorf("lastError should be kept and truncated on FAILED: %v", got.LastError)
	}

	// Multi-byte text is cut on a rune boundary, never mid-character.
	req = validStepReq()
	req.Status = "FAILED"
	wide := strings.Repeat("é", 1200)
	req.LastError = &wide
	if _, err := svc.Upsert(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if got := repo.upserts[2]; got.LastError == nil || !utf8.ValidString(*got.LastError) ||
		utf8.RuneCountInString(*got.LastError) != maxOnboardingStepErrorChars {
		t.Errorf("lastError must be truncated by runes: valid=%v runes=%d",
			got.LastError != nil && utf8.ValidString(*got.LastError), utf8.RuneCountInString(derefString(got.LastError)))
	}
}

// TestOnboardingStepService_RejectsExternalCallers pins that every method is
// gated on an Unrestricted AccessScope: a portal user (project-scoped) gets
// 403 before the repository is touched.
func TestOnboardingStepService_RejectsExternalCallers(t *testing.T) {
	repo := &recordingStepRepo{}
	svc := NewOnboardingStepService(repo, stubAccess{scope: AccessScope{ProjectIDs: []string{"2f1e8d6a-3b4c-4d5e-8f90-123456789abc"}}})
	ctx := context.Background()

	_, upsertErr := svc.Upsert(ctx, validStepReq())
	_, getErr := svc.GetByMembership(ctx, "a0e1")
	_, searchErr := svc.Search(ctx, domain.SearchOnboardingStepsRequest{})
	for name, err := range map[string]error{"Upsert": upsertErr, "GetByMembership": getErr, "Search": searchErr} {
		var fe *apierror.ForbiddenError
		if !errors.As(err, &fe) {
			t.Errorf("%s: err = %v, want ForbiddenError", name, err)
		}
	}
	if len(repo.upserts) != 0 || repo.searchReq.Pagination.Limit != 0 {
		t.Error("repository must not be reached for an external caller")
	}

	// An AccessService failure is surfaced, not swallowed into a 403.
	svc = NewOnboardingStepService(repo, stubAccess{err: errors.New("idp down")})
	if _, err := svc.GetByMembership(ctx, "a0e1"); err == nil || err.Error() != "idp down" {
		t.Errorf("err = %v, want the AccessService error", err)
	}
}

func TestOnboardingStepService_UpsertValidation(t *testing.T) {
	svc := NewOnboardingStepService(&recordingStepRepo{}, alwaysUnrestrictedAccess{})
	cases := map[string]func(*domain.UpsertOnboardingStepRequest){
		"missing membership": func(r *domain.UpsertOnboardingStepRequest) { r.MembershipSfID = "" },
		"bad step":           func(r *domain.UpsertOnboardingStepRequest) { r.Step = "PAYMENT" },
		"bad status":         func(r *domain.UpsertOnboardingStepRequest) { r.Status = "DONE" },
		"missing eventType":  func(r *domain.UpsertOnboardingStepRequest) { r.EventType = "" },
		"eventType too long": func(r *domain.UpsertOnboardingStepRequest) {
			r.EventType = strings.Repeat("x", maxOnboardingStepEventTypeLen+1)
		},
		"zero eventModified": func(r *domain.UpsertOnboardingStepRequest) { r.EventModifiedOn = time.Time{} },
		"missing email":      func(r *domain.UpsertOnboardingStepRequest) { r.Email = "" },
		"bad projectId":      func(r *domain.UpsertOnboardingStepRequest) { r.ProjectID = sampleStr("not-a-uuid") },
		"bad projectContact": func(r *domain.UpsertOnboardingStepRequest) { r.ProjectContactID = sampleStr("123") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			req := validStepReq()
			mutate(&req)
			_, err := svc.Upsert(context.Background(), req)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Errorf("err = %v, want ValidationError", err)
			}
		})
	}
}

func TestOnboardingStepService_GetByMembership(t *testing.T) {
	repo := &recordingStepRepo{}
	svc := NewOnboardingStepService(repo, alwaysUnrestrictedAccess{})
	resp, err := svc.GetByMembership(context.Background(), "a0e1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Steps == nil || len(resp.Steps) != 0 {
		t.Errorf("unknown membership must yield an empty (non-nil) list: %#v", resp.Steps)
	}
	if _, err := svc.GetByMembership(context.Background(), "  "); err == nil {
		t.Error("blank id must be a ValidationError")
	}
}

func TestOnboardingStepService_Search(t *testing.T) {
	repo := &recordingStepRepo{}
	svc := NewOnboardingStepService(repo, alwaysUnrestrictedAccess{})
	resp, err := svc.Search(context.Background(), domain.SearchOnboardingStepsRequest{
		Filters: domain.OnboardingStepFilters{
			ProjectID:       sampleStr("2f1e8d6a-3b4c-4d5e-8f90-123456789abc"),
			MembershipSfIDs: []string{" a0e1 ", "", "a0e2"},
			Statuses:        []domain.OnboardingStepStatus{"failed"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Limit != defaultLimit || resp.Offset != 0 || resp.Steps == nil {
		t.Errorf("resp = %+v", resp)
	}
	if !reflect.DeepEqual(repo.searchReq.Filters.MembershipSfIDs, []string{"a0e1", "a0e2"}) ||
		!reflect.DeepEqual(repo.searchReq.Filters.Statuses, []domain.OnboardingStepStatus{domain.OnboardingStepFailed}) {
		t.Errorf("filters not normalized: %+v", repo.searchReq.Filters)
	}

	var ve *apierror.ValidationError
	if _, err := svc.Search(context.Background(), domain.SearchOnboardingStepsRequest{
		Filters: domain.OnboardingStepFilters{Statuses: []domain.OnboardingStepStatus{"PENDING"}},
	}); !errors.As(err, &ve) {
		t.Errorf("unknown status: err = %v", err)
	}
	if _, err := svc.Search(context.Background(), domain.SearchOnboardingStepsRequest{
		Filters: domain.OnboardingStepFilters{ProjectID: sampleStr("nope")},
	}); !errors.As(err, &ve) {
		t.Errorf("bad projectId: err = %v", err)
	}
	if _, err := svc.Search(context.Background(), domain.SearchOnboardingStepsRequest{
		Pagination: domain.Pagination{Limit: maxLimit + 1},
	}); !errors.As(err, &ve) {
		t.Errorf("limit over cap: err = %v", err)
	}
}

// TestOnboardingStepService_AcceptsNotificationEventType pins the value
// csm-notification-service actually sends for its IDENTITY and EMAIL steps.
// It is longer than the column's original VARCHAR(20), which is what made
// every one of its ledger writes fail.
func TestOnboardingStepService_AcceptsNotificationEventType(t *testing.T) {
	repo := &recordingStepRepo{}
	svc := NewOnboardingStepService(repo, alwaysUnrestrictedAccess{})
	req := validStepReq()
	req.EventType = "project_contact.invited"

	if _, err := svc.Upsert(context.Background(), req); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
}
