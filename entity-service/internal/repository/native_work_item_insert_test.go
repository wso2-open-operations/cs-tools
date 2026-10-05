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

package repository

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// There is no SQL-mock harness here and no database in unit tests, so these
// tests assert on the statement text. They cannot prove the SQL executes.

// wso2IDRequiredTypes mirrors the work_item_wso2_id_required_by_type CHECK.
var wso2IDRequiredTypes = map[string]bool{
	"CASE": true, "SERVICE_REQUEST": true, "ANNOUNCEMENT": true,
	"ENGAGEMENT": true, "SECURITY_REPORT_ANALYSIS": true,
}

func TestNativeWorkItemInsertsUsePerTypeNumberingAndUUIDv7(t *testing.T) {
	cases := []struct {
		name, typ, query string
	}{
		{"case", "CASE", createCasePortalQuery},
		{"announcement", "ANNOUNCEMENT", createAnnouncementPortalQuery},
		{"service request", "SERVICE_REQUEST", createServiceRequestPortalQuery},
		{"engagement", "ENGAGEMENT", createEngagementPortalQuery},
		{"security report analysis", "SECURITY_REPORT_ANALYSIS", createSecurityReportAnalysisPortalQuery},
		{"incident", "INCIDENT", createIncidentPortalQuery},
		{"problem", "PROBLEM", createProblemPortalQuery},
		{"change request", "CHANGE_REQUEST", createChangeRequestPortalQuery},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := c.query
			if want := "next_work_item_number('" + c.typ + "'::work_item_type_enum)"; !strings.Contains(q, want) {
				t.Errorf("missing %s", want)
			}
			if !strings.Contains(q, "uuidv7()") {
				t.Error("id must come from uuidv7()")
			}
			for _, banned := range []string{"next_portal_", "gen_random_uuid()"} {
				if strings.Contains(q, banned) {
					t.Errorf("must not contain %s", banned)
				}
			}
			if got := strings.Count(q, "next_work_item_number("); got != 1 {
				t.Errorf("next_work_item_number called %d times, want 1", got)
			}
			// wso2_id is generated exactly for the types the CHECK requires it for.
			hasWso2 := strings.Contains(q, "next_wso2_id($2::uuid)")
			if hasWso2 != wso2IDRequiredTypes[c.typ] {
				t.Errorf("next_wso2_id present=%v, required by type=%v", hasWso2, wso2IDRequiredTypes[c.typ])
			}
			if !wso2IDRequiredTypes[c.typ] && strings.Contains(q, "wso2_id") && strings.Contains(q, "INSERT INTO work_item (\n\t\t\tid, created_on, updated_on, created_by, updated_by,\n\t\t\tnumber, wso2_id") {
				t.Error("a type without a wso2_id requirement must not insert one")
			}
		})
	}
}

// The external-id statements (dual-write) must stay byte-identical to the
// base branch. Hashes were taken from the base text before this change.
func TestExternalIDWorkItemInsertsUnchanged(t *testing.T) {
	golden := map[string]string{
		"createCaseFromServiceNowQuery":                   "80763ef809e8437fbecfa660d408135c727f12b675757f910ebc06f5083fc908",
		"createAnnouncementFromServiceNowQuery":           "a6aa7bb546cc7bd34032246653bef1a5dc0286ef120d05f6f01a5e77022a5a3c",
		"createServiceRequestFromServiceNowQuery":         "63eb289346d2007453ce785fd22b17f5e91696e2a4b4536c442b4caacd3b7c27",
		"createEngagementFromServiceNowQuery":             "c0af1a11dc3085a67057ea3ec5f14fb3563eab303e59a9b3121084f081e32e49",
		"createSecurityReportAnalysisFromServiceNowQuery": "0a7bdc1585c1ee4a93f364b74301a44a9eb63ef8dd2c92bac0b153ec963d0dc8",
		"createChangeRequestFromServiceNowQuery":          "f48beaf9d5f37d30094a208be05c2ed1f16ca34b90b8e57ad59a76a3cfd71e97",
		"createIncidentFromServiceNowQuery":               "587bd8f1d513694b4357275819faac58f0d6993d3babb545b606db2845e1109d",
		"createProblemFromServiceNowQuery":                "a858fd63048e12c04b8ab47dcfc3d444a39debfb8f9a0b80a352d650a41a7c63",
	}
	actual := map[string]string{
		"createCaseFromServiceNowQuery":                   createCaseFromServiceNowQuery,
		"createAnnouncementFromServiceNowQuery":           createAnnouncementFromServiceNowQuery,
		"createServiceRequestFromServiceNowQuery":         createServiceRequestFromServiceNowQuery,
		"createEngagementFromServiceNowQuery":             createEngagementFromServiceNowQuery,
		"createSecurityReportAnalysisFromServiceNowQuery": createSecurityReportAnalysisFromServiceNowQuery,
		"createChangeRequestFromServiceNowQuery":          createChangeRequestFromServiceNowQuery,
		"createIncidentFromServiceNowQuery":               createIncidentFromServiceNowQuery,
		"createProblemFromServiceNowQuery":                createProblemFromServiceNowQuery,
	}
	for name, want := range golden {
		if got := fmt.Sprintf("%x", sha256.Sum256([]byte(actual[name]))); got != want {
			t.Errorf("%s changed: sha256 %s, want %s", name, got, want)
		}
	}
}
