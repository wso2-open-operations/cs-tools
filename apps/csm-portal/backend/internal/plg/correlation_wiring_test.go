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

	csmhandler "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/handler"
	csm "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/entityclient"
	plghandler "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/handler"
)

// TestRegister_WiresCorrelationForwardingAroundIdentity covers the one line in
// register() that composes the chain.
//
// The middleware has its own tests, but they exercise it directly — deleting the
// call from register() left every one of them green while no PLG request
// forwarded anything. This drives a request through the REAL registered chain
// instead, and asserts on the header entity-service receives.
//
// It also pins the ORDER. The stub identity below stands in for ResolveIdentity,
// which makes an entity-service call of its own to resolve the caller; it makes
// that call here too. If ForwardCorrelationID were mounted inside identity
// rather than around it, this call — the most frequent PLG request to
// entity-service there is — would go uncorrelated, and the header asserted below
// would be empty.
func TestRegister_WiresCorrelationForwardingAroundIdentity(t *testing.T) {
	const sentID = "99999999-8888-7777-6666-555555555555"

	// Guarded because the handler writes on the server's goroutine while the
	// loop below both resets and reads on the test's own — two writers, not one.
	// net/http does order them in practice, but that edge belongs to the
	// transport rather than the memory model, and `make test` runs -race with
	// `make build` depending on it.
	var (
		mu               sync.Mutex
		receivedByEntity string
	)
	setReceived := func(v string) { mu.Lock(); defer mu.Unlock(); receivedByEntity = v }
	getReceived := func() string { mu.Lock(); defer mu.Unlock(); return receivedByEntity }

	entity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setReceived(r.Header.Get("X-CSM-Correlation-ID"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"products":[]}`))
	}))
	defer entity.Close()

	client := entityclient.New(entityclient.Config{BaseURL: entity.URL})

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

	// EVERY route, not a sample. register() composes them all through one
	// closure, so a single case would pass even if that closure were applied
	// selectively — and the lists are shared with the permission tests, so a new
	// route added without one trips TestRoutes_EveryRouteIsCovered first.
	all := append(append([]struct{ method, path string }{}, everydayRoutes...), playbookManagementRoutes...)
	if len(all) != 25 {
		t.Fatalf("expected 25 PLG routes, found %d — update this test with the route table", len(all))
	}

	for _, rt := range all {
		setReceived("")
		req := httptest.NewRequest(rt.method, rt.path, strings.NewReader("{}"))
		req.Header.Set("X-CSM-Correlation-ID", sentID)
		// admin satisfies both PermUsePlg and PermManagePlaybooks, so every
		// route reaches the chain rather than stopping at a 403.
		user := &csm.UserInfo{Email: "adm@example.com", UserID: "sub-1", Roles: []string{"adm"}}
		req = req.WithContext(csm.WithUserInfo(req.Context(), user))

		// csm.CorrelationID in front, exactly as cmd/server/main.go mounts it.
		csm.CorrelationID(mux).ServeHTTP(httptest.NewRecorder(), req)

		if got := getReceived(); got != sentID {
			t.Errorf("%s %s: entity service received correlation id %q, want %q",
				rt.method, rt.path, got, sentID)
		}
	}
}
