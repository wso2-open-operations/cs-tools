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

package directory

import "testing"

// Regression: a supplied creGroupId was stored unvalidated. sourceIDToUUID
// passes anything that is not exactly 32 hex characters through unchanged, so
// a typo yielded a malformed id that matched nothing on the creTeam filter
// without erroring -- the same silent degradation the other field checks exist
// to prevent.
func TestParseTeamRegistry_RejectsMalformedGroupID(t *testing.T) {
	for _, tc := range []struct{ name, raw string }{
		{"31 characters", "castor|Castor|cre-abt|760e87b247c13910a0a29cd3846d430"},
		{"33 characters", "castor|Castor|cre-abt|760e87b247c13910a0a29cd3846d43011"},
		{"non-hex character", "castor|Castor|cre-abt|760e87b247c13910a0a29cd3846d430z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseTeamRegistry(tc.raw); err == nil {
				t.Fatal("ParseTeamRegistry returned no error, want a rejection of the malformed creGroupId")
			}
		})
	}
}

// A row with no groupId at all stays legal: the id is optional and such a team
// is still listed, it just cannot scope the creTeam filter.
func TestParseTeamRegistry_AbsentGroupIDIsFine(t *testing.T) {
	teams, err := ParseTeamRegistry("castor|Castor|cre-abt,vega|Vega")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(teams) != 2 {
		t.Fatalf("got %d teams, want 2", len(teams))
	}
}

// A 5-field row supplies both the CRE and SRE group ids, and both parse
// independently.
func TestParseTeamRegistry_FiveFieldRowParsesBothGroupIDs(t *testing.T) {
	teams, err := ParseTeamRegistry("castor|Castor|cre-abt|760e87b247c13910a0a29cd3846d4301|aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("got %d teams, want 1", len(teams))
	}
	if got := teams[0].CreGroupID; got != "760e87b247c13910a0a29cd3846d4301" {
		t.Errorf("CreGroupID = %q, want the configured id", got)
	}
	if got := teams[0].SreGroupID; got != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Errorf("SreGroupID = %q, want the configured id", got)
	}
}

// A 4-field row still means "has a CRE group id, no SRE one" exactly as
// before -- SreGroupID must stay empty.
func TestParseTeamRegistry_FourFieldRowLeavesSreGroupIDEmpty(t *testing.T) {
	teams, err := ParseTeamRegistry("castor|Castor|cre-abt|760e87b247c13910a0a29cd3846d4301")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("got %d teams, want 1", len(teams))
	}
	if got := teams[0].CreGroupID; got != "760e87b247c13910a0a29cd3846d4301" {
		t.Errorf("CreGroupID = %q, want the configured id", got)
	}
	if got := teams[0].SreGroupID; got != "" {
		t.Errorf("SreGroupID = %q, want it empty for a 4-field row", got)
	}
}

// A malformed sreGroupId in the 5th field is rejected the same way a
// malformed creGroupId in the 4th field is.
func TestParseTeamRegistry_RejectsMalformedSreGroupID(t *testing.T) {
	if _, err := ParseTeamRegistry("castor|Castor|cre-abt|760e87b247c13910a0a29cd3846d4301|not-32-hex-chars"); err == nil {
		t.Fatal("ParseTeamRegistry returned no error, want a rejection of the malformed sreGroupId")
	}
}

// SME, the product special rotations, is a family the registry accepts --
// without it a deploy listing an SME team fails at startup -- while a family
// nobody branches on is still refused.
func TestParseTeamRegistry_AcceptsTheSMEFamily(t *testing.T) {
	teams, err := ParseTeamRegistry("moesif|Moesif|SME-MOESIF,asgardeo|Asgardeo|sme")
	if err == nil {
		t.Fatalf("SME-MOESIF is not one of the closed families and must be refused, got %+v", teams)
	}
	teams, err = ParseTeamRegistry("moesif|Moesif|SME,iaas|IaaS|SRE")
	if err != nil {
		t.Fatalf("ParseTeamRegistry: %v", err)
	}
	if teams[0].Family != FamilySME || teams[1].Family != FamilySRE {
		t.Fatalf("families = %q, %q; want %q, %q", teams[0].Family, teams[1].Family, FamilySME, FamilySRE)
	}
}
