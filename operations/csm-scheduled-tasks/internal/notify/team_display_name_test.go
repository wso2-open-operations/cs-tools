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

package notify

import "testing"

// The keys are the real ones production stores in team.name; the
// already-named cases are the ones that must survive untouched.
func TestTeamDisplayName(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"customer_onboarding_team", "Customer Onboarding Team"},
		{"americas_cre_team", "Americas CRE Team"},
		{"australia_cre_team", "Australia CRE Team"},
		{"migrations_team", "Migrations Team"},
		{"atlas", "Atlas"},
		{"castor", "Castor"},
		{"sirius", "Sirius"},

		// Already a display name: left exactly as found, including the
		// misspelling the recipients are used to.
		{"Americas CS Team - Integraion", "Americas CS Team - Integraion"},
		{"CS Migrations Team", "CS Migrations Team"},
		{"Castor", "Castor"},

		{"", ""},
	} {
		if got := teamDisplayName(tt.in); got != tt.want {
			t.Errorf("teamDisplayName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
