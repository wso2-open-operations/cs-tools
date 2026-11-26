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
	"encoding/json"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

// prodShapedMinimal is prodShapedConfig with only what stays in the file.
const prodShapedMinimal = `
enabled: true
cre:
  enabled: true
  channel: both
sre:
  enabled: true
  channel: both
`

// valuesFrom is what "Copy what's in use" on the Alert Settings tab saves: every
// ladder value cfg runs with, as RunningCaseAlertValues reports it.
func valuesFrom(t *testing.T, cfg Config) []CaseAlertValue {
	t.Helper()
	b, err := json.Marshal(RunningCaseAlertValues(cfg, nil))
	if err != nil {
		t.Fatal(err)
	}
	var running struct {
		Ladders map[string]map[string]struct {
			Value  json.RawMessage `json:"value"`
			Source string          `json:"source"`
		} `json:"ladders"`
	}
	if err := json.Unmarshal(b, &running); err != nil {
		t.Fatal(err)
	}
	var out []CaseAlertValue
	for ladder, keys := range running.Ladders {
		for key, v := range keys {
			if v.Source != "file" {
				t.Fatalf("%s %s reported from %q with nothing saved", ladder, key, v.Source)
			}
			out = append(out, CaseAlertValue{Ladder: ladder, Key: key, Value: v.Value})
		}
	}
	return out
}

// comparable puts a configuration in a form two that page alike share:
// rooms by URL (resolvedChats), group names in lower case as the resolver
// matches them, and no list where the file has an empty one.
func comparable(cfg Config, lookup func(string) string) Config {
	cfg = resolvedChats(cfg, lookup)
	for _, l := range []*LadderConfig{&cfg.CRE, &cfg.SRE} {
		if l.Teams.Aliases != nil {
			lower := map[string]string{}
			for k, v := range l.Teams.Aliases {
				lower[strings.ToLower(k)] = v
			}
			l.Teams.Aliases = lower
		}
		groups := []string{}
		for _, g := range l.Teams.DefaultGroups {
			groups = append(groups, strings.ToLower(g))
		}
		sort.Strings(groups)
		l.Teams.DefaultGroups = groups
		if len(l.TeamlessChats) == 0 {
			l.TeamlessChats = nil
		}
	}
	return cfg
}

// The small file plus everything copied from what the full file runs with
// pages exactly as the full file does. This is what lets production move to
// the small file: save the settings, then swap the file.
func TestMergeCaseAlertSettings_SmallFilePlusSavedIsTheFullFile(t *testing.T) {
	type pair struct{ full, small Config }
	pairs := map[string]pair{
		"production-shaped": {loadConfigText(t, prodShapedConfig), loadConfigText(t, prodShapedMinimal)},
	}
	full, errFull := LoadConfig("../../../../scripts/csm-compose/paging-alert.yaml")
	small, errSmall := LoadConfig("../../../../scripts/csm-compose/paging-alert.minimal.yaml")
	if errFull != nil || errSmall != nil {
		t.Fatalf("shipped examples: %v / %v", errFull, errSmall)
	}
	pairs["shipped examples"] = pair{full, small}

	for name, p := range pairs {
		t.Run(name, func(t *testing.T) {
			saved := savedFrom(p.full)
			saved.Values = valuesFrom(t, p.full)
			if len(saved.Values) < 15 {
				t.Fatalf("only %d values copied", len(saved.Values))
			}
			merged, env, _, err := MergeCaseAlertSettings(p.small, saved)
			if err != nil {
				t.Fatalf("merge: %v", err)
			}
			want := comparable(p.full, fakeHook)
			got := comparable(merged, func(v string) string { return env[v] })
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("the small file and the saved settings page differently:\n full:  %+v\n saved: %+v\n full SRE:  %+v\n saved SRE: %+v",
					want.CRE, got.CRE, want.SRE, got.SRE)
			}
		})
	}
}

// Each key goes into its own field, and reads back the same.
func TestMergeCaseAlertSettings_EveryValueLandsInItsField(t *testing.T) {
	cfg := loadConfigText(t, prodShapedMinimal)
	values := []CaseAlertValue{
		{"CRE", "trigger.priorities", json.RawMessage(`["S0","S1"]`)},
		{"CRE", "trigger.teams", json.RawMessage(`["phoenix group"]`)},
		{"CRE", "trigger.excludeTeams", json.RawMessage(`["noisy group"]`)},
		{"CRE", "trigger.shifts", json.RawMessage(`["USA"]`)},
		{"CRE", "trigger.requireKnownTeam", json.RawMessage(`true`)},
		{"CRE", "onLeave", json.RawMessage(`"call"`)},
		{"CRE", "teams.abtType", json.RawMessage(`"cre-abt"`)},
		{"CRE", "teams.abts", json.RawMessage(`["atlas","vega"]`)},
		{"CRE", "teams.americas", json.RawMessage(`"americas"`)},
		{"CRE", "teams.leadership", json.RawMessage(`"cre-leadership"`)},
		{"CRE", "teams.teamLeadsToCall", json.RawMessage(`3`)},
		{"CRE", "teams.teamLeads", json.RawMessage(`["atlas"]`)},
		{"CRE", "teams.rotaMembersToCall", json.RawMessage(`{"LK_EVENING":7}`)},
		{"CRE", "teams.americasWeekendRotaShifts", json.RawMessage(`["CRE_WEEKEND_NIGHT"]`)},
		{"CRE", "teams.rotaTeams", json.RawMessage(`["atlas","americas"]`)},
		{"CRE", "teams.unassignedTeamLead", json.RawMessage(`"none"`)},
		{"CRE", "teams.unassignedTeamLeadCount", json.RawMessage(`2`)},
		{"CRE", "teams.unassignedLeads", json.RawMessage(`"shift_leads"`)},
		{"CRE", "heads.cre", json.RawMessage(`{"name":"CRE Head","email":"crehead@example.test"}`)},
		{"CRE", "heads.cs", json.RawMessage(`{"name":"CS Head","email":"cshead@example.test"}`)},
		{"CRE", "alertDuty.tiers", json.RawMessage(`["T1","T2"]`)},
		{"CRE", "alertDuty.perTeam", json.RawMessage(`1`)},
		{"CRE", "acknowledgement.requireBoth", json.RawMessage(`false`)},
		{"CRE", "email.enabled", json.RawMessage(`true`)},
		{"CRE", "email.teamLists", json.RawMessage(`{"atlas":"atlas-list@example.test"}`)},
		{"CRE", "email.creTeamList", json.RawMessage(`"cre-list@example.test"`)},
		{"CRE", "email.csTeamList", json.RawMessage(`"cs-list@example.test"`)},
		{"SRE", "trigger.priorities", json.RawMessage(`["S0"]`)},
		{"SRE", "onLeave", json.RawMessage(`"skip"`)},
		{"SRE", "teams.abtType", json.RawMessage(`"sre-abt"`)},
		{"SRE", "teams.abts", json.RawMessage(`["apollo","artemis","iaas"]`)},
		{"SRE", "teams.defaultRota", json.RawMessage(`"SRE_SAAS"`)},
		{"SRE", "timing.initialWait", json.RawMessage(`"30s"`)},
		{"SRE", "timing.interval", json.RawMessage(`"10m"`)},
		{"SRE", "timing.includeL4", json.RawMessage(`true`)},
		{"SRE", "unansweredChat", json.RawMessage(`true`)},
		{"SRE", "smeHandoffChat", json.RawMessage(`true`)},
		{"SRE", "enabled", json.RawMessage(`true`)},
		{"SRE", "channel", json.RawMessage(`"chat"`)},
		{"SRE", "safety.maxCallsPerLadder", json.RawMessage(`7`)},
		{"SRE", "safety.callWithoutVerifiedLeads", json.RawMessage(`true`)},
		{"SRE", "safety.callHeadsWithoutLowerTiers", json.RawMessage(`true`)},
	}
	merged, _, changes, err := MergeCaseAlertSettings(cfg, CaseAlertSettings{Values: values})
	if err != nil {
		t.Fatal(err)
	}
	c, s := merged.CRE, merged.SRE
	checks := map[string]bool{
		"cre priorities":   slices.Equal(c.Start.Priorities, []string{"S0", "S1"}),
		"cre teams":        slices.Equal(c.Start.Teams, []string{"phoenix group"}),
		"cre exclude":      slices.Equal(c.Start.ExcludeTeams, []string{"noisy group"}),
		"cre shifts":       slices.Equal(c.Start.Shifts, []string{"USA"}),
		"cre known team":   c.Start.RequireKnownTeam,
		"cre on leave":     c.OnLeave == OnLeaveCall,
		"cre abt type":     c.Teams.ABTType == "cre-abt",
		"cre abts":         slices.Equal(c.Teams.ABTs, []string{"atlas", "vega"}),
		"americas":         c.Teams.Americas == "americas",
		"leadership":       c.Teams.Leadership == "cre-leadership",
		"leads to call":    c.Teams.TeamLeadsToCall == 3,
		"team leads":       slices.Equal(c.Teams.TeamLeads, []string{"atlas"}),
		"rota members":     reflect.DeepEqual(c.Teams.RotaMembersToCall, map[string]int{"LK_EVENING": 7}),
		"weekend shifts":   slices.Equal(c.Teams.AmericasWeekendRotaShifts, []string{"CRE_WEEKEND_NIGHT"}),
		"rota teams":       slices.Equal(c.Teams.RotaTeams, []string{"atlas", "americas"}),
		"unassigned":       c.Teams.UnassignedTeamLead == "none" && c.Teams.UnassignedTeamLeadCount == 2,
		"unassigned leads": c.Teams.UnassignedLeads == "shift_leads",
		"cre head":         c.Heads.CRE == Person{Name: "CRE Head", Email: "crehead@example.test"},
		"cs head":          c.Heads.CS == Person{Name: "CS Head", Email: "cshead@example.test"},
		"tiers":            slices.Equal(c.AlertTiers(), []string{"T1", "T2"}),
		"per team":         c.AlertDuty.PerTeam == 1,
		"require both":     !c.RequireBothGestures(),
		"email on":         c.Email.Enabled,
		"email team lists": reflect.DeepEqual(c.Email.TeamLists, map[string]string{"atlas": "atlas-list@example.test"}),
		"email cre list":   c.Email.CRETeamList == "cre-list@example.test",
		"email cs list":    c.Email.CSTeamList == "cs-list@example.test",
		"sre priorities":   slices.Equal(s.Start.Priorities, []string{"S0"}),
		"sre on leave":     s.OnLeave == OnLeaveSkip,
		"sre abts":         slices.Equal(s.Teams.ABTs, []string{"apollo", "artemis", "iaas"}) && s.Teams.ABTType == "sre-abt",
		"default rota":     s.Teams.DefaultRota == "SRE_SAAS",
		"initial wait":     time.Duration(s.Timing.InitialWait) == 30*time.Second,
		"interval":         time.Duration(s.Timing.Interval) == 10*time.Minute,
		"include L4":       s.Timing.IncludeL4,
		"unanswered chat":  s.UnansweredChat,
		"handoff chat":     s.SMEHandoffChat,
		"no safety change": merged.CRE.Safety.MaxCallsPerLadder == cfg.CRE.Safety.MaxCallsPerLadder,
		"sre enabled":      s.Enabled,
		"sre channel":      s.Channel == ChannelChat,
		"sre max calls":    s.Safety.MaxCallsPerLadder == 7,
		"sre leads hold":   s.Safety.CallWithoutVerifiedLeads,
		"sre heads hold":   s.Safety.CallHeadsWithoutLowerTiers,
	}
	for name, ok := range checks {
		if !ok {
			t.Errorf("%s was not set", name)
		}
	}
	covered := map[string]bool{}
	for _, v := range values {
		covered[v.Key] = true
	}
	for k := range caseAlertKeys {
		if !covered[k] {
			t.Errorf("%s is not set by this test: add it", k)
		}
	}
	joined := strings.Join(changes, "\n")
	if !strings.Contains(joined, "cre.heads.cre: changed") || strings.Contains(joined, "example.test") {
		t.Errorf("a head's change names them, or is missing:\n%s", joined)
	}
	if !strings.Contains(joined, `sre.timing.interval: unset -> "10m"`) {
		t.Errorf("no change line for the interval:\n%s", joined)
	}
}

// A list saved on the tab replaces the file's; the file's own list, which the
// caller keeps using in shadow mode, is never written into.
func TestMergeCaseAlertSettings_ValuesNeverWriteIntoTheFile(t *testing.T) {
	cfg := loadConfigText(t, prodShapedConfig)
	pristine := loadConfigText(t, prodShapedConfig)
	_, _, _, err := MergeCaseAlertSettings(cfg, CaseAlertSettings{Values: []CaseAlertValue{
		{"CRE", "teams.abts", json.RawMessage(`["vega"]`)},
		{"SRE", "teams.abts", json.RawMessage(`["apollo"]`)},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, pristine) {
		t.Fatal("the merge wrote into the caller's configuration")
	}
}

// A key this build does not know, or one saved for the other ladder, is
// skipped and said so; a value in the wrong shape refuses the whole merge.
func TestMergeCaseAlertSettings_UnknownAndWrongValues(t *testing.T) {
	cfg := loadConfigText(t, prodShapedConfig)
	merged, _, changes, err := MergeCaseAlertSettings(cfg, CaseAlertSettings{Values: []CaseAlertValue{
		{"CRE", "teams.somethingNew", json.RawMessage(`1`)},
		{"CRE", "timing.interval", json.RawMessage(`"10m"`)},
		{"PAAS", "onLeave", json.RawMessage(`"call"`)},
		{"SME", "heads.cre", json.RawMessage(`{"name":"x","email":"y"}`)},
	}})
	if err != nil || !reflect.DeepEqual(comparable(merged, fakeHook).CRE, comparable(cfg, fakeHook).CRE) {
		t.Fatalf("err %v, CRE changed by skipped values", err)
	}
	joined := strings.Join(changes, "\n")
	for _, want := range []string{"skipped cre.teams.somethingNew", "skipped cre.timing.interval: not a CRE setting", `skipped a value for ladder "PAAS"`, "skipped sme.heads.cre: not a setting this build knows"} {
		if !strings.Contains(joined, want) {
			t.Errorf("changes do not say %q:\n%s", want, joined)
		}
	}

	for _, bad := range []CaseAlertValue{
		{"CRE", "teams.abts", json.RawMessage(`"atlas"`)},
		{"CRE", "heads.cre", json.RawMessage(`{"name":"x","email":"y","phone":"+1"}`)},
		{"SRE", "timing.interval", json.RawMessage(`"soon"`)},
		{"CRE", "onLeave", json.RawMessage(`"maybe"`)}, // refused by the file's own checks
	} {
		if _, env, _, err := MergeCaseAlertSettings(cfg, CaseAlertSettings{Values: []CaseAlertValue{bad}}); err == nil || env != nil {
			t.Errorf("%s %s accepted", bad.Key, bad.Value)
		}
	}
	// The file's checks run on the merged result: a team on both ladders.
	if _, _, _, err := MergeCaseAlertSettings(cfg, CaseAlertSettings{Values: []CaseAlertValue{
		{"CRE", "teams.abts", json.RawMessage(`["apollo"]`)},
	}}); err == nil {
		t.Error("a team saved on both ladders was accepted")
	}
}

// What is reported as running: unset keys left out, sources marked, heads
// without a phone number.
func TestRunningCaseAlertValues(t *testing.T) {
	cfg := loadConfigText(t, prodShapedConfig)
	cfg.CRE.Heads.CS = Person{Name: "CS Head", Email: "cshead@example.test", Phone: "+10000000000"}
	cfg.CRE.Safety.AllowedNumbers = []string{"+10000000001", "+10000000002"}
	got := RunningCaseAlertValues(cfg, map[string]map[string]bool{"CRE": {"teams.abts": true}})
	b, _ := json.Marshal(got)
	var r struct {
		Ladders map[string]map[string]struct {
			Value  json.RawMessage
			Source string
		}
	}
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	cre, sre := r.Ladders["CRE"], r.Ladders["SRE"]
	if cre["teams.abts"].Source != "saved" || cre["onLeave"].Source != "file" {
		t.Errorf("sources: abts %q onLeave %q", cre["teams.abts"].Source, cre["onLeave"].Source)
	}
	if strings.Contains(string(b), "+10000000000") {
		t.Error("a head's phone number was reported")
	}
	var fileOnly struct {
		FileOnly map[string]map[string]any `json:"fileOnly"`
	}
	_ = json.Unmarshal(b, &fileOnly)
	if strings.Contains(string(b), "+10000000001") || fileOnly.FileOnly["CRE"]["safety.allowedNumbers"] != float64(2) {
		t.Errorf("allowed numbers listed, or not counted: %v", fileOnly.FileOnly["CRE"]["safety.allowedNumbers"])
	}
	if f := fileOnly.FileOnly["SRE"]; f["channel"] != "both" || f["enabled"] != true || f["phones.source"] != "profile" {
		t.Errorf("file-only SRE settings reported %v", f)
	}
	if _, ok := cre["heads.cre"]; ok {
		t.Error("an unset head was reported")
	}
	if _, ok := cre["timing.interval"]; ok {
		t.Error("an SRE key was reported for CRE")
	}
	if _, ok := sre["timing.interval"]; ok {
		t.Error("an unset interval was reported")
	}
	keys := make([]string, 0, len(sre))
	for k := range sre {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if !slices.Contains(keys, "teams.abts") || !slices.Contains(keys, "onLeave") {
		t.Errorf("SRE keys reported: %v", keys)
	}
}

// The keys the tab can save, as entity-service lists them
// (entity-service/internal/service/case_alert_ladder_values.go). A key added
// on one side only is skipped by the other, so both lists change together.
func TestCaseAlertKeysMatchTheTab(t *testing.T) {
	want := map[string][]string{
		"trigger.priorities": caseAlertBoth, "trigger.teams": caseAlertBoth, "trigger.excludeTeams": caseAlertBoth,
		"trigger.shifts": caseAlertBoth, "trigger.requireKnownTeam": caseAlertBoth, "onLeave": caseAlertBoth,
		"teams.abtType": caseAlertBoth, "teams.abts": caseAlertBoth,
		"teams.americas": caseAlertCRE, "teams.leadership": caseAlertCRE, "teams.teamLeadsToCall": caseAlertCRE,
		"teams.teamLeads": caseAlertCRE, "teams.rotaMembersToCall": caseAlertCRE, "teams.americasWeekendRotaShifts": caseAlertCRE,
		"teams.rotaTeams": caseAlertCRE, "teams.unassignedTeamLead": caseAlertCRE, "teams.unassignedTeamLeadCount": caseAlertCRE, "teams.unassignedLeads": caseAlertCRE,
		"heads.cre": caseAlertCRE, "heads.cs": caseAlertCRE, "alertDuty.tiers": caseAlertCRE, "alertDuty.perTeam": caseAlertCRE,
		"acknowledgement.requireBoth": caseAlertCRE,
		"email.enabled":               caseAlertCRE, "email.teamLists": caseAlertCRE, "email.creTeamList": caseAlertCRE,
		"email.csTeamList":  caseAlertCRE,
		"teams.defaultRota": caseAlertSRE, "timing.initialWait": caseAlertSRE, "timing.interval": caseAlertSRE,
		"timing.includeL4": caseAlertSRE, "unansweredChat": caseAlertSRE, "smeHandoffChat": caseAlertSRE,
		"enabled": caseAlertBoth, "channel": caseAlertBoth, "safety.maxCallsPerLadder": caseAlertBoth,
		"safety.callWithoutVerifiedLeads": caseAlertBoth, "safety.callHeadsWithoutLowerTiers": caseAlertBoth,
	}
	if len(want) != len(caseAlertKeys) {
		t.Errorf("%d keys here, %d listed", len(caseAlertKeys), len(want))
	}
	for k, ladders := range want {
		if spec, ok := caseAlertKeys[k]; !ok || !slices.Equal(spec.ladders, ladders) {
			t.Errorf("%s: %v here, want %v", k, spec.ladders, ladders)
		}
	}
}

// SME values (0238) lay over the file's sme: section, one key each, and are
// reported back with where each came from.
func TestMergeCaseAlertSettings_SMEValues(t *testing.T) {
	cfg := loadConfigText(t, prodShapedConfig)
	saved := CaseAlertSettings{Values: []CaseAlertValue{
		{"SME", "timing.interval", json.RawMessage(`"10m"`)},
		{"SME", "onLeave", json.RawMessage(`"call"`)},
		{"SME", "unansweredChat", json.RawMessage(`true`)},
		{"SME", "teams", json.RawMessage(`{"asgardeo-ops":"asgardeo"}`)},
	}}
	merged, _, changes, err := MergeCaseAlertSettings(cfg, saved)
	if err != nil {
		t.Fatal(err)
	}
	if time.Duration(merged.SME.Timing.Interval) != 10*time.Minute || merged.SME.OnLeave != OnLeaveCall ||
		!merged.SME.UnansweredChat || merged.SME.Teams["asgardeo-ops"] != "asgardeo" {
		t.Fatalf("sme = %+v", merged.SME)
	}
	if cfg.SME.Teams != nil || cfg.SME.OnLeave == OnLeaveCall {
		t.Fatal("the merge wrote into the caller's configuration")
	}
	if !strings.Contains(strings.Join(changes, "\n"), "sme.timing.interval: unset -> \"10m\"") {
		t.Errorf("changes = %v", changes)
	}
	running := RunningCaseAlertValues(merged, SavedCaseAlertKeys(saved))
	b, _ := json.Marshal(running)
	for _, want := range []string{`"SME":{`, `"timing.interval":{"source":"saved","value":"10m"}`, `"teams":{"source":"saved"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("running does not carry %s:\n%s", want, b)
		}
	}
}

// The ladders' own switches (enabled, channel, safety) and the SME page's
// on/off and channel can be saved on the tab, laid over the file like any
// other value. A file with no sme: section still gets the SME page from the
// tab.
func TestMergeCaseAlertSettings_Switches(t *testing.T) {
	cfg := loadConfigText(t, prodShapedConfig)
	cfg.SME = SMEConfig{}
	saved := CaseAlertSettings{Values: []CaseAlertValue{
		{"SME", "enabled", json.RawMessage(`true`)},
		{"SME", "channel", json.RawMessage(`"chat"`)},
		{"CRE", "channel", json.RawMessage(`"CHAT"`)},
		{"SRE", "enabled", json.RawMessage(`false`)},
		{"SRE", "safety.callWithoutVerifiedLeads", json.RawMessage(`true`)},
		{"SRE", "safety.callHeadsWithoutLowerTiers", json.RawMessage(`true`)},
		{"SRE", "safety.maxCallsPerLadder", json.RawMessage(`4`)},
	}}
	merged, _, _, err := MergeCaseAlertSettings(cfg, saved)
	if err != nil {
		t.Fatal(err)
	}
	if !merged.SME.Enabled || merged.SME.Channel != ChannelChat {
		t.Fatalf("sme = enabled %v channel %q", merged.SME.Enabled, merged.SME.Channel)
	}
	if merged.CRE.Channel != ChannelChat || merged.SRE.Enabled {
		t.Fatalf("cre channel %q, sre enabled %v", merged.CRE.Channel, merged.SRE.Enabled)
	}
	if s := merged.SRE.Safety; !s.CallWithoutVerifiedLeads || !s.CallHeadsWithoutLowerTiers || s.MaxCallsPerLadder != 4 {
		t.Fatalf("sre safety = %+v", s)
	}
	if cfg.SME.Enabled || cfg.CRE.Channel == ChannelChat {
		t.Fatal("the merge wrote into the caller's configuration")
	}
	running := RunningCaseAlertValues(merged, SavedCaseAlertKeys(saved))
	b, _ := json.Marshal(running)
	for _, want := range []string{
		`"enabled":{"source":"saved","value":true}`,
		`"channel":{"source":"saved","value":"chat"}`,
		`"safety.maxCallsPerLadder":{"source":"saved","value":4}`,
	} {
		if !strings.Contains(string(b), want) {
			t.Errorf("running does not carry %s:\n%s", want, b)
		}
	}
}

// A saved channel the alert service does not know is refused, and the merge
// keeps the file.
func TestMergeCaseAlertSettings_UnknownChannelKeepsTheFile(t *testing.T) {
	cfg := loadConfigText(t, prodShapedConfig)
	saved := CaseAlertSettings{Values: []CaseAlertValue{{"CRE", "channel", json.RawMessage(`"sms"`)}}}
	if _, _, _, err := MergeCaseAlertSettings(cfg, saved); err == nil {
		t.Fatal("an unknown channel was accepted")
	}
	saved = CaseAlertSettings{Values: []CaseAlertValue{{"SME", "channel", json.RawMessage(`""`)}}}
	if _, _, _, err := MergeCaseAlertSettings(cfg, saved); err == nil {
		t.Fatal("an empty SME channel was accepted")
	}
}

// Each SRE team's channel is the ladder's narrowed by its own; the SME
// teams with a room use the SME page's channel; whether a ladder runs needs
// the file's master switch too.
func TestEffectiveChannels(t *testing.T) {
	cfg := Config{
		Enabled: true,
		CRE:     LadderConfig{Enabled: true, Channel: ChannelBoth},
		SRE: LadderConfig{
			Enabled:      true,
			Channel:      ChannelBoth,
			Teams:        TeamKeys{ABTs: []string{"apollo", "artemis", "iaas"}},
			TeamChats:    map[string]Chat{"apollo": {Audience: "Apollo"}},
			TeamChannels: map[string]Channel{"iaas": ChannelChat, "artemis": ChannelLog},
		},
		SME: SMEConfig{Enabled: true, Channel: ChannelChat, TeamChats: map[string]Chat{"choreo-runtime": {Audience: "Choreo Runtime SME"}}},
	}
	got := EffectiveChannels(cfg)
	want := []EffectiveChannel{
		{Team: "apollo", Channel: "both", Room: true},
		{Team: "artemis", Channel: "log", TeamChannel: "log"},
		{Team: "iaas", Channel: "chat", TeamChannel: "chat"},
	}
	if !reflect.DeepEqual(got["SRE"].Teams, want) || got["SRE"].Channel != "both" || !got["SRE"].Enabled {
		t.Fatalf("SRE = %+v", got["SRE"])
	}
	if sme := got["SME"]; !sme.Enabled || sme.Channel != "chat" || len(sme.Teams) != 1 || sme.Teams[0].Team != "choreo-runtime" || !sme.Teams[0].Room {
		t.Fatalf("SME = %+v", sme)
	}
	cfg.Enabled = false
	if got := EffectiveChannels(cfg); got["CRE"].Enabled || got["SRE"].Enabled || got["SME"].Enabled {
		t.Fatalf("the master switch off still reads as running: %+v", got)
	}
	// An unset channel reads as call, the code's default.
	if got := EffectiveChannels(Config{}); got["CRE"].Channel != "call" {
		t.Fatalf("unset channel = %q", got["CRE"].Channel)
	}
}

// The running report says how each assignment group reaches a team, file and
// saved together, so the tab can tell a group the file already routes from one
// nothing routes.
func TestCaseAlertGroupRouting(t *testing.T) {
	cfg := Config{
		SRE: LadderConfig{
			Teams: TeamKeys{
				ABTs:          []string{"apollo", "artemis"},
				Aliases:       map[string]string{" Apollo SRE Group ": "Apollo", "artemis sre group": "artemis", "": "apollo"},
				DefaultGroups: []string{"MS/PC SRE Group", "default sre group", "Default SRE Group"},
			},
			TeamChats:     map[string]Chat{"apollo": {Audience: "Apollo"}, "artemis": {Audience: "Artemis"}},
			TeamlessChats: []string{"Apollo", "artemis"},
		},
		CRE: LadderConfig{Teams: TeamKeys{Aliases: map[string]string{"americas": "americas_cre_team"}}},
	}
	got := CaseAlertGroupRouting(cfg)
	sre := got["SRE"]
	if !reflect.DeepEqual(sre.Aliases, map[string]string{"apollo sre group": "apollo", "artemis sre group": "artemis"}) {
		t.Fatalf("SRE aliases = %v", sre.Aliases)
	}
	if !reflect.DeepEqual(sre.DefaultGroups, []string{"default sre group", "ms/pc sre group"}) {
		t.Fatalf("SRE default groups = %v", sre.DefaultGroups)
	}
	if !reflect.DeepEqual(sre.TeamlessRooms, []string{"apollo", "artemis"}) {
		t.Fatalf("SRE teamless rooms = %v", sre.TeamlessRooms)
	}
	cre := got["CRE"]
	if cre.Aliases["americas"] != "americas_cre_team" || len(cre.DefaultGroups) != 0 || len(cre.TeamlessRooms) != 0 {
		t.Fatalf("CRE = %+v", cre)
	}
	// Empty, never null on the wire.
	b, err := json.Marshal(CaseAlertGroupRouting(Config{}))
	if err != nil || strings.Contains(string(b), "null") {
		t.Fatalf("empty routing = %s, %v", b, err)
	}
	if _, ok := RunningCaseAlertValues(cfg, nil)["groups"]; !ok {
		t.Fatal("the running report carries no groups")
	}
}

// The tab shows every CRE setting, including the ones only the file or the
// code sets: whether a stopped chain replies, the rung emails (counted or
// flagged, never their addresses), whether the rule table is the built-in
// one, and when each step is called -- the same minutes the engine runs on.
func TestRunningCaseAlertValues_CREFileOnlyShowsTheWholeChain(t *testing.T) {
	cfg := loadConfigText(t, prodShapedConfig)
	cfg.CRE.Email = RungEmail{
		Enabled:     true,
		TeamLists:   map[string]string{"atlas": "atlas-list@example.test", "vega": "vega-list@example.test"},
		CRETeamList: "cre-list@example.test",
	}
	b, _ := json.Marshal(RunningCaseAlertValues(cfg, nil))
	var r struct {
		FileOnly map[string]map[string]json.RawMessage `json:"fileOnly"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	// The lists are tab values now (shown with their addresses, like a
	// head's); the read-only summary still only counts or flags them.
	if fo, _ := json.Marshal(r.FileOnly); strings.Contains(string(fo), "list@example.test") {
		t.Fatal("a rung email address was reported among the file-only values")
	}
	cre, sre := r.FileOnly["CRE"], r.FileOnly["SRE"]
	for key, want := range map[string]string{
		"ackChat":           "true",
		"email.enabled":     "true",
		"email.teamLists":   "2",
		"email.creTeamList": "true",
		"email.csTeamList":  "false",
		"rules.custom":      "false",
	} {
		if got := string(cre[key]); got != want {
			t.Errorf("CRE %s = %s, want %s", key, got, want)
		}
	}
	var timing map[string][][]int
	if err := json.Unmarshal(cre["timing"], &timing); err != nil {
		t.Fatal(err)
	}
	if got, want := timing["S0"], [][]int{{0}, {1, 2, 3}, {4, 5, 6}, {8, 9, 10}, {12, 13, 14}}; !reflect.DeepEqual(got, want) {
		t.Errorf("S0 timing = %v, want %v (the rules sheet's CRE timing)", got, want)
	}
	if got := timing["S1"][1]; !reflect.DeepEqual(got, []int{9, 11, 13}) {
		t.Errorf("S1 L1 = %v, want [9 11 13]", got)
	}
	if string(sre["ackChat"]) != "true" {
		t.Errorf("SRE ackChat = %s, want true", sre["ackChat"])
	}
	for _, key := range []string{"timing", "email.enabled", "rules.custom"} {
		if _, ok := sre[key]; ok {
			t.Errorf("SRE reports %s, a CRE-only setting", key)
		}
	}
}
