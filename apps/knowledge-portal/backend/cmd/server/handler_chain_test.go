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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/handler"
)

// panicEntity makes every upstream call panic, standing in for a handler bug.
type panicEntity struct{}

func (panicEntity) ListKnowledgeBases(context.Context) ([]entity.KnowledgeBase, error) {
	panic("boom with internal detail")
}

func (panicEntity) SearchKBArticles(context.Context, entity.SearchRequest) (entity.SearchResponse, error) {
	panic("boom with internal detail")
}

func (panicEntity) GetKBArticle(context.Context, string) (entity.KBArticle, error) {
	panic("boom with internal detail")
}

// TestServedChainTurnsAPanicIntoAJSON500 builds the handler with newHandler,
// the same constructor main serves, and checks that a panic deep in a route
// still yields the JSON error envelope with the outer middleware's headers.
func TestServedChainTurnsAPanicIntoAJSON500(t *testing.T) {
	h := newHandler(handler.NewKBHandler(panicEntity{}, nil))

	req := httptest.NewRequest(http.MethodGet, "/public/knowledge-bases", nil)
	req.Header.Set("X-CSM-Correlation-ID", "caller-chosen")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}
	var body struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Message == "" {
		t.Fatalf("body %q is not the {message} envelope: %v", rec.Body, err)
	}
	if strings.Contains(rec.Body.String(), "internal detail") {
		t.Errorf("panic detail leaked: %s", rec.Body)
	}
	for k, v := range map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"Content-Security-Policy":   "upgrade-insecure-requests",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
	} {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	id := rec.Header().Get("X-CSM-Correlation-ID")
	if id == "" || id == "caller-chosen" {
		t.Errorf("correlation id header = %q, want a server-minted id", id)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("CORS header set by the service; the gateway owns CORS")
	}
}

// TestServedChainRoutes checks the routes newHandler registers answer, and that
// an unknown path gets the JSON 404 with the same headers.
func TestServedChainRoutes(t *testing.T) {
	h := newHandler(handler.NewKBHandler(panicEntity{}, nil))
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/health", http.StatusOK},
		{http.MethodGet, "/nope", http.StatusNotFound},
		{http.MethodGet, "/public/kb-articles/not-a-uuid", http.StatusBadRequest},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != tc.want {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.path, rec.Code, tc.want)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("X-CSM-Correlation-ID") == "" {
			t.Errorf("%s %s: missing security or correlation header", tc.method, tc.path)
		}
	}
}
