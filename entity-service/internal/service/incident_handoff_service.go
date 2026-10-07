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
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The specialist handoff on Postgres ports ServiceNow's
// x_wso2_customer_0.IncidentHandoffUtils.handOff, which mirrors the
// "Escalate to Special Ops" UI action (sys_ui_action
// 1b8e482c1b0b3010d64e64a2604bcb51). It writes Postgres only; ServiceNow is
// never called. Discovery script 68 (csm-flow-service docs) has the UI
// action and its history.

// The routing -- which Special Ops teams serve which services, each team's
// assignment group, and where the GitHub issue goes -- is configuration
// (SpecialistHandoffConfig, SPECIALIST_HANDOFF_CONFIG). ServiceNow hard-codes
// it in IncidentHandoffUtils' IHU_SERVICE_ROUTING.

// maxEscalationTeamLen bounds a team key, in the request and the config.
const maxEscalationTeamLen = 64

// handoffReasons are the two reasons the UI action's modal offers: the
// description written into the reason note, and the runbook task's subject.
var handoffReasons = map[domain.IncidentSpecialistHandoffReasonCode]struct {
	description string
	taskSubject func(number string) string
}{
	domain.IncidentSpecialistHandoffReasonNoRunbook: {
		description: "Runbook is not available",
		taskSubject: func(n string) string { return "[Runbook Task] No entry available for " + n },
	},
	domain.IncidentSpecialistHandoffReasonRunbookNotWorking: {
		description: "Runbook doesn't solve the incident",
		taskSubject: func(n string) string { return "[Runbook Task] Entry didn't solve the incident " + n },
	},
}

// HandoffIssueCreator is the slice of the GitHub client a handoff needs.
type HandoffIssueCreator interface {
	CreateIssue(ctx context.Context, owner, repository, title, body string, labels []string) (*github.CreatedIssue, error)
}

// SpecialistHandoffIssueClients are the GitHub clients handoffs file issues
// with, by credential name.
type SpecialistHandoffIssueClients map[string]HandoffIssueCreator

// WithHandoffIssueCreators gives a Postgres-backed IncidentService the
// GitHub clients its specialist handoffs file issues with, by credential
// name (SpecialistHandoffGithub.Credential). A ServiceNow-backed service is
// left as it is.
func WithHandoffIssueCreators(svc IncidentService, clients SpecialistHandoffIssueClients) IncidentService {
	if pg, ok := svc.(*incidentService); ok {
		pg.handoffIssues = clients
	}
	return svc
}

// WithSpecialistHandoffConfig gives a Postgres-backed IncidentService its
// specialist handoff routing. Without one no incident can be handed off. A
// ServiceNow-backed service, whose routing is ServiceNow's, is left as it is.
func WithSpecialistHandoffConfig(svc IncidentService, cfg *SpecialistHandoffConfig) IncidentService {
	if pg, ok := svc.(*incidentService); ok {
		pg.handoffConfig = cfg
	}
	return svc
}

// specialistHandoffConflict is IncidentHandoffUtils' 409 for an incident the
// handoff cannot take.
func specialistHandoffConflict(detail string) error {
	return &apierror.ConflictError{Msg: "Incident is not eligible for a specialist handoff: " + detail}
}

// planSpecialistHandoff is IncidentHandoffUtils.checkEligibility plus the
// writes handOff makes, decided on the locked incident: the incident's
// service must belong to a configured product, the incident must be In
// Progress and not already with one of that product's Special Ops groups.
// A product with several teams needs req's escalationTeam to name one; a
// product with one team takes it, and the reason note records no team, as
// ServiceNow's does for Asgardeo. It returns the product, for the GitHub
// issue, and the request as it is recorded.
//
// The runbook task goes to the same Special Ops group as the incident.
// ServiceNow sends it to WSO2 SRE Team, which no longer exists; the Special
// Ops team now owns its runbooks.
func planSpecialistHandoff(cfg *SpecialistHandoffConfig, req domain.HandOffIncidentToSpecialistRequest, snap repository.SpecialistHandoffSnapshot) (repository.SpecialistHandoffPlan, *SpecialistHandoffProduct, domain.HandOffIncidentToSpecialistRequest, error) {
	fail := func(err error) (repository.SpecialistHandoffPlan, *SpecialistHandoffProduct, domain.HandOffIncidentToSpecialistRequest, error) {
		return repository.SpecialistHandoffPlan{}, nil, req, err
	}
	product := cfg.productFor(snap.ServiceID)
	if product == nil {
		return fail(specialistHandoffConflict("No specialist group is configured for this incident's service."))
	}
	if snap.State != string(domain.IncidentStateInProgress) {
		return fail(specialistHandoffConflict("Only an In Progress incident can be handed off."))
	}
	if product.holdsGroup(snap.AssignmentGroupID) {
		return fail(specialistHandoffConflict("The incident already sits with a " + product.Name + " specialist group."))
	}

	var team *SpecialistHandoffConfigTeam
	if len(product.Teams) == 1 {
		team, req.EscalationTeam = &product.Teams[0], nil
	} else {
		keys := make([]string, 0, len(product.Teams))
		for _, t := range product.Teams {
			keys = append(keys, t.Key)
		}
		if req.EscalationTeam == nil {
			return fail(&apierror.ValidationError{Msg: "escalationTeam is required for a " + product.Name + " incident: one of " + strings.Join(keys, ", ")})
		}
		if team = product.team(string(*req.EscalationTeam)); team == nil {
			return fail(&apierror.ValidationError{Msg: "invalid escalationTeam for a " + product.Name + " incident: " + string(*req.EscalationTeam) + " (one of " + strings.Join(keys, ", ") + ")"})
		}
	}

	reason := handoffReasons[req.ReasonCode]
	blob, err := handoffReasonBlob(req, reason.description)
	if err != nil {
		return fail(err)
	}
	groupID := team.GroupID
	return repository.SpecialistHandoffPlan{
		GroupID:     groupID,
		TaskSubject: reason.taskSubject(snap.Number),
		TaskGroupID: &groupID,
		WorkNotes:   []string{blob},
	}, product, req, nil
}

// handoffReasonBlob is the reason work note, byte for byte the JSON the UI
// action's modal writes: {"reasonCode":...,"reasonDescription":...,
// "escalationTeam":...}, escalationTeam null when none was picked. HTML
// escaping is off because JavaScript's JSON.stringify does none.
func handoffReasonBlob(req domain.HandOffIncidentToSpecialistRequest, description string) (string, error) {
	var team *string
	if req.EscalationTeam != nil {
		t := string(*req.EscalationTeam)
		team = &t
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(struct {
		ReasonCode        string  `json:"reasonCode"`
		ReasonDescription string  `json:"reasonDescription"`
		EscalationTeam    *string `json:"escalationTeam"`
	}{string(req.ReasonCode), description, team}); err != nil {
		return "", fmt.Errorf("specialist handoff: encode reason: %w", err)
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// handoffEscalatedNote is the note handOff writes after the GitHub attempt,
// with the issue link when one was opened.
func handoffEscalatedNote(actorLabel string, issue *domain.IncidentSpecialistHandoffGithubIssue) string {
	note := "Escalated to Special Ops team. Escalated by " + actorLabel
	if issue != nil {
		note += " Opened an internal issue. Please access the ticket using the link " + issue.URL +
			" to add more details to the ticket if needed"
	}
	return note
}

// handoffActor is who performs the handoff: their email from the caller's
// token, and the "Name(email)" label the notes carry -- the user's name when
// this database knows them, the email otherwise, as IncidentHandoffUtils'
// _actorLabel does.
func (s *incidentService) handoffActor(ctx context.Context) (email, label string, err error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return "", "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	email, err = emailFromJWT(token)
	if err != nil {
		return "", "", &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	name := email
	if s.userRepo != nil {
		if u, uerr := s.userRepo.GetUserByEmail(ctx, email); uerr == nil {
			if n := strings.TrimSpace(u.FirstName + " " + u.LastName); n != "" {
				name = n
			}
		}
	}
	return email, name + "(" + email + ")", nil
}

// HandOffIncidentToSpecialist implements IncidentService for Postgres.
//
// In one transaction it applies IncidentHandoffUtils' eligibility rules
// (planSpecialistHandoff), moves the incident to the Special Ops team's
// group, clears its assignee, opens the runbook task and writes the reason
// note. Then, outside the transaction and best effort -- the handoff stands
// whatever GitHub does, as in IncidentHandoffUtils -- it files the internal
// GitHub issue and writes the "Escalated to Special Ops team." note.
func (s *incidentService) HandOffIncidentToSpecialist(ctx context.Context, req domain.HandOffIncidentToSpecialistRequest) (domain.HandOffIncidentToSpecialistResponse, error) {
	if err := validateHandOffRequest(req); err != nil {
		return domain.HandOffIncidentToSpecialistResponse{}, err
	}
	// Whether the key is one of the product's teams is known only once the
	// incident's service is read; here, only its shape.
	if req.EscalationTeam != nil {
		if t := strings.TrimSpace(string(*req.EscalationTeam)); t == "" || len(t) > maxEscalationTeamLen {
			return domain.HandOffIncidentToSpecialistResponse{}, &apierror.ValidationError{Msg: "invalid escalationTeam: " + string(*req.EscalationTeam)}
		}
	}
	email, label, err := s.handoffActor(ctx)
	if err != nil {
		return domain.HandOffIncidentToSpecialistResponse{}, err
	}

	var product *SpecialistHandoffProduct
	written, err := s.repo.ApplySpecialistHandoff(ctx, req.IncidentID, email,
		func(snap repository.SpecialistHandoffSnapshot) (repository.SpecialistHandoffPlan, error) {
			plan, p, recorded, perr := planSpecialistHandoff(s.handoffConfig, req, snap)
			product, req = p, recorded
			return plan, perr
		})
	if err != nil {
		return domain.HandOffIncidentToSpecialistResponse{}, err
	}

	result := domain.IncidentSpecialistHandoffResult{
		ReasonCode:        req.ReasonCode,
		ReasonDescription: handoffReasons[req.ReasonCode].description,
		EscalationTeam:    req.EscalationTeam,
		Task: domain.IncidentSpecialistHandoffTask{
			ID: written.TaskID, Number: written.TaskNumber, Subject: handoffReasons[req.ReasonCode].taskSubject(written.Before.Number),
		},
	}
	if written.Before.AssignmentGroupID != nil {
		result.PreviousAssignmentGroup = &domain.EntityRef{ID: *written.Before.AssignmentGroupID, Name: derefString(written.Before.AssignmentGroupName)}
	}

	if req.CreateGithubIssue == nil || *req.CreateGithubIssue {
		result.GithubIssue, result.GithubIssueError = s.fileHandoffIssue(ctx, product, written.Before)
	}
	if _, err := s.repo.CreateIncidentComment(ctx, req.IncidentID, domain.CommentTypeWorkNote, handoffEscalatedNote(label, result.GithubIssue), email); err != nil {
		// The handoff itself is committed; only this note is missing.
		slog.ErrorContext(ctx, "specialist handoff: escalated note not written", "incidentId", req.IncidentID, "error", err)
	}

	view, err := s.incidentView(ctx, req.IncidentID)
	if err != nil {
		return domain.HandOffIncidentToSpecialistResponse{}, err
	}
	result.Incident = view
	if view.AssignmentGroup != nil {
		result.AssignmentGroup = *view.AssignmentGroup
	}
	return domain.HandOffIncidentToSpecialistResponse{
		Message: "Incident handed off to the specialist group.",
		Handoff: result,
	}, nil
}

// fileHandoffIssue opens the internal GitHub issue in the product's
// repository, with the token its credential names, titled and bodied with
// the incident's subject and description. It never fails the handoff: a
// product with no repository, a credential with no token, or a GitHub error
// comes back as the error text.
func (s *incidentService) fileHandoffIssue(ctx context.Context, product *SpecialistHandoffProduct, inc repository.SpecialistHandoffSnapshot) (*domain.IncidentSpecialistHandoffGithubIssue, *string) {
	if product == nil || product.Github == nil {
		msg := "No GitHub repository is configured for this product's specialist handoffs"
		return nil, &msg
	}
	owner, repo := product.Github.Owner, product.Github.Repo
	client := s.handoffIssues[product.Github.credential()]
	if client == nil {
		msg := "GitHub issue creation is not configured on this deployment"
		return nil, &msg
	}
	created, err := client.CreateIssue(ctx, owner, repo, inc.Subject, derefString(inc.Description), nil)
	if err != nil {
		slog.WarnContext(ctx, "specialist handoff: GitHub issue not created", "incidentId", inc.IncidentID, "error", err)
		// IncidentHandoffUtils' wording: the status GitHub answered with, or
		// the failure when there was no answer.
		msg := "GitHub issue creation failed: " + err.Error()
		if apiErr := (*github.Error)(nil); errors.As(err, &apiErr) {
			msg = fmt.Sprintf("GitHub issue creation failed (%d)", apiErr.StatusCode)
		}
		return nil, &msg
	}
	return &domain.IncidentSpecialistHandoffGithubIssue{URL: created.HTMLURL, Number: created.Number, Repo: repo}, nil
}

// ListSpecialistHandoffTeams implements IncidentService for Postgres: the
// teams of the product serviceID belongs to, in configured order; none for
// a service no product covers. An empty serviceID lists every product's.
func (s *incidentService) ListSpecialistHandoffTeams(_ context.Context, serviceID string) (domain.SpecialistHandoffTeamsResponse, error) {
	teams := []domain.SpecialistHandoffTeam{}
	if serviceID == "" {
		if s.handoffConfig != nil {
			for i := range s.handoffConfig.Products {
				teams = append(teams, s.handoffConfig.Products[i].teamOptions()...)
			}
		}
		return domain.SpecialistHandoffTeamsResponse{Teams: teams}, nil
	}
	if err := validateUUIDs("serviceId", []string{serviceID}); err != nil {
		return domain.SpecialistHandoffTeamsResponse{}, err
	}
	if p := s.handoffConfig.productFor(&serviceID); p != nil {
		teams = p.teamOptions()
	}
	return domain.SpecialistHandoffTeamsResponse{Teams: teams}, nil
}

// incidentView is the repository's view of an incident with what the
// handoff configuration decides added: whether the incident can be handed
// off now (the same rules planSpecialistHandoff applies, read without a
// lock -- it only decides whether to offer the action), and its handoff's
// escalation team only when the configuration knows the team, as
// getHandoffSummary keeps only the teams IncidentHandoffUtils knows.
func (s *incidentService) incidentView(ctx context.Context, id string) (domain.IncidentView, error) {
	v, err := s.repo.GetIncidentByID(ctx, id)
	if err != nil {
		return v, err
	}
	can := false
	if v.State != nil && *v.State == string(domain.IncidentStateInProgress) && v.Service != nil {
		if p := s.handoffConfig.productFor(&v.Service.ID); p != nil {
			var group *string
			if v.AssignmentGroup != nil {
				group = &v.AssignmentGroup.ID
			}
			can = !p.holdsGroup(group)
		}
	}
	v.CanHandOffToSpecialist = &can
	if sum := v.SpecialistHandoff; sum != nil && sum.EscalationTeam != nil && !s.handoffConfig.knowsTeam(string(*sum.EscalationTeam)) {
		sum.EscalationTeam = nil
	}
	return v, nil
}
