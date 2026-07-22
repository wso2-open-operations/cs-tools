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

package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-integration-service/internal/middleware"
)

func TestCorrelationID_GeneratesWhenAbsent(t *testing.T) {
	t.Parallel()

	var gotFromContext string
	handler := middleware.CorrelationID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFromContext = middleware.CorrelationIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	echoed := w.Header().Get("X-CSM-Correlation-ID")
	if echoed == "" {
		t.Fatal("X-CSM-Correlation-ID response header is empty, want a generated ID")
	}
	if !strings.HasPrefix(echoed, "cis-") {
		t.Errorf("generated ID = %q, want it prefixed with %q", echoed, "cis-")
	}
	if gotFromContext != echoed {
		t.Errorf("CorrelationIDFromContext() = %q, want it to match the echoed header %q", gotFromContext, echoed)
	}
}

func TestCorrelationID_PrefixesIncomingWithoutPrefix(t *testing.T) {
	t.Parallel()

	const incomingID = "caller-supplied-id-123"
	const wantID = "cis-" + incomingID
	var gotFromContext string
	handler := middleware.CorrelationID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFromContext = middleware.CorrelationIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-CSM-Correlation-ID", incomingID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if echoed := w.Header().Get("X-CSM-Correlation-ID"); echoed != wantID {
		t.Errorf("echoed X-CSM-Correlation-ID = %q, want %q", echoed, wantID)
	}
	if gotFromContext != wantID {
		t.Errorf("CorrelationIDFromContext() = %q, want %q", gotFromContext, wantID)
	}
}

func TestCorrelationID_PreservesAlreadyPrefixedIncoming(t *testing.T) {
	t.Parallel()

	const incomingID = "cis-already-prefixed-id-456"
	var gotFromContext string
	handler := middleware.CorrelationID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFromContext = middleware.CorrelationIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-CSM-Correlation-ID", incomingID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if echoed := w.Header().Get("X-CSM-Correlation-ID"); echoed != incomingID {
		t.Errorf("echoed X-CSM-Correlation-ID = %q, want unchanged %q (no double prefix)", echoed, incomingID)
	}
	if gotFromContext != incomingID {
		t.Errorf("CorrelationIDFromContext() = %q, want %q", gotFromContext, incomingID)
	}
}

func TestCorrelationID_NormalizesRepeatedPrefix(t *testing.T) {
	t.Parallel()

	const incomingID = "cis-cis-cis-repeated-id-789"
	const wantID = "cis-repeated-id-789"
	var gotFromContext string
	handler := middleware.CorrelationID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFromContext = middleware.CorrelationIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-CSM-Correlation-ID", incomingID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if echoed := w.Header().Get("X-CSM-Correlation-ID"); echoed != wantID {
		t.Errorf("echoed X-CSM-Correlation-ID = %q, want normalized to exactly one prefix %q", echoed, wantID)
	}
	if gotFromContext != wantID {
		t.Errorf("CorrelationIDFromContext() = %q, want %q", gotFromContext, wantID)
	}
}

func TestCorrelationID_GeneratesDistinctIDs(t *testing.T) {
	t.Parallel()

	handler := middleware.CorrelationID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	seen := make(map[string]bool)
	for i := 0; i < 10; i++ {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		id := w.Header().Get("X-CSM-Correlation-ID")
		if seen[id] {
			t.Fatalf("generated duplicate correlation ID %q across requests", id)
		}
		seen[id] = true
	}
}
