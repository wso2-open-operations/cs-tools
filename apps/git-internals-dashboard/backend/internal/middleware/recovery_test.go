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

package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRecoveryConvertsPanicToEnvelope verifies a panic in the wrapped
// handler is recovered into a 500 JSON error envelope.
func TestRecoveryConvertsPanicToEnvelope(t *testing.T) {
	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/issues", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected valid JSON envelope, got err: %v, body: %s", err, rec.Body.String())
	}
	if body.Error.Code != "internal" {
		t.Fatalf("expected code=internal, got %q", body.Error.Code)
	}
}

// TestRecoveryRepanicsErrAbortHandler verifies net/http's sanctioned "abort
// this response, don't log a stack trace" signal —
// panic(http.ErrAbortHandler) — propagates through Recovery rather than
// being converted into a logged "panic recovered" plus a 500 envelope write
// on a connection net/http already intends to drop.
func TestRecoveryRepanicsErrAbortHandler(t *testing.T) {
	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	req := httptest.NewRequest(http.MethodGet, "/issues", nil)
	rec := httptest.NewRecorder()

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		handler.ServeHTTP(rec, req)
	}()

	if recovered != http.ErrAbortHandler {
		t.Fatalf("expected http.ErrAbortHandler to propagate out of Recovery, got: %v", recovered)
	}
	if rec.Code != http.StatusOK { // httptest.NewRecorder defaults to 200 — nothing was ever written
		t.Errorf("expected no response to have been written, got status %d body %q", rec.Code, rec.Body.String())
	}
}

// TestRecoveryDoesNotWriteAfterResponseCommitted guards against corrupting an
// already-started response: once a downstream handler has written a status
// (or body) and then panics, net/http keeps what was already sent regardless
// of anything written afterward, so Recovery must not attempt to append the
// 500 envelope on top of it.
func TestRecoveryDoesNotWriteAfterResponseCommitted(t *testing.T) {
	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
		panic("boom after commit")
	}))

	req := httptest.NewRequest(http.MethodGet, "/issues", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected the original 200 to be preserved, got %d", rec.Code)
	}
	if rec.Body.String() != `{"ok":true}` {
		t.Fatalf("expected the body to be untouched by the recovered panic, got %q", rec.Body.String())
	}
}

// TestRecoveryPassesThroughWithoutPanic verifies a handler that completes
// normally is unaffected by Recovery.
func TestRecoveryPassesThroughWithoutPanic(t *testing.T) {
	called := false
	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/issues", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected wrapped handler to run")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
