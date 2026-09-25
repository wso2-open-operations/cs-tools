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

// Package service is declared in interfaces.go.
package service

import (
	"context"
	"encoding/json"
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

type caseService struct {
	repo     repository.CaseRepository
	userRepo repository.UserRepository
	// publisher is nil when Event Hub is not configured — see
	// snCaseService.publisher's own doc comment for the same convention.
	// Currently only ever read by UpdateCase's (inert — see
	// events.TypeCaseBillableStatusChanged's own doc comment)
	// case.billable_status_changed detection.
	publisher EventPublisherService
	access    AccessService
	// snWriteback/snMirror back CreateCase, UpdateCase, and CreateCaseComment's
	// ServiceNow-facing paths under DATA_SOURCE=postgres-servicenow-dual-write —
	// both nil in every other mode. Set only via NewCaseServiceWithSNWriteback
	// (see that constructor's own doc comment for why not here). snMirror
	// serves three distinct purposes, all documented at their own call sites:
	//   - UpdateCase dispatches a best-effort, asynchronous State/Severity/
	//     WorkState mirror write onto it via snWriteback, through the
	//     snFieldPatcher interface below (patchCaseFields, a bare PATCH with
	//     none of UpdateCase's own read/no-op-detection/event-publish side
	//     effects).
	//   - CreateCaseComment dispatches a best-effort, asynchronous comment
	//     mirror write onto it via snWriteback, through the snCommentMirror
	//     interface below (CreateBareCaseComment, a bare POST with none of
	//     CreateCaseComment's own state-transition/event side effects).
	//   - CreateCase calls it directly, synchronously, BEFORE writing to
	//     Postgres at all — see CreateCase's own doc comment for why create
	//     is SN-first while update/comment are Postgres-first.
	snWriteback *SNWritebackDispatcher
	snMirror    CaseService
}

// snFieldPatcher is implemented by *snCaseService (see patchCaseFields's own
// doc comment). A narrow interface — rather than adding patchCaseFields to
// the full CaseService interface, which every implementer (including the
// plain, Postgres-backed caseService itself) would then have to satisfy —
// named exactly for what UpdateCase's mirror needs: a bare PATCH with none of
// snCaseService.UpdateCase's own read-before-write behavior.
type snFieldPatcher interface {
	patchCaseFields(ctx context.Context, caseID string, state *domain.CaseState, severity *domain.CaseSeverity, workState *domain.CaseWorkState) (domain.UpdatedCase, error)
}

// snCommentMirror is implemented by *snCaseService (see
// CreateBareCaseComment's own doc comment). A narrow interface for the same
// reason snFieldPatcher is one: CreateCaseComment's mirror needs a bare
// POST, not the full CommentService/CaseService surface.
type snCommentMirror interface {
	CreateBareCaseComment(ctx context.Context, caseID string, commentType domain.CommentType, content string) (domain.CaseCommentDetail, error)
}

// snWatchListPatcher is implemented by *snCaseService (see
// patchCaseWatchList's own doc comment). A narrow interface for the same
// reason snFieldPatcher is one: updateCaseWatchList's mirror needs a bare
// watch-list-only PATCH, not the full CaseService surface.
type snWatchListPatcher interface {
	patchCaseWatchList(ctx context.Context, caseID string, userIDs []string) (domain.UpdatedCase, error)
}

// snAssigneePatcher is implemented by *snCaseService (see patchCaseAssignee's
// own doc comment). A narrow interface for the same reason snFieldPatcher is
// one: updateCaseAssignee's mirror needs a bare assignee-only PATCH, not the
// full CaseService surface.
type snAssigneePatcher interface {
	patchCaseAssignee(ctx context.Context, caseID, assigneeEmail string) error
}

// snAcknowledgePatcher is implemented by *snCaseService (see
// patchCaseAcknowledge's own doc comment). A narrow interface for the same
// reason snFieldPatcher is one: acknowledgeCase's mirror needs a bare
// acknowledge-only PATCH, not the full CaseService surface.
type snAcknowledgePatcher interface {
	patchCaseAcknowledge(ctx context.Context, caseID string) error
}

// snParentPatcher is implemented by *snCaseService (see patchCaseParent's own
// doc comment). A narrow interface for the same reason snFieldPatcher is
// one: updateCaseParent's mirror needs a bare parentId-only PATCH, not the
// full CaseService surface.
type snParentPatcher interface {
	patchCaseParent(ctx context.Context, caseID, parentID string) error
}

// snFieldsBundlePatcher is implemented by *snCaseService (see
// patchCaseFieldsBundle's own doc comment). A narrow interface for the same
// reason snFieldPatcher is one: updateCaseFields' mirror needs a bare PATCH
// covering the combinable "plain field" bundle, not the full CaseService
// surface.
type snFieldsBundlePatcher interface {
	patchCaseFieldsBundle(ctx context.Context, caseID string, req domain.UpdateCaseRequest) error
}

// NewCaseService constructs a CaseService backed by the given repositories.
// publisher may be nil (see caseService.publisher's own doc comment). access
// scopes GetCaseByID/SearchCases's reads (see AccessService).
func NewCaseService(repo repository.CaseRepository, userRepo repository.UserRepository, publisher EventPublisherService, access AccessService) CaseService {
	return &caseService{repo: repo, userRepo: userRepo, publisher: publisher, access: access}
}

// NewCaseServiceWithSNWriteback is NewCaseService plus the wiring
// DATA_SOURCE=postgres-servicenow-dual-write needs for its case pilot (see
// config.DataSourcePostgresServiceNowDualWrite): UpdateCase's best-effort,
// asynchronous ServiceNow mirror write (WorkState only — see UpdateCase's
// own doc comment for why), and CreateCase's synchronous, SN-first creation
// (see CreateCase's own doc comment). A separate constructor rather than
// extending NewCaseService's own signature: every other call site (every
// existing test, plus every other DataSource branch in routes.go) keeps
// working completely unchanged.
//
// mirror is the ServiceNow-backed CaseService (from NewServiceNowCaseService)
// whose CreateCase/UpdateCase perform the real ServiceNow POST/PATCH. It is
// never made the active CaseService here — reads always stay on Postgres in
// this mode.
func NewCaseServiceWithSNWriteback(repo repository.CaseRepository, userRepo repository.UserRepository, publisher EventPublisherService, access AccessService, dispatcher *SNWritebackDispatcher, mirror CaseService) CaseService {
	return &caseService{
		repo: repo, userRepo: userRepo, publisher: publisher, access: access,
		snWriteback: dispatcher,
		snMirror:    mirror,
	}
}

var validCaseSortField = map[domain.CaseSortField]bool{
	domain.CaseSortFieldCreatedOn: true,
	domain.CaseSortFieldUpdatedOn: true,
	domain.CaseSortFieldSeverity:  true,
	domain.CaseSortFieldState:     true,
}

var validCaseType = map[string]bool{
	"case":                     true,
	"service_request":          true,
	"security_report_analysis": true,
	"announcement":             true,
	"engagement":               true,
}

// caseTypeAliases maps caller-supplied case type values this API does not
// consider canonical to the value it actually recognises. "default_case" is
// the real, currently-in-production customer-portal frontend's value for
// this type (it's ServiceNow's own raw caseType wire value, which the
// frontend was built directly against before this service's Postgres-backed
// "case" enum existed — see migrations/000008_create_cases.up.sql's
// case_type_enum) and must keep working indefinitely, not just during a
// migration window. Applying this alias is the FIRST thing that happens to
// any caller-supplied case type value, before validCaseType or any
// data-source-specific mapping (snCaseTypeMap, the Postgres repo's enum
// cast) ever sees it, so every downstream consumer only ever has to know
// about the canonical value "case".
var caseTypeAliases = map[string]string{
	"default_case": "case",
}

// normalizeCaseType resolves a caller-supplied case type value to its
// canonical form via caseTypeAliases, or returns it unchanged if it isn't an
// alias (including if it's already canonical, or altogether invalid --
// validCaseType is what rejects the latter).
func normalizeCaseType(t string) string {
	if canonical, ok := caseTypeAliases[t]; ok {
		return canonical
	}
	return t
}

var validEngagementType = map[domain.EngagementType]bool{
	domain.EngagementTypeMigration:             true,
	domain.EngagementTypeConsultancy:           true,
	domain.EngagementTypeNewFeatureImprovement: true,
	domain.EngagementTypeFollowUp:              true,
	domain.EngagementTypeOnboarding:            true,
}

var validEngagementPaymentType = map[domain.EngagementPaymentType]bool{
	domain.EngagementPaymentTypePaid: true,
	domain.EngagementPaymentTypeFOC:  true,
}

var validCaseSortOrder = map[domain.CaseSortOrder]bool{
	domain.CaseSortOrderAsc:  true,
	domain.CaseSortOrderDesc: true,
}

var validCaseState = map[domain.CaseState]bool{
	domain.CaseStateOpen:             true,
	domain.CaseStateWorkInProgress:   true,
	domain.CaseStateWaitingOnWSO2:    true,
	domain.CaseStateAwaitingInfo:     true,
	domain.CaseStateReopened:         true,
	domain.CaseStateSolutionProposed: true,
	domain.CaseStateClosed:           true,
}

var validCaseSeverity = map[domain.CaseSeverity]bool{
	domain.CaseSeverityCatastrophic: true,
	domain.CaseSeverityCritical:     true,
	domain.CaseSeverityHigh:         true,
	domain.CaseSeverityMedium:       true,
	domain.CaseSeverityLow:          true,
}

// validCaseAggregateField is the allow-list for AggregateCasesRequest.GroupBy,
// matching openapi.yaml's AggregateCasesRequest.groupBy enum exactly.
var validCaseAggregateField = map[string]bool{
	"account":  true,
	"state":    true,
	"severity": true,
	"type":     true,
}

var validCaseIssueType = map[domain.CaseIssueType]bool{
	domain.CaseIssueTypeError:                  true,
	domain.CaseIssueTypePartialOutage:          true,
	domain.CaseIssueTypePerformanceDegradation: true,
	domain.CaseIssueTypeQuestion:               true,
	domain.CaseIssueTypeSecurityOrCompliance:   true,
	domain.CaseIssueTypeTotalOutage:            true,
}

// validCaseCause guards UpdateCase's Cause field the same way
// validCaseIssueType/validCaseSeverity guard theirs -- domain.CaseCause's own
// values already match "case".cause's case_cause_enum labels by identity
// (unlike CaseResolutionCode, which needs caseResolutionCodeToEnum's actual
// translation table in the repository package), so this is a plain
// existence check catching a typo/garbage value before it ever reaches SQL.
var validCaseCause = map[domain.CaseCause]bool{
	domain.CaseCauseSolutionArchitecture:          true,
	domain.CaseCauseDeploymentArchitecture:        true,
	domain.CaseCauseUserErrorConfiguration:        true,
	domain.CaseCauseUserErrorProductConcept:       true,
	domain.CaseCauseUserErrorRuntime:              true,
	domain.CaseCauseUserErrorRecommendation:       true,
	domain.CaseCauseCustomizationLimitation:       true,
	domain.CaseCauseCustomizationBug:              true,
	domain.CaseCauseDocumentationGap:              true,
	domain.CaseCauseDocumentationError:            true,
	domain.CaseCauseProductLimitation:             true,
	domain.CaseCauseProductBug:                    true,
	domain.CaseCauseProductRegression:             true,
	domain.CaseCauseProductMigration:              true,
	domain.CaseCauseInfrastructureDatabase:        true,
	domain.CaseCauseInfrastructureOS:              true,
	domain.CaseCauseInfrastructureNetwork:         true,
	domain.CaseCauseInfrastructureJDK:             true,
	domain.CaseCauseInfrastructureLDAP:            true,
	domain.CaseCauseInfrastructureLoadBalancer:    true,
	domain.CaseCauseInfrastructureIAAS:            true,
	domain.CaseCauseInfrastructureExternalProduct: true,
	domain.CaseCauseInfrastructureProxy:           true,
	domain.CaseCauseInfrastructureOther:           true,
	domain.CaseCauseUnknown:                       true,
}

var validCaseWorkState = map[domain.CaseWorkState]bool{
	domain.CaseWorkStateOngoing: true,
	domain.CaseWorkStatePaused:  true,
}

// validateCreateCaseRequest validates fields common to all CreateCase data
// sources. Normalizes req.Type via normalizeCaseType FIRST (req is a
// pointer specifically so this mutation is visible to the caller's own
// switch on req.Type and its SN/repo payload-building afterwards) — every
// other check in this function, and everything downstream, only ever sees
// the canonical value.
// UUID format of ID fields is not checked here — postgres IDs are UUIDs but
// ServiceNow IDs are opaque hex strings; callers add format checks as needed.
func validateCreateCaseRequest(req *domain.CreateCaseRequest) error {
	req.Type = normalizeCaseType(req.Type)
	if req.Type == "" {
		return &apierror.ValidationError{Msg: "type is required"}
	}
	if !validCaseType[req.Type] {
		return &apierror.ValidationError{Msg: "type contains invalid value: " + req.Type}
	}
	if req.ProjectID == "" {
		return &apierror.ValidationError{Msg: "projectId is required"}
	}
	// Announcements have no deployment/deployed-product concept: these fields
	// are deliberately omitted at the ServiceNow layer, not just optional.
	if req.Type != "announcement" {
		if req.DeploymentID == "" {
			return &apierror.ValidationError{Msg: "deploymentId is required"}
		}
		if req.DeployedProductID == "" {
			return &apierror.ValidationError{Msg: "deployedProductId is required"}
		}
	}

	switch req.Type {
	case "case":
		if req.Subject == "" {
			return &apierror.ValidationError{Msg: "subject is required"}
		}
		if req.Description == "" {
			return &apierror.ValidationError{Msg: "description is required"}
		}
		if !validCaseSeverity[req.Severity] {
			return &apierror.ValidationError{Msg: "severity contains invalid value: " + string(req.Severity)}
		}
		if !validCaseIssueType[req.IssueType] {
			return &apierror.ValidationError{Msg: "issueType contains invalid value: " + string(req.IssueType)}
		}
	case "service_request":
		if req.CatalogID == "" {
			return &apierror.ValidationError{Msg: "catalogId is required for service_request"}
		}
		if req.CatalogItemID == "" {
			return &apierror.ValidationError{Msg: "catalogItemId is required for service_request"}
		}
		if len(req.Variables) == 0 {
			return &apierror.ValidationError{Msg: "variables are required for service_request"}
		}
	case "security_report_analysis":
		if req.Subject == "" {
			return &apierror.ValidationError{Msg: "subject is required for security_report_analysis"}
		}
		if req.Description == "" {
			return &apierror.ValidationError{Msg: "description is required for security_report_analysis"}
		}
		// Attachments are optional here (not backend-enforced by ServiceNow either):
		// the FE creates the case first, then uploads attachments in a separate
		// request per file, so a failed attachment upload never masks a
		// successful case creation.
		for i, a := range req.Attachments {
			if a.Name == "" {
				return &apierror.ValidationError{Msg: fmt.Sprintf("attachments[%d].name is required", i)}
			}
			if a.File == "" {
				return &apierror.ValidationError{Msg: fmt.Sprintf("attachments[%d].file is required", i)}
			}
		}
	case "engagement":
		if req.Subject == "" {
			return &apierror.ValidationError{Msg: "subject is required for engagement"}
		}
		if req.Description == "" {
			return &apierror.ValidationError{Msg: "description is required for engagement"}
		}
		if !validEngagementType[req.EngagementType] {
			return &apierror.ValidationError{Msg: "engagementType contains invalid value: " + string(req.EngagementType)}
		}
		if !validEngagementPaymentType[req.EngagementPaymentType] {
			return &apierror.ValidationError{Msg: "engagementPaymentType contains invalid value: " + string(req.EngagementPaymentType)}
		}
	case "announcement":
		if req.Subject == "" {
			return &apierror.ValidationError{Msg: "subject is required for announcement"}
		}
		if req.Description == "" {
			return &apierror.ValidationError{Msg: "description is required for announcement"}
		}
	}

	return nil
}

// CreateCase implements CaseService.
//
// Under DATA_SOURCE=postgres-servicenow-dual-write (snMirror != nil), this
// delegates to createCaseSNFirst instead of writing to Postgres directly —
// see that method's own doc comment for why CREATE is ServiceNow-first and
// synchronous, unlike UpdateCase's Postgres-first/async WorkState mirror.
func (s *caseService) CreateCase(ctx context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
	if err := validateCreateCaseRequest(&req); err != nil {
		return domain.CreateCaseResponse{}, err
	}
	// announcement/service_request/engagement/security_report_analysis only
	// exist on the SN-first path (s.snMirror != nil): case_repo's direct
	// Postgres insert only knows how to write a "CASE" work_item row, so on
	// a pure-Postgres data source (s.snMirror == nil) these four would either
	// hit an untyped uuid cast error (announcement's empty deployment id) or
	// a missing work_item.number generator, both surfacing as an opaque
	// 500/503 instead of a clean validation error.
	switch req.Type {
	case "case":
		// supported unconditionally
	case "announcement", "service_request", "engagement", "security_report_analysis":
		if s.snMirror == nil {
			return domain.CreateCaseResponse{}, &apierror.ValidationError{Msg: "type \"" + req.Type + "\" is supported only for DATA_SOURCE=postgres-servicenow-dual-write"}
		}
	default:
		return domain.CreateCaseResponse{}, &apierror.ValidationError{Msg: "only type \"case\", \"announcement\", \"service_request\", \"engagement\", or \"security_report_analysis\" is supported for the Postgres data source"}
	}
	if err := validateUUIDs("projectId", []string{req.ProjectID}); err != nil {
		return domain.CreateCaseResponse{}, err
	}
	// Announcements have no deployment/deployed-product concept (same
	// reasoning as validateCreateCaseRequest's own conditional) -- req.DeploymentID/
	// req.DeployedProductID are "" for an announcement, which validateUUIDs
	// would otherwise reject as an invalid UUID. None of the other four types
	// supported here (case/service_request/engagement/security_report_analysis)
	// omit deployment/deployed-product.
	if req.Type != "announcement" {
		if err := validateUUIDs("deploymentId", []string{req.DeploymentID}); err != nil {
			return domain.CreateCaseResponse{}, err
		}
		if err := validateUUIDs("deployedProductId", []string{req.DeployedProductID}); err != nil {
			return domain.CreateCaseResponse{}, err
		}
	}

	if s.snMirror != nil {
		return s.createCaseSNFirst(ctx, req)
	}

	// Only the pure-Postgres path below (no ServiceNow mirror at all) is
	// genuinely limited to type "case" — it writes directly into the
	// work_item+"case" tables, which have no equivalent extension table for
	// engagement/service_request/security_report_analysis/announcement yet
	// (see CaseRepository's own doc comment). This check used to run before
	// the snMirror branch above, unconditionally rejecting "announcement"
	// even when ServiceNow (which does support it — snCaseTypeMap has a real
	// entry) was about to handle the actual create.
	if req.Type != "case" {
		return domain.CreateCaseResponse{}, &apierror.ValidationError{Msg: "only type \"case\" is supported for the Postgres data source"}
	}

	if req.CreatedBy == "" {
		token := middleware.UserIDTokenFromContext(ctx)
		if token == "" {
			return domain.CreateCaseResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
		}
		email, err := emailFromJWT(token)
		if err != nil {
			return domain.CreateCaseResponse{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
		}
		user, err := s.userRepo.GetUserByEmail(ctx, email)
		if err != nil {
			return domain.CreateCaseResponse{}, err
		}
		req.CreatedBy = user.ID
	}
	c, err := s.repo.CreateCase(ctx, req)
	if err != nil {
		return domain.CreateCaseResponse{}, err
	}
	state := ""
	if c.State != nil {
		state = string(*c.State)
	}
	return domain.CreateCaseResponse{
		Message: "Case created successfully.",
		Case: domain.CreateCaseDetails{
			ID:         c.ID,
			InternalID: c.InternalID,
			Number:     c.Number,
			CreatedBy:  c.CreatedBy,
			CreatedOn:  c.CreatedOn,
			State:      state,
		},
	}, nil
}

// createCaseSNFirst implements CreateCase's DATA_SOURCE=postgres-servicenow-dual-write
// path: ServiceNow-FIRST and SYNCHRONOUS — the opposite order from
// UpdateCase's WorkState mirror (Postgres-first, ServiceNow best-effort and
// async afterward). That asymmetry is deliberate, not an inconsistency: an
// async-after-commit CREATE can leave a Postgres row with no ServiceNow
// counterpart if the background ServiceNow write then fails — a PERMANENT
// orphan, since every later comment/attachment/state-change on that case has
// no ServiceNow parent to attach to (ServiceNow is still the real backing
// store this platform proxies most writes onto). Calling ServiceNow first,
// and only writing to Postgres once that succeeds, makes that orphan
// impossible: either both systems end up with the case, or neither does. An
// UPDATE has no equivalent failure mode — the case already exists in both
// systems either way, so a failed async mirror write just leaves one field
// stale until retried, not orphaned.
//
// This call is made exactly once, with no internal retry: retrying here
// risked a worse failure than the one it absorbed. If ServiceNow's create
// actually succeeds server-side but the HTTP response back to
// entity-service is lost (timeout/network blip), a retry sends a second
// CREATE, producing a duplicate case in ServiceNow with Postgres only ever
// learning about whichever attempt's response happened to come back — an
// orphan duplicate in ServiceNow. Retry policy belongs to the caller, which
// knows whether its own request was already reattempted upstream.
//
// On success, id/number/wso2ID/createdBy come from ServiceNow's own
// response and are used AS-IS for the Postgres insert
// (CaseRepository.CreateCaseFromServiceNow) rather than generated — see
// that method's own doc comment. This is also what finally makes case
// creation possible on Postgres at all in this mode:
// CaseRepository.CreateCase's own doc comment explains why Postgres can't
// generate work_item.number/wso2_id itself (no sequence was ever added, and
// the intended format was never decided); ServiceNow being the identity
// source here sidesteps that unresolved question rather than answering it,
// which is exactly why this pilot could not have unblocked CreateCase any
// other way.
func (s *caseService) createCaseSNFirst(ctx context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
	snResp, err := s.snMirror.CreateCase(ctx, req)
	if err != nil {
		// ServiceNow never accepted the case — nothing is written to
		// Postgres at all, by construction (s.repo.CreateCaseFromServiceNow
		// is simply never called on this path). No orphan gets created.
		return domain.CreateCaseResponse{}, err
	}

	// state resolves ServiceNow's raw create-response state label
	// (snResp.Case.State, e.g. "Open") to the target extension table's own
	// state enum literal, for every type on this path other than "case" --
	// see snAnnouncementStateToEnum/snServiceRequestStateToEnum/
	// snEngagementStateToEnum/snSecurityReportAnalysisStateToEnum's own doc
	// comments for why this can't just hardcode 'OPEN' the way the "case"
	// insert does. Left "" for req.Type == "case", where the repository
	// ignores it entirely (hardcodes OPEN itself, same as before this change).
	var state string
	switch req.Type {
	case "announcement":
		state, err = snAnnouncementStateToEnum(snResp.Case.State)
	case "service_request":
		state, err = snServiceRequestStateToEnum(snResp.Case.State)
	case "engagement":
		state, err = snEngagementStateToEnum(snResp.Case.State)
	case "security_report_analysis":
		state, err = snSecurityReportAnalysisStateToEnum(snResp.Case.State)
	}
	if err != nil {
		// ServiceNow already has the record at this point (same drift
		// concern CreateCaseFromServiceNow's own error path below
		// documents) -- logged loudly since nothing else records it.
		slog.ErrorContext(ctx, "sn create case: record created but its ServiceNow state could not be mapped",
			"caseId", snResp.Case.ID, "snNumber", snResp.Case.Number, "type", req.Type, "snState", snResp.Case.State, "error", err)
		return domain.CreateCaseResponse{}, err
	}

	c, err := s.repo.CreateCaseFromServiceNow(ctx, req, snResp.Case.ID, snResp.Case.Number, snResp.Case.InternalID, snResp.Case.CreatedBy, state)
	if err != nil {
		// ServiceNow already has the case at this point — this is now real
		// drift (ServiceNow has it, Postgres doesn't) needing operator
		// attention, not a safely-rejected request. Logged loudly rather
		// than only returned, since nothing else records this particular
		// failure shape (it is not a writeback failure — SNWritebackDispatcher
		// is for the opposite direction, a Postgres row with no ServiceNow
		// counterpart — so it has no sn_writeback_failures row either).
		// Still returned as an error either way: the caller never gets a
		// usable response from this request regardless.
		slog.ErrorContext(ctx, "sn create case: ServiceNow case created but the Postgres insert failed",
			"caseId", snResp.Case.ID, "snNumber", snResp.Case.Number, "error", err)
		return domain.CreateCaseResponse{}, err
	}

	// Every case gets its account's four named stakeholders as watchers by
	// default -- a pure Postgres lookup, independent of req.WatchList and of
	// ServiceNow entirely (no forwarding, no email/UUID resolution). Must run
	// before the publish call below: it builds its own Recipients from a
	// GetCaseByID call, which reads watchers from work_item_watcher.
	s.addAccountDefaultWatchers(ctx, c.ID, c.ProjectID, c.CreatedBy)

	// Only now — Postgres has confirmed the row this mode's reads actually
	// depend on — is it safe to publish. See publishCaseCreatedEvent's doc
	// comment for why this can't just be snCaseService's own automatic
	// publish (that fires right after the ServiceNow POST, before this
	// Postgres insert was even attempted) — same reasoning
	// incidentService.createIncidentSNFirst already established.
	publishCaseCreatedEvent(ctx, s.publisher, s.GetCaseByID, req, c.ID)

	responseState := ""
	if c.State != nil {
		responseState = string(*c.State)
	}
	return domain.CreateCaseResponse{
		Message: "Case created successfully.",
		Case: domain.CreateCaseDetails{
			ID:         c.ID,
			InternalID: c.InternalID,
			Number:     c.Number,
			CreatedBy:  c.CreatedBy,
			CreatedOn:  c.CreatedOn,
			State:      responseState,
		},
	}, nil
}

// addAccountDefaultWatchers adds a just-created case's account's four named
// stakeholders (customer_success_manager_id, technical_owner_id,
// secondary_technical_owner_id, account_manager_id -- migration 000008) as
// its initial watchers -- see createCaseSNFirst's own call site comment for
// why this exists. A plain Postgres lookup keyed by projectID, independent
// of req.WatchList and of ServiceNow entirely: no forwarding, no email/UUID
// resolution -- the four columns are already user ids.
//
// Best-effort: ServiceNow already has the case by the time this runs (see
// createCaseSNFirst's own "no orphan gets created" vs. "real drift"
// distinction), so a failure here must not fail the create -- logged rather
// than returned, the same posture publishCaseCreatedEvent's own doc comment
// documents for the sibling publish step right after this one. A project
// with no linked account, or none of the four roles set, is a normal state
// (AccountDefaultWatcherIDs returns an empty slice), not an error.
func (s *caseService) addAccountDefaultWatchers(ctx context.Context, caseID, projectID, callerEmail string) {
	userIDs, err := s.repo.AccountDefaultWatcherIDs(ctx, projectID)
	if err != nil {
		slog.ErrorContext(ctx, "create case: resolving account default watchers failed", "caseId", caseID, "error", err)
		return
	}
	if len(userIDs) == 0 {
		return
	}
	if _, _, err := s.repo.SetCaseWatchList(ctx, caseID, userIDs, callerEmail); err != nil {
		slog.ErrorContext(ctx, "create case: adding account default watchers failed", "caseId", caseID, "error", err)
	}
}

// GetCaseByID implements CaseService.
func (s *caseService) GetCaseByID(ctx context.Context, id string) (domain.CaseView, error) {
	scope, err := resolveScopeForID(ctx, s.access, id)
	if err != nil {
		return domain.CaseView{}, err
	}
	return s.repo.GetCaseByID(ctx, id, scope)
}

// authorizeCaseAccess confirms the caller may see caseID before an operation
// acts on its comments/attachments/tags/watch-list/fields. Reuses
// GetCaseByID's own scoping (the same NotFoundError it already returns for a
// case outside the caller's scope) rather than reimplementing the check, so
// the two can never drift. Every caller of this method previously had no
// per-resource authorization at all -- see e.g. ConfirmCaseAttachment's own
// doc comment ("any authenticated user may perform on any case attachment").
func (s *caseService) authorizeCaseAccess(ctx context.Context, caseID string) error {
	_, err := s.GetCaseByID(ctx, caseID)
	return err
}

// authorizeAttachmentAccess fetches the attachment by id and confirms the
// caller may see its owning case, returning the fetched attachment for
// callers that need it (avoiding a second fetch). See authorizeCaseAccess's
// own doc comment for why this check exists.
func (s *caseService) authorizeAttachmentAccess(ctx context.Context, attachmentID string) (domain.Attachment, error) {
	a, err := s.repo.GetCaseAttachmentByID(ctx, attachmentID)
	if err != nil {
		return domain.Attachment{}, err
	}
	if err := s.authorizeCaseAccess(ctx, a.ReferenceID); err != nil {
		return domain.Attachment{}, err
	}
	return a, nil
}

var validCommentType = map[domain.CommentType]bool{
	domain.CommentTypeWorkNote: true,
	domain.CommentTypeComment:  true,
	domain.CommentTypeActivity: true,
}

// CreateCaseComment implements CaseService.
func (s *caseService) CreateCaseComment(ctx context.Context, req domain.CreateCaseCommentRequest) (domain.CreateCaseCommentResponse, error) {
	if err := validateUUIDs("caseId", []string{req.CaseID}); err != nil {
		return domain.CreateCaseCommentResponse{}, err
	}
	if !validCommentType[req.Type] {
		return domain.CreateCaseCommentResponse{}, &apierror.ValidationError{Msg: "type contains invalid value: " + string(req.Type)}
	}
	if req.Content == "" {
		return domain.CreateCaseCommentResponse{}, &apierror.ValidationError{Msg: "content is required"}
	}
	if err := s.authorizeCaseAccess(ctx, req.CaseID); err != nil {
		return domain.CreateCaseCommentResponse{}, err
	}
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.CreateCaseCommentResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return domain.CreateCaseCommentResponse{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return domain.CreateCaseCommentResponse{}, err
	}
	// comment.created_by (migration 000037) is a free-text VARCHAR, not a
	// UUID FK -- see CaseRepository.CreateCaseComment's own doc comment.
	req.CreatedBy = user.Email
	c, err := s.repo.CreateCaseComment(ctx, req)
	if err != nil {
		return domain.CreateCaseCommentResponse{}, err
	}

	// Event publishing follows the write, not DATA_SOURCE -- see
	// publishCaseCreatedEvent's own doc comment for why. authorName reuses
	// the actor already resolved above (user) -- unlike snCaseService's own
	// version, this data source never needs publishCommentAdded's
	// SearchCaseComments re-fetch trick, since the create response never
	// loses the author's identity here in the first place.
	if s.publisher != nil {
		if cv, err := s.GetCaseByID(ctx, req.CaseID); err != nil {
			slog.ErrorContext(ctx, "create comment: enrich case for case.comment_added publish failed", "caseId", req.CaseID)
		} else {
			authorName := strings.TrimSpace(user.FirstName + " " + user.LastName)
			if authorName == "" {
				authorName = user.Email
			}
			publishCommentAddedEvent(ctx, s.publisher, cv, req, c.ID, authorName)
		}
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only (snWriteback/snMirror are both nil otherwise — see
	// NewCaseServiceWithSNWriteback's own doc comment). Postgres has already
	// committed by this point and is fully authoritative for the comment —
	// this mirror's ONLY job is making sure ServiceNow's copy of the comment
	// text exists too. It deliberately does NOT replicate
	// snCaseService.CreateCaseComment's auto state-transition-on-reply
	// (applyCustomerReplyStateTransition) — Postgres already owns the real
	// outcome (note: this Postgres-native CreateCaseComment does not
	// currently implement state-transition-on-reply at all; that is a
	// separate, larger feature-parity gap, out of scope here — see
	// CreateBareCaseComment's own doc comment). Uses CreateBareCaseComment
	// specifically (not the full CreateCaseComment) so neither side effect
	// ever fires twice, or fires against ServiceNow for an outcome only
	// Postgres actually decided.
	// "activity" comments have no ServiceNow counterpart at all
	// (CreateBareCaseComment rejects the type outright -- see its own doc
	// comment) -- this is a permanent, 100%-guaranteed incompatibility, not
	// a transient failure worth recording for backfill, so skip dispatch
	// entirely rather than filling sn_writeback_failures with noise nobody
	// can ever act on.
	if s.snWriteback != nil && req.Type != domain.CommentTypeActivity {
		if m, ok := s.snMirror.(snCommentMirror); ok {
			mirrorCaseID, mirrorType, mirrorContent := req.CaseID, req.Type, req.Content
			s.snWriteback.Dispatch(ctx, "case_comment", req.CaseID, "create",
				map[string]any{"caseId": mirrorCaseID, "type": mirrorType, "content": mirrorContent},
				func(writeCtx context.Context) error {
					_, err := m.CreateBareCaseComment(writeCtx, mirrorCaseID, mirrorType, mirrorContent)
					return err
				},
			)
		}
	}

	return domain.CreateCaseCommentResponse{
		Message: "Comment created successfully",
		Comment: domain.CaseCommentDetail{
			ID:        c.ID,
			CreatedOn: c.CreatedOn,
			CreatedBy: user.Email,
		},
	}, nil
}

// SearchCaseComments implements CaseService.
func (s *caseService) SearchCaseComments(ctx context.Context, req domain.SearchCaseCommentsRequest) (domain.SearchCaseCommentsResponse, error) {
	if err := validateUUIDs("caseId", []string{req.CaseID}); err != nil {
		return domain.SearchCaseCommentsResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCaseCommentsResponse{}, err
	}
	if req.Filters != nil && req.Filters.Type != nil && !validCommentType[*req.Filters.Type] {
		return domain.SearchCaseCommentsResponse{}, &apierror.ValidationError{Msg: "filters.type contains invalid value: " + string(*req.Filters.Type)}
	}
	if err := s.authorizeCaseAccess(ctx, req.CaseID); err != nil {
		return domain.SearchCaseCommentsResponse{}, err
	}
	comments, total, err := s.repo.SearchCaseComments(ctx, req)
	if err != nil {
		return domain.SearchCaseCommentsResponse{}, err
	}
	return domain.SearchCaseCommentsResponse{
		Comments: comments,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(comments) < total,
	}, nil
}

// UpdateCase implements CaseService.
func (s *caseService) UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error) {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.UpdateCaseResponse{}, err
	}
	// Gates every branch below (watchList/assignee/parent/acknowledge/fields/
	// plain state-severity-workState) in one place: none of those sub-methods
	// had any per-resource authorization at all before this fix (see e.g.
	// updateCaseAssignee's own lack of one) -- any authenticated caller could
	// mutate any case by id, regardless of project.
	if err := s.authorizeCaseAccess(ctx, req.ID); err != nil {
		return domain.UpdateCaseResponse{}, err
	}
	// Fields with no Postgres implementation at all: a full type transfer,
	// and everything sn_case_service.go's own UpdateCase only ever accepts
	// as PART of one -- engagementType/engagementPaymentType/issueType/
	// catalogId/catalogItemId/variables are rejected there too whenever
	// req.Type is nil ("... are only allowed when type is also provided").
	// addPublicComment/product/publicTicket (the "Share Fix ETA" comment
	// side effect) and autocloseHoldUntil (no backing column anywhere in
	// this schema) have no Postgres equivalent either.
	if req.Type != nil || req.EngagementType != nil || req.EngagementPaymentType != nil || req.IssueType != nil ||
		req.CatalogID != nil || req.CatalogItemID != nil || len(req.Variables) > 0 ||
		req.AddPublicComment != nil || req.Product != nil || req.PublicTicket != nil ||
		req.AutocloseHoldUntil != nil {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "type, engagementType, engagementPaymentType, issueType, catalogId, catalogItemId, variables, addPublicComment, product, publicTicket, and autocloseHoldUntil are only supported for the ServiceNow data source"}
	}

	// The exclusive/combinable split below mirrors sn_case_service.go's own
	// UpdateCase exactly (see that method's identically-named
	// exclusiveCount/combinableCount) -- the two data sources must accept
	// the same field combinations, not two different rulebooks for the same
	// PATCH endpoint. State/Severity/WorkState/WatchList/AssigneeEmail/
	// ParentID/Acknowledge each carry their own side effects (billable-status
	// detection, watch-list replacement, first-write-wins acknowledgement,
	// ...) and are mutually exclusive both with each other and with every
	// "plain field" in the combinable bucket.
	exclusiveCount := 0
	if req.State != nil || req.Severity != nil || req.WorkState != nil {
		exclusiveCount++
	}
	if req.WatchList != nil {
		exclusiveCount++
	}
	if req.AssigneeEmail != nil {
		exclusiveCount++
	}
	if req.ParentID != nil {
		exclusiveCount++
	}
	if req.Acknowledge != nil {
		exclusiveCount++
	}
	combinableCount := 0
	if req.Subject != nil {
		combinableCount++
	}
	if req.Description != nil {
		combinableCount++
	}
	if req.DeploymentID != nil {
		combinableCount++
	}
	if req.DeployedProductID != nil {
		combinableCount++
	}
	if req.BestCaseFixEta != nil {
		combinableCount++
	}
	if req.MostLikelyFixEta != nil {
		combinableCount++
	}
	if req.WorstCaseFixEta != nil {
		combinableCount++
	}
	if req.RelatedCaseID != nil {
		combinableCount++
	}
	if req.WorkaroundProvided != nil {
		combinableCount++
	}
	const fieldList = "state, severity, workState, watchList, assigneeEmail, parentId, acknowledge, " +
		"subject, description, deploymentId, deployedProductId, bestCaseFixEta, mostLikelyFixEta, " +
		"worstCaseFixEta, relatedCaseId, or workaroundProvided"
	if exclusiveCount == 0 && combinableCount == 0 {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "at least one of " + fieldList + " must be provided"}
	}
	if exclusiveCount > 1 || (exclusiveCount == 1 && combinableCount > 0) {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "state, severity, workState, watchList, assigneeEmail, parentId, and acknowledge cannot be combined with each other or with any other field in the same request"}
	}
	// resolutionCode/cause/closeNotes ride along with a state change only --
	// same restriction sn_case_service.go's own UpdateCase enforces
	// (snResolutionStates: closed or solution_proposed only). They have no
	// meaning attached to a severity or workState change, and no meaning at
	// all without a state change in the same request. This must run before
	// the branch dispatch below: neither exclusiveCount nor combinableCount
	// counts these three fields at all (found by CodeRabbit review on
	// PR #1986), so a request like {assigneeEmail, resolutionCode} or
	// {subject, closeNotes} would otherwise sail past both checks above and
	// have its resolution fields silently dropped by whichever branch
	// handles the other field, never validated or written.
	if req.ResolutionCode != nil || req.Cause != nil || req.CloseNotes != nil {
		if req.State == nil {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "resolutionCode, cause, and closeNotes are only allowed when state is also provided"}
		}
		if *req.State != domain.CaseStateClosed && *req.State != domain.CaseStateSolutionProposed {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "resolutionCode, cause, and closeNotes are only allowed when state is closed or solution_proposed"}
		}
	}

	if req.WatchList != nil {
		return s.updateCaseWatchList(ctx, req)
	}
	if req.AssigneeEmail != nil {
		return s.updateCaseAssignee(ctx, req)
	}
	if req.ParentID != nil {
		return s.updateCaseParent(ctx, req)
	}
	if req.Acknowledge != nil {
		if !*req.Acknowledge {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "acknowledge only accepts true"}
		}
		return s.acknowledgeCase(ctx, req)
	}
	if combinableCount > 0 {
		return s.updateCaseFields(ctx, req)
	}

	if req.State != nil && !validCaseState[*req.State] {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "state contains invalid value: " + string(*req.State)}
	}
	if req.Severity != nil && !validCaseSeverity[*req.Severity] {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "severity contains invalid value: " + string(*req.Severity)}
	}
	if req.WorkState != nil && !validCaseWorkState[*req.WorkState] {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "workState contains invalid value: " + string(*req.WorkState)}
	}
	if req.Cause != nil && !validCaseCause[*req.Cause] {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "cause contains invalid value: " + string(*req.Cause)}
	}

	// before is the case's full view immediately prior to this update — used
	// to detect a genuine state/workState change (a caller re-PATCHing the
	// case's current state must not send every watcher a false "status
	// changed" notification, nor gain a spurious activity-feed entry, the
	// same guard snCaseService.UpdateCase already applies) and, on a genuine
	// state change, to build case.status_changed's payload
	// (Recipients/ProjectID/etc. — see publishStatusChangedEvent). Fetched
	// whenever either field is set, regardless of s.publisher: previously
	// this was skipped entirely when nothing could possibly publish, but
	// the activity-feed write below needs it either way, unlike the
	// publish-only concern this comment used to describe alone. A fetch
	// failure here is logged and treated as "skip the publish/activity
	// entry," not a failed update: the case update itself does not depend
	// on this.
	var before *domain.CaseView
	if req.State != nil || req.WorkState != nil {
		if cv, err := s.GetCaseByID(ctx, req.ID); err != nil {
			slog.ErrorContext(ctx, "update case: enrich case for case.status_changed publish/activity failed", "caseId", req.ID)
		} else {
			before = &cv
		}
	}

	// actorEmail is used only for this update's own activity-feed entry
	// below -- resolved best-effort, not required, since this branch has
	// never required an authenticated caller before now (no permission
	// model exists for Postgres-side case mutations yet -- see
	// updateCaseAssignee's own doc comment) and must not start rejecting a
	// caller who omits x-user-id-token just because this data source can
	// now also log field changes to work_item_activity.
	var actorEmail string
	if actor, err := s.resolveActor(ctx); err == nil {
		actorEmail = actor.Email
	}

	// oldSeverity is the case's severity immediately before this update —
	// accurate even under a concurrent update to the same case, since the
	// repository locks the row before reading it whenever req.Severity is
	// set (see CaseRepository.UpdateCase's own doc comment). Only meaningful
	// when req.Severity != nil; otherwise it's just the unchanged severity.
	c, oldSeverity, err := s.repo.UpdateCase(ctx, req)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}

	if req.Severity != nil {
		s.detectBillableStatusChange(ctx, req.ID, oldSeverity, c.Severity)
	}

	// Activity-feed logging follows the write, same as event publishing
	// below -- see CaseRepository.RecordCaseFieldChangeActivity's own doc
	// comment for why this table needed a write path at all on this data
	// source. State/Severity/WorkState are mutually exclusive on this
	// request (fieldCount above), so at most one of these three fires.
	if req.State != nil && before != nil && derefState(before.State) != *req.State {
		s.recordFieldChangeActivity(ctx, req.ID, "state", caseStateDisplayLabel[derefState(before.State)], caseStateDisplayLabel[*req.State], actorEmail)
	}
	if req.Severity != nil && c.Severity != nil && derefSeverity(oldSeverity) != *c.Severity {
		s.recordFieldChangeActivity(ctx, req.ID, "severity", humanizeSnakeCase(string(derefSeverity(oldSeverity))), humanizeSnakeCase(string(*c.Severity)), actorEmail)
	}
	if req.WorkState != nil && before != nil && c.WorkState != nil && derefWorkState(before.WorkState) != *c.WorkState {
		s.recordFieldChangeActivity(ctx, req.ID, "work_state", humanizeSnakeCase(string(derefWorkState(before.WorkState))), humanizeSnakeCase(string(*c.WorkState)), actorEmail)
	}

	// Event publishing follows the write, not DATA_SOURCE -- see
	// publishCaseCreatedEvent's own doc comment for why: whatever mode
	// actually recorded this change in Postgres is the one that should
	// notify about it, so a future data-source change doesn't silently stop
	// notifications from firing. State/Severity are mutually exclusive on
	// this request (fieldCount above), so at most one of these two fires.
	if req.State != nil && before != nil && derefState(before.State) != *req.State {
		if label, ok := caseStateDisplayLabel[*req.State]; ok {
			publishStatusChangedEvent(ctx, s.publisher, req.ID, label, *before)
		}
	}
	if req.Severity != nil && c.Severity != nil && derefSeverity(oldSeverity) != *c.Severity {
		if cv, err := s.GetCaseByID(ctx, req.ID); err != nil {
			slog.ErrorContext(ctx, "update case: enrich case for case.severity_changed publish failed", "caseId", req.ID)
		} else {
			publishSeverityChangedEvent(ctx, s.publisher, req.ID, string(derefSeverity(oldSeverity)), string(*c.Severity), cv)
		}
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only (snWriteback/snMirror are both nil otherwise — see
	// NewCaseServiceWithSNWriteback's own doc comment). Postgres has already
	// committed by this point; this fires after, asynchronously, and never
	// affects this response. UPDATE stays Postgres-first/async — unlike
	// CreateCase (see that method's own doc comment for why create is
	// SN-first/synchronous instead): a failed async mirror write here just
	// means ServiceNow's copy of an EXISTING, already-created case is stale
	// on one field until retried by hand, not a permanent orphan the way a
	// failed async case CREATE would be.
	//
	// Covers State/Severity/WorkState — fieldCount above guarantees at most
	// one of the three is set on req, so exactly one of these three branches
	// ever fires per call. State/Severity joined this mirror later than
	// WorkState did: snCaseService.UpdateCase (the ServiceNow-mode method
	// WorkState originally mirrored through) performs a live GetCaseByID
	// read against ServiceNow before its PATCH whenever State or Severity is
	// set, which this mode must never do. patchCaseFields (sn_case_service.go)
	// is the fix — a bare PATCH with none of UpdateCase's read/no-op-detection/
	// event-publish behavior — reached here through the snFieldPatcher
	// interface rather than the full snMirror.UpdateCase this method used to
	// call for WorkState. Each branch builds its own single-field mirror
	// request/payload rather than forwarding req itself, so this can never
	// accidentally carry a second field into the mirror call.
	if s.snWriteback != nil {
		if patcher, ok := s.snMirror.(snFieldPatcher); ok {
			switch {
			case req.State != nil:
				state := *req.State
				s.snWriteback.Dispatch(ctx, "case", req.ID, "update",
					map[string]any{"id": req.ID, "state": state},
					func(writeCtx context.Context) error {
						_, err := patcher.patchCaseFields(writeCtx, req.ID, &state, nil, nil)
						return err
					},
				)
			case req.Severity != nil:
				severity := *req.Severity
				s.snWriteback.Dispatch(ctx, "case", req.ID, "update",
					map[string]any{"id": req.ID, "severity": severity},
					func(writeCtx context.Context) error {
						_, err := patcher.patchCaseFields(writeCtx, req.ID, nil, &severity, nil)
						return err
					},
				)
			case req.WorkState != nil:
				workState := *req.WorkState
				s.snWriteback.Dispatch(ctx, "case", req.ID, "update",
					map[string]any{"id": req.ID, "workState": workState},
					func(writeCtx context.Context) error {
						_, err := patcher.patchCaseFields(writeCtx, req.ID, nil, nil, &workState)
						return err
					},
				)
			}
		}
	}

	return domain.UpdateCaseResponse{
		Message: "Case updated successfully",
		Case: domain.UpdatedCase{
			ID:        c.ID,
			UpdatedOn: c.UpdatedOn,
			State:     c.State,
			Severity:  c.Severity,
			WorkState: c.WorkState,
		},
	}, nil
}

// updateCaseWatchList implements UpdateCase's WatchList branch: replacing
// the case's watch list wholesale with req's user ids via
// CaseRepository.SetCaseWatchList. An explicitly empty (non-nil) WatchList
// clears the watch list -- validateUUIDs on an empty slice is a no-op, so
// that reaches the repository as an empty replacement, not an error.
func (s *caseService) updateCaseWatchList(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error) {
	userIDs := *req.WatchList
	if err := validateUUIDs("watchList", userIDs); err != nil {
		return domain.UpdateCaseResponse{}, err
	}

	actor, err := s.resolveActor(ctx)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}

	watchers, updatedOn, err := s.repo.SetCaseWatchList(ctx, req.ID, userIDs, actor.Email)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only (snWriteback/snMirror are both nil otherwise -- see
	// NewCaseServiceWithSNWriteback's own doc comment). Postgres has already
	// committed by this point; this fires after, asynchronously, and never
	// affects this response. Safe to mirror by id -- case CREATE is
	// ServiceNow-first under this data source (createCaseSNFirst), so req.ID
	// IS the real ServiceNow sys_id round-tripped through sysidToUUID.
	// patchCaseWatchList (sn_case_service.go) is a bare PATCH with none of
	// snCaseService.UpdateCase's own read-before-write/event-publish
	// behavior, reached through the snWatchListPatcher interface, same
	// pattern as the State/Severity/WorkState mirror above.
	if s.snWriteback != nil {
		if patcher, ok := s.snMirror.(snWatchListPatcher); ok {
			mirrorUserIDs := append([]string(nil), userIDs...)
			s.snWriteback.Dispatch(ctx, "case", req.ID, "update",
				map[string]any{"id": req.ID, "watchList": mirrorUserIDs},
				func(writeCtx context.Context) error {
					_, err := patcher.patchCaseWatchList(writeCtx, req.ID, mirrorUserIDs)
					return err
				},
			)
		}
	}

	return domain.UpdateCaseResponse{
		Message: "Case updated successfully",
		Case: domain.UpdatedCase{
			ID:        req.ID,
			UpdatedOn: updatedOn,
			WatchList: watchers,
		},
	}, nil
}

// updateCaseAssignee implements UpdateCase's AssigneeEmail branch: resolving
// the target user, writing work_item.assigned_to_id (migration 000036,
// already read by the assignedUserId search filter and GetCaseByID's own
// AssignedEngineer), and echoing the assignee back on both AssignedTo
// (ServiceNow-shaped) and AssignedToUser (the canonical reference, per that
// field's own doc comment). Unlike ServiceNow, this data source has no
// caller-role check to enforce here (no role/permission model exists for
// Postgres-side case mutations at all yet -- see AccessService's own "not
// yet wired" list) -- deliberately left unenforced rather than invented.
func (s *caseService) updateCaseAssignee(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error) {
	if *req.AssigneeEmail == "" {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "assigneeEmail must not be empty"}
	}
	assignee, err := s.userRepo.GetUserByEmail(ctx, *req.AssigneeEmail)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}
	actor, err := s.resolveActor(ctx)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}

	// previousAssigneeName is best-effort DISPLAY data only, for the
	// activity-feed entry's old value below -- fetched before the write, so
	// it can lag under a genuine concurrent race (another call reassigning
	// the case between this read and the write below), but that only means
	// an activity entry's "old" value is stale, never a duplicate
	// publish/activity write. Left "" (rather than a literal "Unassigned"
	// placeholder) when there was no previous assignee, or on a fetch
	// failure: CaseActivitiesFeed.tsx/describeAuditEntry (csm-portal webapp)
	// already render a field-change entry with no "from" arrow at all when
	// previousValue is empty, which reads as "Assigned to: X" rather than
	// the confusing "Assigned to: Unassigned -> X".
	previousAssigneeName := ""
	if cv, err := s.GetCaseByID(ctx, req.ID); err != nil {
		slog.ErrorContext(ctx, "update case: enrich case for case.assigned activity failed", "caseId", req.ID)
	} else if cv.AssignedEngineer != nil {
		previousAssigneeName = cv.AssignedEngineer.Name
	}

	// changed is the ONLY signal that gates the publish/activity-log calls
	// below, and comes from the write itself (CaseRepository.
	// UpdateCaseAssignee's own atomic UPDATE...WHERE assigned_to_id IS
	// DISTINCT FROM...RETURNING), not a separate pre-write read -- two
	// concurrent requests assigning the same case to the same engineer
	// can't both observe "unchanged" (or both "changed") this way, unlike a
	// check-then-act GetCaseByID comparison, which could let both calls see
	// a stale "not yet assigned" state and both publish/mirror the same
	// no-op, sending watchers a duplicate "case assigned" notification
	// (CodeRabbit finding on PR #1989).
	updatedOn, changed, err := s.repo.UpdateCaseAssignee(ctx, req.ID, assignee.ID, actor.Email)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}

	assigneeName := strings.TrimSpace(assignee.FirstName + " " + assignee.LastName)
	if assigneeName == "" {
		assigneeName = assignee.Email
	}

	// Event publishing/activity-feed logging both follow the write, not
	// DATA_SOURCE -- see publishCaseCreatedEvent's own doc comment for why.
	if changed {
		s.publishCaseAssigned(ctx, req.ID, assigneeName, assignee.Email)
		s.recordFieldChangeActivity(ctx, req.ID, "assigned_to_id", previousAssigneeName, assigneeName, actor.Email)
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only -- same Postgres-first/async posture as updateCaseWatchList's own
	// mirror above.
	if s.snWriteback != nil {
		if patcher, ok := s.snMirror.(snAssigneePatcher); ok {
			assigneeEmail := *req.AssigneeEmail
			s.snWriteback.Dispatch(ctx, "case", req.ID, "update",
				map[string]any{"id": req.ID, "assigneeEmail": assigneeEmail},
				func(writeCtx context.Context) error {
					return patcher.patchCaseAssignee(writeCtx, req.ID, assigneeEmail)
				},
			)
		}
	}

	return domain.UpdateCaseResponse{
		Message: "Case updated successfully",
		Case: domain.UpdatedCase{
			ID:             req.ID,
			UpdatedOn:      updatedOn,
			AssignedTo:     &domain.AssignedEngineerRef{ID: assignee.ID, Name: assigneeName, Email: &assignee.Email},
			AssignedToUser: domain.NewUserReference(assignee.ID, assignee.Email, assigneeName),
		},
	}, nil
}

// publishCaseAssigned mirrors snCaseService.publishCaseAssigned -- see that
// function's own doc comment for the full design rationale (Recipients is
// the case's watch list only, bounded by publishCaseAssignedTimeout).
// Unlike the ServiceNow version, this re-fetches the case AFTER the write:
// updateCaseAssignee's own pre-write GetCaseByID (above) exists only to
// detect the no-op case, not to reuse as a payload source, since
// cv.ProjectDetails/cv.WatchList don't change based on the assignment
// itself either way.
func (s *caseService) publishCaseAssigned(ctx context.Context, caseID, assigneeName, assigneeEmail string) {
	if s.publisher == nil || assigneeEmail == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, publishCaseAssignedTimeout)
	defer cancel()

	cv, err := s.GetCaseByID(ctx, caseID)
	if err != nil {
		slog.ErrorContext(ctx, "update case: enrich case for case.assigned publish failed", "caseId", caseID)
		return
	}

	recipients := watchListUserEmails(cv.WatchList)
	if len(recipients) == 0 {
		slog.InfoContext(ctx, "update case: case.assigned not published, case has no watchers to email", "caseId", caseID)
		return
	}

	// cv.ProjectDetails is nilable on this data source (unlike ServiceNow's
	// own CaseView, where it's always populated) -- see GetCaseByID's own
	// comment on why project/deployment joins are LEFT joins here.
	projectID := ""
	if cv.ProjectDetails != nil {
		projectID = cv.ProjectDetails.ID
	}

	payload, err := json.Marshal(events.CaseAssignedPayload{
		AssigneeName:  assigneeName,
		AssigneeEmail: assigneeEmail,
		ProjectID:     projectID,
		CaseID:        caseID,
		CaseNumber:    cv.Number,
		WSO2CaseID:    cv.InternalID,
		CaseTitle:     cv.Subject,
		Recipients:    recipients,
	})
	if err != nil {
		slog.ErrorContext(ctx, "update case: encode case.assigned payload failed", "caseId", caseID, "error", err)
		return
	}
	if err := s.publisher.Publish(ctx, events.TypeCaseAssigned, caseID, payload); err != nil {
		slog.ErrorContext(ctx, "update case: publish case.assigned failed", "caseId", caseID)
	}
}

// acknowledgeCase implements UpdateCase's Acknowledge branch: claiming the
// case for the calling engineer via CaseRepository.AcknowledgeCase's atomic
// "first write wins" semantics -- same idempotent contract
// domain.UpdateCaseRequest.Acknowledge's own doc comment documents for the
// ServiceNow data source. Like updateCaseAssignee, there is no caller-role
// check here (no Postgres-side permission model exists yet) -- deliberately
// unenforced rather than invented.
func (s *caseService) acknowledgeCase(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error) {
	actor, err := s.resolveActor(ctx)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}

	alreadyAcknowledged, ackBy, number, updatedOn, err := s.repo.AcknowledgeCase(ctx, req.ID, actor.ID, actor.Email)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}

	// Only publish/mirror when this call is the one that actually claimed
	// it -- a repeat Acknowledge:true against an already-acknowledged case
	// changed nothing in Postgres, so there's nothing new to react to. See
	// snCaseService.publishCaseAcknowledged's own doc comment for the same
	// AlreadyAcknowledged distinction on the ServiceNow data source.
	if !alreadyAcknowledged {
		s.publishCaseAcknowledged(ctx, req.ID, ackBy.Name)
		s.recordFieldChangeActivity(ctx, req.ID, "acknowledged_by_user_id", "", ackBy.Name, actor.Email)
	}

	// Only mirror to ServiceNow when this call is the one that actually
	// claimed it -- a repeat Acknowledge:true against an already-acknowledged
	// case changed nothing in Postgres, so there's nothing new to mirror.
	if !alreadyAcknowledged && s.snWriteback != nil {
		if patcher, ok := s.snMirror.(snAcknowledgePatcher); ok {
			s.snWriteback.Dispatch(ctx, "case", req.ID, "update",
				map[string]any{"id": req.ID, "acknowledge": true},
				func(writeCtx context.Context) error {
					return patcher.patchCaseAcknowledge(writeCtx, req.ID)
				},
			)
		}
	}

	return domain.UpdateCaseResponse{
		Message: "Case updated successfully",
		Case: domain.UpdatedCase{
			ID:                  req.ID,
			UpdatedOn:           updatedOn,
			Number:              number,
			AlreadyAcknowledged: &alreadyAcknowledged,
			AcknowledgedBy:      &ackBy,
		},
	}, nil
}

// publishCaseAcknowledged mirrors snCaseService.publishCaseAcknowledged --
// see that function's own doc comment for the full design rationale
// (Chat-only, no Recipients/email reaction; bounded by
// publishCaseAcknowledgedTimeout). acknowledgerName comes straight from
// CaseRepository.AcknowledgeCase's own return value (ackBy.Name) rather
// than a second lookup -- unlike ServiceNow, this data source computed that
// identity itself in the same round trip that performed the claim.
func (s *caseService) publishCaseAcknowledged(ctx context.Context, caseID, acknowledgerName string) {
	if s.publisher == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, publishCaseAcknowledgedTimeout)
	defer cancel()

	cv, err := s.GetCaseByID(ctx, caseID)
	if err != nil {
		slog.ErrorContext(ctx, "update case: enrich case for case.acknowledged publish failed", "caseId", caseID)
		return
	}

	payload, err := json.Marshal(events.CaseAcknowledgedPayload{
		CaseID:           caseID,
		CaseNumber:       cv.Number,
		WSO2CaseID:       cv.InternalID,
		Severity:         strings.ToUpper(string(derefSeverity(cv.Severity))),
		Product:          caseProductName(cv),
		Team:             caseTeamName(cv),
		AcknowledgerName: acknowledgerName,
	})
	if err != nil {
		slog.ErrorContext(ctx, "update case: encode case.acknowledged payload failed", "caseId", caseID, "error", err)
		return
	}
	if err := s.publisher.Publish(ctx, events.TypeCaseAcknowledged, caseID, payload); err != nil {
		slog.ErrorContext(ctx, "update case: publish case.acknowledged failed", "caseId", caseID)
	}
}

// updateCaseParent implements UpdateCase's ParentID branch: writing
// work_item.parent_id via CaseRepository.UpdateCaseParent. Its own dedicated
// branch, not the field bundle below, because sn_case_service.go's own
// UpdateCase keeps parentId fully exclusive of every other field -- see this
// file's exclusiveCount/combinableCount split in UpdateCase itself.
func (s *caseService) updateCaseParent(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error) {
	if err := validateUUIDs("parentId", []string{*req.ParentID}); err != nil {
		return domain.UpdateCaseResponse{}, err
	}
	actor, err := s.resolveActor(ctx)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}

	// oldParentNumber is best-effort display data for the activity-feed
	// entry below only -- a fetch failure just means that entry's old value
	// comes back empty, never a reason to fail the parent change itself.
	oldParentNumber := ""
	if before, err := s.GetCaseByID(ctx, req.ID); err == nil && before.ParentCase != nil {
		oldParentNumber = before.ParentCase.Number
	}

	updatedOn, err := s.repo.UpdateCaseParent(ctx, req.ID, *req.ParentID, actor.Email)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}

	newParentNumber := ""
	if newParent, err := s.GetCaseByID(ctx, *req.ParentID); err == nil {
		newParentNumber = newParent.Number
	}
	s.recordFieldChangeActivity(ctx, req.ID, "parent_id", oldParentNumber, newParentNumber, actor.Email)

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only -- same Postgres-first/async posture as every sibling branch above.
	if s.snWriteback != nil {
		if patcher, ok := s.snMirror.(snParentPatcher); ok {
			parentID := *req.ParentID
			s.snWriteback.Dispatch(ctx, "case", req.ID, "update",
				map[string]any{"id": req.ID, "parentId": parentID},
				func(writeCtx context.Context) error {
					return patcher.patchCaseParent(writeCtx, req.ID, parentID)
				},
			)
		}
	}

	return domain.UpdateCaseResponse{
		Message: "Case updated successfully",
		Case:    domain.UpdatedCase{ID: req.ID, UpdatedOn: updatedOn},
	}, nil
}

// updateCaseFields implements UpdateCase's combinable "plain field" bundle --
// any subset of Subject/Description/DeploymentID/DeployedProductID/
// BestCaseFixEta/MostLikelyFixEta/WorstCaseFixEta/RelatedCaseID/
// WorkaroundProvided, mirroring sn_case_service.go's own UpdateCase
// combinableCount group exactly (see UpdateCase's own doc comment for why
// resolutionCode/cause/closeNotes/issueType are deliberately NOT among
// them). Validates the fields this data source can express before writing
// (UUIDs, YYYY-MM-DD dates); the ones it doesn't check here are validated
// downstream anyway by the FK/date-cast Postgres itself enforces.
func (s *caseService) updateCaseFields(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error) {
	if req.DeploymentID != nil {
		if err := validateUUIDs("deploymentId", []string{*req.DeploymentID}); err != nil {
			return domain.UpdateCaseResponse{}, err
		}
	}
	if req.DeployedProductID != nil {
		if err := validateUUIDs("deployedProductId", []string{*req.DeployedProductID}); err != nil {
			return domain.UpdateCaseResponse{}, err
		}
	}
	if req.RelatedCaseID != nil {
		if err := validateUUIDs("relatedCaseId", []string{*req.RelatedCaseID}); err != nil {
			return domain.UpdateCaseResponse{}, err
		}
	}
	if req.BestCaseFixEta != nil {
		if err := validateDateOnly("bestCaseFixEta", *req.BestCaseFixEta); err != nil {
			return domain.UpdateCaseResponse{}, err
		}
	}
	if req.MostLikelyFixEta != nil {
		if err := validateDateOnly("mostLikelyFixEta", *req.MostLikelyFixEta); err != nil {
			return domain.UpdateCaseResponse{}, err
		}
	}
	if req.WorstCaseFixEta != nil {
		if err := validateDateOnly("worstCaseFixEta", *req.WorstCaseFixEta); err != nil {
			return domain.UpdateCaseResponse{}, err
		}
	}

	// The actor is only needed to stamp workaround_provided_by_user_id and
	// work_item.updated_by -- resolved once regardless, since updated_by is
	// always written.
	actor, err := s.resolveActor(ctx)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}

	updatedOn, err := s.repo.UpdateCaseFields(ctx, req, actor.ID, actor.Email)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only -- same Postgres-first/async posture as every sibling branch
	// above. The recorded payload carries the actual values patchCaseFieldsBundle
	// forwards to ServiceNow (only the four fields that backing service
	// actually supports -- see that method's own doc comment), not a fixed
	// field-name placeholder, so a manual replay off sn_writeback_failures
	// has something to replay.
	if s.snWriteback != nil {
		if patcher, ok := s.snMirror.(snFieldsBundlePatcher); ok {
			writebackPayload := map[string]any{"id": req.ID}
			if req.BestCaseFixEta != nil {
				writebackPayload["bestCaseFixEta"] = *req.BestCaseFixEta
			}
			if req.MostLikelyFixEta != nil {
				writebackPayload["mostLikelyFixEta"] = *req.MostLikelyFixEta
			}
			if req.WorstCaseFixEta != nil {
				writebackPayload["worstCaseFixEta"] = *req.WorstCaseFixEta
			}
			if req.WorkaroundProvided != nil {
				writebackPayload["workaroundProvided"] = *req.WorkaroundProvided
			}
			s.snWriteback.Dispatch(ctx, "case", req.ID, "update", writebackPayload,
				func(writeCtx context.Context) error {
					return patcher.patchCaseFieldsBundle(writeCtx, req.ID, req)
				},
			)
		}
	}

	resp := domain.UpdatedCase{ID: req.ID, UpdatedOn: updatedOn}
	// BestCaseFixEta/MostLikelyFixEta/WorstCaseFixEta are echoed back only
	// when the update set them -- see UpdatedCase's own doc comment on each.
	// Every other field in this bundle follows ServiceNow's own "a plain
	// field write only returns {id, updatedOn, updatedBy}" contract (see
	// WorkaroundProvided's doc comment on UpdatedCase) -- the caller re-reads
	// via GetCaseByID to see the new value.
	resp.BestCaseFixEta = req.BestCaseFixEta
	resp.MostLikelyFixEta = req.MostLikelyFixEta
	resp.WorstCaseFixEta = req.WorstCaseFixEta
	return domain.UpdateCaseResponse{Message: "Case updated successfully", Case: resp}, nil
}

// detectBillableStatusChange checks whether a severity update just crossed
// the LOW boundary in either direction — entering LOW means every time
// card on this case should become billable, leaving it means they should
// become non-billable (see events.CaseBillableStatusChangedPayload's own
// doc comment for why LOW is the one severity that matters here). A
// Postgres-backed case's Type is always "case" and can never change (see
// this file's own UpdateCase, which rejects req.Type entirely on this data
// source), so unlike the ServiceNow data source this reduces to a single
// severity comparison — no Type-transition case to handle.
//
// Publishing events.TypeCaseBillableStatusChanged is commented out below
// rather than live — see that type's own doc comment: nothing consumes it
// yet (Postgres has no time_cards table/repo/service at all today), so
// publishing now would produce an event nothing acts on. The detection
// itself is real; only the actual Publish call is inert.
func (s *caseService) detectBillableStatusChange(ctx context.Context, caseID string, oldSeverity, newSeverity *domain.CaseSeverity) {
	oldLow := oldSeverity != nil && *oldSeverity == domain.CaseSeverityLow
	newLow := newSeverity != nil && *newSeverity == domain.CaseSeverityLow
	if oldLow == newLow {
		return
	}
	isBillable := newLow

	// TODO: enable once a consumer exists for events.TypeCaseBillableStatusChanged
	// (bulk-flipping every time card's IsBillable for caseId) — see that
	// type's own doc comment for what's still missing.
	//
	// payload, err := json.Marshal(events.CaseBillableStatusChangedPayload{CaseID: caseID, IsBillable: isBillable})
	// if err != nil {
	// 	slog.ErrorContext(ctx, "case update: encode case.billable_status_changed payload failed", "caseId", caseID, "error", err)
	// 	return
	// }
	// if s.publisher == nil {
	// 	return
	// }
	// if err := s.publisher.Publish(ctx, events.TypeCaseBillableStatusChanged, caseID, payload); err != nil {
	// 	slog.ErrorContext(ctx, "case update: publish case.billable_status_changed failed", "caseId", caseID)
	// }

	slog.InfoContext(ctx, "case update: severity crossed the billable boundary, event hub publish not yet enabled", "caseId", caseID, "isBillable", isBillable)
}

// validateCaseFieldValues rejects malformed ids and unknown enum spellings in the
// fields a case search accepts both at the top level and inside an anyOf branch,
// so they fail as a validation error instead of reaching SQL as a cast error.
func validateCaseFieldValues(g domain.CaseFilterGroup) error {
	if err := validateUUIDs("projectId", g.ProjectIDs); err != nil {
		return err
	}
	if err := validateUUIDs("deploymentId", g.DeploymentIDs); err != nil {
		return err
	}
	for _, t := range g.Types {
		if !validCaseType[t] {
			return &apierror.ValidationError{Msg: "type contains invalid value: " + t}
		}
	}
	for _, st := range g.States {
		if !validCaseState[st] {
			return &apierror.ValidationError{Msg: "state contains invalid value: " + string(st)}
		}
	}
	for _, sv := range g.Severities {
		if !validCaseSeverity[sv] {
			return &apierror.ValidationError{Msg: "severity contains invalid value: " + string(sv)}
		}
	}
	for _, it := range g.IssueTypes {
		if !validCaseIssueType[it] {
			return &apierror.ValidationError{Msg: "issueType contains invalid value: " + string(it)}
		}
	}
	for _, et := range g.EngagementTypes {
		if !validEngagementType[et] {
			return &apierror.ValidationError{Msg: "engagementType contains invalid value: " + string(et)}
		}
	}
	for _, ws := range g.WorkStates {
		if !validCaseWorkState[ws] {
			return &apierror.ValidationError{Msg: "workState contains invalid value: " + string(ws)}
		}
	}
	return validateUUIDs("assignedUserId", g.AssignedUserIDs)
}

// SearchCases implements CaseService.
func (s *caseService) SearchCases(ctx context.Context, req domain.SearchCasesRequest) (domain.SearchCasesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCasesResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchCasesResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	callerEmail, callerEmailErr := resolveCaseFilterCallerEmail(token)
	parsed, err := ParseCaseFieldFilters(req.Filters.Filters, callerEmail, callerEmailErr, time.Now().UTC())
	if err != nil {
		return domain.SearchCasesResponse{}, err
	}

	if err := validateUUIDs("projectId", parsed.ExcludeProjectIDs); err != nil {
		return domain.SearchCasesResponse{}, err
	}
	if parsed.ParentID != nil {
		if err := validateUUIDs("parentId", []string{*parsed.ParentID}); err != nil {
			return domain.SearchCasesResponse{}, err
		}
	}
	// The same checks apply to the top-level fields and to each anyOf branch, so
	// they live in one function.
	if err := validateCaseFieldValues(domain.CaseFilterGroup{
		Types: parsed.Types, States: parsed.States, Severities: parsed.Severities,
		EngagementTypes: parsed.EngagementTypes, IssueTypes: parsed.IssueTypes,
		WorkStates: parsed.WorkStates, ProjectIDs: parsed.ProjectIDs,
		DeploymentIDs: parsed.DeploymentIDs, AssignedUserIDs: parsed.AssignedUserIDs,
	}); err != nil {
		return domain.SearchCasesResponse{}, err
	}

	// anyOf branches: parse into OR groups (only the ServiceNow adapter did this
	// before) and validate each branch's values exactly like the top level.
	orGroups, err := ParseCaseFieldFilterGroups(req.Filters.AnyOf)
	if err != nil {
		return domain.SearchCasesResponse{}, err
	}
	for _, g := range orGroups {
		if err := validateCaseFieldValues(g); err != nil {
			return domain.SearchCasesResponse{}, err
		}
	}
	parsed.OrGroups = orGroups

	if parsed.CreatedByMe {
		parsed.CreatedBy = append(parsed.CreatedBy, callerEmail)
	}

	if parsed.ClosedEndDate != nil && parsed.ClosedStartDate != nil &&
		parsed.ClosedEndDate.Before(*parsed.ClosedStartDate) {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "closedOn: lte value must not be before gte value"}
	}
	if parsed.EndCreatedDate != nil && parsed.StartCreatedDate != nil &&
		parsed.EndCreatedDate.Before(*parsed.StartCreatedDate) {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "createdOn: lte value must not be before gte value"}
	}
	if parsed.EndUpdatedDate != nil && parsed.StartUpdatedDate != nil &&
		parsed.EndUpdatedDate.Before(*parsed.StartUpdatedDate) {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "updatedOn: lte value must not be before gte value"}
	}
	// resolvedOn has no backing column in the relational schema and
	// caseRepo.SearchCases models no predicate for it, so accepting it here
	// would drop the bound silently and answer 200 with every case rather
	// than the resolved-in-range ones asked for. Reject, same as every other
	// predicate this data source cannot express.
	if parsed.ResolvedStartDate != nil || parsed.ResolvedEndDate != nil {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "resolvedOn" is not supported by this data source`}
	}

	// These fields dot-walk into ServiceNow-specific concepts (product family,
	// project type, integration-CS/SRE team, etc.) that caseRepo.SearchCases has
	// no query for today. Reject rather than silently drop the predicate and
	// widen the result set. (tag, projectOnboardingStatus and
	// taskSLABusinessElapsedPercent are implemented there, so are absent here.)
	// state+in is supported here; state+notIn has no repository query support,
	// and dropping an exclusion silently would widen the result set.
	// parentId is also implemented (wi.parent_id, migration 000036 -- the
	// "Linked Items" tab's child-case lookup), so it too is absent here.
	if len(parsed.ExcludeStates) > 0 {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "state" (notIn) is not supported by this data source`}
	}
	if len(parsed.ProductNames) > 0 {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "product" is not supported by this data source`}
	}
	if len(parsed.ProjectTypeNames) > 0 {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "projectType" is not supported by this data source`}
	}
	if len(parsed.CreTeamIDs) > 0 {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "creTeam" is not supported by this data source`}
	}
	if len(parsed.SreTeamIDs) > 0 {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "sreTeam" is not supported by this data source`}
	}
	// accountId+in has no repository query support today either (see
	// domain.ParsedCaseFilters.AccountIDs); accountId+notIn is rejected the
	// same way rather than silently dropping the exclusion and widening the
	// result set.
	if len(parsed.ExcludeAccountIDs) > 0 {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "accountId" (notIn) is not supported by this data source`}
	}
	if parsed.Unassigned {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "assignedUserId" (isEmpty) is not supported by this data source`}
	}
	if parsed.ResolutionNotesEmpty {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "resolutionNotes" is not supported by this data source`}
	}

	// The slaBreached and account-escalation predicates and grouped counts are
	// implemented only in the ServiceNow case service (snCaseService.SearchCases);
	// caseRepo.SearchCases models none of them (tag, projectOnboardingStatus,
	// taskSLABusinessElapsedPercent, escalationLevel, escalation and anyOf, by
	// contrast, are implemented there and so are deliberately absent from these
	// guards). ParseCaseFieldFilters accepts them
	// because it is shared by both data sources, so without these guards a
	// Postgres deployment would drop the predicate and answer 200 with a wider
	// result set than the caller asked for. These stay ServiceNow-only by design:
	// reject loudly rather than implement them here.
	if parsed.HasBreachedSLA != nil {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "slaBreached" is not supported by this data source`}
	}
	if parsed.HasActiveAccountEscalation != nil {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: `field "accountEscalationActive" is not supported by this data source`}
	}
	if req.GroupBy != "" {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "groupBy is not supported by this data source"}
	}

	req.Parsed = parsed

	if req.SortBy.Field == "" {
		req.SortBy.Field = domain.CaseSortFieldCreatedOn
	} else if !validCaseSortField[req.SortBy.Field] {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "sortBy.field must be one of: createdOn, updatedOn, severity, state"}
	}
	if req.SortBy.Order == "" {
		req.SortBy.Order = domain.CaseSortOrderDesc
	} else if !validCaseSortOrder[req.SortBy.Order] {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "sortBy.order must be one of: asc, desc"}
	}

	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return domain.SearchCasesResponse{}, err
	}

	cases, total, err := s.repo.SearchCases(ctx, req, scope)
	if err != nil {
		return domain.SearchCasesResponse{}, err
	}

	return domain.SearchCasesResponse{
		Cases:  cases,
		Total:  total,
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
	}, nil
}

func (s *caseService) AggregateCases(_ context.Context, _ domain.AggregateCasesRequest) (domain.AggregateResponse, error) {
	return domain.AggregateResponse{}, &apierror.ServiceUnavailableError{Msg: "groupBy is only supported for the ServiceNow data source"}
}

// resolveActor authenticates the caller from the x-user-id-token header
// carried on ctx and resolves it to a platform user record. It is the same
// authentication step CreateCaseComment above already performs -- there is no
// finer-grained per-case ACL check in this data source's case service beyond
// "the caller must be a known, authenticated user," so attachment mutations
// reuse it verbatim rather than inventing a new authorization pattern.
func (s *caseService) resolveActor(ctx context.Context) (domain.User, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.User{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return domain.User{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	return s.userRepo.GetUserByEmail(ctx, email)
}

// recordFieldChangeActivity is a best-effort wrapper around
// CaseRepository.RecordCaseFieldChangeActivity: a failure here is logged and
// otherwise ignored, since the case mutation that triggered it already
// succeeded and must not be undone -- or reported to the caller as failed --
// just because its own activity-feed entry couldn't be written. actorEmail
// empty is treated as "nothing to attribute this to" and skips the write
// entirely, rather than inserting a row with a blank user_email (see
// UpdateCase's own state/severity/workState branch for why actorEmail can be
// empty there).
func (s *caseService) recordFieldChangeActivity(ctx context.Context, caseID, fieldName, oldValue, newValue, actorEmail string) {
	if actorEmail == "" {
		return
	}
	if err := s.repo.RecordCaseFieldChangeActivity(ctx, caseID, fieldName, oldValue, newValue, actorEmail); err != nil {
		slog.ErrorContext(ctx, "update case: record field change activity failed", "caseId", caseID, "field", fieldName, "error", err)
	}
}

// humanizeSnakeCase renders a lowercase snake_case domain enum value (e.g.
// "ongoing", "catastrophic") as a space-separated Title Case display string
// (e.g. "Ongoing", "Catastrophic") for the case activity feed -- the raw
// value read poorly there next to "state"'s own caseStateDisplayLabel
// lookup, which this doesn't replace: a couple of that map's labels don't
// title-case cleanly (e.g. "waiting_on_wso2" -> "Waiting on WSO2", not
// "Waiting On Wso2"), but severity/workState's values are single words with
// no such irregularity, so a plain generic rendering is enough for them.
func humanizeSnakeCase(value string) string {
	words := strings.Split(value, "_")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// CreateCaseAttachment implements CaseService for the CSM-native (Postgres)
// data source. Unlike ServiceNow, this data source never receives file bytes
// directly: the caller must have already uploaded the file to SFTPGo and
// supplies its storage_key plus the size/name/type metadata. Only
// ReferenceTypeCase is supported -- the other ReferenceType values
// (conversation, change_request, deployment, incident) have no Postgres
// schema backing on this data source.
func (s *caseService) CreateCaseAttachment(ctx context.Context, req domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error) {
	if err := validateUUIDs("referenceId", []string{req.ReferenceID}); err != nil {
		return domain.CreateAttachmentResponse{}, err
	}
	if req.ReferenceType != domain.ReferenceTypeCase {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "referenceType must be 'case' for this data source"}
	}
	if req.Name == "" {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "name is required"}
	}
	if req.Type == "" {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "type is required"}
	}
	if req.StorageKey == nil || *req.StorageKey == "" {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "storageKey is required: this data source has no base64 payload alternative, the file must already be uploaded to SFTPGo"}
	}
	if req.SizeBytes <= 0 {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "sizeBytes must be greater than zero"}
	}
	// Empty defaults to complete: every caller before this change (and every
	// existing Postgres-path caller that doesn't know about the pending
	// state) gets exactly today's behavior. Only a caller that explicitly
	// wants the two-step upload flow passes "pending".
	switch req.Status {
	case "":
		req.Status = domain.AttachmentStatusComplete
	case domain.AttachmentStatusPending, domain.AttachmentStatusComplete:
		// valid
	default:
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid status %q: must be 'pending' or 'complete'", req.Status)}
	}
	if err := s.authorizeCaseAccess(ctx, req.ReferenceID); err != nil {
		return domain.CreateAttachmentResponse{}, err
	}

	user, err := s.resolveActor(ctx)
	if err != nil {
		return domain.CreateAttachmentResponse{}, err
	}
	req.CreatedBy = user.ID

	a, err := s.repo.CreateCaseAttachment(ctx, req)
	if err != nil {
		return domain.CreateAttachmentResponse{}, err
	}

	return domain.CreateAttachmentResponse{
		Message: "Attachment created successfully",
		Attachment: domain.AttachmentDetail{
			ID:         a.ID,
			SizeBytes:  a.SizeBytes,
			CreatedOn:  a.CreatedOn,
			CreatedBy:  user.Email,
			StorageKey: a.StorageKey,
			Status:     a.Status,
			// No DownloadURL: this service holds no bytes for a Postgres-sourced
			// attachment, only its storage_key. Resolving storage_key to an
			// actual download location is the downstream CSM backend's job.
		},
	}, nil
}

// ConfirmCaseAttachment implements CaseService for the CSM-native (Postgres)
// data source. It is the second half of the two-step upload flow: the caller
// (the CSM backend) registers a 'pending' row via CreateCaseAttachment
// *before* minting an SFTPGo upload credential, then calls this once the
// browser reports the upload succeeded, transitioning the row to 'complete'.
//
// Ownership: unlike UpdateAttachment/DeleteCaseAttachment (which any
// authenticated user may perform on any case attachment -- there is no
// per-resource ACL in this data source beyond authentication, see
// resolveActor's doc comment), confirming is restricted to the same actor
// who created the pending row. A pending row represents an upload a specific
// user initiated; there is no legitimate case yet for a different user to
// confirm it on their behalf, and allowing it would let any authenticated
// user "complete" an attachment they never uploaded.
func (s *caseService) ConfirmCaseAttachment(ctx context.Context, id string) (domain.ConfirmAttachmentResponse, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.ConfirmAttachmentResponse{}, err
	}

	user, err := s.resolveActor(ctx)
	if err != nil {
		return domain.ConfirmAttachmentResponse{}, err
	}

	existing, err := s.authorizeAttachmentAccess(ctx, id)
	if err != nil {
		return domain.ConfirmAttachmentResponse{}, err
	}
	if existing.CreatedBy == nil || existing.CreatedBy.ID == nil || *existing.CreatedBy.ID != user.ID {
		return domain.ConfirmAttachmentResponse{}, &apierror.ForbiddenError{Msg: "attachment was not created by the current user"}
	}
	if existing.Status != domain.AttachmentStatusPending {
		return domain.ConfirmAttachmentResponse{}, &apierror.ConflictError{Msg: fmt.Sprintf("attachment is not pending (current status: %q)", existing.Status)}
	}

	a, err := s.repo.ConfirmCaseAttachment(ctx, id)
	if err != nil {
		return domain.ConfirmAttachmentResponse{}, err
	}

	return domain.ConfirmAttachmentResponse{
		Message: "Attachment confirmed successfully",
		Attachment: domain.AttachmentDetail{
			ID:         a.ID,
			SizeBytes:  a.SizeBytes,
			CreatedOn:  a.CreatedOn,
			CreatedBy:  user.Email,
			StorageKey: a.StorageKey,
			Status:     a.Status,
		},
	}, nil
}

// SearchCaseAttachments implements CaseService for the CSM-native (Postgres)
// data source.
//
// Read-path status decision: the underlying repository query filters out
// 'pending' rows entirely (see caseRepo.SearchCaseAttachments), so a case's
// attachment list never shows a still-uploading placeholder to other users.
// This is a deliberate product-behavior choice, not an oversight: a pending
// row may never complete (the upload could fail, or the tab could just
// close), and showing it in a shared list before that's known risks other
// team members seeing and trying to act on a file that doesn't exist yet. A
// specific-id lookup (GetAttachmentByID) is not filtered this way -- it
// still returns a pending row -- which is what the confirm step relies on,
// and is also how an uploader could be shown their own in-flight upload if
// the FE chooses to poll it directly rather than via this list.
func (s *caseService) SearchCaseAttachments(ctx context.Context, req domain.SearchAttachmentsRequest) (domain.SearchAttachmentsResponse, error) {
	if err := validateUUIDs("referenceId", []string{req.ReferenceID}); err != nil {
		return domain.SearchAttachmentsResponse{}, err
	}
	if req.ReferenceType != domain.ReferenceTypeCase {
		return domain.SearchAttachmentsResponse{}, &apierror.ValidationError{Msg: "referenceType must be 'case' for this data source"}
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchAttachmentsResponse{}, err
	}
	if err := s.authorizeCaseAccess(ctx, req.ReferenceID); err != nil {
		return domain.SearchAttachmentsResponse{}, err
	}

	attachments, total, err := s.repo.SearchCaseAttachments(ctx, req.ReferenceID, req.Pagination)
	if err != nil {
		return domain.SearchAttachmentsResponse{}, err
	}

	return domain.SearchAttachmentsResponse{
		Attachments: attachments,
		Total:       total,
		Limit:       req.Pagination.Limit,
		Offset:      req.Pagination.Offset,
		HasMore:     req.Pagination.Offset+len(attachments) < total,
	}, nil
}

// SearchCaseActivities implements CaseService.
//
// Merges comments, complete attachments, and (when req.IncludeFieldChanges
// is true) field-change entries into one feed -- see
// CaseRepository.SearchCaseActivities's own doc comment for how
// work_item_activity (migration 000056) backs the field-change branch.
func (s *caseService) SearchCaseActivities(ctx context.Context, req domain.SearchCaseActivitiesRequest) (domain.SearchCaseActivitiesResponse, error) {
	if err := validateUUIDs("caseId", []string{req.CaseID}); err != nil {
		return domain.SearchCaseActivitiesResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCaseActivitiesResponse{}, err
	}
	if err := s.authorizeCaseAccess(ctx, req.CaseID); err != nil {
		return domain.SearchCaseActivitiesResponse{}, err
	}

	activity, total, err := s.repo.SearchCaseActivities(ctx, req)
	if err != nil {
		return domain.SearchCaseActivitiesResponse{}, err
	}

	return domain.SearchCaseActivitiesResponse{
		Activity: activity,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(activity) < total,
	}, nil
}

// GetCaseAttachmentContent implements CaseService for the CSM-native
// (Postgres) data source. This service never holds the file bytes for a
// Postgres-sourced attachment -- they live in SFTPGo, addressed by the
// attachment's storage_key (see GetAttachmentByID / SearchCaseAttachments).
// Callers must resolve content externally via that storage_key rather than
// through this endpoint.
func (s *caseService) GetCaseAttachmentContent(_ context.Context, _ string) ([]byte, string, error) {
	return nil, "", &apierror.ServiceUnavailableError{Msg: "this data source does not serve attachment bytes directly; resolve content via the attachment's storageKey"}
}

// DeleteCaseAttachment implements CaseService for the CSM-native (Postgres)
// data source. This only removes the metadata row -- it does not delete the
// backing SFTPGo file, which is the downstream CSM backend's responsibility
// when it also calls SFTPGo's own delete API.
func (s *caseService) DeleteCaseAttachment(ctx context.Context, req domain.DeleteAttachmentRequest) (domain.DeleteAttachmentResponse, error) {
	if err := validateUUIDs("attachmentId", []string{req.AttachmentID}); err != nil {
		return domain.DeleteAttachmentResponse{}, err
	}
	if _, err := s.resolveActor(ctx); err != nil {
		return domain.DeleteAttachmentResponse{}, err
	}
	if _, err := s.authorizeAttachmentAccess(ctx, req.AttachmentID); err != nil {
		return domain.DeleteAttachmentResponse{}, err
	}
	if err := s.repo.DeleteCaseAttachment(ctx, req.AttachmentID); err != nil {
		return domain.DeleteAttachmentResponse{}, err
	}
	return domain.DeleteAttachmentResponse{Message: "Attachment deleted successfully"}, nil
}

func (s *caseService) GetAttachment(_ context.Context, _ string) (domain.Attachment, error) {
	return domain.Attachment{}, &apierror.ServiceUnavailableError{Msg: "attachments are only supported for the ServiceNow data source"}
}

// AddCaseTag implements CaseService.
//
// Persists via tag/work_item_tag (migration 000021), added after this
// method was written as a detection-only stub (see
// detectPatchTagBillableOverride's own doc comment for that history) — it
// now actually attaches label to caseID, idempotently (a repeat call for an
// already-attached label, case-insensitively, returns the existing tag
// rather than erroring or duplicating). The "patch" + LOW-severity detection
// still only logs: (a) case tags having real storage is now true, but (b)
// no consumer exists yet for events.TypeCaseBillableStatusChanged (bulk-
// flipping every time card's IsBillable for caseId), so the actual publish
// stays commented out in detectPatchTagBillableOverride until that exists.
func (s *caseService) AddCaseTag(ctx context.Context, caseID, label string) (domain.Tag, error) {
	actor, err := s.resolveActor(ctx)
	if err != nil {
		return domain.Tag{}, err
	}
	return s.addCaseTagAs(ctx, caseID, label, actor.Email)
}

// AddCaseTagAs implements CaseService for a caller that already knows the
// acting email and has no x-user-id-token to resolve one from -- see the
// CaseService interface's own doc comment on this method.
func (s *caseService) AddCaseTagAs(ctx context.Context, caseID, label, actorEmail string) (domain.Tag, error) {
	return s.addCaseTagAs(ctx, caseID, label, actorEmail)
}

// addCaseTagAs is the shared validation/attach logic behind both
// AddCaseTag (token-resolved actor) and AddCaseTagAs (caller-supplied
// actor) -- everything past actor resolution is identical between the two.
func (s *caseService) addCaseTagAs(ctx context.Context, caseID, label, actorEmail string) (domain.Tag, error) {
	if err := validateUUIDs("caseId", []string{caseID}); err != nil {
		return domain.Tag{}, err
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return domain.Tag{}, &apierror.ValidationError{Msg: "label is required"}
	}
	if len(label) > 255 {
		return domain.Tag{}, &apierror.ValidationError{Msg: "label must not exceed 255 characters"}
	}
	if err := s.authorizeCaseAccess(ctx, caseID); err != nil {
		return domain.Tag{}, err
	}

	s.detectPatchTagBillableOverride(ctx, caseID, label)

	tag, err := s.repo.AddCaseTag(ctx, caseID, label, actorEmail)
	if err != nil {
		return domain.Tag{}, err
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only (snWriteback/snMirror are both nil otherwise -- see
	// NewCaseServiceWithSNWriteback's own doc comment). Postgres has already
	// committed by this point; this fires after, asynchronously, and never
	// affects this response. Mirrors by label, not by the Postgres tag id
	// (which has no ServiceNow counterpart -- SN's own tag/label-entry id is
	// generated independently on its own POST) -- snMirror.AddCaseTagAs
	// (sn_case_service.go) is already a bare, idempotent-by-label POST with
	// no GET-before-write or notification side effects, so it's called
	// directly here through the full CaseService interface rather than a
	// narrower one, same as snMirror.CreateCase above.
	//
	// RemoveCaseTag has NO equivalent mirror (see its own doc comment for
	// why): it identifies the tag to remove by the Postgres tag id alone,
	// and there is no stored mapping from that id to ServiceNow's own tag
	// sys_id to remove there too.
	if s.snWriteback != nil {
		mirrorCaseID, mirrorLabel, mirrorActorEmail := caseID, label, actorEmail
		s.snWriteback.Dispatch(ctx, "case_tag", caseID, "add",
			map[string]any{"caseId": mirrorCaseID, "label": mirrorLabel},
			func(writeCtx context.Context) error {
				_, err := s.snMirror.AddCaseTagAs(writeCtx, mirrorCaseID, mirrorLabel, mirrorActorEmail)
				return err
			},
		)
	}

	return tag, nil
}

// detectPatchTagBillableOverride DETECTS AND LOGS ONLY — it does not
// itself change any time card's billable status, publish an event, or
// persist the tag (see AddCaseTag's own doc comment). It is a special case
// of detectBillableStatusChange's normal "entering LOW/S4 severity makes
// time cards billable" rule: a case tagged "patch" while at LOW severity
// should eventually have its time cards non-billable regardless — WSO2
// still covers a patch under support even for an otherwise best-efforts S4
// case — but nothing in this codebase acts on that yet (see the TODO
// below). Label matching is case/whitespace-insensitive, same reasoning as
// this codebase's other free-text label lookups (e.g.
// slaSeverityLabelAndColor in csm-notification-service). Unlike
// detectBillableStatusChange, the eventual reaction is meant to be
// one-directional: removing the tag (or adding any other label) should
// never reverse it — only ever set isBillable=false, never back to true,
// since there's no natural "un-patch" event to react to.
//
// Same commented-out-publish posture as detectBillableStatusChange: logs
// only, since there is still no time_cards consumer to act on
// events.TypeCaseBillableStatusChanged (see that type's own doc comment).
// AddCaseTag itself now succeeds (see its own doc comment) -- the remaining
// gap is purely the missing consumer, not the tag storage this was
// originally blocked on.
func (s *caseService) detectPatchTagBillableOverride(ctx context.Context, caseID, label string) {
	if !strings.EqualFold(strings.TrimSpace(label), "patch") {
		return
	}

	// Unrestricted: this is an internal re-fetch of a case AddCaseTag just
	// wrote to, not a caller-facing read -- there's no separate caller
	// identity to scope here, and the tag write itself already happened.
	cv, err := s.repo.GetCaseByID(ctx, caseID, repository.SearchScope{Unrestricted: true})
	if err != nil {
		slog.ErrorContext(ctx, "add case tag: patch billable override not evaluated, get case failed", "caseId", caseID)
		return
	}
	if cv.Severity == nil || *cv.Severity != domain.CaseSeverityLow {
		return
	}

	// TODO: enable once (a) case tags have real Postgres storage so
	// AddCaseTag can actually succeed, and (b) a consumer exists for
	// events.TypeCaseBillableStatusChanged (bulk-flipping every time
	// card's IsBillable for caseId) — see that type's own doc comment for
	// what's still missing there.
	//
	// payload, err := json.Marshal(events.CaseBillableStatusChangedPayload{CaseID: caseID, IsBillable: false})
	// if err != nil {
	// 	slog.ErrorContext(ctx, "add case tag: encode case.billable_status_changed payload failed", "caseId", caseID, "error", err)
	// 	return
	// }
	// if s.publisher == nil {
	// 	return
	// }
	// if err := s.publisher.Publish(ctx, events.TypeCaseBillableStatusChanged, caseID, payload); err != nil {
	// 	slog.ErrorContext(ctx, "add case tag: publish case.billable_status_changed failed", "caseId", caseID)
	// }

	slog.InfoContext(ctx, "add case tag: patch tag detected on an S4 case, time cards would need to become non-billable once a real tag/time-card path exists (detection only, no action taken)", "caseId", caseID, "isBillable", false)
}

// RemoveCaseTag implements CaseService.
//
// Deliberately NOT mirrored to ServiceNow under
// DATA_SOURCE=postgres-servicenow-dual-write (unlike AddCaseTag -- see that
// method's own doc comment): tagID here is the Postgres "tag" table's own
// primary key, and there is nowhere this schema records the corresponding
// ServiceNow label-entry sys_id AddCaseTag's mirror created (SN's AddCaseTag
// response is discarded after firing -- see that mirror's own comment).
// Resolving one from the other would need either a new mapping column/table
// (a schema change, out of scope here) or a fragile runtime lookup (list
// ServiceNow's tags for the case and match by label, which breaks on
// multiple same-label tags and silently no-ops when the original AddCaseTag
// mirror itself never landed). Left unmirrored rather than guessed at.
func (s *caseService) RemoveCaseTag(ctx context.Context, caseID, tagID string) error {
	if err := validateUUIDs("caseId", []string{caseID}); err != nil {
		return err
	}
	if err := validateUUIDs("tagId", []string{tagID}); err != nil {
		return err
	}
	if err := s.authorizeCaseAccess(ctx, caseID); err != nil {
		return err
	}
	actor, err := s.resolveActor(ctx)
	if err != nil {
		return err
	}
	return s.repo.RemoveCaseTag(ctx, caseID, tagID, actor.Email)
}

// SearchTags implements CaseService.
func (s *caseService) SearchTags(ctx context.Context, req domain.SearchTagsRequest) ([]domain.Tag, error) {
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > maxLimit {
		return nil, &apierror.ValidationError{Msg: fmt.Sprintf("limit cannot exceed %d", maxLimit)}
	}

	actor, err := s.resolveActor(ctx)
	if err != nil {
		return nil, err
	}

	return s.repo.SearchTags(ctx, req.Filters.SearchQuery, actor.Email, limit)
}

func (s *caseService) GetCaseFeedback(_ context.Context, _ string) (domain.CaseEmojiFeedback, error) {
	return domain.CaseEmojiFeedback{}, &apierror.ServiceUnavailableError{Msg: "case feedback is only supported for the ServiceNow data source"}
}

func (s *caseService) SubmitCaseFeedback(_ context.Context, _ string, _ domain.SubmitCaseFeedbackRequest) (domain.SubmitCaseFeedbackResponse, error) {
	return domain.SubmitCaseFeedbackResponse{}, &apierror.ServiceUnavailableError{Msg: "case feedback is only supported for the ServiceNow data source"}
}

// GetAttachmentByID implements CaseService for the CSM-native (Postgres) data
// source. Content is always nil: this service holds no bytes for a
// Postgres-sourced attachment, only its storage_key -- see
// GetCaseAttachmentContent's doc comment for why content must be resolved
// externally via StorageKey instead.
func (s *caseService) GetAttachmentByID(ctx context.Context, id string) (domain.AttachmentDetails, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.AttachmentDetails{}, err
	}

	a, err := s.authorizeAttachmentAccess(ctx, id)
	if err != nil {
		return domain.AttachmentDetails{}, err
	}

	var createdBy string
	if a.CreatedBy != nil {
		createdBy = a.CreatedBy.Email
	}

	return domain.AttachmentDetails{
		ID:            a.ID,
		ReferenceID:   a.ReferenceID,
		ReferenceType: &a.ReferenceType,
		Name:          a.Name,
		Type:          a.Type,
		SizeBytes:     a.SizeBytes,
		Description:   a.Description,
		CreatedBy:     createdBy,
		CreatedOn:     a.CreatedOn,
		DownloadURL:   a.DownloadURL,
		PreviewURL:    a.PreviewURL,
		Content:       nil,
		StorageKey:    a.StorageKey,
		Status:        a.Status,
	}, nil
}

// validatePGAttachmentUpdate mirrors the ServiceNow path's
// validateAttachmentUpdate, narrowed to the one reference type this data
// source's attachments table actually models: deployment attachments have no
// Postgres schema backing here, so that branch is rejected outright rather
// than silently accepted.
func validatePGAttachmentUpdate(req domain.UpdateAttachmentRequest) error {
	if req.ReferenceType != domain.ReferenceTypeCase {
		return &apierror.ValidationError{Msg: fmt.Sprintf("invalid reference type %q: only 'case' is supported for this data source", req.ReferenceType)}
	}
	if req.Description != nil {
		return &apierror.ValidationError{Msg: "description field is not allowed for case reference type"}
	}
	if req.Name == nil || strings.TrimSpace(*req.Name) == "" {
		return &apierror.ValidationError{Msg: "name field is required for case reference type"}
	}
	return nil
}

// UpdateAttachment implements CaseService for the CSM-native (Postgres) data
// source. Only renaming is supported, mirroring the one mutation the
// ServiceNow path allows for reference type "case" (name required,
// description forbidden).
func (s *caseService) UpdateAttachment(ctx context.Context, req domain.UpdateAttachmentRequest) (domain.UpdateAttachmentResponse, error) {
	if err := validateUUIDs("id", []string{req.AttachmentID}); err != nil {
		return domain.UpdateAttachmentResponse{}, err
	}
	if err := validateUUIDs("referenceId", []string{req.ReferenceID}); err != nil {
		return domain.UpdateAttachmentResponse{}, err
	}
	if err := validatePGAttachmentUpdate(req); err != nil {
		return domain.UpdateAttachmentResponse{}, err
	}
	if _, err := s.authorizeAttachmentAccess(ctx, req.AttachmentID); err != nil {
		return domain.UpdateAttachmentResponse{}, err
	}

	user, err := s.resolveActor(ctx)
	if err != nil {
		return domain.UpdateAttachmentResponse{}, err
	}

	updatedOn, err := s.repo.UpdateCaseAttachmentName(ctx, req.AttachmentID, strings.TrimSpace(*req.Name), user.ID)
	if err != nil {
		return domain.UpdateAttachmentResponse{}, err
	}

	return domain.UpdateAttachmentResponse{
		Message: "Attachment updated successfully",
		Attachment: domain.UpdatedAttachment{
			ID:        req.AttachmentID,
			UpdatedOn: updatedOn,
			UpdatedBy: user.Email,
		},
	}, nil
}
