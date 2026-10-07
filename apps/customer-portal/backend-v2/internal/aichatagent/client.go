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

// Package aichatagent is the HTTP client for the upstream AI chat agent — a
// separate Python service (not entity-service) that powers the customer
// portal's AI chat feature: case classification, chat responses, and KB
// article recommendations.
package aichatagent

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

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// tokenFetchTimeout is the HTTP client timeout for token-endpoint requests.
var tokenFetchTimeout = 10 * time.Second

// maxResponseBodyBytes bounds how much of an AI chat agent response this
// client will read into memory.
const maxResponseBodyBytes = 10 << 20 // 10 MiB

func noRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

type ctxKey string

const correlationIDKey ctxKey = "x-csm-correlation-id" // #nosec G101 -- context map key, not a credential

// WithCorrelationID returns a copy of ctx carrying the correlation ID to be
// forwarded as X-CSM-Correlation-ID on every outgoing AI chat agent request.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

func correlationIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(correlationIDKey).(string)
	return v
}

// Config holds the configuration for the AI chat agent client.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client is an HTTP client for the upstream AI chat agent, authenticated via
// the OAuth2 client credentials grant.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client that authenticates against the AI chat agent
// using the OAuth2 client credentials grant type.
func NewClient(cfg Config) *Client {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}

	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: tokenFetchTimeout, CheckRedirect: noRedirect})

	// clientcredentials.Config.Client's returned *http.Client and its
	// Transport must not be mutated (see the package doc comment) — wrap its
	// Transport in a fresh http.Client we own instead of setting fields
	// directly on the returned one.
	oauthClient := cc.Client(tokenCtx)
	httpClient := &http.Client{
		Transport:     oauthClient.Transport,
		Timeout:       60 * time.Second,
		CheckRedirect: noRedirect,
	}

	return &Client{
		http:    httpClient,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("aichatagent: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if id := correlationIDFromContext(ctx); id != "" {
		req.Header.Set("X-CSM-Correlation-ID", id)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("aichatagent: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxResponseBodyBytes+1)
	respBody, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("aichatagent: read response body: %w", err)
	}
	if len(respBody) > maxResponseBodyBytes {
		return nil, fmt.Errorf("aichatagent: %s %s: response body exceeds %d bytes", method, path, maxResponseBodyBytes)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, apierror.NewUpstreamError(resp.StatusCode, respBody)
	}

	return respBody, nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	body, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("aichatagent: decode response for GET %s: %w", path, err)
	}
	return nil
}

func (c *Client) postJSON(ctx context.Context, path string, reqBody, out any) error {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("aichatagent: encode request for POST %s: %w", path, err)
	}
	body, err := c.do(ctx, http.MethodPost, path, payload)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("aichatagent: decode response for POST %s: %w", path, err)
	}
	return nil
}

// CreateCaseClassification calls POST /case_classification.
func (c *Client) CreateCaseClassification(ctx context.Context, req CaseClassificationPayload) (CaseClassificationResponse, error) {
	var out CaseClassificationResponse
	err := c.postJSON(ctx, "/case_classification", req, &out)
	return out, err
}

// CreateChat calls POST /chat.
//
// The agent can put its <thinking> reasoning inside the answer. The handlers
// persist this reply as a conversation comment, hand it to the recommender and
// return it to the browser, so it is removed here, once, before any of them see
// it (see StripThinkingBlocks).
func (c *Client) CreateChat(ctx context.Context, req ChatPayload) (ChatResponse, error) {
	var out ChatResponse
	err := c.postJSON(ctx, "/chat", req, &out)
	if err == nil {
		out.Message = StripThinkingBlocks(out.Message)
	}
	return out, err
}

// GetRecommendations calls POST /recommendations.
func (c *Client) GetRecommendations(ctx context.Context, req RecommendationRequest) (RecommendationResponse, error) {
	var out RecommendationResponse
	err := c.postJSON(ctx, "/recommendations", req, &out)
	return out, err
}

// GetConversationSummary calls GET /chat/summary/{projectId}/{conversationId}.
func (c *Client) GetConversationSummary(ctx context.Context, projectID, conversationID string) (ConversationSummaryResponse, error) {
	var out ConversationSummaryResponse
	err := c.getJSON(ctx, "/chat/summary/"+url.PathEscape(projectID)+"/"+url.PathEscape(conversationID), &out)
	return out, err
}
