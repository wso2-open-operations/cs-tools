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

// Package entity is the flow engine's client for this repo's own
// entity-service, authenticated via the shared OAuth2 client-credentials app
// (docs/architecture.md §17.3 — only FLOW_/ENTITY_ BaseURL and Scopes are
// new; the credentials are shared, not a per-service app). It follows the
// Config / Client / NewClient / do() shape used across the repo's Go clients.
//
// It starts minimal — SearchUsersByEmail, ported verbatim from
// csm-notification-service, is the one proven method. Flow-specific methods
// (GetCase, PatchCase with If-Match, watch-list updates, …) are added here as
// each flow is ported, so the client only ever grows the surface a real flow
// needs — never a speculative passthrough of the whole entity API.
//
// Mutating methods must forward an optimistic-concurrency version as If-Match
// once entity-service exposes one (docs/architecture.md §9); until then a port
// that mutates a case is limited to entities already native in Postgres
// (§20).
package entity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/apierror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// tokenFetchTimeout is the HTTP client timeout for token-endpoint requests.
var tokenFetchTimeout = 10 * time.Second

// requestTimeout bounds each entity-service call.
var requestTimeout = 25 * time.Second

// Config holds the entity-service client configuration. TokenURL/ClientID/
// ClientSecret come from the shared OAUTH2_* app; only BaseURL/Scopes are this
// client's own.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client is an HTTP client for entity-service, authenticated via the OAuth2
// client-credentials grant. Tokens are acquired and refreshed automatically.
//
// NewClient never fails and never contacts the token endpoint, so it is safe
// to construct with a zero-value Config — a missing configuration only
// surfaces as an error on the first call.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client.
func NewClient(cfg Config) *Client {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: tokenFetchTimeout})
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = requestTimeout

	return &Client{
		http:    httpClient,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}
}

// do executes an authenticated request against entity-service and returns the
// raw JSON response body. The upstream error body is deliberately omitted from
// the returned apierror.Error: entity-service requests here filter by recipient
// emails, and an error echoing the offending input back would otherwise leak
// that PII into logs (see the "no recipient emails in logs" convention).
func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("entity: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("entity: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("entity: read response body: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: "response omitted (may contain recipient email addresses)"}
	}
	return respBody, nil
}

// UserRoleInfo is the subset of an entity-service user record needed to resolve
// which portal a recipient belongs to. ID is logged (never the email) so a
// delivery can be traced without raw addresses in logs.
type UserRoleInfo struct {
	ID       string   `json:"id"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
	UserType string   `json:"userType"`
}

type searchUsersByEmailRequest struct {
	Pagination struct {
		Limit int `json:"limit"`
	} `json:"pagination"`
	Filters struct {
		Emails []string `json:"emails"`
	} `json:"filters"`
}

// searchUsersByEmailLimit caps how many emails a single POST /users/search
// call filters on — entity-service's own enforced maximum.
const searchUsersByEmailLimit = 50

// SearchUsersByEmail calls POST /users/search filtered to emails (batching into
// calls of at most searchUsersByEmailLimit each) and returns each matched
// user's roles/userType.
func (c *Client) SearchUsersByEmail(ctx context.Context, emails []string) ([]UserRoleInfo, error) {
	var users []UserRoleInfo
	for start := 0; start < len(emails); start += searchUsersByEmailLimit {
		end := min(start+searchUsersByEmailLimit, len(emails))
		batch, err := c.searchUsersByEmailBatch(ctx, emails[start:end])
		if err != nil {
			return nil, err
		}
		users = append(users, batch...)
	}
	return users, nil
}

func (c *Client) searchUsersByEmailBatch(ctx context.Context, emails []string) ([]UserRoleInfo, error) {
	req := searchUsersByEmailRequest{}
	req.Pagination.Limit = searchUsersByEmailLimit
	req.Filters.Emails = emails

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("entity: encode SearchUsersByEmail request: %w", err)
	}
	respBody, err := c.do(ctx, http.MethodPost, "/users/search", reqBody)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Users []UserRoleInfo `json:"users"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("entity: decode SearchUsersByEmail response: %w", err)
	}
	return parsed.Users, nil
}
