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

const (
	testSreAlertID     = "01a21bb0-c1a4-7954-8ee2-f7cb2d4b63f7"
	testSreAlertNumber = "ALT000000042"
)

func TestSearchSreAlerts(t *testing.T) {
	const validBody = `{"filters":{"searchQuery":"cpu","environments":["prod"],"tiers":["priority"],"incident":"linked","receivedFrom":"2026-11-01T00:00:00Z","receivedTo":"2026-11-27T00:00:00Z"},"sortBy":{"field":"receivedOn","order":"desc"},"pagination":{"offset":0,"limit":20}}`

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewSreAlertHandler(&mockEntitySreAlertClient{})
		r := httptest.NewRequest(http.MethodPost, "/sre-alerts/search", strings.NewReader(validBody))
		w := httptest.NewRecorder()
		h.SearchSreAlerts(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("forwards the raw body and returns the upstream response", func(t *testing.T) {
		var captured string
		client := &mockEntitySreAlertClient{
			searchFn: func(_ context.Context, body []byte) ([]byte, error) {
				captured = string(body)
				return []byte(`{"alerts":[{"id":"` + testSreAlertID + `","number":"` + testSreAlertNumber + `"}],"total":1,"offset":0,"limit":20}`), nil
			},
		}
		h := NewSreAlertHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/sre-alerts/search", strings.NewReader(validBody)))
		w := httptest.NewRecorder()
		h.SearchSreAlerts(w, r)
		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if captured != validBody {
			t.Errorf("upstream body = %s, want %s", captured, validBody)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["total"] != float64(1) {
			t.Errorf("total = %v, want 1", resp["total"])
		}
	})

	t.Run("accepts an incidentId filter", func(t *testing.T) {
		h := NewSreAlertHandler(&mockEntitySreAlertClient{})
		body := `{"filters":{"incidentId":"11111111-1111-1111-1111-111111111111"}}`
		r := withUser(httptest.NewRequest(http.MethodPost, "/sre-alerts/search", strings.NewReader(body)))
		w := httptest.NewRecorder()
		h.SearchSreAlerts(w, r)
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("accepts an empty object", func(t *testing.T) {
		h := NewSreAlertHandler(&mockEntitySreAlertClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/sre-alerts/search", strings.NewReader(`{}`)))
		w := httptest.NewRecorder()
		h.SearchSreAlerts(w, r)
		assertStatus(t, w, http.StatusOK)
	})

	invalid := map[string]string{
		"malformed JSON":       `{"filters":`,
		"JSON null":            `null`,
		"JSON array":           `[]`,
		"unknown tier":         `{"filters":{"tiers":["urgent"]}}`,
		"unknown incident":     `{"filters":{"incident":"maybe"}}`,
		"receivedFrom not RFC": `{"filters":{"receivedFrom":"2026-11-01"}}`,
		"receivedTo not RFC":   `{"filters":{"receivedTo":"yesterday"}}`,
		"unknown sort field":   `{"sortBy":{"field":"metricName"}}`,
		"unknown sort order":   `{"sortBy":{"field":"number","order":"up"}}`,
		"negative offset":      `{"pagination":{"offset":-1}}`,
		"limit over 100":       `{"pagination":{"limit":101}}`,
		"tiers wrong type":     `{"filters":{"tiers":"priority"}}`,
		"incidentId not UUID":  `{"filters":{"incidentId":"INC0012345"}}`,
	}
	for name, body := range invalid {
		t.Run("rejects "+name, func(t *testing.T) {
			called := false
			client := &mockEntitySreAlertClient{
				searchFn: func(_ context.Context, _ []byte) ([]byte, error) {
					called = true
					return []byte(`{}`), nil
				},
			}
			h := NewSreAlertHandler(client)
			r := withUser(httptest.NewRequest(http.MethodPost, "/sre-alerts/search", strings.NewReader(body)))
			w := httptest.NewRecorder()
			h.SearchSreAlerts(w, r)
			assertStatus(t, w, http.StatusBadRequest)
			assertErrorMessage(t, w, ErrMsgBadRequest)
			if called {
				t.Error("upstream was called for an invalid body")
			}
		})
	}

	t.Run("rejects a body over the size limit", func(t *testing.T) {
		h := NewSreAlertHandler(&mockEntitySreAlertClient{})
		big := `{"filters":{"searchQuery":"` + strings.Repeat("a", maxRequestBodyBytes) + `"}}`
		r := withUser(httptest.NewRequest(http.MethodPost, "/sre-alerts/search", strings.NewReader(big)))
		w := httptest.NewRecorder()
		h.SearchSreAlerts(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to search alerts.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntitySreAlertClient{
					searchFn: func(_ context.Context, _ []byte) ([]byte, error) { return nil, tc.err },
				}
				h := NewSreAlertHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/sre-alerts/search", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.SearchSreAlerts(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestGetSreAlert(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewSreAlertHandler(&mockEntitySreAlertClient{})
		r := httptest.NewRequest(http.MethodGet, "/sre-alerts/"+testSreAlertID, nil)
		r.SetPathValue("id", testSreAlertID)
		w := httptest.NewRecorder()
		h.GetSreAlert(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	for _, id := range []string{testSreAlertID, testSreAlertNumber} {
		t.Run("forwards "+id, func(t *testing.T) {
			var captured string
			client := &mockEntitySreAlertClient{
				getFn: func(_ context.Context, got string) ([]byte, error) {
					captured = got
					return []byte(`{"id":"` + testSreAlertID + `","number":"` + testSreAlertNumber + `","rawBody":null}`), nil
				},
			}
			h := NewSreAlertHandler(client)
			r := withUser(httptest.NewRequest(http.MethodGet, "/sre-alerts/"+id, nil))
			r.SetPathValue("id", id)
			w := httptest.NewRecorder()
			h.GetSreAlert(w, r)
			assertStatus(t, w, http.StatusOK)
			assertContentType(t, w, "application/json")
			if captured != id {
				t.Errorf("upstream id = %q, want %q", captured, id)
			}
			resp := decodeJSON[map[string]any](t, w)
			if resp["number"] != testSreAlertNumber {
				t.Errorf("number = %v, want %s", resp["number"], testSreAlertNumber)
			}
		})
	}

	for _, id := range []string{"", "not-a-uuid", "ALT42", "ALT0000000420", "alt000000042", "INC0012345", "ALT00000004a", "ALT000000042/x"} {
		t.Run("rejects id "+id, func(t *testing.T) {
			called := false
			client := &mockEntitySreAlertClient{
				getFn: func(_ context.Context, _ string) ([]byte, error) {
					called = true
					return []byte(`{}`), nil
				},
			}
			h := NewSreAlertHandler(client)
			r := withUser(httptest.NewRequest(http.MethodGet, "/sre-alerts/x", nil))
			r.SetPathValue("id", id)
			w := httptest.NewRecorder()
			h.GetSreAlert(w, r)
			assertStatus(t, w, http.StatusBadRequest)
			assertErrorMessage(t, w, errMsgInvalidSreAlertID)
			if called {
				t.Error("upstream was called for an invalid id")
			}
		})
	}

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve alert.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntitySreAlertClient{
					getFn: func(_ context.Context, _ string) ([]byte, error) { return nil, tc.err },
				}
				h := NewSreAlertHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/sre-alerts/"+testSreAlertNumber, nil))
				r.SetPathValue("id", testSreAlertNumber)
				w := httptest.NewRecorder()
				h.GetSreAlert(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestGetSreAlertFacets(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewSreAlertHandler(&mockEntitySreAlertClient{})
		r := httptest.NewRequest(http.MethodGet, "/sre-alerts/facets", nil)
		w := httptest.NewRecorder()
		h.GetSreAlertFacets(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("returns the upstream facets", func(t *testing.T) {
		client := &mockEntitySreAlertClient{
			facetsFn: func(_ context.Context) ([]byte, error) {
				return []byte(`{"environments":["prod","staging"],"services":["svc-a"],"severities":["critical"],"sources":["Azure Monitor"]}`), nil
			},
		}
		h := NewSreAlertHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/sre-alerts/facets", nil))
		w := httptest.NewRecorder()
		h.GetSreAlertFacets(w, r)
		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		resp := decodeJSON[map[string][]string](t, w)
		if len(resp["environments"]) != 2 || resp["sources"][0] != "Azure Monitor" {
			t.Errorf("unexpected facets: %v", resp)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve alert filters.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntitySreAlertClient{
					facetsFn: func(_ context.Context) ([]byte, error) { return nil, tc.err },
				}
				h := NewSreAlertHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/sre-alerts/facets", nil))
				w := httptest.NewRecorder()
				h.GetSreAlertFacets(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
