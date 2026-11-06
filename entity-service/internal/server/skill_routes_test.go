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
	"os"
	"strings"
	"testing"
)

// The skills routes put literal segments ("me", "levels", "search") beside a
// "{id}" wildcard, which net/http's mux accepts only when one pattern is
// strictly more specific than the other. Registering a conflicting pair panics
// at startup, so this registers exactly what routes.go declares and checks each
// literal path is answered by its own handler, not by the wildcard.
func TestSkillRoutesDoNotShadowEachOther(t *testing.T) {
	src, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	var n int
	for _, m := range handleFuncRE.FindAllStringSubmatch(string(src), -1) {
		if !strings.HasPrefix(m[2], "/skills") {
			continue
		}
		pattern := m[1] + " " + m[2]
		mux.HandleFunc(pattern, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(pattern)) })
		n++
	}
	if n != 12 {
		t.Fatalf("found %d skills routes in routes.go, want 12", n)
	}
	for _, c := range []struct{ method, path, want string }{
		{"GET", "/skills/me", "GET /skills/me"},
		{"GET", "/skills/levels", "GET /skills/levels"},
		{"GET", "/skills/categories", "GET /skills/categories"},
		{"GET", "/skills/me/history", "GET /skills/me/history"},
		{"GET", "/skills/5e1e0000-0000-4000-8000-0000000000d1", "GET /skills/{id}"},
		{"PATCH", "/skills/5e1e0000-0000-4000-8000-0000000000d1", "PATCH /skills/{id}"},
		{"POST", "/skills/search", "POST /skills/search"},
		{"POST", "/skills/me", "POST /skills/me"},
		{"POST", "/skills/users/search", "POST /skills/users/search"},
		{"PUT", "/skills/me/5e1e0000-0000-4000-8000-0000000000d1", "PUT /skills/me/{skillId}"},
		{"DELETE", "/skills/me/5e1e0000-0000-4000-8000-0000000000d1", "DELETE /skills/me/{skillId}"},
		{"POST", "/skills", "POST /skills"},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if got := rec.Body.String(); got != c.want {
			t.Errorf("%s %s answered by %q, want %q", c.method, c.path, got, c.want)
		}
	}
}
