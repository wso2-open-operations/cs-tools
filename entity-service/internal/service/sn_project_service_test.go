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

package service

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestSNProjectService_SearchProjects_MapsAccountRef verifies that the account
// reference newly added to ServiceNow's project search response is mapped into
// domain.ProjectView.Account, and that a project with no linked account (blank
// id/name) maps to a nil Account rather than a zero-valued ref.
func TestSNProjectService_SearchProjects_MapsAccountRef(t *testing.T) {
	const accountSysid = "4a6fc0623b16c31091404c6aa5e45a09"

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"projects": []map[string]any{
				{
					"id": "11111111111111111111111111111111", "name": "With account", "key": "WA",
					"type": map[string]any{"name": "Subscription"},
					"endDate": "", "createdOn": "2026-01-01 00:00:00",
					"account": map[string]any{"id": accountSysid, "name": "Automation Test Customer Account"},
				},
				{
					"id": "22222222222222222222222222222222", "name": "No account", "key": "NA",
					"type": map[string]any{"name": "Subscription"},
					"endDate": "", "createdOn": "2026-01-01 00:00:00",
					"account": map[string]any{"id": "", "name": ""},
				},
			},
			"totalRecords": 2, "offset": 0, "limit": 10,
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	resp, err := svc.SearchProjects(contextWithUserIDToken("token"), domain.SearchProjectsRequest{
		Pagination: domain.Pagination{Limit: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(resp.Projects))
	}

	withAccount := resp.Projects[0]
	if withAccount.Account == nil {
		t.Fatalf("expected non-nil Account for project with a linked account")
	}
	if withAccount.Account.ID != sysidToUUID(accountSysid) || withAccount.Account.Name != "Automation Test Customer Account" {
		t.Fatalf("unexpected Account: %+v", withAccount.Account)
	}

	noAccount := resp.Projects[1]
	if noAccount.Account != nil {
		t.Fatalf("expected nil Account for project with no linked account, got %+v", noAccount.Account)
	}
}
