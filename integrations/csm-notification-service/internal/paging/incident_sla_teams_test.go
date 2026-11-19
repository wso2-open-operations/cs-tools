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

package paging

import (
	"slices"
	"testing"
)

func TestSRETeamsForGroup(t *testing.T) {
	l := LadderConfig{
		Teams: TeamKeys{
			ABTs:          []string{"apollo", "artemis", "iaas"},
			Aliases:       map[string]string{"Apollo SRE Group": "apollo", "Artemis SRE Group": "artemis", "WSO2 IaaS Team": "iaas"},
			DefaultGroups: []string{"default", "MS/PC SRE Group", "Default SRE Group"},
		},
		TeamlessChats: []string{"apollo", "Artemis"},
	}
	for _, tc := range []struct {
		group string
		want  []string
	}{
		{"Apollo SRE Group", []string{"apollo"}},
		{" artemis sre group ", []string{"artemis"}},
		{"WSO2 IaaS Team", []string{"iaas"}},
		{"apollo", []string{"apollo"}},
		{"Default SRE Group", []string{"apollo", "artemis"}},
		{"ms/pc sre group", []string{"apollo", "artemis"}},
		{"Choreo Runtime Group", nil},
		{"", nil},
	} {
		if got := SRETeamsForGroup(l, tc.group); !slices.Equal(got, tc.want) {
			t.Errorf("SRETeamsForGroup(%q) = %v, want %v", tc.group, got, tc.want)
		}
	}

	l.TeamlessChats = nil
	if got := SRETeamsForGroup(l, "Default SRE Group"); got != nil {
		t.Errorf("default group with no teamlessChats = %v, want none", got)
	}
}
