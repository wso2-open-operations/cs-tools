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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/updates"
)

// ----- GetRecommendedUpdateLevels -----

func TestGetRecommendedUpdateLevels(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewUpdatesHandler(&mockUpdatesClient{})
		r := httptest.NewRequest(http.MethodGet, "/updates/recommended-update-levels", nil)
		w := httptest.NewRecorder()
		h.GetRecommendedUpdateLevels(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("passes JWT email to upstream and returns 200 with levels", func(t *testing.T) {
		var capturedEmail string
		client := &mockUpdatesClient{
			recommendedFn: func(_ context.Context, email string) ([]updates.RecommendedUpdateLevel, error) {
				capturedEmail = email
				return []updates.RecommendedUpdateLevel{
					{ProductName: "wso2am", ProductBaseVersion: "4.3.0", RecommendedUpdateLevel: 10},
				}, nil
			},
		}
		h := NewUpdatesHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/updates/recommended-update-levels", nil))
		w := httptest.NewRecorder()
		h.GetRecommendedUpdateLevels(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedEmail != testUser.Email {
			t.Errorf("upstream called with email %q, want %q", capturedEmail, testUser.Email)
		}
		resp := decodeJSON[[]updates.RecommendedUpdateLevel](t, w)
		if len(resp) != 1 || resp[0].ProductName != "wso2am" {
			t.Errorf("unexpected response: %+v", resp)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to get recommended update levels.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockUpdatesClient{
					recommendedFn: func(_ context.Context, _ string) ([]updates.RecommendedUpdateLevel, error) {
						return nil, tc.err
					},
				}
				h := NewUpdatesHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/updates/recommended-update-levels", nil))
				w := httptest.NewRecorder()
				h.GetRecommendedUpdateLevels(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

// ----- GetProductUpdateLevels -----

func TestGetProductUpdateLevels(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewUpdatesHandler(&mockUpdatesClient{})
		r := httptest.NewRequest(http.MethodGet, "/updates/product-update-levels", nil)
		w := httptest.NewRecorder()
		h.GetProductUpdateLevels(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("returns 200 with product update levels", func(t *testing.T) {
		client := &mockUpdatesClient{
			productFn: func(_ context.Context) ([]updates.ProductUpdateLevel, error) {
				return []updates.ProductUpdateLevel{
					{
						ProductName: "wso2am",
						ProductUpdateLevels: []updates.UpdateLevel{
							{ProductBaseVersion: "4.3.0", Channel: "full", UpdateLevels: []int{1, 2, 3}},
						},
					},
				}, nil
			},
		}
		h := NewUpdatesHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/updates/product-update-levels", nil))
		w := httptest.NewRecorder()
		h.GetProductUpdateLevels(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		resp := decodeJSON[[]updates.ProductUpdateLevel](t, w)
		if len(resp) != 1 || resp[0].ProductName != "wso2am" {
			t.Errorf("unexpected response: %+v", resp)
		}
		if len(resp[0].ProductUpdateLevels) != 1 || len(resp[0].ProductUpdateLevels[0].UpdateLevels) != 3 {
			t.Errorf("unexpected update levels: %+v", resp[0].ProductUpdateLevels)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to get product update levels.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockUpdatesClient{
					productFn: func(_ context.Context) ([]updates.ProductUpdateLevel, error) {
						return nil, tc.err
					},
				}
				h := NewUpdatesHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/updates/product-update-levels", nil))
				w := httptest.NewRecorder()
				h.GetProductUpdateLevels(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

// ----- SearchUpdatesBetweenUpdateLevels -----

func TestSearchUpdatesBetweenUpdateLevels(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewUpdatesHandler(&mockUpdatesClient{})
		r := httptest.NewRequest(http.MethodPost, "/updates/levels/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchUpdatesBetweenUpdateLevels(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewUpdatesHandler(&mockUpdatesClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/updates/levels/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchUpdatesBetweenUpdateLevels(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewUpdatesHandler(&mockUpdatesClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/updates/levels/search", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchUpdatesBetweenUpdateLevels(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("parses payload and passes email from JWT to upstream", func(t *testing.T) {
		var capturedPayload updates.SearchPayload
		var capturedEmail string
		client := &mockUpdatesClient{
			searchFn: func(_ context.Context, payload updates.SearchPayload, email string) (map[string]updates.UpdateLevelGroup, error) {
				capturedPayload = payload
				capturedEmail = email
				return map[string]updates.UpdateLevelGroup{
					"5": {UpdateType: "security"},
				}, nil
			},
		}
		h := NewUpdatesHandler(client)
		body := `{"productName":"wso2am","productVersion":"4.3.0","startingUpdateLevel":1,"endingUpdateLevel":10}`
		r := withUser(httptest.NewRequest(http.MethodPost, "/updates/levels/search", strings.NewReader(body)))
		w := httptest.NewRecorder()
		h.SearchUpdatesBetweenUpdateLevels(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedEmail != testUser.Email {
			t.Errorf("upstream called with email %q, want %q", capturedEmail, testUser.Email)
		}
		if capturedPayload.ProductName != "wso2am" {
			t.Errorf("payload.ProductName = %q, want wso2am", capturedPayload.ProductName)
		}
		if capturedPayload.StartingUpdateLevel != 1 || capturedPayload.EndingUpdateLevel != 10 {
			t.Errorf("payload levels = [%d, %d], want [1, 10]", capturedPayload.StartingUpdateLevel, capturedPayload.EndingUpdateLevel)
		}
		resp := decodeJSON[map[string]updates.UpdateLevelGroup](t, w)
		if g, ok := resp["5"]; !ok || g.UpdateType != "security" {
			t.Errorf("unexpected response: %+v", resp)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to search updates.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockUpdatesClient{
					searchFn: func(_ context.Context, _ updates.SearchPayload, _ string) (map[string]updates.UpdateLevelGroup, error) {
						return nil, tc.err
					},
				}
				h := NewUpdatesHandler(client)
				body := `{"productName":"wso2am","productVersion":"4.3.0","startingUpdateLevel":1,"endingUpdateLevel":5}`
				r := withUser(httptest.NewRequest(http.MethodPost, "/updates/levels/search", strings.NewReader(body)))
				w := httptest.NewRecorder()
				h.SearchUpdatesBetweenUpdateLevels(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
