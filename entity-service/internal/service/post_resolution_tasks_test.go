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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// resolvedOn returns a source on service with the given resolution code.
func resolvedOn(service, code string) repository.IncidentReportSource {
	src := sampleIncidentSource()
	src.ServiceID = strPtr(service)
	src.ResolutionCode = strPtr(code)
	src.Impact = strPtr("HIGH")
	src.Urgency = strPtr("MEDIUM")
	return src
}

func resolve(t *testing.T, src repository.IncidentReportSource) *fakeIncidentReportTx {
	t.Helper()
	tx := &fakeIncidentReportTx{src: src}
	if err := newTestIncidentReportService(time.Now()).HandleChange(context.Background(), tx,
		incidentStateChange("IN_PROGRESS", "RESOLVED", time.Now())); err != nil {
		t.Fatalf("HandleChange: %v", err)
	}
	return tx
}

// Blocks 2-7: one alert task per alert close code, with ServiceNow's exact
// subject, priority and group. "Duplicate" arrives under both spellings.
func TestPostResolution_AlertTasks(t *testing.T) {
	for _, c := range []struct{ code, subject, priority string }{
		{"FALSE_ALARM", "[Alert Task][Falser Alarm] INC0012345 alert is a false alarm", "CRITICAL"},
		{"DUPLICATE_ALERT", "[Alert Task][Duplicate Alert] INC0012345 alert is a duplicate", "CRITICAL"},
		{"DUPLICATE", "[Alert Task][Duplicate Alert] INC0012345 alert is a duplicate", "CRITICAL"},
		{"NOT_ACTIONABLE_ALERT", "[Alert Task][Not Actionable Alert] INC0012345 is not an actionable alert", "HIGH"},
	} {
		t.Run(c.code, func(t *testing.T) {
			tx := resolve(t, resolvedOn(postResolutionServiceChoreo, c.code))
			if len(tx.tasks) != 1 {
				t.Fatalf("tasks = %d, want 1", len(tx.tasks))
			}
			got := tx.tasks[0]
			if got.Subject != c.subject || got.Priority != c.priority {
				t.Errorf("task = %q %s, want %q %s", got.Subject, got.Priority, c.subject, c.priority)
			}
			if strOrEmpty(got.AssignmentGroupID) != groupWSO2SRETeam || strOrEmpty(got.ServiceID) != postResolutionServiceChoreo ||
				got.IncidentID != incidentReportTestID || got.AssignedToID != nil {
				t.Errorf("task fields = %+v", got)
			}
			if len(tx.problems) != 0 {
				t.Errorf("an alert close code must not create a problem")
			}
		})
	}
}

// Blocks 8-14: a workaround with no problem creates one, copies the
// incident's service/impact/urgency/priority, links it, and picks the group
// by service -- on any service, not only the flow's two: another service's
// problem takes the incident's own group.
func TestPostResolution_WorkaroundCreatesAndLinksAProblem(t *testing.T) {
	const otherService = "11111111-1111-4111-8111-111111111111"
	for service, group := range map[string]string{
		postResolutionServiceChoreo:   groupChoreoSpecialOps,
		postResolutionServiceAsgardeo: groupAsgardeoOperationsTeam,
		otherService:                  "grp",
	} {
		tx := resolve(t, resolvedOn(service, "SOLVED_WORK_AROUND"))
		if len(tx.problems) != 1 {
			t.Fatalf("%s: problems = %d, want 1", service, len(tx.problems))
		}
		p := tx.problems[0]
		if p.Subject != "Fix the root cause of INC0012345" || strOrEmpty(p.ServiceID) != service ||
			strOrEmpty(p.Priority) != "HIGH" || strOrEmpty(p.Impact) != "HIGH" || strOrEmpty(p.Urgency) != "MEDIUM" ||
			p.IncidentID != incidentReportTestID || strOrEmpty(p.AssignmentGroupID) != group {
			t.Errorf("%s: problem = %+v", service, p)
		}
		if tx.links[incidentReportTestID] != "problem-id" {
			t.Errorf("%s: incident not linked to the new problem: %v", service, tx.links)
		}
		if len(tx.tasks) != 0 {
			t.Errorf("%s: a workaround must not create an alert task", service)
		}
	}
}

// Another service's incident with no group, or no service at all, still gets
// its problem -- unassigned, or with the incident's group.
func TestPostResolution_WorkaroundProblemWithoutServiceOrGroup(t *testing.T) {
	noGroup := resolvedOn("11111111-1111-4111-8111-111111111111", "SOLVED_WORK_AROUND")
	noGroup.AssignmentGroupID = nil
	if tx := resolve(t, noGroup); len(tx.problems) != 1 || tx.problems[0].AssignmentGroupID != nil {
		t.Errorf("no group: problems = %+v, want one, unassigned", tx.problems)
	}

	noService := resolvedOn("", "SOLVED_WORK_AROUND")
	noService.ServiceID = nil
	tx := resolve(t, noService)
	if len(tx.problems) != 1 || tx.problems[0].ServiceID != nil || strOrEmpty(tx.problems[0].AssignmentGroupID) != "grp" {
		t.Errorf("no service: problems = %+v, want one with no service and the incident's group", tx.problems)
	}
}

// Block 8's second half: an incident that already has a problem gets no new one.
func TestPostResolution_ExistingProblemIsKept(t *testing.T) {
	src := resolvedOn(postResolutionServiceChoreo, "SOLVED_WORK_AROUND")
	src.ProblemID = strPtr("existing-problem")
	tx := resolve(t, src)
	if len(tx.problems) != 0 || len(tx.links) != 0 {
		t.Errorf("problems = %v links = %v, want none", tx.problems, tx.links)
	}
}

// The trigger: alert tasks only for Choreo and Asgardeo incidents, and other
// close codes do nothing. The incident report is still written for every
// service.
func TestPostResolution_OnlyTheTwoServicesAndCodes(t *testing.T) {
	for name, src := range map[string]repository.IncidentReportSource{
		"other service, false alarm": resolvedOn("11111111-1111-4111-8111-111111111111", "FALSE_ALARM"),
		"other service, duplicate":   resolvedOn("11111111-1111-4111-8111-111111111111", "DUPLICATE"),
		"Choreo, solved permanently": resolvedOn(postResolutionServiceChoreo, "SOLVED_PERMANENTLY"),
		"Choreo, no service": func() repository.IncidentReportSource {
			s := resolvedOn("", "FALSE_ALARM")
			s.ServiceID = nil
			return s
		}(),
	} {
		tx := resolve(t, src)
		if len(tx.tasks) != 0 || len(tx.problems) != 0 {
			t.Errorf("%s: tasks %v problems %v, want none", name, tx.tasks, tx.problems)
		}
		if tx.reports[incidentReportTestID] == "" {
			t.Errorf("%s: the incident report must still be written", name)
		}
	}
}

// In Progress runs the report task only, never the post-resolution blocks.
func TestPostResolution_NotOnInProgress(t *testing.T) {
	tx := &fakeIncidentReportTx{src: resolvedOn(postResolutionServiceChoreo, "FALSE_ALARM")}
	if err := newTestIncidentReportService(time.Now()).HandleChange(context.Background(), tx,
		incidentStateChange("NEW", "IN_PROGRESS", time.Now())); err != nil {
		t.Fatalf("HandleChange: %v", err)
	}
	if len(tx.tasks) != 0 || len(tx.problems) != 0 {
		t.Errorf("tasks %v problems %v, want none", tx.tasks, tx.problems)
	}
}

// A failed write fails the change, so the whole transaction (report
// included) rolls back and is retried.
func TestPostResolution_WriteErrorsPropagate(t *testing.T) {
	boom := errors.New("boom")
	tx := &fakeIncidentReportTx{src: resolvedOn(postResolutionServiceChoreo, "SOLVED_WORK_AROUND"), createErr: boom}
	err := newTestIncidentReportService(time.Now()).HandleChange(context.Background(), tx,
		incidentStateChange("IN_PROGRESS", "RESOLVED", time.Now()))
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want boom", err)
	}
}

// Dual-write: the resolve request creates the workaround problem in both
// stores, so the flow writes the report and alert tasks but no problem.
func TestPostResolution_DualWriteLeavesTheProblemToTheRequest(t *testing.T) {
	for _, src := range []repository.IncidentReportSource{
		resolvedOn(postResolutionServiceChoreo, "SOLVED_WORK_AROUND"),
		resolvedOn("11111111-1111-4111-8111-111111111111", "SOLVED_WORK_AROUND"),
	} {
		tx := &fakeIncidentReportTx{src: src}
		if err := NewDualWriteIncidentReportService().HandleChange(context.Background(), tx,
			incidentStateChange("IN_PROGRESS", "RESOLVED", time.Now())); err != nil {
			t.Fatalf("HandleChange: %v", err)
		}
		if len(tx.problems) != 0 || len(tx.links) != 0 {
			t.Errorf("problems %v links %v, want none in dual-write", tx.problems, tx.links)
		}
		if tx.reports[incidentReportTestID] == "" {
			t.Errorf("the incident report must still be written")
		}
	}
	tx := &fakeIncidentReportTx{src: resolvedOn(postResolutionServiceChoreo, "FALSE_ALARM")}
	if err := NewDualWriteIncidentReportService().HandleChange(context.Background(), tx,
		incidentStateChange("IN_PROGRESS", "RESOLVED", time.Now())); err != nil || len(tx.tasks) != 1 {
		t.Errorf("alert task in dual-write: tasks %d, err %v, want 1", len(tx.tasks), err)
	}
}
