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
package plgsignups

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/httpsec"
)

var tokenFetchTimeout = 10 * time.Second

// callTimeout bounds one ingest or failure call.
const callTimeout = 60 * time.Second

// Config holds the entity-service client's configuration.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client calls entity-service's PLG ingest endpoints.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client authenticated via the client credentials grant.
func NewClient(cfg Config) (*Client, error) {
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("plgsignups: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("plgsignups: base URL: %w", err)
	}
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}
	tokenHTTP := &http.Client{Timeout: tokenFetchTimeout}
	httpsec.RejectInsecureRedirects(tokenHTTP)
	httpClient := cc.Client(context.WithValue(context.Background(), oauth2.HTTPClient, tokenHTTP))
	httpClient.Timeout = callTimeout
	httpsec.RejectInsecureRedirects(httpClient)

	return &Client{http: httpClient, baseURL: strings.TrimRight(cfg.BaseURL, "/")}, nil
}

// Registration is one WSO2 Cloud signup, by id only.
type Registration struct {
	CreatedOn         string `json:"createdOn,omitempty"`
	CompanyID         string `json:"companyId"`
	UserID            string `json:"userId,omitempty"`
	InitiatingProduct string `json:"initiatingProduct"`
}

// Result is what became of one registration.
type Result struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Failure is a signup that could not be landed, recorded by ids only.
type Failure struct {
	EventID    string          `json:"eventId,omitempty"`
	EventType  string          `json:"eventType,omitempty"`
	ReceivedAt string          `json:"receivedAt,omitempty"`
	Payload    json.RawMessage `json:"payload"`
	Failure    string          `json:"failure"`
}

// Ingest lands a batch. Each registration lands on its own, so the results
// must be read: a 2xx does not mean every one landed.
func (c *Client) Ingest(ctx context.Context, regs []Registration) ([]Result, error) {
	var out struct {
		Results []Result `json:"results"`
	}
	if err := c.post(ctx, "/plg/registrations/ingest", map[string]any{"registrations": regs}, &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

// RecordFailure records one failure. entity-service keeps one open row per event id.
func (c *Client) RecordFailure(ctx context.Context, f Failure) error {
	return c.post(ctx, "/plg/ingest-failures", f, nil)
}

func (c *Client) post(ctx context.Context, path string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("plgsignups: encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("plgsignups: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("plgsignups: POST %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("plgsignups: read response body: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &apierror.Error{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("plgsignups: decode response: %w", err)
	}
	return nil
}
