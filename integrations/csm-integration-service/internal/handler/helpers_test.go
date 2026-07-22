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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-integration-service/internal/apierror"
)

// ----- assertion helpers -----

// assertStatus fails if the recorded status code differs from want.
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Errorf("status = %d, want %d; body: %s", w.Code, want, w.Body.String())
	}
}

// assertContentType fails if the Content-Type header differs from want.
func assertContentType(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	if ct := w.Header().Get("Content-Type"); ct != want {
		t.Errorf("Content-Type = %q, want %q", ct, want)
	}
}

// assertErrorMessage decodes {"message":"..."} and checks the message field.
func assertErrorMessage(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v; raw: %s", err, w.Body.String())
	}
	if body.Message != want {
		t.Errorf("message = %q, want %q", body.Message, want)
	}
}

// decodeJSON decodes the recorder body into T and returns it.
func decodeJSON[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(w.Body).Decode(&v); err != nil {
		t.Fatalf("decode response body: %v; raw: %s", err, w.Body.String())
	}
	return v
}

// upstreamErrorCase is the table used by every handler that calls mapUpstreamError.
// It covers all four explicit apierror mappings plus an unmapped code and a plain error.
type upstreamErrorCase struct {
	name     string
	err      error
	wantCode int
	wantMsg  string
}

func upstreamErrors(fallback string) []upstreamErrorCase {
	return []upstreamErrorCase{
		{"apierror 401", &apierror.Error{StatusCode: http.StatusUnauthorized}, http.StatusUnauthorized, ErrMsgUnauthorized},
		{"apierror 403", &apierror.Error{StatusCode: http.StatusForbidden}, http.StatusForbidden, ErrMsgForbidden},
		{"apierror 404", &apierror.Error{StatusCode: http.StatusNotFound}, http.StatusNotFound, ErrMsgNotFound},
		{"apierror 400", &apierror.Error{StatusCode: http.StatusBadRequest}, http.StatusBadRequest, ErrMsgBadRequest},
		{"apierror 409", &apierror.Error{StatusCode: http.StatusConflict, Body: "conflict upstream message"}, http.StatusConflict, fallback},
		{"apierror 422", &apierror.Error{StatusCode: http.StatusUnprocessableEntity, Body: "invalid state"}, http.StatusUnprocessableEntity, fallback},
		{"apierror 502", &apierror.Error{StatusCode: http.StatusBadGateway}, http.StatusServiceUnavailable, fallback},
		{"apierror 503", &apierror.Error{StatusCode: http.StatusServiceUnavailable}, http.StatusServiceUnavailable, fallback},
		{"apierror 504", &apierror.Error{StatusCode: http.StatusGatewayTimeout}, http.StatusServiceUnavailable, fallback},
		{"apierror unmapped (418)", &apierror.Error{StatusCode: http.StatusTeapot}, http.StatusInternalServerError, fallback},
		{"non-apierror error", errors.New("upstream connection refused"), http.StatusInternalServerError, fallback},
	}
}

// ----- mock entity account client -----

type mockEntityAccountClient struct {
	getAccountFn            func(ctx context.Context, id string) ([]byte, error)
	searchAccountsFn        func(ctx context.Context, body []byte) ([]byte, error)
	searchAccountContactsFn func(ctx context.Context, accountID string, body []byte) ([]byte, error)
}

func (m *mockEntityAccountClient) GetAccount(ctx context.Context, id string) ([]byte, error) {
	if m.getAccountFn != nil {
		return m.getAccountFn(ctx, id)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityAccountClient) SearchAccounts(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchAccountsFn != nil {
		return m.searchAccountsFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityAccountClient) SearchAccountContacts(ctx context.Context, accountID string, body []byte) ([]byte, error) {
	if m.searchAccountContactsFn != nil {
		return m.searchAccountContactsFn(ctx, accountID, body)
	}
	return []byte(`{}`), nil
}

// ----- mock entity project client -----

type mockEntityProjectClient struct {
	getProjectFn            func(ctx context.Context, id string) ([]byte, error)
	searchProjectsFn        func(ctx context.Context, body []byte) ([]byte, error)
	searchProjectContactsFn func(ctx context.Context, projectID string, body []byte) ([]byte, error)
}

func (m *mockEntityProjectClient) GetProject(ctx context.Context, id string) ([]byte, error) {
	if m.getProjectFn != nil {
		return m.getProjectFn(ctx, id)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityProjectClient) SearchProjects(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchProjectsFn != nil {
		return m.searchProjectsFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityProjectClient) SearchProjectContacts(ctx context.Context, projectID string, body []byte) ([]byte, error) {
	if m.searchProjectContactsFn != nil {
		return m.searchProjectContactsFn(ctx, projectID, body)
	}
	return []byte(`{}`), nil
}
