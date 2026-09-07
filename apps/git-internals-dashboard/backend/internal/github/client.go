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

package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// graphQLPath is a var (not const) so titles_test.go can point FetchTitles
// at an httptest server; production code never reassigns it.
var graphQLPath = "https://api.github.com/graphql"

// Overridable only by tests, so retry/pagination pacing tests don't take
// real wall-clock seconds; production always uses the zero-value (real)
// timings below.
var (
	gqlTimeout          = 60 * time.Second
	gqlRetryBackoffUnit = 2 * time.Second
	gqlMaxRetries       = 3
	searchPageDelay     = 250 * time.Millisecond
	detailPageDelay     = 200 * time.Millisecond
	// gqlRetryAfterCap bounds how long gql will ever sleep on GitHub's own
	// Retry-After hint (403 secondary rate limit / 429), so a misbehaving or
	// huge value from GitHub can't stall a sync indefinitely.
	gqlRetryAfterCap = 60 * time.Second
)

type httpClient struct {
	token    string
	endpoint string
	hc       *http.Client
}

// NewClient returns a Client that talks to the real GitHub GraphQL API using
// token (a fine-grained PAT with Issues:Read + Projects:Read, SPEC §5.2).
func NewClient(token string) Client {
	return &httpClient{token: token, endpoint: graphQLPath, hc: &http.Client{}}
}

type graphQLError struct {
	Message string `json:"message"`
}

type graphQLResponse[T any] struct {
	Data   *T             `json:"data"`
	Errors []graphQLError `json:"errors"`
}

// nonRetryableStatuses are HTTP statuses where retrying is pointless: an
// expired/invalid token (401), a malformed query (400/422), or a resource
// that doesn't exist (404) will fail identically on every attempt.
var nonRetryableStatuses = map[int]bool{
	http.StatusBadRequest:          true,
	http.StatusUnauthorized:        true,
	http.StatusNotFound:            true,
	http.StatusUnprocessableEntity: true,
}

// gql posts one GraphQL request with a status-aware retry policy
// (AUDIT-FINDINGS A5): 400/401/404/422 never retry; a 403/429 carrying
// Retry-After sleeps that long (capped) instead of the default linear
// backoff, so this client doesn't hammer through GitHub's secondary rate
// limit — repeatedly doing so is what gets a PAT temporarily banned by
// GitHub's abuse detection. Network errors, other 4xx/5xx, and GraphQL-level
// errors keep the original linear backoff, up to gqlMaxRetries attempts.
func gql[T any](ctx context.Context, c *httpClient, query string, variables map[string]any) (T, error) {
	var zero T
	var lastErr error
	for attempt := 0; ; attempt++ {
		data, err := doGQL[T](ctx, c, query, variables)
		if err == nil {
			return data, nil
		}
		lastErr = err

		var apiErr *APIError
		isHTTPStatus := errors.As(err, &apiErr) && apiErr.Kind == errKindHTTPStatus
		if isHTTPStatus && nonRetryableStatuses[apiErr.StatusCode] {
			return zero, err
		}
		if attempt >= gqlMaxRetries {
			return zero, lastErr
		}

		backoff := time.Duration(attempt+1) * gqlRetryBackoffUnit
		if isHTTPStatus && (apiErr.StatusCode == http.StatusForbidden || apiErr.StatusCode == http.StatusTooManyRequests) && apiErr.RetryAfter > 0 {
			backoff = min(apiErr.RetryAfter, gqlRetryAfterCap)
		}

		slog.WarnContext(ctx, "github graphql request failed; retrying", "attempt", attempt+1, "maxAttempts", gqlMaxRetries, "backoff", backoff, "err", err)
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(backoff):
		}
	}
}

func doGQL[T any](ctx context.Context, c *httpClient, query string, variables map[string]any) (T, error) {
	var zero T

	reqCtx, cancel := context.WithTimeout(ctx, gqlTimeout) // avoids HTTP/2 stream timeout
	defer cancel()

	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return zero, NewTransportError(fmt.Sprintf("github: encode request: %v", err), err)
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return zero, NewTransportError(fmt.Sprintf("github: build request: %v", err), err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "git-internals-dashboard")

	resp, err := c.hc.Do(req)
	if err != nil {
		return zero, NewTransportError(fmt.Sprintf("github: request failed: %v", err), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return zero, NewHTTPStatusError(resp.StatusCode, parseRetryAfter(resp.Header.Get("Retry-After")), string(snippet))
	}

	var parsed graphQLResponse[T]
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return zero, NewTransportError(fmt.Sprintf("github: decode response: %v", err), err)
	}
	if len(parsed.Errors) > 0 {
		messages := make([]string, len(parsed.Errors))
		for i, e := range parsed.Errors {
			messages[i] = e.Message
		}
		return zero, NewGraphQLError(messages)
	}
	if parsed.Data == nil {
		return zero, NewTransportError("github graphql: empty response (no data)", nil)
	}
	return *parsed.Data, nil
}

// parseRetryAfter parses GitHub's Retry-After header (always sent in
// delta-seconds for the GraphQL API, never the HTTP-date form) into a
// Duration. Returns 0 when absent or malformed.
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	secs, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || secs < 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

// ---------------------------------------------------------------------------
// Search: issues matching a repo filter, paginated.
// ---------------------------------------------------------------------------

const searchQuery = `
query ($q: String!, $after: String) {
  search(type: ISSUE, query: $q, first: 50, after: $after) {
    pageInfo { hasNextPage endCursor }
    nodes {
      ... on Issue {
        number
        state
        url
        createdAt
        updatedAt
        closedAt
        labels(first: 30) { nodes { name } }
      }
    }
  }
  rateLimit { remaining }
}`

type searchData struct {
	Search struct {
		PageInfo struct {
			HasNextPage bool    `json:"hasNextPage"`
			EndCursor   *string `json:"endCursor"`
		} `json:"pageInfo"`
		Nodes []struct {
			Number    *int    `json:"number"`
			State     string  `json:"state"`
			URL       string  `json:"url"`
			CreatedAt string  `json:"createdAt"`
			UpdatedAt string  `json:"updatedAt"`
			ClosedAt  *string `json:"closedAt"`
			Labels    struct {
				Nodes []struct {
					Name string `json:"name"`
				} `json:"nodes"`
			} `json:"labels"`
		} `json:"nodes"`
	} `json:"search"`
}

// SearchAll runs q against GitHub's issue search, paginating until
// exhausted. GitHub Search returns at most 1000 results per query; our
// filters stay well under that.
func (c *httpClient) SearchAll(ctx context.Context, q string) ([]IssueNode, error) {
	var out []IssueNode
	var after *string
	for {
		data, err := gql[searchData](ctx, c, searchQuery, map[string]any{"q": q, "after": after})
		if err != nil {
			return nil, err
		}
		for _, n := range data.Search.Nodes {
			if n.Number == nil {
				continue
			}
			labels := make([]string, len(n.Labels.Nodes))
			for i, l := range n.Labels.Nodes {
				labels[i] = l.Name
			}
			out = append(out, IssueNode{
				Number:    *n.Number,
				State:     n.State,
				URL:       n.URL,
				CreatedAt: n.CreatedAt,
				UpdatedAt: n.UpdatedAt,
				ClosedAt:  n.ClosedAt,
				Labels:    labels,
			})
		}
		if !data.Search.PageInfo.HasNextPage {
			break
		}
		after = data.Search.PageInfo.EndCursor
		if err := SleepOrDone(ctx, searchPageDelay); err != nil { // gentle on the secondary rate limiter
			return nil, err
		}
	}
	return out, nil
}

// buildRepoIssueQueries composes the open/closed search query strings for
// FetchRepoIssues, factored out so the composition itself is directly
// testable without a network call.
func buildRepoIssueQueries(owner, name, issueQuery string, closedLookbackDays int, now time.Time) (openQ, closedQ string) {
	base := fmt.Sprintf("repo:%s/%s %s", owner, name, issueQuery)
	openQ = base + " is:open sort:updated-desc"
	since := now.Add(-time.Duration(closedLookbackDays) * 24 * time.Hour).UTC().Format("2006-01-02")
	closedQ = fmt.Sprintf("%s is:closed closed:>=%s sort:updated-desc", base, since)
	return openQ, closedQ
}

// FetchRepoIssues returns every open issue matching issueQuery, plus issues
// closed within closedLookbackDays, deduplicated by issue number (open wins
// if both appear).
func (c *httpClient) FetchRepoIssues(ctx context.Context, owner, name, issueQuery string, closedLookbackDays int) ([]IssueNode, error) {
	openQ, closedQ := buildRepoIssueQueries(owner, name, issueQuery, closedLookbackDays, time.Now())

	closed, err := c.SearchAll(ctx, closedQ)
	if err != nil {
		return nil, err
	}
	open, err := c.SearchAll(ctx, openQ)
	if err != nil {
		return nil, err
	}

	byNumber := make(map[int]IssueNode, len(closed)+len(open))
	for _, issue := range closed {
		byNumber[issue.Number] = issue
	}
	for _, issue := range open {
		byNumber[issue.Number] = issue // open overwrites closed
	}
	out := make([]IssueNode, 0, len(byNumber))
	for _, issue := range byNumber {
		out = append(out, issue)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Detail: per-issue Status timeline + current Status per project.
// ---------------------------------------------------------------------------

const detailQuery = `
query ($owner: String!, $name: String!, $number: Int!, $tlCursor: String) {
  repository(owner: $owner, name: $name) {
    issue(number: $number) {
      number
      timelineItems(
        first: 100
        after: $tlCursor
        itemTypes: [PROJECT_V2_ITEM_STATUS_CHANGED_EVENT]
      ) {
        pageInfo { hasNextPage endCursor }
        nodes {
          __typename
          ... on ProjectV2ItemStatusChangedEvent {
            createdAt
            previousStatus
            status
          }
        }
      }
      projectItems(first: 20) {
        nodes {
          createdAt
          project { id }
          fieldValueByName(name: "Status") {
            ... on ProjectV2ItemFieldSingleSelectValue { name updatedAt }
          }
        }
      }
    }
  }
  rateLimit { remaining }
}`

type detailData struct {
	Repository *struct {
		Issue *struct {
			Number        int `json:"number"`
			TimelineItems struct {
				PageInfo struct {
					HasNextPage bool    `json:"hasNextPage"`
					EndCursor   *string `json:"endCursor"`
				} `json:"pageInfo"`
				Nodes []struct {
					Typename       string  `json:"__typename"`
					CreatedAt      *string `json:"createdAt"`
					PreviousStatus *string `json:"previousStatus"`
					Status         *string `json:"status"`
				} `json:"nodes"`
			} `json:"timelineItems"`
			ProjectItems struct {
				Nodes []struct {
					CreatedAt string `json:"createdAt"`
					Project   struct {
						ID string `json:"id"`
					} `json:"project"`
					FieldValueByName *struct {
						Name      string `json:"name"`
						UpdatedAt string `json:"updatedAt"`
					} `json:"fieldValueByName"`
				} `json:"nodes"`
			} `json:"projectItems"`
		} `json:"issue"`
	} `json:"repository"`
}

// FetchIssueDetail returns one issue's status timeline and per-project
// current status, or (nil, nil) if the issue does not exist.
func (c *httpClient) FetchIssueDetail(ctx context.Context, owner, name string, number int) (*IssueDetail, error) {
	var events []StatusEvent
	var projectStatuses []ProjectStatus
	var tlCursor *string

	for {
		data, err := gql[detailData](ctx, c, detailQuery, map[string]any{
			"owner": owner, "name": name, "number": number, "tlCursor": tlCursor,
		})
		if err != nil {
			return nil, err
		}
		if data.Repository == nil || data.Repository.Issue == nil {
			return nil, nil
		}
		issue := data.Repository.Issue

		for _, node := range issue.TimelineItems.Nodes {
			if node.Typename != "ProjectV2ItemStatusChangedEvent" || node.CreatedAt == nil {
				continue
			}
			events = append(events, StatusEvent{
				CreatedAt:      *node.CreatedAt,
				PreviousStatus: node.PreviousStatus,
				Status:         node.Status,
			})
		}

		// projectItems (first: 20) is plenty; capture it once on the first page.
		if len(projectStatuses) == 0 {
			for _, p := range issue.ProjectItems.Nodes {
				ps := ProjectStatus{ProjectID: p.Project.ID, ItemCreatedAt: strPtr(p.CreatedAt)}
				if p.FieldValueByName != nil {
					ps.Status = strPtr(p.FieldValueByName.Name)
					ps.StatusUpdatedAt = strPtr(p.FieldValueByName.UpdatedAt)
				}
				projectStatuses = append(projectStatuses, ps)
			}
		}

		if !issue.TimelineItems.PageInfo.HasNextPage {
			break
		}
		tlCursor = issue.TimelineItems.PageInfo.EndCursor
		if err := SleepOrDone(ctx, detailPageDelay); err != nil {
			return nil, err
		}
	}

	// Ascending by time — the SLA interval walk depends on this.
	sort.SliceStable(events, func(i, j int) bool { return events[i].CreatedAt < events[j].CreatedAt })
	return &IssueDetail{Number: number, Events: events, ProjectStatuses: projectStatuses}, nil
}

func strPtr(s string) *string { return &s }

// SleepOrDone sleeps for d, or returns ctx.Err() early if ctx is canceled
// first. Exported so internal/sync's own inter-issue pacing (AUDIT-FINDINGS
// B4) doesn't need a second identical copy — sync already depends on this
// package.
func SleepOrDone(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
