// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package auth

import (
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// assertion builds an unsigned JWT with the given claims JSON, the shape the gateway forwards.
func assertion(claims string) string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"RS256"}`)) + "." + enc([]byte(claims)) + ".sig"
}

func TestRequireClientID(t *testing.T) {
	call := func(allowed []string, header string) int {
		h := RequireClientID(allowed, slog.New(slog.NewTextHandler(io.Discard, nil)))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		r := httptest.NewRequest(http.MethodGet, "/alert-mute-rules/muted-counts", nil)
		if header != "" {
			r.Header.Set(JWTAssertionHeader, header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Code
	}
	allowed := []string{"portal-app", " integration-app "}
	cases := []struct {
		name    string
		allowed []string
		header  string
		want    int
	}{
		{"allowed client_id", allowed, assertion(`{"client_id":"portal-app"}`), http.StatusOK},
		{"allowed after trimming the setting", allowed, assertion(`{"client_id":"integration-app"}`), http.StatusOK},
		{"azp when there is no client_id", allowed, assertion(`{"azp":"portal-app"}`), http.StatusOK},
		{"client_id wins over azp", allowed, assertion(`{"client_id":"other","azp":"portal-app"}`), http.StatusForbidden},
		{"another client", allowed, assertion(`{"client_id":"someone-else"}`), http.StatusForbidden},
		{"client ids are case sensitive", allowed, assertion(`{"client_id":"Portal-App"}`), http.StatusForbidden},
		{"no assertion", allowed, "", http.StatusUnauthorized},
		{"not a JWT", allowed, "Bearer portal-app", http.StatusUnauthorized},
		{"payload not base64", allowed, "a.%%%.c", http.StatusUnauthorized},
		{"payload not JSON", allowed, "a." + base64.RawURLEncoding.EncodeToString([]byte("nope")) + ".c", http.StatusUnauthorized},
		{"no client in the claims", allowed, assertion(`{"sub":"x"}`), http.StatusUnauthorized},
		{"nothing allowed rejects everyone", nil, assertion(`{"client_id":"portal-app"}`), http.StatusForbidden},
		{"blank entries allow nothing", []string{" ", ""}, assertion(`{"client_id":" "}`), http.StatusUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := call(c.allowed, c.header); got != c.want {
				t.Errorf("status = %d, want %d", got, c.want)
			}
		})
	}
}

func TestSplitClientIDs(t *testing.T) {
	got := SplitClientIDs(" a, ,b,,c ")
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("SplitClientIDs = %q, want %q", got, want)
	}
	if got := SplitClientIDs(""); got != nil {
		t.Errorf("SplitClientIDs(\"\") = %q, want nil", got)
	}
}
