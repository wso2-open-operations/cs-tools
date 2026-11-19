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

// Package engineeringentity is an HTTP client for the WSO2 engineering entity
// service, which fronts GitHub issues
// and projects. It creates and reads the issues of the product repositories
// for the git-internals feature set (see internal/service/product_work_item_service.go).
//
// Auth is OAuth2 client credentials, like internal/salesentity. When no token
// settings are given the client sends no Authorization header (local use
// against http://localhost:9090).
package engineeringentity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultTimeout   = 30 * time.Second
	tokenExpirySlack = 30 * time.Second
	maxBodyBytes     = 8 << 20
)

// ClientCredentialsConfig holds the OAuth2 client credentials. A zero value
// (no TokenURL) turns authentication off.
type ClientCredentialsConfig struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	// Scopes is a space-separated list of OAuth2 scopes; optional.
	Scopes string
}

func (c ClientCredentialsConfig) enabled() bool {
	return c.TokenURL != "" && c.ClientID != "" && c.ClientSecret != ""
}

// Error is a non-2xx answer from the engineering entity, or a transport
// failure (StatusCode 0). Message never carries the response body, a token or a
// secret: it is safe to log, and is never returned to a portal caller.
type Error struct {
	// StatusCode is the HTTP status of the answer; 0 when no answer arrived
	// (network failure or timeout).
	StatusCode int
	// RetryAfter is the parsed Retry-After header (seconds form); 0 if absent.
	RetryAfter time.Duration
	// GitHubStatus is GitHub's own status code when the failure is an answer from
	// GitHub, from the X-GitHub-Status response header; 0 when absent (the
	// engineering entity failed without a GitHub answer, or nothing answered).
	GitHubStatus int
	Message      string
}

func (e *Error) Error() string {
	return fmt.Sprintf("engineeringentity: %s", e.Message)
}

// Definite reports whether the answer means the request certainly did not
// create anything on GitHub: only 400, 401, 403, 404 and 422. Every other
// status (408, 409, 425, 429, other 4xx, any 5xx) and a transport failure or
// timeout (StatusCode 0) is ambiguous: the call may have reached GitHub.
func (e *Error) Definite() bool {
	switch e.StatusCode {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden,
		http.StatusNotFound, http.StatusUnprocessableEntity:
		return true
	}
	return false
}

// IssueRef is the part of the create answer this service needs.
type IssueRef struct {
	ID     int64  `json:"id"`
	NodeID string `json:"nodeId"`
	Number int    `json:"number"`
}

// IssueDetail is GET .../issue-details/{n}.
type IssueDetail struct {
	ID            int64    `json:"id"`
	NodeID        string   `json:"nodeId"`
	Number        int      `json:"number"`
	Title         string   `json:"title"`
	Body          *string  `json:"body"`
	State         string   `json:"state"`
	StateReason   *string  `json:"stateReason"`
	Labels        []string `json:"labels"`
	AuthorLogin   *string  `json:"authorLogin"`
	HTMLURL       string   `json:"htmlUrl"`
	CreatedAt     string   `json:"createdAt"`
	UpdatedAt     string   `json:"updatedAt"`
	ClosedAt      *string  `json:"closedAt"`
	AssigneeLogin []string `json:"assigneeLogins"`
}

// IssueComment is one element of GET .../issues/{n}/comments.
type IssueComment struct {
	ID          int64   `json:"id"`
	NodeID      string  `json:"nodeId"`
	Body        string  `json:"body"`
	AuthorLogin *string `json:"authorLogin"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
	HTMLURL     string  `json:"htmlUrl"`
}

// ProjectStatusChange is one Status change of an issue on a project board.
type ProjectStatusChange struct {
	EventNodeID    string  `json:"eventNodeId"`
	PreviousStatus *string `json:"previousStatus"`
	NewStatus      *string `json:"newStatus"`
	OccurredAt     string  `json:"occurredAt"`
	ActorLogin     *string `json:"actorLogin"`
}

// ProjectStatusHistory is the Status of an issue on one board, with history.
type ProjectStatusHistory struct {
	ProjectNumber int                   `json:"projectNumber"`
	ProjectNodeID string                `json:"projectNodeId"`
	ItemID        string                `json:"itemId"`
	IsArchived    bool                  `json:"isArchived"`
	CurrentStatus *string               `json:"currentStatus"`
	Changes       []ProjectStatusChange `json:"changes"`
}

// ProjectItemFieldValue is one set field of a project item. Value is the
// engineering entity's simplified value: a string for a single select (the
// column name), a number for a number field.
type ProjectItemFieldValue struct {
	Name     string          `json:"name"`
	DataType string          `json:"dataType"`
	Value    json.RawMessage `json:"value"`
}

// ProjectItemContent is the issue or pull request behind a project item.
type ProjectItemContent struct {
	NodeID string `json:"nodeId"`
	// Repository is "owner/name".
	Repository    string   `json:"repository"`
	Number        int      `json:"number"`
	Title         string   `json:"title"`
	URL           string   `json:"url"`
	State         string   `json:"state"` // OPEN, CLOSED (MERGED for a pull request)
	StateReason   *string  `json:"stateReason"`
	IsPullRequest bool     `json:"isPullRequest"`
	Body          string   `json:"body"`
	Labels        []string `json:"labels"`
	Assignees     []string `json:"assignees"`
	CreatedAt     string   `json:"createdAt"`
	UpdatedAt     string   `json:"updatedAt"`
	ClosedAt      *string  `json:"closedAt"`
}

// RepositoryOwner and RepositoryName split Repository ("owner/name"); both are
// "" when it has no slash.
func (c ProjectItemContent) RepositoryOwner() string {
	owner, _, _ := strings.Cut(c.Repository, "/")
	if !strings.Contains(c.Repository, "/") {
		return ""
	}
	return owner
}

// RepositoryName is the part of Repository after the slash.
func (c ProjectItemContent) RepositoryName() string {
	_, name, ok := strings.Cut(c.Repository, "/")
	if !ok {
		return ""
	}
	return name
}

// ProjectItem is one item of a project board with its content and field values.
type ProjectItem struct {
	// ItemID is the numeric item id the item update route needs (an additive field
	// of the engineering entity); 0 when the engineering entity does not send it.
	ItemID int64  `json:"itemId"`
	NodeID string `json:"nodeId"`
	// Type is ISSUE, PULL_REQUEST, DRAFT_ISSUE or REDACTED.
	Type string `json:"type"`
	// Content is nil for a draft or redacted item.
	Content     *ProjectItemContent     `json:"content"`
	FieldValues []ProjectItemFieldValue `json:"fieldValues"`
}

// Status returns the item's Status column as a string; ok is false when the
// item has no single-select Status value.
func (i ProjectItem) Status() (status string, ok bool) {
	for _, f := range i.FieldValues {
		if !strings.EqualFold(f.Name, "Status") || !strings.EqualFold(f.DataType, "SINGLE_SELECT") {
			continue
		}
		var v string
		if json.Unmarshal(f.Value, &v) != nil {
			return "", false
		}
		return v, true
	}
	return "", false
}

// ProjectItemsPage is one page of GET .../projects/{n}/items-with-content.
type ProjectItemsPage struct {
	Items []ProjectItem `json:"items"`
	// NextCursor is the cursor of the next page; nil on the last page.
	NextCursor *string `json:"nextCursor"`
}

// ListIssueDetailsQuery are the filters of GET .../issue-details. Zero values
// are left out of the request.
type ListIssueDetailsQuery struct {
	State   string
	Labels  []string
	Since   string // ISO 8601
	PerPage int
	Page    int
}

// ListCommentsQuery are the filters of GET .../issues/{n}/comments.
type ListCommentsQuery struct {
	Since   string
	PerPage int
	Page    int
}

type createIssueRequest struct {
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels,omitempty"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// Client calls the engineering entity.
type Client struct {
	baseURL    string
	creds      ClientCredentialsConfig
	httpClient *http.Client

	// mu guards cachedToken and tokenExpiry and is never held across a network
	// call. refresh serialises token fetches; it is a channel so a caller waiting
	// for another's fetch gives up when its own context ends.
	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
	refresh     chan struct{}
}

// New constructs a Client with a 30-second timeout. Without token settings it
// sends no Authorization header.
func New(baseURL string, creds ClientCredentialsConfig) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		creds:      creds,
		httpClient: &http.Client{Timeout: defaultTimeout},
		refresh:    make(chan struct{}, 1),
	}
}

// AuthEnabled reports whether the client authenticates its calls.
func (c *Client) AuthEnabled() bool { return c.creds.enabled() }

func issuesBase(org, owner, repo string) string {
	return "/orgs/" + url.PathEscape(org) + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
}

// CreateIssue creates a GitHub issue via POST .../issues. The engineering
// entity answers non-404 failures with a bare 500, so a 5xx here is ambiguous
// (GitHub may or may not have created the issue); see Error.Definite for the
// statuses that are not.
func (c *Client) CreateIssue(ctx context.Context, org, owner, repo, title, body string, labels []string) (IssueRef, error) {
	var out IssueRef
	err := c.do(ctx, http.MethodPost, issuesBase(org, owner, repo)+"/issues", nil,
		createIssueRequest{Title: title, Body: body, Labels: labels}, &out)
	if err != nil {
		return IssueRef{}, err
	}
	if out.Number <= 0 {
		return IssueRef{}, &Error{StatusCode: http.StatusBadGateway, Message: "create issue answer has no issue number"}
	}
	return out, nil
}

// GetIssueDetail reads one issue (GET .../issue-details/{n}).
func (c *Client) GetIssueDetail(ctx context.Context, org, owner, repo string, number int) (IssueDetail, error) {
	var out IssueDetail
	err := c.do(ctx, http.MethodGet, issuesBase(org, owner, repo)+"/issue-details/"+strconv.Itoa(number), nil, nil, &out)
	return out, err
}

// ListIssueDetails reads one page of issues (GET .../issue-details).
func (c *Client) ListIssueDetails(ctx context.Context, org, owner, repo string, q ListIssueDetailsQuery) ([]IssueDetail, error) {
	v := url.Values{}
	if q.State != "" {
		v.Set("state", q.State)
	}
	if len(q.Labels) > 0 {
		v.Set("labels", strings.Join(q.Labels, ","))
	}
	if q.Since != "" {
		v.Set("since", q.Since)
	}
	if q.PerPage > 0 {
		v.Set("perPage", strconv.Itoa(q.PerPage))
	}
	if q.Page > 0 {
		v.Set("page", strconv.Itoa(q.Page))
	}
	var out []IssueDetail
	err := c.do(ctx, http.MethodGet, issuesBase(org, owner, repo)+"/issue-details", v, nil, &out)
	return out, err
}

// ListIssueComments reads one page of an issue's comments, oldest first.
func (c *Client) ListIssueComments(ctx context.Context, org, owner, repo string, number int, q ListCommentsQuery) ([]IssueComment, error) {
	v := url.Values{}
	if q.Since != "" {
		v.Set("since", q.Since)
	}
	if q.PerPage > 0 {
		v.Set("perPage", strconv.Itoa(q.PerPage))
	}
	if q.Page > 0 {
		v.Set("page", strconv.Itoa(q.Page))
	}
	var out []IssueComment
	err := c.do(ctx, http.MethodGet, issuesBase(org, owner, repo)+"/issues/"+strconv.Itoa(number)+"/comments", v, nil, &out)
	return out, err
}

// GetProjectStatusHistory reads the board Status history of an issue.
func (c *Client) GetProjectStatusHistory(ctx context.Context, org, owner, repo string, number int) ([]ProjectStatusHistory, error) {
	var out []ProjectStatusHistory
	err := c.do(ctx, http.MethodGet, issuesBase(org, owner, repo)+"/issues/"+strconv.Itoa(number)+"/project-status-history", nil, nil, &out)
	return out, err
}

// ListProjectItemsWithContent reads one page of a project board's items with
// their issue content and field values (GET .../projects/{n}/items-with-content).
// after is the previous page's NextCursor ("" for the first page); perPage 0
// leaves the engineering entity's default (100). The engineering entity may
// serve this read from a cache of up to a minute.
func (c *Client) ListProjectItemsWithContent(ctx context.Context, org string, projectNumber int, after string, perPage int) (ProjectItemsPage, error) {
	v := url.Values{}
	if perPage > 0 {
		v.Set("perPage", strconv.Itoa(perPage))
	}
	if after != "" {
		v.Set("after", after)
	}
	var out ProjectItemsPage
	err := c.do(ctx, http.MethodGet, "/orgs/"+url.PathEscape(org)+"/projects/"+strconv.Itoa(projectNumber)+"/items-with-content", v, nil, &out)
	return out, err
}

// IssueState is the part of an issue update answer the write-back needs.
type IssueState struct {
	ID          int64   `json:"id"`
	NodeID      string  `json:"nodeId"`
	Number      int     `json:"number"`
	State       string  `json:"state"` // "open" or "closed", lower case as GitHub spells it
	StateReason *string `json:"stateReason"`
}

// ProjectFieldOption is one option of a single-select project field.
type ProjectFieldOption struct {
	ID string `json:"id"`
	// Name is the option's display name as the engineering entity flattens it (the
	// HTML form of GitHub's name; plain text for an ordinary name).
	Name string `json:"name"`
}

// ProjectField is one field of a project board (GET .../projects/{n}/fields).
type ProjectField struct {
	ID       int64                `json:"id"`
	NodeID   string               `json:"nodeId"`
	Name     string               `json:"name"`
	DataType string               `json:"dataType"` // e.g. "single_select"
	Options  []ProjectFieldOption `json:"options"`
}

// ProjectItemRef is one element of GET .../projects/{n}/items: the numeric item id
// the update needs, and the issue behind it. The route returns no repository, so an
// item is identified by Content.NodeID (unique across GitHub), not by number alone.
type ProjectItemRef struct {
	ID      int64  `json:"id"`
	NodeID  string `json:"nodeId"`
	Content struct {
		ID     int64  `json:"id"`
		NodeID string `json:"nodeId"`
		Number int    `json:"number"`
	} `json:"content"`
}

// ListProjectItemsQuery are the parameters of GET .../projects/{n}/items. The route
// has no paging: it returns one page of at most PerPage items (eng default 30, GitHub
// max 100) matching Q (eng default "is:open").
type ListProjectItemsQuery struct {
	PerPage int
	Q       string
}

// AddIssueComment posts a comment (POST .../issues/{n}/comments). The answer carries
// the comment id and timestamps; AuthorLogin is not part of it (nil).
func (c *Client) AddIssueComment(ctx context.Context, org, owner, repo string, number int, body string) (IssueComment, error) {
	var out IssueComment
	err := c.do(ctx, http.MethodPost, issuesBase(org, owner, repo)+"/issues/"+strconv.Itoa(number)+"/comments", nil,
		map[string]string{"body": body}, &out)
	if err != nil {
		return IssueComment{}, err
	}
	if out.ID <= 0 {
		return IssueComment{}, &Error{StatusCode: http.StatusBadGateway, Message: "add comment answer has no comment id"}
	}
	return out, nil
}

// CloseIssue closes an issue (PATCH .../issues/{n}) with the given state reason
// ("completed"). The answer is the issue's state after the call.
func (c *Client) CloseIssue(ctx context.Context, org, owner, repo string, number int, stateReason string) (IssueState, error) {
	var out IssueState
	err := c.do(ctx, http.MethodPatch, issuesBase(org, owner, repo)+"/issues/"+strconv.Itoa(number), nil,
		map[string]string{"state": "closed", "stateReason": stateReason}, &out)
	return out, err
}

// ListProjectFields reads the fields of a project board with their options.
func (c *Client) ListProjectFields(ctx context.Context, org string, projectNumber int) ([]ProjectField, error) {
	v := url.Values{}
	v.Set("perPage", "100")
	var out []ProjectField
	err := c.do(ctx, http.MethodGet, "/orgs/"+url.PathEscape(org)+"/projects/"+strconv.Itoa(projectNumber)+"/fields", v, nil, &out)
	return out, err
}

// ListProjectItems reads one page of a board's items (GET .../projects/{n}/items).
func (c *Client) ListProjectItems(ctx context.Context, org string, projectNumber int, q ListProjectItemsQuery) ([]ProjectItemRef, error) {
	v := url.Values{}
	if q.PerPage > 0 {
		v.Set("perPage", strconv.Itoa(q.PerPage))
	}
	if q.Q != "" {
		v.Set("q", q.Q)
	}
	var out []ProjectItemRef
	err := c.do(ctx, http.MethodGet, "/orgs/"+url.PathEscape(org)+"/projects/"+strconv.Itoa(projectNumber)+"/items", v, nil, &out)
	return out, err
}

type projectFieldUpdate struct {
	ID    int64  `json:"id"`
	Value string `json:"value"`
}

// UpdateProjectItemStatus sets a single-select field of a board item to an option
// (PATCH .../projects/{n}/items/{itemId}). optionID is the option's id, not its name.
func (c *Client) UpdateProjectItemStatus(ctx context.Context, org string, projectNumber int, itemID, fieldID int64, optionID string) error {
	body := map[string][]projectFieldUpdate{"fields": {{ID: fieldID, Value: optionID}}}
	return c.do(ctx, http.MethodPatch, "/orgs/"+url.PathEscape(org)+"/projects/"+strconv.Itoa(projectNumber)+"/items/"+strconv.FormatInt(itemID, 10), nil, body, nil)
}

// do performs one call, refreshing the token once on a 401.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return fmt.Errorf("engineeringentity: marshal request: %w", err)
		}
	}
	used, err := c.once(ctx, method, path, query, payload, out)
	var ee *Error
	if errors.As(err, &ee) && ee.StatusCode == http.StatusUnauthorized && c.creds.enabled() {
		c.invalidateToken(used)
		_, err = c.once(ctx, method, path, query, payload, out)
	}
	return err
}

// once makes one attempt and returns the token it sent ("" when none).
func (c *Client) once(ctx context.Context, method, path string, query url.Values, payload []byte, out any) (string, error) {
	token := ""
	if c.creds.enabled() {
		var err error
		if token, err = c.accessToken(ctx); err != nil {
			return "", err
		}
	}
	return token, c.send(ctx, token, method, path, query, payload, out)
}

func (c *Client) send(ctx context.Context, token, method, path string, query url.Values, payload []byte, out any) error {
	// path is already escaped segment by segment; build the URL by string so
	// the escaping is not undone.
	full := c.baseURL + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}
	var rdr io.Reader
	if payload != nil {
		rdr = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, full, rdr)
	if err != nil {
		return fmt.Errorf("engineeringentity: build request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return transportError(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return &Error{Message: "read response: " + redactErr(err)}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out == nil {
			return nil
		}
		if err := json.Unmarshal(raw, out); err != nil {
			// A 2xx we cannot parse: the call may have succeeded upstream.
			return &Error{StatusCode: http.StatusBadGateway, Message: "parse response: " + redactErr(err)}
		}
		return nil
	}
	e := &Error{StatusCode: resp.StatusCode, Message: fmt.Sprintf("%s %s returned %d", method, safePath(path), resp.StatusCode)}
	if gs := strings.TrimSpace(resp.Header.Get("X-GitHub-Status")); gs != "" {
		if code, perr := strconv.Atoi(gs); perr == nil && code > 0 {
			e.GitHubStatus = code
		}
	}
	if ra := strings.TrimSpace(resp.Header.Get("Retry-After")); ra != "" {
		if secs, perr := strconv.Atoi(ra); perr == nil && secs > 0 {
			e.RetryAfter = time.Duration(secs) * time.Second
		}
	}
	return e
}

// transportError maps a network failure or timeout to an *Error with status 0.
// Context cancellation is kept as-is so the caller can tell it from upstream.
func transportError(err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	return &Error{Message: "request failed: " + redactErr(err)}
}

// redactErr returns the error text without the URL: net/http errors embed the
// full URL, and the token URL query could carry nothing secret, but a request
// URL is not worth logging verbatim either.
func redactErr(err error) string {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Op + ": " + ue.Err.Error()
	}
	return err.Error()
}

func safePath(p string) string {
	if i := strings.Index(p, "?"); i >= 0 {
		return p[:i]
	}
	return p
}

// cachedValidToken returns the cached token when it is still usable.
func (c *Client) cachedValidToken() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cachedToken != "" && time.Now().Add(tokenExpirySlack).Before(c.tokenExpiry) {
		return c.cachedToken, true
	}
	return "", false
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	if t, ok := c.cachedValidToken(); ok {
		return t, nil
	}
	// One fetch at a time, but a waiting caller honours its own context.
	select {
	case c.refresh <- struct{}{}:
		defer func() { <-c.refresh }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	// Another caller may have refreshed while this one waited.
	if t, ok := c.cachedValidToken(); ok {
		return t, nil
	}
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", c.creds.ClientID)
	data.Set("client_secret", c.creds.ClientSecret)
	if c.creds.Scopes != "" {
		data.Set("scope", c.creds.Scopes)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.creds.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("engineeringentity: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", transportError(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", &Error{Message: "read token response: " + redactErr(err)}
	}
	if resp.StatusCode != http.StatusOK {
		// 401/403 here means our credentials are wrong: report as a 502-class
		// failure of the dependency, never as the caller's 401.
		return "", &Error{StatusCode: http.StatusBadGateway, Message: fmt.Sprintf("token endpoint returned %d", resp.StatusCode)}
	}
	var tr tokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil || tr.AccessToken == "" {
		return "", &Error{StatusCode: http.StatusBadGateway, Message: "token endpoint returned an unusable answer"}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedToken = tr.AccessToken
	if tr.ExpiresIn > 0 {
		c.tokenExpiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	} else {
		c.tokenExpiry = time.Now().Add(time.Hour)
	}
	return c.cachedToken, nil
}

// invalidateToken drops the cached token only if it is still the one the
// failed request used: a concurrent caller may already have refreshed it, and
// that newer token must not be thrown away.
func (c *Client) invalidateToken(used string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cachedToken != used {
		return
	}
	c.cachedToken = ""
	c.tokenExpiry = time.Time{}
}
