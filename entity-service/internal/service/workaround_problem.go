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
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The workaround problem in DATA_SOURCE=postgres-servicenow-dual-write.
//
// Resolving an incident as Solved (Workaround) creates a problem for it (the
// post-resolution flow, blocks 8-14). Under DATA_SOURCE=postgres the
// background flow creates it in Postgres. In dual-write it has to exist in
// ServiceNow too: every problem transition there is ServiceNow-first, by id,
// and the number has to be ServiceNow's PRB number. The background flow has
// no user and the CSM API needs the caller's x-user-id-token, so in this mode
// the resolve request creates the problem itself, ServiceNow-first like the
// portal's Create problem, and the flow skips it
// (NewDualWriteIncidentReportService). Reads stay on Postgres.

// WorkaroundProblemSource is the resolved incident a workaround problem is
// created for: its service and group decide the problem's group.
type WorkaroundProblemSource struct {
	IncidentID, Number           string
	ServiceID, AssignmentGroupID *string
}

// WorkaroundProblemCreator creates the workaround problem for a just-resolved
// incident and links the incident to it, returning the problem's id.
type WorkaroundProblemCreator interface {
	CreateWorkaroundProblem(ctx context.Context, src WorkaroundProblemSource) (string, error)
}

// errWorkaroundProblemNeedsServiceNow: CreateWorkaroundProblem is the
// dual-write path only; DATA_SOURCE=postgres uses the background flow.
var errWorkaroundProblemNeedsServiceNow = errors.New("workaround problem: only DATA_SOURCE=postgres-servicenow-dual-write creates it in the request")

// CreateWorkaroundProblem implements WorkaroundProblemCreator: the problem
// is created ServiceNow-first (createProblemSNFirst: ServiceNow's id and PRB
// number, then the Postgres row with the same values), then its group is set
// and the incident linked to it in Postgres. The group is mirrored to
// ServiceNow like any problem field edit, and UpdateIncident mirrors the
// incident's link, so both stores hold the same problem.
func (s *problemService) CreateWorkaroundProblem(ctx context.Context, src WorkaroundProblemSource) (string, error) {
	if s.snMirror == nil {
		return "", errWorkaroundProblemNeedsServiceNow
	}
	f := workaroundProblemFields(src.Number, src.ServiceID, src.AssignmentGroupID, nil, nil)
	incidentID := src.IncidentID
	created, err := s.createProblemSNFirst(ctx, domain.CreateProblemRequest{Subject: f.Subject, PrimaryIncidentID: &incidentID})
	if err != nil {
		return "", err
	}
	id := strOrEmpty(created.ID)
	actor, err := s.resolveActorEmail(ctx)
	if err != nil {
		actor = incidentReportActor
	}
	// Only what ServiceNow holds too: its id, number, priority and default
	// impact/urgency came with the create; the group and the incident link
	// are set here and mirrored. Its API takes no service, impact or urgency
	// (discovery script 72), so neither store gets the incident's.
	group, err := s.repo.LinkWorkaroundProblem(ctx, id, incidentID, f.AssignmentGroupID, actor)
	if err != nil {
		// The problem exists in both stores; only the group and the link are missing.
		slog.ErrorContext(ctx, "workaround problem: created, but linking it in Postgres failed",
			"problemId", id, "problemNumber", strOrEmpty(created.Number), "incidentId", incidentID, "error", err)
		return id, err
	}
	if group != nil {
		s.mirrorProblemFields(ctx, domain.UpdateProblemRequest{ID: id, AssignmentGroupID: group}, nil)
	}
	slog.InfoContext(ctx, "workaround problem: created",
		"incidentId", incidentID, "problemId", id, "problemNumber", strOrEmpty(created.Number))
	return id, nil
}

// WithWorkaroundProblemCreator gives a dual-write incident service the
// problem service that creates the workaround problem on resolve. Any other
// IncidentService is returned unchanged.
func WithWorkaroundProblemCreator(svc IncidentService, creator WorkaroundProblemCreator) IncidentService {
	if pg, ok := svc.(*incidentService); ok {
		pg.workaroundProblems = creator
	}
	return svc
}

// createWorkaroundProblem is UpdateIncident's dual-write step: when this
// request moved the incident to Resolved as Solved (Workaround) and it has
// no problem, create one in both stores. before is the incident ahead of the
// change (empty if it could not be read), view after it. Returns the new
// problem's id, or "" when there is nothing to do or the create failed -- a
// failure is logged and never undoes the resolve, as the background flow's
// problem never blocked one either.
func (s *incidentService) createWorkaroundProblem(ctx context.Context, req domain.UpdateIncidentRequest, before, view domain.IncidentView) string {
	if s.workaroundProblems == nil || req.State == nil {
		return ""
	}
	resolved := string(domain.IncidentStateResolved)
	if strOrEmpty(view.State) != resolved || strOrEmpty(before.State) == resolved ||
		strOrEmpty(view.ResolutionCode) != string(domain.IncidentResolutionCodeSolvedWorkaround) || view.Problem != nil {
		return ""
	}
	src := WorkaroundProblemSource{IncidentID: req.ID, Number: strOrEmpty(view.Number)}
	if view.Service != nil {
		src.ServiceID = &view.Service.ID
	}
	if view.AssignmentGroup != nil {
		src.AssignmentGroupID = &view.AssignmentGroup.ID
	}
	id, err := s.workaroundProblems.CreateWorkaroundProblem(ctx, src)
	if err != nil || id == "" {
		slog.ErrorContext(ctx, "update incident: resolved with a workaround, but its problem was not created",
			"incidentId", req.ID, "error", err)
		return ""
	}
	return id
}
