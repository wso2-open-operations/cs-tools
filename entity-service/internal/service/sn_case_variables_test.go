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
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestSNCaseService_GetCaseByID_MapsVariables verifies that the catalog answers a
// service request was raised with reach the case detail response, in the backing
// data source's own order, and that a case carrying none reports no variables at
// all rather than an empty list.
func TestSNCaseService_GetCaseByID_MapsVariables(t *testing.T) {
	tests := []struct {
		name          string
		variablesJSON string
		want          []domain.CaseVariable
	}{
		{
			name: "service request with answers keeps order",
			variablesJSON: `,
		"variables": [
			{"name": "Existing Product Version/s", "value": "WSO2 API Manager 3.2.0"},
			{"name": "Reason For Migration", "value": "End of support"},
			{"name": "Target Date", "value": "2026-09-01"}
		]`,
			want: []domain.CaseVariable{
				{Name: "Existing Product Version/s", Value: "WSO2 API Manager 3.2.0"},
				{Name: "Reason For Migration", Value: "End of support"},
				{Name: "Target Date", Value: "2026-09-01"},
			},
		},
		{
			name:          "field absent",
			variablesJSON: ``,
			want:          nil,
		},
		{
			name:          "field present but empty",
			variablesJSON: `, "variables": []`,
			want:          nil,
		},
		{
			name:          "field null",
			variablesJSON: `, "variables": null`,
			want:          nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{
				"id": "` + testWLCaseSysid + `",
				"internalId": "WSO2-002",
				"number": "CS0001002",
				"title": "Migration request",
				"description": "Migrate to the latest version",
				"createdOn": "2026-01-01 10:00:00",
				"updatedOn": "2026-01-02 10:00:00",
				"createdBy": "reporter@example.com",
				"project": {"id": "` + testProjectSysid + `", "name": "Project A"},
				"deployment": {"id": "", "name": ""},
				"deployedProduct": {"id": "", "name": "", "version": ""},
				"state": {"id": 1, "label": "Open"}` + tt.variablesJSON + `
			}`

			client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			})

			svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

			cv, err := svc.GetCaseByID(contextWithUserIDToken("token"), sysidToUUID(testWLCaseSysid))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(cv.Variables) != len(tt.want) {
				t.Fatalf("Variables = %+v, want %+v", cv.Variables, tt.want)
			}
			for i, want := range tt.want {
				if cv.Variables[i] != want {
					t.Fatalf("Variables[%d] = %+v, want %+v", i, cv.Variables[i], want)
				}
			}
		})
	}
}
