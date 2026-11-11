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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// An unresolved caller reaching the scoped repository must be refused with the
// same 403 body the access service gives, never a 500 and never an unrestricted
// fallback; every other error maps exactly as before.
func TestWriteServiceError_NoCallerIdentity(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{"bare", repository.ErrNoCallerIdentity, http.StatusForbidden, "no access for this user"},
		{"wrapped", fmt.Errorf("search change requests: %w", repository.ErrNoCallerIdentity), http.StatusForbidden, "no access for this user"},
		{"access service forbidden", &apierror.ForbiddenError{Msg: "no access for this user"}, http.StatusForbidden, "no access for this user"},
		{"unauthorized stays 401", &apierror.UnauthorizedError{Msg: "a verified user token (x-user-id-token) is required"}, http.StatusUnauthorized, "a verified user token (x-user-id-token) is required"},
		{"validation stays 400", &apierror.ValidationError{Msg: "bad input"}, http.StatusBadRequest, "bad input"},
		{"not found stays 404", &apierror.NotFoundError{Msg: "missing"}, http.StatusNotFound, "missing"},
		{"unknown stays 500", errors.New("boom"), http.StatusInternalServerError, "internal server error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeServiceError(rec, httptest.NewRequest(http.MethodPost, "/x", nil), tc.err)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			var body apierror.ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body %q: %v", rec.Body.String(), err)
			}
			if body.Code != tc.wantStatus || body.Message != tc.wantMsg || body.ErrorCode != "" {
				t.Errorf("body = %+v, want code %d message %q and no errorCode", body, tc.wantStatus, tc.wantMsg)
			}
		})
	}
}

// Real handlers, services and scoped repositories over a request that carries
// no caller identity. The pool is nil on purpose: Scoped refuses before it
// touches the pool, so any query reaching it would panic and fail the test.
func TestNoCallerIdentity_ScopedEndpoints(t *testing.T) {
	scoped := repository.NewScoped(nil)

	mux := http.NewServeMux()
	crHandler := NewChangeRequestHandler(service.NewChangeRequestService(repository.NewChangeRequestRepository(scoped), nil))
	mux.HandleFunc("POST /change-requests/search", crHandler.SearchChangeRequests)
	depHandler := NewDeploymentHandler(service.NewDeploymentService(repository.NewDeploymentRepository(scoped)))
	mux.HandleFunc("POST /deployments/search", depHandler.SearchDeployments)
	tcHandler := NewTimeCardHandler(service.NewTimeCardService(repository.NewTimeCardRepository(scoped), nil))
	mux.HandleFunc("POST /time-cards/search", tcHandler.SearchTimeCards)
	attHandler := NewStoredAttachmentHandler(service.NewStoredAttachmentService(
		repository.NewStoredAttachmentRepository(scoped), service.StoredAttachmentLimits{MaxBytes: 1 << 20}))
	mux.HandleFunc("POST /attachments/upload", attHandler.InitiateUpload)
	srv := middleware.UserIDToken(mux)

	payload, _ := json.Marshal(map[string]string{"email": "jane.doe@example.com"})
	token := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".sig"

	cases := []struct {
		path       string
		body       string
		wantStatus int
		wantMsg    string
	}{
		{"/change-requests/search", `{}`, http.StatusForbidden, "no access for this user"},
		{"/deployments/search", `{}`, http.StatusForbidden, "no access for this user"},
		{"/time-cards/search", `{}`, http.StatusForbidden, "no access for this user"},
		// Unchanged: the attachment repository already maps this to a 401.
		{"/attachments/upload",
			`{"type":"work_item","parentId":"00000000-0000-0000-0000-000000000001","name":"a.txt","contentType":"text/plain","sizeBytes":10}`,
			http.StatusUnauthorized, "a verified user token (x-user-id-token) is required"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("x-user-id-token", token)
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			var body apierror.ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body %q: %v", rec.Body.String(), err)
			}
			if body.Message != tc.wantMsg {
				t.Errorf("message = %q, want %q", body.Message, tc.wantMsg)
			}
		})
	}
}
