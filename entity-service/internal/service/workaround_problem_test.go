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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// recordingWorkaroundCreator records what UpdateIncident asked for.
type recordingWorkaroundCreator struct {
	calls []WorkaroundProblemSource
	id    string
	err   error
}

func (c *recordingWorkaroundCreator) CreateWorkaroundProblem(_ context.Context, src WorkaroundProblemSource) (string, error) {
	c.calls = append(c.calls, src)
	return c.id, c.err
}

// workaroundIncident is the incident as GetIncidentByID returns it: before
// the resolve (first read), then after it.
func workaroundIncident(state, code string, problem *domain.EntityRef) domain.IncidentView {
	id, number, impact, urgency := testDeploymentUUID, "INC0099967", "HIGH", "MEDIUM"
	return domain.IncidentView{
		ID: &id, Number: &number, State: &state, ResolutionCode: &code,
		Service:         &domain.EntityRef{ID: "svc-1", Name: "client-medlineprod-alert-integration"},
		AssignmentGroup: &domain.EntityRef{ID: "grp-1", Name: "Choreo SRE Team"},
		Impact:          &impact, Urgency: &urgency, Problem: problem,
	}
}

// resolveWithCreator resolves the incident through the dual-write service,
// whose repository returns views in turn, and returns the mirror request.
func resolveWithCreator(t *testing.T, creator WorkaroundProblemCreator, views ...domain.IncidentView) (domain.UpdateIncidentResponse, domain.UpdateIncidentRequest) {
	t.Helper()
	reads := 0
	repo := &stubIncidentRepo{
		updateIncidentLifecycle: func(context.Context, string, repository.IncidentLifecycleUpdate, string) error { return nil },
		getIncidentByID: func(context.Context, string) (domain.IncidentView, error) {
			v := views[min(reads, len(views)-1)]
			reads++
			return v, nil
		},
	}
	mirrored := make(chan domain.UpdateIncidentRequest, 1)
	mirror := &stubMirrorIncidentService{
		updateIncident: func(_ context.Context, req domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error) {
			mirrored <- req
			return domain.UpdateIncidentResponse{}, nil
		},
	}
	svc := NewIncidentServiceWithSNMirror(repo, stubUpdateIncidentUserRepo{}, mirror, nil, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
	if creator != nil {
		svc = WithWorkaroundProblemCreator(svc, creator)
	}
	state, code := domain.IncidentStateResolved, domain.IncidentResolutionCodeSolvedWorkaround
	resp, err := svc.UpdateIncident(context.Background(), domain.UpdateIncidentRequest{ID: testDeploymentUUID, State: &state, ResolutionCode: &code})
	if err != nil {
		t.Fatalf("UpdateIncident: %v", err)
	}
	select {
	case req := <-mirrored:
		return resp, req
	case <-time.After(2 * time.Second):
		t.Fatal("the ServiceNow mirror was never called")
	}
	return resp, domain.UpdateIncidentRequest{}
}

// A resolve with a workaround creates the problem from the incident, links
// ServiceNow's incident to it through the mirror, and the response shows it.
func TestUpdateIncident_WorkaroundCreatesTheProblemInBothStores(t *testing.T) {
	problem := &domain.EntityRef{ID: "prb-1", Name: "PRB0040215"}
	creator := &recordingWorkaroundCreator{id: "prb-1"}
	resp, mirrored := resolveWithCreator(t, creator,
		workaroundIncident("IN_PROGRESS", "", nil),
		workaroundIncident("RESOLVED", "SOLVED_WORKAROUND", nil),
		workaroundIncident("RESOLVED", "SOLVED_WORKAROUND", problem))

	if len(creator.calls) != 1 {
		t.Fatalf("creator calls = %d, want 1", len(creator.calls))
	}
	got := creator.calls[0]
	if got.IncidentID != testDeploymentUUID || got.Number != "INC0099967" || strOrEmpty(got.ServiceID) != "svc-1" ||
		strOrEmpty(got.AssignmentGroupID) != "grp-1" {
		t.Errorf("creator source = %+v", got)
	}
	if strOrEmpty(mirrored.ProblemID) != "prb-1" {
		t.Errorf("mirrored problemId = %v, want prb-1", mirrored.ProblemID)
	}
	if resp.Incident.Problem == nil || resp.Incident.Problem.ID != "prb-1" {
		t.Errorf("response problem = %+v, want the new problem", resp.Incident.Problem)
	}
}

// No problem when the incident was already resolved, the code is not a
// workaround, or it already has a problem.
func TestUpdateIncident_WorkaroundProblemOnlyWhenNeeded(t *testing.T) {
	existing := &domain.EntityRef{ID: "prb-0"}
	for name, views := range map[string][]domain.IncidentView{
		"already resolved":   {workaroundIncident("RESOLVED", "SOLVED_WORKAROUND", nil), workaroundIncident("RESOLVED", "SOLVED_WORKAROUND", nil)},
		"solved permanently": {workaroundIncident("IN_PROGRESS", "", nil), workaroundIncident("RESOLVED", "SOLVED_PERMANENTLY", nil)},
		"has a problem":      {workaroundIncident("IN_PROGRESS", "", existing), workaroundIncident("RESOLVED", "SOLVED_WORKAROUND", existing)},
	} {
		creator := &recordingWorkaroundCreator{id: "prb-1"}
		_, mirrored := resolveWithCreator(t, creator, views...)
		if len(creator.calls) != 0 || mirrored.ProblemID != nil {
			t.Errorf("%s: creator calls %d, mirrored problemId %v, want none", name, len(creator.calls), mirrored.ProblemID)
		}
	}
}

// A failed create never undoes the resolve, and ServiceNow's incident is not
// pointed at a problem that does not exist.
func TestUpdateIncident_WorkaroundProblemFailureKeepsTheResolve(t *testing.T) {
	creator := &recordingWorkaroundCreator{err: errors.New("ServiceNow is down")}
	_, mirrored := resolveWithCreator(t, creator,
		workaroundIncident("IN_PROGRESS", "", nil),
		workaroundIncident("RESOLVED", "SOLVED_WORKAROUND", nil))
	if len(creator.calls) != 1 || mirrored.ProblemID != nil || mirrored.State == nil {
		t.Errorf("calls %d, mirrored %+v: want one attempt, the resolve mirrored, no problem link", len(creator.calls), mirrored)
	}
}

// CreateWorkaroundProblem: ServiceNow first (subject, primary incident; its
// id, number and priority), then Postgres gets the group and the link -- and
// nothing ServiceNow cannot hold -- then the stored group is mirrored.
func TestCreateWorkaroundProblem_ServiceNowFirstThenPostgres(t *testing.T) {
	var linkedGroup *string
	var linkedIncident string
	repo := &stubProblemRepo{
		createProblemFromServiceNow: func(_ context.Context, req domain.CreateProblemRequest, id, number, _ string, _ *string) (domain.ProblemDetail, error) {
			if id != "prb-1" || number != "PRB0040215" {
				t.Errorf("Postgres insert = %s %s, want ServiceNow's id and number", id, number)
			}
			return domain.ProblemDetail{ID: &id, Number: &number, Subject: &req.Subject}, nil
		},
		linkWorkaroundProblem: func(_ context.Context, problemID, incidentID string, groupID *string, _ string) (*string, error) {
			if problemID != "prb-1" {
				t.Errorf("linked problem = %s, want ServiceNow's id", problemID)
			}
			linkedGroup, linkedIncident = groupID, incidentID
			return groupID, nil
		},
	}
	var snCreate domain.CreateProblemRequest
	mirroredGroup := make(chan *string, 1)
	mirror := &stubMirrorProblemService{
		createProblem: func(_ context.Context, req domain.CreateProblemRequest) (domain.ProblemDetail, error) {
			snCreate = req
			id, number, priority := "prb-1", "PRB0040215", "5 - Planning"
			return domain.ProblemDetail{ID: &id, Number: &number, Priority: &priority}, nil
		},
		updateProblem: func(_ context.Context, req domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
			mirroredGroup <- req.AssignmentGroupID
			return domain.UpdateProblemResponse{}, nil
		},
	}
	svc := NewProblemServiceWithSNMirror(repo, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	id, err := svc.(WorkaroundProblemCreator).CreateWorkaroundProblem(ctx, WorkaroundProblemSource{
		IncidentID: "inc-1", Number: "INC0099967", ServiceID: strPtr(postResolutionServiceChoreo), AssignmentGroupID: strPtr("grp-1"),
	})
	if err != nil || id != "prb-1" {
		t.Fatalf("CreateWorkaroundProblem = %q, %v", id, err)
	}
	if snCreate.Subject != "Fix the root cause of INC0099967" || strOrEmpty(snCreate.PrimaryIncidentID) != "inc-1" {
		t.Errorf("ServiceNow create = %+v", snCreate)
	}
	if repo.lastCreatePriority != "PLANNING" {
		t.Errorf("Postgres priority = %q, want ServiceNow's (Planning)", repo.lastCreatePriority)
	}
	if linkedIncident != "inc-1" || strOrEmpty(linkedGroup) != groupChoreoSpecialOps {
		t.Errorf("link = incident %s group %v, want inc-1 and Choreo Special Ops", linkedIncident, linkedGroup)
	}
	select {
	case g := <-mirroredGroup:
		if strOrEmpty(g) != groupChoreoSpecialOps {
			t.Errorf("mirrored group = %v, want Choreo Special Ops", g)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the group was never mirrored to ServiceNow")
	}
}

// A group Postgres lacks is stored as none, so none is mirrored either.
func TestCreateWorkaroundProblem_GroupPostgresLacksIsNotMirrored(t *testing.T) {
	repo := &stubProblemRepo{
		createProblemFromServiceNow: func(_ context.Context, req domain.CreateProblemRequest, id, number, _ string, _ *string) (domain.ProblemDetail, error) {
			return domain.ProblemDetail{ID: &id, Number: &number}, nil
		},
		linkWorkaroundProblem: func(context.Context, string, string, *string, string) (*string, error) { return nil, nil },
	}
	mirror := &stubMirrorProblemService{
		createProblem: func(context.Context, domain.CreateProblemRequest) (domain.ProblemDetail, error) {
			id, number := "prb-1", "PRB0040215"
			return domain.ProblemDetail{ID: &id, Number: &number}, nil
		},
		updateProblem: func(context.Context, domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
			t.Error("no group was stored, so nothing must be mirrored")
			return domain.UpdateProblemResponse{}, nil
		},
	}
	svc := NewProblemServiceWithSNMirror(repo, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.(WorkaroundProblemCreator).CreateWorkaroundProblem(ctx, WorkaroundProblemSource{
		IncidentID: "inc-1", Number: "INC0099967", AssignmentGroupID: strPtr("grp-missing"),
	}); err != nil {
		t.Fatalf("CreateWorkaroundProblem: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
}

// DATA_SOURCE=postgres has no ServiceNow; the background flow creates the
// problem there, so this path refuses.
func TestCreateWorkaroundProblem_PostgresOnlyRefuses(t *testing.T) {
	svc := NewProblemService(&stubProblemRepo{})
	if _, err := svc.(WorkaroundProblemCreator).CreateWorkaroundProblem(context.Background(), WorkaroundProblemSource{}); !errors.Is(err, errWorkaroundProblemNeedsServiceNow) {
		t.Errorf("err = %v, want errWorkaroundProblemNeedsServiceNow", err)
	}
}
