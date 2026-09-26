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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchCustomerAnnouncementAudience(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewAnnouncementHandler(&mockEntityProjectClient{}, nil)
		r := httptest.NewRequest(http.MethodPost, "/announcements/audience/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchCustomerAnnouncementAudience(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewAnnouncementHandler(&mockEntityProjectClient{}, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/announcements/audience/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchCustomerAnnouncementAudience(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewAnnouncementHandler(&mockEntityProjectClient{}, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/announcements/audience/search", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchCustomerAnnouncementAudience(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("injects the configured excluded project keys, overwriting any value the caller supplied", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityProjectClient{
			searchProjectsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"projects":[],"total":0}`), nil
			},
		}
		h := NewAnnouncementHandler(client, []string{"Apexia", "Veridian", "Veloxis"})
		reqPayload := `{"pagination":{"limit":50},"excludeClosureStates":["Restricted"],"excludeProjectKeys":["SHOULD-BE-IGNORED"]}`
		r := withUser(httptest.NewRequest(http.MethodPost, "/announcements/audience/search", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.SearchCustomerAnnouncementAudience(w, r)

		assertStatus(t, w, http.StatusOK)

		var forwarded map[string]any
		if err := json.Unmarshal(capturedBody, &forwarded); err != nil {
			t.Fatalf("upstream body is not valid JSON: %v", err)
		}
		gotKeys, ok := forwarded["excludeProjectKeys"].([]any)
		if !ok {
			t.Fatalf("excludeProjectKeys = %v (%T), want a []string-shaped array", forwarded["excludeProjectKeys"], forwarded["excludeProjectKeys"])
		}
		want := []string{"Apexia", "Veridian", "Veloxis"}
		if len(gotKeys) != len(want) {
			t.Fatalf("excludeProjectKeys = %v, want %v", gotKeys, want)
		}
		for i, k := range want {
			if gotKeys[i] != k {
				t.Errorf("excludeProjectKeys[%d] = %v, want %q", i, gotKeys[i], k)
			}
		}
		// The caller's other fields (pagination, excludeClosureStates) must
		// still reach the entity service unchanged — only excludeProjectKeys
		// is a mandatory, server-enforced override.
		if _, ok := forwarded["excludeClosureStates"]; !ok {
			t.Errorf("excludeClosureStates was dropped from the forwarded body: %v", forwarded)
		}
	})

	t.Run("forwards an empty excludeProjectKeys when nothing is configured", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityProjectClient{
			searchProjectsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"projects":[],"total":0}`), nil
			},
		}
		h := NewAnnouncementHandler(client, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/announcements/audience/search", strings.NewReader(`{}`)))
		w := httptest.NewRecorder()
		h.SearchCustomerAnnouncementAudience(w, r)

		assertStatus(t, w, http.StatusOK)
		var forwarded map[string]any
		if err := json.Unmarshal(capturedBody, &forwarded); err != nil {
			t.Fatalf("upstream body is not valid JSON: %v", err)
		}
		v, ok := forwarded["excludeProjectKeys"]
		if !ok {
			t.Fatalf("excludeProjectKeys missing from forwarded body, want []")
		}
		arr, ok := v.([]any)
		if !ok || len(arr) != 0 {
			t.Errorf("excludeProjectKeys = %v, want an explicit empty array", v)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to resolve the announcement audience.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityProjectClient{
					searchProjectsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewAnnouncementHandler(client, nil)
				r := withUser(httptest.NewRequest(http.MethodPost, "/announcements/audience/search", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.SearchCustomerAnnouncementAudience(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}

func TestGetExcludedProjectKeys(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewAnnouncementHandler(&mockEntityProjectClient{}, []string{"Apexia"})
		r := httptest.NewRequest(http.MethodGet, "/announcements/audience/excluded-project-keys", nil)
		w := httptest.NewRecorder()
		h.GetExcludedProjectKeys(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("returns the configured list read-only, in order", func(t *testing.T) {
		h := NewAnnouncementHandler(&mockEntityProjectClient{}, []string{"Apexia", "Veridian", "Veloxis"})
		r := withUser(httptest.NewRequest(http.MethodGet, "/announcements/audience/excluded-project-keys", nil))
		w := httptest.NewRecorder()
		h.GetExcludedProjectKeys(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		resp := decodeJSON[excludedProjectKeysResponse](t, w)
		want := []string{"Apexia", "Veridian", "Veloxis"}
		if len(resp.ExcludedProjectKeys) != len(want) {
			t.Fatalf("ExcludedProjectKeys = %v, want %v", resp.ExcludedProjectKeys, want)
		}
		for i, k := range want {
			if resp.ExcludedProjectKeys[i] != k {
				t.Errorf("ExcludedProjectKeys[%d] = %q, want %q", i, resp.ExcludedProjectKeys[i], k)
			}
		}
	})

	t.Run("returns an empty array, never null, when nothing is configured", func(t *testing.T) {
		h := NewAnnouncementHandler(&mockEntityProjectClient{}, nil)
		r := withUser(httptest.NewRequest(http.MethodGet, "/announcements/audience/excluded-project-keys", nil))
		w := httptest.NewRecorder()
		h.GetExcludedProjectKeys(w, r)

		assertStatus(t, w, http.StatusOK)
		if !strings.Contains(w.Body.String(), `"excludedProjectKeys":[]`) {
			t.Errorf("body = %s, want excludedProjectKeys to serialize as [] not null", w.Body.String())
		}
	})
}
