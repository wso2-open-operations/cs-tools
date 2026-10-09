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
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// WithCaseTypeTransfer attaches the case type transfer (PATCH /cases/{id}
// with a "type") to an already-constructed CaseService, the same
// post-construction wiring as WithProductCategoryEnforcement and for the same
// reason. Without it a type change is refused as it was before this existed
// ("only supported for the ServiceNow data source"). A no-op if svc is not a
// *caseService or repo does not implement repository.CaseTypeTransferRepository.
func WithCaseTypeTransfer(svc CaseService, repo repository.CaseRepository) CaseService {
	cs, ok := svc.(*caseService)
	if !ok {
		return svc
	}
	if tr, ok := repo.(repository.CaseTypeTransferRepository); ok {
		cs.typeTransfer = tr
	}
	return svc
}

// transferableCaseTypes are the types a case can be moved between.
// "announcement" and the hosting_* types are system-managed and are neither a
// source nor a target (the webapp's dialog offers exactly these four).
var transferableCaseTypes = map[string]bool{
	"case":                     true,
	"engagement":               true,
	"service_request":          true,
	"security_report_analysis": true,
}

// otherFieldsAlongsideType names every field of the request that may not ride
// along with a type transfer. type, severity (for "case"), issueType,
// engagementType, engagementPaymentType, catalogId, catalogItemId and
// variables are the transfer's own and are judged per target.
func otherFieldsAlongsideType(req domain.UpdateCaseRequest) []string {
	var set []string
	add := func(name string, present bool) {
		if present {
			set = append(set, name)
		}
	}
	add("state", req.State != nil)
	add("workState", req.WorkState != nil)
	add("watchList", req.WatchList != nil)
	add("assigneeEmail", len(req.AssigneeEmail) > 0)
	add("resolutionCode", req.ResolutionCode != nil)
	add("cause", req.Cause != nil)
	add("closeNotes", req.CloseNotes != nil)
	add("parentId", req.ParentID != nil)
	add("relatedCaseId", req.RelatedCaseID != nil)
	add("autocloseHoldUntil", req.AutocloseHoldUntil != nil)
	add("subject", req.Subject != nil)
	add("description", req.Description != nil)
	add("deploymentId", req.DeploymentID != nil)
	add("deployedProductId", req.DeployedProductID != nil)
	add("bestCaseFixEta", req.BestCaseFixEta != nil)
	add("mostLikelyFixEta", req.MostLikelyFixEta != nil)
	add("worstCaseFixEta", req.WorstCaseFixEta != nil)
	add("addPublicComment", req.AddPublicComment != nil)
	add("product", req.Product != nil)
	add("publicTicket", req.PublicTicket != nil)
	add("acknowledge", req.Acknowledge != nil)
	add("workaroundProvided", req.WorkaroundProvided != nil)
	add("markFixIssued", req.MarkFixIssued != nil)
	return set
}

// validateCaseTypeTransfer validates a request that carries a type and returns
// the canonical target type. It applies the same rules as the ServiceNow data
// source's own UpdateCase (sn_case_service.go), so both data sources accept the
// same transfers and reject the same ones with the same words:
//
//   - the type is one of the four transferable types (aliases such as
//     "default_case" resolved first);
//   - nothing else rides along but the transfer's own companions;
//   - "case" needs a severity AND an issue type (the severity also decides
//     Incident (S0-S3) vs Query (S4) in ServiceNow);
//   - "engagement" needs an engagement type AND a payment type;
//   - "service_request" needs a catalog, a catalog item and at least one answer;
//   - "security_report_analysis" takes no companion at all.
func validateCaseTypeTransfer(req domain.UpdateCaseRequest) (string, error) {
	if req.Type == nil {
		return "", &apierror.ValidationError{Msg: "type is required"}
	}
	target := normalizeCaseType(*req.Type)
	if !validCaseType[target] {
		return "", &apierror.ValidationError{Msg: "type contains invalid value: " + *req.Type}
	}
	if !transferableCaseTypes[target] {
		return "", &apierror.ValidationError{Msg: "type " + target + " is system-managed and cannot be a transfer target"}
	}
	if others := otherFieldsAlongsideType(req); len(others) > 0 {
		return "", &apierror.ValidationError{Msg: "type cannot be combined with any other field in the same request (got " + strings.Join(others, ", ") + ")"}
	}

	hasEngagement := req.EngagementType != nil || req.EngagementPaymentType != nil
	hasCatalog := req.CatalogID != nil || req.CatalogItemID != nil || len(req.Variables) > 0
	switch target {
	case "engagement":
		if req.EngagementType == nil {
			return "", &apierror.ValidationError{Msg: "engagementType is required when type is \"engagement\""}
		}
		if req.EngagementPaymentType == nil {
			return "", &apierror.ValidationError{Msg: "engagementPaymentType is required when type is \"engagement\""}
		}
		if hasCatalog {
			return "", &apierror.ValidationError{Msg: "catalogId, catalogItemId, and variables are only accepted when type is \"service_request\""}
		}
		if req.IssueType != nil {
			return "", &apierror.ValidationError{Msg: "issueType is only accepted when type is \"case\""}
		}
		if req.Severity != nil {
			return "", &apierror.ValidationError{Msg: "severity may only accompany type when type is \"case\""}
		}
		if !validEngagementType[*req.EngagementType] {
			return "", &apierror.ValidationError{Msg: "engagementType contains invalid value: " + string(*req.EngagementType)}
		}
		if !validEngagementPaymentType[*req.EngagementPaymentType] {
			return "", &apierror.ValidationError{Msg: "engagementPaymentType contains invalid value: " + string(*req.EngagementPaymentType)}
		}
	case "service_request":
		if req.CatalogID == nil || req.CatalogItemID == nil {
			return "", &apierror.ValidationError{Msg: "catalogId and catalogItemId are required when type is \"service_request\""}
		}
		if len(req.Variables) == 0 {
			return "", &apierror.ValidationError{Msg: "variables must contain at least one entry when type is \"service_request\""}
		}
		if hasEngagement {
			return "", &apierror.ValidationError{Msg: "engagementType and engagementPaymentType are only accepted when type is \"engagement\""}
		}
		if req.IssueType != nil {
			return "", &apierror.ValidationError{Msg: "issueType is only accepted when type is \"case\""}
		}
		if req.Severity != nil {
			return "", &apierror.ValidationError{Msg: "severity may only accompany type when type is \"case\""}
		}
		if err := validateUUIDs("catalogId", []string{*req.CatalogID}); err != nil {
			return "", err
		}
		if err := validateUUIDs("catalogItemId", []string{*req.CatalogItemID}); err != nil {
			return "", err
		}
		for i, v := range req.Variables {
			if err := validateUUIDs(fmt.Sprintf("variables[%d].id", i), []string{v.ID}); err != nil {
				return "", err
			}
		}
	case "case":
		if hasEngagement || hasCatalog {
			return "", &apierror.ValidationError{Msg: "engagementType and engagementPaymentType are only accepted when type is \"engagement\"; catalogId, catalogItemId, and variables are only accepted when type is \"service_request\""}
		}
		if req.Severity == nil {
			return "", &apierror.ValidationError{Msg: "severity is required when type is \"case\""}
		}
		if req.IssueType == nil {
			return "", &apierror.ValidationError{Msg: "issueType is required when type is \"case\""}
		}
		if !validCaseSeverity[*req.Severity] {
			return "", &apierror.ValidationError{Msg: "severity contains invalid value: " + string(*req.Severity)}
		}
		if !validCaseIssueType[*req.IssueType] {
			return "", &apierror.ValidationError{Msg: "issueType contains invalid value: " + string(*req.IssueType)}
		}
	default: // security_report_analysis
		if hasEngagement || hasCatalog {
			return "", &apierror.ValidationError{Msg: "engagementType and engagementPaymentType are only accepted when type is \"engagement\"; catalogId, catalogItemId, and variables are only accepted when type is \"service_request\""}
		}
		if req.IssueType != nil {
			return "", &apierror.ValidationError{Msg: "issueType is only accepted when type is \"case\""}
		}
		if req.Severity != nil {
			return "", &apierror.ValidationError{Msg: "severity may only accompany type when type is \"case\""}
		}
	}
	return target, nil
}

// transferCaseType implements UpdateCase for a request that carries a type: a
// case moved to Case (an Incident at S0-S3, a Query at S4), Engagement,
// Service Request or Security Report Analysis.
//
// Internal callers only. On the ServiceNow data source this is decided by the
// roles ServiceNow itself holds; on this one there is no such layer, and a
// customer must never be able to re-type a case (customer-portal's BFF forwards
// whatever PATCH a customer's session builds).
//
// Under DATA_SOURCE=postgres-servicenow-dual-write ServiceNow is the system of
// record for what a transfer means (its own validation, the catalog item, the
// SLA it recalculates), so it is asked FIRST -- but from inside the Postgres
// transaction, after every Postgres statement has already succeeded (see
// repository.CaseTypeTransferRepository.TransferCaseType). Postgres is the
// side that can be rolled back, so it goes last in time even though it was
// prepared first: either both stores move, or neither does. Plain Postgres has
// no ServiceNow behind it and the same transaction simply has no remote step.
//
// The new extension row's state is the one ServiceNow reports (dual-write), else
// the one the case already had. The case keeps its number, id, project,
// deployment, watchers, tags, comments and attachments: all of them live on the
// work item, not on the type's extension row.
//
// What follows a committed transfer is best-effort and never undoes it:
//   - an activity entry ("Type: Case → Engagement");
//   - the CSM SLA clocks: a case moved INTO "case" gets a fresh set for its
//     severity (S4, a Query, only has the response clock), and one moved OUT of
//     "case" has its clocks cancelled, since the severity-keyed policy no longer
//     applies. This is exactly what ReviseCaseClocks does for a severity change;
//   - time cards: re-marked billable inside the transaction when the LOW/S4
//     boundary is crossed (repository.TransferCaseType).
func (s *caseService) transferCaseType(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error) {
	if err := RequireInternalCaller(ctx, s.access, "only internal users can change a case's type"); err != nil {
		return domain.UpdateCaseResponse{}, err
	}
	target, err := validateCaseTypeTransfer(req)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}
	// Hand the canonical type on: "default_case" means "case" to everything below.
	req.Type = &target

	// Resolved best-effort, like every other branch of UpdateCase that records
	// who did something: a caller whose token cannot be resolved still transfers.
	var actorEmail string
	if actor, err := s.resolveActor(ctx); err == nil {
		actorEmail = actor.Email
	}

	plan := repository.CaseTypeTransfer{
		CaseID:                req.ID,
		TargetType:            target,
		Severity:              req.Severity,
		IssueType:             req.IssueType,
		EngagementType:        req.EngagementType,
		EngagementPaymentType: req.EngagementPaymentType,
		ActorEmail:            actorEmail,
	}

	var snResp *domain.UpdateCaseResponse
	var remote repository.CaseTypeTransferRemote
	if s.snMirror != nil {
		remote = func(ctx context.Context, _ repository.CaseTypeTransferBefore) (*repository.CaseTypeTransferRemoteResult, error) {
			callCtx, cancel := serviceNowTransferContext(ctx)
			defer cancel()
			resp, err := s.snMirror.UpdateCase(callCtx, req)
			if err != nil {
				if serviceNowRefused(err) {
					return nil, err
				}
				// A deadline, a lost connection or a 5xx does not say whether
				// ServiceNow applied the transfer. Rolling Postgres back on a
				// transfer ServiceNow did apply would leave the case a different
				// type in each, so ask ServiceNow what it holds before deciding.
				state, applied := s.serviceNowHasType(ctx, req.ID, target)
				if !applied {
					return nil, err
				}
				slog.WarnContext(ctx, "update case: ServiceNow answered the type transfer with an error but holds the new type; completing it in Postgres",
					"caseId", req.ID, "type", target, "error", err)
				snResp = &domain.UpdateCaseResponse{Case: domain.UpdatedCase{ID: req.ID, State: state, Type: target}}
				return &repository.CaseTypeTransferRemoteResult{State: state}, nil
			}
			snResp = &resp
			return &repository.CaseTypeTransferRemoteResult{State: resp.Case.State}, nil
		}
	}

	result, err := s.typeTransfer.TransferCaseType(ctx, plan, remote)
	if err != nil {
		if snResp != nil {
			// ServiceNow has already moved the case but Postgres did not commit: real
			// drift needing operator attention (the next sync of this case corrects
			// Postgres), not a safely rejected request. Logged loudly because nothing
			// else records it -- the remote step ran, so there is no writeback failure.
			slog.ErrorContext(ctx, "update case: ServiceNow transferred the case type but the Postgres transfer did not commit",
				"caseId", req.ID, "type", target, "error", err)
		}
		return domain.UpdateCaseResponse{}, err
	}

	s.recordFieldChangeActivity(ctx, req.ID, "type", humanizeSnakeCase(result.PreviousType), humanizeSnakeCase(result.Type), actorEmail)

	if s.slaEngine != nil {
		switch {
		case result.Type == "case":
			s.slaEngine.ReviseCaseClocks(ctx, req.ID, result.Severity, result.ProjectID)
		case result.PreviousType == "case":
			s.slaEngine.ReviseCaseClocks(ctx, req.ID, nil, result.ProjectID)
		}
	}

	resp := domain.UpdateCaseResponse{
		Message: "Case type changed successfully.",
		Case: domain.UpdatedCase{
			ID:        req.ID,
			UpdatedOn: result.UpdatedOn,
			UpdatedBy: actorEmail,
			State:     result.State,
			Severity:  result.Severity,
			Type:      result.Type,
			WorkState: result.WorkState,
		},
	}
	if snResp != nil {
		// ServiceNow's own receipt is the richer one (it names who did it by their
		// ServiceNow identity and echoes the resolved fields); this keeps it and
		// only guarantees the fields the portal reads next.
		resp = *snResp
		resp.Case.ID = req.ID
		resp.Case.Type = result.Type
		if resp.Case.Severity == nil {
			resp.Case.Severity = result.Severity
		}
		if resp.Case.State == nil {
			resp.Case.State = result.State
		}
		// A transfer completed after an ambiguous ServiceNow failure has no receipt of
		// ServiceNow's own to speak of, and even a real one may omit these: take what
		// Postgres committed, never overwriting what ServiceNow did supply.
		if resp.Case.UpdatedOn.IsZero() {
			resp.Case.UpdatedOn = result.UpdatedOn
		}
		if resp.Case.UpdatedBy == "" {
			resp.Case.UpdatedBy = actorEmail
		}
		if resp.Case.WorkState == nil {
			resp.Case.WorkState = result.WorkState
		}
		if resp.Message == "" {
			resp.Message = "Case type changed successfully."
		}
	}
	return resp, nil
}

// Time kept back from the request's own deadline for what follows the
// ServiceNow call: asking ServiceNow whether it applied the transfer (up to
// serviceNowRecheckTimeout) and then committing. Without it a ServiceNow call
// slow enough to use the whole request would leave neither.
const (
	serviceNowTransferMargin   = 20 * time.Second
	serviceNowRecheckTimeout   = 10 * time.Second
	serviceNowTransferMinSpare = 25 * time.Second
)

// serviceNowTransferContext bounds the ServiceNow call so the request still has
// serviceNowTransferMargin left when it returns. A request with little time left
// to begin with (or none declared) is left alone: there is no margin to give.
func serviceNowTransferContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > serviceNowTransferMinSpare {
			return context.WithTimeout(ctx, remaining-serviceNowTransferMargin)
		}
	}
	return ctx, func() {}
}

// serviceNowRefused reports whether err is ServiceNow (or the integration
// service in front of it) turning the request down, as opposed to a failure that
// leaves it unknown whether the change was applied: the client maps 400, 401,
// 403, 404 and 409 to these types and everything else (a deadline, a connection
// failure, a 5xx) to something else.
func serviceNowRefused(err error) bool {
	var (
		validation   *apierror.ValidationError
		unauthorized *apierror.UnauthorizedError
		forbidden    *apierror.ForbiddenError
		notFound     *apierror.NotFoundError
		conflict     *apierror.ConflictError
	)
	return errors.As(err, &validation) || errors.As(err, &unauthorized) || errors.As(err, &forbidden) ||
		errors.As(err, &notFound) || errors.As(err, &conflict)
}

// serviceNowHasType asks ServiceNow for the case and reports whether it already
// has the given type, with the state it holds. It runs detached from the
// request's cancellation (the request may be the thing that just ran out of
// time) but keeps its values, the caller's token included, and has its own bound.
func (s *caseService) serviceNowHasType(ctx context.Context, caseID, target string) (*domain.CaseState, bool) {
	checkCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), serviceNowRecheckTimeout)
	defer cancel()
	cv, err := s.snMirror.GetCaseByID(checkCtx, caseID)
	if err != nil {
		slog.WarnContext(ctx, "update case: could not ask ServiceNow whether the type transfer was applied", "caseId", caseID, "error", err)
		return nil, false
	}
	if cv.Type == nil || normalizeCaseType(*cv.Type) != target {
		return nil, false
	}
	return cv.State, true
}
