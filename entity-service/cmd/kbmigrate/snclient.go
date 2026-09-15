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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// httpDoer is the minimal http.Client surface this package depends on, so
// tests can substitute a fake round tripper instead of hitting the network.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// snClient reads ServiceNow Table API data via OAuth2 client_credentials.
// This is a from-scratch implementation for this one-shot backfill -- it
// deliberately does not import or depend on digiops-cs/operations/
// csm-sync-service's Go code (a separate, continuously-syncing tool with
// its own schema), even though the OAuth/pagination shape is conceptually
// similar.
//
// IMPORTANT: every request omits sysparm_display_value, so reference fields
// come back as raw sys_id strings, never display names -- required for
// sysIDToUUID and the version-chain walk to work at all.
type snClient struct {
	instanceBaseURL string // e.g. https://wso2sndev.service-now.com
	clientID        string
	clientSecret    string
	httpClient      httpDoer

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

// newSNClient constructs a client against the given SN instance base URL
// (no trailing slash) using OAuth2 client_credentials.
func newSNClient(instanceBaseURL, clientID, clientSecret string, httpClient httpDoer) *snClient {
	return &snClient{
		instanceBaseURL: strings.TrimRight(instanceBaseURL, "/"),
		clientID:        clientID,
		clientSecret:    clientSecret,
		httpClient:      httpClient,
	}
}

// tokenResponse is the OAuth2 client_credentials token endpoint's response body.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// authenticate fetches (or reuses a still-valid) OAuth2 access token via the
// client_credentials grant against {instance}/oauth_token.do.
func (c *snClient) authenticate(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.expiresAt) {
		return c.accessToken, nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.instanceBaseURL+"/oauth_token.do", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed: status %d: %s", resp.StatusCode, string(body))
	}

	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("token response carried no access_token")
	}

	c.accessToken = tok.AccessToken
	// Refresh a little early to avoid a request racing token expiry mid-run.
	expiresIn := tok.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 1800 // SN's default OAuth token lifetime; used only if the endpoint omits expires_in
	}
	c.expiresAt = time.Now().Add(time.Duration(expiresIn)*time.Second - 30*time.Second)

	return c.accessToken, nil
}

// tableAPIPageSize bounds each Table API page. Kept well under SN's own hard
// cap so a single slow page doesn't dominate the run.
const tableAPIPageSize = 200

// tableQueryResponse is the Table API's {"result": [...]} envelope. Each
// result row is decoded generically as map[string]string, since every field
// this tool needs (booleans, numbers, references, timestamps) round-trips
// through the Table API as a JSON string when sysparm_display_value is not
// requested.
type tableQueryResponse struct {
	Result []map[string]string `json:"result"`
}

// fetchAllRows pages through table via the Table API, ordered and
// keyset-paginated on sys_id (ORDERBYsys_id^sys_id>{lastID}), never
// sysparm_offset -- large SN tables make offset pagination unreliable
// (result drift under concurrent writes) and keyset pagination is the
// pattern this backfill standardizes on throughout.
//
// fields lists the exact sysparm_fields to request; sysparm_display_value is
// never set, so every reference field comes back as a raw sys_id.
func (c *snClient) fetchAllRows(ctx context.Context, table string, fields []string) ([]map[string]string, error) {
	var all []map[string]string
	lastID := ""

	for {
		page, err := c.fetchPage(ctx, table, fields, lastID)
		if err != nil {
			return nil, fmt.Errorf("fetch %s page after sys_id %q: %w", table, lastID, err)
		}
		all = append(all, page...)
		if len(page) < tableAPIPageSize {
			return all, nil
		}
		lastID = page[len(page)-1]["sys_id"]
		if lastID == "" {
			return nil, fmt.Errorf("fetch %s: page returned rows with no sys_id, cannot page further", table)
		}
	}
}

func (c *snClient) fetchPage(ctx context.Context, table string, fields []string, afterSysID string) ([]map[string]string, error) {
	token, err := c.authenticate(ctx)
	if err != nil {
		return nil, fmt.Errorf("authenticate: %w", err)
	}

	query := "ORDERBYsys_id"
	if afterSysID != "" {
		query = fmt.Sprintf("ORDERBYsys_id^sys_id>%s", afterSysID)
	}

	q := url.Values{}
	q.Set("sysparm_query", query)
	q.Set("sysparm_limit", strconv.Itoa(tableAPIPageSize))
	q.Set("sysparm_fields", strings.Join(fields, ","))
	// sysparm_display_value deliberately never set -- see snClient's doc comment.

	reqURL := fmt.Sprintf("%s/api/now/table/%s?%s", c.instanceBaseURL, table, q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var parsed tableQueryResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return parsed.Result, nil
}
