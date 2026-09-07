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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fastTimings(t *testing.T) {
	t.Helper()
	origBackoff, origDelay1, origDelay2, origCap := gqlRetryBackoffUnit, searchPageDelay, detailPageDelay, gqlRetryAfterCap
	gqlRetryBackoffUnit = time.Millisecond
	searchPageDelay = time.Millisecond
	detailPageDelay = time.Millisecond
	gqlRetryAfterCap = 50 * time.Millisecond
	t.Cleanup(func() {
		gqlRetryBackoffUnit, searchPageDelay, detailPageDelay, gqlRetryAfterCap = origBackoff, origDelay1, origDelay2, origCap
	})
}

func newTestClient(t *testing.T, handler http.HandlerFunc) *httpClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &httpClient{token: "test-token", endpoint: srv.URL, hc: srv.Client()}
}

func TestBuildRepoIssueQueries(t *testing.T) {
	now := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	openQ, closedQ := buildRepoIssueQueries("wso2-enterprise", "wso2-iam-internal", `label:"Origin/CS" -label:"Type/Patch"`, 90, now)

	wantOpen := `repo:wso2-enterprise/wso2-iam-internal label:"Origin/CS" -label:"Type/Patch" is:open sort:updated-desc`
	if openQ != wantOpen {
		t.Errorf("openQ:\n got:  %s\n want: %s", openQ, wantOpen)
	}
	wantClosed := `repo:wso2-enterprise/wso2-iam-internal label:"Origin/CS" -label:"Type/Patch" is:closed closed:>=2025-12-10 sort:updated-desc`
	if closedQ != wantClosed {
		t.Errorf("closedQ:\n got:  %s\n want: %s", closedQ, wantClosed)
	}
}

func TestSearchAllPaginatesUntilExhausted(t *testing.T) {
	fastTimings(t)
	var calls int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_, _ = w.Write([]byte(`{"data":{"search":{"pageInfo":{"hasNextPage":true,"endCursor":"cursor1"},
				"nodes":[{"number":1,"state":"OPEN","url":"https://x/1","createdAt":"2026-01-01T00:00:00Z",
				"updatedAt":"2026-01-02T00:00:00Z","closedAt":null,"labels":{"nodes":[{"name":"Priority/High(P2)"}]}}]}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"search":{"pageInfo":{"hasNextPage":false,"endCursor":null},
			"nodes":[{"number":2,"state":"CLOSED","url":"https://x/2","createdAt":"2026-01-03T00:00:00Z",
			"updatedAt":"2026-01-04T00:00:00Z","closedAt":"2026-01-05T00:00:00Z","labels":{"nodes":[]}}]}}}`))
	})

	issues, err := client.SearchAll(context.Background(), `repo:acme/widgets is:open`)
	if err != nil {
		t.Fatalf("SearchAll: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues across both pages, got %d", len(issues))
	}
	if issues[0].Number != 1 || issues[0].Labels[0] != "Priority/High(P2)" {
		t.Errorf("unexpected first issue: %+v", issues[0])
	}
	if issues[1].Number != 2 || issues[1].ClosedAt == nil || *issues[1].ClosedAt != "2026-01-05T00:00:00Z" {
		t.Errorf("unexpected second issue: %+v", issues[1])
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Errorf("expected 2 HTTP calls (one per page), got %d", calls)
	}
}

func TestGqlRetriesThenSucceeds(t *testing.T) {
	fastTimings(t)
	var calls int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"search":{"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[]}}}`))
	})

	_, err := client.SearchAll(context.Background(), "q")
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if atomic.LoadInt32(&calls) != 3 {
		t.Errorf("expected 3 attempts (2 failures + 1 success), got %d", calls)
	}
}

func TestGqlGivesUpAfterMaxRetries(t *testing.T) {
	fastTimings(t)
	var calls int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})

	_, err := client.SearchAll(context.Background(), "q")
	if err == nil {
		t.Fatal("expected an error after exhausting retries")
	}
	if want := int32(gqlMaxRetries + 1); atomic.LoadInt32(&calls) != want {
		t.Errorf("expected %d total attempts, got %d", want, calls)
	}
}

func TestGqlSurfacesGraphQLLevelErrors(t *testing.T) {
	fastTimings(t)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"message":"Could not resolve to a Repository"}]}`))
	})

	_, err := client.SearchAll(context.Background(), "q")
	if err == nil || !strings.Contains(err.Error(), "Could not resolve to a Repository") {
		t.Fatalf("expected the GraphQL error message surfaced, got: %v", err)
	}
}

// TestTransportErrorUnwrapsToOriginalError guards against silently
// flattening the error chain: an APIError wrapping a transport failure must
// still let errors.Is/errors.As see through to the original error (e.g. a
// canceled context), not just its own Kind/StatusCode.
func TestTransportErrorUnwrapsToOriginalError(t *testing.T) {
	fastTimings(t)
	origMaxRetries := gqlMaxRetries
	gqlMaxRetries = 0 // fail on the first attempt, no retry loop involved
	t.Cleanup(func() { gqlMaxRetries = origMaxRetries })

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.SearchAll(ctx, "q")
	if err == nil {
		t.Fatal("expected an error for a canceled context")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected the returned error to be (or wrap) an *APIError, got: %T (%v)", err, err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected errors.Is(err, context.Canceled) to see through the APIError wrapper, got: %v", err)
	}
}

// TestGqlDoesNotRetry401 (AUDIT-FINDINGS A5): an expired/invalid token fails
// identically on every attempt, so retrying it 3 times with backoff is pure
// waste — gql must give up after the first response.
func TestGqlDoesNotRetry401(t *testing.T) {
	fastTimings(t)
	var calls int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Bad credentials"))
	})

	_, err := client.SearchAll(context.Background(), "q")
	if err == nil {
		t.Fatal("expected an error")
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 attempt for a 401, got %d", calls)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected an *APIError with StatusCode=401, got %v", err)
	}
}

// TestGqlHonorsRetryAfterOnSecondaryRateLimit (AUDIT-FINDINGS A5): a 403
// secondary-rate-limit response carrying Retry-After must be waited out
// exactly that long (capped), not hammered on the default linear schedule —
// repeatedly ignoring Retry-After is what gets a PAT temporarily banned by
// GitHub's abuse detection.
func TestGqlHonorsRetryAfterOnSecondaryRateLimit(t *testing.T) {
	fastTimings(t)                     // gqlRetryBackoffUnit=1ms, so a linear-backoff bug would return almost instantly
	gqlRetryAfterCap = 5 * time.Second // above the 1s Retry-After used below, so the cap doesn't mask this test

	var calls int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "1") // GitHub always sends delta-seconds
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("secondary rate limit"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"search":{"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[]}}}`))
	})

	start := time.Now()
	_, err := client.SearchAll(context.Background(), "q")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("expected success after honoring Retry-After, got: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 attempts, got %d", calls)
	}
	if elapsed < 900*time.Millisecond {
		t.Errorf("expected gql to wait out the 1s Retry-After (not the 1ms linear backoff), elapsed=%v", elapsed)
	}
}

// TestGqlCapsRetryAfter (AUDIT-FINDINGS A5): an oversized Retry-After must
// be capped rather than honored verbatim, so a misbehaving response can't
// stall a sync indefinitely.
func TestGqlCapsRetryAfter(t *testing.T) {
	fastTimings(t) // gqlRetryAfterCap=50ms in tests; GitHub sends a huge Retry-After below

	var calls int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "3600")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("rate limited"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"search":{"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[]}}}`))
	})

	start := time.Now()
	_, err := client.SearchAll(context.Background(), "q")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if elapsed > time.Second {
		t.Errorf("expected the 3600s Retry-After to be capped to ~50ms, elapsed=%v", elapsed)
	}
}

func TestFetchIssueDetailParsesTimelineAndProjectStatus(t *testing.T) {
	fastTimings(t)
	var reqBodies []map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		reqBodies = append(reqBodies, body)
		w.Header().Set("Content-Type", "application/json")
		if len(reqBodies) == 1 {
			_, _ = w.Write([]byte(`{"data":{"repository":{"issue":{"number":42,
				"timelineItems":{"pageInfo":{"hasNextPage":true,"endCursor":"tl1"},
					"nodes":[{"__typename":"ProjectV2ItemStatusChangedEvent","createdAt":"2026-01-02T00:00:00Z","previousStatus":null,"status":"Open"}]},
				"projectItems":{"nodes":[{"createdAt":"2026-01-01T00:00:00Z","project":{"id":"PVT_1"},
					"fieldValueByName":{"name":"Open","updatedAt":"2026-01-02T00:00:00Z"}}]}}}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"repository":{"issue":{"number":42,
			"timelineItems":{"pageInfo":{"hasNextPage":false,"endCursor":null},
				"nodes":[{"__typename":"ProjectV2ItemStatusChangedEvent","createdAt":"2026-01-01T00:00:00Z","previousStatus":null,"status":"WOC"}]},
			"projectItems":{"nodes":[]}}}}}`))
	})

	detail, err := client.FetchIssueDetail(context.Background(), "acme", "widgets", 42)
	if err != nil {
		t.Fatalf("FetchIssueDetail: %v", err)
	}
	if detail == nil {
		t.Fatal("expected a non-nil detail")
	}
	if len(detail.Events) != 2 {
		t.Fatalf("expected 2 timeline events across both pages, got %d", len(detail.Events))
	}
	// Ascending by createdAt regardless of page order.
	if detail.Events[0].CreatedAt != "2026-01-01T00:00:00Z" || detail.Events[1].CreatedAt != "2026-01-02T00:00:00Z" {
		t.Errorf("expected ascending order, got %+v", detail.Events)
	}
	if len(detail.ProjectStatuses) != 1 || detail.ProjectStatuses[0].ProjectID != "PVT_1" {
		t.Fatalf("expected project status captured from first page only, got %+v", detail.ProjectStatuses)
	}
	if *detail.ProjectStatuses[0].Status != "Open" {
		t.Errorf("expected status=Open, got %v", detail.ProjectStatuses[0].Status)
	}
	if reqBodies[1]["variables"].(map[string]any)["tlCursor"] != "tl1" {
		t.Errorf("expected second request to carry the endCursor from the first page, got %+v", reqBodies[1]["variables"])
	}
}

func TestFetchIssueDetailReturnsNilForMissingIssue(t *testing.T) {
	fastTimings(t)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"repository":{"issue":null}}}`))
	})

	detail, err := client.FetchIssueDetail(context.Background(), "acme", "widgets", 404)
	if err != nil {
		t.Fatalf("expected no error for a missing issue, got: %v", err)
	}
	if detail != nil {
		t.Errorf("expected nil detail for a missing issue, got %+v", detail)
	}
}

func TestFetchRepoIssuesDedupesOpenOverClosed(t *testing.T) {
	fastTimings(t)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		q, _ := body["variables"].(map[string]any)["q"].(string)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(q, "is:closed") {
			_, _ = w.Write([]byte(`{"data":{"search":{"pageInfo":{"hasNextPage":false,"endCursor":null},
				"nodes":[{"number":1,"state":"CLOSED","url":"https://x/1","createdAt":"2026-01-01T00:00:00Z",
				"updatedAt":"2026-01-01T00:00:00Z","closedAt":"2026-01-01T00:00:00Z","labels":{"nodes":[]}},
				{"number":2,"state":"CLOSED","url":"https://x/2","createdAt":"2026-01-01T00:00:00Z",
				"updatedAt":"2026-01-01T00:00:00Z","closedAt":"2026-01-01T00:00:00Z","labels":{"nodes":[]}}]}}}`))
			return
		}
		// Issue 1 reappears as open — must win over the closed copy.
		_, _ = w.Write([]byte(`{"data":{"search":{"pageInfo":{"hasNextPage":false,"endCursor":null},
			"nodes":[{"number":1,"state":"OPEN","url":"https://x/1","createdAt":"2026-01-01T00:00:00Z",
			"updatedAt":"2026-01-02T00:00:00Z","closedAt":null,"labels":{"nodes":[]}}]}}}`))
	})

	issues, err := client.FetchRepoIssues(context.Background(), "acme", "widgets", `label:"Origin/CS"`, 90)
	if err != nil {
		t.Fatalf("FetchRepoIssues: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("expected 2 deduplicated issues, got %d", len(issues))
	}
	for _, issue := range issues {
		if issue.Number == 1 && issue.State != "OPEN" {
			t.Errorf("expected issue #1's open copy to win, got state=%s", issue.State)
		}
	}
}
