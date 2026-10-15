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
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
)

// A request's files are held in a table with no row-level security and the actor of
// an upload is whatever the caller names, so the five attachment routes sit behind
// internalOnly on the route itself. These drive the REAL router (token middleware,
// scope resolution, routes) with a lazily connected pool, so no database is needed:
// a request that passes the gate reaches its handler and fails there, which is what
// "let through" means below.

const (
	annAttCSMClient      = "ann-att-csm-portal-backend"
	annAttCustomerClient = "ann-att-customer-portal-backend"
	annAttM2MClient      = "ann-att-internal-m2m"
	annAttStaffDomain    = "example.test"

	annAttRequestID    = "3f1e8d6a-3b4c-4d5e-8f90-123456789abc"
	annAttAttachmentID = "7a1b2c3d-4e5f-4a6b-8c7d-0123456789ab"
	annAttCaseID       = "5c6d7e8f-9a0b-4c1d-8e2f-3a4b5c6d7e8f"

	// The message internalOnly's scope check gives a caller that has no usable identity.
	annAttInternalGateMessage = "authorized internal client credential"
)

var annAttRoutes = []struct{ name, method, path, body string }{
	{"upload", http.MethodPost, "/announcement-requests/" + annAttRequestID + "/attachments",
		`{"name":"a.pdf","file":"data:application/pdf;base64,JVBERi0xLjQ=","actorId":"u1","actorEmail":"u1@example.test"}`},
	{"list", http.MethodGet, "/announcement-requests/" + annAttRequestID + "/attachments", ""},
	{"content", http.MethodGet, "/announcement-requests/" + annAttRequestID + "/attachments/" + annAttAttachmentID + "/content", ""},
	{"remove", http.MethodDelete, "/announcement-requests/" + annAttRequestID + "/attachments/" + annAttAttachmentID + "?actorId=u1", ""},
	{"copy onto a case", http.MethodPost, "/announcement-requests/" + annAttRequestID + "/cases/" + annAttCaseID + "/attachments",
		`{"actorId":"u1","actorEmail":"u1@example.test"}`},
}

type annAttEnv struct {
	router http.Handler
	sign   func(email string) string
}

func newAnnAttEnv(t *testing.T) *annAttEnv {
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

	pool, err := pgxpool.New(context.Background(), "postgres://unused:unused@127.0.0.1:1/unused?sslmode=disable")
	if err != nil {
		t.Fatalf("construct pool: %v", err)
	}
	t.Cleanup(pool.Close)

	cfg := &config.Config{
		DataSource:                    config.DataSourcePostgres,
		DBUser:                        "unused",
		DBPassword:                    "unused",
		DBName:                        "unused",
		AuthIssuer:                    issuer,
		AuthJWKSURL:                   jwks.URL,
		AuthUserTokenAudiences:        []string{"test-spa"},
		RequestTimeout:                30 * time.Second,
		UpstreamClientTimeout:         30 * time.Second,
		CSMPortalBackendClientID:      annAttCSMClient,
		CSMPortalUserDomain:           annAttStaffDomain,
		CustomerPortalBackendClientID: annAttCustomerClient,
		M2MClientIDs:                  map[string]bool{annAttM2MClient: true},
	}
	router, closeFn := NewRouter(pool, cfg)
	t.Cleanup(closeFn)

	return &annAttEnv{router: router, sign: func(email string) string {
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

func (e *annAttEnv) do(t *testing.T, method, path, body, clientID, email string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
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

// passedTheGate reports whether the request got past internalOnly. The handlers behind
// it have no database here, so a request that passes fails later with some other status.
func passedTheGate(rec *httptest.ResponseRecorder) bool {
	return rec.Code != http.StatusUnauthorized && rec.Code != http.StatusForbidden
}

// TestAnnouncementRequestAttachmentRoutesAreInternalOnly pins the route registration:
// with no token at all every one of the five is stopped by the scope check, before
// its handler.
func TestAnnouncementRequestAttachmentRoutesAreInternalOnly(t *testing.T) {
	env := newAnnAttEnv(t)
	for _, tc := range annAttRoutes {
		t.Run(tc.name, func(t *testing.T) {
			rec := env.do(t, tc.method, tc.path, tc.body, "", "")
			if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), annAttInternalGateMessage) {
				t.Errorf("%s %s = %d (%s), want 401 from the internalOnly gate", tc.method, tc.path, rec.Code, strings.TrimSpace(rec.Body.String()))
			}
		})
	}
}

// TestAnnouncementRequestAttachmentRoutesRefuseTheCustomerPortal: the customer portal's
// backend is never an internal caller, whatever else it is listed as, so a request it
// forwards without a staff identity cannot reach a draft's files.
func TestAnnouncementRequestAttachmentRoutesRefuseTheCustomerPortal(t *testing.T) {
	env := newAnnAttEnv(t)
	for _, tc := range annAttRoutes {
		t.Run(tc.name, func(t *testing.T) {
			rec := env.do(t, tc.method, tc.path, tc.body, annAttCustomerClient, "")
			if passedTheGate(rec) {
				t.Errorf("%s %s = %d (%s), want the internalOnly gate to refuse the customer portal's client", tc.method, tc.path, rec.Code, strings.TrimSpace(rec.Body.String()))
			}
		})
	}
}

// TestAnnouncementRequestAttachmentRoutesLetInternalCallersThrough is the other half,
// and what keeps the gate from breaking the feature: the identities that really use
// these routes still get past it. The CSM portal backend (its client id plus a staff
// user of the configured domain) is how every request from the web app arrives, and an
// internal machine client is how the scheduled publish is.
func TestAnnouncementRequestAttachmentRoutesLetInternalCallersThrough(t *testing.T) {
	env := newAnnAttEnv(t)
	for _, caller := range []struct{ name, clientID, email string }{
		{"the CSM portal backend acting for a staff user", annAttCSMClient, "staff@" + annAttStaffDomain},
		{"an internal machine client", annAttM2MClient, ""},
	} {
		for _, tc := range annAttRoutes {
			t.Run(caller.name+"/"+tc.name, func(t *testing.T) {
				rec := env.do(t, tc.method, tc.path, tc.body, caller.clientID, caller.email)
				if !passedTheGate(rec) || strings.Contains(rec.Body.String(), annAttInternalGateMessage) {
					t.Errorf("%s %s = %d (%s), want the request to get past the internalOnly gate", tc.method, tc.path, rec.Code, strings.TrimSpace(rec.Body.String()))
				}
			})
		}
	}
}

// TestAnnouncementRequestAttachmentRoutesRefuseAnotherDomainBehindTheCSMBackend: the
// CSM backend's client alone is not enough; the user it acts for has to be staff.
func TestAnnouncementRequestAttachmentRoutesRefuseAnotherDomainBehindTheCSMBackend(t *testing.T) {
	env := newAnnAttEnv(t)
	for _, tc := range annAttRoutes {
		t.Run(tc.name, func(t *testing.T) {
			rec := env.do(t, tc.method, tc.path, tc.body, annAttCSMClient, "someone@customer.example")
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s %s = %d (%s), want 403", tc.method, tc.path, rec.Code, strings.TrimSpace(rec.Body.String()))
			}
		})
	}
}
