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

func TestPostDeployment(t *testing.T) {
	const validBody = `{"projectId":"11111111-1111-1111-1111-111111111111","name":"Prod","type":"primary_production","description":"Main prod deployment"}`

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewDeploymentHandler(&mockEntityDeploymentClient{})
		r := httptest.NewRequest(http.MethodPost, "/deployments", strings.NewReader(validBody))
		w := httptest.NewRecorder()
		h.PostDeployment(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewDeploymentHandler(&mockEntityDeploymentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/deployments", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.PostDeployment(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewDeploymentHandler(&mockEntityDeploymentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/deployments", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.PostDeployment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 201", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityDeploymentClient{
			postDeploymentFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"Deployment created successfully","deployment":{"id":"11111111-1111-1111-1111-111111111111","createdOn":"2026-06-26T00:00:00Z","createdBy":"user@wso2.com"}}`), nil
			},
		}
		h := NewDeploymentHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/deployments", strings.NewReader(validBody)))
		w := httptest.NewRecorder()
		h.PostDeployment(w, r)

		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != validBody {
			t.Errorf("upstream received body %q, want %q", capturedBody, validBody)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["message"] != "Deployment created successfully" {
			t.Errorf("message = %v, want %q", resp["message"], "Deployment created successfully")
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to create deployment.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityDeploymentClient{
					postDeploymentFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewDeploymentHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/deployments", strings.NewReader(validBody)))
				w := httptest.NewRecorder()
				h.PostDeployment(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestSearchDeployments(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewDeploymentHandler(&mockEntityDeploymentClient{})
		r := httptest.NewRequest(http.MethodPost, "/deployments/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchDeployments(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewDeploymentHandler(&mockEntityDeploymentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/deployments/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchDeployments(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewDeploymentHandler(&mockEntityDeploymentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/deployments/search", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchDeployments(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body to upstream and returns 200", func(t *testing.T) {
		const reqPayload = `{"projectIds":["proj-1"],"searchQuery":"prod","deploymentTypeKeys":["primary_production"]}`
		var capturedBody []byte
		client := &mockEntityDeploymentClient{
			searchDeploymentsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"deployments":[{"id":"dep-1","name":"Production"}],"total":1}`), nil
			},
		}
		h := NewDeploymentHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/deployments/search", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.SearchDeployments(w, r)

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
		for _, tc := range upstreamErrors("Failed to search deployments.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityDeploymentClient{
					searchDeploymentsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewDeploymentHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/deployments/search", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.SearchDeployments(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestPatchDeployment(t *testing.T) {
	const validID = "11111111-1111-1111-1111-111111111111"

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewDeploymentHandler(&mockEntityDeploymentClient{})
		r := httptest.NewRequest(http.MethodPatch, "/deployments/"+validID, strings.NewReader(`{"name":"prod"}`))
		r.SetPathValue("id", validID)
		w := httptest.NewRecorder()
		h.PatchDeployment(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID deployment id", func(t *testing.T) {
		h := NewDeploymentHandler(&mockEntityDeploymentClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/deployments/not-a-uuid", strings.NewReader(`{"name":"prod"}`)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.PatchDeployment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewDeploymentHandler(&mockEntityDeploymentClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/deployments/"+validID, strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("id", validID)
		w := httptest.NewRecorder()
		h.PatchDeployment(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewDeploymentHandler(&mockEntityDeploymentClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/deployments/"+validID, strings.NewReader(`not-json`)))
		r.SetPathValue("id", validID)
		w := httptest.NewRecorder()
		h.PatchDeployment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body and deployment id to upstream and returns 200", func(t *testing.T) {
		const reqPayload = `{"name":"new-name"}`
		var capturedID string
		var capturedBody []byte
		client := &mockEntityDeploymentClient{
			patchDeploymentFn: func(_ context.Context, deploymentID string, body []byte) ([]byte, error) {
				capturedID = deploymentID
				capturedBody = body
				return []byte(`{"message":"Deployment updated successfully","deployment":{"id":"` + validID + `","updatedOn":"2026-06-26T00:00:00Z","updatedBy":"user@wso2.com"}}`), nil
			},
		}
		h := NewDeploymentHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/deployments/"+validID, strings.NewReader(reqPayload)))
		r.SetPathValue("id", validID)
		w := httptest.NewRecorder()
		h.PatchDeployment(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedID != validID {
			t.Errorf("upstream received id %q, want %q", capturedID, validID)
		}
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["message"] != "Deployment updated successfully" {
			t.Errorf("message = %v, want %q", resp["message"], "Deployment updated successfully")
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to update deployment.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityDeploymentClient{
					patchDeploymentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewDeploymentHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPatch, "/deployments/"+validID, strings.NewReader(`{"name":"x"}`)))
				r.SetPathValue("id", validID)
				w := httptest.NewRecorder()
				h.PatchDeployment(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestSearchDeployedProducts(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewDeploymentHandler(&mockEntityDeploymentClient{})
		r := httptest.NewRequest(http.MethodPost, "/deployments/dep-1/products/search", strings.NewReader(`{}`))
		r.SetPathValue("id", "dep-1")
		w := httptest.NewRecorder()
		h.SearchDeployedProducts(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewDeploymentHandler(&mockEntityDeploymentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/deployments/dep-1/products/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		r.SetPathValue("id", "dep-1")
		w := httptest.NewRecorder()
		h.SearchDeployedProducts(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewDeploymentHandler(&mockEntityDeploymentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/deployments/dep-1/products/search", strings.NewReader(`not-json`)))
		r.SetPathValue("id", "dep-1")
		w := httptest.NewRecorder()
		h.SearchDeployedProducts(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("handles JSON null body as empty object without panic", func(t *testing.T) {
		h := NewDeploymentHandler(&mockEntityDeploymentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/deployments/dep-1/products/search", strings.NewReader(`null`)))
		r.SetPathValue("id", "dep-1")
		w := httptest.NewRecorder()
		h.SearchDeployedProducts(w, r)
		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects missing deployment id", func(t *testing.T) {
		h := NewDeploymentHandler(&mockEntityDeploymentClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/deployments//products/search", strings.NewReader(`{}`)))
		w := httptest.NewRecorder()
		h.SearchDeployedProducts(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("injects deploymentId and returns 200 with response", func(t *testing.T) {
		const deploymentID = "dep-1"
		var capturedBody []byte
		client := &mockEntityDeploymentClient{
			searchDeployedProductsFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"deployedProducts":[{"id":"dp-1","deploymentId":"dep-1"}],"total":1}`), nil
			},
		}
		h := NewDeploymentHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/deployments/dep-1/products/search", strings.NewReader(`{"pagination":{"limit":10,"offset":0}}`)))
		r.SetPathValue("id", deploymentID)
		w := httptest.NewRecorder()
		h.SearchDeployedProducts(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		var sent map[string]json.RawMessage
		if err := json.Unmarshal(capturedBody, &sent); err != nil {
			t.Fatalf("upstream received invalid JSON: %v", err)
		}
		var ids []string
		if err := json.Unmarshal(sent["deploymentIds"], &ids); err != nil || len(ids) != 1 || ids[0] != deploymentID {
			t.Errorf("upstream deploymentIds = %v, want [%q]", ids, deploymentID)
		}

		resp := decodeJSON[map[string]any](t, w)
		if resp["total"] != float64(1) {
			t.Errorf("total = %v, want 1", resp["total"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to search deployed products.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityDeploymentClient{
					searchDeployedProductsFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewDeploymentHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/deployments/dep-1/products/search", strings.NewReader(`{}`)))
				r.SetPathValue("id", "dep-1")
				w := httptest.NewRecorder()
				h.SearchDeployedProducts(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
