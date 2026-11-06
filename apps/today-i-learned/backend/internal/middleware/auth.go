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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// jwtAssertionHeader carries the caller's token. The API gateway in front of
// this service sets it; nothing else is read.
const jwtAssertionHeader = "x-jwt-assertion"

const msgUnauthorized = "Your session isn't valid or has expired. Sign in again."

// AuthConfig is how incoming tokens are checked.
type AuthConfig struct {
	// Issuer is the expected "iss" claim.
	Issuer string
	// Audiences, when not empty, lists the client ids a token may be issued
	// to; it has to carry at least one of them (see User.ClientIDs).
	Audiences []string
	// ModeratorGroups are the groups whose members may edit or delete any
	// entry. A user in none of them can change only their own.
	ModeratorGroups []string
	// ClockSkew is the leeway allowed on time-based claims.
	ClockSkew time.Duration
}

// User is the signed-in caller, from a verified token.
type User struct {
	// Subject is the token's "sub": an opaque id, safe to log.
	Subject     string
	Email       string
	DisplayName string
	IsModerator bool
	// ClientIDs are the applications the token was issued to: its "aud"
	// values plus its "client_id" (or "azp"). They come from the signed
	// token itself, so a caller can't claim to be a different application.
	ClientIDs []string
}

// IssuedTo reports whether the token was issued to the given client id.
func (u *User) IssuedTo(clientID string) bool {
	return clientID != "" && slices.Contains(u.ClientIDs, clientID)
}

type userKey struct{}

// UserFromContext returns the signed-in user, or nil outside Auth.
func UserFromContext(ctx context.Context) *User {
	u, _ := ctx.Value(userKey{}).(*User)
	return u
}

// WithUser returns ctx carrying u, for handler tests.
func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userKey{}, u)
}

type claims struct {
	Email      string     `json:"email"`
	Name       string     `json:"name"`
	GivenName  string     `json:"given_name"`
	FamilyName string     `json:"family_name"`
	ClientID   string     `json:"client_id"`
	AZP        string     `json:"azp"`
	Groups     stringList `json:"groups"`
	jwt.RegisteredClaims
}

// stringList decodes a claim Asgardeo sends as a bare string when it holds
// one value and as an array when it holds several, as it does for "groups".
type stringList []string

func (l *stringList) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*l = nil
		for _, part := range strings.Split(one, ",") {
			if part = strings.TrimSpace(part); part != "" {
				*l = append(*l, part)
			}
		}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return fmt.Errorf("claim must be a string or an array of strings: %w", err)
	}
	*l = many
	return nil
}

// NewKeyfunc loads the signing keys published at jwksURL and keeps them
// refreshed. It fails when the keys can't be loaded, so a misconfigured
// service stops at startup instead of rejecting every request.
func NewKeyfunc(ctx context.Context, jwksURL string) (jwt.Keyfunc, error) {
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &x5cStrippingTransport{base: http.DefaultTransport},
	}
	k, err := keyfunc.NewDefaultOverrideCtx(ctx, []string{jwksURL}, keyfunc.Override{Client: client})
	if err != nil {
		return nil, fmt.Errorf("auth: load signing keys from %s: %w", jwksURL, err)
	}
	return k.Keyfunc, nil
}

// Auth verifies the token on every request except GET /health and puts the
// User in the request context. A missing or invalid token gets a 401.
func Auth(cfg AuthConfig, keyFunc jwt.Keyfunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}
			token := r.Header.Get(jwtAssertionHeader)
			if token == "" {
				// Behind the gateway, a request only arrives without one when
				// the gateway isn't passing the caller's security context on.
				slog.WarnContext(r.Context(), "auth: no token in "+jwtAssertionHeader)
				writeError(w, http.StatusUnauthorized, msgUnauthorized)
				return
			}
			user, err := verify(token, cfg, keyFunc)
			if err != nil {
				slog.WarnContext(r.Context(), "auth: token rejected", "err", err)
				writeError(w, http.StatusUnauthorized, msgUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
		})
	}
}

func verify(token string, cfg AuthConfig, keyFunc jwt.Keyfunc) (*User, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, keyFunc,
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(cfg.Issuer),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(cfg.ClockSkew),
	)
	if err != nil {
		return nil, err
	}
	if c.Email == "" {
		return nil, errors.New("token has no email claim")
	}
	// Every log line for the request names the caller by "sub", so a token
	// without one would leave the audit log unattributed.
	if c.Subject == "" {
		return nil, errors.New("token has no subject claim")
	}

	clientIDs := append([]string(nil), c.Audience...)
	if c.ClientID != "" {
		clientIDs = append(clientIDs, c.ClientID)
	} else if c.AZP != "" {
		clientIDs = append(clientIDs, c.AZP)
	}
	if len(cfg.Audiences) > 0 && !slices.ContainsFunc(clientIDs, func(id string) bool { return slices.Contains(cfg.Audiences, id) }) {
		return nil, errors.New("token not issued to an allowed client")
	}

	// Asgardeo's access tokens carry given_name/family_name but no combined
	// name, so those come first.
	name := strings.TrimSpace(c.GivenName + " " + c.FamilyName)
	if name == "" {
		name = c.Name
	}
	if name == "" {
		name = c.Email
	}

	return &User{
		Subject:     c.Subject,
		Email:       c.Email,
		DisplayName: name,
		IsModerator: slices.ContainsFunc(c.Groups, func(g string) bool { return slices.Contains(cfg.ModeratorGroups, g) }),
		ClientIDs:   clientIDs,
	}, nil
}

// x5cStrippingTransport removes the "x5c" certificate chain from each key in
// a JWKS response. Verification needs only the key itself, but the JWKS
// parser also parses "x5c", and Asgardeo publishes certificates with a
// negative serial number, which Go's x509 parser rejects. The parser then
// silently drops that key, and every token signed with it fails to verify.
type x5cStrippingTransport struct {
	base http.RoundTripper
}

func (t *x5cStrippingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return resp, err
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read JWKS response: %w", err)
	}

	var set struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(body, &set); err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}
	for _, key := range set.Keys {
		delete(key, "x5c")
	}
	cleaned, err := json.Marshal(set)
	if err != nil {
		return nil, fmt.Errorf("rewrite JWKS response: %w", err)
	}
	resp.Body = io.NopCloser(bytes.NewReader(cleaned))
	resp.ContentLength = int64(len(cleaned))
	resp.Header.Set("Content-Length", fmt.Sprint(len(cleaned)))
	return resp, nil
}
