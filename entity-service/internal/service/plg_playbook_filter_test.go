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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestValidateOrganizationFiltersChecksPlaybookAnswers: the playbook answer
// filters every PLG search, trend and aggregate shares.
func TestValidateOrganizationFiltersChecksPlaybookAnswers(t *testing.T) {
	yes, no := true, false
	const task = "ELIGIBILITY_ASSESSMENT_OUTCOME"
	for _, c := range []struct {
		name    string
		filters domain.OrganizationSearchFilters
		want    string // "" for accepted
	}{
		{"answers", domain.OrganizationSearchFilters{PlaybookTask: task, PlaybookAnswers: []string{"MONITOR", " EXCLUDED "}}, ""},
		{"unanswered", domain.OrganizationSearchFilters{PlaybookTask: task, PlaybookUnanswered: &yes}, ""},
		{"answers, unanswered false", domain.OrganizationSearchFilters{PlaybookTask: task, PlaybookAnswers: []string{"MONITOR"}, PlaybookUnanswered: &no}, ""},
		{"a task on its own", domain.OrganizationSearchFilters{PlaybookTask: task}, ""},
		{"unanswered false on its own", domain.OrganizationSearchFilters{PlaybookUnanswered: &no}, ""},
		{"100-character codes", domain.OrganizationSearchFilters{PlaybookTask: strings.Repeat("T", 100), PlaybookAnswers: []string{strings.Repeat("A", 100)}}, ""},
		{"a blank task", domain.OrganizationSearchFilters{PlaybookTask: "  ", PlaybookAnswers: []string{"MONITOR"}}, "playbookTask must be 1 to 100 characters"},
		{"a 101-character task", domain.OrganizationSearchFilters{PlaybookTask: strings.Repeat("T", 101), PlaybookAnswers: []string{"MONITOR"}}, "playbookTask must be 1 to 100 characters"},
		{"an empty answer", domain.OrganizationSearchFilters{PlaybookTask: task, PlaybookAnswers: []string{""}}, "playbookAnswers entries must be 1 to 100 characters"},
		{"a blank answer", domain.OrganizationSearchFilters{PlaybookTask: task, PlaybookAnswers: []string{"MONITOR", " "}}, "playbookAnswers entries must be 1 to 100 characters"},
		{"a 101-character answer", domain.OrganizationSearchFilters{PlaybookTask: task, PlaybookAnswers: []string{strings.Repeat("A", 101)}}, "playbookAnswers entries must be 1 to 100 characters"},
		{"answers without a task", domain.OrganizationSearchFilters{PlaybookAnswers: []string{"MONITOR"}}, "need filters.playbookTask"},
		{"unanswered without a task", domain.OrganizationSearchFilters{PlaybookUnanswered: &yes}, "need filters.playbookTask"},
		{"answers and unanswered", domain.OrganizationSearchFilters{PlaybookTask: task, PlaybookAnswers: []string{"MONITOR"}, PlaybookUnanswered: &yes}, "cannot be combined"},
	} {
		err := validateOrganizationFilters(&c.filters)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: err = %v, want accepted", c.name, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: err = %v, want one mentioning %q", c.name, err, c.want)
		}
	}
}
