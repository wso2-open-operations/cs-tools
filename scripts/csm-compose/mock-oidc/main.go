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
//
// mock-oidc is a minimal, spec-lite OpenID Connect provider for the
// docker-compose local dev stack described in apps/csm-portal/README.md.
//
// LOCAL DEVELOPMENT ONLY. It signs tokens for any requested username with no
// real credential check, and must never be pointed to by anything other
// than a developer's own machine. It exists so contributors without access
// to a real identity provider can still run the full CSM platform stack:
// it implements just enough of OIDC discovery, authorization-code-with-PKCE,
// client-credentials, JWKS and userinfo for the webapps and Go backends in
// this repo to authenticate against it.
//
// It deliberately names no real identity provider anywhere in its code,
// responses, or comments -- describe it generically as "a mock OIDC
// provider" in any documentation that references it.
package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const kid = "mock-oidc-1"

type server struct {
	key      *rsa.PrivateKey
	issuer   string
	mu       sync.Mutex
	authReqs map[string]*authRequest // code -> request
}

type authRequest struct {
	clientID            string
	redirectURI         string
	state               string
	codeChallenge       string
	codeChallengeMethod string
	claims              map[string]any
	expiresAt           time.Time
}

func main() {
	issuer := envOrDefault("MOCK_OIDC_ISSUER", "http://localhost:9100")
	port := envOrDefault("PORT", "9100")

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("mock-oidc: generating signing key: %v", err)
	}

	s := &server{
		key:      key,
		issuer:   issuer,
		authReqs: make(map[string]*authRequest),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", s.handleDiscovery)
	mux.HandleFunc("GET /oauth2/jwks", s.handleJWKS)
	mux.HandleFunc("GET /oauth2/authorize", s.handleAuthorizeForm)
	mux.HandleFunc("POST /oauth2/authorize", s.handleAuthorizeSubmit)
	mux.HandleFunc("POST /oauth2/token", s.handleToken)
	mux.HandleFunc("GET /oauth2/userinfo", s.handleUserinfo)
	mux.HandleFunc("GET /oidc/logout", s.handleLogout)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	slog.Info("mock-oidc: listening", "port", port, "issuer", issuer)
	log.Fatal(http.ListenAndServe(":"+port, logRequests(mux)))
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("mock-oidc: request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (s *server) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	doc := map[string]any{
		"issuer":                                s.issuer,
		"authorization_endpoint":                s.issuer + "/oauth2/authorize",
		"token_endpoint":                        s.issuer + "/oauth2/token",
		"userinfo_endpoint":                     s.issuer + "/oauth2/userinfo",
		"jwks_uri":                              s.issuer + "/oauth2/jwks",
		"end_session_endpoint":                  s.issuer + "/oidc/logout",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                      []string{"openid", "profile", "email"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "client_secret_basic", "none"},
		"code_challenge_methods_supported":      []string{"S256", "plain"},
		"grant_types_supported":                 []string{"authorization_code", "client_credentials"},
		"claims_supported":                      []string{"sub", "email", "userid", "groups", "roles"},
	}
	writeJSON(w, doc)
}

func (s *server) handleJWKS(w http.ResponseWriter, r *http.Request) {
	n := base64.RawURLEncoding.EncodeToString(s.key.PublicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big3(s.key.PublicKey.E))
	writeJSON(w, map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"kid": kid,
				"n":   n,
				"e":   e,
			},
		},
	})
}

// big3 encodes a small int (the RSA public exponent, typically 65537) as the
// minimal big-endian byte string a JWK "e" member expects.
func big3(v int) []byte {
	buf := []byte{byte(v >> 16), byte(v >> 8), byte(v)}
	i := 0
	for i < len(buf)-1 && buf[i] == 0 {
		i++
	}
	return buf[i:]
}

// handleAuthorizeForm renders a trivial local-dev login form: any typed
// username becomes the token's email/userid, and a comma-separated groups
// field lets a developer simulate any role (e.g. cs_engineer, admin)
// without a real identity backend.
func (s *server) handleAuthorizeForm(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html>
<html><body style="font-family:sans-serif;max-width:420px;margin:40px auto">
<h2>Mock OIDC sign-in (local dev only)</h2>
<p>Enter any username. This provider performs no credential check.</p>
<form method="POST" action="/oauth2/authorize">
<input type="hidden" name="client_id" value="%s">
<input type="hidden" name="redirect_uri" value="%s">
<input type="hidden" name="state" value="%s">
<input type="hidden" name="scope" value="%s">
<input type="hidden" name="code_challenge" value="%s">
<input type="hidden" name="code_challenge_method" value="%s">
<label>Email<br><input name="email" value="jane.doe@example.com" style="width:100%%"></label><br><br>
<label>Groups (comma-separated)<br><input name="groups" value="cs_engineer" style="width:100%%"></label><br><br>
<button type="submit">Sign in</button>
</form>
</body></html>`,
		html.EscapeString(q.Get("client_id")),
		html.EscapeString(q.Get("redirect_uri")),
		html.EscapeString(q.Get("state")),
		html.EscapeString(q.Get("scope")),
		html.EscapeString(q.Get("code_challenge")),
		html.EscapeString(q.Get("code_challenge_method")),
	)
}

func (s *server) handleAuthorizeSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	email := r.FormValue("email")
	groups := splitCSV(r.FormValue("groups"))
	code := randomToken()

	s.mu.Lock()
	s.authReqs[code] = &authRequest{
		clientID:            r.FormValue("client_id"),
		redirectURI:         r.FormValue("redirect_uri"),
		state:               r.FormValue("state"),
		codeChallenge:       r.FormValue("code_challenge"),
		codeChallengeMethod: r.FormValue("code_challenge_method"),
		claims: map[string]any{
			"email":  email,
			"userid": email,
			"groups": groups,
			"roles":  groups,
		},
		expiresAt: time.Now().Add(2 * time.Minute),
	}
	s.mu.Unlock()

	redirect := r.FormValue("redirect_uri")
	u, err := url.Parse(redirect)
	if err != nil {
		http.Error(w, "bad redirect_uri", http.StatusBadRequest)
		return
	}
	q := u.Query()
	q.Set("code", code)
	if st := r.FormValue("state"); st != "" {
		q.Set("state", st)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (s *server) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	grantType := r.FormValue("grant_type")

	var claims map[string]any
	var clientID string

	switch grantType {
	case "authorization_code":
		code := r.FormValue("code")
		s.mu.Lock()
		req, ok := s.authReqs[code]
		if ok {
			delete(s.authReqs, code)
		}
		s.mu.Unlock()
		if !ok || time.Now().After(req.expiresAt) {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		// PKCE verification (S256), matching how a typical SPA-flavoured
		// OIDC client library drives the authorization_code grant.
		if req.codeChallenge != "" {
			verifier := r.FormValue("code_verifier")
			sum := sha256.Sum256([]byte(verifier))
			computed := base64.RawURLEncoding.EncodeToString(sum[:])
			if computed != req.codeChallenge {
				http.Error(w, `{"error":"invalid_grant","error_description":"PKCE verification failed"}`, http.StatusBadRequest)
				return
			}
		}
		claims = req.claims
		clientID = req.clientID

	case "client_credentials":
		clientID = r.FormValue("client_id")
		claims = map[string]any{
			"sub":    clientID,
			"userid": clientID,
			"groups": []string{"m2m"},
			"roles":  []string{"m2m"},
		}

	default:
		http.Error(w, `{"error":"unsupported_grant_type"}`, http.StatusBadRequest)
		return
	}

	audience := envOrDefault("MOCK_OIDC_AUDIENCE", clientID)
	accessToken, err := s.signJWT(claims, audience, 1*time.Hour)
	if err != nil {
		http.Error(w, "token signing failed", http.StatusInternalServerError)
		return
	}
	idToken, err := s.signJWT(claims, clientID, 1*time.Hour)
	if err != nil {
		http.Error(w, "token signing failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"access_token": accessToken,
		"id_token":     idToken,
		"token_type":   "Bearer",
		"expires_in":   3600,
	})
}

func (s *server) handleUserinfo(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	claims, err := decodeClaimsUnsafe(strings.TrimPrefix(auth, "Bearer "))
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	writeJSON(w, claims)
}

func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if redirect := r.URL.Query().Get("post_logout_redirect_uri"); redirect != "" {
		http.Redirect(w, r, redirect, http.StatusFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// signJWT builds and signs an RS256 JWT with standard registered claims plus
// whatever custom claims are supplied. Hand-rolled with the stdlib (no
// third-party JWT library) since this program has no other dependencies.
func (s *server) signJWT(customClaims map[string]any, audience string, ttl time.Duration) (string, error) {
	header := map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid}
	now := time.Now()
	claims := map[string]any{
		"iss": s.issuer,
		"aud": audience,
		"iat": now.Unix(),
		"exp": now.Add(ttl).Unix(),
	}
	for k, v := range customClaims {
		claims[k] = v
	}
	if _, ok := claims["sub"]; !ok {
		if email, ok := claims["email"].(string); ok {
			claims["sub"] = email
		}
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)

	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func decodeClaimsUnsafe(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func randomToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
