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
	"context"
	"strings"
	"testing"
)

// rotaCatalogue is the catalogue as migration 0200 shapes it: SaaS SRE
// (apollo, artemis) on TZ1, IaaS SRE (iaas) on its Day zone, three SME teams on
// theirs. withRotas false strips every rota code, as an entity-service from
// before rotas answers.
func rotaCatalogue(withRotas bool) *scheduleCatalogue {
	rota := func(code string) *string {
		if !withRotas {
			return nil
		}
		return optStr(code)
	}
	cat := &scheduleCatalogue{}
	for _, t := range []struct{ key, family, rota string }{
		{"apollo", "SRE", RotaSRESaaS}, {"artemis", "SRE", RotaSRESaaS}, {"iaas", "SRE", RotaSREIaaS},
		{"atlas", "CRE", ""},
		{"asgardeo", "SME", "SME_ASGARDEO"}, {"choreo-sme", "SME", "SME_CHOREO"}, {"bijira", "SME", "SME_BIJIRA"},
	} {
		cat.Teams = append(cat.Teams, catalogueTeam{Key: t.key, Name: t.key, Family: t.family, RotaCode: rota(t.rota)})
	}
	for _, z := range []struct{ code, rota string }{
		{"TZ1", RotaSRESaaS}, {"IAAS_D", RotaSREIaaS},
		{"ASG_D", "SME_ASGARDEO"}, {"CRT_D", "SME_CHOREO"}, {"BIJ_D", "SME_BIJIRA"},
	} {
		cat.Zones = append(cat.Zones, catalogueZone{Code: z.code, RotaCode: rota(z.rota)})
	}
	for _, sh := range []struct{ code, family, zone, tier string }{
		{"SRE_TZ1_L1", "SRE", "TZ1", "L1"},
		{"SRE_TZ1", "SRE", "TZ1", ""},
		{"SRE_IAAS_DAY", "SRE", "IAAS_D", ""},
		{"SME_ASG_DAY", "SME", "ASG_D", ""},
		{"SME_CRT_DAY", "SME", "CRT_D", ""},
		{"SME_BIJ_DAY", "SME", "BIJ_D", ""},
		{"CRE_MORNING", "CRE", "", ""},
	} {
		cat.Shifts = append(cat.Shifts, catalogueShift{Code: sh.code, Family: sh.family, ZoneCode: optStr(sh.zone), Tier: optStr(sh.tier)})
	}
	return cat
}

// rotaReader is 10:00 on a weekday with both SRE rotas and two SME teams on
// duty. SaaS has L1 and L2 but no L3; IaaS has L1 and L3 but no L2. The IaaS
// zone sorts ahead of TZ1, so a resolver that did not filter by rota would
// prefer the IaaS holder for any incident without a team of its own.
func rotaReader(withRotas bool) *stubScheduleReader {
	return &stubScheduleReader{
		catalogue: rotaCatalogue(withRotas),
		onDuty: []onDutyAssignment{
			held("a-l1", "apollo", "SRE_TZ1_L1", ""),
			held("a-l2", "apollo", "SRE_TZ1", "L2"),
			held("i-l1", "iaas", "SRE_IAAS_DAY", "L1"),
			held("i-l3", "iaas", "SRE_IAAS_DAY", "L3"),
			held("s-b", "asgardeo", "SME_ASG_DAY", ""),
			held("s-a", "asgardeo", "SME_ASG_DAY", ""),
			held("c-1", "choreo-sme", "SME_CRT_DAY", ""),
		},
	}
}

// rotaTeams is the SRE configuration with IaaS: three SRE teams, and IaaS's
// assignment group mapped to its key.
var rotaTeams = TeamKeys{
	SRE:     []string{"apollo", "artemis", "iaas"},
	Aliases: map[string]string{"SRE IaaS": "iaas"},
}

func rotaResolver(s *stubScheduleReader, defaultRota string) TeamScheduleResolver {
	teams := rotaTeams
	teams.DefaultRota = defaultRota
	return NewTeamScheduleResolver(s, teams, nil)
}

// A SaaS incident never reaches an IaaS engineer, and the other way round --
// not even on a tier the incident's own rota has nobody for.
func TestResolveSRE_RotasNeverCross(t *testing.T) {
	r := rotaResolver(rotaReader(true), "")
	cases := []struct {
		name, team string
		level      Level
		want       string
	}{
		{"SaaS L1", "Apollo", Level0, "a-l1@example.com"},
		{"SaaS L2", "Apollo", Level1, "a-l2@example.com"},
		{"SaaS L3: only IaaS has one", "Apollo", Level2, ""},
		{"IaaS L1", "SRE IaaS", Level0, "i-l1@example.com"},
		{"IaaS L2: only SaaS has one", "SRE IaaS", Level1, ""},
		{"IaaS L3", "SRE IaaS", Level2, "i-l3@example.com"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := strings.Join(resolveSRE(t, r, c.level, c.team), ",")
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// An incident with no SRE team of its own -- a CRE case at S0, a monitoring
// alert with no team -- pages sre.teams.defaultRota: SaaS unless set.
func TestResolveSRE_NoSRETeamPagesTheDefaultRota(t *testing.T) {
	for _, team := range []string{"Atlas", ""} {
		if got := resolveSRE(t, rotaResolver(rotaReader(true), ""), Level0, team); len(got) != 1 || got[0] != "a-l1@example.com" {
			t.Errorf("team %q, default rota: got %v, want the SaaS L1", team, got)
		}
		if got := resolveSRE(t, rotaResolver(rotaReader(true), "sre_iaas"), Level0, team); len(got) != 1 || got[0] != "i-l1@example.com" {
			t.Errorf("team %q, defaultRota SRE_IAAS: got %v, want the IaaS L1", team, got)
		}
	}
	rota, err := rotaResolver(rotaReader(true), "").SRERota(context.Background(), RoutingContext{AssignedCRETeam: "Atlas"})
	if err != nil || rota != RotaSRESaaS {
		t.Errorf("SRERota(Atlas) = %q, %v; want %s", rota, err, RotaSRESaaS)
	}
}

// A catalogue with no rota codes behaves exactly as before rotas: every SRE
// window counts, so the zone sorting first answers a team-less incident.
func TestResolveSRE_NoRotaCodesKeepsTheOldBehaviour(t *testing.T) {
	r := rotaResolver(rotaReader(false), "")
	if got := resolveSRE(t, r, Level0, "Atlas"); len(got) != 1 || got[0] != "i-l1@example.com" {
		t.Errorf("team-less L1 = %v; with no rotas the IaaS zone sorts first, as it always did", got)
	}
	// An SRE team's incident still reaches only its own team, rota codes or
	// not: the IaaS L3 is never Apollo's.
	if got := resolveSRE(t, r, Level2, "Apollo"); strings.Contains(strings.Join(got, ","), "i-l3") {
		t.Errorf("Apollo L3 = %v; another team's L3 must not answer an Apollo incident", got)
	}
	if rota, _ := r.SRERota(context.Background(), RoutingContext{AssignedCRETeam: "Apollo"}); rota != "" {
		t.Errorf("SRERota = %q; want empty with no rotas", rota)
	}
}

// The engine stamps the rota on the plan and names the chain wherever people
// read it: the card, the voice message, the work note.
func TestEngine_NamesTheSREChain(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct{ team, rota, chain, l1 string }{
		{"Apollo", RotaSRESaaS, "SaaS SRE", "a-l1"},
		{"SRE IaaS", RotaSREIaaS, "IaaS SRE", "i-l1"},
	} {
		t.Run(c.chain, func(t *testing.T) {
			chat, store, notes := &fakeChat{}, newMemStore(), &fakeNotes{}
			e := smeEngine(rotaReader(true), store, notes, chat, &fakeChat{}, SMEConfig{})
			if err := e.Handle(ctx, created(t, c.team, "HIGH")); err != nil {
				t.Fatal(err)
			}
			st, found, _ := store.Get(ctx, testIncidentID)
			if !found || st.Plan.Trigger.Routing.Rota != c.rota {
				t.Fatalf("found=%v rota=%q; want %s", found, st.Plan.Trigger.Routing.Rota, c.rota)
			}
			if err := e.Tick(ctx, testClock); err != nil {
				t.Fatal(err)
			}
			if len(chat.posted) != 1 || chat.posted[0].RungRole != c.chain+" L1 Support" || chat.posted[0].RecipientName != c.l1 {
				t.Fatalf("cards = %+v; want one %s L1 card for %s", chat.posted, c.chain, c.l1)
			}
			if !strings.Contains(st.Plan.Trigger.VoiceMessagePlain(), c.chain) {
				t.Errorf("voice message %q does not name the %s chain", st.Plan.Trigger.VoiceMessagePlain(), c.chain)
			}
			if err := e.Handle(ctx, assigned(t)); err != nil {
				t.Fatal(err)
			}
			if len(notes.notes) != 1 || !strings.Contains(notes.notes[0], "Paging chain: "+c.chain) {
				t.Errorf("work note = %v; want it to name the %s chain", notes.notes, c.chain)
			}
		})
	}
}

// U2 and B-Central's chains are named after their assignment groups, so a
// reader can tell where the incident came from; their rota codes keep the
// SME_ prefix 0217 gave them.
func TestSREChain_U2AndBCentral(t *testing.T) {
	for rota, want := range map[string]string{
		RotaSREU2:        "U2-SRE",
		RotaSREBCentral:  "B-Central-SRE",
		"sme_b_central ": "B-Central-SRE",
		RotaSRESaaS:      "SaaS SRE",
		RotaSREIaaS:      "IaaS SRE",
	} {
		if got := (RoutingContext{Rota: rota}).SREChain(); got != want {
			t.Errorf("SREChain(%q) = %q, want %q", rota, got, want)
		}
	}
	if got := rungRole(Level0, RoutingContext{Ladder: LadderSRE, Rota: RotaSREU2}); got != "U2-SRE L1 Support" {
		t.Errorf("rung role = %q, want U2-SRE L1 Support", got)
	}
}

// Without rotas nothing changes in what people read.
func TestEngine_UnknownRotaKeepsTheOldWording(t *testing.T) {
	tr := Trigger{Routing: RoutingContext{Ladder: LadderSRE}}
	if got := rungRole(Level0, tr.Routing); got != "L1 Support" {
		t.Errorf("rung role = %q", got)
	}
	if strings.Contains(tr.VoiceMessagePlain(), "chain") {
		t.Errorf("voice message %q names a chain with no rota", tr.VoiceMessagePlain())
	}
	if got := (RoutingContext{Rota: "SRE_PAAS"}).SREChain(); got != "SRE" {
		t.Errorf("an unnamed rota reads %q, want SRE", got)
	}
}

// sre.teams.defaultRota is an SRE setting with an SRE rota code.
func TestConfig_DefaultRota(t *testing.T) {
	for name, c := range map[string]struct {
		yaml string
		ok   bool
	}{
		"SaaS":            {"sre:\n  teams:\n    defaultRota: SRE_SAAS\n", true},
		"IaaS lower-case": {"sre:\n  teams:\n    defaultRota: sre_iaas\n", true},
		"not a rota":      {"sre:\n  teams:\n    defaultRota: apollo\n", false},
		"under cre":       {"cre:\n  teams:\n    defaultRota: SRE_SAAS\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := LoadConfig(writeConfig(t, c.yaml))
			if (err == nil) != c.ok {
				t.Errorf("err = %v, want ok=%v", err, c.ok)
			}
		})
	}
}
