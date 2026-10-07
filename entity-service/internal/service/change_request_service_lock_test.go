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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The customer requirements lock and the planned window, seen from the
// ServiceNow dual-write: PATCH is PostgreSQL-first and the mirror write runs only
// after PostgreSQL committed, so a refusal (the lock's, or a malformed date) must
// return before anything is dispatched; and what is mirrored carries the planned
// window in the layout ServiceNow takes.

// A refusal from the repository -- every message the lock produces -- is passed
// back unchanged and writes nothing to ServiceNow: the mirror is never called and
// no writeback failure is recorded for it.
func TestChangeRequestService_PatchChangeRequest_RefusalIsNeverMirrored(t *testing.T) {
	project, no := "3bbbbbbb-0000-0000-0000-000000000012", false
	assess, newState := domain.ChangeRequestStateAssess, domain.ChangeRequestStateNew
	for _, tc := range []struct {
		name string
		req  domain.PatchChangeRequestRequest
		msg  string
	}{
		{"the project after Request Approval", domain.PatchChangeRequestRequest{ProjectID: &project},
			"projectId can no longer be changed: the Customer Project is fixed once approval has been requested (current state: assess). Cancel this change request and clone it to use another project."},
		{"unticking a box", domain.PatchChangeRequestRequest{CustomerApprovalRequired: &no},
			"customerApprovalRequired can no longer be turned off: once approval has been requested a customer requirement can be added but never removed (current state: authorize). Cancel and clone to correct it."},
		{"ticking a box with no project", domain.PatchChangeRequestRequest{CustomerReviewRequired: boolPtr(true)},
			"customerReviewRequired cannot be turned on: this change request has no Customer Project, and one can no longer be set after approval was requested. Cancel and clone it with a project."},
		{"back to New", domain.PatchChangeRequestRequest{State: &newState},
			`state "new" cannot be set: a change request that has left New cannot return to it. Cancel it and clone it instead.`},
		{"Request Approval with no project", domain.PatchChangeRequestRequest{State: &assess, Comment: strPtr("riding along")},
			"approval cannot be requested: the customer's approval and/or review is required but no Customer Project is set, so there is nobody to ask. Select a Customer Project first (or clear the requirement)."},
		{"Request Approval with nobody to ask", domain.PatchChangeRequestRequest{State: &assess},
			"customer approval and customer review are required but nobody on this project can be asked (no registered contact other than the requester): register a contact for the project first"},
		{"ticking a box with nobody to ask", domain.PatchChangeRequestRequest{CustomerReviewRequired: boolPtr(true)},
			"customer review is required but nobody on this project can be asked (no registered contact other than the requester): register a contact for the project first"},
		{"a malformed date", domain.PatchChangeRequestRequest{PlannedStartOn: strPtr("tomorrow")},
			"plannedStartOn must be a valid date-time, either RFC 3339 (2030-03-01T09:00:00Z) or YYYY-MM-DD HH:MM:SS in UTC, in the years 2000 to 2100"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubChangeRequestRepo{
				patchChangeRequest: func(context.Context, string, domain.PatchChangeRequestRequest, string) (domain.ChangeRequest, error) {
					return domain.ChangeRequest{}, &apierror.ValidationError{Msg: tc.msg}
				},
			}
			mirror := &stubMirrorChangeRequestService{
				patchChangeRequest: func(context.Context, string, domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
					t.Error("a refused PATCH was mirrored to ServiceNow")
					return domain.PatchChangeRequestResponse{}, nil
				},
			}
			failures := &recordingSNWritebackFailures{}
			svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{}, mirror, NewSNWritebackDispatcher(failures))
			ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
			_, err := svc.PatchChangeRequest(ctx, testUUID, tc.req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) || ve.Msg != tc.msg {
				t.Fatalf("err = %v, want the repository's 400 %q", err, tc.msg)
			}
			time.Sleep(150 * time.Millisecond) // the dispatch is asynchronous; give a wrongly-fired one time to show
			if got := failures.count(); got != 0 {
				t.Errorf("writeback failures = %d, want 0", got)
			}
		})
	}
}
