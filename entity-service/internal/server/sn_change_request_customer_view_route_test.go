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

package server

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
)

// The ServiceNow data source applies the legacy change request visibility rule (a
// customer never gets New, Assess or Authorize) in the services, for every request
// whose scope callerIdentityMiddleware did not resolve as unrestricted. These tests
// drive the REAL router (the token middleware, the scope resolution and the routes of
// NewRouter) against a fake ServiceNow that answers every state whatever it was asked,
// so they also pin that staff are still resolved as unrestricted on this data source
// (a service-level test injects the scope and cannot catch that going missing).

const (
	snRouteCustomerClient = "route-customer-portal-backend"
	snRouteCSMClient      = "route-csm-portal-backend"
	snRouteM2MClient      = "route-internal-m2m"
	snRouteProject        = "00000000-0000-4000-8000-000000000001"
)

var snRouteStates = []struct {
	label string
	key   int
}{
	{"New", -5}, {"Assess", -4}, {"Authorize", -3}, {"Customer Approval", 5}, {"Scheduled", -2},
	{"Implement", -1}, {"Review", 0}, {"Customer Review", 1}, {"Rollback", 2}, {"Closed", 3}, {"Canceled", 4},
}

type snRouteFake struct {
	searches atomic.Int32
	lastBody atomic.Value // map[string]any
}

// snRouteEnv is a ServiceNow-mode router with a signer for the user tokens it accepts.
type snRouteEnv struct {
	router http.Handler
	fake   *snRouteFake
	sign   func(email string) string
}

func newSNRouteEnv(t *testing.T) *snRouteEnv {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{
			"kty": "RSA", "kid": "k1", "use": "sig", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
		}}})
	}))
	t.Cleanup(jwks.Close)
	const issuer = "https://test-issuer.invalid/oauth2/token"

	fake := &snRouteFake{}
	sn := http.NewServeMux()
	sn.HandleFunc("/oauth2/token", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "sn-token", "expires_in": 3600})
	})
	sn.HandleFunc("/change-requests/search", func(w http.ResponseWriter, r *http.Request) {
		fake.searches.Add(1)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		fake.lastBody.Store(body)
		rows := make([]map[string]any, 0, len(snRouteStates))
		for i, st := range snRouteStates {
			rows = append(rows, map[string]any{
				"id": fmt.Sprintf("%032x", i+1), "number": fmt.Sprintf("CR-FAKE-%02d", i+1), "title": "Example change " + st.label,
				"createdOn": "2026-01-01 00:00:00",
				"project":   map[string]any{"id": "00000000000040008000000000000001", "name": "Example Corp Platform"},
				"state":     map[string]any{"label": st.label},
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"changeRequests": rows, "totalRecords": len(rows), "offset": 0, "limit": 50})
	})
	sn.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		choices := make([]map[string]any, 0, len(snRouteStates))
		for _, st := range snRouteStates {
			choices = append(choices, map[string]any{"id": st.key, "label": st.label})
		}
		if strings.HasSuffix(r.URL.Path, "/change-requests/stats") {
			counts := make([]map[string]any, 0, len(snRouteStates))
			for _, st := range snRouteStates {
				counts = append(counts, map[string]any{"id": st.key, "label": st.label, "count": 1})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"totalCount": len(snRouteStates), "activeCount": 8, "outstandingCount": 8, "actionRequiredCount": 2,
				"stateCount": counts, "resolvedCount": map[string]any{"total": 1, "currentMonth": 0, "pastThirtyDays": 0},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"changeRequestStates": choices})
	})
	snSrv := httptest.NewServer(sn)
	t.Cleanup(snSrv.Close)

	cfg := &config.Config{
		DataSource:                               config.DataSourceServiceNow,
		ServiceNowIntegrationServiceBaseURL:      snSrv.URL,
		ServiceNowIntegrationServiceTokenURL:     snSrv.URL + "/oauth2/token",
		ServiceNowIntegrationServiceClientID:     "test-client",
		ServiceNowIntegrationServiceClientSecret: "test-secret",
		AuthIssuer:                               issuer,
		AuthJWKSURL:                              jwks.URL,
		AuthUserTokenAudiences:                   []string{"test-spa"},
		RequestTimeout:                           30 * time.Second,
		UpstreamClientTimeout:                    30 * time.Second,
		CustomerPortalBackendClientID:            snRouteCustomerClient,
		CSMPortalBackendClientID:                 snRouteCSMClient,
		CSMPortalUserDomain:                      "example.test",
		// The customer portal's client is listed as machine-to-machine too, by mistake: the
		// customer portal's client is checked first and is never unrestricted.
		M2MClientIDs: map[string]bool{snRouteM2MClient: true, snRouteCustomerClient: true},
	}
	router, closeFn := NewRouter(nil, cfg)
	t.Cleanup(closeFn)

	return &snRouteEnv{router: router, fake: fake, sign: func(email string) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"iss": issuer, "aud": "test-spa", "exp": time.Now().Add(time.Hour).Unix(), "email": email, "userid": "user-1", "sub": "sub-1",
		})
		tok.Header["kid"] = "k1"
		signed, err := tok.SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return signed
	}}
}

// assertion is the gateway's x-jwt-assertion: only decoded, so unsigned content is fine.
func snRouteAssertion(t *testing.T, clientID string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"client_id": clientID})
	signed, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func (e *snRouteEnv) do(t *testing.T, method, path, body, clientID, email string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if clientID != "" {
		req.Header.Set("x-jwt-assertion", snRouteAssertion(t, clientID))
	}
	if email != "" {
		req.Header.Set("x-user-id-token", e.sign(email))
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func (e *snRouteEnv) sentStateKeys(t *testing.T) []int {
	t.Helper()
	body, _ := e.fake.lastBody.Load().(map[string]any)
	filters, _ := body["filters"].(map[string]any)
	raw, present := filters["stateKeys"]
	if !present {
		return nil
	}
	var keys []int
	for _, v := range raw.([]any) {
		keys = append(keys, int(v.(float64)))
	}
	return keys
}

func snRouteStatesOf(t *testing.T, rec *httptest.ResponseRecorder) []string {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ChangeRequests []struct {
			State string `json:"state"`
		} `json:"changeRequests"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var states []string
	for _, cr := range resp.ChangeRequests {
		states = append(states, cr.State)
	}
	return states
}

const snRouteSearchBody = `{"filters":{"projectIds":["` + snRouteProject + `"]},"pagination":{"limit":50}}`

func TestServiceNowRouter_ACustomerNeverGetsNewAssessOrAuthorizeChangeRequests(t *testing.T) {
	e := newSNRouteEnv(t)
	visible := []int{5, -2, -1, 0, 1, 2, 3, 4}

	for _, tc := range []struct {
		name, client, email string
	}{
		{"the customer portal's backend for a customer", snRouteCustomerClient, "dana@customer.example"},
		{"the customer portal's backend for a WSO2 address", snRouteCustomerClient, "agent@example.test"},
		{"a CSM portal user outside WSO2's domain", snRouteCSMClient, "dana@customer.example"},
		{"a request with a user token and no client", "", "dana@customer.example"},
		{"a request with no identity at all", "", ""},
	} {
		t.Run(tc.name+": naming no state", func(t *testing.T) {
			states := snRouteStatesOf(t, e.do(t, http.MethodPost, "/change-requests/search", snRouteSearchBody, tc.client, tc.email))
			if got := e.sentStateKeys(t); !slices.Equal(got, visible) {
				t.Fatalf("stateKeys sent to ServiceNow = %v, want %v", got, visible)
			}
			for _, s := range states {
				if s == "new" || s == "assess" || s == "authorize" {
					t.Fatalf("a customer was handed a change request in state %q (states %v)", s, states)
				}
			}
			if len(states) != len(visible) {
				t.Fatalf("got %d change requests, want %d", len(states), len(visible))
			}
		})
		t.Run(tc.name+": naming only Authorize", func(t *testing.T) {
			before := e.fake.searches.Load()
			body := `{"filters":{"projectIds":["` + snRouteProject + `"],"states":["authorize"]},"pagination":{"limit":50}}`
			states := snRouteStatesOf(t, e.do(t, http.MethodPost, "/change-requests/search", body, tc.client, tc.email))
			if len(states) != 0 {
				t.Fatalf("got %v, want no change requests", states)
			}
			if e.fake.searches.Load() != before {
				t.Fatal("ServiceNow was asked for a state no customer may see")
			}
		})
	}
}

func TestServiceNowRouter_StaffAreNotNarrowed(t *testing.T) {
	e := newSNRouteEnv(t)
	for _, tc := range []struct {
		name, client, email string
	}{
		{"a machine-to-machine client", snRouteM2MClient, ""},
		{"a machine-to-machine client with a user token", snRouteM2MClient, "someone@customer.example"},
		{"the CSM portal's backend for a user of WSO2's domain", snRouteCSMClient, "agent@example.test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			states := snRouteStatesOf(t, e.do(t, http.MethodPost, "/change-requests/search", snRouteSearchBody, tc.client, tc.email))
			if got := e.sentStateKeys(t); got != nil {
				t.Fatalf("stateKeys sent to ServiceNow = %v, want none (staff are not narrowed)", got)
			}
			if len(states) != len(snRouteStates) {
				t.Fatalf("staff got %d change requests, want all %d", len(states), len(snRouteStates))
			}
		})
	}
}

// What every state filter is built from.
func TestServiceNowRouter_ACustomerIsNotOfferedAHiddenStateToFilterBy(t *testing.T) {
	e := newSNRouteEnv(t)
	idsOf := func(rec *httptest.ResponseRecorder) []int {
		t.Helper()
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			ChangeRequestStates []struct {
				ID any `json:"id"`
			} `json:"changeRequestStates"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		var ids []int
		for _, s := range resp.ChangeRequestStates {
			switch v := s.ID.(type) {
			case float64:
				ids = append(ids, int(v))
			case string:
				var n int
				if _, err := fmt.Sscan(v, &n); err != nil {
					t.Fatalf("id %q is not a number", v)
				}
				ids = append(ids, n)
			}
		}
		return ids
	}

	customer := idsOf(e.do(t, http.MethodGet, "/projects/"+snRouteProject+"/metadata", "", snRouteCustomerClient, "dana@customer.example"))
	if want := []int{5, -2, -1, 0, 1, 2, 3, 4}; !slices.Equal(customer, want) {
		t.Fatalf("a customer is offered the states %v, want %v", customer, want)
	}
	staff := idsOf(e.do(t, http.MethodGet, "/projects/"+snRouteProject+"/metadata", "", snRouteM2MClient, ""))
	if len(staff) != len(snRouteStates) {
		t.Fatalf("staff are offered %v, want all %d states", staff, len(snRouteStates))
	}
}

// The per-state counts of the change request stats: the same vocabulary.
func TestServiceNowRouter_ACustomerIsNotToldTheCountsOfHiddenStates(t *testing.T) {
	e := newSNRouteEnv(t)
	countsOf := func(rec *httptest.ResponseRecorder) (ids []int, total int) {
		t.Helper()
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			TotalCount int `json:"totalCount"`
			StateCount []struct {
				ID any `json:"id"`
			} `json:"stateCount"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		for _, s := range resp.StateCount {
			switch v := s.ID.(type) {
			case float64:
				ids = append(ids, int(v))
			case string:
				var n int
				_, _ = fmt.Sscan(v, &n)
				ids = append(ids, n)
			}
		}
		return ids, resp.TotalCount
	}
	path := "/projects/" + snRouteProject + "/change-requests/stats"

	customer, total := countsOf(e.do(t, http.MethodGet, path, "", snRouteCustomerClient, "dana@customer.example"))
	if want := []int{5, -2, -1, 0, 1, 2, 3, 4}; !slices.Equal(customer, want) {
		t.Fatalf("a customer is told the counts of states %v, want %v", customer, want)
	}
	if total != len(snRouteStates) {
		t.Fatalf("totalCount = %d, want ServiceNow's own figure passed through (%d)", total, len(snRouteStates))
	}
	staff, _ := countsOf(e.do(t, http.MethodGet, path, "", snRouteM2MClient, ""))
	if len(staff) != len(snRouteStates) {
		t.Fatalf("staff are told the counts of %v, want all %d states", staff, len(snRouteStates))
	}
}
