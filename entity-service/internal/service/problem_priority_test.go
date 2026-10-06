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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// ServiceNow returns a new problem's priority as its display value; every
// new problem there is "5 - Planning" (discovery script 63), which is also
// the fallback for a missing or unrecognised value.
func TestProblemPriorityFromServiceNow(t *testing.T) {
	sp := func(s string) *string { return &s }
	for in, want := range map[*string]string{
		sp("1 - Critical"): "CRITICAL",
		sp("2 - High"):     "HIGH",
		sp("3 - Moderate"): "MODERATE",
		sp("4 - Low"):      "LOW",
		sp("5 - Planning"): "PLANNING",
		sp("3"):            "MODERATE",
		sp(" 2 - High "):   "HIGH",
		sp(""):             "PLANNING",
		sp("Urgent"):       "PLANNING",
		nil:                "PLANNING",
	} {
		label := "<nil>"
		if in != nil {
			label = *in
		}
		if got := problemPriorityFromServiceNow(in); got != want {
			t.Errorf("%q -> %q, want %q", label, got, want)
		}
	}
}

// Dual-write: the Postgres copy gets the priority ServiceNow gave the
// problem, not a blank.
func TestCreateProblem_DualWriteStoresServiceNowsPriority(t *testing.T) {
	id, number, prio := "11111111-2222-4333-8444-555555555555", "PRB0040215", "5 - Planning"
	mirror := &stubMirrorProblemService{
		createProblem: func(context.Context, domain.CreateProblemRequest) (domain.ProblemDetail, error) {
			return domain.ProblemDetail{ID: &id, Number: &number, Priority: &prio}, nil
		},
	}
	repo := &stubProblemRepo{
		createProblemFromServiceNow: func(_ context.Context, _ domain.CreateProblemRequest, id, number, _ string, _ *string) (domain.ProblemDetail, error) {
			return domain.ProblemDetail{ID: &id, Number: &number}, nil
		},
	}
	svc := NewProblemServiceWithSNMirror(repo, mirror, nil)
	if _, err := svc.CreateProblem(userCtxProblem(t), validCreateProblemRequest()); err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}
	if repo.lastCreatePriority != "PLANNING" {
		t.Errorf("Postgres got priority %q, want PLANNING", repo.lastCreatePriority)
	}
}

// ServiceNow's priority lookup, the same nine rows for incidents
// (dl_u_priority) and problems (dl_problem_priority) -- discovery script 65.
func TestPriorityFromImpactUrgency(t *testing.T) {
	want := map[[2]string]string{
		{"HIGH", "HIGH"}: "CRITICAL", {"HIGH", "MEDIUM"}: "HIGH", {"HIGH", "LOW"}: "MODERATE",
		{"MEDIUM", "HIGH"}: "HIGH", {"MEDIUM", "MEDIUM"}: "MODERATE", {"MEDIUM", "LOW"}: "LOW",
		{"LOW", "HIGH"}: "MODERATE", {"LOW", "MEDIUM"}: "LOW", {"LOW", "LOW"}: "PLANNING",
	}
	for in, p := range want {
		if got := priorityFromImpactUrgency(in[0], in[1]); got != p {
			t.Errorf("impact %s x urgency %s -> %s, want %s", in[0], in[1], got, p)
		}
	}
	if f := newProblemPriorityFields(); f != (repository.ProblemPriorityFields{Priority: "PLANNING", Impact: "LOW", Urgency: "LOW"}) {
		t.Errorf("a new problem gets %+v, want ServiceNow's LOW x LOW -> PLANNING", f)
	}
}

// The post-resolution problem's priority is derived, as ServiceNow's problem
// lookup overwrites the copied one -- so an incident with no priority still
// gives its problem one.
func TestProblemFor_DerivesPriority(t *testing.T) {
	src := repository.IncidentReportSource{IncidentID: "inc", Number: "INC0001"}
	m, h := "MEDIUM", "HIGH"
	src.Impact, src.Urgency = &m, &h // Priority deliberately nil
	p := problemFor(src)
	if strOrEmpty(p.Priority) != "HIGH" || strOrEmpty(p.Impact) != "MEDIUM" || strOrEmpty(p.Urgency) != "HIGH" {
		t.Errorf("priority/impact/urgency = %s/%s/%s, want HIGH/MEDIUM/HIGH", strOrEmpty(p.Priority), strOrEmpty(p.Impact), strOrEmpty(p.Urgency))
	}
	src.Impact, src.Urgency = nil, nil
	if p := problemFor(src); strOrEmpty(p.Priority) != "PLANNING" {
		t.Errorf("no impact/urgency: priority %s, want PLANNING (ServiceNow's 3 x 3 default)", strOrEmpty(p.Priority))
	}
}
