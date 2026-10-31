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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// suspendRecorder is a StandardChangeService that only answers SuspendTemplate;
// any other method panics on the nil embedded interface.
type suspendRecorder struct {
	service.StandardChangeService
	called bool
	id     string
	req    domain.SuspendStandardChangeTemplateRequest
	err    error
}

func (s *suspendRecorder) SuspendTemplate(_ context.Context, id string, req domain.SuspendStandardChangeTemplateRequest) (domain.StandardChangeTemplate, error) {
	s.called, s.id, s.req = true, id, req
	if s.err != nil {
		return domain.StandardChangeTemplate{}, s.err
	}
	return domain.StandardChangeTemplate{ID: id, State: domain.StandardChangeTemplateStateSuspended}, nil
}

func TestStandardChangeHandler_SuspendTemplateBody(t *testing.T) {
	const id = "5b3f1c2e-0000-4000-8000-000000000001"
	cases := []struct {
		name       string
		body       io.Reader
		svcErr     error
		wantStatus int
		wantCalled bool
		wantReason string
		wantMsg    string
	}{
		{name: "no body", body: nil, wantStatus: http.StatusOK, wantCalled: true},
		{name: "empty body", body: strings.NewReader(""), wantStatus: http.StatusOK, wantCalled: true},
		{name: "whitespace body", body: strings.NewReader(" \n"), wantStatus: http.StatusOK, wantCalled: true},
		{name: "empty object", body: strings.NewReader(`{}`), wantStatus: http.StatusOK, wantCalled: true},
		{name: "with a reason", body: strings.NewReader(`{"reason":"Rolled back twice"}`), wantStatus: http.StatusOK, wantCalled: true, wantReason: "Rolled back twice"},
		{name: "unknown field", body: strings.NewReader(`{"why":"x"}`), wantStatus: http.StatusBadRequest, wantMsg: "unknown field"},
		{name: "malformed", body: strings.NewReader(`{"reason":`), wantStatus: http.StatusBadRequest},
		{name: "wrong type", body: strings.NewReader(`{"reason":5}`), wantStatus: http.StatusBadRequest, wantMsg: "expected string"},
		{name: "two objects", body: strings.NewReader(`{} {}`), wantStatus: http.StatusBadRequest, wantMsg: "single JSON object"},
		{name: "too large", body: strings.NewReader(`{"reason":"` + strings.Repeat("x", int(maxRequestBodySize)) + `"}`), wantStatus: http.StatusBadRequest, wantMsg: "too large"},
		{name: "refused by the service", body: strings.NewReader(`{"reason":"x"}`), svcErr: &apierror.ValidationError{Msg: "reason must be at most 1000 characters"},
			wantStatus: http.StatusBadRequest, wantCalled: true, wantReason: "x", wantMsg: "at most 1000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &suspendRecorder{err: tc.svcErr}
			h := NewStandardChangeHandler(svc)
			req := httptest.NewRequest(http.MethodPost, "/standard-change-templates/"+id+"/suspend", tc.body)
			req.SetPathValue("id", id)
			rec := httptest.NewRecorder()
			h.SuspendTemplate(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if svc.called != tc.wantCalled {
				t.Fatalf("service called = %v, want %v", svc.called, tc.wantCalled)
			}
			if tc.wantCalled && (svc.id != id || svc.req.Reason != tc.wantReason) {
				t.Fatalf("service got id %q reason %q, want %q %q", svc.id, svc.req.Reason, id, tc.wantReason)
			}
			if tc.wantMsg != "" && !strings.Contains(rec.Body.String(), tc.wantMsg) {
				t.Fatalf("body %s does not contain %q", rec.Body.String(), tc.wantMsg)
			}
		})
	}
}
