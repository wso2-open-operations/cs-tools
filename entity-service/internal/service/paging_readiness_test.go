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
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Friday 9 October 2026, then the weekend: three days that exercise both
// day scopes.
var readinessFrom = time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)

var readinessDates = []string{"2026-10-09", "2026-10-10", "2026-10-11"}

func creMember(team, role, tier, name string) repository.PagingReadinessMember {
	return repository.PagingReadinessMember{TeamKey: team, Role: role, AlertTier: tier, Name: name, Email: strings.ToLower(name) + "@wso2.com"}
}

// readinessFixture is a whole organisation with a few holes in it, each of
// which the tests below expect to be reported exactly once.
func readinessFixture() repository.PagingReadinessFacts {
	f := repository.PagingReadinessFacts{
		CRETeams: []repository.PagingReadinessTeam{
			{Key: "americas", Name: "Americas", Type: "cre", IsAmericas: true},
			{Key: "orion", Name: "Orion", Type: "cre-abt"},
			{Key: "vega", Name: "Vega", Type: "cre-abt"},
		},
		CREMembers: []repository.PagingReadinessMember{
			creMember("cre-leadership", "cre_head", "", "Hana"),
			creMember("americas", "americas_team_lead", "", "Ava"),
			creMember("americas", "lead", "", "Amir"),
			creMember("americas", "engineer", "T1", "Ana"),
			creMember("americas", "engineer", "T2", "Abe"),
			creMember("americas", "engineer", "T3", "Ali"),
			// Orion: no lead, and only its first responder.
			creMember("orion", "engineer", "T1", "Oli"),
			creMember("orion", "engineer", "", "Ora"),
			creMember("vega", "lead", "", "Val"),
			creMember("vega", "engineer", "T1", "Vic"),
			creMember("vega", "engineer", "T2", "Viv"),
			creMember("vega", "engineer", "T3", "Vin"),
		},
		Rotas: []repository.PagingReadinessRota{
			{Code: "SRE_SAAS", Label: "SaaS", Family: "SRE"},
			{Code: "SME_ASGARDEO", Label: "Asgardeo", Family: "SME"},
		},
		Windows: []repository.PagingReadinessWindow{
			{ShiftCode: "SRE_TZ1_L1", ShiftLabel: "TZ1 L1 support", ZoneCode: "TZ1", ZoneLabel: "Time zone 1", ZoneSortOrder: 1, RotaCode: "SRE_SAAS", DayScope: "WEEKDAY", Tier: "L1"},
			{ShiftCode: "SRE_TZ1", ShiftLabel: "TZ1 escalation", ZoneCode: "TZ1", ZoneLabel: "Time zone 1", ZoneSortOrder: 1, RotaCode: "SRE_SAAS", DayScope: "WEEKDAY"},
			{ShiftCode: "SRE_TZ2", ShiftLabel: "TZ2 escalation", ZoneCode: "TZ2", ZoneLabel: "Time zone 2", ZoneSortOrder: 2, RotaCode: "SRE_SAAS", DayScope: "WEEKDAY"},
			{ShiftCode: "SRE_TZ3", ShiftLabel: "TZ3 escalation", ZoneCode: "TZ3", ZoneLabel: "Time zone 3", ZoneSortOrder: 3, RotaCode: "SRE_SAAS", DayScope: "ANY"},
			{ShiftCode: "SRE_WE_TZ1", ShiftLabel: "Weekend TZ1 + TZ2", ZoneCode: "TZ1", ZoneLabel: "Time zone 1", ZoneSortOrder: 1, RotaCode: "SRE_SAAS", DayScope: "WEEKEND"},
			{ShiftCode: "SME_ASG_DAY", ShiftLabel: "Asgardeo day escalation", ZoneCode: "ASG_D", ZoneLabel: "Asgardeo · Day", ZoneSortOrder: 31, RotaCode: "SME_ASGARDEO", DayScope: "ANY"},
			{ShiftCode: "SME_ASG_NIGHT", ShiftLabel: "Asgardeo night escalation", ZoneCode: "ASG_N", ZoneLabel: "Asgardeo · Night", ZoneSortOrder: 32, RotaCode: "SME_ASGARDEO", DayScope: "ANY"},
		},
		SMETeams: []repository.PagingReadinessTeam{
			{Key: "asgardeo", Name: "Asgardeo", Type: "sme-asgardeo", RotaCode: "SME_ASGARDEO"},
			// A team of SME type no active rota claims: not rostered, so not
			// checked, but still a team a handoff may name.
			{Key: "moesif", Name: "Moesif", Type: "sme-moesif"},
		},
		AccountCRETeams: []repository.PagingReadinessAccountTeam{
			{Name: "old team", Accounts: 3},
			{Name: "orion", Accounts: 1},
			{Name: "vega", Accounts: 10},
		},
	}

	// SaaS: every zone worked each day has an L1, an L2 and an L3, except
	// Friday's TZ2 L3 and Sunday's TZ3 L2.
	add := func(date, shift, zone, tier, name string) {
		f.Assignments = append(f.Assignments, repository.PagingReadinessAssignment{
			RotaDate: date, ShiftCode: shift, ZoneCode: zone, RotaCode: "SRE_SAAS", TeamKey: "apollo", Tier: tier,
			Name: name, Email: strings.ToLower(name) + "@wso2.com",
		})
	}
	for _, d := range readinessDates {
		weekend := d != "2026-10-09"
		tz1 := "SRE_TZ1"
		if weekend {
			tz1 = "SRE_WE_TZ1"
		}
		add(d, tz1, "TZ1", "L1", "Sam")
		add(d, tz1, "TZ1", "L2", "Sid")
		add(d, tz1, "TZ1", "L3", "Sol")
		if !(d == "2026-10-11") {
			add(d, "SRE_TZ3", "TZ3", "L2", "Sid")
		}
		add(d, "SRE_TZ3", "TZ3", "L1", "Sam")
		add(d, "SRE_TZ3", "TZ3", "L3", "Sol")
		if !weekend {
			add(d, "SRE_TZ2", "TZ2", "L1", "Sam")
			add(d, "SRE_TZ2", "TZ2", "L2", "Sid")
		}
	}
	// A Saturday TZ2 row: TZ2 is not worked at the weekend, so it neither
	// fills nor opens a gap.
	add("2026-10-10", "SRE_TZ2", "TZ2", "L3", "Sol")

	// Asgardeo: every window covered but Saturday night.
	for _, d := range readinessDates {
		f.Assignments = append(f.Assignments, repository.PagingReadinessAssignment{
			RotaDate: d, ShiftCode: "SME_ASG_DAY", ZoneCode: "ASG_D", RotaCode: "SME_ASGARDEO", TeamKey: "asgardeo", Tier: "L1", Name: "Ama", Email: "ama@wso2.com",
		})
		if d != "2026-10-10" {
			f.Assignments = append(f.Assignments, repository.PagingReadinessAssignment{
				RotaDate: d, ShiftCode: "SME_ASG_NIGHT", ZoneCode: "ASG_N", RotaCode: "SME_ASGARDEO", TeamKey: "asgardeo", Tier: "L1", Name: "Ash", Email: "ash@wso2.com",
			})
		}
	}
	return f
}

func readinessHandoff(t *testing.T) *SpecialistHandoffConfig {
	t.Helper()
	cfg, err := ParseSpecialistHandoffConfig(`{"products":[
	 {"name":"Asgardeo","serviceIds":["97ed1b8b-1ba2-6c10-00ae-86acdd4bcbd3"],
	  "teams":[{"key":"asgardeo-special-ops","label":"Asgardeo Special Ops","groupId":"7fb4f4c6-1b4b-3810-aea4-a936604bcb90","smeTeam":"asgardeo"}]},
	 {"name":"Choreo","serviceIds":["b9c999f8-1b86-a010-00ae-86acdd4bcb61"],
	  "teams":[{"key":"choreo-special-ops","label":"Choreo Special Ops","groupId":"fe0d8868-1b0b-3010-d64e-64a2604bcb3c"},
	           {"key":"choreo-runtime-team","label":"Choreo Runtime Team","groupId":"80dade5d-1b70-0710-a002-c9d3604bcbd7","smeTeam":"choreo-runtime"},
	           {"key":"moesif-team","label":"Moesif","groupId":"55555555-5555-4555-8555-555555555555","smeTeam":"moesif"}]}]}`)
	if err != nil {
		t.Fatalf("parse handoff config: %v", err)
	}
	return cfg
}

func chainOf(t *testing.T, resp domain.PagingReadinessResponse, chain string) domain.PagingReadinessChain {
	t.Helper()
	for _, c := range resp.Chains {
		if c.Chain == chain {
			return c
		}
	}
	t.Fatalf("no %s chain in %+v", chain, resp.Chains)
	return domain.PagingReadinessChain{}
}

// gk is a gap's key: every field but the message, which is wording for
// people.
func gk(code, severity, fix, teamKey, date, zone, tier, shift string) string {
	return strings.Join([]string{code, severity, fix, teamKey, date, zone, tier, shift}, "|")
}

func gapKeys(gaps []domain.PagingReadinessGap) []string {
	out := make([]string, 0, len(gaps))
	for _, g := range gaps {
		out = append(out, gk(g.Code, g.Severity, g.Fix, g.TeamKey, g.Date, g.ZoneCode, g.Tier, g.ShiftCode))
	}
	return out
}

func TestComputePagingReadiness_Envelope(t *testing.T) {
	now := time.Date(2026, 10, 8, 20, 0, 0, 0, time.UTC)
	resp := computePagingReadiness(readinessFixture(), readinessHandoff(t), readinessFrom, 3, now)
	if resp.From != "2026-10-09" || resp.To != "2026-10-11" || resp.GeneratedAt != "2026-10-08T20:00:00Z" {
		t.Errorf("envelope from=%s to=%s generatedAt=%s", resp.From, resp.To, resp.GeneratedAt)
	}
	var chains, labels []string
	for _, c := range resp.Chains {
		chains = append(chains, c.Chain)
		labels = append(labels, c.Label)
		if c.Gaps == nil || c.People == nil {
			t.Errorf("%s: gaps and people must be lists, never null", c.Chain)
		}
		for _, g := range c.Gaps {
			if g.Message == "" || !strings.HasSuffix(g.Message, ".") {
				t.Errorf("%s %s: message %q is not one sentence", c.Chain, g.Code, g.Message)
			}
		}
	}
	if !reflect.DeepEqual(chains, []string{"CRE", "SRE_SAAS", "SRE_IAAS", "SME"}) ||
		!reflect.DeepEqual(labels, []string{"CRE chain", "SaaS SRE chain", "IaaS SRE chain", "SME chain"}) {
		t.Errorf("chains %v labels %v", chains, labels)
	}
}

func TestComputePagingReadiness_CRE(t *testing.T) {
	c := chainOf(t, computePagingReadiness(readinessFixture(), nil, readinessFrom, 3, time.Now()), "CRE")
	want := []string{
		gk("MISSING_HEAD", "error", "responders", "", "", "", "", ""),
		gk("MISSING_TEAM_LEAD", "error", "responders", "orion", "", "", "", ""),
		gk("MISSING_RESPONDER", "error", "responders", "orion", "", "", "T2", ""),
		gk("MISSING_RESPONDER", "error", "responders", "orion", "", "", "T3", ""),
		gk("ACCOUNT_TEAM_NOT_ABT", "warning", "data", "", "", "", "", ""),
	}
	if got := gapKeys(c.Gaps); !reflect.DeepEqual(got, want) {
		t.Errorf("gaps\n got %v\nwant %v", got, want)
	}
	if c.Ready {
		t.Error("ready with errors")
	}
	if !strings.Contains(c.Gaps[0].Message, "CS head") {
		t.Errorf("MISSING_HEAD names %q, want the CS head", c.Gaps[0].Message)
	}
	if msg := c.Gaps[4].Message; !strings.Contains(msg, `"old team"`) || !strings.HasPrefix(msg, "3 accounts") {
		t.Errorf("account warning %q", msg)
	}

	roles := map[string]string{}
	for _, p := range c.People {
		roles[p.Role] = p.Email
	}
	for role, email := range map[string]string{
		"CRE head":                "hana@wso2.com",
		"America lead":            "ava@wso2.com",
		"Team lead · Americas":    "amir@wso2.com",
		"T3 responder · Americas": "ali@wso2.com",
		"T1 responder · Orion":    "oli@wso2.com",
		"Team lead · Vega":        "val@wso2.com",
		"T2 responder · Vega":     "viv@wso2.com",
	} {
		if roles[role] != email {
			t.Errorf("people[%q] = %q, want %q", role, roles[role], email)
		}
	}
	for _, p := range c.People {
		if p.Email == "ora@wso2.com" {
			t.Error("an engineer holding no position is not on the chain")
		}
	}
}

// TestComputePagingReadiness_PagingTiers pins the tier each position is first
// called at, and which numbers may be missing: only the 2nd and 3rd responders'
// (called together with the 1st); from the 1st responder up, a number is a must.
func TestComputePagingReadiness_PagingTiers(t *testing.T) {
	resp := computePagingReadiness(readinessFixture(), readinessHandoff(t), readinessFrom, 3, time.Now())
	type want struct {
		tier     int
		optional bool
	}
	got := map[string]want{}
	for _, p := range chainOf(t, resp, "CRE").People {
		got[p.Role] = want{p.PagingTier, p.PhoneOptional}
	}
	for role, w := range map[string]want{
		"T1 responder · Orion":    {1, false},
		"T2 responder · Vega":     {1, true},
		"T3 responder · Americas": {1, true},
		"Team lead · Vega":        {2, false},
		"Team lead · Americas":    {2, false},
		"America lead":            {3, false},
		"CRE head":                {4, false},
	} {
		if got[role] != w {
			t.Errorf("%s: tier/optional = %+v, want %+v", role, got[role], w)
		}
	}

	sreTier := map[string]int{}
	for _, p := range chainOf(t, resp, "SRE_SAAS").People {
		if p.PhoneOptional {
			t.Errorf("SRE %s: optional number, but every SRE tier is one person", p.Email)
		}
		sreTier[p.Email] = p.PagingTier
	}
	if !reflect.DeepEqual(sreTier, map[string]int{"sam@wso2.com": 1, "sid@wso2.com": 2, "sol@wso2.com": 3}) {
		t.Errorf("SRE tiers %v", sreTier)
	}
	for _, p := range chainOf(t, resp, "SME").People {
		if p.PagingTier != 1 || p.PhoneOptional {
			t.Errorf("SME %s: tier %d optional %v, want 1 and required", p.Email, p.PagingTier, p.PhoneOptional)
		}
	}
}

func TestComputePagingReadiness_CREWithoutAmericas(t *testing.T) {
	f := readinessFixture()
	f.CRETeams = f.CRETeams[1:]
	c := chainOf(t, computePagingReadiness(f, nil, readinessFrom, 1, time.Now()), "CRE")
	found := false
	for _, g := range c.Gaps {
		if g.Code == "MISSING_AMERICAS_LEAD" {
			found = g.Fix == "data" && g.TeamKey == ""
		}
	}
	if !found {
		t.Errorf("no Americas team must be one MISSING_AMERICAS_LEAD fixed in the data, got %v", gapKeys(c.Gaps))
	}

	f = readinessFixture()
	f.CREMembers = f.CREMembers[:1] // only the CRE head is left
	f.CREMembers = append(f.CREMembers, creMember("cre-leadership", "cs_head", "", "Cyd"))
	c = chainOf(t, computePagingReadiness(f, nil, readinessFrom, 1, time.Now()), "CRE")
	codes := map[string]int{}
	for _, g := range c.Gaps {
		codes[g.Code]++
	}
	if codes["MISSING_HEAD"] != 0 || codes["MISSING_AMERICAS_LEAD"] != 1 || codes["MISSING_TEAM_LEAD"] != 2 || codes["MISSING_RESPONDER"] != 9 {
		t.Errorf("gap counts %v", codes)
	}
}

func TestComputePagingReadiness_SRE(t *testing.T) {
	resp := computePagingReadiness(readinessFixture(), nil, readinessFrom, 3, time.Now())
	saas := chainOf(t, resp, "SRE_SAAS")
	want := []string{
		gk("TIER_UNCOVERED", "error", "rota", "", "2026-10-09", "TZ2", "L3", ""),
		gk("TIER_UNCOVERED", "error", "rota", "", "2026-10-11", "TZ3", "L2", ""),
	}
	if got := gapKeys(saas.Gaps); !reflect.DeepEqual(got, want) {
		t.Errorf("SaaS gaps\n got %v\nwant %v", got, want)
	}
	if saas.Ready {
		t.Error("SaaS ready with uncovered tiers")
	}
	people := map[string]string{}
	for _, p := range saas.People {
		people[p.Email] = p.Role
	}
	if !reflect.DeepEqual(people, map[string]string{"sam@wso2.com": "L1", "sid@wso2.com": "L2", "sol@wso2.com": "L3"}) {
		t.Errorf("SaaS people %v", people)
	}

	iaas := chainOf(t, resp, "SRE_IAAS")
	if got := gapKeys(iaas.Gaps); !reflect.DeepEqual(got, []string{gk("ROTA_INACTIVE", "error", "config", "", "", "", "", "")}) || iaas.Ready {
		t.Errorf("IaaS with no active windows: %v ready=%v", got, iaas.Ready)
	}
}

func TestComputePagingReadiness_SREInactiveWindowsOnly(t *testing.T) {
	// The rota exists but none of its windows is active (IaaS as 0200
	// seeds it): still one ROTA_INACTIVE, not a gap per day.
	f := readinessFixture()
	f.Rotas = append(f.Rotas, repository.PagingReadinessRota{Code: "SRE_IAAS", Label: "IaaS", Family: "SRE"})
	iaas := chainOf(t, computePagingReadiness(f, nil, readinessFrom, 7, time.Now()), "SRE_IAAS")
	if len(iaas.Gaps) != 1 || iaas.Gaps[0].Code != "ROTA_INACTIVE" {
		t.Errorf("gaps %v", gapKeys(iaas.Gaps))
	}
}

// U2 and B-Central get a chain of their own, named after their assignment
// groups, once migration 0242 has made their rotas SRE rotas -- and none while
// the rotas are still SME ones, so nothing is shown twice before it runs.
func TestComputePagingReadiness_U2AndBCentralChains(t *testing.T) {
	chainsOf := func(resp domain.PagingReadinessResponse) []string {
		var out []string
		for _, c := range resp.Chains {
			out = append(out, c.Chain+"="+c.Label)
		}
		return out
	}

	before := readinessFixture()
	before.Rotas = append(before.Rotas,
		repository.PagingReadinessRota{Code: "SME_U2", Label: "U2", Family: "SME"},
		repository.PagingReadinessRota{Code: "SME_B_CENTRAL", Label: "B-Central", Family: "SME"})
	if got := chainsOf(computePagingReadiness(before, nil, readinessFrom, 3, time.Now())); !reflect.DeepEqual(got,
		[]string{"CRE=CRE chain", "SRE_SAAS=SaaS SRE chain", "SRE_IAAS=IaaS SRE chain", "SME=SME chain"}) {
		t.Errorf("SME rotas: chains %v", got)
	}

	after := readinessFixture()
	after.Rotas = append(after.Rotas,
		repository.PagingReadinessRota{Code: "SME_U2", Label: "U2", Family: "SRE"},
		repository.PagingReadinessRota{Code: "SME_B_CENTRAL", Label: "B-Central", Family: "SRE"})
	after.Windows = append(after.Windows, repository.PagingReadinessWindow{
		ShiftCode: "SME_U2_DAY", ShiftLabel: "U2 day escalation", ZoneCode: "U2_D", ZoneLabel: "U2 · Day", ZoneSortOrder: 101, RotaCode: "SME_U2", DayScope: "ANY",
	})
	for _, d := range readinessDates[:1] {
		for _, tier := range []string{"L1", "L2", "L3"} {
			after.Assignments = append(after.Assignments, repository.PagingReadinessAssignment{
				RotaDate: d, ShiftCode: "SME_U2_DAY", ZoneCode: "U2_D", RotaCode: "SME_U2", TeamKey: "u2", Tier: tier,
				Name: "U" + tier, Email: "u" + strings.ToLower(tier) + "@wso2.com",
			})
		}
	}
	resp := computePagingReadiness(after, nil, readinessFrom, 1, time.Now())
	if got := chainsOf(resp); !reflect.DeepEqual(got, []string{
		"CRE=CRE chain", "SRE_SAAS=SaaS SRE chain", "SRE_IAAS=IaaS SRE chain",
		"SRE_U2=U2-SRE chain", "SRE_B_CENTRAL=B-Central-SRE chain", "SME=SME chain",
	}) {
		t.Errorf("SRE rotas: chains %v", got)
	}
	if u2 := chainOf(t, resp, "SRE_U2"); !u2.Ready || len(u2.People) != 3 {
		t.Errorf("U2-SRE with L1-L3 rostered: ready=%v gaps=%v people=%d", u2.Ready, gapKeys(u2.Gaps), len(u2.People))
	}
	if bc := chainOf(t, resp, "SRE_B_CENTRAL"); bc.Ready || len(bc.Gaps) != 1 || bc.Gaps[0].Code != "ROTA_INACTIVE" {
		t.Errorf("B-Central-SRE with no windows: ready=%v gaps=%v", bc.Ready, gapKeys(bc.Gaps))
	}
}

func TestComputePagingReadiness_SME(t *testing.T) {
	c := chainOf(t, computePagingReadiness(readinessFixture(), readinessHandoff(t), readinessFrom, 3, time.Now()), "SME")
	// Each Day and Night window needs an L1, an L2 and an L3: only L1s are
	// rostered, and nobody on the night of the 10th. A missing L1 is an
	// error; a missing L2 or L3 a warning.
	var want []string
	for _, day := range []string{"2026-10-09", "2026-10-10", "2026-10-11"} {
		for _, shift := range []string{"SME_ASG_DAY", "SME_ASG_NIGHT"} {
			if day == "2026-10-10" && shift == "SME_ASG_NIGHT" {
				want = append(want, gk("TIER_UNCOVERED", "error", "rota", "asgardeo", day, "", "L1", shift))
			}
			want = append(want,
				gk("TIER_UNCOVERED", "warning", "rota", "asgardeo", day, "", "L2", shift),
				gk("TIER_UNCOVERED", "warning", "rota", "asgardeo", day, "", "L3", shift))
		}
	}
	want = append(want,
		gk("HANDOFF_TEAM_UNMAPPED", "error", "config", "", "", "", "", ""),
		gk("HANDOFF_TEAM_UNMAPPED", "error", "config", "choreo-runtime", "", "", "", ""))
	if got := gapKeys(c.Gaps); !reflect.DeepEqual(got, want) {
		t.Errorf("gaps\n got %v\nwant %v", got, want)
	}
	if c.Ready {
		t.Error("ready with an L1 missing")
	}
	if m := c.Gaps[len(c.Gaps)-2].Message; !strings.Contains(m, "choreo-special-ops") {
		t.Errorf("unmapped team message %q does not name the dialog team", m)
	}
	var emails []string
	for _, p := range c.People {
		emails = append(emails, p.Email+"="+p.Role)
	}
	sort.Strings(emails)
	if !reflect.DeepEqual(emails, []string{"ama@wso2.com=Asgardeo L1", "ash@wso2.com=Asgardeo L1"}) {
		t.Errorf("SME people %v", emails)
	}
}

func TestComputePagingReadiness_ReadyWhenNothingIsMissing(t *testing.T) {
	f := readinessFixture()
	f.CREMembers = append(f.CREMembers,
		creMember("cre-leadership", "cs_head", "", "Cyd"),
		creMember("orion", "lead", "", "Oto"),
		creMember("orion", "engineer", "T2", "Ole"),
		creMember("orion", "engineer", "T3", "Oma"),
	)
	f.Assignments = append(f.Assignments,
		repository.PagingReadinessAssignment{RotaDate: "2026-10-09", ShiftCode: "SRE_TZ2", ZoneCode: "TZ2", RotaCode: "SRE_SAAS", Tier: "L3", Email: "sol@wso2.com"},
		repository.PagingReadinessAssignment{RotaDate: "2026-10-11", ShiftCode: "SRE_TZ3", ZoneCode: "TZ3", RotaCode: "SRE_SAAS", Tier: "L2", Email: "sid@wso2.com"},
		repository.PagingReadinessAssignment{RotaDate: "2026-10-10", ShiftCode: "SME_ASG_NIGHT", ZoneCode: "ASG_N", RotaCode: "SME_ASGARDEO", TeamKey: "ASGARDEO", Email: "ash@wso2.com"},
	)
	resp := computePagingReadiness(f, nil, readinessFrom, 3, time.Now())
	for _, chain := range []string{"CRE", "SRE_SAAS", "SME"} {
		c := chainOf(t, resp, chain)
		if !c.Ready {
			t.Errorf("%s not ready: %v", chain, gapKeys(c.Gaps))
		}
	}
	// The account warning is still there, and does not make CRE unready.
	if cre := chainOf(t, resp, "CRE"); len(cre.Gaps) != 1 || cre.Gaps[0].Severity != "warning" {
		t.Errorf("CRE gaps %v", gapKeys(cre.Gaps))
	}
}

// readinessRepo records the range the service asks for.
type readinessRepo struct {
	*stubPagingRepo
	from, to string
	facts    repository.PagingReadinessFacts
}

func (r *readinessRepo) ReadinessFacts(_ context.Context, from, to string) (repository.PagingReadinessFacts, error) {
	r.from, r.to = from, to
	return r.facts, nil
}

type deniedAccess struct{}

func (deniedAccess) ResolveScope(context.Context) (AccessScope, error) { return AccessScope{}, nil }

func TestGetPagingReadiness_Days(t *testing.T) {
	for _, tc := range []struct {
		days, from, to string
	}{
		// 20:00 UTC on the 8th is already the 9th in the rota's clock.
		{"", "2026-10-09", "2026-10-15"},
		{"1", "2026-10-09", "2026-10-09"},
		{" 14 ", "2026-10-09", "2026-10-22"},
	} {
		repo := &readinessRepo{stubPagingRepo: pagingFixture()}
		svc := NewPagingChainService(repo, nil, alwaysUnrestrictedAccess{}, nil, nil).(*pagingChainService)
		svc.now = func() time.Time { return time.Date(2026, 10, 8, 20, 0, 0, 0, time.UTC) }
		resp, err := svc.GetPagingReadiness(context.Background(), tc.days)
		if err != nil {
			t.Fatalf("days=%q: %v", tc.days, err)
		}
		if repo.from != tc.from || repo.to != tc.to || resp.From != tc.from || resp.To != tc.to {
			t.Errorf("days=%q: asked %s..%s, answered %s..%s, want %s..%s", tc.days, repo.from, repo.to, resp.From, resp.To, tc.from, tc.to)
		}
	}

	for _, days := range []string{"0", "15", "-1", "seven", "7.5"} {
		repo := &readinessRepo{stubPagingRepo: pagingFixture()}
		_, err := NewPagingChainService(repo, nil, alwaysUnrestrictedAccess{}, nil, nil).GetPagingReadiness(context.Background(), days)
		var invalid *apierror.ValidationError
		if !errors.As(err, &invalid) || repo.from != "" {
			t.Errorf("days=%q: want ValidationError and no read, got %v", days, err)
		}
	}
}

func TestGetPagingReadiness_InternalCallersOnly(t *testing.T) {
	repo := &readinessRepo{stubPagingRepo: pagingFixture()}
	_, err := NewPagingChainService(repo, nil, deniedAccess{}, nil, nil).GetPagingReadiness(context.Background(), "")
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) || repo.from != "" {
		t.Fatalf("want ForbiddenError and no read, got %v", err)
	}
}

func TestComputePagingReadiness_OnLeave(t *testing.T) {
	f := readinessFixture()
	for i := range f.CREMembers {
		f.CREMembers[i].UserID = "u-" + f.CREMembers[i].Name
	}
	f.CREAbsences = []repository.PagingReadinessAbsence{
		// Vic, Vega's T1: two absences back to back, one run of days.
		{UserID: "u-Vic", StartsOn: "2026-10-01", EndsOn: "2026-10-09", Label: "Annual leave"},
		{UserID: "u-Vic", StartsOn: "2026-10-10", EndsOn: "2026-10-10", Label: "Lieu leave"},
		// Val, Vega's lead: open-ended, from the last day.
		{UserID: "u-Val", StartsOn: "2026-10-11", Label: "Annual leave"},
		// Hana, the CRE head: two separate runs.
		{UserID: "u-Hana", StartsOn: "2026-10-09", EndsOn: "2026-10-09", Label: "Sick leave"},
		{UserID: "u-Hana", StartsOn: "2026-10-11", EndsOn: "2026-10-20", Label: "Annual leave"},
		// Ana, the Americas T1, is on a span that moves her TO Americas: not away.
		{UserID: "u-Ana", StartsOn: "2026-10-01", EndsOn: "2026-12-31", Label: "Brazil rotation", MovesToTeam: "americas"},
		// Ora holds no position; her leave is no gap.
		{UserID: "u-Ora", StartsOn: "2026-10-09", EndsOn: "2026-10-11", Label: "Annual leave"},
		// Outside the range.
		{UserID: "u-Viv", StartsOn: "2026-10-12", EndsOn: "2026-10-14", Label: "Annual leave"},
	}
	c := chainOf(t, computePagingReadiness(f, nil, readinessFrom, 3, time.Now()), "CRE")
	var got []string
	var msgs []string
	for _, g := range c.Gaps {
		if g.Code == "ON_LEAVE" {
			got = append(got, gk(g.Code, g.Severity, g.Fix, g.TeamKey, g.Date, g.ZoneCode, g.Tier, g.ShiftCode))
			msgs = append(msgs, g.Message)
		}
	}
	want := []string{
		gk("ON_LEAVE", "warning", "responders", "cre-leadership", "2026-10-09", "", "", ""),
		gk("ON_LEAVE", "warning", "responders", "cre-leadership", "2026-10-11", "", "", ""),
		gk("ON_LEAVE", "warning", "responders", "vega", "2026-10-11", "", "", ""),
		gk("ON_LEAVE", "error", "responders", "vega", "2026-10-09", "", "", ""),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ON_LEAVE gaps\n got %v\nwant %v", got, want)
	}
	if msgs[3] != "Vic, T1 responder · Vega, is away from 2026-10-09 to 2026-10-10 (Annual leave, Lieu leave)." {
		t.Errorf("responder message %q", msgs[3])
	}
	if msgs[0] != "Hana, CRE head, is away on 2026-10-09 (Sick leave)." {
		t.Errorf("head message %q", msgs[0])
	}
}

func TestComputePagingReadiness_PeoplePagingPhones(t *testing.T) {
	f := readinessFixture()
	for i := range f.CREMembers {
		f.CREMembers[i].UserID = "u-" + f.CREMembers[i].Name
	}
	for i := range f.Assignments {
		f.Assignments[i].UserID = "u-" + f.Assignments[i].Name
	}
	f.PagingPhones = map[string]string{"u-Vic": "completed", "u-Sam": "", "u-Ama": "no-answer"}
	f.ProfilePhones = map[string]bool{"u-Val": true, "u-Vic": true}
	resp := computePagingReadiness(f, nil, readinessFrom, 3, time.Now())

	type view struct {
		has     bool
		status  string
		profile bool
	}
	seen := map[string]view{}
	for _, c := range resp.Chains {
		for _, p := range c.People {
			if p.UserID == "" {
				t.Errorf("%s: %s has no userId", c.Chain, p.Email)
			}
			v := view{has: p.HasPagingPhone, profile: p.HasProfilePhone}
			if p.PagingPhoneLastTestStatus != nil {
				v.status = *p.PagingPhoneLastTestStatus
			}
			seen[p.UserID] = v
		}
	}
	for id, want := range map[string]view{
		"u-Vic": {true, "completed", true},  // CRE responder, both numbers
		"u-Sam": {true, "", false},          // SRE engineer, never tested
		"u-Ama": {true, "no-answer", false}, // SME engineer
		"u-Val": {false, "", true},          // a profile number only
	} {
		if seen[id] != want {
			t.Errorf("%s: %+v, want %+v", id, seen[id], want)
		}
	}
	raw, _ := json.Marshal(resp.Chains[0].People[0])
	for _, field := range []string{`"userId"`, `"hasPagingPhone"`, `"hasProfilePhone"`} {
		if !strings.Contains(string(raw), field) {
			t.Errorf("person %s lacks %s", raw, field)
		}
	}
}
