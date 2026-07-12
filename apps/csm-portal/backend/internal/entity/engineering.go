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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// engineeringTokenFetchTimeout is the HTTP client timeout for token-endpoint
// requests. Overridden in tests to keep them fast.
var engineeringTokenFetchTimeout = 10 * time.Second

// EngineeringEntityConfig holds the configuration for the engineering entity
// service client (wso2-enterprise/digiops-engineering).
type EngineeringEntityConfig struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// EngineeringEntityClient is an HTTP client for the engineering entity service,
// authenticated via the OAuth2 client credentials grant. Tokens are acquired
// and refreshed automatically; callers need not manage them.
//
// NewEngineeringEntityClient never fails and never contacts the token
// endpoint, so it is safe to construct with a zero-value EngineeringEntityConfig
// (e.g. when this service is not yet configured for a given deployment) — a
// missing or invalid configuration only surfaces as an error the first time a
// method is called.
type EngineeringEntityClient struct {
	http    *http.Client
	baseURL string
}

// NewEngineeringEntityClient constructs an EngineeringEntityClient that
// authenticates against the engineering entity service using the OAuth2
// client credentials grant type.
func NewEngineeringEntityClient(cfg EngineeringEntityConfig) *EngineeringEntityClient {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}

	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{Timeout: engineeringTokenFetchTimeout})
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 25 * time.Second

	return &EngineeringEntityClient{
		http:    httpClient,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}
}

// do executes an authenticated HTTP request against the engineering entity
// service and returns the raw JSON response body. The caller owns the
// returned slice.
func (c *EngineeringEntityClient) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("engineering entity: build request %s %s: %w", method, path, err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if id := correlationIDFromContext(ctx); id != "" {
		req.Header.Set("X-CSM-Correlation-ID", id)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("engineering entity: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("engineering entity: read response body: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := respBody
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	return respBody, nil
}

// GitHubIssueLabel is a label attached to a GitHub issue.
type GitHubIssueLabel struct {
	Name        string  `json:"name"`
	Color       string  `json:"color"`
	Description *string `json:"description"`
}

// GitHubIssue is the engineering entity service's representation of a GitHub issue.
type GitHubIssue struct {
	ID          int64              `json:"id"`
	NodeID      string             `json:"nodeId"`
	Number      int                `json:"number"`
	State       string             `json:"state"`
	StateReason *string            `json:"stateReason"`
	Title       string             `json:"title"`
	Body        *string            `json:"body"`
	Labels      []GitHubIssueLabel `json:"labels"`
}

// createGitHubIssueRequest is the wire shape expected by
// POST /orgs/{orgName}/repos/{owner}/{repoName}/issues.
type createGitHubIssueRequest struct {
	Title  string   `json:"title"`
	Body   string   `json:"body,omitempty"`
	Labels []string `json:"labels,omitempty"`
}

// CreateGitIssue creates a GitHub issue in the given org/owner/repo via the
// engineering entity service.
func (c *EngineeringEntityClient) CreateGitIssue(ctx context.Context, orgName, owner, repoName, title, body string, labels []string) (GitHubIssue, error) {
	if orgName == "" || owner == "" || repoName == "" {
		return GitHubIssue{}, fmt.Errorf("engineering entity: orgName, owner, and repoName are required")
	}
	if title == "" {
		return GitHubIssue{}, fmt.Errorf("engineering entity: title is required")
	}

	reqBody, err := json.Marshal(createGitHubIssueRequest{Title: title, Body: body, Labels: labels})
	if err != nil {
		return GitHubIssue{}, fmt.Errorf("engineering entity: encode create issue request: %w", err)
	}

	path := fmt.Sprintf("/orgs/%s/repos/%s/%s/issues",
		url.PathEscape(orgName), url.PathEscape(owner), url.PathEscape(repoName))

	raw, err := c.do(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return GitHubIssue{}, err
	}

	var issue GitHubIssue
	if err := json.Unmarshal(raw, &issue); err != nil {
		return GitHubIssue{}, fmt.Errorf("engineering entity: decode create issue response: %w", err)
	}
	return issue, nil
}
