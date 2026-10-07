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

package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// TestDecodeRequestWithLimit_OversizedTrailingData reproduces a case where the
// first JSON object fits within the size cap, but trailing data after it pushes
// the body over the limit. The trailing-data check must report this as a
// size-limit error (the caller-supplied tooLargeMsg), not the generic "must
// contain a single JSON object" message — the two errors mean different things
// to the caller.
func TestDecodeRequestWithLimit_OversizedTrailingData(t *testing.T) {
	const limit = 1024

	body := `{"x":"ok"}` + strings.Repeat(" ", limit*2)
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	rec := httptest.NewRecorder()

	var dst struct {
		X string `json:"x"`
	}
	ok := decodeRequestWithLimit(rec, req, &dst, limit, attachmentTooLargeMsg)

	if ok {
		t.Fatal("expected decodeRequestWithLimit to return false for oversized trailing data")
	}
	// Prove this actually reached the trailing-data check (the second Decode)
	// rather than failing on the first one — dst must reflect a fully-decoded
	// first object.
	if dst.X != "ok" {
		t.Fatalf("dst.X = %q, want %q — the first Decode should have succeeded before the trailing-data check ran", dst.X, "ok")
	}
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), attachmentTooLargeMsg) {
		t.Errorf("response body = %q, want it to contain the size-limit message %q", rec.Body.String(), attachmentTooLargeMsg)
	}
	if strings.Contains(rec.Body.String(), "must contain a single JSON object") {
		t.Errorf("response body = %q, should not fall back to the generic trailing-data message for a size-limit case", rec.Body.String())
	}
}

// TestWriteServiceError_CarriesTheMachineReadableCode pins what a client branches
// on: a refusal that has a code (apierror/codes.go) is returned with it as the
// body's errorCode, the HTTP status and the message exactly as they were, and one
// that has none is returned without the key at all.
func TestWriteServiceError_CarriesTheMachineReadableCode(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
		wantCode   string
	}{
		{"on hold", &apierror.ConflictError{Msg: "this change request is on hold, so a new implementation time cannot be proposed now", Code: apierror.CodeChangeRequestOnHold},
			409, "this change request is on hold, so a new implementation time cannot be proposed now", "change_request_on_hold"},
		{"window changed", &apierror.ConflictError{Msg: "the planned implementation time of this change request changed after you opened it (it is now x)", Code: apierror.CodeChangeRequestScheduleChanged},
			409, "the planned implementation time of this change request changed after you opened it (it is now x)", "change_request_schedule_changed"},
		{"approval not pending", &apierror.ConflictError{Msg: "this approval is no longer pending", Code: apierror.CodeChangeRequestApprovalNotPending},
			409, "this approval is no longer pending", "change_request_approval_not_pending"},
		{"not proposable", &apierror.ConflictError{Msg: "not open to a new time", Code: apierror.CodeChangeRequestNotProposable},
			409, "not open to a new time", "change_request_not_proposable"},
		{"not asked", &apierror.ForbiddenError{Msg: "only members asked may answer", Code: apierror.CodeChangeRequestNotAsked},
			403, "only members asked may answer", "change_request_not_asked"},
		{"forbidden", &apierror.ForbiddenError{Msg: "not a contact", Code: apierror.CodeChangeRequestForbidden},
			403, "not a contact", "change_request_forbidden"},
		{"a wrapped refusal keeps its code", fmt.Errorf("answer: %w", &apierror.ConflictError{Msg: "m", Code: apierror.CodeChangeRequestOnHold}),
			409, "m", "change_request_on_hold"},
		{"a conflict with no code", &apierror.ConflictError{Msg: "some other conflict"}, 409, "some other conflict", ""},
		{"a forbidden with no code", &apierror.ForbiddenError{Msg: "some other refusal"}, 403, "some other refusal", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeServiceError(rec, httptest.NewRequest("PATCH", "/change-requests/x", nil), tc.err)
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body is not JSON: %v (%s)", err, rec.Body.String())
			}
			if body["message"] != tc.wantMsg {
				t.Errorf("message = %v, want %q", body["message"], tc.wantMsg)
			}
			if int(body["code"].(float64)) != tc.wantStatus {
				t.Errorf("code = %v, want the status %d", body["code"], tc.wantStatus)
			}
			got, has := body["errorCode"]
			if tc.wantCode == "" {
				if has {
					t.Errorf("errorCode = %v, want the key absent", got)
				}
			} else if got != tc.wantCode {
				t.Errorf("errorCode = %v, want %q", got, tc.wantCode)
			}
		})
	}
}

// TestWriteServiceError_OnlyTheRefusalsThatNameThemCarryACode: a validation, a
// not-found and an unexpected failure carry no errorCode, and an unexpected
// failure still returns only the generic message.
func TestWriteServiceError_OnlyTheRefusalsThatNameThemCarryACode(t *testing.T) {
	for name, err := range map[string]error{
		"validation": &apierror.ValidationError{Msg: "bad"},
		"not found":  &apierror.NotFoundError{Msg: "nope"},
		"unexpected": errors.New("db exploded"),
	} {
		rec := httptest.NewRecorder()
		writeServiceError(rec, httptest.NewRequest("PATCH", "/x", nil), err)
		if strings.Contains(rec.Body.String(), "errorCode") {
			t.Errorf("%s: body %s carries an errorCode", name, rec.Body.String())
		}
	}
}
