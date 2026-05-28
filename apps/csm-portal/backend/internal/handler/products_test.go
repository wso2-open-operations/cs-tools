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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchProducts(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewProductHandler(&mockEntityProductClient{})
		r := httptest.NewRequest(http.MethodPost, "/products/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchProducts(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewProductHandler(&mockEntityProductClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/products/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchProducts(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewProductHandler(&mockEntityProductClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/products/search", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchProducts(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 200 with response", func(t *testing.T) {
		const reqPayload = `{"searchQuery":"wso2","pagination":{"limit":10,"offset":0}}`
		var capturedBody []byte
		client := &mockEntityProductClient{
			searchProductsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"products":[{"id":"prod-1","name":"WSO2 API Manager"}],"total":1}`), nil
			},
		}
		h := NewProductHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/products/search", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.SearchProducts(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["total"] != float64(1) {
			t.Errorf("total = %v, want 1", resp["total"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to search products.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityProductClient{
					searchProductsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewProductHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/products/search", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.SearchProducts(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestSearchProductVersions(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewProductHandler(&mockEntityProductClient{})
		r := httptest.NewRequest(http.MethodPost, "/product/prod-1/versions/search", strings.NewReader(`{}`))
		r.SetPathValue("id", "prod-1")
		w := httptest.NewRecorder()
		h.SearchProductVersions(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects missing product id", func(t *testing.T) {
		h := NewProductHandler(&mockEntityProductClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/product//versions/search", strings.NewReader(`{}`)))
		w := httptest.NewRecorder()
		h.SearchProductVersions(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewProductHandler(&mockEntityProductClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/product/prod-1/versions/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("id", "prod-1")
		w := httptest.NewRecorder()
		h.SearchProductVersions(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewProductHandler(&mockEntityProductClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/product/prod-1/versions/search", strings.NewReader(`not-json`)))
		r.SetPathValue("id", "prod-1")
		w := httptest.NewRecorder()
		h.SearchProductVersions(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards product id and body to upstream and returns 200 with response", func(t *testing.T) {
		const reqPayload = `{"searchQuery":"4.2","pagination":{"limit":10,"offset":0}}`
		var capturedProductID string
		var capturedBody []byte
		client := &mockEntityProductClient{
			searchProductVersionsFn: func(_ context.Context, productID string, body []byte) ([]byte, error) {
				capturedProductID = productID
				capturedBody = body
				return []byte(`{"productVersions":[{"id":"pv-1","version":"4.2.0"}],"total":1}`), nil
			},
		}
		h := NewProductHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/product/prod-1/versions/search", strings.NewReader(reqPayload)))
		r.SetPathValue("id", "prod-1")
		w := httptest.NewRecorder()
		h.SearchProductVersions(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedProductID != "prod-1" {
			t.Errorf("upstream received productID %q, want %q", capturedProductID, "prod-1")
		}
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["total"] != float64(1) {
			t.Errorf("total = %v, want 1", resp["total"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to search product versions.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityProductClient{
					searchProductVersionsFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewProductHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/product/prod-1/versions/search", strings.NewReader(`{}`)))
				r.SetPathValue("id", "prod-1")
				w := httptest.NewRecorder()
				h.SearchProductVersions(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
