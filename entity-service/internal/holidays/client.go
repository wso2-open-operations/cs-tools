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

// Package holidays holds the two read-only clients the Team Schedule's public
// holidays need, both the same calls the WSO2 leave app
// (people-ops-suite apps/leave-app) makes:
//
//   - CalendarClient: the shared calendar-events service,
//     GET /holidays?countries=<country>&startTimeMin=..&startTimeMax=..
//     (leave app calendar_events:fetchHolidaysForCountry);
//   - EmployeeClient: the HR entity GraphQL service, employee(email){location}
//     (leave app employee:getEmployeeLocation).
//
// Each authenticates with an OAuth2 client-credentials grant (a Choreo
// connection), like internal/salesentity and internal/scim.
package holidays

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultTimeout   = 15 * time.Second
	tokenExpirySlack = 30 * time.Second
	// maxResponseBytes bounds what is read from either service.
	maxResponseBytes = 4 << 20
)

// Credentials is one Choreo connection's OAuth2 client credentials.
type Credentials struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
}

// errUnauthorized marks a 401 from the API, so the caller refreshes the token
// once and retries.
var errUnauthorized = errors.New("holidays: request unauthorized")

// tokenSource fetches and caches a client-credentials access token.
type tokenSource struct {
	creds      Credentials
	httpClient *http.Client

	mu     sync.Mutex
	token  string
	expiry time.Time
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// accessToken returns the cached bearer token while it is still valid, else
// fetches a new one from the token endpoint with the client credentials grant.
func (t *tokenSource) accessToken(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.token != "" && time.Now().Add(tokenExpirySlack).Before(t.expiry) {
		return t.token, nil
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", t.creds.ClientID)
	form.Set("client_secret", t.creds.ClientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.creds.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("holidays: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("holidays: token endpoint unavailable: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("holidays: read token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("holidays: token endpoint returned %d", resp.StatusCode)
	}
	var tr tokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return "", fmt.Errorf("holidays: parse token response: %w", err)
	}
	if tr.AccessToken == "" {
		return "", errors.New("holidays: token endpoint returned an empty access_token")
	}
	t.token = tr.AccessToken
	if tr.ExpiresIn > 0 {
		t.expiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	} else {
		t.expiry = time.Now().Add(time.Hour)
	}
	return t.token, nil
}

// invalidate drops the cached token so the next call fetches a new one.
func (t *tokenSource) invalidate() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.token = ""
	t.expiry = time.Time{}
}

// do sends req with a bearer token and returns the 2xx body. A 401 refreshes
// the token and retries once; build must therefore return a fresh request.
func (t *tokenSource) do(ctx context.Context, what string, build func() (*http.Request, error)) ([]byte, error) {
	body, err := t.doOnce(ctx, what, build)
	if errors.Is(err, errUnauthorized) {
		t.invalidate()
		body, err = t.doOnce(ctx, what, build)
	}
	return body, err
}

// doOnce sends one request built by build with a bearer token and returns the
// 2xx body; a 401 is reported as errUnauthorized so the caller can retry.
func (t *tokenSource) doOnce(ctx context.Context, what string, build func() (*http.Request, error)) ([]byte, error) {
	token, err := t.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	req, err := build()
	if err != nil {
		return nil, fmt.Errorf("holidays: build %s request: %w", what, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("holidays: %s unavailable: %w", what, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("holidays: read %s response: %w", what, err)
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, errUnauthorized
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return nil, fmt.Errorf("holidays: %s returned %d", what, resp.StatusCode)
	}
	return raw, nil
}

// newTokenSource returns a tokenSource that fetches tokens with the given
// credentials over an HTTP client with the default timeout.
func newTokenSource(creds Credentials) *tokenSource {
	return &tokenSource{creds: creds, httpClient: &http.Client{Timeout: defaultTimeout}}
}
