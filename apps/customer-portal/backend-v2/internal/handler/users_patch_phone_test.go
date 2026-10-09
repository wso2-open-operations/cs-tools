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
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

type patchRecorder struct {
	calls []string
	reqs  []entity.PatchUserMeRequest
	err   error
}

type patchEntityClient struct {
	fakeFirstAccessUserClient
	rec *patchRecorder
}

func (f *patchEntityClient) PatchMe(_ context.Context, req entity.PatchUserMeRequest) (entity.PatchUserMeResponse, error) {
	f.rec.calls = append(f.rec.calls, "entity")
	f.rec.reqs = append(f.rec.reqs, req)
	return entity.PatchUserMeResponse{}, f.rec.err
}

type patchSCIMClient struct {
	noopSCIMUserClient
	rec    *patchRecorder
	result *string
	err    error
}

func (f *patchSCIMClient) UpdateUserPhone(context.Context, string, string) (*string, error) {
	f.rec.calls = append(f.rec.calls, "scim")
	return f.result, f.err
}

func sp(s string) *string { return &s }

func doPatchMe(h *UserHandler, body string) *httptest.ResponseRecorder {
	r := getMeRequest()
	r2 := httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(body)).WithContext(r.Context())
	r2.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.PatchMe(w, r2)
	return w
}

func newPatchHandler(rec *patchRecorder, scimResult *string, scimErr error) *UserHandler {
	return NewUserHandler(&patchEntityClient{rec: rec}, &patchSCIMClient{rec: rec, result: scimResult, err: scimErr}, false)
}

func TestPatchMe_PhoneGoesToSCIMThenEntity(t *testing.T) {
	rec := &patchRecorder{}
	// SCIM stores a normalized value; entity must get that, not the request.
	h := newPatchHandler(rec, sp("+15555550123"), nil)

	w := doPatchMe(h, `{"phoneNumber":"+1 555 555 0123"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body)
	}
	if got := strings.Join(rec.calls, ","); got != "scim,entity" {
		t.Fatalf("call order = %s", got)
	}
	req := rec.reqs[0]
	if req.Phone == nil || *req.Phone != "+15555550123" || req.TimeZone != nil {
		t.Fatalf("entity req = %+v", req)
	}
	if !strings.Contains(w.Body.String(), "+15555550123") {
		t.Fatalf("response lost phoneNumber: %s", w.Body)
	}
}

func TestPatchMe_SCIMReturnsNoneFallsBackToRequested(t *testing.T) {
	rec := &patchRecorder{}
	h := newPatchHandler(rec, nil, nil)

	w := doPatchMe(h, `{"phoneNumber":"+15555550123"}`)

	if w.Code != http.StatusOK || len(rec.reqs) != 1 || *rec.reqs[0].Phone != "+15555550123" {
		t.Fatalf("status %d reqs %+v", w.Code, rec.reqs)
	}
}

func TestPatchMe_SCIMFailureSkipsEntity(t *testing.T) {
	rec := &patchRecorder{}
	h := newPatchHandler(rec, nil, errors.New("scim down"))

	w := doPatchMe(h, `{"phoneNumber":"+15555550123"}`)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", w.Code)
	}
	if got := strings.Join(rec.calls, ","); got != "scim" {
		t.Fatalf("calls = %s, entity must not be called", got)
	}
}

func TestPatchMe_EntityFailureReturnsError(t *testing.T) {
	rec := &patchRecorder{err: &apierror.Error{StatusCode: http.StatusServiceUnavailable}}
	h := newPatchHandler(rec, sp("+15555550123"), nil)

	w := doPatchMe(h, `{"phoneNumber":"+15555550123"}`)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "Failed to update profile") {
		t.Fatalf("body = %s", w.Body)
	}
}

func TestPatchMe_PhoneAndTimeZoneInOneEntityCall(t *testing.T) {
	rec := &patchRecorder{}
	h := newPatchHandler(rec, sp("+15555550123"), nil)

	w := doPatchMe(h, `{"phoneNumber":"+15555550123","timeZone":"Asia/Colombo"}`)

	if w.Code != http.StatusOK || len(rec.reqs) != 1 {
		t.Fatalf("status %d, entity calls %d", w.Code, len(rec.reqs))
	}
	req := rec.reqs[0]
	if *req.Phone != "+15555550123" || req.TimeZone == nil || *req.TimeZone != "Asia/Colombo" {
		t.Fatalf("entity req = %+v", req)
	}
}

func TestPatchMe_TimeZoneOnlyDoesNotCallSCIM(t *testing.T) {
	rec := &patchRecorder{}
	h := newPatchHandler(rec, nil, nil)

	w := doPatchMe(h, `{"timeZone":"Asia/Colombo"}`)

	if w.Code != http.StatusOK || strings.Join(rec.calls, ",") != "entity" || rec.reqs[0].Phone != nil {
		t.Fatalf("status %d calls %v reqs %+v", w.Code, rec.calls, rec.reqs)
	}
}

func TestPatchMe_EmptyPhoneClears(t *testing.T) {
	rec := &patchRecorder{}
	h := newPatchHandler(rec, nil, nil)

	w := doPatchMe(h, `{"phoneNumber":""}`)

	if w.Code != http.StatusOK || len(rec.reqs) != 1 {
		t.Fatalf("status %d reqs %+v", w.Code, rec.reqs)
	}
	if rec.reqs[0].Phone == nil || *rec.reqs[0].Phone != "" {
		t.Fatalf("empty phone must be sent as empty string, got %+v", rec.reqs[0])
	}
}

func TestPatchMe_EmptyTimeZoneRejectedBeforeAnyUpstreamCall(t *testing.T) {
	for name, body := range map[string]string{
		"empty with phone": `{"phoneNumber":"+15555550123","timeZone":""}`,
		"empty alone":      `{"timeZone":""}`,
		"whitespace only":  `{"phoneNumber":"+15555550123","timeZone":"   "}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := &patchRecorder{}
			h := newPatchHandler(rec, sp("+15555550123"), nil)

			w := doPatchMe(h, body)

			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "timeZone must not be empty.") {
				t.Fatalf("status %d body %s", w.Code, w.Body)
			}
			if len(rec.calls) != 0 {
				t.Fatalf("upstream calls = %v, want none", rec.calls)
			}
		})
	}
}
