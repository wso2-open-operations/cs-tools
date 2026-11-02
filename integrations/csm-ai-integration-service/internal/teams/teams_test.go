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

package teams

import (
	"strings"
	"testing"
)

var registry = []Team{
	{Key: "rigel", Name: "Rigel", Family: "cre-abt", Field: "creTeam"},
	{Key: "castor", Name: "Castor", Family: "cre-abt", Field: "creTeam"},
	{Key: "americas", Name: "Americas", Family: "cre", Field: "creTeam"},
	{Key: "apollo", Name: "Apollo", Family: "sre-abt", Field: "sreTeam"},
	{Key: "sre-core", Name: "SRE Core", Family: "sre", Field: "sreTeam"},
}

// The exact text the CSM portal backend wrote before the model call moved
// here (dashboard_teams.go's teamsSection), so the model's prompt is
// unchanged by the move.
const wantSection = "## Teams\n\n" +
	"These are the only teams a dashboard can filter by. A team is filtered by its **key**, in its\n" +
	"**field**: `{\"field\": \"<field>\", \"op\": \"in\", \"values\": [\"<key>\"]}`. Never write a team's id.\n\n" +
	"\"ABT\" (account based team) means only the teams whose family is CRE ABT: Rigel, Castor. No other team is an ABT team.\n\n" +
	"| Team | Key | Family | Field |\n|---|---|---|---|\n" +
	"| Rigel | `rigel` | CRE ABT | `creTeam` |\n" +
	"| Castor | `castor` | CRE ABT | `creTeam` |\n" +
	"| Americas | `americas` | CRE | `creTeam` |\n" +
	"| Apollo | `apollo` | SRE ABT | `sreTeam` |\n" +
	"| SRE Core | `sre-core` | SRE | `sreTeam` |\n"

func TestSectionIsTheBackendsText(t *testing.T) {
	if got := Section(registry); got != wantSection {
		t.Errorf("Section() =\n%s\nwant\n%s", got, wantSection)
	}
}

func TestSectionWithNoABTTeamSaysSo(t *testing.T) {
	got := Section([]Team{{Key: "americas", Name: "Americas", Family: "cre", Field: "creTeam"}})
	if !strings.Contains(got, "There are no ABT teams: a request about ABTs is not expressible.") {
		t.Errorf("Section() = %q, want the no-ABT line", got)
	}
	if strings.Contains(got, "means only the teams") {
		t.Error("Section() names ABT teams when there are none")
	}
}

func TestNoTeamsIsNoSection(t *testing.T) {
	if got := Section(nil); got != "" {
		t.Errorf("Section(nil) = %q, want empty", got)
	}
	if got := WithTeams("catalogue\n", nil); got != "catalogue\n" {
		t.Errorf("WithTeams(no teams) = %q, want the catalogue unchanged", got)
	}
}

func TestWithTeamsAppendsAfterASeparator(t *testing.T) {
	got := WithTeams("catalogue\n\n\n", registry)
	if want := "catalogue\n\n---\n\n" + wantSection; got != want {
		t.Errorf("WithTeams() =\n%q\nwant\n%q", got, want)
	}
}
