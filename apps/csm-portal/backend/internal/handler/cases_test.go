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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// upstreamErrorCases is the table used by every handler that calls mapUpstreamError.
// It covers all four explicit apierror mappings plus an unmapped code and a plain error.
type upstreamErrorCase struct {
	name         string
	err          error
	wantCode     int
	wantMsg      string
}

func upstreamErrors(fallback string) []upstreamErrorCase {
	return []upstreamErrorCase{
		{"apierror 401", &apierror.Error{StatusCode: http.StatusUnauthorized}, http.StatusUnauthorized, ErrMsgUnauthorized},
		{"apierror 403", &apierror.Error{StatusCode: http.StatusForbidden}, http.StatusForbidden, ErrMsgForbidden},
		{"apierror 404", &apierror.Error{StatusCode: http.StatusNotFound}, http.StatusNotFound, ErrMsgNotFound},
		{"apierror 400", &apierror.Error{StatusCode: http.StatusBadRequest}, http.StatusBadRequest, ErrMsgBadRequest},
		{"apierror unmapped (418)", &apierror.Error{StatusCode: http.StatusTeapot}, http.StatusInternalServerError, fallback},
		{"non-apierror error", errors.New("upstream connection refused"), http.StatusInternalServerError, fallback},
	}
}

// ----- CreateCase -----

func TestCreateCase(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.CreateCase(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.CreateCase(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 201 with response", func(t *testing.T) {
		const reqPayload = `{"title":"Fix login bug","priority":"high"}`
		var capturedBody []byte
		client := &mockEntityCaseClient{
			createCaseFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"id":"case-1","title":"Fix login bug"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.CreateCase(w, r)

		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["id"] != "case-1" {
			t.Errorf("response id = %v, want case-1", resp["id"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to create case.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					createCaseFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.CreateCase(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

// ----- SearchCases -----

func TestSearchCases(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
	})

	t.Run("forwards body to upstream and returns 200 with response", func(t *testing.T) {
		const reqPayload = `{"status":"open","limit":20}`
		var capturedBody []byte
		client := &mockEntityCaseClient{
			searchCasesFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"cases":[{"id":"case-1"}],"total":1}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["total"] != float64(1) {
			t.Errorf("response total = %v, want 1", resp["total"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to search cases.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					searchCasesFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.SearchCases(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

// ----- GetCase -----

func TestGetCase(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodGet, "/cases/case-1", nil)
		r.SetPathValue("id", "case-1")
		w := httptest.NewRecorder()
		h.GetCase(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		// PathValue("id") returns "" when not set — exercises the explicit guard.
		r := withUser(httptest.NewRequest(http.MethodGet, "/cases/", nil))
		w := httptest.NewRecorder()
		h.GetCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, "Case ID cannot be empty!")
		assertContentType(t, w, "application/json")
	})

	t.Run("passes case ID to upstream and returns 200 with response", func(t *testing.T) {
		var capturedID string
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, caseID string) ([]byte, error) {
				capturedID = caseID
				return []byte(`{"id":"case-42","title":"Deployment failure"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/cases/case-42", nil))
		r.SetPathValue("id", "case-42")
		w := httptest.NewRecorder()
		h.GetCase(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedID != "case-42" {
			t.Errorf("upstream received caseID %q, want %q", capturedID, "case-42")
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["id"] != "case-42" {
			t.Errorf("response id = %v, want case-42", resp["id"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to retrieve case details.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/cases/case-1", nil))
				r.SetPathValue("id", "case-1")
				w := httptest.NewRecorder()
				h.GetCase(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
