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
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestSaveSystemDashboard(t *testing.T) {
	client, got := newCustomDashboardTestClient(t, http.StatusOK,
		`{"id":"00000000-0000-0000-0000-000000000001","systemKey":"team-overview","isSystem":true,"name":"Team Overview","definition":"{}","createdOn":"x","updatedOn":"y"}`)
	d, err := client.SaveSystemDashboard(userContext(), "team-overview", "Team Overview", `{"widgets":[]}`)
	if err != nil {
		t.Fatalf("SaveSystemDashboard: %v", err)
	}
	if got.method != http.MethodPut || got.path != "/system-dashboards/team-overview" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if got.userToken != "" {
		t.Errorf("x-user-id-token = %q, want none on a service call", got.userToken)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(got.body), &body); err != nil {
		t.Fatalf("request body %q: %v", got.body, err)
	}
	if len(body) != 2 || body["name"] != "Team Overview" || body["definition"] != `{"widgets":[]}` {
		t.Errorf("request body = %v, want exactly name and definition (as a string)", body)
	}
	if d.Key != "team-overview" || !d.IsSystem {
		t.Errorf("dashboard = %+v", d)
	}
}

func TestListSystemDashboards(t *testing.T) {
	client, got := newCustomDashboardTestClient(t, http.StatusOK,
		`{"dashboards":[{"id":"a","systemKey":"alpha","isSystem":true,"name":"Alpha","definition":"{}","createdOn":"x","updatedOn":"y"}]}`)
	list, err := client.ListSystemDashboards(userContext())
	if err != nil {
		t.Fatalf("ListSystemDashboards: %v", err)
	}
	if got.userToken != "" {
		t.Errorf("x-user-id-token = %q, want none: a system dashboard call must not forward the caller's token", got.userToken)
	}
	if got.method != http.MethodGet || got.path != "/system-dashboards" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if len(list) != 1 || list[0].Key != "alpha" || list[0].Definition != "{}" {
		t.Errorf("list = %+v", list)
	}
}

func TestListSystemDashboards_EmptyIsNotNil(t *testing.T) {
	client, _ := newCustomDashboardTestClient(t, http.StatusOK, `{"dashboards":null}`)
	list, err := client.ListSystemDashboards(context.Background())
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("list = %v, err = %v; want empty non-nil", list, err)
	}
}

func TestListSystemDashboards_StatusErrorPassesThrough(t *testing.T) {
	client, _ := newCustomDashboardTestClient(t, http.StatusForbidden, `{"code":403}`)
	if _, err := client.ListSystemDashboards(context.Background()); err == nil {
		t.Fatal("expected an error for a 403")
	}
}
