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

package plgvalidation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/httpsec"
)

const (
	tokenFetchTimeout = 10 * time.Second
	callTimeout       = 60 * time.Second
)

// Config holds the entity-service client's configuration.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client calls entity-service's registration validation routes.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client authenticated via the client credentials grant.
func NewClient(cfg Config) (*Client, error) {
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("plgvalidation: token URL: %w", err)
	}
	if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("plgvalidation: base URL: %w", err)
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

// QueueItem is one pairing to validate. Ids only.
type QueueItem struct {
	OrgPlatformID       string     `json:"orgPlatformId"`
	CompanyID           string     `json:"companyId"`
	RegisteredOn        time.Time  `json:"registeredOn"`
	EmailClassification *string    `json:"emailClassification"`
	LastValidatedOn     *time.Time `json:"lastValidatedOn"`
}

// Summary is the stored activity summary.
type Summary struct {
	WindowStart  time.Time      `json:"windowStart"`
	WindowEnd    time.Time      `json:"windowEnd"`
	FirstEventOn *time.Time     `json:"firstEventOn"`
	LastEventOn  *time.Time     `json:"lastEventOn"`
	EventCount   int            `json:"eventCount"`
	ActionCounts map[string]int `json:"actionCounts"`
	FurthestStep *string        `json:"furthestStep"`
}

// Value sets one task by code; no value field clears it.
type Value struct {
	Code         string    `json:"code"`
	BoolValue    *bool     `json:"boolValue,omitempty"`
	NumberValue  *float64  `json:"numberValue,omitempty"`
	TextValue    *string   `json:"textValue,omitempty"`
	CheckedCodes *[]string `json:"checkedCodes,omitempty"`
}

// ApplyRequest is one pairing's write.
type ApplyRequest struct {
	Summary Summary `json:"summary"`
	Values  []Value `json:"values"`
	Abandon bool    `json:"abandon"`
}

// ApplyResult is what entity-service wrote.
type ApplyResult struct {
	Attached bool     `json:"attached"`
	Written  []string `json:"written"`
	Skipped  []struct {
		Code   string `json:"code"`
		Reason string `json:"reason"`
	} `json:"skipped"`
	Abandoned bool `json:"abandoned"`
}

// Queue lists a product's pairings registered after the given time.
func (c *Client) Queue(ctx context.Context, product string, registeredAfter time.Time, limit int) ([]QueueItem, error) {
	var out struct {
		Items []QueueItem `json:"items"`
	}
	err := c.do(ctx, http.MethodPost, "/plg/registrations/validation-queue",
		map[string]any{"productCode": product, "registeredAfter": registeredAfter, "limit": limit}, &out)
	return out.Items, err
}

// Apply writes one pairing's summary and values.
func (c *Client) Apply(ctx context.Context, orgPlatformID string, req ApplyRequest) (ApplyResult, error) {
	var out ApplyResult
	err := c.do(ctx, http.MethodPut, "/plg/pairings/"+url.PathEscape(orgPlatformID)+"/registration-validation", req, &out)
	return out, err
}

func (c *Client) do(ctx context.Context, method, path string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("plgvalidation: encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("plgvalidation: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("plgvalidation: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("plgvalidation: read response body: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &apierror.Error{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("plgvalidation: decode response: %w", err)
	}
	return nil
}
