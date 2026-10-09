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
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// publishCaseEscalatedTimeout bounds publishCaseEscalated's publish call, the
// same bound case.acknowledged uses.
const publishCaseEscalatedTimeout = 5 * time.Second

type escalationService struct {
	repo     repository.EscalationRepository
	userRepo repository.UserRepository
	caseRepo repository.CaseRepository
	access   AccessService
	// snWriteback/snMirror back CreateEscalation's best-effort, asynchronous
	// ServiceNow mirror write under DATA_SOURCE=postgres-servicenow-dual-write
	// -- both nil in every other mode. Set only via
	// NewEscalationServiceWithSNWriteback. Unlike call_request/deployment,
	// there is no id-mapping concern afterward: each escalate/de-escalate is
	// its own new, read-only-after-creation row (no later "update this
	// escalation by id" operation exists anywhere in this codebase), so the
	// mirror is dispatched the same simple way
	// deploymentService.UpdateDeployment's mirror is, just applied to a
	// CREATE instead of an UPDATE.
	snWriteback *SNWritebackDispatcher
	snMirror    EscalationService
	// publisher sends case.escalated after CreateEscalation commits, so
	// csm-notification-service emails the escalation's notification list.
	// nil unless wired via WithEscalationNotices.
	publisher EventPublisherService
}

// WithEscalationNotices attaches the case.escalated publisher to an
// already-constructed EscalationService, the same post-construction wiring as
// WithSRNotices. A no-op if svc is not the Postgres *escalationService (the
// ServiceNow data source sends its own notifications) or publisher is nil.
func WithEscalationNotices(svc EscalationService, publisher EventPublisherService) EscalationService {
	if es, ok := svc.(*escalationService); ok && publisher != nil {
		es.publisher = publisher
	}
	return svc
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

// NewEscalationServiceWithSNWriteback is NewEscalationService plus the wiring
// DATA_SOURCE=postgres-servicenow-dual-write needs: CreateEscalation
// dispatches a best-effort, asynchronous ServiceNow mirror write onto mirror
// after the Postgres write commits -- see escalationService's own
// snWriteback/snMirror doc comment.
func NewEscalationServiceWithSNWriteback(repo repository.EscalationRepository, userRepo repository.UserRepository, caseRepo repository.CaseRepository, access AccessService, dispatcher *SNWritebackDispatcher, mirror EscalationService) EscalationService {
	return &escalationService{repo: repo, userRepo: userRepo, caseRepo: caseRepo, access: access, snWriteback: dispatcher, snMirror: mirror}
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
	cv, err := s.caseRepo.GetCaseByID(ctx, req.CaseID, scope)
	if err != nil {
		return domain.CreateEscalationResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.CreateEscalationResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return domain.CreateEscalationResponse{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
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

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only (snWriteback/snMirror are both nil otherwise -- see
	// escalationService's own doc comment). Postgres has already committed by
	// this point. req is forwarded verbatim: snEscalationService.CreateEscalation
	// does its own CaseID-to-sys_id conversion and action/reason normalization,
	// so no translation is needed here.
	if s.snWriteback != nil {
		s.snWriteback.Dispatch(ctx, "escalation", escalation.ID, "create", req,
			func(writeCtx context.Context) error {
				_, err := s.snMirror.CreateEscalation(writeCtx, req)
				return err
			},
		)
	}

	// ServiceNow's "Internal Escalation notification" flow mails only rows
	// with u_current_level != 0, so a de-escalation (always to EL0) sends
	// nothing.
	if action == domain.EscalationActionEscalate {
		s.publishCaseEscalated(ctx, cv, actor, escalation)
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

// publishCaseEscalated publishes case.escalated for an escalation that has
// already committed. Best effort, like every other case.* publish: a failure
// is logged (Publish has already recorded it in event_publish_failures) and
// never fails the escalation. cv is the case as read for the scope check just
// before the write; only fields the escalation does not change are used.
//
// ServiceNow's escalation stores who to notify on u_notification_list, and
// its "Internal Escalation notification" flow mails that list; this is the
// port of the flow. An escalation that resolved nobody is not published
// (SN's flow fails that run with "Email has no recipients").
func (s *escalationService) publishCaseEscalated(ctx context.Context, cv domain.CaseView, actor domain.User, e domain.CreatedEscalation) {
	if s.publisher == nil {
		return
	}
	// Internal staff only, whatever the notification list holds: the email
	// is an internal escalation notice, and no customer may receive it even
	// if a per-case field (account owner, technical owner, CSM) or a role
	// grant points at a non-WSO2 user.
	recipients := filterWso2Emails(escalationRecipientEmails(e.NotificationSentTo))
	if len(recipients) == 0 {
		slog.InfoContext(ctx, "create escalation: no notification recipients, case.escalated not published",
			"caseId", e.Case.ID, "escalationId", e.ID)
		return
	}

	payload := events.CaseEscalatedPayload{
		CaseID:        e.Case.ID,
		CaseNumber:    cv.Number,
		CaseTitle:     cv.Subject,
		Severity:      strings.ToUpper(string(derefSeverity(cv.Severity))),
		Product:       caseProductName(cv),
		EscalationID:  e.ID,
		PreviousLevel: escalationLevelNumber(e.PreviousLevel),
		CurrentLevel:  escalationLevelNumber(e.CurrentLevel),
		ActorEmail:    actor.Email,
		EscalatedOn:   e.CreatedOn,
		Recipients:    recipients,
	}
	if e.Case.Number != nil && *e.Case.Number != "" {
		payload.CaseNumber = *e.Case.Number
	}
	if e.Reason != nil {
		payload.Reason = strings.TrimSpace(*e.Reason)
	}
	if dp := cv.DeployedProductDetails; dp != nil && dp.DisplayName != nil && *dp.DisplayName != "" {
		payload.Product = *dp.DisplayName
	}
	if cv.AccountDetails != nil {
		payload.AccountName = cv.AccountDetails.Name
	}
	if cv.DeploymentDetails != nil {
		payload.Environment = cv.DeploymentDetails.Name
	}
	if cv.AssignedEngineer != nil {
		payload.AssignedEngineerEmail = cv.AssignedEngineer.Email
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		slog.ErrorContext(ctx, "create escalation: encode case.escalated payload failed", "caseId", e.Case.ID, "error", err)
		return
	}
	pubCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), publishCaseEscalatedTimeout)
	defer cancel()
	if err := s.publisher.Publish(pubCtx, events.TypeCaseEscalated, e.Case.ID, raw); err != nil {
		slog.ErrorContext(ctx, "create escalation: publish case.escalated failed", "caseId", e.Case.ID, "escalationId", e.ID)
		return
	}
	slog.InfoContext(ctx, "create escalation: case.escalated published", "caseId", e.Case.ID, "escalationId", e.ID,
		"level", payload.CurrentLevel, "recipients", len(recipients))
}

// escalationRecipientEmails returns the notified users' emails, trimmed,
// lowercased and de-duplicated in list order. A user with no email is
// skipped: ServiceNow mails the same list and has nowhere to send one either.
func escalationRecipientEmails(users []domain.EscalationNotifiedUser) []string {
	seen := make(map[string]bool, len(users))
	out := make([]string, 0, len(users))
	for _, u := range users {
		if u.Email == nil {
			continue
		}
		email := strings.ToLower(strings.TrimSpace(*u.Email))
		if email == "" || seen[email] {
			continue
		}
		seen[email] = true
		out = append(out, email)
	}
	return out
}

// escalationLevelNumber turns an escalation level choice ("0".."5", or
// ServiceNow's "EL0".."EL5" label) into its number; anything else is 0.
func escalationLevelNumber(c domain.ChoiceListItem) int {
	for _, v := range []string{c.ID, c.Label} {
		v = strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(v)), "EL")
		if len(v) == 1 && v[0] >= '0' && v[0] <= '5' {
			return int(v[0] - '0')
		}
	}
	return 0
}
