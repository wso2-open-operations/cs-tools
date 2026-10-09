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

package plg

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
	csmhandler "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/handler"
	csm "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/entityclient"
	plghandler "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/handler"
)

// TestRegister_ForwardsTheUserIDTokenOnEveryRoute covers the one line in
// register() that mounts the forwarder.
//
// EVERY route, not a sample. register() composes them all through one closure,
// so a single case would pass even if that closure were applied selectively —
// which is exactly how the correlation id stayed broken while its own
// middleware's unit tests were green.
//
// It asserts on the HEADER entity-service RECEIVES, not on a context value. The
// bug this prevents is two packages using different context keys: the token was
// in the request context the whole time, just not where PLG's client looked.
// Only the wire shows that.
//
// It also pins the ORDER. The stub identity below stands in for ResolveIdentity,
// which makes an entity-service call of its own; it makes one here too. If the
// forwarder were mounted inside identity rather than around it, that call — the
// most frequent PLG request to entity-service there is — would arrive
// unattributed and the assertion would fail.
func TestRegister_ForwardsTheUserIDTokenOnEveryRoute(t *testing.T) {
	const sentToken = "header.payload.signature"

	// Guarded: the handler writes on the server's goroutine while the loop
	// below resets and reads on the test's own. `make test` runs -race and
	// `make build` depends on it.
	var (
		mu       sync.Mutex
		received string
	)
	set := func(v string) { mu.Lock(); defer mu.Unlock(); received = v }
	get := func() string { mu.Lock(); defer mu.Unlock(); return received }

	entitySvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		set(r.Header.Get("x-user-id-token"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"products":[]}`))
	}))
	defer entitySvc.Close()

	client := entityclient.New(entityclient.Config{BaseURL: entitySvc.URL})

	// Stands in for ResolveIdentity, including its upstream call.
	identity := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := client.ListProducts(r.Context()); err != nil {
				t.Errorf("identity's entity call failed: %v", err)
			}
			w.WriteHeader(http.StatusNoContent)
		})
	}

	mux := http.NewServeMux()
	guard := csmhandler.NewAccessGuard(csmhandler.AccessConfig{Admin: []string{"adm"}})
	route := func(pattern string, perm csmhandler.Permission, h http.HandlerFunc) {
		mux.HandleFunc(pattern, guard.Require(perm, h))
	}
	register(&plghandler.Handlers{}, identity, route)

	all := append(append([]struct{ method, path string }{}, everydayRoutes...), playbookManagementRoutes...)
	if len(all) == 0 {
		t.Fatal("no routes found — the route tables are empty, so this test would prove nothing")
	}

	for _, rt := range all {
		set("")
		req := httptest.NewRequest(rt.method, rt.path, strings.NewReader("{}"))
		// admin satisfies both PLG permissions, so every route reaches the chain
		// rather than stopping at a 403.
		user := &csm.UserInfo{Email: "adm@example.com", UserID: "sub-1", Roles: []string{"adm"}}
		ctx := csm.WithUserInfo(req.Context(), user)
		// Exactly what csm-portal's Auth middleware does before PLG is reached.
		ctx = entity.WithUserIDToken(ctx, sentToken)
		mux.ServeHTTP(httptest.NewRecorder(), req.WithContext(ctx))

		if got := get(); got != sentToken {
			t.Errorf("%s %s: entity service received x-user-id-token %q, want %q",
				rt.method, rt.path, got, sentToken)
		}
	}
}
