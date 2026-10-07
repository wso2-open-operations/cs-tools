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

// Package kbclient is a minimal client for this repo's own entity-service,
// used only by kbdraftengine: fetching a closed case's comments, and
// creating the resulting draft KB article(s). Deliberately separate from
// internal/entity (which implements exactly POST /users/search, for a
// different consumer) rather than extended, matching this service's
// existing one-client-per-concern convention. Points at the same
// entity-service as internal/entity, reusing the same
// CUSTOMER_ENTITY_BASE_URL/OAUTH2_* configuration -- no new env vars needed
// for connectivity.
package kbclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

var tokenFetchTimeout = 10 * time.Second

// Config mirrors internal/entity.CustomerEntityConfig's shape -- same
// shared OAuth2 app, different BaseURL/Scopes env vars if this ever needs
// its own.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

type Client struct {
	http    *http.Client
	baseURL string
}

func New(cfg Config) *Client {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: tokenFetchTimeout})
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 25 * time.Second

	return &Client{http: httpClient, baseURL: strings.TrimRight(cfg.BaseURL, "/")}
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("kbclient: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kbclient: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("kbclient: read response body: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	return respBody, nil
}

// Comment is the subset of entity-service's CaseComment kbdraftengine
// needs -- just enough text to feed the draft generator.
type Comment struct {
	Content string `json:"content"`
}

// SearchCaseComments returns every comment on the given case, unpaginated
// up to limit -- a closed case is expected to have a bounded, human-scale
// number of comments, not an unbounded stream.
func (c *Client) SearchCaseComments(ctx context.Context, caseID string, limit, offset int) ([]Comment, error) {
	reqBody, err := json.Marshal(map[string]any{
		"pagination": map[string]int{"limit": limit, "offset": offset},
	})
	if err != nil {
		return nil, fmt.Errorf("kbclient: encode SearchCaseComments request: %w", err)
	}
	respBody, err := c.do(ctx, http.MethodPost, "/cases/"+caseID+"/comments/search", reqBody)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Comments []Comment `json:"comments"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("kbclient: decode SearchCaseComments response: %w", err)
	}
	return parsed.Comments, nil
}

// CreateKBArticleRequest mirrors entity-service's domain.CreateKBArticleRequest.
type CreateKBArticleRequest struct {
	KnowledgeBaseID string  `json:"knowledgeBaseId"`
	Title           string  `json:"title"`
	Body            string  `json:"body"`
	AuthorID        string  `json:"authorId"`
	TeamKey         *string `json:"teamKey,omitempty"`
}

// CreateKBArticle creates a new draft article via POST /kb-articles.
func (c *Client) CreateKBArticle(ctx context.Context, req CreateKBArticleRequest) error {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("kbclient: encode CreateKBArticle request: %w", err)
	}
	_, err = c.do(ctx, http.MethodPost, "/kb-articles", reqBody)
	return err
}

// ListKnowledgeBases mirrors entity-service's GET /knowledge-bases.
type KnowledgeBase struct {
	ID        string `json:"id"`
	ProductID string `json:"productId"`
	Name      string `json:"name"`
	IsActive  bool   `json:"isActive"`
}

func (c *Client) ListKnowledgeBases(ctx context.Context) ([]KnowledgeBase, error) {
	respBody, err := c.do(ctx, http.MethodGet, "/knowledge-bases", nil)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		KnowledgeBases []KnowledgeBase `json:"knowledgeBases"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("kbclient: decode ListKnowledgeBases response: %w", err)
	}
	return parsed.KnowledgeBases, nil
}
