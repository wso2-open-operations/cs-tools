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

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The health listener is published at Public network visibility (see
// .choreo/component.yaml), so what it does NOT serve is as much a part of the
// contract as what it does.
func TestHealthServer_ServesOnlyHealthRoutes(t *testing.T) {
	srv := NewHealthServer(":0", nil)

	// Both probes are routed. What matters here is that neither is a 404,
	// i.e. that both are actually registered; the exact body each returns
	// is asserted in the handler's own tests.
	served := []string{"/health", "/health/database"}
	for _, path := range served {
		rec := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want %d", path, rec.Code, http.StatusOK)
		}
	}

	// A sample of real business routes from the main router. None of them
	// may ever be reachable here — this listener is the public surface.
	notServed := []string{
		"/users/me",
		"/accounts/search",
		"/cases/search",
		"/metadata",
		"/",
	}
	for _, path := range notServed {
		rec := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d on the public health listener, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

func TestHealthServer_HasRequestTimeouts(t *testing.T) {
	// A slow or half-open client must not be able to tie up the listener:
	// a health endpoint that stops answering reads as an outage to whatever
	// is polling it.
	srv := NewHealthServer(":0", nil)

	if srv.ReadTimeout == 0 || srv.WriteTimeout == 0 || srv.IdleTimeout == 0 {
		t.Errorf("health server has an unbounded timeout: read=%v write=%v idle=%v",
			srv.ReadTimeout, srv.WriteTimeout, srv.IdleTimeout)
	}
}
