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

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// The ladder values are the rest of csm-notification-service's paging file
// that can be set on the Alert Settings tab: one key of the file's cre:, sre:
// or (0238) sme: section each, in the file's own shape. Only the keys below can be saved,
// each with its limits. What decides whether paging runs and what it may cost
// -- enabled, channel, safety, phones.source, testCall -- is deliberately not
// among them: changing those stays a reviewed change to the file.
//
// The alert service checks the merged result again with the file's own checks
// (Config.validate) and keeps the file when they refuse it, so these limits
// are the first line, not the only one.

// caseAlertValueCheck checks one value and returns it as it is saved.
type caseAlertValueCheck func(ctx context.Context, s *caseAlertSettingsService, ladder string, raw json.RawMessage) (any, error)

type caseAlertValueSpec struct {
	ladders []string
	check   caseAlertValueCheck
}

const (
	maxCaseAlertValues     = 50
	maxCaseAlertValueBytes = 8 << 10
)

var (
	bothLadders = []string{familyCRE, familySRE}
	creOnly     = []string{familyCRE}
	sreOnly     = []string{familySRE}
	sreSME      = []string{familySRE, familySME}
	allLadders  = []string{familyCRE, familySRE, familySME}
	smeOnly     = []string{familySME}

	// caseAlertTeamKey is what a team key looks like (team.key).
	caseAlertTeamKey = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}$`)
	caseAlertTier    = regexp.MustCompile(`^T[1-9]$`)
	caseAlertPrio    = regexp.MustCompile(`^[SP][0-4]$`)

	// caseAlertShifts are the alert service's shift names (its resolver.go),
	// which trigger.shifts and teams.rotaMembersToCall are keyed by. They are
	// not Team Schedule shift codes.
	caseAlertShifts = []string{"LK", "LK_MORNING", "LK_EVENING", "LK_WEEKEND", "USA", "USA_WEEKEND"}
)

// caseAlertValueSpecs are the keys that can be saved, by the file's name.
var caseAlertValueSpecs = map[string]caseAlertValueSpec{
	// The ladder's own switches: whether it runs, how it reaches people and
	// the safety holds. The file's top-level enabled and the alert service's
	// INCIDENT_ESCALATION_ENABLED kill switch stay outside the tab.
	"enabled":                           {allLadders, boolValue},
	"channel":                           {allLadders, enumValue("call", "chat", "both", "log")},
	"safety.maxCallsPerLadder":          {bothLadders, intValue(1, 50)},
	"safety.callWithoutVerifiedLeads":   {bothLadders, boolValue},
	"safety.callHeadsWithoutLowerTiers": {bothLadders, boolValue},

	"trigger.priorities":       {bothLadders, stringListValue(5, priorityItem)},
	"trigger.teams":            {bothLadders, stringListValue(100, groupNameItem)},
	"trigger.excludeTeams":     {bothLadders, stringListValue(100, groupNameItem)},
	"trigger.shifts":           {bothLadders, stringListValue(len(caseAlertShifts), shiftNameItem)},
	"trigger.requireKnownTeam": {bothLadders, boolValue},
	"onLeave":                  {allLadders, enumValue("skip", "call")},
	"teams.abtType":            {bothLadders, abtTypeValue},
	"teams.abts":               {bothLadders, stringListValue(30, ladderTeamItem)},

	"teams.americas":                  {creOnly, teamValue(familyCRE)},
	"teams.leadership":                {creOnly, teamKeyFormatValue},
	"teams.teamLeadsToCall":           {creOnly, intValue(0, 20)},
	"teams.teamLeads":                 {creOnly, stringListValue(30, ladderTeamItem)},
	"teams.rotaMembersToCall":         {creOnly, rotaMembersValue},
	"teams.americasWeekendRotaShifts": {creOnly, stringListValue(10, shiftCodeItem)},
	"teams.rotaTeams":                 {creOnly, stringListValue(30, anyTeamItem)},
	"teams.unassignedTeamLead":        {creOnly, enumValue("pool", "none")},
	"teams.unassignedTeamLeadCount":   {creOnly, intValue(0, 10)},
	"teams.unassignedLeads":           {creOnly, enumValue("rules", "shift_leads")},
	"heads.cre":                       {creOnly, headValue},
	"heads.cs":                        {creOnly, headValue},
	"alertDuty.tiers":                 {creOnly, stringListValue(9, tierItem)},
	"alertDuty.perTeam":               {creOnly, intValue(0, 10)},
	"acknowledgement.requireBoth":     {creOnly, boolValue},

	// The Case Alert Notification email: its switch, each ABT's (and the
	// Americas') team list Cc'd on L1/L2, and the CRE and CS team lists
	// Cc'd on L3/L4. Lists are mailing addresses, not users.
	"email.enabled":     {creOnly, boolValue},
	"email.teamLists":   {creOnly, emailTeamListsValue},
	"email.creTeamList": {creOnly, mailingListValue},
	"email.csTeamList":  {creOnly, mailingListValue},

	"teams.defaultRota":  {sreOnly, defaultRotaValue},
	"timing.initialWait": {sreSME, durationValue(0, 30*time.Minute, "0s or 2m")},
	"timing.interval":    {sreSME, durationValue(time.Minute, 30*time.Minute, "5m")},
	"timing.includeL4":   {sreOnly, boolValue},
	"unansweredChat":     {sreSME, boolValue},
	"smeHandoffChat":     {sreOnly, boolValue},

	// sme.teams: a Special Ops team key to the SME team it pages.
	"teams": {smeOnly, smeTeamsValue},
}

// smeTeamsValue is sme.teams: Special Ops team keys, each to one of the Team
// Schedule's SME teams.
func smeTeamsValue(ctx context.Context, s *caseAlertSettingsService, _ string, raw json.RawMessage) (any, error) {
	var in map[string]string
	if err := json.Unmarshal(raw, &in); err != nil || in == nil {
		return nil, invalidf("send an object of Special Ops team key to SME team key")
	}
	if len(in) > 50 {
		return nil, invalidf("at most 50 teams")
	}
	out := make(map[string]string, len(in))
	for from, to := range in {
		k := strings.ToLower(strings.TrimSpace(from))
		if !caseAlertTeamKey.MatchString(k) {
			return nil, invalidf("%q is not a team key", from)
		}
		team, err := teamItem(ctx, s, familySME, strings.TrimSpace(to))
		if err != nil {
			return nil, err
		}
		out[k] = team
	}
	return out, nil
}

// checkCaseAlertValue checks one key's value for ladder and returns it as
// JSON, as it is saved. Every refusal names the key.
func (s *caseAlertSettingsService) checkCaseAlertValue(ctx context.Context, ladder, key string, raw json.RawMessage) ([]byte, error) {
	spec, ok := caseAlertValueSpecs[key]
	if !ok {
		return nil, &apierror.ValidationError{Msg: key + " is not a setting that can be saved here"}
	}
	if !slices.Contains(spec.ladders, ladder) {
		return nil, &apierror.ValidationError{Msg: fmt.Sprintf("%s is not a %s setting", key, ladder)}
	}
	if len(raw) > maxCaseAlertValueBytes {
		return nil, &apierror.ValidationError{Msg: fmt.Sprintf("%s: the value is longer than %d bytes", key, maxCaseAlertValueBytes)}
	}
	if t := bytes.TrimSpace(raw); len(t) == 0 || bytes.Equal(t, []byte("null")) {
		return nil, &apierror.ValidationError{Msg: key + ": send a value; to go back to the alert service's own, remove the saved one"}
	}
	v, err := spec.check(ctx, s, ladder, raw)
	var bad caseAlertValueError
	if errors.As(err, &bad) {
		return nil, &apierror.ValidationError{Msg: key + ": " + bad.msg}
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

// caseAlertValueError is a value refused by its limits; anything else a
// check returns is a failure to look something up.
type caseAlertValueError struct{ msg string }

func (e caseAlertValueError) Error() string { return e.msg }

func invalidf(format string, args ...any) error {
	return caseAlertValueError{msg: fmt.Sprintf(format, args...)}
}

func boolValue(_ context.Context, _ *caseAlertSettingsService, _ string, raw json.RawMessage) (any, error) {
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, invalidf("must be true or false")
	}
	return b, nil
}

func intValue(min, max int) caseAlertValueCheck {
	return func(_ context.Context, _ *caseAlertSettingsService, _ string, raw json.RawMessage) (any, error) {
		var n int
		if err := json.Unmarshal(raw, &n); err != nil || n < min || n > max {
			return nil, invalidf("must be a whole number from %d to %d", min, max)
		}
		return n, nil
	}
}

func stringValue(raw json.RawMessage) (string, error) {
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", invalidf("must be text")
	}
	return strings.TrimSpace(v), nil
}

func enumValue(options ...string) caseAlertValueCheck {
	return func(_ context.Context, _ *caseAlertSettingsService, _ string, raw json.RawMessage) (any, error) {
		v, err := stringValue(raw)
		if err != nil {
			return nil, err
		}
		v = strings.ToLower(v)
		if !slices.Contains(options, v) {
			return nil, invalidf("must be %s", strings.Join(options, " or "))
		}
		return v, nil
	}
}

// durationValue is a time from min to max; example is one or two values in
// that range, as the refusal shows them.
func durationValue(min, max time.Duration, example string) caseAlertValueCheck {
	return func(_ context.Context, _ *caseAlertSettingsService, _ string, raw json.RawMessage) (any, error) {
		v, err := stringValue(raw)
		if err != nil {
			return nil, err
		}
		d, err := time.ParseDuration(v)
		if err != nil || d < min || d > max {
			return nil, invalidf("must be a time from %s to %s, written like %s", fmtCaseAlertDuration(min), fmtCaseAlertDuration(max), example)
		}
		return v, nil
	}
}

func fmtCaseAlertDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	if d%time.Minute == 0 {
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
	return d.String()
}

// caseAlertListItem checks and normalises one item of a list value.
type caseAlertListItem func(ctx context.Context, s *caseAlertSettingsService, ladder, v string) (string, error)

// stringListValue is a list of at most max items, each checked by item,
// with no item twice. An empty list is a value too: it replaces the file's
// list with none.
func stringListValue(max int, item caseAlertListItem) caseAlertValueCheck {
	return func(ctx context.Context, s *caseAlertSettingsService, ladder string, raw json.RawMessage) (any, error) {
		var in []string
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, invalidf("must be a list of text")
		}
		if len(in) > max {
			return nil, invalidf("can have at most %d entries", max)
		}
		out := make([]string, 0, len(in))
		for _, v := range in {
			n, err := item(ctx, s, ladder, strings.TrimSpace(v))
			if err != nil {
				return nil, err
			}
			if slices.ContainsFunc(out, func(o string) bool { return strings.EqualFold(o, n) }) {
				return nil, invalidf("lists %q twice", n)
			}
			out = append(out, n)
		}
		return out, nil
	}
}

func priorityItem(_ context.Context, _ *caseAlertSettingsService, _, v string) (string, error) {
	p := strings.ToUpper(v)
	if !caseAlertPrio.MatchString(p) {
		return "", invalidf("%q is not a priority; use S0 to S4", v)
	}
	return "S" + p[1:], nil
}

func groupNameItem(_ context.Context, _ *caseAlertSettingsService, _, v string) (string, error) {
	if v == "" || len(v) > maxCaseAlertGroupName {
		return "", invalidf("each group name must be 1 to %d characters", maxCaseAlertGroupName)
	}
	return v, nil
}

func shiftNameItem(_ context.Context, _ *caseAlertSettingsService, _, v string) (string, error) {
	n := strings.ToUpper(v)
	if !slices.Contains(caseAlertShifts, n) {
		return "", invalidf("%q is not a shift; use %s", v, strings.Join(caseAlertShifts, ", "))
	}
	return n, nil
}

func tierItem(_ context.Context, _ *caseAlertSettingsService, _, v string) (string, error) {
	n := strings.ToUpper(v)
	if !caseAlertTier.MatchString(n) {
		return "", invalidf("%q is not a tier; use T1 to T9", v)
	}
	return n, nil
}

func shiftCodeItem(ctx context.Context, s *caseAlertSettingsService, _, v string) (string, error) {
	n := strings.ToUpper(v)
	ok, err := s.repo.ShiftCodeExists(ctx, n)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", invalidf("%q is not an active Team Schedule shift", v)
	}
	return n, nil
}

// teamItem is a Team Schedule team; family "" takes any family.
func teamItem(ctx context.Context, s *caseAlertSettingsService, family, v string) (string, error) {
	k := strings.ToLower(v)
	if !caseAlertTeamKey.MatchString(k) {
		return "", invalidf("%q is not a team key", v)
	}
	got, ok, err := s.repo.RosteredTeamFamily(ctx, k)
	if err != nil {
		return "", err
	}
	if !ok || (family != "" && got != family) {
		if family == "" {
			return "", invalidf("%q is not a Team Schedule team", v)
		}
		return "", invalidf("%q is not one of the Team Schedule's %s teams", v, family)
	}
	return k, nil
}

func ladderTeamItem(ctx context.Context, s *caseAlertSettingsService, ladder, v string) (string, error) {
	return teamItem(ctx, s, ladder, v)
}

func anyTeamItem(ctx context.Context, s *caseAlertSettingsService, _, v string) (string, error) {
	return teamItem(ctx, s, "", v)
}

func teamValue(family string) caseAlertValueCheck {
	return func(ctx context.Context, s *caseAlertSettingsService, _ string, raw json.RawMessage) (any, error) {
		v, err := stringValue(raw)
		if err != nil {
			return nil, err
		}
		return teamItem(ctx, s, family, v)
	}
}

// teamKeyFormatValue is a team key that need not exist: teams.leadership only
// matters when a head is not named, and production names both.
func teamKeyFormatValue(_ context.Context, _ *caseAlertSettingsService, _ string, raw json.RawMessage) (any, error) {
	v, err := stringValue(raw)
	if err != nil {
		return nil, err
	}
	k := strings.ToLower(v)
	if !caseAlertTeamKey.MatchString(k) {
		return nil, invalidf("%q is not a team key", v)
	}
	return k, nil
}

func abtTypeValue(ctx context.Context, s *caseAlertSettingsService, ladder string, raw json.RawMessage) (any, error) {
	v, err := stringValue(raw)
	if err != nil {
		return nil, err
	}
	t := strings.ToLower(v)
	if !strings.HasPrefix(t, strings.ToLower(ladder)) || !caseAlertTeamKey.MatchString(t) {
		return nil, invalidf("%q is not a %s team type; use one like %s-abt", v, ladder, strings.ToLower(ladder))
	}
	ok, err := s.repo.TeamTypeExists(ctx, t)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, invalidf("no team has type %q", v)
	}
	return t, nil
}

func rotaMembersValue(_ context.Context, _ *caseAlertSettingsService, _ string, raw json.RawMessage) (any, error) {
	var in map[string]int
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, invalidf("must map a shift to a whole number, like {\"LK_EVENING\": 7}")
	}
	out := make(map[string]int, len(in))
	for k, n := range in {
		shift := strings.ToUpper(strings.TrimSpace(k))
		if !slices.Contains(caseAlertShifts, shift) {
			return nil, invalidf("%q is not a shift; use %s", k, strings.Join(caseAlertShifts, ", "))
		}
		if _, dup := out[shift]; dup {
			return nil, invalidf("names shift %s twice", shift)
		}
		if n < 0 || n > 50 {
			return nil, invalidf("%s: must be a whole number from 0 to 50", shift)
		}
		out[shift] = n
	}
	return out, nil
}

func defaultRotaValue(ctx context.Context, s *caseAlertSettingsService, _ string, raw json.RawMessage) (any, error) {
	v, err := stringValue(raw)
	if err != nil {
		return nil, err
	}
	code := strings.ToUpper(v)
	family, ok, err := s.repo.ActiveRotaFamily(ctx, code)
	if err != nil {
		return nil, err
	}
	if !ok || family != familySRE {
		return nil, invalidf("%q is not an active SRE rota", v)
	}
	return code, nil
}

// emailAddress is one bare email address, as the rung email's lists are
// written: no display name, no second address.
func emailAddress(v string) (string, error) {
	if a, err := mail.ParseAddress(v); err != nil || a.Address != v || len(v) > 255 {
		return "", invalidf("%q is not a single email address", v)
	}
	return v, nil
}

// mailingListValue is one team mailing list address; empty clears it.
func mailingListValue(_ context.Context, _ *caseAlertSettingsService, _ string, raw json.RawMessage) (any, error) {
	v, err := stringValue(raw)
	if err != nil || v == "" {
		return v, err
	}
	return emailAddress(v)
}

// emailTeamListsValue is cre.email.teamLists: a CRE team key (an ABT or the
// Americas team) to that team's mailing list address.
func emailTeamListsValue(ctx context.Context, s *caseAlertSettingsService, _ string, raw json.RawMessage) (any, error) {
	var in map[string]string
	if err := json.Unmarshal(raw, &in); err != nil || in == nil {
		return nil, invalidf("send an object of CRE team key to mailing list address, like {\"atlas\": \"atlas-team@example.com\"}")
	}
	if len(in) > 30 {
		return nil, invalidf("at most 30 team lists")
	}
	// Every key and address is checked before any team is looked up, so bad
	// input is always refused as such, never as a failed lookup.
	checked := make(map[string]string, len(in))
	for team, addr := range in {
		k := strings.ToLower(strings.TrimSpace(team))
		if !caseAlertTeamKey.MatchString(k) {
			return nil, invalidf("%q is not a team key", team)
		}
		if _, dup := checked[k]; dup {
			return nil, invalidf("names team %s twice", k)
		}
		a, err := emailAddress(strings.TrimSpace(addr))
		if err != nil {
			return nil, err
		}
		checked[k] = a
	}
	out := make(map[string]string, len(checked))
	for k, a := range checked {
		key, err := teamItem(ctx, s, familyCRE, k)
		if err != nil {
			return nil, err
		}
		out[key] = a
	}
	return out, nil
}

// caseAlertHead is a head as saved: who, not how to reach them. Their number
// comes from their CSM Portal profile, as for everyone else the ladder calls.
type caseAlertHead struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func headValue(ctx context.Context, s *caseAlertSettingsService, _ string, raw json.RawMessage) (any, error) {
	var h caseAlertHead
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&h); err != nil {
		return nil, invalidf("must be a name and an email, like {\"name\": \"...\", \"email\": \"...\"}; a phone number is not saved here")
	}
	h.Name, h.Email = strings.TrimSpace(h.Name), strings.TrimSpace(h.Email)
	if h.Name == "" || len(h.Name) > maxCaseAlertLabel {
		return nil, invalidf("the name must be 1 to %d characters", maxCaseAlertLabel)
	}
	if a, err := mail.ParseAddress(h.Email); err != nil || a.Address != h.Email || len(h.Email) > 255 {
		return nil, invalidf("%q is not an email address", h.Email)
	}
	ok, err := s.repo.ActiveUserExists(ctx, h.Email)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, invalidf("%s is not an active CSM Portal user", h.Email)
	}
	return h, nil
}
