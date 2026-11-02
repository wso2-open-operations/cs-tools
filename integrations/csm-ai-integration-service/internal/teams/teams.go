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

// Package teams writes the catalogue's Teams section from the team list a
// request carries. The service holds no team registry: the CSM portal backend
// owns it (CSM_TEAM_REGISTRY) and sends, with each generate request, only the
// teams a dashboard can filter by (those with a group configured). The
// backend also swaps each team key the model writes for its group id, so the
// model never sees a group id.
package teams

import (
	"fmt"
	"strings"
)

// Team is one team as the model is told about it.
type Team struct {
	Key    string `json:"key"`    // what a filter names, e.g. "rigel"
	Name   string `json:"name"`   // display name, e.g. "Rigel"
	Family string `json:"family"` // "cre-abt", "cre", "sre-abt" or "sre"
	Field  string `json:"field"`  // the case field it filters: "creTeam" or "sreTeam"
}

// familyLabels are the team families as the Teams section names them.
var familyLabels = map[string]string{
	"cre-abt": "CRE ABT",
	"cre":     "CRE",
	"sre-abt": "SRE ABT",
	"sre":     "SRE",
}

// Section is the catalogue's Teams section: a table of the teams, in the
// order given. Empty when there are none.
func Section(teams []Team) string {
	var rows strings.Builder
	var abt []string
	for _, team := range teams {
		fmt.Fprintf(&rows, "| %s | `%s` | %s | `%s` |\n", team.Name, team.Key, familyLabels[team.Family], team.Field)
		if team.Family == "cre-abt" {
			abt = append(abt, team.Name)
		}
	}
	if rows.Len() == 0 {
		return ""
	}
	abtLine := "There are no ABT teams: a request about ABTs is not expressible.\n\n"
	if len(abt) > 0 {
		abtLine = "\"ABT\" (account based team) means only the teams whose family is CRE ABT: " +
			strings.Join(abt, ", ") + ". No other team is an ABT team.\n\n"
	}
	return "## Teams\n\n" +
		"These are the only teams a dashboard can filter by. A team is filtered by its **key**, in its\n" +
		"**field**: `{\"field\": \"<field>\", \"op\": \"in\", \"values\": [\"<key>\"]}`. Never write a team's id.\n\n" +
		abtLine +
		"| Team | Key | Family | Field |\n|---|---|---|---|\n" + rows.String()
}

// WithTeams appends the Teams section to a catalogue's text, after a
// separator. The text is returned as is when there are no teams.
func WithTeams(catalogue string, teams []Team) string {
	section := Section(teams)
	if section == "" {
		return catalogue
	}
	return strings.TrimRight(catalogue, "\n") + "\n\n---\n\n" + section
}
