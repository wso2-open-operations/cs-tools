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

	"github.com/wso2-open-operations/cs-tools/operations/legacy-product-api/internal/apierror"
)

// decodeJSON decodes a recorded response body into T, failing the test on error.
func decodeJSON[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(w.Body).Decode(&v); err != nil {
		t.Fatalf("decode response body: %v; raw: %s", err, w.Body.String())
	}
	return v
}

// ----- mock entity product API client -----

type mockEntityProductAPIClient struct {
	versionsFn     func(ctx context.Context, feed string) ([]byte, error)
	releaseLinesFn func(ctx context.Context, includeRetired bool) ([]byte, error)
	carbonFn       func(ctx context.Context) ([]byte, error)
	kernelsFn      func(ctx context.Context) ([]byte, error)
}

func (m *mockEntityProductAPIClient) ListProductVersions(ctx context.Context, feed string) ([]byte, error) {
	if m.versionsFn != nil {
		return m.versionsFn(ctx, feed)
	}
	return []byte(`{"productVersions":[]}`), nil
}

func (m *mockEntityProductAPIClient) ListReleaseLines(ctx context.Context, includeRetired bool) ([]byte, error) {
	if m.releaseLinesFn != nil {
		return m.releaseLinesFn(ctx, includeRetired)
	}
	return []byte(`{"releaseLines":[]}`), nil
}

func (m *mockEntityProductAPIClient) ListCarbonPlatformHistory(ctx context.Context) ([]byte, error) {
	if m.carbonFn != nil {
		return m.carbonFn(ctx)
	}
	return []byte(`{"items":[]}`), nil
}

func (m *mockEntityProductAPIClient) ListCarbonKernelReleases(ctx context.Context) ([]byte, error) {
	if m.kernelsFn != nil {
		return m.kernelsFn(ctx)
	}
	return []byte(`{"items":[]}`), nil
}

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

// upstreamErrorCase is one way an entity-service call can fail. wantCode is
// 503 for an unavailable upstream and 500 for everything else, the only two
// failures legacy ever answered with.
type upstreamErrorCase struct {
	name     string
	err      error
	wantCode int
}

func upstreamErrors(string) []upstreamErrorCase {
	return []upstreamErrorCase{
		{"apierror 401", &apierror.Error{StatusCode: http.StatusUnauthorized}, http.StatusInternalServerError},
		{"apierror 403", &apierror.Error{StatusCode: http.StatusForbidden}, http.StatusInternalServerError},
		{"apierror 404", &apierror.Error{StatusCode: http.StatusNotFound}, http.StatusInternalServerError},
		{"apierror 400", &apierror.Error{StatusCode: http.StatusBadRequest, Body: `{"message":"bad"}`}, http.StatusInternalServerError},
		{"apierror 502", &apierror.Error{StatusCode: http.StatusBadGateway}, http.StatusServiceUnavailable},
		{"apierror 503", &apierror.Error{StatusCode: http.StatusServiceUnavailable}, http.StatusServiceUnavailable},
		{"apierror 504", &apierror.Error{StatusCode: http.StatusGatewayTimeout}, http.StatusServiceUnavailable},
		{"apierror unmapped (418)", &apierror.Error{StatusCode: http.StatusTeapot}, http.StatusInternalServerError},
		{"non-apierror error", errors.New("upstream connection refused"), http.StatusInternalServerError},
	}
}
