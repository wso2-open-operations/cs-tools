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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestCaseSearchOrder_OnlyEmitsKnownColumnsAndDirections(t *testing.T) {
	cases := []struct {
		name     string
		sort     domain.CaseSort
		col, dir string
	}{
		{"defaults", domain.CaseSort{}, "wi.created_on", "DESC"},
		{"asc", domain.CaseSort{Field: domain.CaseSortFieldUpdatedOn, Order: domain.CaseSortOrderAsc}, "wi.updated_on", "ASC"},
		{"upper-case ASC", domain.CaseSort{Field: domain.CaseSortFieldSeverity, Order: "ASC"}, "c.severity", "ASC"},
		{"desc", domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: domain.CaseSortOrderDesc}, "wi.created_on", "DESC"},
		{"injected order", domain.CaseSort{Field: domain.CaseSortFieldCreatedOn, Order: "DESC; DROP TABLE work_item --"}, "wi.created_on", "DESC"},
		{"unknown field", domain.CaseSort{Field: "wi.id; --", Order: domain.CaseSortOrderAsc}, "wi.created_on", "ASC"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			col, dir := caseSearchOrder(tc.sort)
			if col != tc.col || dir != tc.dir {
				t.Errorf("caseSearchOrder(%+v) = %q %q, want %q %q", tc.sort, col, dir, tc.col, tc.dir)
			}
		})
	}
}
