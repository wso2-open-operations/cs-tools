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

package worklogreport_test

import (
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogreport"
)

// The delivered mails' headers, verbatim. These are the specification for
// who receives what, and the point of copying them here is that a change
// altering either list fails loudly.
var (
	weeklyTo = []string{
		"mara@example.com", "nils@example.com", "omar@example.com",
		"pia@example.com", "quinn@example.com",
	}
	weeklyCc = []string{"leads-list@example.com", "all-list@example.com"}

	dailyTo = []string{
		"rosa@example.com", "said@example.com", "mara@example.com",
		"nils@example.com", "omar@example.com", "alpha-list@example.com",
		"beta-list@example.com", "all-list@example.com", "tomas@example.com",
		"gamma-list@example.com",
	}
	dailyCc = []string{
		"uma@example.com", "quinn@example.com", "mara@example.com",
		"vikt@example.com", "said@example.com", "wren@example.com",
		"xenia@example.com", "yusuf@example.com", "zara@example.com",
		"arlo@example.com", "bea@example.com", "ciro@example.com",
		"dilan@example.com", "enzo@example.com", "fay@example.com",
		"pia@example.com", "gio@example.com", "hana@example.com",
		"ivo@example.com", "juno@example.com", "kira@example.com",
		"liam@example.com", "mina@example.com", "noor@example.com",
	}
)

// The configured lists must survive resolution untouched, including the
// parts that look like mistakes. mara@ and said@ really are in
// both the To and the Cc of the delivered daily mail; "tidying" that away
// would stop us reproducing it.
func TestResolveReproducesDeliveredHeaders(t *testing.T) {
	derived := worklogreport.Recipients{
		To: []string{"some-lead@example.com"},
		Cc: []string{"some-team-group@example.com"},
	}

	weekly := worklogreport.Resolve(derived,
		worklogreport.Audience{To: weeklyTo, Cc: weeklyCc},
		worklogreport.SourceConfigured, nil)
	if strings.Join(weekly.To, ",") != strings.Join(weeklyTo, ",") {
		t.Errorf("weekly To = %v, want the delivered header %v", weekly.To, weeklyTo)
	}
	if strings.Join(weekly.Cc, ",") != strings.Join(weeklyCc, ",") {
		t.Errorf("weekly Cc = %v, want the delivered header %v", weekly.Cc, weeklyCc)
	}

	daily := worklogreport.Resolve(derived,
		worklogreport.Audience{To: dailyTo, Cc: dailyCc},
		worklogreport.SourceConfigured, nil)
	if strings.Join(daily.To, ",") != strings.Join(dailyTo, ",") {
		t.Errorf("daily To = %v, want the delivered header %v", daily.To, dailyTo)
	}
	if strings.Join(daily.Cc, ",") != strings.Join(dailyCc, ",") {
		t.Errorf("daily Cc = %v, want the delivered header %v", daily.Cc, dailyCc)
	}

	// The two audiences differ in size, membership AND shape: the daily
	// carries group addresses in To, the weekly only in Cc.
	if strings.Join(daily.To, ",") == strings.Join(weekly.To, ",") {
		t.Error("the daily and weekly To lines are identical; they must differ")
	}
	var weeklyToGroups int
	for _, a := range weekly.To {
		if strings.Contains(a, "-list") {
			weeklyToGroups++
		}
	}
	if weeklyToGroups != 0 {
		t.Errorf("weekly To carries %d group addresses; the delivered mail carries none", weeklyToGroups)
	}
	var dailyToGroups int
	for _, a := range daily.To {
		if strings.Contains(a, "-list") {
			dailyToGroups++
		}
	}
	if dailyToGroups != 4 {
		t.Errorf("daily To carries %d shared mailboxes, want 4", dailyToGroups)
	}
}

// A person in both To and Cc of the same mail must stay in both.
func TestResolveKeepsCrossListDuplicates(t *testing.T) {
	got := worklogreport.Resolve(worklogreport.Recipients{},
		worklogreport.Audience{To: dailyTo, Cc: dailyCc},
		worklogreport.SourceConfigured, nil)
	for _, addr := range []string{"mara@example.com", "said@example.com"} {
		if !contains(got.To, addr) || !contains(got.Cc, addr) {
			t.Errorf("%s is in both To and Cc of the delivered daily mail and must stay in both", addr)
		}
	}
}

func TestResolvePrecedence(t *testing.T) {
	derived := worklogreport.Recipients{To: []string{"lead@example.com"}, Cc: []string{"g@example.com"}}
	cfg := worklogreport.Audience{To: weeklyTo, Cc: weeklyCc}

	// Override beats everything, and clears Cc: a test run must not
	// merely prepend itself to a live list.
	got := worklogreport.Resolve(derived, cfg, worklogreport.SourceConfigured, []string{"tester@example.com"})
	if strings.Join(got.To, ",") != "tester@example.com" || len(got.Cc) != 0 {
		t.Errorf("override did not replace the audience: %+v", got)
	}

	// With nothing configured, nothing is sent. There is deliberately
	// no fallback to the derived audience: a blank configured entry
	// would otherwise mail the registry-derived recipients, which is
	// what configuring the lists exists to prevent. Sending nothing is
	// the safe failure.
	got = worklogreport.Resolve(derived, worklogreport.Audience{}, worklogreport.SourceConfigured, nil)
	if len(got.To) != 0 || len(got.Cc) != 0 {
		t.Errorf("want no audience when nothing is configured, got %+v", got)
	}

	// And a blank configured entry is not an audience either.
	got = worklogreport.Resolve(derived, worklogreport.Audience{To: []string{" "}}, worklogreport.SourceConfigured, nil)
	if len(got.To) != 0 {
		t.Errorf("a blank configured to produced an audience: %v", got.To)
	}
}

// Drift is the early warning that a hand-maintained list has gone stale.
func TestDrift(t *testing.T) {
	derived := worklogreport.Recipients{
		To: []string{"newlead@example.com", "mara@example.com"},
	}
	cfg := worklogreport.Audience{
		To: []string{"mara@example.com", "departed@example.com", "all-list@example.com"},
	}

	missing, extra := worklogreport.Drift(derived, cfg)
	if strings.Join(missing, ",") != "newlead@example.com" {
		t.Errorf("missing = %v, want the lead nobody added", missing)
	}
	// departed@ is flagged; cs-group@ is a shared mailbox with no derived
	// counterpart and must not be.
	if strings.Join(extra, ",") != "departed@example.com" {
		t.Errorf("extra = %v, want only the departed individual", extra)
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// A standing list can be copied on the derived audience without taking
// ownership of the whole To line.
//
// The alternative was a team row carrying the address, which only works
// if the row claims a type it does not have -- the routing is driven by
// team.type -- and which would promote anyone later attached to it onto
// the To line.
func TestResolveDerivedAddsConfiguredCc(t *testing.T) {
	derived := worklogreport.Recipients{
		To: []string{"lead-a@example.com", "lead-b@example.com"},
		Cc: []string{"team-a@example.com"},
	}
	configured := worklogreport.Audience{
		To: []string{"should-be-ignored@example.com"},
		Cc: []string{"cs-group@example.com"},
	}

	got := worklogreport.Resolve(derived, configured, worklogreport.SourceDerived, nil)

	// The To line is untouched: adding a cc must never widen it.
	if strings.Join(got.To, ",") != "lead-a@example.com,lead-b@example.com" {
		t.Errorf("To = %v, want the derived list only", got.To)
	}
	if strings.Join(got.Cc, ",") != "team-a@example.com,cs-group@example.com" {
		t.Errorf("Cc = %v, want the derived group then the configured one", got.Cc)
	}

	// Configured source is unchanged -- it still owns both lines.
	cfg := worklogreport.Resolve(derived, configured, worklogreport.SourceConfigured, nil)
	if strings.Join(cfg.To, ",") != "should-be-ignored@example.com" ||
		strings.Join(cfg.Cc, ",") != "cs-group@example.com" {
		t.Errorf("configured source changed: To=%v Cc=%v", cfg.To, cfg.Cc)
	}

	// An override still replaces everything, cc included.
	ov := worklogreport.Resolve(derived, configured, worklogreport.SourceDerived, []string{"me@example.com"})
	if strings.Join(ov.To, ",") != "me@example.com" || len(ov.Cc) != 0 {
		t.Errorf("override leaked: To=%v Cc=%v", ov.To, ov.Cc)
	}

	// A duplicate address appears once.
	dup := worklogreport.Resolve(derived,
		worklogreport.Audience{Cc: []string{"TEAM-A@example.com"}},
		worklogreport.SourceDerived, nil)
	if len(dup.Cc) != 1 {
		t.Errorf("Cc = %v, want the duplicate collapsed", dup.Cc)
	}
}
