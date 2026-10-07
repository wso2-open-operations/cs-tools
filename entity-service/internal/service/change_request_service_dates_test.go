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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The planned window on the ServiceNow dual-write: what is mirrored carries the
// window in the layout ServiceNow takes, and the ServiceNow-first create validates
// it before ServiceNow is called.

// What is mirrored after an accepted PATCH: projectId (a resend after New, the
// only one that can arrive) is forwarded as before, the Postgres-only boxes are
// stripped, and the planned window is written in ServiceNow's layout -- an RFC
// 3339 value, which PostgreSQL takes and ServiceNow's service refuses, would
// otherwise commit in PostgreSQL and then fail every mirror write.
func TestChangeRequestService_PatchChangeRequest_MirrorGetsTheWindowInServiceNowsLayout(t *testing.T) {
	project := "3bbbbbbb-0000-0000-0000-000000000011"
	yes := true
	repo := &stubChangeRequestRepo{
		patchChangeRequest: func(_ context.Context, id string, _ domain.PatchChangeRequestRequest, _ string) (domain.ChangeRequest, error) {
			return domain.ChangeRequest{SearchChangeRequestView: domain.SearchChangeRequestView{ID: id}}, nil
		},
	}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	for _, tc := range []struct {
		name, start, end, wantStart, wantEnd string
	}{
		{"RFC 3339 with an offset", "2030-03-01T14:30:00+05:30", "2030-03-01T16:30:00+05:30", "2030-03-01 09:00:00", "2030-03-01 11:00:00"},
		{"RFC 3339 in UTC", "2030-03-01T09:00:00Z", "2030-03-01T11:00:00Z", "2030-03-01 09:00:00", "2030-03-01 11:00:00"},
		{"ServiceNow's own layout is unchanged", "2030-03-01 09:00:00", "2030-03-01 11:00:00", "2030-03-01 09:00:00", "2030-03-01 11:00:00"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			called := make(chan domain.PatchChangeRequestRequest, 1)
			mirror := &stubMirrorChangeRequestService{
				patchChangeRequest: func(_ context.Context, _ string, r domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
					called <- r
					return domain.PatchChangeRequestResponse{}, nil
				},
			}
			svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{}, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
			req := domain.PatchChangeRequestRequest{PlannedStartOn: &tc.start, PlannedEndOn: &tc.end, ProjectID: &project, CustomerApprovalRequired: &yes}
			if _, err := svc.PatchChangeRequest(ctx, testUUID, req); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			select {
			case got := <-called:
				if got.PlannedStartOn == nil || *got.PlannedStartOn != tc.wantStart || got.PlannedEndOn == nil || *got.PlannedEndOn != tc.wantEnd {
					t.Errorf("mirrored window = %v .. %v, want %s .. %s", got.PlannedStartOn, got.PlannedEndOn, tc.wantStart, tc.wantEnd)
				}
				if got.ProjectID == nil || *got.ProjectID != project {
					t.Errorf("mirrored projectId = %v, want it forwarded", got.ProjectID)
				}
				if got.CustomerApprovalRequired != nil {
					t.Error("the Postgres-only box was mirrored")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("mirror.PatchChangeRequest was never called")
			}
			// What the caller sent is not rewritten under them.
			if req.PlannedStartOn == nil || *req.PlannedStartOn != tc.start {
				t.Errorf("the request's own planned start was rewritten to %v", req.PlannedStartOn)
			}
		})
	}
}

// The ServiceNow-first create validates the planned window BEFORE ServiceNow is
// called -- a refusal after it would leave a ServiceNow record with no PostgreSQL
// row -- as the plain Postgres create does, so 'tomorrow' / 'infinity' / a year
// in the thousands never reach either; and what ServiceNow is given is the
// original, validated text.
func TestChangeRequestService_CreateChangeRequest_SNFirstValidatesTheWindowBeforeServiceNow(t *testing.T) {
	mirror := &stubMirrorChangeRequestService{createChangeRequest: func(context.Context, domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
		t.Fatal("ServiceNow was called although the planned window is invalid")
		return domain.CreateChangeRequestResponse{}, nil
	}}
	repo := &stubChangeRequestRepo{createChangeRequestFromServiceNow: func(context.Context, domain.CreateChangeRequestRequest, string, string, string) (domain.CreateChangeRequestResponse, error) {
		t.Fatal("PostgreSQL was written although the planned window is invalid")
		return domain.CreateChangeRequestResponse{}, nil
	}}
	svc := NewChangeRequestServiceWithSNMirror(repo, stubUserRepo{}, mirror)
	for _, hostile := range []string{"tomorrow", "now", "infinity", "-infinity", "epoch", "", "   ", "2030-03-01", "99999-01-01 00:00:00", "next friday",
		"2030-03-01 09:00:00'; DROP TABLE change_request; --", "1999-12-31 23:59:59"} {
		for field, mod := range map[string]func(*domain.CreateChangeRequestRequest){
			"plannedStartDate": func(r *domain.CreateChangeRequestRequest) { r.PlannedStartDate = &hostile },
			"plannedEndDate":   func(r *domain.CreateChangeRequestRequest) { r.PlannedEndDate = &hostile },
		} {
			req := validCreateChangeRequestRequest()
			mod(&req)
			_, err := svc.CreateChangeRequest(context.Background(), req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) || !strings.HasPrefix(ve.Msg, field+" must be a valid date-time") {
				t.Fatalf("%s = %q: err = %v, want a 400 naming the field", field, hostile, err)
			}
		}
	}

	// A valid window passes: ServiceNow gets it as sent, PostgreSQL gets the request.
	start, end := "2030-03-01 09:00:00", "2030-03-01 11:00:00"
	var toSN, toPG domain.CreateChangeRequestRequest
	mirror.createChangeRequest = func(_ context.Context, r domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
		toSN = r
		resp := domain.CreateChangeRequestResponse{}
		resp.ChangeRequest.ID = testUUID
		return resp, nil
	}
	repo.createChangeRequestFromServiceNow = func(_ context.Context, r domain.CreateChangeRequestRequest, id, _, _ string) (domain.CreateChangeRequestResponse, error) {
		toPG = r
		resp := domain.CreateChangeRequestResponse{}
		resp.ChangeRequest.ID = id
		return resp, nil
	}
	req := validCreateChangeRequestRequest()
	req.PlannedStartDate, req.PlannedEndDate = &start, &end
	if _, err := svc.CreateChangeRequest(context.Background(), req); err != nil {
		t.Fatalf("a valid window: %v", err)
	}
	if toSN.PlannedStartDate == nil || *toSN.PlannedStartDate != start || toSN.PlannedEndDate == nil || *toSN.PlannedEndDate != end {
		t.Errorf("ServiceNow got the window %v .. %v, want it as sent", toSN.PlannedStartDate, toSN.PlannedEndDate)
	}
	if toPG.PlannedStartDate == nil || toPG.PlannedEndDate == nil {
		t.Errorf("PostgreSQL did not get the window: %+v", toPG)
	}
}
