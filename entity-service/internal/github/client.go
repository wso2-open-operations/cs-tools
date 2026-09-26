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

// Package github talks to the GitHub issues API.
//
// Deliberately small: the change-request sync needs five operations and this
// implements exactly those, rather than pulling in a general-purpose client for
// a fraction of its surface. Add an operation when something needs it.
//
// The base URL is configurable so the same code works against github.com and
// GitHub Enterprise.
package github

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
	"time"
)

// DefaultBaseURL is api.github.com. GitHub Enterprise installations serve the
// same API under /api/v3 on their own host.
const DefaultBaseURL = "https://api.github.com"

const (
	defaultTimeout = 10 * time.Second
	// maxErrBody caps how much of an error response is kept. Enough to
	// identify the failure, bounded so a large body cannot end up in a log.
	maxErrBody = 512
)

// Config configures a Client.
type Config struct {
	BaseURL string
	// Token authenticates every request. A fine-grained PAT or an App
	// installation token; the client does not care which.
	Token   string
	Timeout time.Duration
	// UserAgent is required by GitHub, which rejects requests without one.
	UserAgent string
}

// Client is a GitHub issues client.
type Client struct {
	baseURL   string
	token     string
	userAgent string
	http      *http.Client
}

// NewClient constructs a client. It does not contact GitHub -- a bad token or
// base URL surfaces on the first call, not here, so a misconfigured deployment
// still starts and reports the problem where it happens.
func NewClient(cfg Config) *Client {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = DefaultBaseURL
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	ua := cfg.UserAgent
	if ua == "" {
		ua = "wso2-entity-service"
	}
	return &Client{
		baseURL:   base,
		token:     cfg.Token,
		userAgent: ua,
		http:      &http.Client{Timeout: timeout},
	}
}

// Issue identifies one issue. Every operation takes one.
type Issue struct {
	Owner      string
	Repository string
	Number     int
}

func (i Issue) valid() error {
	if i.Owner == "" || i.Repository == "" || i.Number <= 0 {
		return fmt.Errorf("github: incomplete issue reference %q/%q#%d", i.Owner, i.Repository, i.Number)
	}
	return nil
}

func (i Issue) path() string {
	return fmt.Sprintf("/repos/%s/%s/issues/%d",
		url.PathEscape(i.Owner), url.PathEscape(i.Repository), i.Number)
}

// Comment is one issue comment as GitHub returns it.
type Comment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	HTMLURL   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
	User      User      `json:"user"`
}

// User is the subset of a GitHub account the notices render.
type User struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
}

// State is an issue's open/closed state.
type State string

const (
	StateOpen   State = "open"
	StateClosed State = "closed"
)

// CreatedIssue is what filing an issue returns: the number this service
// stores on the case, and the URL it hands back to the caller.
type CreatedIssue struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
}

// CreateIssue opens a new issue.
//
// THE ONLY OPERATION HERE THAT ADDRESSES A REPOSITORY RATHER THAN AN ISSUE,
// for the obvious reason that the issue does not exist yet. Everything else in
// this client takes an Issue because it acts on one.
//
// Labels are sent as supplied. A label that does not exist in the repository is
// created by GitHub rather than rejected, which is why an unknown label cannot
// fail this call -- and why the caller, not this client, decides what is
// allowed to be sent.
func (c *Client) CreateIssue(ctx context.Context, owner, repository, title, body string, labels []string) (*CreatedIssue, error) {
	if strings.TrimSpace(owner) == "" || strings.TrimSpace(repository) == "" {
		return nil, fmt.Errorf("github: create issue needs an owner and a repository")
	}
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("github: create issue needs a title")
	}

	payload := map[string]any{"title": title, "body": body}
	if len(labels) > 0 {
		payload["labels"] = labels
	}

	var out CreatedIssue
	if err := c.do(ctx, http.MethodPost,
		"/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repository)+"/issues",
		payload, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateComment posts a comment on an issue.
func (c *Client) CreateComment(ctx context.Context, issue Issue, body string) (*Comment, error) {
	if err := issue.valid(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body) == "" {
		return nil, errors.New("github: refusing to post an empty comment")
	}
	var out Comment
	if err := c.do(ctx, http.MethodPost, issue.path()+"/comments",
		map[string]string{"body": body}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListComments returns every comment on an issue, following pagination.
//
// GitHub pages at 100; an issue with a long history would silently truncate at
// the first page otherwise, which is the kind of bug that only shows up on the
// busiest record.
func (c *Client) ListComments(ctx context.Context, issue Issue) ([]Comment, error) {
	if err := issue.valid(); err != nil {
		return nil, err
	}
	var all []Comment
	for page := 1; ; page++ {
		var batch []Comment
		path := issue.path() + "/comments?per_page=100&page=" + strconv.Itoa(page)
		if err := c.do(ctx, http.MethodGet, path, nil, &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < 100 {
			return all, nil
		}
		// A runaway pager is worse than a truncated list.
		if page >= 50 {
			return all, fmt.Errorf("github: comment listing for %s exceeded 50 pages", issue.path())
		}
	}
}

// SetLabels replaces an issue's labels with exactly the set given.
//
// PUT, not POST: the callers that use this are reconciling a computed set, and
// adding to whatever happened to be there would leave labels the caller
// believes it removed.
func (c *Client) SetLabels(ctx context.Context, issue Issue, labels []string) error {
	if err := issue.valid(); err != nil {
		return err
	}
	if labels == nil {
		labels = []string{}
	}
	return c.do(ctx, http.MethodPut, issue.path()+"/labels",
		map[string][]string{"labels": labels}, nil)
}

// RemoveLabel deletes a single label from an issue.
//
// A label that is not there is not an error: the caller wants it gone, and it
// is gone. GitHub says 404, which would otherwise make a retry fail where the
// first attempt succeeded.
// Dispatch fires a repository_dispatch event, which is how ServiceNow drove
// this integration: it never wrote to the issue itself. A GitHub Actions
// workflow in the target repository listens for the event type and decides
// what to do -- comment, label, close.
//
//	POST /repos/{owner}/{repo}/dispatches
//	{"event_type": "...", "client_payload": {...}}
//
// 204 No Content on success. A 422 means no workflow in that repository is
// listening for the type, which GitHub reports as a validation failure rather
// than an error -- see asError, which keeps that distinguishable.
//
// THE REPOSITORY IS NOT AN ISSUE. This takes owner and repo directly rather
// than an Issue, because the event is addressed to the repository; the issue
// number travels inside the payload where the workflow reads it.
func (c *Client) Dispatch(ctx context.Context, owner, repository, eventType string, payload map[string]any) error {
	if strings.TrimSpace(owner) == "" || strings.TrimSpace(repository) == "" {
		return fmt.Errorf("github: dispatch needs an owner and a repository")
	}
	if strings.TrimSpace(eventType) == "" {
		return fmt.Errorf("github: dispatch needs an event type")
	}
	return c.do(ctx, http.MethodPost,
		"/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repository)+"/dispatches",
		map[string]any{"event_type": eventType, "client_payload": payload}, nil)
}

// AddLabel adds one label, leaving the others alone.
//
// POST, not the PUT that SetLabels uses: PUT replaces the whole set, so adding
// a status label with it would silently drop every label a person had put on
// the issue by hand.
func (c *Client) AddLabel(ctx context.Context, issue Issue, label string) error {
	if err := issue.valid(); err != nil {
		return err
	}
	return c.do(ctx, http.MethodPost, issue.path()+"/labels",
		map[string]any{"labels": []string{label}}, nil)
}

func (c *Client) RemoveLabel(ctx context.Context, issue Issue, label string) error {
	if err := issue.valid(); err != nil {
		return err
	}
	if label == "" {
		return errors.New("github: empty label")
	}
	err := c.do(ctx, http.MethodDelete,
		issue.path()+"/labels/"+url.PathEscape(label), nil, nil)
	var apiErr *Error
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		return nil
	}
	return err
}

// SetState opens or closes an issue.
func (c *Client) SetState(ctx context.Context, issue Issue, state State) error {
	if err := issue.valid(); err != nil {
		return err
	}
	if state != StateOpen && state != StateClosed {
		return fmt.Errorf("github: unknown state %q", state)
	}
	return c.do(ctx, http.MethodPatch, issue.path(),
		map[string]string{"state": string(state)}, nil)
}

// Error is a non-2xx response from GitHub.
type Error struct {
	StatusCode int
	// Message is GitHub's own error message, bounded. It never carries the
	// request body back, so a failure cannot echo what was being sent.
	Message string
	// RetryAfter is set when GitHub asked us to back off.
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	return fmt.Sprintf("github: %d: %s", e.StatusCode, e.Message)
}

// RateLimited reports whether this failure was a rate limit rather than a
// problem with the request, which is the difference between retrying and
// giving up.
func (e *Error) RateLimited() bool {
	return e.StatusCode == http.StatusTooManyRequests ||
		(e.StatusCode == http.StatusForbidden && e.RetryAfter > 0)
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("github: encode request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("github: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", c.userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("github: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return c.asError(resp)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("github: decode %s %s: %w", method, path, err)
	}
	return nil
}

func (c *Client) asError(resp *http.Response) error {
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrBody))

	// GitHub returns a JSON body with a message; fall back to the raw snippet
	// when it does not.
	var parsed struct {
		Message string `json:"message"`
	}
	msg := strings.TrimSpace(string(snippet))
	if json.Unmarshal(snippet, &parsed) == nil && parsed.Message != "" {
		msg = parsed.Message
	}

	e := &Error{StatusCode: resp.StatusCode, Message: msg}
	if v := resp.Header.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			e.RetryAfter = time.Duration(secs) * time.Second
		}
	}
	// A secondary rate limit reports a zero remaining quota rather than a
	// Retry-After, so read that too before deciding this is retryable.
	if e.RetryAfter == 0 && resp.Header.Get("X-RateLimit-Remaining") == "0" {
		if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
			if unix, err := strconv.ParseInt(reset, 10, 64); err == nil {
				if d := time.Until(time.Unix(unix, 0)); d > 0 {
					e.RetryAfter = d
				}
			}
		}
	}
	return e
}
