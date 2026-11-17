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
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"sort"
	"strings"
	"time"
)

// The ladder values are the rest of this file's cre: and sre: sections that
// can be saved on the Team Schedule's Alert Settings tab, one key each, in the
// file's own shape (entity-service checks each one's limits). A saved value
// replaces that key of the file; a key not saved keeps the file's value.
//
// What decides whether paging runs and what it may cost -- enabled, channel,
// safety, phones.source, testCall, sme, cre.rules -- is not among them and
// stays in the file.

// CaseAlertValue is a saved ladder value.
type CaseAlertValue struct {
	Ladder string          `json:"ladder"`
	Key    string          `json:"key"`
	Value  json.RawMessage `json:"value"`
}

// caseAlertKey is one key: which ladders have it, how a saved value is laid
// over a ladder, and how a ladder's value is read back for the report (false
// when the file leaves it unset).
type caseAlertKey struct {
	ladders []string
	set     func(l *LadderConfig, raw json.RawMessage) error
	get     func(l LadderConfig) (any, bool)
	// private values are logged as changed, never with the value (heads,
	// the rung email lists).
	private bool
}

var (
	caseAlertBoth = []string{"CRE", "SRE"}
	caseAlertCRE  = []string{"CRE"}
	caseAlertSRE  = []string{"SRE"}
)

// caseAlertKeys are the ladder values, by the file's name.
var caseAlertKeys = map[string]caseAlertKey{
	// The ladder's own switches (whether it runs, how it reaches people, and
	// the safety holds). The file's top-level enabled and the
	// INCIDENT_ESCALATION_ENABLED kill switch stay outside the tab.
	"enabled":                           boolKey(caseAlertBoth, func(l *LadderConfig) *bool { return &l.Enabled }),
	"channel":                           channelKey(caseAlertBoth, func(l *LadderConfig) *Channel { return &l.Channel }),
	"safety.maxCallsPerLadder":          intKey(caseAlertBoth, func(l *LadderConfig) *int { return &l.Safety.MaxCallsPerLadder }),
	"safety.callWithoutVerifiedLeads":   boolKey(caseAlertBoth, func(l *LadderConfig) *bool { return &l.Safety.CallWithoutVerifiedLeads }),
	"safety.callHeadsWithoutLowerTiers": boolKey(caseAlertBoth, func(l *LadderConfig) *bool { return &l.Safety.CallHeadsWithoutLowerTiers }),

	"trigger.priorities":       listKey(caseAlertBoth, func(l *LadderConfig) *[]string { return &l.Start.Priorities }),
	"trigger.teams":            listKey(caseAlertBoth, func(l *LadderConfig) *[]string { return &l.Start.Teams }),
	"trigger.excludeTeams":     listKey(caseAlertBoth, func(l *LadderConfig) *[]string { return &l.Start.ExcludeTeams }),
	"trigger.shifts":           listKey(caseAlertBoth, func(l *LadderConfig) *[]string { return &l.Start.Shifts }),
	"trigger.requireKnownTeam": boolKey(caseAlertBoth, func(l *LadderConfig) *bool { return &l.Start.RequireKnownTeam }),
	"onLeave": {
		ladders: caseAlertBoth,
		set: func(l *LadderConfig, raw json.RawMessage) error {
			var v OnLeave
			if err := strictJSON(raw, &v); err != nil {
				return err
			}
			l.OnLeave = v
			return nil
		},
		get: func(l LadderConfig) (any, bool) {
			v := OnLeave(strings.ToLower(strings.TrimSpace(string(l.OnLeave))))
			if v == "" {
				v = OnLeaveSkip
			}
			return string(v), true
		},
	},
	"teams.abtType": stringKey(caseAlertBoth, func(l *LadderConfig) *string { return &l.Teams.ABTType }),
	"teams.abts":    listKey(caseAlertBoth, func(l *LadderConfig) *[]string { return &l.Teams.ABTs }),

	"teams.americas":                  stringKey(caseAlertCRE, func(l *LadderConfig) *string { return &l.Teams.Americas }),
	"teams.leadership":                stringKey(caseAlertCRE, func(l *LadderConfig) *string { return &l.Teams.Leadership }),
	"teams.teamLeadsToCall":           intKey(caseAlertCRE, func(l *LadderConfig) *int { return &l.Teams.TeamLeadsToCall }),
	"teams.teamLeads":                 listKey(caseAlertCRE, func(l *LadderConfig) *[]string { return &l.Teams.TeamLeads }),
	"teams.americasWeekendRotaShifts": listKey(caseAlertCRE, func(l *LadderConfig) *[]string { return &l.Teams.AmericasWeekendRotaShifts }),
	"teams.rotaTeams":                 listKey(caseAlertCRE, func(l *LadderConfig) *[]string { return &l.Teams.RotaTeams }),
	"teams.unassignedTeamLead":        stringKey(caseAlertCRE, func(l *LadderConfig) *string { return &l.Teams.UnassignedTeamLead }),
	"teams.unassignedTeamLeadCount":   intKey(caseAlertCRE, func(l *LadderConfig) *int { return &l.Teams.UnassignedTeamLeadCount }),
	"teams.unassignedLeads":           stringKey(caseAlertCRE, func(l *LadderConfig) *string { return &l.Teams.UnassignedLeads }),
	"teams.rotaMembersToCall": {
		ladders: caseAlertCRE,
		set: func(l *LadderConfig, raw json.RawMessage) error {
			var v map[string]int
			if err := strictJSON(raw, &v); err != nil {
				return err
			}
			l.Teams.RotaMembersToCall = v
			return nil
		},
		get: func(l LadderConfig) (any, bool) {
			if l.Teams.RotaMembersToCall == nil {
				return nil, false
			}
			return maps.Clone(l.Teams.RotaMembersToCall), true
		},
	},
	"heads.cre": headKey(func(l *LadderConfig) *Person { return &l.Heads.CRE }),
	"heads.cs":  headKey(func(l *LadderConfig) *Person { return &l.Heads.CS }),
	// The Case Alert Notification email (rung_email.go): its switch, the
	// ABT/Americas team lists Cc'd on L1/L2, and the CRE and CS team lists
	// Cc'd on L3/L4.
	"email.enabled":     boolKey(caseAlertCRE, func(l *LadderConfig) *bool { return &l.Email.Enabled }),
	"email.creTeamList": privateKey(stringKey(caseAlertCRE, func(l *LadderConfig) *string { return &l.Email.CRETeamList })),
	"email.csTeamList":  privateKey(stringKey(caseAlertCRE, func(l *LadderConfig) *string { return &l.Email.CSTeamList })),
	"email.teamLists": {
		ladders: caseAlertCRE,
		private: true,
		set: func(l *LadderConfig, raw json.RawMessage) error {
			var v map[string]string
			if err := strictJSON(raw, &v); err != nil {
				return err
			}
			l.Email.TeamLists = v
			return nil
		},
		get: func(l LadderConfig) (any, bool) {
			if l.Email.TeamLists == nil {
				return nil, false
			}
			return maps.Clone(l.Email.TeamLists), true
		},
	},
	"alertDuty.tiers":   listKey(caseAlertCRE, func(l *LadderConfig) *[]string { return &l.AlertDuty.Tiers }),
	"alertDuty.perTeam": intKey(caseAlertCRE, func(l *LadderConfig) *int { return &l.AlertDuty.PerTeam }),
	"acknowledgement.requireBoth": {
		ladders: caseAlertCRE,
		set: func(l *LadderConfig, raw json.RawMessage) error {
			var v bool
			if err := strictJSON(raw, &v); err != nil {
				return err
			}
			l.Acknowledgement.RequireBoth = &v
			return nil
		},
		get: func(l LadderConfig) (any, bool) {
			if l.Acknowledgement.RequireBoth == nil {
				return nil, false
			}
			return *l.Acknowledgement.RequireBoth, true
		},
	},

	"teams.defaultRota":  stringKey(caseAlertSRE, func(l *LadderConfig) *string { return &l.Teams.DefaultRota }),
	"timing.initialWait": durationKey(func(l *LadderConfig) *Duration { return &l.Timing.InitialWait }, true),
	"timing.interval":    durationKey(func(l *LadderConfig) *Duration { return &l.Timing.Interval }, false),
	"timing.includeL4":   boolKey(caseAlertSRE, func(l *LadderConfig) *bool { return &l.Timing.IncludeL4 }),
	"unansweredChat":     boolKey(caseAlertSRE, func(l *LadderConfig) *bool { return &l.UnansweredChat }),
	"smeHandoffChat":     boolKey(caseAlertSRE, func(l *LadderConfig) *bool { return &l.SMEHandoffChat }),
}

// smeCaseAlertKey is one key of the file's sme: section that can be saved
// (ladder SME, 0238): how a saved value is laid over it, and read back.
type smeCaseAlertKey struct {
	set func(s *SMEConfig, raw json.RawMessage) error
	get func(s SMEConfig) (any, bool)
}

// smeCaseAlertKeys are the SME page's values, by the file's name, including
// whether it runs and its channel. Its safety and chat rooms are not among
// them (rooms are saved as rooms).
var smeCaseAlertKeys = map[string]smeCaseAlertKey{
	"enabled": {
		set: func(s *SMEConfig, raw json.RawMessage) error { return strictJSON(raw, &s.Enabled) },
		get: func(s SMEConfig) (any, bool) { return s.Enabled, true },
	},
	"channel": {
		set: func(s *SMEConfig, raw json.RawMessage) error { return setChannel(raw, &s.Channel) },
		get: func(s SMEConfig) (any, bool) { return channelValue(s.Channel), true },
	},
	"timing.initialWait": smeDurationKey(func(s *SMEConfig) *Duration { return &s.Timing.InitialWait }, true),
	"timing.interval":    smeDurationKey(func(s *SMEConfig) *Duration { return &s.Timing.Interval }, false),
	"onLeave": {
		set: func(s *SMEConfig, raw json.RawMessage) error {
			var v OnLeave
			if err := strictJSON(raw, &v); err != nil {
				return err
			}
			s.OnLeave = v
			return nil
		},
		get: func(s SMEConfig) (any, bool) {
			v := OnLeave(strings.ToLower(strings.TrimSpace(string(s.OnLeave))))
			if v == "" {
				v = OnLeaveSkip
			}
			return string(v), true
		},
	},
	"unansweredChat": {
		set: func(s *SMEConfig, raw json.RawMessage) error { return strictJSON(raw, &s.UnansweredChat) },
		get: func(s SMEConfig) (any, bool) { return s.UnansweredChat, true },
	},
	"teams": {
		set: func(s *SMEConfig, raw json.RawMessage) error {
			var v map[string]string
			if err := strictJSON(raw, &v); err != nil {
				return err
			}
			s.Teams = v
			return nil
		},
		get: func(s SMEConfig) (any, bool) {
			if s.Teams == nil {
				return nil, false
			}
			return maps.Clone(s.Teams), true
		},
	},
}

func smeDurationKey(field func(*SMEConfig) *Duration, zeroIsSet bool) smeCaseAlertKey {
	return smeCaseAlertKey{
		set: func(s *SMEConfig, raw json.RawMessage) error {
			var v string
			if err := strictJSON(raw, &v); err != nil {
				return err
			}
			d, err := time.ParseDuration(strings.TrimSpace(v))
			if err != nil {
				return fmt.Errorf("%q is not a duration", v)
			}
			*field(s) = Duration(d)
			return nil
		},
		get: func(s SMEConfig) (any, bool) {
			d := time.Duration(*field(&s))
			if d == 0 && !zeroIsSet {
				return "", false
			}
			return fileDuration(d), true
		},
	}
}

// strictJSON decodes raw into v, refusing what v does not have room for.
func strictJSON(raw json.RawMessage, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("not the shape this setting takes: %w", err)
	}
	return nil
}

func listKey(ladders []string, field func(*LadderConfig) *[]string) caseAlertKey {
	return caseAlertKey{
		ladders: ladders,
		set: func(l *LadderConfig, raw json.RawMessage) error {
			var v []string
			if err := strictJSON(raw, &v); err != nil {
				return err
			}
			if v == nil {
				v = []string{}
			}
			*field(l) = v
			return nil
		},
		get: func(l LadderConfig) (any, bool) {
			// Absent from the file is unset; written as [] it is a value.
			v := *field(&l)
			if v == nil {
				return nil, false
			}
			return slices.Clone(v), true
		},
	}
}

func boolKey(ladders []string, field func(*LadderConfig) *bool) caseAlertKey {
	return caseAlertKey{
		ladders: ladders,
		set: func(l *LadderConfig, raw json.RawMessage) error {
			return strictJSON(raw, field(l))
		},
		get: func(l LadderConfig) (any, bool) { return *field(&l), true },
	}
}

// channelKey is a ladder's channel: call, chat, both or log.
func channelKey(ladders []string, field func(*LadderConfig) *Channel) caseAlertKey {
	return caseAlertKey{
		ladders: ladders,
		set:     func(l *LadderConfig, raw json.RawMessage) error { return setChannel(raw, field(l)) },
		get:     func(l LadderConfig) (any, bool) { return channelValue(*field(&l)), true },
	}
}

// setChannel reads a saved channel; anything but call, chat, both or log is
// refused, so the merge keeps the file.
func setChannel(raw json.RawMessage, dst *Channel) error {
	var v string
	if err := strictJSON(raw, &v); err != nil {
		return err
	}
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("a channel is required: call, chat, both or log")
	}
	ch, err := ParseChannel(v)
	if err != nil {
		return err
	}
	*dst = ch
	return nil
}

// channelValue is a channel as reported: unset reads as call, the code's
// default.
func channelValue(c Channel) string {
	ch, err := ParseChannel(string(c))
	if err != nil {
		return string(c)
	}
	return string(ch)
}

func intKey(ladders []string, field func(*LadderConfig) *int) caseAlertKey {
	return caseAlertKey{
		ladders: ladders,
		set: func(l *LadderConfig, raw json.RawMessage) error {
			return strictJSON(raw, field(l))
		},
		get: func(l LadderConfig) (any, bool) { return *field(&l), true },
	}
}

// stringKey is a text value; an empty one is left out of the report, as
// unset in the file -- the code's default applies either way.
func stringKey(ladders []string, field func(*LadderConfig) *string) caseAlertKey {
	return caseAlertKey{
		ladders: ladders,
		set: func(l *LadderConfig, raw json.RawMessage) error {
			return strictJSON(raw, field(l))
		},
		get: func(l LadderConfig) (any, bool) {
			v := strings.TrimSpace(*field(&l))
			return v, v != ""
		},
	}
}

// durationKey is an SRE timing value. zeroIsSet says whether zero is a value
// (initialWait: call at once) or unset (interval: the five-minute default).
func durationKey(field func(*LadderConfig) *Duration, zeroIsSet bool) caseAlertKey {
	return caseAlertKey{
		ladders: caseAlertSRE,
		set: func(l *LadderConfig, raw json.RawMessage) error {
			var s string
			if err := strictJSON(raw, &s); err != nil {
				return err
			}
			d, err := time.ParseDuration(strings.TrimSpace(s))
			if err != nil {
				return fmt.Errorf("%q is not a duration", s)
			}
			*field(l) = Duration(d)
			return nil
		},
		get: func(l LadderConfig) (any, bool) {
			d := time.Duration(*field(&l))
			if d == 0 && !zeroIsSet {
				return "", false
			}
			return fileDuration(d), true
		},
	}
}

// fileDuration writes a duration the way the file does: 5m, 30s, 0s.
func fileDuration(d time.Duration) string {
	switch {
	case d == 0:
		return "0s"
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	case d%time.Second == 0:
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
	return d.String()
}

// caseAlertHead is a head as saved on the tab: who, not how to reach them --
// the number comes from their profile, as for everyone else the ladder calls.
type caseAlertHead struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// dropUnknownEmailTeamLists drops the rung email lists of teams that are
// neither an ABT nor the Americas team, the one check on cre.email that
// spans keys. A list saved on the tab for such a team, or a saved
// teams.abts that leaves out a team the file has a list for, would
// otherwise refuse the whole merge and put every saved setting out of use.
// The list is replaced, never written into, so the file's map is untouched.
func dropUnknownEmailTeamLists(l *LadderConfig, changes *[]string) {
	if len(l.Teams.ABTs) == 0 || len(l.Email.TeamLists) == 0 {
		return
	}
	var dropped []string
	for k := range l.Email.TeamLists {
		key := teamKeyFor(k)
		if key != "" && !contains(l.Teams.ABTs, key) && key != teamKeyFor(l.Teams.Americas) {
			dropped = append(dropped, k)
		}
	}
	if len(dropped) == 0 {
		return
	}
	slices.Sort(dropped)
	kept := maps.Clone(l.Email.TeamLists)
	for _, k := range dropped {
		delete(kept, k)
		*changes = append(*changes, fmt.Sprintf("cre.email.teamLists: dropped %s (neither in teams.abts nor teams.americas)", k))
	}
	l.Email.TeamLists = kept
}

// privateKey logs k's changes without the value (an email address).
func privateKey(k caseAlertKey) caseAlertKey {
	k.private = true
	return k
}

func headKey(field func(*LadderConfig) *Person) caseAlertKey {
	return caseAlertKey{
		ladders: caseAlertCRE,
		private: true,
		set: func(l *LadderConfig, raw json.RawMessage) error {
			var h caseAlertHead
			if err := strictJSON(raw, &h); err != nil {
				return err
			}
			*field(l) = Person{Name: strings.TrimSpace(h.Name), Email: strings.TrimSpace(h.Email)}
			return nil
		},
		get: func(l LadderConfig) (any, bool) {
			p := *field(&l)
			if !p.Set() {
				return nil, false
			}
			return caseAlertHead{Name: p.Name, Email: p.Email}, true
		},
	}
}

// ladderFor is the ladder a saved value is for, in out.
func ladderFor(out *Config, ladder string) *LadderConfig {
	switch ladder {
	case "CRE":
		return &out.CRE
	case "SRE":
		return &out.SRE
	}
	return nil
}

// usableCaseAlertValue is the ladder and spec of a saved value this build can
// apply, or why not.
func usableCaseAlertValue(v CaseAlertValue) (string, caseAlertKey, string) {
	ladder := strings.ToUpper(strings.TrimSpace(v.Ladder))
	key := strings.TrimSpace(v.Key)
	spec, known := caseAlertKeys[key]
	where := strings.ToLower(ladder) + "." + key
	if ladder == "SME" {
		if _, ok := smeCaseAlertKeys[key]; !ok {
			return "", spec, fmt.Sprintf("skipped %s: not a setting this build knows", where)
		}
		return ladder, spec, ""
	}
	switch {
	case ladder != "CRE" && ladder != "SRE":
		return "", spec, fmt.Sprintf("skipped a value for ladder %q", v.Ladder)
	case !known:
		return "", spec, fmt.Sprintf("skipped %s: not a setting this build knows", where)
	case !slices.Contains(spec.ladders, ladder):
		return "", spec, fmt.Sprintf("skipped %s: not a %s setting", where, ladder)
	}
	return ladder, spec, ""
}

// SavedCaseAlertKeys is which keys of each ladder the saved values set, as
// MergeCaseAlertSettings applies them.
func SavedCaseAlertKeys(s CaseAlertSettings) map[string]map[string]bool {
	out := map[string]map[string]bool{"CRE": {}, "SRE": {}, "SME": {}}
	for _, v := range s.Values {
		if ladder, _, skip := usableCaseAlertValue(v); skip == "" {
			out[ladder][strings.TrimSpace(v.Key)] = true
		}
	}
	return out
}

// mergeCaseAlertValues lays the saved values over out, describing each change
// in changes. A key this build does not know, or one saved for the other
// ladder, is skipped and said so: entity-service may be newer than this
// service.
func mergeCaseAlertValues(out *Config, values []CaseAlertValue, changes *[]string) error {
	sorted := slices.Clone(values)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Ladder+" "+sorted[i].Key < sorted[j].Ladder+" "+sorted[j].Key
	})
	for _, v := range sorted {
		ladder, spec, skip := usableCaseAlertValue(v)
		if skip != "" {
			*changes = append(*changes, skip)
			continue
		}
		if ladder == "SME" {
			if err := mergeSMEValue(out, v, changes); err != nil {
				return err
			}
			continue
		}
		l := ladderFor(out, ladder)
		where := strings.ToLower(ladder) + "." + strings.TrimSpace(v.Key)
		was, wasSet := spec.get(*l)
		if err := spec.set(l, v.Value); err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}
		now, _ := spec.get(*l)
		if wasSet && sameJSON(was, now) {
			continue
		}
		if spec.private {
			*changes = append(*changes, where+": changed")
			continue
		}
		from := "unset"
		if wasSet {
			from = compactJSON(was)
		}
		*changes = append(*changes, fmt.Sprintf("%s: %s -> %s", where, from, compactJSON(now)))
	}
	return nil
}

// mergeSMEValue lays one saved value over the file's sme: section.
func mergeSMEValue(out *Config, v CaseAlertValue, changes *[]string) error {
	key := strings.TrimSpace(v.Key)
	spec := smeCaseAlertKeys[key]
	where := "sme." + key
	was, wasSet := spec.get(out.SME)
	if err := spec.set(&out.SME, v.Value); err != nil {
		return fmt.Errorf("%s: %w", where, err)
	}
	now, _ := spec.get(out.SME)
	if wasSet && sameJSON(was, now) {
		return nil
	}
	from := "unset"
	if wasSet {
		from = compactJSON(was)
	}
	*changes = append(*changes, fmt.Sprintf("%s: %s -> %s", where, from, compactJSON(now)))
	return nil
}

func compactJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func sameJSON(a, b any) bool { return compactJSON(a) == compactJSON(b) }

// fileOnlyCaseAlertValues is what only the file (or the code) sets, shown
// read-only on the tab so nobody has to open the file to see what a ladder may
// do: whether it runs, its channel, its caps, where numbers come from, and
// whether it replies when it stops. For CRE also the rung emails, whether the
// rule table is the built-in one, and when each step is called. The allowed
// numbers are counted, never listed; the rung emails' lists are counted or
// flagged, never their addresses.
func fileOnlyCaseAlertValues(cfg Config, ladder string, l LadderConfig) map[string]any {
	out := map[string]any{
		"enabled":                           cfg.Enabled && l.Enabled,
		"channel":                           string(l.Channel),
		"safety.maxCallsPerLadder":          l.Safety.MaxCallsPerLadder,
		"safety.allowedNumbers":             len(l.Safety.AllowedNumbers),
		"safety.callWithoutVerifiedLeads":   l.Safety.CallWithoutVerifiedLeads,
		"safety.callHeadsWithoutLowerTiers": l.Safety.CallHeadsWithoutLowerTiers,
		"phones.source":                     l.PhoneSource(),
		"ackChat":                           l.AckChatOn(),
	}
	if lvl, capped := l.CapLevel(); capped {
		out["safety.maxLevel"] = int(lvl)
	}
	if ladder == "CRE" {
		out["email.enabled"] = l.Email.Enabled
		out["email.teamLists"] = len(l.Email.TeamLists)
		out["email.creTeamList"] = strings.TrimSpace(l.Email.CRETeamList) != ""
		out["email.csTeamList"] = strings.TrimSpace(l.Email.CSTeamList) != ""
		out["rules.custom"] = len(l.Rules) > 0
		out["timing"] = creChainTiming(DefaultPolicy)
	}
	return out
}

// creChainTiming is when each step of the CRE chain is called, by severity:
// minutes after the case is created, one list per step L0 to L4. It comes from
// the policy the CRE engine runs on, so the tab cannot drift from the code.
// Every CRE rule has an L0 step, so L0 is always included.
func creChainTiming(policies map[string]PriorityPolicy) map[string][][]int {
	out := map[string][][]int{}
	for i, sev := range []string{"S0", "S1", "S2", "S3", "S4"} {
		p, ok := policies[fmt.Sprintf("P%d", i)]
		if !ok {
			continue
		}
		steps := make([][]int, int(Level4)+1)
		for _, a := range Schedule(p, true) {
			steps[a.Level] = append(steps[a.Level], int(a.After/time.Minute))
		}
		out[sev] = steps
	}
	return out
}

// RunningCaseAlertValues is what cfg runs with, key by key, for the Case
// Alerts tab: each ladder value's value and where it came from ("saved" for
// a key in saved, else "file"). A key the file leaves unset is left out.
// Heads carry a name and an email, never a phone number. fileOnly is what
// only the file sets (fileOnlyCaseAlertValues).
func RunningCaseAlertValues(cfg Config, saved map[string]map[string]bool) map[string]any {
	fileOnly := map[string]any{}
	for _, ladder := range caseAlertBoth {
		fileOnly[ladder] = fileOnlyCaseAlertValues(cfg, ladder, *ladderFor(&cfg, ladder))
	}
	fileOnly["SME"] = map[string]any{
		"enabled":               cfg.Enabled && cfg.SME.Enabled,
		"channel":               string(cfg.SME.Channel),
		"safety.allowedNumbers": len(cfg.SME.Safety.AllowedNumbers),
	}
	ladders := map[string]any{}
	sme := map[string]any{}
	for key, spec := range smeCaseAlertKeys {
		v, ok := spec.get(cfg.SME)
		if !ok {
			continue
		}
		source := "file"
		if saved["SME"][key] {
			source = "saved"
		}
		sme[key] = map[string]any{"value": v, "source": source}
	}
	ladders["SME"] = sme
	for _, ladder := range caseAlertBoth {
		l := ladderFor(&cfg, ladder)
		out := map[string]any{}
		for key, spec := range caseAlertKeys {
			if !slices.Contains(spec.ladders, ladder) {
				continue
			}
			v, ok := spec.get(*l)
			if !ok {
				continue
			}
			source := "file"
			if saved[ladder][key] {
				source = "saved"
			}
			out[key] = map[string]any{"value": v, "source": source}
		}
		ladders[ladder] = out
	}
	return map[string]any{"ladders": ladders, "fileOnly": fileOnly, "channels": EffectiveChannels(cfg), "groups": CaseAlertGroupRouting(cfg)}
}

// GroupRouting is how one ladder sends an assignment group to a team as the
// service runs now: the file's teams.aliases and teams.defaultGroups with the
// saved routes merged in (a route to a team is an alias, a route to a rota a
// default group). Group names are lower-cased and trimmed, as the resolver
// matches them; teams are team keys. TeamlessRooms are the SRE team rooms a
// default group's cards go to (sre.teamlessChats); empty, they go to the
// ladder's shared room. The tab uses it to say where each assignment group's
// alerts go, and which groups nothing routes.
type GroupRouting struct {
	Aliases       map[string]string `json:"aliases"`
	DefaultGroups []string          `json:"defaultGroups"`
	TeamlessRooms []string          `json:"teamlessRooms"`
	// GroupRooms are the groups with a chat room of their own
	// (sre.groupChats), lower-cased. SRE only.
	GroupRooms []string `json:"groupRooms,omitempty"`
	// SharedRoom is whether the ladder has its shared room (sre.chat), where
	// a group that maps to no room gets its card: its variable is set, or it
	// names an audience. SRE only; absent from an older report.
	SharedRoom *bool `json:"sharedRoom,omitempty"`
}

// chatConfigured reports whether a chat section can be posted to: its
// variable holds a URL, or (with no variable) it names an audience, whose
// GOOGLE_CHAT_SPACES entry is not checked here.
func chatConfigured(c Chat) bool {
	if name := strings.TrimSpace(c.WebhookURLEnv); name != "" {
		return strings.TrimSpace(os.Getenv(name)) != ""
	}
	return strings.TrimSpace(c.Audience) != ""
}

// CaseAlertGroupRouting reports GroupRouting for the CRE and SRE ladders.
// Only SRE has default groups and teamless rooms.
func CaseAlertGroupRouting(cfg Config) map[string]GroupRouting {
	out := map[string]GroupRouting{}
	for _, ladder := range caseAlertBoth {
		l := ladderFor(&cfg, ladder)
		g := GroupRouting{Aliases: map[string]string{}, DefaultGroups: []string{}, TeamlessRooms: []string{}}
		for group, team := range l.Teams.Aliases {
			if k := teamKeyFor(group); k != "" && teamKeyFor(team) != "" {
				g.Aliases[k] = teamKeyFor(team)
			}
		}
		if ladder == "SRE" {
			for _, d := range l.Teams.DefaultGroups {
				if k := teamKeyFor(d); k != "" && !slices.Contains(g.DefaultGroups, k) {
					g.DefaultGroups = append(g.DefaultGroups, k)
				}
			}
			for _, k := range l.TeamlessChats {
				if k = teamKeyFor(k); k != "" && !slices.Contains(g.TeamlessRooms, k) {
					g.TeamlessRooms = append(g.TeamlessRooms, k)
				}
			}
			sort.Strings(g.DefaultGroups)
			for group := range l.GroupChats {
				if k := teamKeyFor(group); k != "" {
					g.GroupRooms = append(g.GroupRooms, k)
				}
			}
			sort.Strings(g.GroupRooms)
			shared := chatConfigured(l.Chat)
			g.SharedRoom = &shared
		}
		out[ladder] = g
	}
	return out
}

// EffectiveChannel is how one ladder, or one team on it, reaches people as
// the service runs now: the ladder's channel narrowed by the team's own (an
// SRE team's saved channel can only drop calls or chat, never add one).
type EffectiveChannel struct {
	Team        string `json:"team,omitempty"`
	Channel     string `json:"channel"`
	TeamChannel string `json:"teamChannel,omitempty"`
	Room        bool   `json:"room"`
}

// LadderChannels is one ladder's switches as running, and its teams.
type LadderChannels struct {
	Enabled bool               `json:"enabled"`
	Channel string             `json:"channel"`
	Teams   []EffectiveChannel `json:"teams"`
}

// EffectiveChannels reports, per ladder, whether it runs, its channel, and
// for each SRE and SME team the channel it actually uses and whether it has
// a room of its own. CRE teams have no channel of their own.
func EffectiveChannels(cfg Config) map[string]LadderChannels {
	out := map[string]LadderChannels{
		"CRE": {Enabled: cfg.Enabled && cfg.CRE.Enabled, Channel: channelValue(cfg.CRE.Channel), Teams: []EffectiveChannel{}},
	}
	sre := LadderChannels{Enabled: cfg.Enabled && cfg.SRE.Enabled, Channel: channelValue(cfg.SRE.Channel), Teams: []EffectiveChannel{}}
	ladderCh, _ := ParseChannel(string(cfg.SRE.Channel))
	keys := map[string]bool{}
	for _, k := range cfg.SRE.Teams.ABTs {
		keys[teamKeyFor(k)] = true
	}
	for k := range cfg.SRE.TeamChats {
		keys[teamKeyFor(k)] = true
	}
	for k := range cfg.SRE.TeamChannels {
		keys[teamKeyFor(k)] = true
	}
	for _, k := range slices.Sorted(maps.Keys(keys)) {
		if k == "" {
			continue
		}
		own := cfg.SRE.TeamChannels[k]
		_, room := cfg.SRE.TeamChats[k]
		sre.Teams = append(sre.Teams, EffectiveChannel{
			Team: k, Channel: string(narrowChannel(ladderCh, own)), TeamChannel: string(own), Room: room,
		})
	}
	out["SRE"] = sre
	sme := LadderChannels{Enabled: cfg.Enabled && cfg.SME.Enabled, Channel: channelValue(cfg.SME.Channel), Teams: []EffectiveChannel{}}
	for _, k := range slices.Sorted(maps.Keys(cfg.SME.TeamChats)) {
		sme.Teams = append(sme.Teams, EffectiveChannel{Team: teamKeyFor(k), Channel: sme.Channel, Room: true})
	}
	out["SME"] = sme
	return out
}
