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
	"testing"
)

func TestHealth_OKWhenDatabaseReachable(t *testing.T) {
	store := &mockStore{pingFn: func(ctx context.Context) error { return nil }}
	h := NewHealthHandler(store)

	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.Health(w, r)

	assertStatus(t, w, http.StatusOK)
}

func TestHealth_UnavailableWhenDatabaseUnreachable(t *testing.T) {
	store := &mockStore{pingFn: func(ctx context.Context) error { return errors.New("connection refused") }}
	h := NewHealthHandler(store)

	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.Health(w, r)

	assertStatus(t, w, http.StatusServiceUnavailable)
}
