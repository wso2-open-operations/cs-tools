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
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// The client calls reuse newCustomDashboardTestClient and userContext
// (custom_dashboard_test.go).

func TestListCodeDashboards(t *testing.T) {
	client, got := newCustomDashboardTestClient(t, http.StatusOK,
		`{"dashboards":[{"id":"`+testCustomDashboardID+`","name":"Open Cases","createdOn":"2026-11-13T09:21:45Z","updatedOn":"2026-11-13T09:22:00Z"}]}`)
	list, err := client.ListCodeDashboards(userContext())
	if err != nil {
		t.Fatalf("ListCodeDashboards: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/users/me/code-dashboards" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.userToken != "user-token" {
		t.Errorf("x-user-id-token = %q, want the caller's token forwarded", got.userToken)
	}
	if len(list) != 1 || list[0].Name != "Open Cases" || list[0].UpdatedOn != "2026-11-13T09:22:00Z" {
		t.Fatalf("list = %+v", list)
	}
}

func TestListCodeDashboards_EmptyIsNotNil(t *testing.T) {
	client, _ := newCustomDashboardTestClient(t, http.StatusOK, `{"dashboards":null}`)
	list, err := client.ListCodeDashboards(userContext())
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("list = %#v, %v; want an empty, non-nil slice", list, err)
	}
}

func TestGetCodeDashboard(t *testing.T) {
	client, got := newCustomDashboardTestClient(t, http.StatusOK,
		`{"id":"`+testCustomDashboardID+`","name":"Open Cases","source":"function render() {}","createdOn":"x","updatedOn":"y"}`)
	d, err := client.GetCodeDashboard(userContext(), testCustomDashboardID)
	if err != nil {
		t.Fatalf("GetCodeDashboard: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/users/me/code-dashboards/"+testCustomDashboardID {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if d.Source != "function render() {}" || d.Name != "Open Cases" || d.CreatedOn != "x" {
		t.Fatalf("dashboard = %+v", d)
	}
}

func TestCodeDashboardClient_EscapesID(t *testing.T) {
	client, got := newCustomDashboardTestClient(t, http.StatusBadRequest, `{"code":400,"message":"not a UUID"}`)
	_, _ = client.GetCodeDashboard(userContext(), "../users/x")
	if got.rawPath != "/users/me/code-dashboards/..%2Fusers%2Fx" {
		t.Errorf("escaped path = %q, want the id kept as one segment", got.rawPath)
	}
}

func TestSaveCodeDashboard(t *testing.T) {
	client, got := newCustomDashboardTestClient(t, http.StatusOK,
		`{"id":"`+testCustomDashboardID+`","name":"Open Cases","source":"function render() {}","createdOn":"x","updatedOn":"y"}`)
	d, err := client.SaveCodeDashboard(userContext(), testCustomDashboardID, "Open Cases", "function render() {}")
	if err != nil {
		t.Fatalf("SaveCodeDashboard: %v", err)
	}
	if got.method != http.MethodPut || got.path != "/users/me/code-dashboards/"+testCustomDashboardID {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(got.body), &body); err != nil {
		t.Fatalf("request body %q: %v", got.body, err)
	}
	if len(body) != 2 || body["name"] != "Open Cases" || body["source"] != "function render() {}" {
		t.Errorf("request body = %v, want exactly name and source", body)
	}
	if d.ID != testCustomDashboardID || d.Source != "function render() {}" {
		t.Errorf("dashboard = %+v", d)
	}
}

func TestDeleteCodeDashboard(t *testing.T) {
	client, got := newCustomDashboardTestClient(t, http.StatusNoContent, "")
	if err := client.DeleteCodeDashboard(userContext(), testCustomDashboardID); err != nil {
		t.Fatalf("DeleteCodeDashboard: %v", err)
	}
	if got.method != http.MethodDelete || got.path != "/users/me/code-dashboards/"+testCustomDashboardID {
		t.Errorf("request = %s %s", got.method, got.path)
	}
}

func TestCodeDashboardClient_PassesStatusErrorsThrough(t *testing.T) {
	cases := []struct {
		name   string
		status int
		call   func(*CustomerEntityClient) error
	}{
		{"get missing", http.StatusNotFound, func(c *CustomerEntityClient) error {
			_, err := c.GetCodeDashboard(userContext(), testCustomDashboardID)
			return err
		}},
		{"save name taken", http.StatusConflict, func(c *CustomerEntityClient) error {
			_, err := c.SaveCodeDashboard(userContext(), testCustomDashboardID, "X", "x")
			return err
		}},
		{"delete missing", http.StatusNotFound, func(c *CustomerEntityClient) error {
			return c.DeleteCodeDashboard(userContext(), testCustomDashboardID)
		}},
		{"list unauthorized", http.StatusUnauthorized, func(c *CustomerEntityClient) error {
			_, err := c.ListCodeDashboards(userContext())
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := newCustomDashboardTestClient(t, tc.status, `{"code":0,"message":"from entity-service"}`)
			err := tc.call(client)
			var apiErr *apierror.Error
			if !errors.As(err, &apiErr) || apiErr.StatusCode != tc.status {
				t.Fatalf("err = %v, want *apierror.Error with status %d", err, tc.status)
			}
		})
	}
}

const testCodeShareUserID = "5e636a12-0000-4000-8000-0000000000aa"

// Each sharing call goes to the code dashboard route with the method and ids
// it names, forwards the caller's token and decodes the answer.
func TestCodeDashboardSharingCalls(t *testing.T) {
	own := "/users/me/code-dashboards/" + testCustomDashboardID
	shares := `{"shares":[{"user":{"id":"` + testCodeShareUserID + `","email":"alice@example.com","name":"Alice"},"sharedOn":"2026-11-13T10:00:00Z"}]}`
	teams := `{"shares":[{"team":{"id":"t1","name":"Vega","type":"cre-abt"},"sharedOn":"2026-11-13T10:00:00Z"}]}`
	link := `{"enabled":true,"linkId":"l1","openedBy":2}`
	cases := []struct {
		name, status, body, wantMethod, wantPath string
		call                                     func(c *CustomerEntityClient) (any, error)
		check                                    func(v any) bool
	}{
		{"list shares", "200", shares, http.MethodGet, own + "/shares",
			func(c *CustomerEntityClient) (any, error) {
				return c.ListCodeDashboardShares(userContext(), testCustomDashboardID)
			},
			func(v any) bool { s := v.([]CustomDashboardShare); return len(s) == 1 && s[0].User.Name == "Alice" }},
		{"add share", "200", shares, http.MethodPut, own + "/shares/" + testCodeShareUserID,
			func(c *CustomerEntityClient) (any, error) {
				return c.AddCodeDashboardShare(userContext(), testCustomDashboardID, testCodeShareUserID)
			},
			func(v any) bool { return len(v.([]CustomDashboardShare)) == 1 }},
		{"remove share", "204", "", http.MethodDelete, own + "/shares/" + testCodeShareUserID,
			func(c *CustomerEntityClient) (any, error) {
				return nil, c.RemoveCodeDashboardShare(userContext(), testCustomDashboardID, testCodeShareUserID)
			},
			func(any) bool { return true }},
		{"list team shares", "200", teams, http.MethodGet, own + "/team-shares",
			func(c *CustomerEntityClient) (any, error) {
				return c.ListCodeDashboardTeamShares(userContext(), testCustomDashboardID)
			},
			func(v any) bool {
				s := v.([]CustomDashboardTeamShare)
				return len(s) == 1 && s[0].Team.Type == "cre-abt"
			}},
		{"add team share", "200", teams, http.MethodPut, own + "/team-shares/t1",
			func(c *CustomerEntityClient) (any, error) {
				return c.AddCodeDashboardTeamShare(userContext(), testCustomDashboardID, "t1")
			},
			func(v any) bool { return len(v.([]CustomDashboardTeamShare)) == 1 }},
		{"remove team share", "204", "", http.MethodDelete, own + "/team-shares/t1",
			func(c *CustomerEntityClient) (any, error) {
				return nil, c.RemoveCodeDashboardTeamShare(userContext(), testCustomDashboardID, "t1")
			},
			func(any) bool { return true }},
		{"get link", "200", link, http.MethodGet, own + "/link",
			func(c *CustomerEntityClient) (any, error) {
				return c.GetCodeDashboardLink(userContext(), testCustomDashboardID)
			},
			func(v any) bool {
				l := v.(CustomDashboardLink)
				return l.Enabled && l.LinkID == "l1" && l.OpenedBy == 2
			}},
		{"enable link", "200", link, http.MethodPut, own + "/link",
			func(c *CustomerEntityClient) (any, error) {
				return c.EnableCodeDashboardLink(userContext(), testCustomDashboardID)
			},
			func(v any) bool { return v.(CustomDashboardLink).Enabled }},
		{"disable link", "204", "", http.MethodDelete, own + "/link",
			func(c *CustomerEntityClient) (any, error) {
				return nil, c.DisableCodeDashboardLink(userContext(), testCustomDashboardID)
			},
			func(any) bool { return true }},
		{"list shared", "200", `{"dashboards":[{"id":"d1","name":"Open Cases","owner":null,"sharedViaTeam":null,"sharedViaLink":true}]}`, http.MethodGet, "/users/me/shared-code-dashboards",
			func(c *CustomerEntityClient) (any, error) { return c.ListSharedCodeDashboards(userContext()) },
			func(v any) bool {
				s := v.([]SharedCustomDashboardSummary)
				return len(s) == 1 && s[0].SharedViaLink && s[0].Owner == nil
			}},
		{"get shared", "200", `{"id":"d1","name":"Open Cases","source":"function render() {}","sharedViaTeam":{"id":"t1","name":"Vega","type":"cre-abt"}}`, http.MethodGet, "/users/me/shared-code-dashboards/" + testCustomDashboardID,
			func(c *CustomerEntityClient) (any, error) {
				return c.GetSharedCodeDashboard(userContext(), testCustomDashboardID)
			},
			func(v any) bool {
				d := v.(SharedCodeDashboard)
				return d.Source == "function render() {}" && d.SharedViaTeam != nil && d.SharedViaTeam.Name == "Vega"
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status := http.StatusOK
			if tc.status == "204" {
				status = http.StatusNoContent
			}
			client, got := newCustomDashboardTestClient(t, status, tc.body)
			v, err := tc.call(client)
			if err != nil {
				t.Fatalf("call: %v", err)
			}
			if got.method != tc.wantMethod || got.path != tc.wantPath {
				t.Errorf("request = %s %s, want %s %s", got.method, got.path, tc.wantMethod, tc.wantPath)
			}
			if got.userToken != "user-token" {
				t.Errorf("x-user-id-token = %q, want the caller's token forwarded", got.userToken)
			}
			if !tc.check(v) {
				t.Errorf("decoded = %+v", v)
			}
		})
	}
}

func TestCodeDashboardSharingCalls_EmptyListsAreNotNil(t *testing.T) {
	client, _ := newCustomDashboardTestClient(t, http.StatusOK, `{"shares":null,"dashboards":null}`)
	if s, err := client.ListCodeDashboardShares(userContext(), testCustomDashboardID); err != nil || s == nil {
		t.Errorf("ListCodeDashboardShares = %#v, %v", s, err)
	}
	if s, err := client.ListCodeDashboardTeamShares(userContext(), testCustomDashboardID); err != nil || s == nil {
		t.Errorf("ListCodeDashboardTeamShares = %#v, %v", s, err)
	}
	if s, err := client.ListSharedCodeDashboards(userContext()); err != nil || s == nil {
		t.Errorf("ListSharedCodeDashboards = %#v, %v", s, err)
	}
}

// The link id travels in the body, never the path.
func TestOpenCodeDashboardLink(t *testing.T) {
	client, got := newCustomDashboardTestClient(t, http.StatusOK, `{"dashboardId":"d1","own":true}`)
	o, err := client.OpenCodeDashboardLink(userContext(), "secret-link")
	if err != nil || o.DashboardID != "d1" || !o.Own {
		t.Fatalf("OpenCodeDashboardLink = %+v, %v", o, err)
	}
	if got.method != http.MethodPost || got.path != "/users/me/shared-code-dashboards/link" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	var body map[string]string
	if err := json.Unmarshal([]byte(got.body), &body); err != nil || body["linkId"] != "secret-link" {
		t.Errorf("body = %q, want the link id", got.body)
	}
}

func TestCodeDashboardSharingCalls_EscapeIDsAndPassErrorsThrough(t *testing.T) {
	client, got := newCustomDashboardTestClient(t, http.StatusNotFound, `{"code":404,"message":"dashboard not found"}`)
	_, err := client.AddCodeDashboardShare(userContext(), "../x", "a/b")
	if got.rawPath != "/users/me/code-dashboards/..%2Fx/shares/a%2Fb" {
		t.Errorf("escaped path = %q", got.rawPath)
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("err = %v, want the 404 passed through", err)
	}
	_ = client.RemoveCodeDashboardTeamShare(userContext(), "x", "t/1")
	if got.rawPath != "/users/me/code-dashboards/x/team-shares/t%2F1" {
		t.Errorf("escaped team path = %q", got.rawPath)
	}
	_, _ = client.GetSharedCodeDashboard(userContext(), "a/b")
	if got.rawPath != "/users/me/shared-code-dashboards/a%2Fb" {
		t.Errorf("escaped shared path = %q", got.rawPath)
	}
}
