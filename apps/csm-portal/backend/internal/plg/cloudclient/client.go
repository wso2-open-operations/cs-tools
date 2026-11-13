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

// Package cloudclient reads an organisation's name and owner email from the
// WSO2 Cloud Directory API (v3.10.1) for the reveal. Responses are personal
// data (Cache-Control: no-store): never cached, stored, logged or put in errors.
package cloudclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxResponseBytes caps one reply. An organisation or user record is small.
const maxResponseBytes = 1 << 20

var (
	// ErrNotFound: no such customer organisation, user or email.
	ErrNotFound = errors.New("cloud api: not found")
	// ErrRateLimited is the gateway's 429 (600 calls a minute).
	ErrRateLimited = errors.New("cloud api: rate limited")
)

// Config holds what the client needs to reach the Cloud Directory API.
type Config struct {
	BaseURL string
	// OrgPath is the org-by-UUID path, with {orgId} where the id goes.
	OrgPath string
	// UserEmailPath is the user-email path, with {userId} where the id goes.
	UserEmailPath string
	Timeout       time.Duration
	// HTTPClient carries the OAuth2 credentials; nil builds a plain client.
	HTTPClient *http.Client
}

// Client talks to the Cloud Directory API.
type Client struct {
	http          *http.Client
	baseURL       string
	orgPath       string
	userEmailPath string
}

// New builds a Client.
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}
	return &Client{
		http:          httpClient,
		baseURL:       strings.TrimRight(cfg.BaseURL, "/"),
		orgPath:       cfg.OrgPath,
		userEmailPath: cfg.UserEmailPath,
	}
}

// Org is what GET /orgs/{orgId} returns.
type Org struct {
	UUID         string `json:"uuid"`
	FriendlyName string `json:"friendlyName"`
	Handle       string `json:"handle"`
}

// UserEmail is what GET /users/{userId}/email returns.
type UserEmail struct {
	UserUUID string `json:"userUuid"`
	OrgUUID  string `json:"orgUuid"`
	Email    string `json:"email"`
}

// GetOrg reads one organisation by its UUID.
func (c *Client) GetOrg(ctx context.Context, orgID string) (*Org, error) {
	var out Org
	path := strings.ReplaceAll(c.orgPath, "{orgId}", url.PathEscape(orgID))
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetUserEmail reads one user's email by their UUID.
func (c *Client) GetUserEmail(ctx context.Context, userID string) (*UserEmail, error) {
	var out UserEmail
	path := strings.ReplaceAll(c.userEmailPath, "{userId}", url.PathEscape(userID))
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// get issues one GET into out; errors carry the call and status, never the body.
func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("cloud api: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cloud api GET: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("cloud api GET: read reply: %w", err)
	}
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode == http.StatusTooManyRequests:
		return ErrRateLimited
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return fmt.Errorf("cloud api GET: status %d", resp.StatusCode)
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return errors.New("cloud api GET: decode reply")
	}
	return nil
}
