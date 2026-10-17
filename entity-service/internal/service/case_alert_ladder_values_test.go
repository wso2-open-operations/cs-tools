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
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func (f *fakeCaseAlertRepo) ListValues(context.Context) ([]repository.CaseAlertValueRow, error) {
	out := []repository.CaseAlertValueRow{}
	for _, v := range f.values {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ladder+out[i].Key < out[j].Ladder+out[j].Key })
	return out, nil
}

func (f *fakeCaseAlertRepo) GetValue(_ context.Context, id string) (*repository.CaseAlertValueRow, error) {
	for _, v := range f.values {
		if v.ID == id {
			return &v, nil
		}
	}
	return nil, nil
}

func (f *fakeCaseAlertRepo) SaveValues(_ context.Context, actor, ladder string, values map[string][]byte) error {
	f.actor = actor
	for k, v := range values {
		id := fmt.Sprintf("66666666-6666-4666-8666-%012d", len(f.values)+1)
		if was, ok := f.values[ladder+" "+k]; ok {
			id = was.ID
		}
		f.values[ladder+" "+k] = repository.CaseAlertValueRow{ID: id, Ladder: ladder, Key: k, Value: v, UpdatedBy: actor, UpdatedOn: time.Unix(0, 0)}
	}
	return nil
}

func (f *fakeCaseAlertRepo) DeleteValue(_ context.Context, actor, id string) (bool, error) {
	f.actor = actor
	for k, v := range f.values {
		if v.ID == id {
			delete(f.values, k)
			return true, nil
		}
	}
	return false, nil
}

// caseAlertGroups are the CSM Portal teams of the fake, by name.
var caseAlertGroups = map[string]string{"apollo": "SRE", "artemis sre group": "SRE", "asgardeo": "SME", "vega": "CRE"}

func (f *fakeCaseAlertRepo) RouteGroups(context.Context) ([]repository.CaseAlertTeamRow, error) {
	return []repository.CaseAlertTeamRow{{Key: "vega", Name: "Vega", Family: "CRE"}, {Key: "apollo", Name: "Apollo", Family: "SRE"}, {Key: "asgardeo", Name: "Asgardeo", Family: "SME"}}, nil
}

func (f *fakeCaseAlertRepo) RouteTeams(context.Context) ([]repository.CaseAlertTeamRow, error) {
	return []repository.CaseAlertTeamRow{{Key: "vega", Name: "Vega", Family: "CRE"}, {Key: "apollo", Name: "Apollo", Family: "SRE"}}, nil
}

func (f *fakeCaseAlertRepo) SRERotas(context.Context) ([]repository.CaseAlertRotaRow, error) {
	return []repository.CaseAlertRotaRow{{Code: "SRE_SAAS", Label: "SaaS SRE"}}, nil
}

func (f *fakeCaseAlertRepo) TeamFamilyByName(_ context.Context, name string) (string, bool, error) {
	fam, ok := caseAlertGroups[strings.ToLower(strings.TrimSpace(name))]
	return fam, ok, nil
}

// caseAlertCRENames are CRE teams no route group lists (a leadership team).
var caseAlertCRENames = map[string]bool{"vega": true, "cre leadership": true}

func (f *fakeCaseAlertRepo) IsCRETeamName(_ context.Context, name string) (bool, error) {
	return caseAlertCRENames[strings.ToLower(strings.TrimSpace(name))], nil
}

func (f *fakeCaseAlertRepo) TeamTypeExists(_ context.Context, t string) (bool, error) {
	return f.types[t], nil
}

func (f *fakeCaseAlertRepo) ShiftCodeExists(_ context.Context, code string) (bool, error) {
	return f.shifts[code], nil
}

func (f *fakeCaseAlertRepo) ActiveUserExists(_ context.Context, email string) (bool, error) {
	return f.users[strings.ToLower(email)], nil
}

// Each key takes only what the file would, within its limits, and is saved
// the way the file spells it.
func TestCaseAlertValueChecks(t *testing.T) {
	svc, _ := newCaseAlerts(newFakeCaseAlertRepo(), testCaseAlertKey)
	cases := []struct {
		ladder, key, raw string
		want             string // saved JSON; "" when refused
		refusal          string // part of the refusal
	}{
		{"CRE", "trigger.priorities", `["s0", "P1"]`, `["S0","S1"]`, ""},
		{"CRE", "trigger.priorities", `["S5"]`, "", "not a priority"},
		{"CRE", "trigger.priorities", `["S1", "s1"]`, "", "twice"},
		{"SRE", "trigger.teams", `[" SRE - Apollo "]`, `["SRE - Apollo"]`, ""},
		{"SRE", "trigger.excludeTeams", `[""]`, "", "1 to 255"},
		{"CRE", "trigger.shifts", `["lk_evening"]`, `["LK_EVENING"]`, ""},
		{"CRE", "trigger.shifts", `["NIGHT"]`, "", "not a shift"},
		{"SRE", "trigger.requireKnownTeam", `true`, `true`, ""},
		{"SRE", "trigger.requireKnownTeam", `"yes"`, "", "true or false"},
		{"CRE", "onLeave", `"CALL"`, `"call"`, ""},
		{"CRE", "onLeave", `"maybe"`, "", "skip or call"},
		{"CRE", "teams.abtType", `"cre-abt"`, `"cre-abt"`, ""},
		{"CRE", "teams.abtType", `"sre-abt"`, "", "not a CRE team type"},
		{"SRE", "teams.abtType", `"sre-nothing"`, "", "no team has type"},
		{"CRE", "teams.abts", `["Vega", "atlas"]`, `["vega","atlas"]`, ""},
		{"CRE", "teams.abts", `["apollo"]`, "", "not one of the Team Schedule's CRE teams"},
		{"SRE", "teams.abts", `["apollo", "artemis", "iaas"]`, `["apollo","artemis","iaas"]`, ""},
		{"CRE", "teams.abts", `[]`, `[]`, ""},
		{"CRE", "teams.americas", `"americas"`, `"americas"`, ""},
		{"CRE", "teams.americas", `"apollo"`, "", "CRE teams"},
		{"CRE", "teams.leadership", `"cre-leadership"`, `"cre-leadership"`, ""},
		{"CRE", "teams.leadership", `"CRE Leadership!"`, "", "not a team key"},
		{"CRE", "teams.teamLeadsToCall", `3`, `3`, ""},
		{"CRE", "teams.teamLeadsToCall", `21`, "", "0 to 20"},
		{"CRE", "teams.teamLeadsToCall", `3.5`, "", "whole number"},
		{"CRE", "teams.teamLeadsToCall", `"3"`, "", "whole number"},
		{"CRE", "teams.rotaMembersToCall", `{"lk_evening": 7}`, `{"LK_EVENING":7}`, ""},
		{"CRE", "teams.rotaMembersToCall", `{"NIGHT": 1}`, "", "not a shift"},
		{"CRE", "teams.rotaMembersToCall", `{"LK": 51}`, "", "0 to 50"},
		{"CRE", "teams.americasWeekendRotaShifts", `["cre_weekend_night"]`, `["CRE_WEEKEND_NIGHT"]`, ""},
		{"CRE", "teams.americasWeekendRotaShifts", `["NOPE"]`, "", "not an active Team Schedule shift"},
		{"CRE", "teams.rotaTeams", `["apollo", "vega"]`, `["apollo","vega"]`, ""},
		{"CRE", "teams.unassignedTeamLead", `"pool"`, `"pool"`, ""},
		{"CRE", "teams.unassignedTeamLeadCount", `11`, "", "0 to 10"},
		{"CRE", "heads.cre", `{"name": " CRE Head ", "email": "crehead@wso2.com"}`, `{"name":"CRE Head","email":"crehead@wso2.com"}`, ""},
		{"CRE", "heads.cs", `{"name": "CS Head", "email": "crehead@wso2.com", "phone": "+94770000000"}`, "", "a phone number is not saved here"},
		{"CRE", "heads.cs", `{"name": "CS Head", "email": "nobody@wso2.com"}`, "", "not an active CSM Portal user"},
		{"CRE", "heads.cs", `{"name": "CS Head", "email": "not an email"}`, "", "not an email address"},
		{"CRE", "heads.cs", `{"name": "", "email": "crehead@wso2.com"}`, "", "1 to 100"},
		{"CRE", "alertDuty.tiers", `["t1", "T2", "T3"]`, `["T1","T2","T3"]`, ""},
		{"CRE", "alertDuty.tiers", `["X1"]`, "", "not a tier"},
		{"CRE", "alertDuty.perTeam", `3`, `3`, ""},
		{"CRE", "acknowledgement.requireBoth", `false`, `false`, ""},
		{"SRE", "teams.defaultRota", `"sre_saas"`, `"SRE_SAAS"`, ""},
		{"SRE", "teams.defaultRota", `"CRE_ABT"`, "", "not an active SRE rota"},
		{"SRE", "timing.initialWait", `"0s"`, `"0s"`, ""},
		{"SRE", "timing.interval", `"5m"`, `"5m"`, ""},
		{"SRE", "timing.interval", `"30s"`, "", "from 1m to 30m"},
		{"SRE", "timing.interval", `"31m"`, "", "from 1m to 30m"},
		{"SRE", "timing.interval", `"soon"`, "", "written like 5m"},
		{"SRE", "timing.initialWait", `"31m"`, "", "from 0s to 30m, written like 0s or 2m"},
		{"SRE", "timing.includeL4", `false`, `false`, ""},
		{"SRE", "unansweredChat", `true`, `true`, ""},
		{"SRE", "smeHandoffChat", `false`, `false`, ""},
		// The wrong ladder, keys that stay in the file, no value.
		{"CRE", "timing.interval", `"5m"`, "", "not a CRE setting"},
		{"SRE", "heads.cre", `{"name": "x", "email": "crehead@wso2.com"}`, "", "not a SRE setting"},
		{"CRE", "enabled", `false`, "", "not a setting that can be saved here"},
		{"SRE", "channel", `"log"`, "", "not a setting that can be saved here"},
		{"SRE", "safety.maxCallsPerLadder", `100`, "", "not a setting that can be saved here"},
		{"SRE", "teams.aliases", `{}`, "", "not a setting that can be saved here"},
		{"CRE", "onLeave", `null`, "", "send a value"},
	}
	for _, c := range cases {
		t.Run(c.ladder+" "+c.key+" "+c.raw, func(t *testing.T) {
			got, err := svc.checkCaseAlertValue(context.Background(), c.ladder, c.key, json.RawMessage(c.raw))
			if c.want != "" {
				if err != nil || string(got) != c.want {
					t.Fatalf("got %s, %v; want %s", got, err, c.want)
				}
				return
			}
			wantAPIError[*apierror.ValidationError](t, err, "refusal")
			if !strings.Contains(err.Error(), c.refusal) || !strings.HasPrefix(err.Error(), c.key) {
				t.Fatalf("refusal %q; want it to name %s and say %q", err, c.key, c.refusal)
			}
		})
	}
}

// A value too long is refused before it is looked at.
func TestCaseAlertValueTooLong(t *testing.T) {
	svc, _ := newCaseAlerts(newFakeCaseAlertRepo(), testCaseAlertKey)
	long := `["` + strings.Repeat("a", maxCaseAlertValueBytes) + `"]`
	_, err := svc.checkCaseAlertValue(context.Background(), "SRE", "trigger.teams", json.RawMessage(long))
	wantAPIError[*apierror.ValidationError](t, err, "too long")
}

func TestCaseAlertSaveValues(t *testing.T) {
	repo := newFakeCaseAlertRepo()
	svc, _ := newCaseAlerts(repo, testCaseAlertKey)

	// The SRE rota admin changes SRE, not CRE; a head changes CRE, not SRE;
	// the SME rota admin changes SME, which has its own keys only.
	sre := asPerson("sre.rota@wso2.com")
	_, err := svc.SaveValues(sre, domain.SaveCaseAlertValuesRequest{Ladder: "CRE", Values: map[string]json.RawMessage{"onLeave": json.RawMessage(`"call"`)}})
	wantAPIError[*apierror.ForbiddenError](t, err, "SRE rota admin on CRE")
	head := asPerson("crehead@wso2.com")
	_, err = svc.SaveValues(head, domain.SaveCaseAlertValuesRequest{Ladder: "SRE", Values: map[string]json.RawMessage{"onLeave": json.RawMessage(`"call"`)}})
	wantAPIError[*apierror.ForbiddenError](t, err, "head on SRE")
	if _, err = svc.SaveValues(head, domain.SaveCaseAlertValuesRequest{Ladder: "CRE", Values: map[string]json.RawMessage{"onLeave": json.RawMessage(`"skip"`)}}); err != nil {
		t.Fatalf("head on CRE: %v", err)
	}
	sme := asPerson("sme.rota@wso2.com")
	_, err = svc.SaveValues(sme, domain.SaveCaseAlertValuesRequest{Ladder: "SRE", Values: map[string]json.RawMessage{"onLeave": json.RawMessage(`"call"`)}})
	wantAPIError[*apierror.ForbiddenError](t, err, "SME rota admin on SRE")
	_, err = svc.SaveValues(sme, domain.SaveCaseAlertValuesRequest{Ladder: "SME", Values: map[string]json.RawMessage{"heads.cre": json.RawMessage(`{"name":"x","email":"x@example.test"}`)}})
	wantAPIError[*apierror.ValidationError](t, err, "a CRE key on SME")
	if _, err = svc.SaveValues(sme, domain.SaveCaseAlertValuesRequest{Ladder: "sme", Values: map[string]json.RawMessage{
		"timing.interval": json.RawMessage(`"10m"`),
		"onLeave":         json.RawMessage(`"call"`),
		"unansweredChat":  json.RawMessage(`true`),
		"teams":           json.RawMessage(`{"Asgardeo-Ops":"asgardeo"}`),
	}}); err != nil {
		t.Fatalf("SME values: %v", err)
	}
	_, err = svc.SaveValues(sme, domain.SaveCaseAlertValuesRequest{Ladder: "SME", Values: map[string]json.RawMessage{"teams": json.RawMessage(`{"ops":"apollo"}`)}})
	wantAPIError[*apierror.ValidationError](t, err, "sme.teams to an SRE team")
	repo.values = map[string]repository.CaseAlertValueRow{}

	// One refused value saves none of them.
	_, err = svc.SaveValues(sre, domain.SaveCaseAlertValuesRequest{Ladder: "sre", Values: map[string]json.RawMessage{
		"timing.interval":  json.RawMessage(`"10m"`),
		"timing.includeL4": json.RawMessage(`"maybe"`),
	}})
	wantAPIError[*apierror.ValidationError](t, err, "one bad value")
	if len(repo.values) != 0 {
		t.Fatalf("saved %d values from a refused request", len(repo.values))
	}

	resp, err := svc.SaveValues(sre, domain.SaveCaseAlertValuesRequest{Ladder: "sre", Values: map[string]json.RawMessage{
		"timing.interval":   json.RawMessage(`"10m"`),
		"teams.defaultRota": json.RawMessage(`"sre_saas"`),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if repo.actor != "sre.rota@wso2.com" || len(resp.Values) != 2 {
		t.Fatalf("actor %q, %d values back", repo.actor, len(resp.Values))
	}
	if got := string(repo.values["SRE teams.defaultRota"].Value); got != `"SRE_SAAS"` {
		t.Fatalf("defaultRota saved as %s", got)
	}

	// Only that ladder's values come back.
	admin := asPerson("someone@wso2.com", "cs_admin")
	resp, err = svc.SaveValues(admin, domain.SaveCaseAlertValuesRequest{Ladder: "CRE", Values: map[string]json.RawMessage{"onLeave": json.RawMessage(`"call"`)}})
	if err != nil || len(resp.Values) != 1 || resp.Values[0].Ladder != "CRE" {
		t.Fatalf("CRE save: %v %+v", err, resp)
	}

	for name, req := range map[string]domain.SaveCaseAlertValuesRequest{
		"nothing":      {Ladder: "CRE"},
		"bad ladder":   {Ladder: "PAAS", Values: map[string]json.RawMessage{"onLeave": json.RawMessage(`"call"`)}},
		"over the cap": {Ladder: "CRE", Values: tooManyValues()},
	} {
		if _, err := svc.SaveValues(admin, req); err == nil {
			t.Errorf("%s: saved", name)
		}
	}
}

func tooManyValues() map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for i := 0; i <= maxCaseAlertValues; i++ {
		out[fmt.Sprintf("k%d", i)] = json.RawMessage(`1`)
	}
	return out
}

func TestCaseAlertDeleteValue(t *testing.T) {
	repo := newFakeCaseAlertRepo()
	svc, _ := newCaseAlerts(repo, testCaseAlertKey)
	admin := asPerson("someone@wso2.com", "cs_admin")
	if _, err := svc.SaveValues(admin, domain.SaveCaseAlertValuesRequest{Ladder: "CRE", Values: map[string]json.RawMessage{"onLeave": json.RawMessage(`"call"`)}}); err != nil {
		t.Fatal(err)
	}
	id := repo.values["CRE onLeave"].ID

	wantAPIError[*apierror.ForbiddenError](t, svc.DeleteValue(asPerson("sre.rota@wso2.com"), id), "SRE rota admin, CRE value")
	wantAPIError[*apierror.ValidationError](t, svc.DeleteValue(admin, "not-a-uuid"), "bad id")
	wantAPIError[*apierror.NotFoundError](t, svc.DeleteValue(admin, "77777777-7777-4777-8777-777777777777"), "unknown id")
	if err := svc.DeleteValue(asPerson("cre.rota@wso2.com"), id); err != nil {
		t.Fatal(err)
	}
	if len(repo.values) != 0 || repo.actor != "cre.rota@wso2.com" {
		t.Fatalf("values %v, actor %q", repo.values, repo.actor)
	}
}

// The tab shows the values saved here, and what the alert service reported it
// runs with -- only from a start in shadow or db mode.
func TestCaseAlertGetShowsValuesAndRunning(t *testing.T) {
	repo := newFakeCaseAlertRepo()
	svc, _ := newCaseAlerts(repo, testCaseAlertKey)
	admin := asPerson("someone@wso2.com", "cs_admin")
	if _, err := svc.SaveValues(admin, domain.SaveCaseAlertValuesRequest{Ladder: "SRE", Values: map[string]json.RawMessage{"timing.interval": json.RawMessage(`"5m"`)}}); err != nil {
		t.Fatal(err)
	}
	shadow := "shadow"
	repo.mode = &shadow
	repo.summary = []byte(`{"rooms":0,"running":{"ladders":{"SRE":{"timing.interval":{"value":"5m","source":"file"}}},"fileOnly":{"SRE":{"channel":"call","safety.maxCallsPerLadder":4}}}}`)

	got, err := svc.Get(admin)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Values) != 1 || got.Values[0].Key != "timing.interval" || string(got.Values[0].Value) != `"5m"` {
		t.Fatalf("values %+v", got.Values)
	}
	if got.Running == nil || got.Running.Mode != "shadow" || got.Running.Ladders["SRE"]["timing.interval"].Source != "file" {
		t.Fatalf("running %+v", got.Running)
	}
	if string(got.Running.FileOnly["SRE"]["channel"]) != `"call"` {
		t.Fatalf("file-only settings %v", got.Running.FileOnly)
	}

	off := "off"
	repo.mode = &off
	if got, _ := svc.Get(admin); got.Running != nil {
		t.Fatalf("running shown from an off start: %+v", got.Running)
	}
	repo.mode, repo.summary = &shadow, []byte(`{"rooms":0}`)
	if got, _ := svc.Get(admin); got.Running != nil {
		t.Fatalf("running shown without a report of it: %+v", got.Running)
	}

	// The lists a route is made from.
	if o := got.RouteOptions; o == nil || len(o.Groups) != 3 || len(o.Teams) != 2 || len(o.Rotas) != 1 || o.Groups[2].Family != "SME" {
		t.Fatalf("route options %+v", got.RouteOptions)
	}

	// The CRE rota admin is sent neither SRE's values nor what SRE runs with.
	repo.mode, repo.summary = &shadow, []byte(`{"rooms":0,"running":{"ladders":{"SRE":{"timing.interval":{"value":"5m","source":"file"}},"CRE":{"onLeave":{"value":"skip","source":"file"}}},"fileOnly":{"SRE":{"channel":"call"},"CRE":{"channel":"both"}}}}`)
	cre, _ := svc.Get(asPerson("cre.rota@wso2.com"))
	if len(cre.Values) != 0 || cre.Running == nil || cre.Running.Ladders["SRE"] != nil || cre.Running.FileOnly["SRE"] != nil || cre.Running.Ladders["CRE"] == nil {
		t.Fatalf("CRE rota admin was sent %+v / %+v", cre.Values, cre.Running)
	}

	// Somebody who may see nothing gets no values either.
	if got, _ := svc.Get(asPerson("eng@wso2.com")); len(got.Values) != 0 || got.Running != nil || got.RouteOptions != nil {
		t.Fatalf("engineer saw %+v", got)
	}
}

func TestCaseAlertRuntimeCarriesValues(t *testing.T) {
	repo := newFakeCaseAlertRepo()
	svc, _ := newCaseAlerts(repo, testCaseAlertKey)
	if _, err := svc.SaveValues(asPerson("someone@wso2.com", "cs_admin"), domain.SaveCaseAlertValuesRequest{Ladder: "CRE", Values: map[string]json.RawMessage{
		"teams.abts": json.RawMessage(`["vega"]`),
	}}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Runtime(asService("csm-notification-service"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Values) != 1 || got.Values[0].Ladder != "CRE" || got.Values[0].Key != "teams.abts" || string(got.Values[0].Value) != `["vega"]` {
		t.Fatalf("runtime values %+v", got.Values)
	}
}

// A team's row takes how its alerts go out; the shared room does not (the
// ladder's channel is the file's).
func TestCaseAlertRoomChannel(t *testing.T) {
	ch := func(v string) *string { return &v }
	sre := asPerson("sre.rota@wso2.com")
	cases := []struct {
		name string
		req  domain.SaveCaseAlertRoomRequest
		want string // saved channel, or "refused"
	}{
		{"chat only for a team", domain.SaveCaseAlertRoomRequest{Ladder: "SRE", TeamKey: "artemis", Label: "Artemis SRE", Channel: ch(" Chat ")}, "chat"},
		{"back to the ladder's", domain.SaveCaseAlertRoomRequest{Ladder: "SRE", TeamKey: "artemis", Label: "Artemis SRE", Channel: ch("")}, ""},
		{"not given keeps none", domain.SaveCaseAlertRoomRequest{Ladder: "SRE", TeamKey: "artemis", Label: "Artemis SRE"}, ""},
		{"not a channel", domain.SaveCaseAlertRoomRequest{Ladder: "SRE", TeamKey: "artemis", Label: "Artemis SRE", Channel: ch("sms")}, "refused"},
		{"the shared room", domain.SaveCaseAlertRoomRequest{Ladder: "SRE", Label: "SRE Case Alerts", Channel: ch("call")}, "refused"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := newFakeCaseAlertRepo()
			svc, _ := newCaseAlerts(repo, testCaseAlertKey)
			_, err := svc.SaveRoom(sre, c.req)
			if c.want == "refused" {
				wantAPIError[*apierror.ValidationError](t, err, c.name)
				return
			}
			if err != nil || repo.saved == nil || repo.saved.Channel != c.want {
				t.Fatalf("err %v, saved %+v", err, repo.saved)
			}
		})
	}
}

// A team's row saved for its channel alone reaches the alert service without
// a URL; an enabled row with neither is left out, as before.
func TestCaseAlertRuntimeCarriesChannels(t *testing.T) {
	repo := newFakeCaseAlertRepo()
	repo.rooms[roomApollo].Channel = "chat" // no URL saved
	svc, _ := newCaseAlerts(repo, testCaseAlertKey)
	got, err := svc.Runtime(asService("csm-notification-service"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rooms) != 1 || got.Rooms[0].TeamKey != "apollo" || got.Rooms[0].Channel != "chat" || got.Rooms[0].WebhookURL != "" {
		t.Fatalf("runtime rooms %+v", got.Rooms)
	}
}
