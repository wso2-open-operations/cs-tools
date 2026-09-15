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

// TestSNProjectService_SearchProjects_MapsAccountIsPartner verifies that the linked
// account's raw partner flag, merged in by the Ballerina layer onto the project search
// row's nested account object, is surfaced as domain.ProjectSearchAccountRef.IsPartner --
// and that a project with no linked account has no IsPartner to report (nil Account).
func TestSNProjectService_SearchProjects_MapsAccountIsPartner(t *testing.T) {
	const accountSysid = "4a6fc0623b16c31091404c6aa5e45a09"

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"projects": []map[string]any{
				{
					"id": "11111111111111111111111111111111", "name": "Partner project", "key": "PP",
					"type":    map[string]any{"name": "Subscription"},
					"endDate": "", "createdOn": "2026-01-01 00:00:00",
					"account": map[string]any{"id": accountSysid, "name": "Partner Co", "partner": true},
				},
				{
					"id": "22222222222222222222222222222222", "name": "No account", "key": "NA",
					"type":    map[string]any{"name": "Subscription"},
					"endDate": "", "createdOn": "2026-01-01 00:00:00",
					"account": map[string]any{"id": "", "name": ""},
				},
			},
			"totalRecords": 2, "offset": 0, "limit": 50,
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	got, err := svc.SearchProjects(contextWithUserIDToken("token"), domain.SearchProjectsRequest{
		Pagination: domain.Pagination{Limit: 50, Offset: 0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Projects) != 2 {
		t.Fatalf("len(Projects) = %d, want 2", len(got.Projects))
	}

	partnerProject := got.Projects[0]
	if partnerProject.Account == nil || partnerProject.Account.IsPartner == nil || !*partnerProject.Account.IsPartner {
		t.Errorf("Projects[0].Account.IsPartner = %v, want true", partnerProject.Account)
	}

	noAccountProject := got.Projects[1]
	if noAccountProject.Account != nil {
		t.Errorf("Projects[1].Account = %v, want nil", noAccountProject.Account)
	}
}

// TestSNProjectService_GetProjectByID_MapsAccountIsPartner verifies the same partner-flag
// passthrough on the single-project detail response's nested account object.
func TestSNProjectService_GetProjectByID_MapsAccountIsPartner(t *testing.T) {
	projectSysid := sysid32('3')
	accountSysid := sysid32('4')

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": projectSysid, "name": "Partner Detail", "key": "PD", "sfId": "sf-3",
			"createdOn": "2026-01-01 00:00:00", "startDate": "2026-01-01", "endDate": "2026-12-31",
			"type":    map[string]any{"name": "Subscription"},
			"account": map[string]any{"id": accountSysid, "name": "Partner Co", "partner": true},
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	got, err := svc.GetProjectByID(contextWithUserIDToken("token"), sysidToUUID(projectSysid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Account.IsPartner == nil || !*got.Account.IsPartner {
		t.Errorf("Account.IsPartner = %v, want true", got.Account.IsPartner)
	}
}
