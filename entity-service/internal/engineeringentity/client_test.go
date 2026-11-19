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

package engineeringentity

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

func tokenServer(t *testing.T, counter *int32, tokens ...string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(atomic.AddInt32(counter, 1))
		if err := r.ParseForm(); err != nil || r.PostForm.Get("grant_type") != "client_credentials" ||
			r.PostForm.Get("client_id") != "cid" || r.PostForm.Get("client_secret") != "csecret" {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		tok := tokens[len(tokens)-1]
		if n <= len(tokens) {
			tok = tokens[n-1]
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": tok, "expires_in": 3600})
	}))
}

func TestClient_TokenFetchedOnceAndReused(t *testing.T) {
	var tokenCalls int32
	ts := tokenServer(t, &tokenCalls, "tok1")
	defer ts.Close()
	var gotAuth []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = append(gotAuth, r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"id":1,"nodeId":"I_1","number":7,"title":"t","state":"open","labels":["a"],"htmlUrl":"https://github.com/o/r/issues/7","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","assigneeLogins":[]}`))
	}))
	defer api.Close()
	c := New(api.URL+"/", ClientCredentialsConfig{TokenURL: ts.URL, ClientID: "cid", ClientSecret: "csecret"})
	for i := 0; i < 2; i++ {
		d, err := c.GetIssueDetail(context.Background(), "o", "o", "r", 7)
		if err != nil || d.Number != 7 || d.NodeID != "I_1" || d.HTMLURL == "" {
			t.Fatalf("detail=%+v err=%v", d, err)
		}
	}
	if tokenCalls != 1 {
		t.Fatalf("token fetched %d times, want 1", tokenCalls)
	}
	for _, a := range gotAuth {
		if a != "Bearer tok1" {
			t.Fatalf("Authorization = %q", a)
		}
	}
}

func TestClient_RefreshesOnceOn401(t *testing.T) {
	var tokenCalls int32
	ts := tokenServer(t, &tokenCalls, "old", "new")
	defer ts.Close()
	var calls int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.Header.Get("Authorization") != "Bearer new" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer api.Close()
	c := New(api.URL, ClientCredentialsConfig{TokenURL: ts.URL, ClientID: "cid", ClientSecret: "csecret"})
	if _, err := c.ListIssueDetails(context.Background(), "o", "o", "r", ListIssueDetailsQuery{}); err != nil {
		t.Fatal(err)
	}
	if tokenCalls != 2 || calls != 2 {
		t.Fatalf("tokenCalls=%d apiCalls=%d, want 2/2", tokenCalls, calls)
	}
}

func TestClient_Persistent401IsReportedAfterOneRetry(t *testing.T) {
	var tokenCalls int32
	ts := tokenServer(t, &tokenCalls, "a", "b", "c")
	defer ts.Close()
	var calls int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer api.Close()
	c := New(api.URL, ClientCredentialsConfig{TokenURL: ts.URL, ClientID: "cid", ClientSecret: "csecret"})
	_, err := c.GetProjectStatusHistory(context.Background(), "o", "o", "r", 1)
	var ee *Error
	if !errors.As(err, &ee) || ee.StatusCode != 401 || calls != 2 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestClient_NoAuthModeSendsNoAuthorization(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Header["Authorization"]; ok {
			t.Errorf("Authorization header sent in no-auth mode")
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer api.Close()
	c := New(api.URL, ClientCredentialsConfig{})
	if c.AuthEnabled() {
		t.Fatal("auth should be off")
	}
	if _, err := c.ListIssueComments(context.Background(), "o", "o", "r", 3, ListCommentsQuery{}); err != nil {
		t.Fatal(err)
	}
}

func TestClient_ErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		status     int
		retryAfter string
		wantRA     time.Duration
		definite   bool
	}{
		{404, "", 0, true},
		{422, "", 0, true},
		{400, "", 0, true},
		{401, "", 0, true},
		{403, "", 0, true},
		{408, "", 0, false},
		{409, "", 0, false},
		{425, "", 0, false},
		{429, "7", 7 * time.Second, false},
		{429, "soon", 0, false},
		{500, "", 0, false},
		{503, "3", 3 * time.Second, false},
	} {
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tc.retryAfter != "" {
				w.Header().Set("Retry-After", tc.retryAfter)
			}
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(`{"message":"upstream secret detail"}`))
		}))
		c := New(api.URL, ClientCredentialsConfig{})
		_, err := c.CreateIssue(context.Background(), "o", "o", "r", "t", "b", nil)
		api.Close()
		var ee *Error
		if !errors.As(err, &ee) {
			t.Fatalf("status %d: err=%v", tc.status, err)
		}
		if ee.StatusCode != tc.status || ee.RetryAfter != tc.wantRA || ee.Definite() != tc.definite {
			t.Errorf("status %d: %+v definite=%v", tc.status, ee, ee.Definite())
		}
		if strings.Contains(ee.Error(), "upstream secret detail") {
			t.Errorf("error carries the upstream body: %v", ee)
		}
	}
}

func TestClient_TransportFailureIsStatusZero(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := api.URL
	api.Close()
	c := New(url, ClientCredentialsConfig{})
	_, err := c.GetIssueDetail(context.Background(), "o", "o", "r", 1)
	var ee *Error
	if !errors.As(err, &ee) || ee.StatusCode != 0 || ee.Definite() {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(err.Error(), url) {
		t.Errorf("error leaks the URL: %v", err)
	}
}

func TestClient_TokenFailureDoesNotLeakSecret(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }))
	defer ts.Close()
	c := New("http://127.0.0.1:1", ClientCredentialsConfig{TokenURL: ts.URL, ClientID: "cid", ClientSecret: "csecret"})
	_, err := c.GetIssueDetail(context.Background(), "o", "o", "r", 1)
	if err == nil || strings.Contains(err.Error(), "csecret") {
		t.Fatalf("err=%v", err)
	}
}

func TestClient_PathEscapingAndQuery(t *testing.T) {
	var gotURI, gotMethod string
	var gotBody map[string]any
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI, gotMethod = r.RequestURI, r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":1,"nodeId":"n","number":9}`))
	}))
	defer api.Close()
	c := New(api.URL, ClientCredentialsConfig{})
	ref, err := c.CreateIssue(context.Background(), "wso2/x", "own er", "re?po#", "T", "B", []string{"Origin/CS"})
	if err != nil || ref.Number != 9 || ref.NodeID != "n" {
		t.Fatalf("ref=%+v err=%v", ref, err)
	}
	if gotMethod != "POST" || gotURI != "/orgs/wso2%2Fx/repos/own%20er/re%3Fpo%23/issues" {
		t.Fatalf("method=%s uri=%s", gotMethod, gotURI)
	}
	if gotBody["title"] != "T" || gotBody["body"] != "B" {
		t.Fatalf("body=%v", gotBody)
	}

	_, _ = c.ListIssueDetails(context.Background(), "o", "o", "r", ListIssueDetailsQuery{
		State: "all", Labels: []string{"Origin/CS", "Type/Query"}, Since: "2026-01-01T00:00:00Z", PerPage: 50, Page: 2})
	if !strings.HasPrefix(gotURI, "/orgs/o/repos/o/r/issue-details?") ||
		!strings.Contains(gotURI, "labels=Origin%2FCS%2CType%2FQuery") || !strings.Contains(gotURI, "perPage=50") ||
		!strings.Contains(gotURI, "since=2026-01-01T00%3A00%3A00Z") || !strings.Contains(gotURI, "page=2") {
		t.Fatalf("uri=%s", gotURI)
	}
}

func TestClient_InvalidateTokenOnlyDropsTheTokenTheFailedRequestUsed(t *testing.T) {
	c := New("http://x", ClientCredentialsConfig{TokenURL: "http://t", ClientID: "cid", ClientSecret: "csecret"})
	c.cachedToken, c.tokenExpiry = "fresh", time.Now().Add(time.Hour)
	c.invalidateToken("stale") // a concurrent caller already refreshed
	if tok, ok := c.cachedValidToken(); !ok || tok != "fresh" {
		t.Fatalf("a newer token was thrown away: %q %v", tok, ok)
	}
	c.invalidateToken("fresh")
	if _, ok := c.cachedValidToken(); ok {
		t.Fatal("the token the request used should have been dropped")
	}
}

func TestClient_WaitingForTheTokenRefreshHonoursItsOwnContext(t *testing.T) {
	c := New("http://x", ClientCredentialsConfig{TokenURL: "http://t", ClientID: "cid", ClientSecret: "csecret"})
	c.refresh <- struct{}{} // another caller is mid-fetch
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.accessToken(ctx)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 2*time.Second {
		t.Fatalf("err=%v after %v", err, time.Since(start))
	}
}

const itemsWithContentPage = `{
 "items": [
  {"nodeId":"PVTI_1","type":"ISSUE",
   "content":{"nodeId":"I_1","repository":"example-org/example-repo","number":1,"title":"A title","url":"https://github.com/example-org/example-repo/issues/1",
     "state":"CLOSED","stateReason":"COMPLETED","isPullRequest":false,"body":"b","labels":["Origin/CS","Priority/High(P2)"],"assignees":["octo"],
     "parent":null,"createdAt":"2026-11-17T05:20:22Z","updatedAt":"2026-11-17T05:20:30Z","closedAt":"2026-11-17T06:00:00Z"},
   "draft":null,
   "fieldValues":[{"name":"Target","dataType":"NUMBER","value":3},{"name":"Status","dataType":"SINGLE_SELECT","value":"WOC"}]},
  {"nodeId":"PVTI_2","type":"DRAFT_ISSUE","content":null,"draft":{"title":"d","body":""},"fieldValues":[]},
  {"nodeId":"PVTI_3","type":"ISSUE",
   "content":{"nodeId":"I_3","repository":"example-org/example-repo","number":3,"title":"T3","url":"u","state":"OPEN","stateReason":null,"isPullRequest":false,"body":"",
     "labels":[],"assignees":[],"createdAt":"2026-11-17T05:20:22Z","updatedAt":"2026-11-17T05:20:30Z","closedAt":null},
   "fieldValues":[]}
 ],
 "nextCursor": "Y3Vyc29y"
}`

func TestClient_ListProjectItemsWithContent(t *testing.T) {
	var gotPath, gotQuery string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.EscapedPath(), r.URL.RawQuery
		_, _ = w.Write([]byte(itemsWithContentPage))
	}))
	defer api.Close()
	c := New(api.URL, ClientCredentialsConfig{})

	page, err := c.ListProjectItemsWithContent(context.Background(), "example org", 112, "after/1", 50)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/orgs/example%20org/projects/112/items-with-content" {
		t.Errorf("path = %q", gotPath)
	}
	if gotQuery != "after=after%2F1&perPage=50" {
		t.Errorf("query = %q", gotQuery)
	}
	if len(page.Items) != 3 || page.NextCursor == nil || *page.NextCursor != "Y3Vyc29y" {
		t.Fatalf("page = %+v", page)
	}
	first := page.Items[0]
	if first.Content == nil || first.Content.NodeID != "I_1" || first.Content.Number != 1 || first.Content.State != "CLOSED" ||
		first.Content.StateReason == nil || *first.Content.StateReason != "COMPLETED" || len(first.Content.Labels) != 2 ||
		first.Content.ClosedAt == nil || first.Content.IsPullRequest {
		t.Errorf("content = %+v", first.Content)
	}
	if first.Content.RepositoryOwner() != "example-org" || first.Content.RepositoryName() != "example-repo" {
		t.Errorf("owner/name = %q %q", first.Content.RepositoryOwner(), first.Content.RepositoryName())
	}
	if s, ok := first.Status(); !ok || s != "WOC" {
		t.Errorf("status = %q %v", s, ok)
	}
	if page.Items[1].Content != nil || page.Items[1].Type != "DRAFT_ISSUE" {
		t.Errorf("draft item = %+v", page.Items[1])
	}
	if _, ok := page.Items[2].Status(); ok {
		t.Error("an item with no Status value reports one")
	}
	if (ProjectItemContent{Repository: "norepo"}).RepositoryOwner() != "" || (ProjectItemContent{Repository: "norepo"}).RepositoryName() != "" {
		t.Error("a repository without a slash must split to empty parts")
	}

	// First page: no cursor, default page size.
	if _, err := c.ListProjectItemsWithContent(context.Background(), "o", 1, "", 0); err != nil || gotQuery != "" {
		t.Errorf("first page query = %q err=%v", gotQuery, err)
	}
}

func TestClient_ListProjectItemsWithContent_Errors(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "42")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer api.Close()
	_, err := New(api.URL, ClientCredentialsConfig{}).ListProjectItemsWithContent(context.Background(), "o", 1, "", 10)
	var ee *Error
	if !errors.As(err, &ee) || ee.StatusCode != http.StatusTooManyRequests || ee.RetryAfter != 42*time.Second {
		t.Fatalf("err = %v", err)
	}
}
