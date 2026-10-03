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
	"context"
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strings"
	"sync"
	"time"

	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// keyLen must match alerts-core's internal/auth.KeyLen: both read the same integration_users rows.
const keyLen = 32

// minIterations/maxIterations bound a row's PBKDF2 iterations, so a bad row can't burn CPU per guess.
const (
	minIterations = 1_000
	maxIterations = 200_000
)

// IntegrationUsers checks webhooks against integration_users, caching verified credentials for cacheTTL.
type IntegrationUsers struct {
	pool     *pgxpool.Pool
	timeout  time.Duration
	cacheTTL time.Duration

	mu    sync.RWMutex
	cache map[string]cacheEntry
}

// cacheEntry holds a verified secret's SHA-256 (never the secret), expiring by the row's expires_at.
type cacheEntry struct {
	digest  [32]byte
	expires time.Time
}

// NewIntegrationUsers wraps pool for read-only checks; a zero cacheTTL disables caching.
func NewIntegrationUsers(pool *pgxpool.Pool, queryTimeout, cacheTTL time.Duration) *IntegrationUsers {
	return &IntegrationUsers{
		pool:     pool,
		timeout:  queryTimeout,
		cacheTTL: cacheTTL,
		cache:    make(map[string]cacheEntry),
	}
}

// Authenticate accepts an enabled, unexpired user with a matching secret; one credential suits every source.
func (a *IntegrationUsers) Authenticate(r *http.Request, _ string) error {
	username, secret, ok := parseCredentials(r)
	if !ok {
		return ErrUnauthorized
	}
	if a.cachedHit(username, secret) {
		return nil
	}

	ctx, cancel := context.WithTimeout(r.Context(), a.timeout)
	defer cancel()

	var (
		hash, salt string
		iterations int
		enabled    bool
		expiresAt  time.Time
	)
	err := a.pool.QueryRow(ctx,
		`SELECT secret_hash, salt, iterations, enabled, expires_at FROM integration_users WHERE username = $1`,
		username,
	).Scan(&hash, &salt, &iterations, &enabled, &expiresAt)
	if err != nil || !enabled {
		return ErrUnauthorized
	}
	if iterations < minIterations || iterations > maxIterations {
		return ErrUnauthorized
	}
	// expires_at defaults to the Unix epoch in the schema, not NULL, so "at or before epoch" means unset.
	if !expiresAt.IsZero() && expiresAt.After(time.Unix(0, 0)) && time.Now().After(expiresAt) {
		return ErrUnauthorized
	}
	if !verifySecret(secret, salt, hash, iterations) {
		return ErrUnauthorized
	}
	a.remember(username, secret, expiresAt)
	return nil
}

// cachedHit reports whether username's cached digest matches secret and is still fresh.
func (a *IntegrationUsers) cachedHit(username, secret string) bool {
	if a.cacheTTL <= 0 {
		return false
	}
	a.mu.RLock()
	e, ok := a.cache[username]
	a.mu.RUnlock()
	if !ok || time.Now().After(e.expires) {
		return false
	}
	got := sha256.Sum256([]byte(secret))
	return subtle.ConstantTimeCompare(got[:], e.digest[:]) == 1
}

// remember caches secret capped at rowExpiresAt; a disable or rotation still takes up to cacheTTL.
func (a *IntegrationUsers) remember(username, secret string, rowExpiresAt time.Time) {
	if a.cacheTTL <= 0 {
		return
	}
	expires := time.Now().Add(a.cacheTTL)
	if !rowExpiresAt.IsZero() && rowExpiresAt.After(time.Unix(0, 0)) && rowExpiresAt.Before(expires) {
		expires = rowExpiresAt
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cache[username] = cacheEntry{
		digest:  sha256.Sum256([]byte(secret)),
		expires: expires,
	}
}

// verifySecret recomputes the PBKDF2-HMAC-SHA256 hash and compares in constant time.
func verifySecret(secret, saltB64, hashB64 string, iterations int) bool {
	salt, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil {
		return false
	}
	want, err := base64.StdEncoding.DecodeString(hashB64)
	if err != nil {
		return false
	}
	// Stdlib crypto/pbkdf2, same algorithm as alerts-core's x/crypto, so no new dependency.
	got, err := pbkdf2.Key(sha256.New, secret, salt, iterations, keyLen)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// parseCredentials reads Bearer base64("user:secret") or Basic; the scheme is case-insensitive (RFC 7235).
func parseCredentials(r *http.Request) (username, secret string, ok bool) {
	const prefix = "bearer "
	if h := r.Header.Get("Authorization"); len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(h[len(prefix):]))
		if err != nil {
			return "", "", false
		}
		username, secret, found := strings.Cut(string(decoded), ":")
		if !found || username == "" || secret == "" {
			return "", "", false
		}
		return username, secret, true
	}
	username, secret, ok = r.BasicAuth()
	if !ok || username == "" || secret == "" {
		return "", "", false
	}
	return username, secret, true
}
