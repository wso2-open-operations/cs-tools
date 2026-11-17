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
	"net/url"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// fakeProductCatalogClient records what the handler forwarded and answers
// with a fixed body (or err, when set).
type fakeProductCatalogClient struct {
	calls []string
	id    string
	body  []byte
	query url.Values
	err   error
}

func (f *fakeProductCatalogClient) answer(op, id string, body []byte) ([]byte, error) {
	f.calls = append(f.calls, op)
	f.id, f.body = id, body
	return []byte(`{"ok":true}`), f.err
}

func (f *fakeProductCatalogClient) GetProduct(_ context.Context, id string) ([]byte, error) {
	return f.answer("GetProduct", id, nil)
}
func (f *fakeProductCatalogClient) CreateProduct(_ context.Context, b []byte) ([]byte, error) {
	return f.answer("CreateProduct", "", b)
}
func (f *fakeProductCatalogClient) UpdateProduct(_ context.Context, id string, b []byte) ([]byte, error) {
	return f.answer("UpdateProduct", id, b)
}
func (f *fakeProductCatalogClient) CreateProductVersions(_ context.Context, id string, b []byte) ([]byte, error) {
	return f.answer("CreateProductVersions", id, b)
}
func (f *fakeProductCatalogClient) SearchAllProductVersions(_ context.Context, b []byte) ([]byte, error) {
	return f.answer("SearchAllProductVersions", "", b)
}
func (f *fakeProductCatalogClient) GetProductVersion(_ context.Context, id string) ([]byte, error) {
	return f.answer("GetProductVersion", id, nil)
}
func (f *fakeProductCatalogClient) UpdateProductVersion(_ context.Context, id string, b []byte) ([]byte, error) {
	return f.answer("UpdateProductVersion", id, b)
}
func (f *fakeProductCatalogClient) RetireProductVersion(_ context.Context, id string) ([]byte, error) {
	return f.answer("RetireProductVersion", id, nil)
}
func (f *fakeProductCatalogClient) ListReleaseLines(_ context.Context, q url.Values) ([]byte, error) {
	f.query = q
	return f.answer("ListReleaseLines", "", nil)
}
func (f *fakeProductCatalogClient) CreateReleaseLine(_ context.Context, b []byte) ([]byte, error) {
	return f.answer("CreateReleaseLine", "", b)
}
func (f *fakeProductCatalogClient) UpdateReleaseLine(_ context.Context, id string, b []byte) ([]byte, error) {
	return f.answer("UpdateReleaseLine", id, b)
}
func (f *fakeProductCatalogClient) RetireReleaseLine(_ context.Context, id string) ([]byte, error) {
	return f.answer("RetireReleaseLine", id, nil)
}
func (f *fakeProductCatalogClient) ListCarbonPlatformHistory(context.Context) ([]byte, error) {
	return f.answer("ListCarbonPlatformHistory", "", nil)
}
func (f *fakeProductCatalogClient) ListCarbonKernelReleases(context.Context) ([]byte, error) {
	return f.answer("ListCarbonKernelReleases", "", nil)
}

const testCatalogID = "11111111-1111-4111-8111-111111111111"

// catalogRoute is one handler with how to call it.
type catalogRoute struct {
	name   string
	fn     func(h *ProductCatalogHandler) http.HandlerFunc
	method string
	withID bool
	body   string
	status int
	op     string
}

var catalogRoutes = []catalogRoute{
	{"GetProduct", func(h *ProductCatalogHandler) http.HandlerFunc { return h.GetProduct }, http.MethodGet, true, "", http.StatusOK, "GetProduct"},
	{"CreateProduct", func(h *ProductCatalogHandler) http.HandlerFunc { return h.CreateProduct }, http.MethodPost, false, `{"name":"X","class":"software"}`, http.StatusCreated, "CreateProduct"},
	{"UpdateProduct", func(h *ProductCatalogHandler) http.HandlerFunc { return h.UpdateProduct }, http.MethodPatch, true, `{"code":null}`, http.StatusOK, "UpdateProduct"},
	{"CreateProductVersions", func(h *ProductCatalogHandler) http.HandlerFunc { return h.CreateProductVersions }, http.MethodPost, true, `{"version":"1.0.0","profiles":["All In One"]}`, http.StatusCreated, "CreateProductVersions"},
	{"SearchAllProductVersions", func(h *ProductCatalogHandler) http.HandlerFunc { return h.SearchAllProductVersions }, http.MethodPost, false, `{"excludeRetired":true}`, http.StatusOK, "SearchAllProductVersions"},
	{"GetProductVersion", func(h *ProductCatalogHandler) http.HandlerFunc { return h.GetProductVersion }, http.MethodGet, true, "", http.StatusOK, "GetProductVersion"},
	{"UpdateProductVersion", func(h *ProductCatalogHandler) http.HandlerFunc { return h.UpdateProductVersion }, http.MethodPatch, true, `{"supportEolDate":"2030-01-31"}`, http.StatusOK, "UpdateProductVersion"},
	{"RetireProductVersion", func(h *ProductCatalogHandler) http.HandlerFunc { return h.RetireProductVersion }, http.MethodPost, true, "", http.StatusOK, "RetireProductVersion"},
	{"ListReleaseLines", func(h *ProductCatalogHandler) http.HandlerFunc { return h.ListReleaseLines }, http.MethodGet, false, "", http.StatusOK, "ListReleaseLines"},
	{"CreateReleaseLine", func(h *ProductCatalogHandler) http.HandlerFunc { return h.CreateReleaseLine }, http.MethodPost, false, `{"name":"APIM-5.x"}`, http.StatusCreated, "CreateReleaseLine"},
	{"UpdateReleaseLine", func(h *ProductCatalogHandler) http.HandlerFunc { return h.UpdateReleaseLine }, http.MethodPatch, true, `{"status":"Active"}`, http.StatusOK, "UpdateReleaseLine"},
	{"RetireReleaseLine", func(h *ProductCatalogHandler) http.HandlerFunc { return h.RetireReleaseLine }, http.MethodPost, true, "", http.StatusOK, "RetireReleaseLine"},
	{"ListCarbonPlatformHistory", func(h *ProductCatalogHandler) http.HandlerFunc { return h.ListCarbonPlatformHistory }, http.MethodGet, false, "", http.StatusOK, "ListCarbonPlatformHistory"},
	{"ListCarbonKernelReleases", func(h *ProductCatalogHandler) http.HandlerFunc { return h.ListCarbonKernelReleases }, http.MethodGet, false, "", http.StatusOK, "ListCarbonKernelReleases"},
}

func (rt catalogRoute) request(user bool, id, body string) *http.Request {
	r := httptest.NewRequest(rt.method, "/x", strings.NewReader(body))
	if rt.withID {
		r.SetPathValue("id", id)
	}
	if user {
		r = withUser(r)
	}
	return r
}

func TestProductCatalogHandler_ForwardsEveryRoute(t *testing.T) {
	for _, rt := range catalogRoutes {
		t.Run(rt.name, func(t *testing.T) {
			fake := &fakeProductCatalogClient{}
			w := httptest.NewRecorder()
			rt.fn(NewProductCatalogHandler(fake))(w, rt.request(true, testCatalogID, rt.body))
			if w.Code != rt.status {
				t.Fatalf("status = %d, want %d; body %s", w.Code, rt.status, w.Body.String())
			}
			if len(fake.calls) != 1 || fake.calls[0] != rt.op {
				t.Fatalf("calls = %v, want [%s]", fake.calls, rt.op)
			}
			if rt.withID && fake.id != testCatalogID {
				t.Errorf("id = %q", fake.id)
			}
			if rt.body != "" && string(fake.body) != rt.body {
				t.Errorf("body = %s, want %s", fake.body, rt.body)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("Cache-Control = %q", w.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestProductCatalogHandler_RequiresAUser(t *testing.T) {
	for _, rt := range catalogRoutes {
		fake := &fakeProductCatalogClient{}
		w := httptest.NewRecorder()
		rt.fn(NewProductCatalogHandler(fake))(w, rt.request(false, testCatalogID, rt.body))
		if w.Code != http.StatusUnauthorized || len(fake.calls) != 0 {
			t.Errorf("%s without a user: status = %d calls = %v", rt.name, w.Code, fake.calls)
		}
	}
}

func TestProductCatalogHandler_RejectsABadID(t *testing.T) {
	for _, rt := range catalogRoutes {
		if !rt.withID {
			continue
		}
		fake := &fakeProductCatalogClient{}
		w := httptest.NewRecorder()
		rt.fn(NewProductCatalogHandler(fake))(w, rt.request(true, "not-a-uuid", rt.body))
		if w.Code != http.StatusBadRequest || len(fake.calls) != 0 {
			t.Errorf("%s with a bad id: status = %d calls = %v", rt.name, w.Code, fake.calls)
		}
	}
}

func TestProductCatalogHandler_BodyMustBeAJSONObject(t *testing.T) {
	for _, rt := range catalogRoutes {
		if rt.body == "" {
			continue
		}
		for _, bad := range []string{"", "nope", "[1]", `{"a":1`, "null"} {
			fake := &fakeProductCatalogClient{}
			w := httptest.NewRecorder()
			rt.fn(NewProductCatalogHandler(fake))(w, rt.request(true, testCatalogID, bad))
			if w.Code != http.StatusBadRequest || len(fake.calls) != 0 {
				t.Errorf("%s with body %q: status = %d calls = %v", rt.name, bad, w.Code, fake.calls)
			}
		}
	}
}

func TestProductCatalogHandler_ListReleaseLinesQuery(t *testing.T) {
	h := func(fake *fakeProductCatalogClient) http.HandlerFunc {
		return NewProductCatalogHandler(fake).ListReleaseLines
	}
	fake := &fakeProductCatalogClient{}
	w := httptest.NewRecorder()
	h(fake)(w, withUser(httptest.NewRequest(http.MethodGet, "/release-lines?includeRetired=true&x=1", nil)))
	if w.Code != http.StatusOK || fake.query.Encode() != "includeRetired=true" {
		t.Fatalf("status = %d query = %q", w.Code, fake.query.Encode())
	}
	for _, target := range []string{"/release-lines?includeRetired=1", "/release-lines?includeRetired=true&includeRetired=false"} {
		fake := &fakeProductCatalogClient{}
		w := httptest.NewRecorder()
		h(fake)(w, withUser(httptest.NewRequest(http.MethodGet, target, nil)))
		if w.Code != http.StatusBadRequest || len(fake.calls) != 0 {
			t.Errorf("%s: status = %d calls = %v", target, w.Code, fake.calls)
		}
	}
}

func TestProductCatalogHandler_ReadHidesTheUpstreamDetail(t *testing.T) {
	fake := &fakeProductCatalogClient{err: &apierror.Error{StatusCode: http.StatusBadRequest, Body: `{"message":"internal detail"}`}}
	w := httptest.NewRecorder()
	NewProductCatalogHandler(fake).SearchAllProductVersions(w, withUser(httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{}`))))
	if w.Code != http.StatusBadRequest || strings.Contains(w.Body.String(), "internal detail") {
		t.Fatalf("status = %d body = %s, want 400 with the fallback message", w.Code, w.Body.String())
	}
}

func TestProductCatalogHandler_WriteSurfacesTheUpstreamReason(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusConflict} {
		fake := &fakeProductCatalogClient{err: &apierror.Error{StatusCode: status, Body: `{"code":400,"message":"supportEolDate must not be before releaseDate"}`}}
		r := withUser(httptest.NewRequest(http.MethodPatch, "/x", strings.NewReader(`{"supportEolDate":"2020-01-01"}`)))
		r.SetPathValue("id", testCatalogID)
		w := httptest.NewRecorder()
		NewProductCatalogHandler(fake).UpdateProductVersion(w, r)
		if w.Code != status || !strings.Contains(w.Body.String(), "supportEolDate must not be before releaseDate") {
			t.Errorf("upstream %d: status = %d body = %s", status, w.Code, w.Body.String())
		}
	}
}
