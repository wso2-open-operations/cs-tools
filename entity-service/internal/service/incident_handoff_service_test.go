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
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	hoIncidentID      = "11111111-1111-4111-8111-111111111111"
	hoChoreoService   = "b9c999f8-1b86-a010-00ae-86acdd4bcb61"
	hoAsgardeoService = "97ed1b8b-1ba2-6c10-00ae-86acdd4bcbd3"
	hoMoesifService   = "d4aa1a3b-3b29-0b10-9140-4c6aa5e45a61"
	hoChoreoSpecial   = "fe0d8868-1b0b-3010-d64e-64a2604bcb3c"
	hoChoreoRuntime   = "80dade5d-1b70-0710-a002-c9d3604bcbd7"
	hoChoreoAPIM      = "a79a1e9d-1b70-0710-a002-c9d3604bcb20"
	hoAsgardeoSpecial = "7fb4f4c6-1b4b-3810-aea4-a936604bcb90"
	hoMoesifGroup     = "55555555-5555-4555-8555-555555555555"
)

// hoConfigJSON is SPECIALIST_HANDOFF_CONFIG as a deployment sets it:
// ServiceNow's routing for Choreo (three teams) and Asgardeo (one), plus a
// one-team product with no GitHub repository.
const hoConfigJSON = `{"products":[
 {"name":"Choreo","serviceIds":["B9C999F8-1B86-A010-00AE-86ACDD4BCB61"],"github":{"owner":"wso2-enterprise","repo":"choreo"},
  "teams":[{"key":"choreo-special-ops","label":"Choreo Special Ops","groupId":"fe0d8868-1b0b-3010-d64e-64a2604bcb3c"},
           {"key":"choreo-runtime-team","label":"Choreo Runtime Team","groupId":"80dade5d-1b70-0710-a002-c9d3604bcbd7"},
           {"key":"choreo-apim-team","label":"Choreo APIM Team","groupId":"a79a1e9d-1b70-0710-a002-c9d3604bcb20"}]},
 {"name":"Asgardeo","serviceIds":["97ed1b8b-1ba2-6c10-00ae-86acdd4bcbd3"],"github":{"owner":"wso2-enterprise","repo":"asgardeo-product","credential":"asgardeo"},
  "teams":[{"key":"asgardeo-special-ops","label":"Asgardeo Special Ops","groupId":"7fb4f4c6-1b4b-3810-aea4-a936604bcb90"}]},
 {"name":"Moesif","serviceIds":["d4aa1a3b-3b29-0b10-9140-4c6aa5e45a61"],
  "teams":[{"key":"moesif-special-ops","label":"Moesif Special Ops","groupId":"55555555-5555-4555-8555-555555555555"}]}]}`

func hoConfig(t *testing.T) *SpecialistHandoffConfig {
	t.Helper()
	cfg, err := ParseSpecialistHandoffConfig(hoConfigJSON)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	return cfg
}

func hoStr(s string) *string { return &s }

func hoTeam(t domain.IncidentSpecialistHandoffEscalationTeam) *domain.IncidentSpecialistHandoffEscalationTeam {
	return &t
}

func hoSnapshot(service, group, state string) repository.SpecialistHandoffSnapshot {
	snap := repository.SpecialistHandoffSnapshot{
		IncidentID: hoIncidentID, Number: "INC0099001", Subject: "Gateway 502s", State: state,
		Description: hoStr("All gateways return 502."),
	}
	if service != "" {
		snap.ServiceID = &service
	}
	if group != "" {
		snap.AssignmentGroupID = &group
		snap.AssignmentGroupName = hoStr("Choreo Operations")
	}
	return snap
}

func TestParseSpecialistHandoffConfig(t *testing.T) {
	cfg := hoConfig(t)
	if len(cfg.Products) != 3 || cfg.Products[0].ServiceIDs[0] != hoChoreoService {
		t.Fatalf("products %+v, want three with service ids lower-cased", cfg.Products)
	}
	if empty, err := ParseSpecialistHandoffConfig("  "); err != nil || len(empty.Products) != 0 {
		t.Errorf("empty value: %+v %v, want an empty config", empty, err)
	}
	team := func(key, group string) string {
		return `{"key":"` + key + `","label":"L","groupId":"` + group + `"}`
	}
	product := func(name, service, github string, teams ...string) string {
		return `{"name":"` + name + `","serviceIds":["` + service + `"]` + github + `,"teams":[` + strings.Join(teams, ",") + `]}`
	}
	wrap := func(products ...string) string { return `{"products":[` + strings.Join(products, ",") + `]}` }
	ok := team("a", hoMoesifGroup)
	for name, raw := range map[string]string{
		"not JSON":           `{"products":`,
		"unknown field":      `{"products":[],"extra":1}`,
		"no name":            wrap(product("", hoMoesifService, "", ok)),
		"no services":        wrap(`{"name":"X","teams":[` + ok + `]}`),
		"bad service id":     wrap(product("X", "moesif", "", ok)),
		"service twice":      wrap(product("X", hoMoesifService, "", ok), product("Y", hoMoesifService, "", ok)),
		"no teams":           wrap(product("X", hoMoesifService, "")),
		"team without key":   wrap(product("X", hoMoesifService, "", team("", hoMoesifGroup))),
		"team key too long":  wrap(product("X", hoMoesifService, "", team(strings.Repeat("k", 65), hoMoesifGroup))),
		"team without label": wrap(product("X", hoMoesifService, "", `{"key":"a","groupId":"`+hoMoesifGroup+`"}`)),
		"duplicate key":      wrap(product("X", hoMoesifService, "", ok, ok)),
		"bad group id":       wrap(product("X", hoMoesifService, "", team("a", "Moesif"))),
		"half a github":      wrap(product("X", hoMoesifService, `,"github":{"owner":"wso2-enterprise"}`, ok)),
	} {
		if _, err := ParseSpecialistHandoffConfig(raw); err == nil || !strings.Contains(err.Error(), "SPECIALIST_HANDOFF_CONFIG") {
			t.Errorf("%s: err %v, want a SPECIALIST_HANDOFF_CONFIG error", name, err)
		}
	}
}

// TestPlanSpecialistHandoff_Eligibility ports IncidentHandoffUtils
// .checkEligibility: In Progress only, a service some product covers only,
// not already with one of that product's Special Ops groups.
func TestPlanSpecialistHandoff_Eligibility(t *testing.T) {
	cfg := hoConfig(t)
	req := domain.HandOffIncidentToSpecialistRequest{IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook,
		EscalationTeam: hoTeam(domain.IncidentSpecialistHandoffTeamChoreoRuntime)}
	for name, snap := range map[string]repository.SpecialistHandoffSnapshot{
		"not In Progress":          hoSnapshot(hoChoreoService, "", "NEW"),
		"no service":               hoSnapshot("", "", "IN_PROGRESS"),
		"service in no product":    hoSnapshot("22222222-2222-4222-8222-222222222222", "", "IN_PROGRESS"),
		"already Choreo SpecOps":   hoSnapshot(hoChoreoService, hoChoreoSpecial, "IN_PROGRESS"),
		"already Choreo Runtime":   hoSnapshot(hoChoreoService, strings.ToUpper(hoChoreoRuntime), "IN_PROGRESS"),
		"already Asgardeo SpecOps": hoSnapshot(hoAsgardeoService, hoAsgardeoSpecial, "IN_PROGRESS"),
	} {
		t.Run(name, func(t *testing.T) {
			_, _, _, err := planSpecialistHandoff(cfg, req, snap)
			var ce *apierror.ConflictError
			if !errors.As(err, &ce) {
				t.Fatalf("expected ConflictError, got %T: %v", err, err)
			}
		})
	}
	if _, _, _, err := planSpecialistHandoff(nil, req, hoSnapshot(hoChoreoService, "", "IN_PROGRESS")); err == nil {
		t.Error("no config: want a ConflictError")
	}
}

// TestPlanSpecialistHandoff_Routing: the incident's product picks the team.
// A product with several teams needs one named; a product with one takes it
// whatever was sent and records no team. The runbook task goes to the same
// group as the incident.
func TestPlanSpecialistHandoff_Routing(t *testing.T) {
	cfg := hoConfig(t)
	cases := []struct {
		name     string
		service  string
		team     *domain.IncidentSpecialistHandoffEscalationTeam
		want     string
		product  string
		recorded *domain.IncidentSpecialistHandoffEscalationTeam
	}{
		{"choreo special ops", hoChoreoService, hoTeam("choreo-special-ops"), hoChoreoSpecial, "Choreo", hoTeam("choreo-special-ops")},
		{"choreo runtime", hoChoreoService, hoTeam(domain.IncidentSpecialistHandoffTeamChoreoRuntime), hoChoreoRuntime, "Choreo", hoTeam(domain.IncidentSpecialistHandoffTeamChoreoRuntime)},
		{"choreo apim", hoChoreoService, hoTeam(domain.IncidentSpecialistHandoffTeamChoreoAPIM), hoChoreoAPIM, "Choreo", hoTeam(domain.IncidentSpecialistHandoffTeamChoreoAPIM)},
		{"asgardeo", hoAsgardeoService, nil, hoAsgardeoSpecial, "Asgardeo", nil},
		{"asgardeo ignores team", hoAsgardeoService, hoTeam(domain.IncidentSpecialistHandoffTeamChoreoRuntime), hoAsgardeoSpecial, "Asgardeo", nil},
		{"moesif", hoMoesifService, nil, hoMoesifGroup, "Moesif", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := domain.HandOffIncidentToSpecialistRequest{IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook, EscalationTeam: c.team}
			plan, product, recorded, err := planSpecialistHandoff(cfg, req, hoSnapshot(c.service, "", "IN_PROGRESS"))
			if err != nil {
				t.Fatalf("plan: %v", err)
			}
			if plan.GroupID != c.want || product == nil || product.Name != c.product {
				t.Errorf("group %s product %+v, want %s %s", plan.GroupID, product, c.want, c.product)
			}
			if plan.TaskGroupID == nil || *plan.TaskGroupID != c.want {
				t.Errorf("runbook task group %v, want the Special Ops group %s", plan.TaskGroupID, c.want)
			}
			if (recorded.EscalationTeam == nil) != (c.recorded == nil) || (c.recorded != nil && *recorded.EscalationTeam != *c.recorded) {
				t.Errorf("recorded team %v, want %v", recorded.EscalationTeam, c.recorded)
			}
		})
	}

	for name, team := range map[string]*domain.IncidentSpecialistHandoffEscalationTeam{
		"choreo without team":  nil,
		"choreo unknown team":  hoTeam("moesif-special-ops"),
		"choreo asgardeo team": hoTeam("asgardeo-special-ops"),
	} {
		req := domain.HandOffIncidentToSpecialistRequest{IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook, EscalationTeam: team}
		_, _, _, err := planSpecialistHandoff(cfg, req, hoSnapshot(hoChoreoService, "", "IN_PROGRESS"))
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || !strings.Contains(err.Error(), "choreo-runtime-team") {
			t.Errorf("%s: %v, want a ValidationError listing Choreo's teams", name, err)
		}
	}
}

// TestPlanSpecialistHandoff_NotesAndTask: the reason note is byte for byte
// what the UI action's modal writes, and the task subject is the UI action's.
func TestPlanSpecialistHandoff_NotesAndTask(t *testing.T) {
	cfg := hoConfig(t)
	cases := []struct {
		service string
		reason  domain.IncidentSpecialistHandoffReasonCode
		team    *domain.IncidentSpecialistHandoffEscalationTeam
		blob    string
		subject string
	}{
		{hoAsgardeoService, domain.IncidentSpecialistHandoffReasonNoRunbook, nil,
			`{"reasonCode":"no-runbook","reasonDescription":"Runbook is not available","escalationTeam":null}`,
			"[Runbook Task] No entry available for INC0099001"},
		{hoChoreoService, domain.IncidentSpecialistHandoffReasonRunbookNotWorking, hoTeam(domain.IncidentSpecialistHandoffTeamChoreoAPIM),
			`{"reasonCode":"runbook-not-working","reasonDescription":"Runbook doesn't solve the incident","escalationTeam":"choreo-apim-team"}`,
			"[Runbook Task] Entry didn't solve the incident INC0099001"},
	}
	for _, c := range cases {
		req := domain.HandOffIncidentToSpecialistRequest{IncidentID: hoIncidentID, ReasonCode: c.reason, EscalationTeam: c.team}
		plan, _, _, err := planSpecialistHandoff(cfg, req, hoSnapshot(c.service, "", "IN_PROGRESS"))
		if err != nil {
			t.Fatalf("plan: %v", err)
		}
		if len(plan.WorkNotes) != 1 || plan.WorkNotes[0] != c.blob {
			t.Errorf("work notes %q, want [%s]", plan.WorkNotes, c.blob)
		}
		if plan.TaskSubject != c.subject {
			t.Errorf("task subject %q, want %q", plan.TaskSubject, c.subject)
		}
	}
}

type fakeHandoffIssues struct {
	calls  int
	owner  string
	repo   string
	title  string
	body   string
	result *github.CreatedIssue
	err    error
}

func (f *fakeHandoffIssues) CreateIssue(_ context.Context, owner, repository, title, body string, _ []string) (*github.CreatedIssue, error) {
	f.calls++
	f.owner, f.repo, f.title, f.body = owner, repository, title, body
	return f.result, f.err
}

// handoffHarness runs HandOffIncidentToSpecialist against a stub repository
// that applies the real plan to snap, and records the follow-up note.
func handoffHarness(t *testing.T, snap repository.SpecialistHandoffSnapshot, issues *fakeHandoffIssues, req domain.HandOffIncidentToSpecialistRequest) (domain.HandOffIncidentToSpecialistResponse, error, *repository.SpecialistHandoffPlan, []string) {
	t.Helper()
	var applied *repository.SpecialistHandoffPlan
	var notes []string
	repo := &stubIncidentRepo{
		applySpecialistHandoff: func(_ context.Context, id, actor string, plan func(repository.SpecialistHandoffSnapshot) (repository.SpecialistHandoffPlan, error)) (repository.SpecialistHandoffWritten, error) {
			if id != hoIncidentID || actor != "jane.doe@example.com" {
				t.Errorf("ApplySpecialistHandoff(%s, %s)", id, actor)
			}
			p, err := plan(snap)
			if err != nil {
				return repository.SpecialistHandoffWritten{}, err
			}
			applied = &p
			return repository.SpecialistHandoffWritten{Before: snap, GroupName: "Choreo Special Ops", TaskID: "33333333-3333-4333-8333-333333333333", TaskNumber: "CS-PORTAL-000123"}, nil
		},
		createIncidentComment: func(_ context.Context, _ string, ct domain.CommentType, content, _ string) (domain.CaseComment, error) {
			if ct != domain.CommentTypeWorkNote {
				t.Errorf("follow-up note type %s, want work note", ct)
			}
			notes = append(notes, content)
			return domain.CaseComment{}, nil
		},
		getIncidentByID: func(_ context.Context, id string) (domain.IncidentView, error) {
			return domain.IncidentView{ID: &id, AssignmentGroup: &domain.EntityRef{ID: hoChoreoSpecial, Name: "Choreo Special Ops"}}, nil
		},
	}
	svc := WithSpecialistHandoffConfig(NewIncidentService(repo, nil), hoConfig(t))
	if issues != nil {
		svc = WithHandoffIssueCreators(svc, SpecialistHandoffIssueClients{"wso2-enterprise": issues, "asgardeo": issues})
	}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	resp, err := svc.HandOffIncidentToSpecialist(ctx, req)
	return resp, err, applied, notes
}

func TestHandOffIncidentToSpecialist_FilesIssueAndNotes(t *testing.T) {
	issues := &fakeHandoffIssues{result: &github.CreatedIssue{Number: 42, HTMLURL: "https://github.com/wso2-enterprise/choreo/issues/42"}}
	req := domain.HandOffIncidentToSpecialistRequest{IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook, EscalationTeam: hoTeam("choreo-special-ops")}
	resp, err, applied, notes := handoffHarness(t, hoSnapshot(hoChoreoService, "44444444-4444-4444-8444-444444444444", "IN_PROGRESS"), issues, req)
	if err != nil {
		t.Fatalf("handoff: %v", err)
	}
	if applied == nil || applied.GroupID != hoChoreoSpecial {
		t.Fatalf("applied plan %+v", applied)
	}
	if issues.calls != 1 || issues.owner != "wso2-enterprise" || issues.repo != "choreo" || issues.title != "Gateway 502s" || issues.body != "All gateways return 502." {
		t.Errorf("CreateIssue(%s/%s, %q, %q) x%d", issues.owner, issues.repo, issues.title, issues.body, issues.calls)
	}
	want := "Escalated to Special Ops team. Escalated by jane.doe@example.com(jane.doe@example.com) Opened an internal issue. Please access the ticket using the link https://github.com/wso2-enterprise/choreo/issues/42 to add more details to the ticket if needed"
	if len(notes) != 1 || notes[0] != want {
		t.Errorf("follow-up notes %q, want [%q]", notes, want)
	}
	h := resp.Handoff
	if h.GithubIssue == nil || h.GithubIssue.Number != 42 || h.GithubIssue.Repo != "choreo" || h.GithubIssueError != nil {
		t.Errorf("github result %+v / %v", h.GithubIssue, h.GithubIssueError)
	}
	if h.AssignmentGroup.ID != hoChoreoSpecial || h.PreviousAssignmentGroup == nil || h.PreviousAssignmentGroup.Name != "Choreo Operations" {
		t.Errorf("groups %+v / %+v", h.AssignmentGroup, h.PreviousAssignmentGroup)
	}
	if h.Task.Number != "CS-PORTAL-000123" || h.Task.Subject != "[Runbook Task] No entry available for INC0099001" {
		t.Errorf("task %+v", h.Task)
	}
	if h.ReasonDescription != "Runbook is not available" || h.Incident.ID == nil {
		t.Errorf("reason %q incident %v", h.ReasonDescription, h.Incident.ID)
	}
}

func TestHandOffIncidentToSpecialist_GithubOutcomes(t *testing.T) {
	short := "Escalated to Special Ops team. Escalated by jane.doe@example.com(jane.doe@example.com)"
	off := false
	cases := []struct {
		name      string
		issues    *fakeHandoffIssues
		create    *bool
		wantCalls int
		wantError string
	}{
		{"not configured", nil, nil, 0, "GitHub issue creation is not configured on this deployment"},
		{"github refuses", &fakeHandoffIssues{err: &github.Error{StatusCode: 403, Message: "Resource not accessible"}}, nil, 1, "GitHub issue creation failed (403)"},
		{"github unreachable", &fakeHandoffIssues{err: errors.New("dial tcp: timeout")}, nil, 1, "GitHub issue creation failed: dial tcp: timeout"},
		{"not requested", &fakeHandoffIssues{}, &off, 0, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := domain.HandOffIncidentToSpecialistRequest{IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook, CreateGithubIssue: c.create}
			resp, err, _, notes := handoffHarness(t, hoSnapshot(hoAsgardeoService, "", "IN_PROGRESS"), c.issues, req)
			if err != nil {
				t.Fatalf("handoff must succeed whatever GitHub does: %v", err)
			}
			if c.issues != nil && c.issues.calls != c.wantCalls {
				t.Errorf("CreateIssue calls %d, want %d", c.issues.calls, c.wantCalls)
			}
			gotErr := ""
			if resp.Handoff.GithubIssueError != nil {
				gotErr = *resp.Handoff.GithubIssueError
			}
			if gotErr != c.wantError || resp.Handoff.GithubIssue != nil {
				t.Errorf("githubIssueError %q issue %+v, want %q and none", gotErr, resp.Handoff.GithubIssue, c.wantError)
			}
			if len(notes) != 1 || notes[0] != short {
				t.Errorf("follow-up notes %q, want [%q]", notes, short)
			}
		})
	}
}

func TestHandOffIncidentToSpecialist_RejectsBadRequestBeforeWriting(t *testing.T) {
	svc := NewIncidentService(&stubIncidentRepo{}, nil) // ApplySpecialistHandoff panics if reached
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	for name, req := range map[string]domain.HandOffIncidentToSpecialistRequest{
		"no reason":  {IncidentID: hoIncidentID},
		"bad reason": {IncidentID: hoIncidentID, ReasonCode: "because"},
		"blank team": {IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook, EscalationTeam: hoTeam(" ")},
		"long team":  {IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook, EscalationTeam: hoTeam(domain.IncidentSpecialistHandoffEscalationTeam(strings.Repeat("x", 65)))},
		"bad id":     {IncidentID: "nope", ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook},
	} {
		_, err := svc.HandOffIncidentToSpecialist(ctx, req)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: got %T %v, want ValidationError", name, err, err)
		}
	}
	if _, err := svc.HandOffIncidentToSpecialist(context.Background(), domain.HandOffIncidentToSpecialistRequest{IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook}); err == nil || !strings.Contains(err.Error(), "x-user-id-token") {
		t.Errorf("missing token: %v, want an x-user-id-token error", err)
	}
}

func TestHandOffIncidentToSpecialist_ProductWithoutRepoFilesNoIssue(t *testing.T) {
	issues := &fakeHandoffIssues{}
	req := domain.HandOffIncidentToSpecialistRequest{IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook}
	resp, err, applied, _ := handoffHarness(t, hoSnapshot(hoMoesifService, "", "IN_PROGRESS"), issues, req)
	if err != nil {
		t.Fatalf("handoff: %v", err)
	}
	if applied == nil || applied.GroupID != hoMoesifGroup {
		t.Errorf("applied plan %+v, want Moesif's one team", applied)
	}
	if issues.calls != 0 || resp.Handoff.GithubIssueError == nil || *resp.Handoff.GithubIssueError != "No GitHub repository is configured for this product's specialist handoffs" {
		t.Errorf("calls %d error %v", issues.calls, resp.Handoff.GithubIssueError)
	}
}

func TestListSpecialistHandoffTeams(t *testing.T) {
	svc := WithSpecialistHandoffConfig(NewIncidentService(&stubIncidentRepo{}, nil), hoConfig(t))
	keys := func(r domain.SpecialistHandoffTeamsResponse) string {
		var k []string
		for _, t := range r.Teams {
			k = append(k, t.Key)
		}
		return strings.Join(k, ",")
	}
	for service, want := range map[string]string{
		hoChoreoService:                        "choreo-special-ops,choreo-runtime-team,choreo-apim-team",
		strings.ToUpper(hoAsgardeoService):     "asgardeo-special-ops",
		hoMoesifService:                        "moesif-special-ops",
		"22222222-2222-4222-8222-222222222222": "",
		"":                                     "choreo-special-ops,choreo-runtime-team,choreo-apim-team,asgardeo-special-ops,moesif-special-ops",
	} {
		got, err := svc.ListSpecialistHandoffTeams(context.Background(), service)
		if err != nil || keys(got) != want || got.Teams == nil {
			t.Errorf("Postgres teams for %q: %q err %v, want %q", service, keys(got), err, want)
		}
	}
	if _, err := svc.ListSpecialistHandoffTeams(context.Background(), "not-a-uuid"); err == nil {
		t.Error("bad serviceId: want a ValidationError")
	}
	if got, _ := NewIncidentService(&stubIncidentRepo{}, nil).ListSpecialistHandoffTeams(context.Background(), hoChoreoService); got.Teams == nil || len(got.Teams) != 0 {
		t.Errorf("no config: %+v, want an empty list", got.Teams)
	}

	sn := &snIncidentService{}
	for service, want := range map[string]string{
		hoChoreoService:   "choreo-special-ops,choreo-runtime-team,choreo-apim-team",
		hoAsgardeoService: "asgardeo-special-ops",
		hoMoesifService:   "",
	} {
		if got, _ := sn.ListSpecialistHandoffTeams(context.Background(), service); keys(got) != want || got.Teams == nil {
			t.Errorf("ServiceNow teams for %s: %q, want %q", service, keys(got), want)
		}
	}
}

// TestIncidentView_SpecialistHandoff: the view offers the handoff exactly
// when planSpecialistHandoff would take it, and keeps a recorded team only
// when the configuration knows it.
func TestIncidentView_SpecialistHandoff(t *testing.T) {
	cfg := hoConfig(t)
	view := func(state, service, group string, team domain.IncidentSpecialistHandoffEscalationTeam) domain.IncidentView {
		v := domain.IncidentView{State: hoStr(state), SpecialistHandoff: &domain.IncidentSpecialistHandoffSummary{}}
		if service != "" {
			v.Service = &domain.EntityRef{ID: service}
		}
		if group != "" {
			v.AssignmentGroup = &domain.EntityRef{ID: group}
		}
		if team != "" {
			v.SpecialistHandoff.EscalationTeam = &team
		}
		return v
	}
	cases := []struct {
		name     string
		in       domain.IncidentView
		cfg      *SpecialistHandoffConfig
		can      bool
		keptTeam bool
	}{
		{"choreo in progress", view("IN_PROGRESS", hoChoreoService, "44444444-4444-4444-8444-444444444444", "choreo-apim-team"), cfg, true, true},
		{"moesif no group", view("IN_PROGRESS", hoMoesifService, "", ""), cfg, true, false},
		{"already special ops", view("IN_PROGRESS", hoChoreoService, hoChoreoAPIM, ""), cfg, false, false},
		{"new", view("NEW", hoChoreoService, "", ""), cfg, false, false},
		{"other service", view("IN_PROGRESS", "22222222-2222-4222-8222-222222222222", "", ""), cfg, false, false},
		{"no service", view("IN_PROGRESS", "", "", ""), cfg, false, false},
		{"unknown team dropped", view("IN_PROGRESS", hoChoreoService, "", "sre-team"), cfg, true, false},
		{"no config", view("IN_PROGRESS", hoChoreoService, "", "choreo-apim-team"), nil, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := c.in
			svc := WithSpecialistHandoffConfig(NewIncidentService(&stubIncidentRepo{
				getIncidentByID: func(context.Context, string) (domain.IncidentView, error) { return in, nil },
			}, nil), c.cfg)
			got, err := svc.GetIncidentByID(context.Background(), hoIncidentID)
			if err != nil {
				t.Fatal(err)
			}
			if got.CanHandOffToSpecialist == nil || *got.CanHandOffToSpecialist != c.can {
				t.Errorf("canHandOffToSpecialist %v, want %v", got.CanHandOffToSpecialist, c.can)
			}
			if (got.SpecialistHandoff.EscalationTeam != nil) != c.keptTeam {
				t.Errorf("escalationTeam %v, want kept=%v", got.SpecialistHandoff.EscalationTeam, c.keptTeam)
			}
		})
	}
}

// TestSpecialistHandoff_GithubCredentials: each product files with the token
// its credential names (its owner by default), so repositories in different
// organisations, or one needing its own token, are configuration.
func TestSpecialistHandoff_GithubCredentials(t *testing.T) {
	cfg := hoConfig(t)
	if got := strings.Join(cfg.Credentials(), ","); got != "wso2-enterprise,asgardeo" {
		t.Errorf("credentials %q, want wso2-enterprise,asgardeo (Choreo by owner, Asgardeo by name, Moesif none)", got)
	}

	token, err := ParseSpecialistHandoffGithubTokens(`{"wso2-enterprise":"tok-org","asgardeo":" tok-asg "}`, "tok-fallback")
	if err != nil {
		t.Fatal(err)
	}
	if token("wso2-enterprise") != "tok-org" || token("asgardeo") != "tok-asg" || token("other") != "tok-fallback" {
		t.Errorf("tokens %q %q %q", token("wso2-enterprise"), token("asgardeo"), token("other"))
	}
	if none, _ := ParseSpecialistHandoffGithubTokens("", ""); none("wso2-enterprise") != "" {
		t.Error("no tokens and no fallback: want empty")
	}
	for _, raw := range []string{`["tok"]`, `{"":"tok"}`, `{"org":""}`} {
		if _, err := ParseSpecialistHandoffGithubTokens(raw, ""); err == nil || strings.Contains(err.Error(), "tok") && !strings.Contains(err.Error(), "TOKENS") {
			t.Errorf("%s: err %v, want an error that quotes no token", raw, err)
		}
	}

	org := &fakeHandoffIssues{result: &github.CreatedIssue{Number: 1, HTMLURL: "https://github.com/wso2-enterprise/choreo/issues/1"}}
	asg := &fakeHandoffIssues{result: &github.CreatedIssue{Number: 2, HTMLURL: "https://github.com/wso2-enterprise/asgardeo-product/issues/2"}}
	clients := SpecialistHandoffIssueClients{"wso2-enterprise": org, "asgardeo": asg}
	for _, c := range []struct {
		service string
		team    *domain.IncidentSpecialistHandoffEscalationTeam
		want    *fakeHandoffIssues
		repo    string
	}{
		{hoChoreoService, hoTeam("choreo-apim-team"), org, "choreo"},
		{hoAsgardeoService, nil, asg, "asgardeo-product"},
	} {
		org.calls, asg.calls = 0, 0
		repo := &stubIncidentRepo{
			applySpecialistHandoff: func(_ context.Context, _, _ string, plan func(repository.SpecialistHandoffSnapshot) (repository.SpecialistHandoffPlan, error)) (repository.SpecialistHandoffWritten, error) {
				snap := hoSnapshot(c.service, "", "IN_PROGRESS")
				if _, err := plan(snap); err != nil {
					return repository.SpecialistHandoffWritten{}, err
				}
				return repository.SpecialistHandoffWritten{Before: snap}, nil
			},
			createIncidentComment: func(context.Context, string, domain.CommentType, string, string) (domain.CaseComment, error) {
				return domain.CaseComment{}, nil
			},
			getIncidentByID: func(_ context.Context, id string) (domain.IncidentView, error) {
				return domain.IncidentView{ID: &id}, nil
			},
		}
		svc := WithHandoffIssueCreators(WithSpecialistHandoffConfig(NewIncidentService(repo, nil), cfg), clients)
		ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
		resp, err := svc.HandOffIncidentToSpecialist(ctx, domain.HandOffIncidentToSpecialistRequest{
			IncidentID: hoIncidentID, ReasonCode: domain.IncidentSpecialistHandoffReasonNoRunbook, EscalationTeam: c.team})
		if err != nil {
			t.Fatalf("%s: %v", c.repo, err)
		}
		if c.want.calls != 1 || org.calls+asg.calls != 1 || c.want.repo != c.repo || resp.Handoff.GithubIssue == nil {
			t.Errorf("%s: org %d asg %d calls, repo %q, issue %+v", c.repo, org.calls, asg.calls, c.want.repo, resp.Handoff.GithubIssue)
		}
	}
}
