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
	"sort"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestReferenceTypeToWorkItemType_CaseCoversAllCaseLikeTypes locks in a real
// bug found live: ReferenceTypeCase used to map to a single "CASE" value,
// so SearchComments/CreateComment (reached via csm-portal-backend's generic
// POST /comments/search, which injects referenceType:"case") returned zero
// comments for a work_item whose real type is one of the other four
// case-like types (ENGAGEMENT/SERVICE_REQUEST/SECURITY_REPORT_ANALYSIS/
// ANNOUNCEMENT) even though it has real comment rows -- CaseRepository's
// own GetCaseByID/SearchCases have served all five since "Case-like
// work_item types" landed, but this file was never updated to match.
func TestReferenceTypeToWorkItemType_CaseCoversAllCaseLikeTypes(t *testing.T) {
	got := append([]string{}, ReferenceTypeToWorkItemType[domain.ReferenceTypeCase]...)
	sort.Strings(got)

	want := []string{"ANNOUNCEMENT", "CASE", "ENGAGEMENT", "SECURITY_REPORT_ANALYSIS", "SERVICE_REQUEST"}

	if len(got) != len(want) {
		t.Fatalf("ReferenceTypeToWorkItemType[case] = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ReferenceTypeToWorkItemType[case] = %v, want %v", got, want)
		}
	}
}

// TestReferenceTypeToWorkItemType_OthersStaySingleType confirms the
// non-case reference types weren't accidentally widened by the same fix --
// conversation/change_request/incident each map to exactly one work_item
// type, with no case-like family of their own.
func TestReferenceTypeToWorkItemType_OthersStaySingleType(t *testing.T) {
	tests := []struct {
		refType domain.ReferenceType
		want    string
	}{
		{domain.ReferenceTypeConversation, "CONVERSATION"},
		{domain.ReferenceTypeChangeRequest, "CHANGE_REQUEST"},
		{domain.ReferenceTypeIncident, "INCIDENT"},
	}
	for _, tt := range tests {
		got := ReferenceTypeToWorkItemType[tt.refType]
		if len(got) != 1 || got[0] != tt.want {
			t.Errorf("ReferenceTypeToWorkItemType[%q] = %v, want [%q]", tt.refType, got, tt.want)
		}
	}
}
