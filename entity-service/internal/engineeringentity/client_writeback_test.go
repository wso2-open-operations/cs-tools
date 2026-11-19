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
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type seen struct {
	method, path, rawQuery string
	body                   map[string]any
}

func capture(t *testing.T, status int, headers map[string]string, answer string) (*Client, *seen) {
	t.Helper()
	s := &seen{}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.method, s.path, s.rawQuery = r.Method, r.URL.EscapedPath(), r.URL.RawQuery
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			_ = json.Unmarshal(b, &s.body)
		}
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(api.Close)
	return New(api.URL, ClientCredentialsConfig{}), s
}

func TestClient_AddIssueComment(t *testing.T) {
	c, s := capture(t, 201, nil, `{"id":6939366954,"nodeId":"IC_1","body":"hi","createdAt":"2026-11-17T05:20:29Z","updatedAt":"2026-11-17T05:21:00Z"}`)
	got, err := c.AddIssueComment(context.Background(), "org", "own", "repo", 4, "hi")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 6939366954 || got.UpdatedAt != "2026-11-17T05:21:00Z" || got.AuthorLogin != nil {
		t.Fatalf("got %+v", got)
	}
	if s.method != "POST" || s.path != "/orgs/org/repos/own/repo/issues/4/comments" || s.body["body"] != "hi" {
		t.Fatalf("request %+v", s)
	}
}

func TestClient_AddIssueComment_NoIDIsAmbiguous(t *testing.T) {
	c, _ := capture(t, 200, nil, `{}`)
	_, err := c.AddIssueComment(context.Background(), "o", "o", "r", 1, "x")
	var ee *Error
	if !errors.As(err, &ee) || ee.StatusCode != 502 || ee.Definite() {
		t.Fatalf("err = %v", err)
	}
}

func TestClient_CloseIssue(t *testing.T) {
	c, s := capture(t, 200, nil, `{"id":1,"nodeId":"I_1","number":4,"state":"closed","stateReason":"completed","title":"t","body":null,"labels":[]}`)
	got, err := c.CloseIssue(context.Background(), "org", "own", "repo", 4, "completed")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "closed" || got.Number != 4 || got.StateReason == nil || *got.StateReason != "completed" {
		t.Fatalf("got %+v", got)
	}
	if s.method != "PATCH" || s.path != "/orgs/org/repos/own/repo/issues/4" || s.body["state"] != "closed" || s.body["stateReason"] != "completed" {
		t.Fatalf("request %+v", s)
	}
}

func TestClient_ListProjectFields(t *testing.T) {
	c, s := capture(t, 200, nil, `[{"id":445481681,"nodeId":"PVTSSF_1","name":"Status","dataType":"single_select","options":[{"id":"f75ad846","name":"WOC"},{"id":"47fc9ee4","name":"WOW"}]},{"id":445481679,"name":"Title","dataType":"title","options":null}]`)
	got, err := c.ListProjectFields(context.Background(), "org", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != 445481681 || got[0].Options[1].ID != "47fc9ee4" || got[0].Options[1].Name != "WOW" || got[1].Options != nil {
		t.Fatalf("got %+v", got)
	}
	if s.method != "GET" || s.path != "/orgs/org/projects/3/fields" {
		t.Fatalf("request %+v", s)
	}
}

func TestClient_ListProjectItems(t *testing.T) {
	c, s := capture(t, 200, nil, `[{"id":307799589,"nodeId":"PVTI_1","content":{"id":6437972409,"nodeId":"I_kwDOVxO0As8AAAABf7upuQ","number":1},"fields":[{"id":445481679,"name":"Title","dataType":"title","options":null,"value":null}]}]`)
	got, err := c.ListProjectItems(context.Background(), "org", 1, ListProjectItemsQuery{PerPage: 100, Q: "is:open"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 307799589 || got[0].Content.NodeID != "I_kwDOVxO0As8AAAABf7upuQ" || got[0].Content.Number != 1 {
		t.Fatalf("got %+v", got)
	}
	if s.path != "/orgs/org/projects/1/items" || s.rawQuery != "perPage=100&q=is%3Aopen" {
		t.Fatalf("request %+v", s)
	}
}

func TestClient_UpdateProjectItemStatus(t *testing.T) {
	c, s := capture(t, 200, nil, `{"id":307799589,"nodeId":"PVTI_1","content":{"id":1,"nodeId":"I_1","number":1}}`)
	if err := c.UpdateProjectItemStatus(context.Background(), "org", 1, 307799589, 445481681, "47fc9ee4"); err != nil {
		t.Fatal(err)
	}
	if s.method != "PATCH" || s.path != "/orgs/org/projects/1/items/307799589" {
		t.Fatalf("request %+v", s)
	}
	fields, _ := s.body["fields"].([]any)
	if len(fields) != 1 {
		t.Fatalf("body %+v", s.body)
	}
	f := fields[0].(map[string]any)
	if f["id"] != float64(445481681) || f["value"] != "47fc9ee4" {
		t.Fatalf("field %+v (the value must be the option id as a string)", f)
	}
}

func TestClient_ErrorCarriesGitHubStatusAndRetryAfter(t *testing.T) {
	c, _ := capture(t, 500, map[string]string{"X-GitHub-Status": "403", "Retry-After": "42"}, `{"message":"Failed","githubStatus":403}`)
	_, err := c.AddIssueComment(context.Background(), "o", "o", "r", 1, "x")
	var ee *Error
	if !errors.As(err, &ee) {
		t.Fatalf("err = %v", err)
	}
	if ee.StatusCode != 500 || ee.GitHubStatus != 403 || ee.RetryAfter != 42*time.Second {
		t.Fatalf("got %+v", ee)
	}
}

func TestClient_ErrorWithoutGitHubStatusHeader(t *testing.T) {
	c, _ := capture(t, 502, map[string]string{"X-GitHub-Status": "junk"}, `{}`)
	err := c.UpdateProjectItemStatus(context.Background(), "o", 1, 1, 1, "x")
	var ee *Error
	if !errors.As(err, &ee) || ee.GitHubStatus != 0 || ee.StatusCode != 502 {
		t.Fatalf("err = %#v", err)
	}
}

func TestClient_TransportFailureHasNoGitHubStatus(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := api.URL
	api.Close()
	_, err := New(url, ClientCredentialsConfig{}).ListProjectFields(context.Background(), "o", 1)
	var ee *Error
	if !errors.As(err, &ee) || ee.StatusCode != 0 || ee.GitHubStatus != 0 {
		t.Fatalf("err = %#v", err)
	}
}
