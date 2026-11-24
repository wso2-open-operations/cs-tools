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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDeadlineSetsARequestDeadline(t *testing.T) {
	var left time.Duration
	var ok bool
	h := Deadline(5*time.Second, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var dl time.Time
		dl, ok = r.Context().Deadline()
		left = time.Until(dl)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/products", nil))
	if !ok {
		t.Fatal("request context has no deadline")
	}
	if left <= 0 || left > 5*time.Second {
		t.Fatalf("deadline in %v, want within 5s", left)
	}
}

func TestDeadlineCancelsWhenTheBudgetRunsOut(t *testing.T) {
	var err error
	h := Deadline(10*time.Millisecond, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		err = r.Context().Err()
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/products", nil))
	if err == nil {
		t.Fatal("context was not cancelled")
	}
}
