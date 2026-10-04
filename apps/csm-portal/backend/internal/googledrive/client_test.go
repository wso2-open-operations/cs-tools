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

package googledrive

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// TestTokenRefreshIsBounded verifies a stalled token endpoint fails the call
// within the refresh timeout instead of hanging.
func TestTokenRefreshIsBounded(t *testing.T) {
	stall := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-stall
	}))
	defer srv.Close()
	defer close(stall)

	prevEndpoint, prevTimeout := googleOAuth2Endpoint, tokenRefreshTimeout
	googleOAuth2Endpoint = oauth2.Endpoint{TokenURL: srv.URL}
	tokenRefreshTimeout = 100 * time.Millisecond
	t.Cleanup(func() { googleOAuth2Endpoint, tokenRefreshTimeout = prevEndpoint, prevTimeout })

	c := NewClient(Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/files", nil)
	start := time.Now()
	resp, err := c.http.Do(req)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected an error from the stalled token endpoint")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("token refresh took %v, want it bounded by the refresh timeout", elapsed)
	}
}
