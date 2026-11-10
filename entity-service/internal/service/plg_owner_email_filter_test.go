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

// TestValidateOrganizationFiltersChecksOwnerEmails: the owner email filter
// every PLG search, trend and aggregate shares.
func TestValidateOrganizationFiltersChecksOwnerEmails(t *testing.T) {
	yes, no := true, false
	long := strings.Repeat("a", 242) + "@example.com" // 254 characters
	for _, c := range []struct {
		name    string
		filters domain.OrganizationSearchFilters
		want    string // "" for accepted
	}{
		{"emails", domain.OrganizationSearchFilters{OwnerEmails: []string{"bob@wso2.com", " Alice@WSO2.com "}}, ""},
		{"emails, unowned false", domain.OrganizationSearchFilters{OwnerEmails: []string{"bob@wso2.com"}, Unowned: &no}, ""},
		{"emails and owner ids", domain.OrganizationSearchFilters{OwnerEmails: []string{"bob@wso2.com"}, OwnerIDs: []string{"0f0e9d4c-6a3b-4c2d-9e8f-7a6b5c4d3e2f"}}, ""},
		{"a 254-character email", domain.OrganizationSearchFilters{OwnerEmails: []string{long}}, ""},
		{"a 254-character email with spaces around it", domain.OrganizationSearchFilters{OwnerEmails: []string{"  " + long + " "}}, ""},
		{"an empty email", domain.OrganizationSearchFilters{OwnerEmails: []string{""}}, "ownerEmails entries must be email addresses of at most 254 characters"},
		{"a blank email", domain.OrganizationSearchFilters{OwnerEmails: []string{"bob@wso2.com", "  "}}, "ownerEmails entries must be email addresses"},
		{"a name, not an email", domain.OrganizationSearchFilters{OwnerEmails: []string{"Bob"}}, "ownerEmails entries must be email addresses"},
		{"an @ alone", domain.OrganizationSearchFilters{OwnerEmails: []string{" @ "}}, "ownerEmails entries must be email addresses"},
		{"no name before the @", domain.OrganizationSearchFilters{OwnerEmails: []string{"@wso2.com"}}, "ownerEmails entries must be email addresses"},
		{"no domain after the @", domain.OrganizationSearchFilters{OwnerEmails: []string{"bob@"}}, "ownerEmails entries must be email addresses"},
		{"a 255-character email", domain.OrganizationSearchFilters{OwnerEmails: []string{"a" + long}}, "ownerEmails entries must be email addresses"},
		{"emails and unowned", domain.OrganizationSearchFilters{OwnerEmails: []string{"bob@wso2.com"}, Unowned: &yes}, "ownerEmails and filters.unowned cannot be combined"},
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
