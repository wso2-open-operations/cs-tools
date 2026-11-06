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

type mockAllocationClient struct {
	searchFn func(ctx context.Context, body []byte) ([]byte, error)
	getFn    func(ctx context.Context, id string) ([]byte, error)
	listFn   func(ctx context.Context, id, limit, offset string) ([]byte, error)
	createFn func(ctx context.Context, id string, body []byte) ([]byte, error)
}

func (m *mockAllocationClient) SearchEngagementAllocations(ctx context.Context, body []byte) ([]byte, error) {
	return m.searchFn(ctx, body)
}
func (m *mockAllocationClient) GetEngagementAllocation(ctx context.Context, id string) ([]byte, error) {
	return m.getFn(ctx, id)
}
func (m *mockAllocationClient) ListEngagementAllocationUpdates(ctx context.Context, id, limit, offset string) ([]byte, error) {
	return m.listFn(ctx, id, limit, offset)
}
func (m *mockAllocationClient) CreateEngagementAllocationUpdate(ctx context.Context, id string, body []byte) ([]byte, error) {
	return m.createFn(ctx, id, body)
}

const allocID = "00000000-0000-0000-0000-000000000001"

func allocUpstreamErr(code int, body string) error {
	return &apierror.Error{StatusCode: code, Body: body}
}

func TestEngagementAllocations_Search(t *testing.T) {
	const path = "/engagement-allocations/search"
	const payload = `{"pagination":{"limit":20,"offset":0},"filters":{"states":["CONFIRMED"]},"sortBy":{"field":"startDate","order":"desc"}}`
	const upstream = `{"allocations":[{"id":"` + allocID + `","engagement":{"id":"00000000-0000-0000-0000-000000000002"}}],"total":1,"limit":20,"offset":0,"hasMore":false}`

	t.Run("requires an authenticated user", func(t *testing.T) {
		h := NewEngagementAllocationHandler(&mockAllocationClient{})
		w := httptest.NewRecorder()
		h.SearchEngagementAllocations(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload)))
		assertStatus(t, w, http.StatusUnauthorized)
	})
	t.Run("rejects invalid JSON", func(t *testing.T) {
		h := NewEngagementAllocationHandler(&mockAllocationClient{})
		w := httptest.NewRecorder()
		h.SearchEngagementAllocations(w, withUser(httptest.NewRequest(http.MethodPost, path, strings.NewReader("not-json"))))
		assertStatus(t, w, http.StatusBadRequest)
	})
	t.Run("rejects an oversized body", func(t *testing.T) {
		h := NewEngagementAllocationHandler(&mockAllocationClient{})
		w := httptest.NewRecorder()
		h.SearchEngagementAllocations(w, withUser(httptest.NewRequest(http.MethodPost, path, strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))))
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
	})
	t.Run("forwards the body unchanged and returns the response as sent", func(t *testing.T) {
		var captured []byte
		h := NewEngagementAllocationHandler(&mockAllocationClient{searchFn: func(_ context.Context, b []byte) ([]byte, error) {
			captured = b
			return []byte(upstream), nil
		}})
		w := httptest.NewRecorder()
		h.SearchEngagementAllocations(w, withUser(httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))))
		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(captured) != payload || w.Body.String() != upstream {
			t.Fatalf("altered in transit: sent %s, got %s", captured, w.Body.String())
		}
	})
	t.Run("maps upstream errors without leaking detail", func(t *testing.T) {
		for _, c := range []struct {
			err  error
			want int
		}{
			{allocUpstreamErr(400, `{"message":"unknown field xyz"}`), 400},
			{allocUpstreamErr(403, `{"message":"not staff"}`), 403},
			{allocUpstreamErr(500, `{"message":"pq: boom"}`), 500},
			{errors.New("down"), 500},
		} {
			h := NewEngagementAllocationHandler(&mockAllocationClient{searchFn: func(context.Context, []byte) ([]byte, error) { return nil, c.err }})
			w := httptest.NewRecorder()
			h.SearchEngagementAllocations(w, withUser(httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))))
			assertStatus(t, w, c.want)
			if strings.Contains(w.Body.String(), "pq:") || strings.Contains(w.Body.String(), "unknown field") {
				t.Fatalf("detail leaked: %s", w.Body.String())
			}
		}
	})
}

func TestEngagementAllocations_Get(t *testing.T) {
	newReq := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/engagement-allocations/"+allocID, nil)
		r.SetPathValue("id", allocID)
		return r
	}
	t.Run("requires an authenticated user", func(t *testing.T) {
		w := httptest.NewRecorder()
		NewEngagementAllocationHandler(&mockAllocationClient{}).GetEngagementAllocation(w, newReq())
		assertStatus(t, w, http.StatusUnauthorized)
	})
	t.Run("passes the id and the response through", func(t *testing.T) {
		const upstream = `{"id":"` + allocID + `","engagement":{"id":"00000000-0000-0000-0000-000000000002"}}`
		var gotID string
		h := NewEngagementAllocationHandler(&mockAllocationClient{getFn: func(_ context.Context, id string) ([]byte, error) {
			gotID = id
			return []byte(upstream), nil
		}})
		w := httptest.NewRecorder()
		h.GetEngagementAllocation(w, withUser(newReq()))
		assertStatus(t, w, http.StatusOK)
		if gotID != allocID || w.Body.String() != upstream {
			t.Fatalf("id %q, body %s", gotID, w.Body.String())
		}
	})
	t.Run("maps 404, 400 and 403", func(t *testing.T) {
		for _, code := range []int{404, 400, 403} {
			h := NewEngagementAllocationHandler(&mockAllocationClient{getFn: func(context.Context, string) ([]byte, error) {
				return nil, allocUpstreamErr(code, `{"message":"x"}`)
			}})
			w := httptest.NewRecorder()
			h.GetEngagementAllocation(w, withUser(newReq()))
			assertStatus(t, w, code)
		}
	})
}

func TestEngagementAllocations_ListUpdates(t *testing.T) {
	newReq := func(query string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/engagement-allocations/"+allocID+"/updates"+query, nil)
		r.SetPathValue("id", allocID)
		return r
	}
	t.Run("requires an authenticated user", func(t *testing.T) {
		w := httptest.NewRecorder()
		NewEngagementAllocationHandler(&mockAllocationClient{}).ListEngagementAllocationUpdates(w, newReq(""))
		assertStatus(t, w, http.StatusUnauthorized)
	})
	t.Run("passes id, paging and the response through", func(t *testing.T) {
		const upstream = `{"updates":[],"total":0,"limit":5,"offset":10,"hasMore":false}`
		var id, limit, offset string
		h := NewEngagementAllocationHandler(&mockAllocationClient{listFn: func(_ context.Context, i, l, o string) ([]byte, error) {
			id, limit, offset = i, l, o
			return []byte(upstream), nil
		}})
		w := httptest.NewRecorder()
		h.ListEngagementAllocationUpdates(w, withUser(newReq("?limit=5&offset=10")))
		assertStatus(t, w, http.StatusOK)
		if id != allocID || limit != "5" || offset != "10" || w.Body.String() != upstream {
			t.Fatalf("id %q limit %q offset %q body %s", id, limit, offset, w.Body.String())
		}
	})
	t.Run("maps 404 and 400", func(t *testing.T) {
		for _, code := range []int{404, 400} {
			h := NewEngagementAllocationHandler(&mockAllocationClient{listFn: func(context.Context, string, string, string) ([]byte, error) {
				return nil, allocUpstreamErr(code, `{"message":"x"}`)
			}})
			w := httptest.NewRecorder()
			h.ListEngagementAllocationUpdates(w, withUser(newReq("")))
			assertStatus(t, w, code)
		}
	})
}

func TestEngagementAllocations_CreateUpdate(t *testing.T) {
	const payload = `{"subject":"Weekly status","content":"On track."}`
	newReq := func(body string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/engagement-allocations/"+allocID+"/updates", strings.NewReader(body))
		r.SetPathValue("id", allocID)
		return r
	}
	t.Run("requires an authenticated user", func(t *testing.T) {
		w := httptest.NewRecorder()
		NewEngagementAllocationHandler(&mockAllocationClient{}).CreateEngagementAllocationUpdate(w, newReq(payload))
		assertStatus(t, w, http.StatusUnauthorized)
	})
	t.Run("rejects invalid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		NewEngagementAllocationHandler(&mockAllocationClient{}).CreateEngagementAllocationUpdate(w, withUser(newReq("nope")))
		assertStatus(t, w, http.StatusBadRequest)
	})
	t.Run("rejects an oversized body", func(t *testing.T) {
		w := httptest.NewRecorder()
		NewEngagementAllocationHandler(&mockAllocationClient{}).CreateEngagementAllocationUpdate(w, withUser(newReq(strings.Repeat("x", maxRequestBodyBytes+1))))
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
	})
	t.Run("forwards the body unchanged and answers 201 with the stored update", func(t *testing.T) {
		const upstream = `{"id":"00000000-0000-0000-0000-000000000009","createdOn":"2026-10-27T00:00:00Z"}`
		var id string
		var captured []byte
		h := NewEngagementAllocationHandler(&mockAllocationClient{createFn: func(_ context.Context, i string, b []byte) ([]byte, error) {
			id, captured = i, b
			return []byte(upstream), nil
		}})
		w := httptest.NewRecorder()
		h.CreateEngagementAllocationUpdate(w, withUser(newReq(payload)))
		assertStatus(t, w, http.StatusCreated)
		if id != allocID || string(captured) != payload || w.Body.String() != upstream {
			t.Fatalf("id %q captured %s body %s", id, captured, w.Body.String())
		}
	})
	t.Run("keeps the reason for 403 and 400, stays generic otherwise", func(t *testing.T) {
		for _, c := range []struct {
			name    string
			err     error
			status  int
			wantMsg string
		}{
			{"403 not the allocated engineer", allocUpstreamErr(403, `{"message":"only the engineer allocated may post an update"}`), 403, "only the engineer allocated may post an update"},
			{"400 validation", allocUpstreamErr(400, `{"message":"subject is required"}`), 400, "subject is required"},
			{"404 unknown allocation", allocUpstreamErr(404, `{"message":"no such allocation"}`), 404, ErrMsgNotFound},
			{"500 generic", allocUpstreamErr(500, `{"message":"pq: boom"}`), 500, "Failed to post the update."},
			{"403 without a message", allocUpstreamErr(403, `not json`), 403, "Failed to post the update."},
		} {
			t.Run(c.name, func(t *testing.T) {
				h := NewEngagementAllocationHandler(&mockAllocationClient{createFn: func(context.Context, string, []byte) ([]byte, error) { return nil, c.err }})
				w := httptest.NewRecorder()
				h.CreateEngagementAllocationUpdate(w, withUser(newReq(payload)))
				assertStatus(t, w, c.status)
				assertErrorMessage(t, w, c.wantMsg)
				if strings.Contains(w.Body.String(), "pq:") {
					t.Fatalf("detail leaked: %s", w.Body.String())
				}
			})
		}
	})
}

// A caller without the permission never reaches the handler: reads need view,
// posting an update needs write, so a viewer can read but not post.
func TestEngagementAllocations_RoutePermissions(t *testing.T) {
	g := NewAccessGuard(testAccessConfig())
	for _, c := range []struct {
		name  string
		perm  Permission
		roles []string
		want  int
	}{
		{"viewer can read", PermView, []string{"test-viewer"}, http.StatusNoContent},
		{"no role cannot read", PermView, []string{"unknown"}, http.StatusForbidden},
		{"viewer cannot post an update", PermWrite, []string{"test-viewer"}, http.StatusForbidden},
		{"cs engineer can post an update", PermWrite, []string{"test-cs-engineer"}, http.StatusNoContent},
	} {
		t.Run(c.name, func(t *testing.T) {
			status, reached := serveWithRoles(g, c.perm, c.roles)
			if status != c.want || reached != (c.want == http.StatusNoContent) {
				t.Fatalf("status %d reached %v, want %d", status, reached, c.want)
			}
		})
	}
}

func TestEngagementAllocations_RejectsNonUUIDID(t *testing.T) {
	fail := func(t *testing.T) *mockAllocationClient {
		boom := func() { t.Fatal("upstream must not be called for a non-UUID id") }
		return &mockAllocationClient{
			getFn:    func(context.Context, string) ([]byte, error) { boom(); return nil, nil },
			listFn:   func(context.Context, string, string, string) ([]byte, error) { boom(); return nil, nil },
			createFn: func(context.Context, string, []byte) ([]byte, error) { boom(); return nil, nil },
		}
	}
	for _, id := range []string{"abc", "00000000-0000-0000-0000-00000000000g", ""} {
		req := func(method, body string) *http.Request {
			r := httptest.NewRequest(method, "/engagement-allocations/x/updates", strings.NewReader(body))
			r.SetPathValue("id", id)
			return withUser(r)
		}
		h := NewEngagementAllocationHandler(fail(t))
		w := httptest.NewRecorder()
		h.GetEngagementAllocation(w, req(http.MethodGet, ""))
		assertStatus(t, w, http.StatusBadRequest)
		w = httptest.NewRecorder()
		h.ListEngagementAllocationUpdates(w, req(http.MethodGet, ""))
		assertStatus(t, w, http.StatusBadRequest)
		w = httptest.NewRecorder()
		h.CreateEngagementAllocationUpdate(w, req(http.MethodPost, `{"subject":"s","content":"c"}`))
		assertStatus(t, w, http.StatusBadRequest)
	}
}
