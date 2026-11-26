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

package main

import "testing"

// The incident tier hold is released only once both the SLA spaces and the
// group emails have loaded, whichever comes first, and only once.
func TestIncidentTeamLookups_ReleaseOnceBothLoaded(t *testing.T) {
	for _, order := range [][]string{{"spaces", "emails"}, {"emails", "spaces"}} {
		releases := 0
		l := newIncidentTeamLookups(func() { releases++ })
		load := func(what string) {
			if what == "spaces" {
				l.slaSpacesLoaded()
			} else {
				l.groupEmailsLoaded()
			}
		}
		load(order[0])
		load(order[0]) // a repeat of one lookup alone never releases
		if releases != 0 {
			t.Fatalf("%v: released after %s only", order, order[0])
		}
		load(order[1])
		if releases != 1 {
			t.Fatalf("%v: releases = %d after both loaded, want 1", order, releases)
		}
		load(order[0])
		load(order[1])
		if releases != 1 {
			t.Errorf("%v: releases = %d after later refreshes, want still 1", order, releases)
		}
	}
}
