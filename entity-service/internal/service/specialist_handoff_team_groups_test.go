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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	tgU2Service      = "02939644-4730-4790-5878-b2c4116d4355"
	tgCloudService   = "1cf18e12-3b5b-8710-3e1e-088aa4e45ac0"
	tgU2Team         = "0b7a1c2d-3e4f-4a5b-8c6d-7e8f9a0b1c2d"
	tgCloudAgentTeam = "1c8b2d3e-4f5a-4b6c-9d7e-8f9a0b1c2d3e"
)

// tgConfigJSON mixes the two kinds of team: Choreo's by groupId, and U2's
// and WSO2 Cloud's by Team Schedule key alone.
const tgConfigJSON = `{"products":[
 {"name":"Choreo","serviceIds":["b9c999f8-1b86-a010-00ae-86acdd4bcb61"],
  "teams":[{"key":"choreo-runtime","label":"Choreo Runtime","groupId":"80dade5d-1b70-0710-a002-c9d3604bcbd7","smeTeam":"choreo-runtime"}]},
 {"name":"U2","serviceIds":["02939644-4730-4790-5878-b2c4116d4355"],"teams":[{"key":"u2"}]},
 {"name":"WSO2 Cloud","serviceIds":["1cf18e12-3b5b-8710-3e1e-088aa4e45ac0"],
  "teams":[{"key":"cloud-agent","label":"WSO2 Cloud · Agent platform"},{"key":"cloud-core","smeTeam":"cloud-core"}]}]}`

// fakeTeamGroups is the team table: key -> group.
type fakeTeamGroups struct {
	teams map[string]repository.SpecialistHandoffTeamGroup
	err   error
	asked [][]string
}

func (f *fakeTeamGroups) EnsureTeamGroups(_ context.Context, keys []string) (map[string]repository.SpecialistHandoffTeamGroup, error) {
	f.asked = append(f.asked, append([]string(nil), keys...))
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]repository.SpecialistHandoffTeamGroup{}
	for _, k := range keys {
		if g, ok := f.teams[strings.ToLower(k)]; ok {
			out[strings.ToLower(k)] = g
		}
	}
	return out, nil
}

func tgConfig(t *testing.T) *SpecialistHandoffConfig {
	t.Helper()
	cfg, err := ParseSpecialistHandoffConfig(tgConfigJSON)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	return cfg
}

// A team with no groupId needs no label and pages the SME team its key
// names; its key must then be a Team Schedule key, and a different smeTeam
// is refused.
func TestParseSpecialistHandoffConfig_TeamWithoutGroup(t *testing.T) {
	cfg := tgConfig(t)
	u2 := cfg.Products[1].Teams[0]
	if !u2.fromTeam || u2.GroupID != "" || u2.Label != "" || u2.SMETeam != "u2" {
		t.Errorf("u2 team %+v, want from the team table, no group or label yet, smeTeam u2", u2)
	}
	if choreo := cfg.Products[0].Teams[0]; choreo.fromTeam {
		t.Errorf("a team with a groupId reads as from the team table: %+v", choreo)
	}
	product := func(team string) string {
		return `{"products":[{"name":"X","serviceIds":["` + tgU2Service + `"],"teams":[` + team + `]}]}`
	}
	for name, raw := range map[string]string{
		"key not a team key":    product(`{"key":"Choreo Runtime"}`),
		"smeTeam is not key":    product(`{"key":"u2","smeTeam":"b-central"}`),
		"groupId without label": product(`{"key":"u2","groupId":"` + tgU2Team + `"}`),
	} {
		if _, err := ParseSpecialistHandoffConfig(raw); err == nil || !strings.Contains(err.Error(), "SPECIALIST_HANDOFF_CONFIG") {
			t.Errorf("%s: err %v, want a SPECIALIST_HANDOFF_CONFIG error", name, err)
		}
	}
}

// ResolveTeamGroups asks only for the teams without a groupId, takes each
// team's id as its group and its name as an empty label, keeps a configured
// label, and reports the keys no team holds -- which a second call retries.
func TestSpecialistHandoffConfig_ResolveTeamGroups(t *testing.T) {
	cfg := tgConfig(t)
	repo := &fakeTeamGroups{teams: map[string]repository.SpecialistHandoffTeamGroup{
		"u2":          {ID: strings.ToUpper(tgU2Team), Name: "U2"},
		"cloud-agent": {ID: tgCloudAgentTeam, Name: "WSO2 Cloud · Agent platform (team)"},
	}}
	missing, err := cfg.ResolveTeamGroups(context.Background(), repo)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if want := [][]string{{"u2", "cloud-agent", "cloud-core"}}; !reflect.DeepEqual(repo.asked, want) {
		t.Errorf("asked for %v, want %v (never the team with a groupId)", repo.asked, want)
	}
	if !reflect.DeepEqual(missing, []string{"cloud-core"}) {
		t.Errorf("missing %v, want [cloud-core]", missing)
	}
	u2, agent, core := cfg.Products[1].Teams[0], cfg.Products[2].Teams[0], cfg.Products[2].Teams[1]
	if u2.GroupID != tgU2Team || u2.Label != "U2" {
		t.Errorf("u2 %+v, want the team's id (lower-cased) and name", u2)
	}
	if agent.GroupID != tgCloudAgentTeam || agent.Label != "WSO2 Cloud · Agent platform" {
		t.Errorf("cloud-agent %+v, want the team's id and the configured label", agent)
	}
	if core.GroupID != "" || core.Label != "cloud-core" {
		t.Errorf("cloud-core %+v, want no group and its key as the label", core)
	}

	repo.teams["cloud-core"] = repository.SpecialistHandoffTeamGroup{ID: tgU2Team[:35] + "e", Name: "WSO2 Cloud · Core"}
	if missing, err = cfg.ResolveTeamGroups(context.Background(), repo); err != nil || len(missing) != 0 {
		t.Fatalf("second resolve: %v %v, want everything resolved", missing, err)
	}
	if got := repo.asked[1]; !reflect.DeepEqual(got, []string{"cloud-core"}) {
		t.Errorf("second call asked for %v, want only the team still missing", got)
	}
	if core = cfg.Products[2].Teams[1]; core.GroupID == "" || core.Label != "cloud-core" {
		t.Errorf("cloud-core %+v, want its group, keeping the label it already shows", core)
	}

	failing := tgConfig(t)
	if missing, err := failing.ResolveTeamGroups(context.Background(), &fakeTeamGroups{err: errors.New("db down")}); err == nil || len(missing) != 3 {
		t.Errorf("repository error: %v %v, want the error and all three keys", missing, err)
	}
}

// A resolved team routes like a configured group: the incident and its task
// go to the team's group, and an incident already there is not offered
// again. An unresolved team refuses the handoff with a 409 naming its key.
func TestPlanSpecialistHandoff_TeamWithoutGroup(t *testing.T) {
	cfg := tgConfig(t)
	if _, err := cfg.ResolveTeamGroups(context.Background(), &fakeTeamGroups{teams: map[string]repository.SpecialistHandoffTeamGroup{
		"u2": {ID: tgU2Team, Name: "U2"}, "cloud-agent": {ID: tgCloudAgentTeam, Name: "Cloud Agent"},
	}}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	req := func(team string) domain.HandOffIncidentToSpecialistRequest {
		r := domain.HandOffIncidentToSpecialistRequest{IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook}
		if team != "" {
			r.EscalationTeam = hoTeam(domain.IncidentSpecialistHandoffEscalationTeam(team))
		}
		return r
	}

	plan, product, _, err := planSpecialistHandoff(cfg, req(""), hoSnapshot(tgU2Service, "", "IN_PROGRESS"))
	if err != nil || product.Name != "U2" || plan.GroupID != tgU2Team || plan.TaskGroupID == nil || *plan.TaskGroupID != tgU2Team {
		t.Fatalf("u2 plan %+v product %v err %v, want the incident and its task in the U2 team's group", plan, product, err)
	}
	if plan, _, _, err = planSpecialistHandoff(cfg, req("cloud-agent"), hoSnapshot(tgCloudService, "", "IN_PROGRESS")); err != nil || plan.GroupID != tgCloudAgentTeam {
		t.Fatalf("cloud-agent plan %+v err %v, want the cloud-agent team's group", plan, err)
	}

	_, _, _, err = planSpecialistHandoff(cfg, req("cloud-core"), hoSnapshot(tgCloudService, "", "IN_PROGRESS"))
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) || !strings.Contains(ce.Msg, "cloud-core") {
		t.Errorf("unresolved cloud-core: %v, want a 409 naming the key", err)
	}
	if _, _, _, err = planSpecialistHandoff(cfg, req(""), hoSnapshot(tgU2Service, tgU2Team, "IN_PROGRESS")); !errors.As(err, &ce) {
		t.Errorf("already in the U2 group: %v, want a 409", err)
	}

	// An incident with no group is never "already with" the unresolved team.
	if cfg.Products[2].holdsGroup(hoStr("")) {
		t.Error("an empty group reads as the unresolved team's")
	}
}

// The Special Ops alert maps a group back to its team the same way: a
// resolved team's group raises it, and the unresolved team matches nothing.
func TestSpecialistHandoffConfig_TeamsForGroup_TeamWithoutGroup(t *testing.T) {
	cfg := tgConfig(t)
	if p, teams := cfg.teamsForGroup(tgU2Team); p != nil || teams != nil {
		t.Fatalf("before resolving: %v %v, want no team", p, teams)
	}
	if _, err := cfg.ResolveTeamGroups(context.Background(), &fakeTeamGroups{teams: map[string]repository.SpecialistHandoffTeamGroup{
		"u2": {ID: tgU2Team, Name: "U2"},
	}}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	p, teams := cfg.teamsForGroup(strings.ToUpper(tgU2Team))
	if p == nil || p.Name != "U2" || len(teams) != 1 || teams[0].Key != "u2" || teams[0].SMETeam != "u2" {
		t.Errorf("u2 group: %v %v, want the U2 product's u2 team paging SME team u2", p, teams)
	}
}

// The alert drainer's copy of the configuration resolves a team configured
// without a groupId when a group change first needs it, so a handoff the
// router's copy took always raises its alert: a failed lookup is returned
// (the change is retried, not dropped), and once every team is resolved an
// unrelated group change looks nothing up.
func TestSpecialOpsAlert_ResolvesTeamWithoutGroupOnDemand(t *testing.T) {
	ctx := context.Background()
	pub := &alertPublisher{}
	repo := &fakeTeamGroups{err: errors.New("db down")}
	svc := WithSpecialOpsAlertTeamGroups(WithSpecialOpsAlerts(NewIncidentReportService(), pub, tgConfig(t)), repo)
	toU2 := groupChange(soOtherGroup, tgU2Team, "jane.doe@wso2.com")

	if err := svc.HandleChange(ctx, alertTx(), toU2); err == nil || len(pub.sent) != 0 {
		t.Fatalf("lookup failing: err %v, sent %d; want an error so the change is retried", err, len(pub.sent))
	}

	repo.err = nil
	repo.teams = map[string]repository.SpecialistHandoffTeamGroup{
		"u2":          {ID: tgU2Team, Name: "U2"},
		"cloud-agent": {ID: tgCloudAgentTeam, Name: "WSO2 Cloud · Agent platform"},
		"cloud-core":  {ID: "2d9c3e4f-5a6b-4c7d-8e9f-a0b1c2d3e4f5", Name: "WSO2 Cloud · Core"},
	}
	if err := svc.HandleChange(ctx, alertTx(), toU2); err != nil || len(pub.sent) != 1 {
		t.Fatalf("lookup working: err %v, sent %d; want one alert", err, len(pub.sent))
	}
	var got events.IncidentSpecialOpsAlertPayload
	if err := json.Unmarshal(pub.sent[0].Payload, &got); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if got.Product != "U2" || got.TeamKey != "u2" || got.TeamLabel != "U2" || got.SMETeam != "u2" || got.AssignmentGroupID != tgU2Team {
		t.Errorf("payload %+v, want product U2, team u2 labelled U2 paging SME team u2, group %s", got, tgU2Team)
	}

	asked := len(repo.asked)
	if err := svc.HandleChange(ctx, alertTx(), groupChange(tgU2Team, soOtherGroup, "jane.doe@wso2.com")); err != nil || len(pub.sent) != 1 {
		t.Fatalf("unrelated group: err %v, sent %d; want no alert", err, len(pub.sent))
	}
	if len(repo.asked) != asked {
		t.Errorf("every team resolved, yet an unrelated group change looked them up again")
	}
}
