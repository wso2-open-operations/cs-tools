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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// wrappedMaxBytesBody fails every read with an *http.MaxBytesError wrapped in
// another error, which a direct type assertion would miss.
type wrappedMaxBytesBody struct{}

func (wrappedMaxBytesBody) Read([]byte) (int, error) {
	return 0, fmt.Errorf("transport: %w", &http.MaxBytesError{Limit: 1})
}
func (wrappedMaxBytesBody) Close() error { return nil }

type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, errors.New("connection reset") }
func (failingBody) Close() error             { return nil }

func TestReadJSONBody(t *testing.T) {
	cases := []struct {
		name     string
		body     io.Reader
		policy   bodyPolicy
		wantOK   bool
		wantBody string
		wantCode int
		wantMsg  string
	}{
		{"required, valid", strings.NewReader(`{"a":1}`), bodyRequired, true, `{"a":1}`, 0, ""},
		{"optional, valid", strings.NewReader(`{"a":1}`), bodyOptional, true, `{"a":1}`, 0, ""},
		{"required, empty", strings.NewReader(``), bodyRequired, false, "", http.StatusBadRequest, ErrMsgBadRequest},
		{"optional, empty", strings.NewReader(``), bodyOptional, true, "", 0, ""},
		{"required, invalid", strings.NewReader(`{`), bodyRequired, false, "", http.StatusBadRequest, ErrMsgBadRequest},
		{"optional, invalid", strings.NewReader(`nope`), bodyOptional, false, "", http.StatusBadRequest, ErrMsgBadRequest},
		{"optional, whitespace only", strings.NewReader(`   `), bodyOptional, false, "", http.StatusBadRequest, ErrMsgBadRequest},
		{"too large", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)), bodyRequired, false, "", http.StatusRequestEntityTooLarge, ErrMsgTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/x", tc.body)
			got, ok := readJSONBody(w, r, tc.policy)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok {
				if string(got) != tc.wantBody {
					t.Errorf("body = %q, want %q", got, tc.wantBody)
				}
				if w.Body.Len() != 0 {
					t.Errorf("wrote a response on success: %s", w.Body.String())
				}
				return
			}
			assertStatus(t, w, tc.wantCode)
			assertErrorMessage(t, w, tc.wantMsg)
		})
	}
}

func TestReadJSONBody_WrappedMaxBytesErrorIs413(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Body = wrappedMaxBytesBody{}
	if _, ok := readJSONBody(w, r, bodyRequired); ok {
		t.Fatal("ok = true, want false")
	}
	assertStatus(t, w, http.StatusRequestEntityTooLarge)
	assertErrorMessage(t, w, ErrMsgTooLarge)
}

func TestReadJSONBody_ReadFailureIs400(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Body = failingBody{}
	if _, ok := readJSONBody(w, r, bodyOptional); ok {
		t.Fatal("ok = true, want false")
	}
	assertStatus(t, w, http.StatusBadRequest)
	assertErrorMessage(t, w, errMsgReadBody)
}
