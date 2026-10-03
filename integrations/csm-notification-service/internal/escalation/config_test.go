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

package escalation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "escalation.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfig_ReadsBothLadders(t *testing.T) {
	path := writeConfig(t, `
enabled: true
cre:
  enabled: true
  channel: log
  trigger:
    priorities: [S0, S1]
    excludeTeams: [cre-abt-noisy]
    requireKnownTeam: true
  safety:
    maxCallsPerLadder: 12
    maxLevel: 2
    allowedNumbers: ["+94770000000"]
sre:
  enabled: false
  channel: chat
`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	cre, running := cfg.For(LadderKeyCRE)
	if !running {
		t.Error("the CRE ladder should be running; both switches are on")
	}
	if cre.Channel != ChannelLog {
		t.Errorf("CRE channel = %q, want log", cre.Channel)
	}
	if _, running := cfg.For(LadderKeySRE); running {
		t.Error("the SRE ladder is disabled in this file and must not run")
	}
	if got, want := cre.Safety.MaxCallsPerLadder, 12; got != want {
		t.Errorf("maxCallsPerLadder = %d, want %d", got, want)
	}
}

// The master switch has to be able to stop a ladder whose own section still
// says enabled, or "turn escalation off" would mean editing every section.
func TestLoadConfig_MasterSwitchStopsAnEnabledLadder(t *testing.T) {
	path := writeConfig(t, `
enabled: false
cre:
  enabled: true
sre:
  enabled: true
`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	for _, key := range []string{LadderKeyCRE, LadderKeySRE} {
		if _, running := cfg.For(key); running {
			t.Errorf("%s runs despite the master switch being off", key)
		}
	}
}

// A file that cannot be read or parsed must never be treated as "no
// configuration, carry on with defaults": this file governs spending, so the
// safe reading of a broken one is that nothing runs.
func TestLoadConfig_FailuresReturnDisabled(t *testing.T) {
	cases := []struct {
		name string
		body string // empty means: do not create the file at all
	}{
		{name: "missing file"},
		{name: "not yaml", body: "\tthis: is: not: yaml:\n  - [unclosed\n"},
		{name: "unknown key", body: "enabled: true\ncre:\n  enabbled: true\n"},
		{name: "unknown channel", body: "enabled: true\ncre:\n  channel: telegram\n"},
		{name: "maxLevel too high", body: "enabled: true\ncre:\n  safety:\n    maxLevel: 9\n"},
		{name: "negative call cap", body: "enabled: true\ncre:\n  safety:\n    maxCallsPerLadder: -1\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "absent.yaml")
			if tc.body != "" {
				path = writeConfig(t, tc.body)
			}
			cfg, err := LoadConfig(path)
			if err == nil {
				t.Fatal("want an error, got none")
			}
			if cfg.Enabled {
				t.Error("a rejected config must come back disabled, not enabled")
			}
			if _, running := cfg.For(LadderKeyCRE); running {
				t.Error("a rejected config must not leave a ladder running")
			}
		})
	}
}

// A misspelled key is the case this protects hardest: somebody has just capped
// the spend, believes they have, and would otherwise get the old behaviour.
func TestLoadConfig_UnknownKeyNamesItself(t *testing.T) {
	path := writeConfig(t, "enabled: true\ncre:\n  safety:\n    maxCalsPerLadder: 4\n")
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("a misspelled key must be rejected, not ignored")
	}
	if !strings.Contains(err.Error(), "maxCalsPerLadder") {
		t.Errorf("the error should name the offending key; got %v", err)
	}
}

// An omitted maxLevel must not read as LEVEL_0, which would silently truncate
// every ladder to its first rung -- the failure mode nobody would notice until
// an incident went unescalated.
func TestLoadConfig_OmittedMaxLevelIsNoCap(t *testing.T) {
	path := writeConfig(t, "enabled: true\ncre:\n  enabled: true\n")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	cre, _ := cfg.For(LadderKeyCRE)
	if _, capped := cre.CapLevel(); capped {
		t.Error("a file that says nothing about maxLevel must not cap the ladder")
	}
}

func TestStartWhen_Allows(t *testing.T) {
	cases := []struct {
		name     string
		start    StartWhen
		priority string
		team     string
		shift    string
		want     bool
	}{
		{
			name: "empty lists have no opinion",
			want: true, priority: "S4", team: "anything", shift: "LK",
		},
		{
			name:     "priority not in the allowlist",
			start:    StartWhen{Priorities: []string{"S0", "S1"}},
			priority: "S3", want: false,
		},
		{
			name:     "priority in the allowlist",
			start:    StartWhen{Priorities: []string{"S0", "S1"}},
			priority: "S1", want: true,
		},
		{
			name:  "excluded team loses even when allowed",
			start: StartWhen{Teams: []string{"cre-abt-vega"}, ExcludeTeams: []string{"cre-abt-vega"}},
			team:  "cre-abt-vega", want: false,
		},
		{
			name:  "matching is case and space insensitive",
			start: StartWhen{Teams: []string{"CRE-ABT-Vega"}},
			team:  "  cre-abt-vega ", want: true,
		},
		{
			name:  "no team, and one is required",
			start: StartWhen{RequireKnownTeam: true},
			team:  "", want: false,
		},
		{
			name:  "no team, and none is required",
			start: StartWhen{},
			team:  "", want: true,
		},
		{
			name:  "shift outside the allowlist",
			start: StartWhen{Shifts: []string{"LK_MORNING"}},
			shift: "AMERICAS", want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := LadderConfig{Start: tc.start}
			got, why := l.Allows(tc.priority, tc.team, tc.shift)
			if got != tc.want {
				t.Errorf("Allows(%q,%q,%q) = %v (%s), want %v",
					tc.priority, tc.team, tc.shift, got, why, tc.want)
			}
			if !got && why == "" {
				t.Error("a refusal must say why; it ends up in a log line")
			}
		})
	}
}

func TestSafety_Dialable(t *testing.T) {
	open := LadderConfig{}
	if !open.Dialable("+94770000000") {
		t.Error("an empty allowedNumbers must not restrict anything")
	}

	restricted := LadderConfig{Safety: Safety{AllowedNumbers: []string{"+94770000000"}}}
	if !restricted.Dialable("+94770000000") {
		t.Error("a listed number must be dialable")
	}
	if restricted.Dialable("+94771111111") {
		t.Error("an unlisted number must not be dialable when the list is set")
	}
}

func TestSafety_CapLevel(t *testing.T) {
	two := int(Level2)
	capped := LadderConfig{Safety: Safety{MaxLevel: &two}}
	got, ok := capped.CapLevel()
	if !ok || got != Level2 {
		t.Errorf("CapLevel() = %v,%v; want %v,true", got, ok, Level2)
	}

	// LEVEL_0 is a legitimate cap -- "first responders only" -- and must work
	// as one.
	lvl0 := int(Level0)
	zero := LadderConfig{Safety: Safety{MaxLevel: &lvl0}}
	if got, ok := zero.CapLevel(); !ok || got != Level0 {
		t.Errorf("CapLevel() = %v,%v; want %v,true", got, ok, Level0)
	}
}

func TestParseChannel_Log(t *testing.T) {
	got, err := ParseChannel("log")
	if err != nil {
		t.Fatalf("ParseChannel(log): %v", err)
	}
	if got != ChannelLog {
		t.Errorf("ParseChannel(log) = %q, want %q", got, ChannelLog)
	}
	if ChannelLog.Reaches() {
		t.Error("the log channel must not claim to reach anybody")
	}
	if !ChannelCall.Reaches() || !ChannelChat.Reaches() {
		t.Error("call and chat both reach people")
	}
	// "both" means both channels that reach somebody. Folding log into it
	// would make a real ladder also write "would notify" lines, which reads
	// like a dry run in the middle of a live page.
	if ChannelBoth.Uses(ChannelLog) {
		t.Error("ChannelBoth must not include the log channel")
	}
}

// The zero value of Safety must impose no cap at all. It is what a deployment
// with no configuration file gets, and reading it as "cap at LEVEL_0" would
// silently truncate every ladder to its first rung -- an outage nobody would
// see until an incident went unescalated.
func TestSafety_ZeroValueCapsNothing(t *testing.T) {
	if _, capped := (LadderConfig{}).CapLevel(); capped {
		t.Fatal("the zero LadderConfig must not cap the ladder")
	}
	if !(LadderConfig{}).Dialable("+94770000000") {
		t.Error("the zero LadderConfig must not restrict numbers")
	}
	if ok, _ := (LadderConfig{}).Allows("S4", "", ""); !ok {
		t.Error("the zero LadderConfig must let every incident through")
	}
}

// maxLevel: 0 in the file is an explicit cap and must be honoured, which is the
// whole reason the field is a pointer.
func TestLoadConfig_ExplicitZeroMaxLevelCaps(t *testing.T) {
	path := writeConfig(t, "enabled: true\ncre:\n  enabled: true\n  safety:\n    maxLevel: 0\n")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	cre, _ := cfg.For(LadderKeyCRE)
	got, capped := cre.CapLevel()
	if !capped || got != Level0 {
		t.Errorf("CapLevel() = %v,%v; want LEVEL_0,true", got, capped)
	}
}

// The example file the compose stack mounts must actually load. It is the
// first thing anybody copies, and KnownFields(true) means a stray key in it
// would disable both ladders on a developer's first run with no clue why.
func TestLoadConfig_ShippedExampleIsValid(t *testing.T) {
	const path = "../../../../scripts/csm-compose/escalation.yaml"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("example config not reachable from here: %v", err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("the shipped example config does not load: %v", err)
	}
	cre, running := cfg.For(LadderKeyCRE)
	if !running {
		t.Error("the example should come up running, or it teaches the wrong thing")
	}
	// It must ship reaching nobody: a first `docker compose up` that dialled
	// real numbers would be the worst possible default.
	if cre.Channel != ChannelLog {
		t.Errorf("example CRE channel = %q; it must ship as log", cre.Channel)
	}
	if sre, _ := cfg.For(LadderKeySRE); sre.Channel != ChannelLog {
		t.Errorf("example SRE channel = %q; it must ship as log", sre.Channel)
	}
}

// Acknowledgement defaults to BOTH gestures. A file that says nothing must get
// the stricter rule, not the permissive one -- the card and the voice message
// both ask for a status move and a public comment.
func TestAcknowledgement_DefaultsToBoth(t *testing.T) {
	if !(LadderConfig{}).RequireBothGestures() {
		t.Error("the zero LadderConfig must require both gestures")
	}
	path := writeConfig(t, "enabled: true\ncre:\n  enabled: true\n")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	cre, _ := cfg.For(LadderKeyCRE)
	if !cre.RequireBothGestures() {
		t.Error("a file that says nothing must still require both")
	}

	off := writeConfig(t, "enabled: true\ncre:\n  acknowledgement:\n    requireBoth: false\n")
	cfg, err = LoadConfig(off)
	if err != nil {
		t.Fatal(err)
	}
	cre, _ = cfg.For(LadderKeyCRE)
	if cre.RequireBothGestures() {
		t.Error("requireBoth: false must be honoured")
	}
}

func TestHeadsAndAlertDuty_ReadFromTheFile(t *testing.T) {
	path := writeConfig(t, `
enabled: true
cre:
  enabled: true
  heads:
    cre: {name: "CRE Head", email: "cre.head@example.com", phone: "+94770000001"}
    cs:  {name: "CS Head",  email: "cs.head@example.com"}
  alertDuty:
    tiers: [T1, T2, T3]
    perTeam: 1
`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	cre, _ := cfg.For(LadderKeyCRE)
	if !cre.Heads.CRE.Set() || cre.Heads.CRE.Phone != "+94770000001" {
		t.Errorf("CRE head = %+v", cre.Heads.CRE)
	}
	// A head with an address but no number is still somebody the chat and log
	// channels reach.
	if !cre.Heads.CS.Set() {
		t.Error("a head with an email but no phone is still reachable")
	}
	if got := cre.AlertDuty.PerTeam; got != 1 {
		t.Errorf("perTeam = %d, want 1", got)
	}
	if got := len(cre.AlertTiers()); got != 3 {
		t.Errorf("tiers = %d, want 3", got)
	}
	// An empty tiers list falls back to the three that exist.
	if got := len((LadderConfig{}).AlertTiers()); got != 3 {
		t.Errorf("default tiers = %d, want 3", got)
	}
}

// The shipped example must name the real teams. Getting this wrong is silent:
// a list naming an SRE team and omitting a CRE one still resolves to real
// people, so the ladder calls the wrong leads and nothing reports it. That
// happened once already.
func TestLoadConfig_ExampleNamesTheRealABTTeams(t *testing.T) {
	const path = "../../../../scripts/csm-compose/escalation.yaml"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("example config not reachable: %v", err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	cre, _ := cfg.For(LadderKeyCRE)
	sre, _ := cfg.For(LadderKeySRE)

	wantCRE := []string{"atlas", "castor", "draco", "phoenix", "rigel", "sirius", "vega"}
	wantSRE := []string{"apollo", "artemis"}
	if !equalStringSets(cre.Teams.ABTs, wantCRE) {
		t.Errorf("cre-abt = %v, want %v", cre.Teams.ABTs, wantCRE)
	}
	if !equalStringSets(sre.Teams.ABTs, wantSRE) {
		t.Errorf("sre-abt = %v, want %v", sre.Teams.ABTs, wantSRE)
	}
	// The two ABTs must not overlap: a team in both would be escalated by
	// both ladders for the same incident.
	for _, c := range cre.Teams.ABTs {
		for _, s := range sre.Teams.ABTs {
			if c == s {
				t.Errorf("%s is in both ABTs", c)
			}
		}
	}
	if cre.Teams.ABTType != "cre-abt" || sre.Teams.ABTType != "sre-abt" {
		t.Errorf("abtType = %q / %q, want cre-abt / sre-abt",
			cre.Teams.ABTType, sre.Teams.ABTType)
	}
	// Americas belongs to neither.
	for _, k := range append(append([]string{}, cre.Teams.ABTs...), sre.Teams.ABTs...) {
		if k == cre.Teams.Americas {
			t.Errorf("%s is listed as an ABT team; Americas is type cre", k)
		}
	}
}

func equalStringSets(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]bool{}
	for _, g := range got {
		seen[g] = true
	}
	for _, w := range want {
		if !seen[w] {
			return false
		}
	}
	return true
}

// trigger.priorities is documented as S0..S4, but no publisher emits that
// notation -- a payload carries CRITICAL, MODERATE or P1. Comparing them raw
// meant an allowlist written exactly as the rules document spells it matched
// nothing, and escalation turned itself off with only an Info line per
// incident to say so.
func TestStartWhen_PrioritiesCompareAcrossNotations(t *testing.T) {
	cases := []struct {
		allow    []string
		priority string
		want     bool
	}{
		{[]string{"S0", "S1"}, "CRITICAL", true},  // S-notation vs a severity label
		{[]string{"S0", "S1"}, "P1", true},        // S-notation vs P-notation
		{[]string{"P0", "P1"}, "CRITICAL", true},  // P-notation vs a label
		{[]string{"CRITICAL"}, "S1", true},        // and the reverse
		{[]string{"S0", "S1"}, "MODERATE", false}, // genuinely outside the list
		{[]string{"S0"}, "P1", false},
		{[]string{" s1 "}, "CRITICAL", true}, // case and space insensitive
		{nil, "ANYTHING", true},              // empty means no opinion
	}
	for _, tc := range cases {
		l := LadderConfig{Start: StartWhen{Priorities: tc.allow}}
		got, why := l.Allows(tc.priority, "vega", "LK")
		if got != tc.want {
			t.Errorf("Allows(%q) with %v = %v (%s), want %v", tc.priority, tc.allow, got, why, tc.want)
		}
	}
}

// A cap keyed by a shift that does not exist must be refused, not ignored.
//
// KnownFields(true) cannot see inside a map's keys, so "LK_EVENINGS" would be
// accepted and quietly cap nothing -- leaving an operator certain they had
// limited the spend when they had not. That is the same failure the strict
// decoder exists to prevent, so it gets the same treatment.
func TestConfig_RejectsAnUnknownShiftInRotaCaps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "escalation.yaml")
	write := func(body string) {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write("enabled: true\ncre:\n  teams:\n    rotaMembersToCall:\n      LK_EVENINGS: 7\n")
	if _, err := LoadConfig(path); err == nil {
		t.Error("a misspelled shift name was accepted; it would have capped nothing")
	}

	write("enabled: true\ncre:\n  teams:\n    rotaMembersToCall:\n      LK_EVENING: -1\n")
	if _, err := LoadConfig(path); err == nil {
		t.Error("a negative cap was accepted")
	}

	// Lower case is fine: the key is normalised before it is looked up.
	write("enabled: true\ncre:\n  teams:\n    rotaMembersToCall:\n      lk_evening: 7\n")
	if _, err := LoadConfig(path); err != nil {
		t.Errorf("a lower-case shift name should be accepted: %v", err)
	}
}
