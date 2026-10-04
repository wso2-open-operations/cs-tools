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
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type escalationService struct {
	repo     repository.EscalationRepository
	userRepo repository.UserRepository
	caseRepo repository.CaseRepository
	access   AccessService
}

// NewEscalationService constructs an EscalationService backed by Postgres.
// userRepo resolves the caller's x-user-id-token into an actor email for
// CreateEscalation's created_by/updated_by attribution, the same
// resolveActor pattern caseService uses. caseRepo/access authorize
// CreateEscalation's caseId against the caller's AccessScope -- see that
// method's own doc comment for why (a real IDOR otherwise: repo.
// CreateEscalation locks and mutates whatever case id it's given, with
// nothing upstream checking the caller may act on it at all).
func NewEscalationService(repo repository.EscalationRepository, userRepo repository.UserRepository, caseRepo repository.CaseRepository, access AccessService) EscalationService {
	return &escalationService{repo: repo, userRepo: userRepo, caseRepo: caseRepo, access: access}
}

// SearchEscalations implements EscalationService.
func (s *escalationService) SearchEscalations(ctx context.Context, req domain.SearchEscalationsRequest) (domain.SearchEscalationsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchEscalationsResponse{}, err
	}
	var caseIDs []string
	var currentLevels []int
	if req.Filters != nil {
		if err := validateUUIDs("filters.caseIds", req.Filters.CaseIDs); err != nil {
			return domain.SearchEscalationsResponse{}, err
		}
		caseIDs = req.Filters.CaseIDs
		for _, l := range req.Filters.CurrentLevels {
			// case_escalation_level_enum only spans EL0..EL5 (migration
			// 000053); an out-of-range value would otherwise reach
			// escalationLevelToEnum and trip a Postgres enum-cast error at
			// query time, surfacing as a 500 instead of a 400.
			if l < 0 || l > 5 {
				return domain.SearchEscalationsResponse{}, &apierror.ValidationError{Msg: "filters.currentLevels must be between 0 and 5"}
			}
		}
		currentLevels = req.Filters.CurrentLevels
	}
	sortField, sortOrder := "", ""
	if req.SortBy != nil {
		sortField = string(req.SortBy.Field)
		sortOrder = string(req.SortBy.Order)
	}

	escalations, total, err := s.repo.SearchEscalations(ctx, caseIDs, currentLevels, sortField, sortOrder, req.Pagination.Limit, req.Pagination.Offset)
	if err != nil {
		return domain.SearchEscalationsResponse{}, err
	}

	return domain.SearchEscalationsResponse{
		Escalations: escalations,
		Total:       total,
		Limit:       req.Pagination.Limit,
		Offset:      req.Pagination.Offset,
	}, nil
}

// CreateEscalation implements EscalationService. Mirrors
// snEscalationService.CreateEscalation's own request validation (action
// default/normalize, reason required when escalating) so the two data
// sources reject the same malformed input the same way -- the actual
// level-transition/notification-recipient rule lives in
// EscalationRepository.CreateEscalation's own doc comment.
//
// req.CaseID is authorized against the caller's AccessScope before
// repo.CreateEscalation ever runs: resolve the scope, then read the case
// through it via CaseRepository.GetCaseByID (the exact same scoped-query
// shape GetCaseByID/SearchCases already use -- see AccessService's own doc
// comment), which returns a NotFoundError for a case outside scope,
// indistinguishable from one that doesn't exist at all (never a 403 that
// would confirm it exists to someone not entitled to know that). Without
// this, any authenticated caller who merely knows another account's case
// UUID could escalate/de-escalate it and read back its details -- an IDOR.
// This is the SOLE implementation both entry points (POST /escalations and
// POST /cases/{id}/escalations, via CaseEscalationService's thin wrapper)
// funnel through, so both are covered by this one check.
//
// This intentionally reverses this codebase's own documented "not yet
// wired" stance on escalations (see CLAUDE.md's "Token validation and
// caller-scoped access" -- escalations were explicitly listed there
// alongside comments/time cards/attachments/etc. as accepted, deferred
// follow-up work, not a decision to fix them now). Every sibling case
// mutation (UpdateCase, AddCaseTag, AcknowledgeCase, CreateCaseComment, ...)
// remains genuinely unscoped after this change -- the same caller this
// closes the door on for escalations can still read/act on an out-of-scope
// case through any of those. Scoping just this one write endpoint, without
// a decision to also close the others, is a real, flagged inconsistency,
// not a silent claim that "every write is scoped now."
func (s *escalationService) CreateEscalation(ctx context.Context, req domain.CreateEscalationRequest) (domain.CreateEscalationResponse, error) {
	if err := validateUUIDs("caseId", []string{req.CaseID}); err != nil {
		return domain.CreateEscalationResponse{}, err
	}

	action := domain.EscalationActionEscalate
	if req.Action != nil {
		action = domain.EscalationAction(strings.ToUpper(string(*req.Action)))
	}
	if action != domain.EscalationActionEscalate && action != domain.EscalationActionDeescalate {
		return domain.CreateEscalationResponse{}, &apierror.ValidationError{
			Msg: fmt.Sprintf("invalid action %q. Allowed actions: %s, %s", action, domain.EscalationActionEscalate, domain.EscalationActionDeescalate),
		}
	}
	if action == domain.EscalationActionEscalate && (req.Reason == nil || strings.TrimSpace(*req.Reason) == "") {
		return domain.CreateEscalationResponse{}, &apierror.ValidationError{Msg: "reason is required when action is ESCALATE"}
	}

	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return domain.CreateEscalationResponse{}, err
	}
	if _, err := s.caseRepo.GetCaseByID(ctx, req.CaseID, scope); err != nil {
		return domain.CreateEscalationResponse{}, err
	}

	email, err := callerEmail(ctx)
	if err != nil {
		return domain.CreateEscalationResponse{}, err
	}
	actor, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		// An already-authenticated caller getting GetUserByEmail's raw
		// NotFoundError back (surfaced as a 404) is confusing on their OWN
		// identity, and leaks the implementation detail that user emails
		// must be pre-seeded in Postgres. Substitute an UnauthorizedError
		// instead -- caseService.resolveActor does NOT have this same
		// handling today (it propagates GetUserByEmail's error verbatim),
		// so this isn't matching an existing pattern; it's a genuine fix
		// that resolveActor likely wants too, out of scope here.
		var notFound *apierror.NotFoundError
		if errors.As(err, &notFound) {
			return domain.CreateEscalationResponse{}, &apierror.UnauthorizedError{Msg: "authenticated user is not recognised in this system"}
		}
		return domain.CreateEscalationResponse{}, err
	}

	escalation, err := s.repo.CreateEscalation(ctx, req.CaseID, action, req.Reason, actor.Email)
	if err != nil {
		return domain.CreateEscalationResponse{}, err
	}

	verb := "escalated"
	if action == domain.EscalationActionDeescalate {
		verb = "de-escalated"
	}
	return domain.CreateEscalationResponse{
		Message:    fmt.Sprintf("case %s from %s to %s", verb, escalation.PreviousLevel.Label, escalation.CurrentLevel.Label),
		Escalation: escalation,
	}, nil
}
