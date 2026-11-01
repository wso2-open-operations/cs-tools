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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

// Package entity is the knowledge portal backend's client for entity-service.
// It is read-only by construction: the only calls it can make are the
// knowledge-base list, the article search and the article get. Response types
// declare only the fields this API may serve, so identity and workflow fields
// (author, reviewer, source case, rejection comment, version lineage) are
// dropped at decode time and cannot reach a handler.
package entity

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

	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/apierror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// tokenFetchTimeout is the HTTP client timeout for token-endpoint requests.
// Overridden in tests to keep them fast.
var tokenFetchTimeout = 10 * time.Second

// StatePublished is the only article state this service ever requests or serves.
const StatePublished = "published"

const (
	// maxResponseBytes caps a successful entity-service response body.
	maxResponseBytes = 8 << 20
	// maxErrBody caps how much of an error body is kept on apierror.Error.Body.
	// It is for server-side logs only and is never relayed to callers.
	maxErrBody = 512

	correlationIDHeader = "X-CSM-Correlation-ID"
	defaultTimeout      = 10 * time.Second
)

type ctxKey string

const correlationIDKey ctxKey = "x-csm-correlation-id" // #nosec G101 -- context map key, not a credential

// WithCorrelationID returns a copy of ctx carrying the correlation ID to be
// forwarded as X-CSM-Correlation-ID on every outgoing entity-service request.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

func correlationIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(correlationIDKey).(string)
	return v
}

// KnowledgeBase is the subset of an entity-service knowledge base this API serves.
type KnowledgeBase struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Active bool   `json:"active"`
}

// KBArticle is the subset of an entity-service KB article this API serves.
// State is kept only so callers can enforce published-only; it is not exposed.
type KBArticle struct {
	ID              string     `json:"id"`
	KnowledgeBaseID string     `json:"knowledgeBaseId"`
	Title           string     `json:"title"`
	Body            string     `json:"body"`
	State           string     `json:"state"`
	Number          *string    `json:"number"`
	CreatedOn       time.Time  `json:"createdOn"`
	UpdatedOn       time.Time  `json:"updatedOn"`
	PublishedOn     *time.Time `json:"publishedOn"`
}

// SearchRequest is the article search body. States is always set by the
// handler to the published state; there is no field for a caller to widen it.
// entity-service decodes strictly, so this struct must carry only fields its
// own request type has.
type SearchRequest struct {
	KnowledgeBaseID string     `json:"knowledgeBaseId,omitempty"`
	States          []string   `json:"states,omitempty"`
	SearchQuery     string     `json:"searchQuery,omitempty"`
	Pagination      Pagination `json:"pagination"`
}

// Pagination is entity-service's limit/offset pair.
type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// SearchResponse is the article search result.
type SearchResponse struct {
	Articles []KBArticle `json:"articles"`
	Total    int         `json:"total"`
	Limit    int         `json:"limit"`
	Offset   int         `json:"offset"`
	HasMore  bool        `json:"hasMore"`
}

// Config holds the configuration for the entity-service client.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	// Timeout is the per-request timeout. Zero selects defaultTimeout.
	Timeout time.Duration
}

// Client is an HTTP client for entity-service, authenticated via the OAuth2
// client credentials grant. This is a
// service identity: no end-user token exists on this public API, so none is
// ever sent. Tokens are acquired and refreshed automatically.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient constructs a Client that authenticates against entity-service using
// the OAuth2 client credentials grant.
func NewClient(cfg Config) *Client {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: tokenFetchTimeout})
	return newClient(cc.Client(tokenCtx), cfg.BaseURL, cfg.Timeout)
}

// newClient finishes construction around any *http.Client (tests pass a plain one).
func newClient(httpClient *http.Client, baseURL string, timeout time.Duration) *Client {
	httpClient.Timeout = timeout
	if httpClient.Timeout <= 0 {
		httpClient.Timeout = defaultTimeout
	}
	// Never follow a redirect: the OAuth2 transport would replay the bearer token
	// to whatever host the response names. A 3xx surfaces as a non-2xx apierror.
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &Client{http: httpClient, baseURL: strings.TrimRight(baseURL, "/")}
}

// ListKnowledgeBases returns every knowledge base entity-service holds,
// including deactivated ones; callers filter.
func (c *Client) ListKnowledgeBases(ctx context.Context) ([]KnowledgeBase, error) {
	var out struct {
		KnowledgeBases []KnowledgeBase `json:"knowledgeBases"`
	}
	if err := c.do(ctx, http.MethodGet, "/knowledge-bases", nil, &out); err != nil {
		return nil, err
	}
	return out.KnowledgeBases, nil
}

// SearchKBArticles runs POST /kb-articles/search.
func (c *Client) SearchKBArticles(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return SearchResponse{}, fmt.Errorf("entity: marshal search request: %w", err)
	}
	var out SearchResponse
	if err := c.do(ctx, http.MethodPost, "/kb-articles/search", body, &out); err != nil {
		return SearchResponse{}, err
	}
	return out, nil
}

// GetKBArticle runs GET /kb-articles/{id}. The id is escaped as a single path
// segment, so a value containing "/", "?", "#" or ".." cannot change the
// upstream path; callers should still validate the id first.
func (c *Client) GetKBArticle(ctx context.Context, id string) (KBArticle, error) {
	var out KBArticle
	if err := c.do(ctx, http.MethodGet, "/kb-articles/"+url.PathEscape(id), nil, &out); err != nil {
		return KBArticle{}, err
	}
	return out, nil
}

// do executes an authenticated request and decodes the JSON response into dst.
// A non-2xx status becomes an *apierror.Error.
func (c *Client) do(ctx context.Context, method, path string, body []byte, dst any) error {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("entity: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if id := correlationIDFromContext(ctx); id != "" {
		req.Header.Set(correlationIDHeader, id)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("entity: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		excerpt, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrBody))
		return &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("entity: read response body: %w", err)
	}
	if len(respBody) > maxResponseBytes {
		return fmt.Errorf("entity: %s %s: response exceeds %d bytes", method, path, maxResponseBytes)
	}
	if err := json.Unmarshal(respBody, dst); err != nil {
		return fmt.Errorf("entity: decode %s %s: %w", method, path, err)
	}
	return nil
}
