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

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// deadlineOf runs one request through the middleware and returns how long its
// context had left.
func deadlineOf(t *testing.T, mw func(http.Handler) http.Handler, path string) time.Duration {
	t.Helper()
	var left time.Duration
	h := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		d, ok := r.Context().Deadline()
		if !ok {
			t.Fatal("the request has no deadline")
		}
		left = time.Until(d)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	return left
}

func TestTimeoutWith_GivesTheLongerLimitOnlyWhereAsked(t *testing.T) {
	mw := TimeoutWith(time.Minute, func(r *http.Request) (time.Duration, bool) {
		return time.Hour, r.URL.Path == "/big"
	})
	if got := deadlineOf(t, mw, "/small"); got > time.Minute || got < 50*time.Second {
		t.Errorf("an ordinary request had %v left, want about a minute", got)
	}
	if got := deadlineOf(t, mw, "/big"); got < 59*time.Minute {
		t.Errorf("a long request had %v left, want about an hour", got)
	}
}
