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

package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// parentRefTypeCase is the CaseNumberRef.Type value for a parent that is itself a case.
// Addressable because CaseNumberRef.Type is an optional (pointer) field.
var parentRefTypeCase = "case"

// caseSeverityToEnum/caseSeverityFromEnum map domain.CaseSeverity's values
// (catastrophic/critical/high/medium/low) to "case".severity's real
// case_severity_enum labels (migration 0023: 'S0'..'S4' -- an entirely
// different label set, not a case-only difference from the domain value the
// way state/issue_type/work_state are). No migration comment or other
// mapping table in this schema states the intended correspondence; this
// follows the standard S0=most-severe/S4=least-severe ITSM convention,
// matching the domain enum's own catastrophic-to-low ordering. Flagged here
// in case that assumption turns out to be wrong -- without it, severity
// can't be written or filtered on Postgres at all (every value the API
// accepts fails "invalid input value for enum case_severity_enum").
var caseSeverityToEnum = map[domain.CaseSeverity]string{
	domain.CaseSeverityCatastrophic: "S0",
	domain.CaseSeverityCritical:     "S1",
	domain.CaseSeverityHigh:         "S2",
	domain.CaseSeverityMedium:       "S3",
	domain.CaseSeverityLow:          "S4",
}

var caseSeverityFromEnum = map[string]domain.CaseSeverity{
	"S0": domain.CaseSeverityCatastrophic,
	"S1": domain.CaseSeverityCritical,
	"S2": domain.CaseSeverityHigh,
	"S3": domain.CaseSeverityMedium,
	"S4": domain.CaseSeverityLow,
}

// announcementTypeEnumValue maps CreateCaseRequest.IsSecurityAnnouncement to
// announcement.announcement_type's announcement_type_enum literal
// (migration 0088). Only meaningful for req.Type == "announcement" -- the
// caller passes req.IsSecurityAnnouncement directly, never derives it from
// anything else.
func announcementTypeEnumValue(isSecurityAnnouncement bool) string {
	if isSecurityAnnouncement {
		return "SECURITY"
	}
	return "GENERAL"
}

// announcementTypeEnumValuePtr is announcementTypeEnumValue's nil-safe
// counterpart for a partial update (COALESCE against the existing column
// value) -- a nil req field must leave announcement_type untouched, not
// silently reset it to GENERAL.
func announcementTypeEnumValuePtr(isSecurityAnnouncement *bool) *string {
	if isSecurityAnnouncement == nil {
		return nil
	}
	v := announcementTypeEnumValue(*isSecurityAnnouncement)
	return &v
}

// caseResolutionCodeToEnum maps domain.CaseResolutionCode (verified against
// ServiceNow's live resolution-code picklist -- see that type's own doc
// comment) to "case".resolution_code's real case_resolution_code_enum
// labels (migration 0023). All but three match by identity once compared
// side by side; those three don't, and are spelled out explicitly rather
// than guessed:
//   - ConsideredForRoadmapAlt/SolvedWorkaroundProvidedAlt are ServiceNow's
//     own duplicate picklist entries for the same underlying meaning (two
//     sys_ids, one concept) -- the Postgres enum only has one canonical
//     label for each, so both map to it.
//   - AbruptlyClosedDueToNonResponsiveness (domain) is
//     ABRUPTLY_CLOSED_DUE_TO_NON_RESPONSIVENESS_THROUGH_AUTO_CLOSURE in the
//     Postgres enum -- a longer label for the identical concept.
//
// Without this, GetCaseByID's read side (which used to cast the stored enum
// label straight into domain.CaseResolutionCode with no mapping at all)
// rendered the long-form label back to callers verbatim -- not a value any
// domain.CaseResolutionCode constant declares -- for every case resolved as
// "abruptly closed due to non-responsiveness". See caseResolutionCodeFromEnum
// for the read-side fix this same map now also makes possible.
var caseResolutionCodeToEnum = map[domain.CaseResolutionCode]string{
	domain.CaseResolutionCodeSolvedFixedBySupportGuidanceProvided: "SOLVED_FIXED_BY_SUPPORT_GUIDANCE_PROVIDED",
	domain.CaseResolutionCodeSolvedFixedByClosingRelatedIncident:  "SOLVED_FIXED_BY_CLOSING_RELATED_INCIDENT",
	domain.CaseResolutionCodeSolvedFixedByClosingRelatedRDTicket:  "SOLVED_FIXED_BY_CLOSING_RELATED_RD_TICKET",
	domain.CaseResolutionCodeSolvedWorkaroundProvided:             "SOLVED_WORKAROUND_PROVIDED",
	domain.CaseResolutionCodeSolvedByCustomer:                     "SOLVED_BY_CUSTOMER",
	domain.CaseResolutionCodeConsideredForRoadmap:                 "CONSIDERED_FOR_ROADMAP",
	domain.CaseResolutionCodeInconclusiveOutOfScope:               "INCONCLUSIVE_OUT_OF_SCOPE",
	domain.CaseResolutionCodeInconclusiveCannotReproduce:          "INCONCLUSIVE_CANNOT_REPRODUCE",
	domain.CaseResolutionCodeInconclusiveNoWorkaround:             "INCONCLUSIVE_NO_WORKAROUND",
	domain.CaseResolutionCodeDuplicateIssue:                       "DUPLICATE_ISSUE",
	domain.CaseResolutionCodeVoidedCanceled:                       "VOIDED_CANCELED",
	domain.CaseResolutionCodeOnHold:                               "ON_HOLD",
	domain.CaseResolutionCodeConsideredForRoadmapAlt:              "CONSIDERED_FOR_ROADMAP",
	domain.CaseResolutionCodeSolvedFixedTheIssue:                  "SOLVED_FIXED_THE_ISSUE",
	domain.CaseResolutionCodeSolvedWorkaroundProvidedAlt:          "SOLVED_WORKAROUND_PROVIDED",
	domain.CaseResolutionCodeSolvedByContributor:                  "SOLVED_BY_CONTRIBUTOR",
	domain.CaseResolutionCodeSolvedByNovera:                       "SOLVED_BY_NOVERA",
	domain.CaseResolutionCodeAbruptlyClosedDueToNonResponsiveness: "ABRUPTLY_CLOSED_DUE_TO_NON_RESPONSIVENESS_THROUGH_AUTO_CLOSURE",
}

// caseResolutionCodeFromEnum is caseResolutionCodeToEnum's inverse for reads
// (GetCaseByID). The two Alt domain values and the long-form
// THROUGH_AUTO_CLOSURE label have no distinct inverse -- a stored row only
// ever needs to render whichever single canonical domain value its enum
// label maps back to, never the ServiceNow-side duplicate a caller might
// have written it through.
var caseResolutionCodeFromEnum = map[string]domain.CaseResolutionCode{
	"SOLVED_FIXED_BY_SUPPORT_GUIDANCE_PROVIDED": domain.CaseResolutionCodeSolvedFixedBySupportGuidanceProvided,
	"SOLVED_FIXED_BY_CLOSING_RELATED_INCIDENT":  domain.CaseResolutionCodeSolvedFixedByClosingRelatedIncident,
	"SOLVED_FIXED_BY_CLOSING_RELATED_RD_TICKET": domain.CaseResolutionCodeSolvedFixedByClosingRelatedRDTicket,
	"SOLVED_WORKAROUND_PROVIDED":                domain.CaseResolutionCodeSolvedWorkaroundProvided,
	"SOLVED_BY_CUSTOMER":                        domain.CaseResolutionCodeSolvedByCustomer,
	"CONSIDERED_FOR_ROADMAP":                    domain.CaseResolutionCodeConsideredForRoadmap,
	"INCONCLUSIVE_OUT_OF_SCOPE":                 domain.CaseResolutionCodeInconclusiveOutOfScope,
	"INCONCLUSIVE_CANNOT_REPRODUCE":             domain.CaseResolutionCodeInconclusiveCannotReproduce,
	"INCONCLUSIVE_NO_WORKAROUND":                domain.CaseResolutionCodeInconclusiveNoWorkaround,
	"DUPLICATE_ISSUE":                           domain.CaseResolutionCodeDuplicateIssue,
	"VOIDED_CANCELED":                           domain.CaseResolutionCodeVoidedCanceled,
	"ON_HOLD":                                   domain.CaseResolutionCodeOnHold,
	"SOLVED_FIXED_THE_ISSUE":                    domain.CaseResolutionCodeSolvedFixedTheIssue,
	"SOLVED_BY_CONTRIBUTOR":                     domain.CaseResolutionCodeSolvedByContributor,
	"SOLVED_BY_NOVERA":                          domain.CaseResolutionCodeSolvedByNovera,
	"ABRUPTLY_CLOSED_DUE_TO_NON_RESPONSIVENESS_THROUGH_AUTO_CLOSURE": domain.CaseResolutionCodeAbruptlyClosedDueToNonResponsiveness,
}

// CaseResolutionCodeFromEnum exports caseResolutionCodeFromEnum's lookup for
// the service package (project_metadata_service.go's resolution-code choice
// list) -- same "" -for-unrecognized contract as CallRequestStateFromEnum.
func CaseResolutionCodeFromEnum(enumLabel string) domain.CaseResolutionCode {
	return caseResolutionCodeFromEnum[enumLabel]
}

// caseLikeWorkItemTypes is validCaseType's (case_service.go) five values,
// spelled as the real work_item_type_enum labels: the work_item types
// GetCaseByID/SearchCases treat as "a case" -- each is a shared-PK
// work_item extension with its own state/cause/close_notes/closed_on/
// resolved_on columns (migrations 0023/0024), unlike CHANGE_REQUEST,
// INCIDENT, PROBLEM, and the rest of work_item_type_enum, which are surfaced
// through entirely different endpoints.
const caseLikeWorkItemTypes = `'{CASE,ENGAGEMENT,SERVICE_REQUEST,SECURITY_REPORT_ANALYSIS,ANNOUNCEMENT}'::work_item_type_enum[]`

// announcementVisibilityLeakGuard excludes an ANNOUNCEMENT-typed work_item
// whose announcement extension row RLS hid from the caller (migration
// 000085/0149's role/security-contact-based policy) -- without it, a
// caller who can't see the announcement row would still see wi.subject/
// wi.description (both live on the unprotected work_item table itself)
// with only the announcement-specific fields absent: a partially-redacted
// row leaking exactly the two fields the visibility policy exists to hide,
// instead of the row disappearing entirely as it should.
//
// A self-contained NOT EXISTS, not a check against an already-joined "ann"
// row's own id being NULL: the shape every one of this fragment's three
// call sites happened to use before this constant existed. That form only
// works when the query already carries a LEFT JOIN announcement ann ON
// ann.id = wi.id with exactly that alias -- true for case_repo.go's own
// two sites, never guaranteed for a future caller, and outright false for
// global_search_repo.go's countQuery (no announcement join at all). This
// form needs nothing but wi (aliased or not) to already be in scope, so
// pasting it into a brand-new query is safe without also checking whether
// an announcement join happens to already exist under the right alias --
// exactly the kind of drift this shared constant exists to prevent, at the
// cost of one extra (PK-indexed, cheap) EXISTS subquery in the two call
// sites that used to reuse an existing join instead.
const announcementVisibilityLeakGuard = `NOT (wi.type = 'ANNOUNCEMENT' AND NOT EXISTS (SELECT 1 FROM announcement rls_ann WHERE rls_ann.id = wi.id))`

// announcementLeakGuardFor returns the announcementVisibilityLeakGuard
// predicate for a caller that RLS actually restricts, and the always-true
// predicate TRUE for an Unrestricted (internal) one.
//
// The guard exists only to hide announcement rows a scoped caller may not
// see; an internal caller sees every announcement, so for them it filters
// nothing. It is not free, though: the EXISTS reads the RLS-protected
// announcement table, so the planner prices the announcement policy's
// correlated sub-selects into a per-row subplan on every work_item row of the
// list, count and aggregate queries. On a ~400K-row work_item that mispricing
// turned staff case searches from ~150ms into 1.2-1.5s. Dropping the guard
// for internal callers restores the plan they would get with RLS off.
//
// Fails closed: anything that is not explicitly Unrestricted (including the
// zero SearchScope) keeps the guard.
func announcementLeakGuardFor(scope SearchScope) string {
	if scope.Unrestricted {
		return "TRUE"
	}
	return announcementVisibilityLeakGuard
}

// caseLikeStateColumns COALESCEs state across every case-like work_item
// extension table (aliased c/eng/sr/sra/ann) -- exactly one is non-null for
// a given row, since each is a shared-PK extension keyed to a specific
// wi.type. All five share the same case_state_enum label set except
// announcement_state_enum, whose CLOSE (not CLOSED) is normalized here so
// the response's lowercased state doesn't diverge from every other type's
// "closed" for what is otherwise the same concept.
const caseLikeStateColumn = `COALESCE(c.state::TEXT, eng.state::TEXT, sr.state::TEXT, sra.state::TEXT,
	CASE WHEN ann.state::TEXT = 'CLOSE' THEN 'CLOSED' ELSE ann.state::TEXT END)`

// caseLikeCauseColumn/caseLikeCloseNotesColumn/caseLikeResolvedOnColumn
// mirror caseLikeStateColumn for the other three columns every case-like
// extension table shares. *_cause_enum's label sets are identical across all
// five tables (unlike state), so cause needs no per-branch normalization.
const caseLikeCauseColumn = `COALESCE(c.cause::TEXT, eng.cause::TEXT, sr.cause::TEXT, sra.cause::TEXT, ann.cause::TEXT)`
const caseLikeCloseNotesColumn = `COALESCE(c.close_notes, eng.close_notes, sr.close_notes, sra.close_notes, ann.close_notes)`
const caseLikeResolvedOnColumn = `COALESCE(c.resolved_on, eng.resolved_on, sr.resolved_on, sra.resolved_on, ann.resolved_on)`
const caseLikeClosedOnColumn = `COALESCE(c.closed_on, eng.closed_on, sr.closed_on, sra.closed_on, ann.closed_on)`

// caseLikeWorkStateColumn/caseLikeResolutionCodeColumn cover the four
// case-like tables that carry these columns (migration 0184 added them to
// engagement/service_request/security_report_analysis, using the same enum
// types as "case"). announcement has neither.
const caseLikeWorkStateColumn = `COALESCE(c.work_state::TEXT, eng.work_state::TEXT, sr.work_state::TEXT, sra.work_state::TEXT)`
const caseLikeResolutionCodeColumn = `COALESCE(c.resolution_code::TEXT, eng.resolution_code::TEXT, sr.resolution_code::TEXT, sra.resolution_code::TEXT)`

// workStateWorkItemTypes are the case-like work_item types that carry a work
// state and resolution code: all of caseLikeWorkItemTypes except ANNOUNCEMENT.
const workStateWorkItemTypes = `'{CASE,ENGAGEMENT,SERVICE_REQUEST,SECURITY_REPORT_ANALYSIS}'::work_item_type_enum[]`

// caseLikeJoins LEFT-joins every case-like work_item extension table other
// than "case" itself (each caller already joins "case" under its own alias,
// since some callers need it INNER/LEFT differently and some don't select
// from it at all). Every join is on the shared-PK pattern (migrations
// 0023/0024): <table>.id = wi.id.
const caseLikeJoins = `
	LEFT JOIN engagement eng ON eng.id = wi.id
	LEFT JOIN service_request sr ON sr.id = wi.id
	LEFT JOIN security_report_analysis sra ON sra.id = wi.id
	LEFT JOIN announcement ann ON ann.id = wi.id`

// caseEscalationLevelFromEnum strips case_escalation_level_enum's 'EL'
// prefix ('EL0'..'EL5') to the "0".."5" id CaseView.EscalationLevel's own
// doc comment specifies.
func caseEscalationLevelFromEnum(raw string) string {
	return strings.TrimPrefix(raw, "EL")
}

// CaseRepository defines the persistence operations for the case entity,
// split across work_item (migration 0021, fields common to every
// work_item type) and "case" (migration 0023, a shared-PK extension
// carrying case-specific fields -- "case".id IS work_item.id).
type CaseRepository interface {
	// CreateCase inserts a new case row (both work_item and "case").
	CreateCase(ctx context.Context, req domain.CreateCaseRequest) (domain.Case, error)
	// CreateCaseFromServiceNow inserts a new case-like row (work_item plus
	// one of "case"/announcement/service_request/engagement/
	// security_report_analysis, branching on req.Type -- see
	// createAnnouncementFromServiceNowQuery/createServiceRequestFromServiceNowQuery/
	// createEngagementFromServiceNowQuery/createSecurityReportAnalysisFromServiceNowQuery's
	// own doc comments for why the four non-case types each need a genuinely
	// different insert, not just a different type literal), for
	// DATA_SOURCE=postgres-servicenow-dual-write's SN-first creation (see
	// caseService.CreateCase's own doc comment): req.Type must already be one
	// of those five (validated by the caller). Unlike CreateCase, identity is
	// NOT generated here -- id/number/wso2ID/createdBy are exactly what
	// ServiceNow already returned for the record it just created, so both
	// systems agree on identity from the moment the Postgres row exists. id
	// must be a canonical UUID (sysidToUUID(sn sys_id) -- the same identity
	// convention every DataSource=servicenow response already uses, see
	// internal/service/sn_id.go). state is the target extension table's own
	// state enum literal value, already resolved by the caller from
	// ServiceNow's state label (ignored for req.Type == "case", which hardcodes
	// 'OPEN' itself, same as before this change). Returns a ValidationError if
	// id is not a valid UUID or if a row already exists for it/number/wso2ID
	// (unique violation) -- the latter should not happen in practice since
	// ServiceNow only just generated these, but is reported precisely rather
	// than as an opaque infrastructure error if it ever does.
	CreateCaseFromServiceNow(ctx context.Context, req domain.CreateCaseRequest, id, number, wso2ID, createdBy, state string) (domain.Case, error)
	// GetCaseByID returns the enriched case view for the given UUID, or a
	// NotFoundError if no matching row exists OR it exists but scope excludes
	// it (existence is never revealed to a caller who can't see it).
	GetCaseByID(ctx context.Context, id string, scope SearchScope) (domain.CaseView, error)
	// SearchCases returns a filtered, paginated slice of enriched case views
	// together with the total count of matching rows before pagination,
	// narrowed to scope regardless of what project filter req itself carries.
	// COUNT and SELECT are executed concurrently on separate pool connections.
	SearchCases(ctx context.Context, req domain.SearchCasesRequest, scope SearchScope) ([]domain.SearchCaseView, int, error)
	// AggregateCases counts the cases matching req's filters per value of
	// groupBy (state, severity, type, engagementType, issueType, workState,
	// account), largest bucket first. Rows with no value for the field are
	// not counted. The caller applies any top-N cap.
	AggregateCases(ctx context.Context, req domain.SearchCasesRequest, groupBy string, scope SearchScope) ([]domain.AggregateBucket, error)
	// CreateCaseComment inserts a new comment row for the given case.
	// createdOn is nil for an ordinary comment (created_on = NOW()); pass a
	// non-nil value to preserve a known past timestamp instead -- see the
	// implementation's own doc comment for why (ServiceNow comment mirroring).
	CreateCaseComment(ctx context.Context, req domain.CreateCaseCommentRequest, createdOn *time.Time) (domain.CaseComment, error)
	// SearchCaseComments returns a paginated slice of comments for the given case
	// together with the total count of matching rows before pagination.
	SearchCaseComments(ctx context.Context, req domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error)
	// UpdateCase updates the state and/or priority of the case identified by
	// req.ID. "case".closed_on is set to NOW() when transitioning to closed.
	//
	// previousSeverity is the case's severity as it stood immediately before
	// this update. When req.Severity is nil, severity can't have changed at
	// all, so this just equals the returned domain.Case.Severity — no extra
	// work needed. When req.Severity is set, this runs inside a transaction
	// that locks the row (SELECT ... FOR UPDATE) before reading its prior
	// severity and applying the update, so previousSeverity is accurate even
	// under a concurrent update to the same case: a plain separate
	// read-then-write (what this used to do) could either miss a genuine
	// LOW-severity-boundary crossing or double-detect one, depending on how
	// two concurrent updates interleave — see caseService.
	// detectBillableStatusChange, the sole caller that needs this value, and
	// the CodeRabbit finding on PR #1683 this fixes.
	//
	// Returns a NotFoundError if no matching row exists.
	UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (c domain.Case, previousSeverity *domain.CaseSeverity, err error)
	// CreateCaseAttachment inserts a new attachment metadata row for the case
	// identified by req.ReferenceID. req.StorageKey must be non-nil: this data
	// source stores file bytes externally in SFTPGo, never inline in Postgres.
	// Returns a ValidationError if req.ReferenceID does not match an existing case.
	CreateCaseAttachment(ctx context.Context, req domain.CreateAttachmentRequest) (domain.Attachment, error)
	// SearchCaseAttachments returns a paginated slice of attachments for the given
	// case, most recently created first, together with the total matching count.
	SearchCaseAttachments(ctx context.Context, caseID string, pagination domain.Pagination) ([]domain.Attachment, int, error)
	// GetCaseAttachmentByID returns the attachment identified by id.
	// Returns a NotFoundError if no matching row exists.
	GetCaseAttachmentByID(ctx context.Context, id string) (domain.Attachment, error)
	// DeleteCaseAttachment permanently removes the attachment metadata row
	// identified by id. It does not delete the backing SFTPGo file -- that
	// remains the downstream CSM backend's responsibility.
	// Returns a NotFoundError if no matching row exists.
	DeleteCaseAttachment(ctx context.Context, id string) error
	// UpdateCaseAttachmentName renames the attachment identified by id and
	// records who did it. Returns a NotFoundError if no matching row exists.
	UpdateCaseAttachmentName(ctx context.Context, id, name, updatedBy string) (updatedOn time.Time, err error)
	// ConfirmCaseAttachment atomically transitions the attachment identified
	// by id from status 'pending' to 'complete'. The WHERE clause's
	// "AND status = 'pending'" guard is the concurrency safety net: if two
	// confirm calls race, only one affects a row. Returns a ConflictError
	// (not a silent no-op) if the row is not currently 'pending' -- including
	// when it doesn't exist, since by the time this is called the caller has
	// already resolved the row via GetCaseAttachmentByID and any mismatch
	// here means it changed state concurrently.
	ConfirmCaseAttachment(ctx context.Context, id string) (domain.Attachment, error)
	// AddCaseTag finds or creates a tag named label (case-insensitively) and
	// attaches it to the case's underlying work_item, unless it is already
	// attached (idempotent: a second call for an already-attached label
	// returns the existing tag, not an error). callerEmail is recorded as
	// created_by/updated_by; the "no fine-grained ACL beyond authenticated
	// caller" model matches caseRepo's existing convention -- callerEmail
	// is threaded down to this layer against a future authorization
	// decision, not checked here yet. Returns a ValidationError if caseID
	// does not reference an existing case.
	AddCaseTag(ctx context.Context, caseID, label, callerEmail string) (domain.Tag, error)
	// RemoveCaseTag detaches the tag identified by tagID from the case
	// identified by caseID. Returns a NotFoundError if that pairing does
	// not exist (the tag might exist but not be on this case, or not exist
	// at all -- both are "not found" from the caller's perspective).
	RemoveCaseTag(ctx context.Context, caseID, tagID, callerEmail string) error
	// SetCaseTagSNSysID best-effort persists ServiceNow's own label_entry
	// sys_id for the (caseID, tagID) attachment on work_item_tag (migration
	// 000088) -- called from AddCaseTag's async ServiceNow mirror success
	// path, never from the synchronous request path. Returns
	// *apierror.NotFoundError if the pairing does not exist (e.g. it was
	// removed concurrently before the mirror finished), so the caller can
	// react by cleaning up the ServiceNow tag this call failed to map.
	SetCaseTagSNSysID(ctx context.Context, caseID, tagID, snSysID string) error
	// GetCaseTagSNSysID returns the ServiceNow label_entry sys_id previously
	// stored for the (caseID, tagID) attachment by SetCaseTagSNSysID, or nil
	// if none is stored yet. Returns a NotFoundError if the pairing does not
	// exist -- callers that need this before a REMOVE (which deletes the
	// work_item_tag row entirely, taking sn_sys_id with it) must call this
	// first, synchronously, while the row still exists.
	GetCaseTagSNSysID(ctx context.Context, caseID, tagID string) (*string, error)
	// SearchTags returns tags (not scoped to any case) whose name matches
	// searchQuery case-insensitively (all tags when searchQuery is empty),
	// most recently created first, capped at limit. callerEmail is threaded
	// down for the same future-authorization reason as AddCaseTag; tags are
	// global vocabulary with no per-case or per-caller scope today.
	SearchTags(ctx context.Context, searchQuery, callerEmail string, limit int) ([]domain.Tag, error)
	// SetCaseWatchList replaces the case's watch list (work_item_watcher
	// rows keyed by the case's own id, which is also its work_item id)
	// wholesale with userIDs, and bumps the case's underlying work_item
	// row's updated_on/updated_by the same way every other UpdateCase
	// branch does -- callerEmail is that updated_by. Returns the resolved
	// watcher list and the new updated_on. Returns a NotFoundError if
	// caseID does not exist; a ValidationError if any userID does not
	// exist.
	SetCaseWatchList(ctx context.Context, caseID string, userIDs []string, callerEmail string) ([]domain.WatchListUser, time.Time, error)
	// AccountDefaultWatcherEmails returns the account owning projectID's
	// four named stakeholders' email addresses -- technical_owner_id,
	// secondary_technical_owner_id, account_manager_id,
	// renewal_account_manager_id (migration 0012), whichever are set and
	// have an email on file, deduplicated. customer_success_manager_id is
	// deliberately excluded: unlike the other four, the CSM is not meant to
	// receive these default case notifications. A project with no linked
	// account, or a project id that does not exist, returns an empty slice
	// rather than an error: this is a default watch list, not a
	// requirement. Used to compute a case.* event's email
	// Recipients fresh at publish time: these four are deliberately never
	// persisted into work_item_watcher (see SetCaseWatchList's own callers'
	// doc comments) specifically so a later stakeholder reassignment is
	// reflected on the very next notification, not stuck on whoever held the
	// role when the case was created or last had its watch list edited. A
	// stakeholder with no email on file is silently excluded, same as
	// watchListUserEmails does for an explicit watcher.
	AccountDefaultWatcherEmails(ctx context.Context, projectID string) ([]string, error)
	// GetCaseEtaSharedOn returns work_item.eta_shared_on for caseID -- nil
	// (not an error) when the case has no fix ETA shared yet, or the case
	// id doesn't exist. See domain.CaseView.EtaSharedOn's own doc comment
	// for why this is a dedicated single-column lookup: the plain-ServiceNow
	// data source's own GetCaseByID has no Postgres row to read this from
	// via its usual join (it never runs one), so snCaseService calls this
	// directly through pgFallback instead, rather than paying for a full
	// CaseRepository.GetCaseByID just for one column.
	GetCaseEtaSharedOn(ctx context.Context, caseID string) (*time.Time, error)
	// ProjectContactEmailsByRole returns the distinct project_contact.email
	// addresses for projectID whose contact currently holds role (a
	// project_role_enum label, e.g. "SECURITY_CONTACT" or "PORTAL_USER") via
	// project_contact_group -> project_group -> project_group_role ->
	// project_role. Excludes DEACTIVATED contacts; every other state
	// (INVITED/REGISTERED/RE-INVITED/NULL) counts, since this is an email
	// audience, not a case-access grant. Used to resolve an announcement
	// case's recipients -- see publishCaseCreatedEvent's own doc comment. A
	// project with no contact holding role returns an empty slice, not an
	// error.
	ProjectContactEmailsByRole(ctx context.Context, projectID, role string) ([]string, error)
	// UpdateCaseAssignee sets work_item.assigned_to_id to userID -- already
	// resolved and validated as a real "user" row by the caller (CaseService.
	// updateCaseAssignee, via GetUserByEmail) -- and bumps updated_on/updated_by,
	// but only when assigned_to_id actually differs from userID: the write
	// itself is the no-op check (a single atomic UPDATE...WHERE...RETURNING,
	// not a separate pre-write read), so two concurrent requests assigning
	// the same case to the same engineer can't both observe "unchanged" and
	// both publish/mirror the same no-op write -- see CaseService.
	// updateCaseAssignee's own doc comment for why that race mattered
	// (CodeRabbit finding on PR #1989). userID is nil to clear the assignee
	// (assigned_to_id UUID REFERENCES "user"(id) ON DELETE SET NULL, migration
	// 000036, is already nullable) and non-nil to set it -- pgx binds a nil
	// *string parameter as SQL NULL automatically, and the query's own
	// IS DISTINCT FROM already treats NULL correctly on both sides, so the SQL
	// itself needs no change for this. changed reports whether this call
	// was the one that wrote it; updatedOn is the row's current value
	// either way. Returns a NotFoundError if caseID does not exist.
	UpdateCaseAssignee(ctx context.Context, caseID string, userID *string, callerEmail string) (updatedOn time.Time, changed bool, err error)
	// AcknowledgeCase atomically claims the case for actorID if nobody has
	// acknowledged it yet (work_item.acknowledged_by_user_id IS NULL), or
	// leaves it untouched if someone already has -- the same idempotent
	// "claim once" semantics domain.UpdateCaseRequest.Acknowledge's own doc
	// comment documents for the ServiceNow data source. Returns whether the
	// case was already acknowledged before this call, a reference to whoever
	// holds the acknowledgement now (this call's actor if newly claimed, else
	// whoever claimed it first), the case's number, and updated_on (only
	// bumped when this call did the claiming). Returns a NotFoundError if
	// caseID does not exist.
	AcknowledgeCase(ctx context.Context, caseID, actorID, actorEmail string) (alreadyAcknowledged bool, acknowledgedBy domain.AssignedEngineerRef, number string, updatedOn time.Time, err error)
	// UpdateCaseParent sets work_item.parent_id (migration 0039, a generic
	// work_item self-reference), already read the other direction by
	// GetCaseByID's own ParentCase. In its own dedicated method, not the
	// field bundle below, because sn_case_service.go's own UpdateCase keeps
	// parentId fully exclusive of every other field in the same request --
	// mirrored here so the two data sources accept the same combinations.
	// Returns the new updated_on. Returns a NotFoundError if caseID does not
	// exist, a ValidationError if parentID does not.
	UpdateCaseParent(ctx context.Context, caseID, parentID, callerEmail string) (time.Time, error)
	// UpdateCaseFields writes any subset of the "plain field" PATCH fields
	// CaseService.UpdateCase's combinable-field-bundle branch accepts --
	// exactly the columns req's own non-nil pointers name, split across
	// work_item (Subject/Description/DeploymentID/DeployedProductID/
	// BestCaseFixEta/MostLikelyFixEta/WorstCaseFixEta/WorkaroundProvided) and
	// "case" (RelatedCaseID only -- see this method's own comment for why
	// ResolutionCode/Cause/CloseNotes/IssueType are deliberately not among
	// them). actorID/actorEmail are only used when req.WorkaroundProvided is
	// non-nil, to stamp who (re)marked it. work_item.updated_on/updated_by
	// are always bumped, even when only "case" columns changed, matching
	// every sibling UpdateCase branch. Returns a NotFoundError if req.ID does
	// not exist.
	UpdateCaseFields(ctx context.Context, req domain.UpdateCaseRequest, actorID, actorEmail string) (time.Time, error)
	// RecordCaseFieldChangeActivity inserts a work_item_activity row
	// (migration 0055) recording that caseID's fieldName changed from
	// oldValue to newValue, attributed to actorEmail. SearchCaseActivities'
	// own field_change branch already renders any field_name generically
	// (see caseActivityFieldChangeLabel) -- this is the missing write half:
	// nothing wrote to this table on the Postgres data source before this,
	// even though the ServiceNow data source's own sync process populates
	// it there, so a Postgres-native mutation's own change never appeared
	// in the case's activity feed. Best-effort by every caller (a failure
	// here must not undo or fail the mutation that already succeeded), so
	// this itself just returns whatever error occurs, with no special
	// handling of its own.
	RecordCaseFieldChangeActivity(ctx context.Context, caseID, fieldName, oldValue, newValue, actorEmail string) error
	// MarkCaseFixIssued stamps work_item.fix_issued_on with the current time
	// (a pre-existing base-schema column, csm-sync-service's designated
	// target for ServiceNow's u_fix_issued -- see this method's own doc
	// comment for the source), first-write-wins: a case whose fix_issued_on
	// is already set is left untouched and alreadySet is returned true with
	// the existing timestamp, rather than being treated as an error. The
	// WHERE fix_issued_on IS NULL guard on the UPDATE makes this atomic on
	// its own -- no explicit row lock or transaction is needed, unlike
	// SetCaseWatchList. Returns a NotFoundError if caseID does not exist.
	MarkCaseFixIssued(ctx context.Context, caseID string) (fixIssued time.Time, alreadySet bool, err error)
	// SearchCaseActivities returns a paginated, newest-first feed combining
	// the case's comments (comment, migration 0040) and complete
	// attachments (case_attachment, migration 0106) into one merged
	// timeline, together with the total matching count. There is no
	// field-change audit table in this schema, so entries of that kind are
	// never produced regardless of req.IncludeFieldChanges -- an absent
	// field-change history is a valid state (see
	// SearchCaseActivitiesRequest's own doc comment: "only comment and
	// attachment entries are returned" is the documented default already).
	SearchCaseActivities(ctx context.Context, req domain.SearchCaseActivitiesRequest) ([]domain.CaseActivity, int, error)
}

type caseRepo struct {
	db *Scoped
}

// NewCaseRepository constructs a CaseRepository backed by the given connection pool.
func NewCaseRepository(db *Scoped) CaseRepository {
	return &caseRepo{db: db}
}

// CreateCase implements CaseRepository.
//
// A case-like record is a work_item row plus its type-specific extension row
// (migrations 0021/0023/0024) sharing its id, written in one transaction via
// one CTE query per type -- the same shared-primary-key pattern
// CreateCaseFromServiceNow's five query consts already use, just generating
// identity instead of taking it from ServiceNow's response. The old version
// of this method only ever wrote a "CASE" row and inserted into a "cases"
// table that does not exist for the rest.
//
// The row's identifiers follow the synced data: work_item.created_by holds the
// creator's EMAIL (6,995 of 8,066 staging cases), so it is taken from the user
// row of req.CreatedBy (a user id), which is also stored as opened_by_user_id.
// A missing user yields no row (the "creator" CTE is empty, so the whole
// chain returns zero rows), reported as a validation error rather than a bare
// foreign-key failure.
//
// work_item.number (every type) and wso2_id (the five case-like types all
// require one, per work_item_wso2_id_required_by_type) come from
// next_portal_work_item_number()/next_portal_wso2_id() (migration 0140),
// which resolves the product decision this method used to defer (see
// CLAUDE.md, "CreateCase and case numbers"): a portal-created record gets a
// visually distinct number/id rather than one drawn from the same series
// ServiceNow's still-running sync allocates from, so the two can never
// collide. service_request/engagement/security_report_analysis/announcement
// were previously rejected outright on this data source
// (caseService.CreateCase's own "supported only for dual-write" check,
// updated alongside this) purely because this method had nowhere to write
// them -- now that it does, that restriction only applies to the plain
// ServiceNow-less gap that no longer exists.
func (r *caseRepo) CreateCase(ctx context.Context, req domain.CreateCaseRequest) (domain.Case, error) {
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (domain.Case, error) {
		return createCaseTx(ctx, tx, req)
	})
}

// createCaseTx is CreateCase's body, extracted so it can run inside
// r.db.InTx's closure (Scoped.InTx pulls caller identity from ctx and sets
// it once for the whole transaction). Dispatches on req.Type the same way
// CreateCaseFromServiceNow does, reusing scanUpdatedCase to decode the
// result since every *PortalQuery below ends in the identical trailing
// SELECT shape that helper already expects.
func createCaseTx(ctx context.Context, tx pgx.Tx, req domain.CreateCaseRequest) (domain.Case, error) {
	var row pgx.Row
	switch req.Type {
	case "announcement":
		row = tx.QueryRow(ctx, createAnnouncementPortalQuery,
			req.CreatedBy, req.ProjectID,
			req.Subject, req.Description,
			announcementTypeEnumValue(req.IsSecurityAnnouncement),
		)
	case "service_request":
		row = tx.QueryRow(ctx, createServiceRequestPortalQuery,
			req.CreatedBy, req.ProjectID, req.DeploymentID, req.DeployedProductID,
			req.Subject, req.Description,
		)
	case "engagement":
		row = tx.QueryRow(ctx, createEngagementPortalQuery,
			req.CreatedBy, req.ProjectID, req.DeploymentID, req.DeployedProductID,
			req.Subject, req.Description,
			strings.ToUpper(string(req.EngagementType)), strings.ToUpper(string(req.EngagementPaymentType)),
		)
	case "security_report_analysis":
		row = tx.QueryRow(ctx, createSecurityReportAnalysisPortalQuery,
			req.CreatedBy, req.ProjectID, req.DeploymentID, req.DeployedProductID,
			req.Subject, req.Description,
		)
	default: // "case"
		row = tx.QueryRow(ctx, createCasePortalQuery,
			req.CreatedBy, req.ProjectID, req.DeploymentID, req.DeployedProductID,
			req.Subject, req.Description,
			caseSeverityToEnum[req.Severity], strings.ToUpper(string(req.IssueType)),
		)
	}
	c, err := scanUpdatedCase(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Case{}, &apierror.ValidationError{Msg: "creating user not found: " + req.CreatedBy}
	}
	if err != nil {
		return domain.Case{}, mapCreateCaseError(err)
	}
	return c, nil
}

// createCasePortalQuery is CreateCase's (the plain-Postgres, caller-initiated
// path) query for req.Type == "case" -- structurally identical to
// createCaseFromServiceNowQuery except identity (id/number/wso2_id) is
// generated here instead of supplied by the caller, and created_by/
// opened_by_user_id/account_id are resolved from req.CreatedBy (a user id)
// via the "creator" CTE rather than taken as already-resolved values, since
// there is no ServiceNow response to have resolved them from. severity uses
// the same NULLIF(...,'') tolerance the pre-dispatch version of this method
// already relied on, for an unset req.Severity.
const createCasePortalQuery = `
	WITH creator AS (
		SELECT id, email FROM "user" WHERE id = $1::uuid
	),
	inserted_work_item AS (
		INSERT INTO work_item (
			id, created_on, updated_on, created_by, updated_by,
			number, wso2_id, subject, description, type,
			project_id, deployment_id, deployed_product_id, opened_by_user_id, account_id
		)
		SELECT gen_random_uuid(), NOW(), NOW(), creator.email, creator.email,
		       next_portal_work_item_number(), next_portal_wso2_id($2::uuid), $5, $6, 'CASE'::work_item_type_enum,
		       $2::uuid, $3::uuid, $4::uuid, creator.id, p.account_id
		FROM creator
		LEFT JOIN project p ON p.id = $2::uuid
		RETURNING id, number, wso2_id, created_by, project_id, deployment_id, deployed_product_id,
		          subject, description, created_on, updated_on
	),
	inserted_case AS (
		INSERT INTO "case" (id, severity, issue_type, state)
		SELECT id, NULLIF($7, '')::case_severity_enum, NULLIF($8, '')::case_issue_type_enum, 'OPEN'::case_state_enum
		FROM inserted_work_item
		RETURNING id, severity, issue_type, state, work_state, closed_on
	)
	SELECT iwi.id, iwi.number, iwi.wso2_id, iwi.created_by, iwi.project_id, iwi.deployment_id, iwi.deployed_product_id,
	       iwi.subject, iwi.description, ic.severity::TEXT, ic.issue_type::TEXT, ic.state::TEXT, ic.work_state::TEXT,
	       iwi.created_on, iwi.updated_on, ic.closed_on
	FROM inserted_work_item iwi
	JOIN inserted_case ic ON ic.id = iwi.id`

// createAnnouncementPortalQuery is createCasePortalQuery's counterpart for
// req.Type == "announcement" -- same relationship createAnnouncementFromServiceNowQuery
// has to createCaseFromServiceNowQuery (no deployment/deployed-product concept,
// announcement_type instead of severity/issue_type).
const createAnnouncementPortalQuery = `
	WITH creator AS (
		SELECT id, email FROM "user" WHERE id = $1::uuid
	),
	inserted_work_item AS (
		INSERT INTO work_item (
			id, created_on, updated_on, created_by, updated_by,
			number, wso2_id, subject, description, type,
			project_id, deployment_id, deployed_product_id, opened_by_user_id, account_id
		)
		SELECT gen_random_uuid(), NOW(), NOW(), creator.email, creator.email,
		       next_portal_work_item_number(), next_portal_wso2_id($2::uuid), $3, $4, 'ANNOUNCEMENT'::work_item_type_enum,
		       $2::uuid, NULL, NULL, creator.id, p.account_id
		FROM creator
		LEFT JOIN project p ON p.id = $2::uuid
		RETURNING id, number, wso2_id, created_by, project_id, deployment_id, deployed_product_id,
		          subject, description, created_on, updated_on
	),
	inserted_announcement AS (
		INSERT INTO announcement (id, state, announcement_type)
		SELECT id, 'OPEN'::announcement_state_enum, $5::announcement_type_enum
		FROM inserted_work_item
		RETURNING id, state, closed_on
	)
	SELECT iwi.id, iwi.number, iwi.wso2_id, iwi.created_by,
	       iwi.project_id, COALESCE(iwi.deployment_id::TEXT, ''), COALESCE(iwi.deployed_product_id::TEXT, ''),
	       iwi.subject, iwi.description,
	       NULL::TEXT, NULL::TEXT, ia.state::TEXT, NULL::TEXT,
	       iwi.created_on, iwi.updated_on, ia.closed_on
	FROM inserted_work_item iwi
	JOIN inserted_announcement ia ON ia.id = iwi.id`

// createServiceRequestPortalQuery is createCasePortalQuery's counterpart for
// req.Type == "service_request" -- same relationship createServiceRequestFromServiceNowQuery
// has to createCaseFromServiceNowQuery. Starts at 'OPEN'::service_request_state_enum,
// the same initial label the case/engagement/security_report_analysis state
// enums all share (confirmed against the live enum catalog, not guessed).
const createServiceRequestPortalQuery = `
	WITH creator AS (
		SELECT id, email FROM "user" WHERE id = $1::uuid
	),
	inserted_work_item AS (
		INSERT INTO work_item (
			id, created_on, updated_on, created_by, updated_by,
			number, wso2_id, subject, description, type,
			project_id, deployment_id, deployed_product_id, opened_by_user_id, account_id
		)
		SELECT gen_random_uuid(), NOW(), NOW(), creator.email, creator.email,
		       next_portal_work_item_number(), next_portal_wso2_id($2::uuid), $5, $6, 'SERVICE_REQUEST'::work_item_type_enum,
		       $2::uuid, $3::uuid, $4::uuid, creator.id, p.account_id
		FROM creator
		LEFT JOIN project p ON p.id = $2::uuid
		RETURNING id, number, wso2_id, created_by, project_id, deployment_id, deployed_product_id,
		          subject, description, created_on, updated_on
	),
	inserted_service_request AS (
		INSERT INTO service_request (id, state)
		SELECT id, 'OPEN'::service_request_state_enum
		FROM inserted_work_item
		RETURNING id, state, closed_on
	)
	SELECT iwi.id, iwi.number, iwi.wso2_id, iwi.created_by,
	       iwi.project_id, iwi.deployment_id, iwi.deployed_product_id,
	       iwi.subject, iwi.description,
	       NULL::TEXT, NULL::TEXT, isr.state::TEXT, NULL::TEXT,
	       iwi.created_on, iwi.updated_on, isr.closed_on
	FROM inserted_work_item iwi
	JOIN inserted_service_request isr ON isr.id = iwi.id`

// createEngagementPortalQuery is createCasePortalQuery's counterpart for
// req.Type == "engagement" -- same relationship createEngagementFromServiceNowQuery
// has to createCaseFromServiceNowQuery, including engagement.type/payment_type
// being genuinely required columns (validateCreateCaseRequest already
// enforces req.EngagementType/req.EngagementPaymentType are set for this type
// on both paths).
const createEngagementPortalQuery = `
	WITH creator AS (
		SELECT id, email FROM "user" WHERE id = $1::uuid
	),
	inserted_work_item AS (
		INSERT INTO work_item (
			id, created_on, updated_on, created_by, updated_by,
			number, wso2_id, subject, description, type,
			project_id, deployment_id, deployed_product_id, opened_by_user_id, account_id
		)
		SELECT gen_random_uuid(), NOW(), NOW(), creator.email, creator.email,
		       next_portal_work_item_number(), next_portal_wso2_id($2::uuid), $5, $6, 'ENGAGEMENT'::work_item_type_enum,
		       $2::uuid, $3::uuid, $4::uuid, creator.id, p.account_id
		FROM creator
		LEFT JOIN project p ON p.id = $2::uuid
		RETURNING id, number, wso2_id, created_by, project_id, deployment_id, deployed_product_id,
		          subject, description, created_on, updated_on
	),
	inserted_engagement AS (
		INSERT INTO engagement (id, state, type, payment_type)
		SELECT id, 'OPEN'::engagement_state_enum, $7::engagement_type_enum, $8::engagement_payment_type_enum
		FROM inserted_work_item
		RETURNING id, state, closed_on
	)
	SELECT iwi.id, iwi.number, iwi.wso2_id, iwi.created_by,
	       iwi.project_id, iwi.deployment_id, iwi.deployed_product_id,
	       iwi.subject, iwi.description,
	       NULL::TEXT, NULL::TEXT, ieng.state::TEXT, NULL::TEXT,
	       iwi.created_on, iwi.updated_on, ieng.closed_on
	FROM inserted_work_item iwi
	JOIN inserted_engagement ieng ON ieng.id = iwi.id`

// createSecurityReportAnalysisPortalQuery is createCasePortalQuery's
// counterpart for req.Type == "security_report_analysis" -- same relationship
// createSecurityReportAnalysisFromServiceNowQuery has to createCaseFromServiceNowQuery.
const createSecurityReportAnalysisPortalQuery = `
	WITH creator AS (
		SELECT id, email FROM "user" WHERE id = $1::uuid
	),
	inserted_work_item AS (
		INSERT INTO work_item (
			id, created_on, updated_on, created_by, updated_by,
			number, wso2_id, subject, description, type,
			project_id, deployment_id, deployed_product_id, opened_by_user_id, account_id
		)
		SELECT gen_random_uuid(), NOW(), NOW(), creator.email, creator.email,
		       next_portal_work_item_number(), next_portal_wso2_id($2::uuid), $5, $6, 'SECURITY_REPORT_ANALYSIS'::work_item_type_enum,
		       $2::uuid, $3::uuid, $4::uuid, creator.id, p.account_id
		FROM creator
		LEFT JOIN project p ON p.id = $2::uuid
		RETURNING id, number, wso2_id, created_by, project_id, deployment_id, deployed_product_id,
		          subject, description, created_on, updated_on
	),
	inserted_security_report_analysis AS (
		INSERT INTO security_report_analysis (id, state)
		SELECT id, 'OPEN'::security_report_analysis_state_enum
		FROM inserted_work_item
		RETURNING id, state, closed_on
	)
	SELECT iwi.id, iwi.number, iwi.wso2_id, iwi.created_by,
	       iwi.project_id, iwi.deployment_id, iwi.deployed_product_id,
	       iwi.subject, iwi.description,
	       NULL::TEXT, NULL::TEXT, isra.state::TEXT, NULL::TEXT,
	       iwi.created_on, iwi.updated_on, isra.closed_on
	FROM inserted_work_item iwi
	JOIN inserted_security_report_analysis isra ON isra.id = iwi.id`

// mapCreateCaseError turns the database errors CreateCase can hit into API errors.
func mapCreateCaseError(err error) error {
	// createCaseTx runs with the caller's own identity (never WithSystemIdentity
	// -- case creation is meant to work for a registered project member, not
	// just internal callers, unlike change_request/incident/problem's
	// internal-only INSERT policies). A caller who is not a member of
	// req.ProjectID has their work_item INSERT rejected by RLS (SQLSTATE
	// 42501, once work_item's own RLS lands) -- mapped to NotFoundError, the
	// same "can't tell 'doesn't exist' from 'isn't yours'" convention every
	// other RLS-protected write in this codebase already follows.
	if IsRLSPolicyViolation(err) {
		return &apierror.NotFoundError{Msg: "case not found"}
	}
	if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503": // foreign_key_violation -- one of the referenced IDs does not exist
			return &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
		case "P0001": // raise_exception from integrity triggers (deployment/project, deployed_product/deployment, catastrophic priority) and from next_portal_wso2_id when project_id doesn't exist
			return &apierror.ValidationError{Msg: pgErr.Message}
		}
	}
	return fmt.Errorf("create case: %w", err)
}

// createCaseFromServiceNowQuery inserts both halves of a case row (work_item
// + "case", the same shared-primary-key pattern updateCaseQuery documents)
// in one round trip via a CTE, using caller-supplied identity throughout
// rather than generating any of it -- see CreateCaseFromServiceNow's own
// doc comment for why. Used only for req.Type == "case" -- req.Type ==
// "announcement" goes through createAnnouncementFromServiceNowQuery instead
// (see its own doc comment for why announcement needs a structurally
// different insert, not just a different type literal). type is hardcoded
// to 'CASE'::work_item_type_enum (the caller guarantees req.Type == "case"
// on this branch) and state to 'OPEN'::case_state_enum
// (every case ServiceNow creates starts in its own equivalent initial state;
// reliably parsing that back out of ServiceNow's raw create-response state
// label would need the same label->enum lookup sn_case_service.go keeps
// unexported for its own internal use, and would still land on the same
// value every time).
//
// Column/output order matches scanUpdatedCase exactly, so that helper is
// reused verbatim rather than duplicated.
const createCaseFromServiceNowQuery = `
	WITH inserted_work_item AS (
		INSERT INTO work_item (
			id, created_on, updated_on, created_by, updated_by,
			number, wso2_id, subject, description, type,
			project_id, deployment_id, deployed_product_id, account_id
		)
		VALUES (
			$1, NOW(), NOW(), $2, $2,
			$3, $4, $5, $6, 'CASE'::work_item_type_enum,
			$7, $8, $9, (SELECT account_id FROM project WHERE id = $7)
		)
		RETURNING id, number, wso2_id, created_by, project_id, deployment_id, deployed_product_id,
		          subject, description, created_on, updated_on
	),
	inserted_case AS (
		INSERT INTO "case" (id, severity, issue_type, state)
		VALUES ($1, $10::case_severity_enum, $11::case_issue_type_enum, 'OPEN'::case_state_enum)
		RETURNING id, severity, issue_type, state, work_state, closed_on
	)
	SELECT iwi.id, iwi.number, iwi.wso2_id, iwi.created_by, iwi.project_id, iwi.deployment_id, iwi.deployed_product_id,
	       iwi.subject, iwi.description,
	       ic.severity::TEXT, ic.issue_type::TEXT, ic.state::TEXT, ic.work_state::TEXT,
	       iwi.created_on, iwi.updated_on, ic.closed_on
	FROM inserted_work_item iwi
	JOIN inserted_case ic ON ic.id = iwi.id`

// createAnnouncementFromServiceNowQuery is createCaseFromServiceNowQuery's
// counterpart for req.Type == "announcement": announcements are NOT a "case"
// row at all -- they extend work_item through the separate "announcement"
// table (migration 0024), which has no severity/issue_type/work_state
// columns and uses announcement_state_enum (only OPEN/CLOSE) rather than
// case_state_enum. deployment_id/deployed_product_id are hardcoded NULL
// (never parameterized as req.DeploymentID/req.DeployedProductID, which are
// "" for an announcement -- binding "" to a UUID column would fail with
// 22P02, not silently store nothing) since announcements have no
// deployment/deployed-product concept (validateCreateCaseRequest's own
// comment). $8 is the announcement's initial state, already resolved to
// announcement_state_enum's literal ('OPEN' in practice -- see
// snAnnouncementStateToEnum) by the caller, not derived here: this layer
// stays free of ServiceNow label vocabulary. $9 is the announcement's
// announcement_type (GENERAL/SECURITY, migration 0088), resolved from
// req.IsSecurityAnnouncement via announcementTypeEnumValue -- reusing that
// existing field (added for a different purpose, deciding the case's
// default watcher audience) rather than a second, redundant flag for the
// same underlying yes/no. cause/closed_by_user_id/
// closed_on/resolved_on are left NULL -- all four are closure-time-only
// fields, meaningless on a fresh row.
//
// Column/output order matches scanUpdatedCase exactly, same as
// createCaseFromServiceNowQuery, with severity/issue_type/work_state as
// literal NULLs (columns that don't exist on "announcement").
const createAnnouncementFromServiceNowQuery = `
	WITH inserted_work_item AS (
		INSERT INTO work_item (
			id, created_on, updated_on, created_by, updated_by,
			number, wso2_id, subject, description, type,
			project_id, deployment_id, deployed_product_id, account_id
		)
		VALUES (
			$1, NOW(), NOW(), $2, $2,
			$3, $4, $5, $6, 'ANNOUNCEMENT'::work_item_type_enum,
			$7, NULL, NULL, (SELECT account_id FROM project WHERE id = $7)
		)
		RETURNING id, number, wso2_id, created_by, project_id, deployment_id, deployed_product_id,
		          subject, description, created_on, updated_on
	),
	inserted_announcement AS (
		INSERT INTO announcement (id, state, announcement_type)
		VALUES ($1, $8::announcement_state_enum, $9::announcement_type_enum)
		RETURNING id, state, closed_on
	)
	SELECT iwi.id, iwi.number, iwi.wso2_id, iwi.created_by,
	       iwi.project_id, COALESCE(iwi.deployment_id::TEXT, ''), COALESCE(iwi.deployed_product_id::TEXT, ''),
	       iwi.subject, iwi.description,
	       NULL::TEXT, NULL::TEXT, ia.state::TEXT, NULL::TEXT,
	       iwi.created_on, iwi.updated_on, ia.closed_on
	FROM inserted_work_item iwi
	JOIN inserted_announcement ia ON ia.id = iwi.id`

// createServiceRequestFromServiceNowQuery is createCaseFromServiceNowQuery's
// counterpart for req.Type == "service_request": service_request (migration
// 000019) has no severity/issue_type/work_state columns and no catalog/
// variables columns either -- req.CatalogID/req.CatalogItemID/req.Variables
// are ServiceNow-only inputs for the actual case creation (see
// snCaseService.CreateCase's "service_request" branch), not persisted here,
// same "id + state only, everything else NULL" pattern as announcement's own
// insert. Unlike announcement, deployment_id/deployed_product_id ARE bound
// from req -- service_request has no exemption from that requirement (see
// validateCreateCaseRequest's own conditional, which only exempts
// "announcement"). $10 is the service_request's initial state, already
// resolved to service_request_state_enum's literal by the caller (see
// snServiceRequestStateToEnum) -- this layer stays free of ServiceNow label
// vocabulary. cause/closed_by_user_id/closed_on/resolved_on/category/
// autoclosure_step/autoclosure_state_on/json_data are left NULL -- all are
// either closure-time-only or SN-detail fields with no counterpart on
// req/CreateCaseRequest.
//
// Column/output order matches scanUpdatedCase exactly, same as
// createCaseFromServiceNowQuery, with severity/issue_type/work_state as
// literal NULLs (columns that don't exist on "service_request").
const createServiceRequestFromServiceNowQuery = `
	WITH inserted_work_item AS (
		INSERT INTO work_item (
			id, created_on, updated_on, created_by, updated_by,
			number, wso2_id, subject, description, type,
			project_id, deployment_id, deployed_product_id, account_id
		)
		VALUES (
			$1, NOW(), NOW(), $2, $2,
			$3, $4, $5, $6, 'SERVICE_REQUEST'::work_item_type_enum,
			$7, $8, $9, (SELECT account_id FROM project WHERE id = $7)
		)
		RETURNING id, number, wso2_id, created_by, project_id, deployment_id, deployed_product_id,
		          subject, description, created_on, updated_on
	),
	inserted_service_request AS (
		INSERT INTO service_request (id, state)
		VALUES ($1, $10::service_request_state_enum)
		RETURNING id, state, closed_on
	)
	SELECT iwi.id, iwi.number, iwi.wso2_id, iwi.created_by,
	       iwi.project_id, iwi.deployment_id, iwi.deployed_product_id,
	       iwi.subject, iwi.description,
	       NULL::TEXT, NULL::TEXT, isr.state::TEXT, NULL::TEXT,
	       iwi.created_on, iwi.updated_on, isr.closed_on
	FROM inserted_work_item iwi
	JOIN inserted_service_request isr ON isr.id = iwi.id`

// createEngagementFromServiceNowQuery is createCaseFromServiceNowQuery's
// counterpart for req.Type == "engagement": engagement (migration 0024)
// has no severity/issue_type/work_state columns, same "id + state only"
// shape as service_request/security_report_analysis for most fields, EXCEPT
// engagement.type/payment_type -- validateCreateCaseRequest requires
// req.EngagementType/req.EngagementPaymentType for this type, and unlike
// service_request's catalog/variables fields, engagement's extension table
// HAS real columns for these (engagement_type_enum/engagement_payment_type_enum),
// so they are genuinely required, non-defaultable creation-time fields that
// must reach this insert, not silently dropped. req.EngagementType/
// req.EngagementPaymentType's own domain values (e.g. "migration", "foc")
// upper-case directly onto their SQL enum literals (e.g. 'MIGRATION', 'FOC')
// -- confirmed against migration 0024's engagement_type_enum/
// engagement_payment_type_enum value lists, same convention
// strings.ToUpper(string(req.IssueType)) already relies on for "case". $10 is
// the engagement's initial state, resolved the same way service_request's is
// (see snEngagementStateToEnum). deployment_id/deployed_product_id ARE bound
// from req -- engagement has no exemption from that requirement either.
//
// Column/output order matches scanUpdatedCase exactly, same as
// createCaseFromServiceNowQuery, with severity/issue_type/work_state as
// literal NULLs (columns that don't exist on "engagement").
const createEngagementFromServiceNowQuery = `
	WITH inserted_work_item AS (
		INSERT INTO work_item (
			id, created_on, updated_on, created_by, updated_by,
			number, wso2_id, subject, description, type,
			project_id, deployment_id, deployed_product_id, account_id
		)
		VALUES (
			$1, NOW(), NOW(), $2, $2,
			$3, $4, $5, $6, 'ENGAGEMENT'::work_item_type_enum,
			$7, $8, $9, (SELECT account_id FROM project WHERE id = $7)
		)
		RETURNING id, number, wso2_id, created_by, project_id, deployment_id, deployed_product_id,
		          subject, description, created_on, updated_on
	),
	inserted_engagement AS (
		INSERT INTO engagement (id, state, type, payment_type)
		VALUES ($1, $10::engagement_state_enum, $11::engagement_type_enum, $12::engagement_payment_type_enum)
		RETURNING id, state, closed_on
	)
	SELECT iwi.id, iwi.number, iwi.wso2_id, iwi.created_by,
	       iwi.project_id, iwi.deployment_id, iwi.deployed_product_id,
	       iwi.subject, iwi.description,
	       NULL::TEXT, NULL::TEXT, ieng.state::TEXT, NULL::TEXT,
	       iwi.created_on, iwi.updated_on, ieng.closed_on
	FROM inserted_work_item iwi
	JOIN inserted_engagement ieng ON ieng.id = iwi.id`

// createSecurityReportAnalysisFromServiceNowQuery is
// createCaseFromServiceNowQuery's counterpart for req.Type ==
// "security_report_analysis": security_report_analysis (migration 0024)
// has no severity/issue_type/work_state columns and no field with a
// counterpart in req.Attachments (attachments are uploaded via a separate
// request per file, same as validateCreateCaseRequest's own comment on this
// type) -- same "id + state only, everything else NULL" pattern as
// service_request. deployment_id/deployed_product_id ARE bound from req --
// this type has no exemption from that requirement either. $10 is the
// record's initial state, resolved the same way service_request's is (see
// snSecurityReportAnalysisStateToEnum).
//
// Column/output order matches scanUpdatedCase exactly, same as
// createCaseFromServiceNowQuery, with severity/issue_type/work_state as
// literal NULLs (columns that don't exist on "security_report_analysis").
const createSecurityReportAnalysisFromServiceNowQuery = `
	WITH inserted_work_item AS (
		INSERT INTO work_item (
			id, created_on, updated_on, created_by, updated_by,
			number, wso2_id, subject, description, type,
			project_id, deployment_id, deployed_product_id, account_id
		)
		VALUES (
			$1, NOW(), NOW(), $2, $2,
			$3, $4, $5, $6, 'SECURITY_REPORT_ANALYSIS'::work_item_type_enum,
			$7, $8, $9, (SELECT account_id FROM project WHERE id = $7)
		)
		RETURNING id, number, wso2_id, created_by, project_id, deployment_id, deployed_product_id,
		          subject, description, created_on, updated_on
	),
	inserted_security_report_analysis AS (
		INSERT INTO security_report_analysis (id, state)
		VALUES ($1, $10::security_report_analysis_state_enum)
		RETURNING id, state, closed_on
	)
	SELECT iwi.id, iwi.number, iwi.wso2_id, iwi.created_by,
	       iwi.project_id, iwi.deployment_id, iwi.deployed_product_id,
	       iwi.subject, iwi.description,
	       NULL::TEXT, NULL::TEXT, isra.state::TEXT, NULL::TEXT,
	       iwi.created_on, iwi.updated_on, isra.closed_on
	FROM inserted_work_item iwi
	JOIN inserted_security_report_analysis isra ON isra.id = iwi.id`

// existingRefOrNil resolves id to itself if it exists in the given table
// (only "deployment" or "deployed_product" -- table selects a fixed literal
// query, never interpolated), or nil (SQL NULL) if id is empty or the row
// doesn't exist. See CreateCaseFromServiceNow's own call site comment for
// why this exists.
func (r *caseRepo) existingRefOrNil(ctx context.Context, table, id string) (any, error) {
	if id == "" {
		return nil, nil
	}
	var query string
	switch table {
	case "deployment":
		query = `SELECT EXISTS (SELECT 1 FROM deployment WHERE id = $1)`
	case "deployed_product":
		query = `SELECT EXISTS (SELECT 1 FROM deployed_product WHERE id = $1)`
	default:
		return nil, fmt.Errorf("existingRefOrNil: unknown table %q", table)
	}
	var exists bool
	if err := r.db.QueryRow(ctx, query, id).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check %s exists: %w", table, err)
	}
	if !exists {
		return nil, nil
	}
	return id, nil
}

// CreateCaseFromServiceNow implements CaseRepository.
//
// WithSystemIdentity: this is a system write on behalf of the ServiceNow
// sync job, not a specific customer's request -- there is no viewer to
// scope to -- for every branch, not just "announcement". Originally only
// announcement's own branch stamped an identity at all (it alone was
// RLS-protected, migration 000085, and Postgres rejects a failed
// INSERT...RETURNING outright rather than silently returning zero rows the
// way it does for UPDATE/DELETE, so every announcement created by the sync
// job failed without this). Now that work_item/"case" are also RLS-protected
// (migration 0147), every branch's INSERT...RETURNING needs the same
// treatment, so the stamp moved up to cover all five uniformly rather than
// staying a special case.
func (r *caseRepo) CreateCaseFromServiceNow(ctx context.Context, req domain.CreateCaseRequest, id, number, wso2ID, createdBy, state string) (domain.Case, error) {
	ctx = WithSystemIdentity(ctx)
	// deployment_id/deployed_product_id are bound from req for every type
	// except "announcement" (see each query's own doc comment), but the FK
	// they reference is Postgres' own deployment/deployed_product tables --
	// which were never backfilled with ServiceNow's full history (the same
	// gap documented on deploymentService.SearchDeployments and
	// catalogService.snMirror). By the time this method runs, ServiceNow
	// already has the case committed (see createCaseSNFirst's own call
	// site): letting a missing mirror row fail this INSERT outright, as it
	// did before this check existed, leaves a real, permanently orphaned
	// ServiceNow case with no Postgres row at all and a raw 400 shown to the
	// caller (confirmed live: SN case CS0446849 created 2026-09-30, no
	// matching work_item row). Resolving each id to nil when the mirror row
	// doesn't exist writes NULL instead -- already a tolerated state
	// throughout this file (every LEFT JOIN here, and announcement's own
	// unconditional NULL) -- so the case's own Postgres row still gets
	// created, just without that one link, rather than not at all.
	deploymentIDArg, err := r.existingRefOrNil(ctx, "deployment", req.DeploymentID)
	if err != nil {
		return domain.Case{}, err
	}
	deployedProductIDArg, err := r.existingRefOrNil(ctx, "deployed_product", req.DeployedProductID)
	if err != nil {
		return domain.Case{}, err
	}
	if (deploymentIDArg == nil && req.DeploymentID != "") || (deployedProductIDArg == nil && req.DeployedProductID != "") {
		slog.WarnContext(ctx, "sn create case: deployment/deployed product not yet mirrored in postgres, creating case without that link",
			"caseId", id, "snNumber", number, "type", req.Type, "deploymentId", req.DeploymentID, "deployedProductId", req.DeployedProductID)
	}

	var row pgx.Row
	switch req.Type {
	case "announcement":
		row = r.db.QueryRow(ctx, createAnnouncementFromServiceNowQuery,
			id, createdBy,
			number, wso2ID, req.Subject, req.Description,
			req.ProjectID, state, announcementTypeEnumValue(req.IsSecurityAnnouncement),
		)
	case "service_request":
		row = r.db.QueryRow(ctx, createServiceRequestFromServiceNowQuery,
			id, createdBy,
			number, wso2ID, req.Subject, req.Description,
			req.ProjectID, deploymentIDArg, deployedProductIDArg,
			state,
		)
	case "engagement":
		row = r.db.QueryRow(ctx, createEngagementFromServiceNowQuery,
			id, createdBy,
			number, wso2ID, req.Subject, req.Description,
			req.ProjectID, deploymentIDArg, deployedProductIDArg,
			state, strings.ToUpper(string(req.EngagementType)), strings.ToUpper(string(req.EngagementPaymentType)),
		)
	case "security_report_analysis":
		row = r.db.QueryRow(ctx, createSecurityReportAnalysisFromServiceNowQuery,
			id, createdBy,
			number, wso2ID, req.Subject, req.Description,
			req.ProjectID, deploymentIDArg, deployedProductIDArg,
			state,
		)
	default:
		row = r.db.QueryRow(ctx, createCaseFromServiceNowQuery,
			id, createdBy,
			number, wso2ID, req.Subject, req.Description,
			req.ProjectID, deploymentIDArg, deployedProductIDArg,
			caseSeverityToEnum[req.Severity], strings.ToUpper(string(req.IssueType)),
		)
	}
	c, err := scanUpdatedCase(row)
	if err != nil {
		return domain.Case{}, mapCreateCaseFromServiceNowError(err, id)
	}
	return c, nil
}

// mapCreateCaseFromServiceNowError translates a low-level error from any
// CreateCaseFromServiceNow branch into the apierror the handler expects.
func mapCreateCaseFromServiceNowError(err error, id string) error {
	if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation on id/number/wso2_id — see CreateCaseFromServiceNow's own doc comment for why this "shouldn't" happen
			return &apierror.ConflictError{Msg: "a case already exists for this ServiceNow id/number/internalId: " + pgErr.Detail}
		case "22P02": // invalid_text_representation — id was not a valid UUID
			return &apierror.ValidationError{Msg: "id is not a valid UUID: " + id}
		case "23503": // foreign_key_violation — one of the referenced IDs does not exist
			return &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
		case "P0001": // raise_exception from integrity triggers (deployment/project, deployed_product/deployment, catastrophic priority)
			return &apierror.ValidationError{Msg: pgErr.Message}
		}
	}
	return fmt.Errorf("create case from servicenow: %w", err)
}

// GetCaseByID implements CaseRepository.
//
// Serves all five case-like work_item types (caseLikeWorkItemTypes), not
// just CASE -- ENGAGEMENT/SERVICE_REQUEST/SECURITY_REPORT_ANALYSIS/
// ANNOUNCEMENT previously 404'd here even though SearchCases already
// returned them. severity/issue_type/work_state/resolution_code/
// current_escalation_level/is_escalated only ever come from "case" (no
// other extension table has them); state/cause/close_notes/resolved_on/
// closed_on are COALESCEd across whichever extension table actually matches
// wi.type (caseLike*Column consts) since exactly one ever does.
func (r *caseRepo) GetCaseByID(ctx context.Context, id string, scope SearchScope) (domain.CaseView, error) {
	// WithCallerIdentity from the explicit scope parameter, not whatever
	// identity ctx already carries: GetCaseByID's own callers (including
	// caseService.detectPatchTagBillableOverride's internal re-fetch, which
	// passes SearchScope{Unrestricted: true} on a real customer's own
	// request ctx) already resolve the exact identity this call should use
	// and pass it explicitly -- the same convention sla_status_repo.go's
	// SearchActiveSLAStatuses already established for its own always-
	// Unrestricted scope, ported onto Scoped here instead of a direct
	// runWithCallerIdentity call.
	ctx = WithCallerIdentity(ctx, scope)
	var cv domain.CaseView
	var (
		// internalID is scanned as *string even though CaseView.InternalID
		// is a required (non-pointer) string -- wi.wso2_id can genuinely be
		// NULL despite the work_item_wso2_id_required_by_type CHECK
		// constraint (confirmed against real data: that constraint isn't
		// actually enforced for a handful of pre-existing rows), so a
		// non-pointer scan here would panic with "cannot scan NULL into
		// *string". stringOrEmpty below converts it back to "" for the
		// response, matching CaseView.InternalID's own doc comment on why
		// it can't become *string.
		internalID                               *string
		aeID, aeName, aeEmail                    *string
		ackID, ackName, ackEmail                 *string
		pcID, pcNum, pcType                      *string
		rcID, rcNum                              *string
		accountID, accountName, accountTier      *string
		severity, issueType, workState, caseType *string
		announcementType                         *string
		state, cause, closeNotes, resolutionCode *string
		escalationLevel                          *string
		isEscalated                              *bool
		resolvedOn                               *time.Time
		description                              *string
		projID, projName                         *string
		depID, depName                           *string
		dpID, dpDisplayName                      *string
		prodID, prodName                         *string
		creTeamID, creTeamName                   *string
		sreTeamID, sreTeamName                   *string
		creatorEmail                             string
		creatorID, creatorName                   *string
		bestCaseEta, mostLikelyEta, worstCaseEta *time.Time
		etaSharedOn                              *time.Time
	)
	// A scoped caller asking for a case outside their access still gets
	// pgx.ErrNoRows -> NotFoundError below, the same as a genuinely
	// nonexistent id: existence is never revealed to a caller who can't see
	// the case, matching GetProjectByID's own reasoning. No Go-side project
	// filter is needed here any more -- work_item's own RLS policy (migration
	// 0147) already applies the identical is_project_member check to every
	// statement WithCallerIdentity stamps, including this one, so a second
	// hand-written copy would only be a second place for the two to drift.
	scopeArgs := []any{id}

	err := r.db.QueryRow(ctx,
		`SELECT wi.id, wi.number, wi.wso2_id, wi.type::TEXT, ann.announcement_type::TEXT,
		        wi.description, c.severity::TEXT, c.issue_type::TEXT, `+caseLikeWorkStateColumn+`,
		        `+caseLikeStateColumn+`, `+caseLikeCauseColumn+`, `+caseLikeCloseNotesColumn+`,
		        `+caseLikeResolutionCodeColumn+`, c.current_escalation_level::TEXT, c.is_escalated,
		        wi.created_on, wi.updated_on, `+caseLikeClosedOnColumn+`, `+caseLikeResolvedOnColumn+`,
		        wi.subject,
		        wi.best_case_eta, wi.most_likely_eta, wi.worst_case_eta, wi.eta_shared_on,
		        wi.created_by, creator.id, COALESCE(creator.name, NULLIF(TRIM(CONCAT_WS(' ', creator.first_name, creator.last_name)), '')),
		        p.id, p.name,
		        d.id, d.name,
		        dp.id, prod.name || COALESCE(' ' || pv.version, ''),
		        prod.id, prod.name,
		        a.id, a.name, a.support_tier::TEXT,
		        cre.id, cre.name, sre.id, sre.name,
		        ae.id, COALESCE(ae.name, NULLIF(TRIM(CONCAT_WS(' ', ae.first_name, ae.last_name)), '')), ae.email,
		        ack.id, COALESCE(ack.name, NULLIF(TRIM(CONCAT_WS(' ', ack.first_name, ack.last_name)), '')), ack.email,
		        pw.id, pw.number, pw.type::TEXT,
		        rc_wi.id, rc_wi.number
		 FROM work_item wi
		 LEFT JOIN "case" c ON c.id = wi.id
		 `+caseLikeJoins+`
		 LEFT JOIN "user" creator ON LOWER(creator.email) = LOWER(wi.created_by)
		 LEFT JOIN project p ON p.id = wi.project_id
		 LEFT JOIN account a ON a.id = wi.account_id
		 LEFT JOIN "group" cre ON cre.id = a.cre_team_id
		 LEFT JOIN "group" sre ON sre.id = a.sre_team_id
		 LEFT JOIN deployment d ON d.id = wi.deployment_id
		 LEFT JOIN deployed_product dp ON dp.id = wi.deployed_product_id
		 LEFT JOIN product prod ON prod.id = dp.product_id
		 LEFT JOIN product_version pv ON pv.id = dp.version_id
		 LEFT JOIN "user" ae ON ae.id = wi.assigned_to_id
		 LEFT JOIN "user" ack ON ack.id = wi.acknowledged_by_user_id
		 LEFT JOIN work_item pw ON pw.id = wi.parent_id
		 LEFT JOIN "case" rc ON rc.id = c.related_case_id
		 LEFT JOIN work_item rc_wi ON rc_wi.id = rc.id
		 WHERE wi.id = $1 AND wi.type = ANY(`+caseLikeWorkItemTypes+`)
		   AND `+announcementVisibilityLeakGuard+``, scopeArgs...,
	).Scan(
		&cv.ID, &cv.Number, &internalID, &caseType, &announcementType,
		&description, &severity, &issueType, &workState,
		&state, &cause, &closeNotes,
		&resolutionCode, &escalationLevel, &isEscalated,
		&cv.CreatedOn, &cv.UpdatedOn, &cv.ClosedOn, &resolvedOn,
		&cv.Subject,
		&bestCaseEta, &mostLikelyEta, &worstCaseEta, &etaSharedOn,
		&creatorEmail, &creatorID, &creatorName,
		&projID, &projName,
		&depID, &depName,
		&dpID, &dpDisplayName,
		&prodID, &prodName,
		&accountID, &accountName, &accountTier,
		&creTeamID, &creTeamName, &sreTeamID, &sreTeamName,
		&aeID, &aeName, &aeEmail,
		&ackID, &ackName, &ackEmail,
		&pcID, &pcNum, &pcType,
		&rcID, &rcNum,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CaseView{}, &apierror.NotFoundError{Msg: "case not found"}
	}
	if err != nil {
		return domain.CaseView{}, fmt.Errorf("get case by id: %w", err)
	}
	cv.InternalID = stringOrEmpty(internalID)
	// announcement_type only exists on the "announcement" extension table (a
	// real ServiceNow field, u_announcement_type, migrated in migration
	// 0088_announcement_add_announcement_type) -- nil for every other
	// case-like type, where the LEFT JOIN never matches.
	cv.AnnouncementType = announcementType
	// work_item.description (migration 0038) has no NOT NULL constraint,
	// unlike subject; CaseView.Description is a required (non-pointer)
	// string, so a NULL column becomes "" rather than left unset.
	if description != nil {
		cv.Description = *description
	}
	// best_case_eta/most_likely_eta/worst_case_eta are DATE columns --
	// formatted back to the same "YYYY-MM-DD" string shape
	// CaseView.BestCaseFixEta/etc. already use for the ServiceNow-sourced
	// value, so a caller can't tell which data source answered. eta_shared_on
	// is a TIMESTAMPTZ with no ServiceNow equivalent at all -- see
	// CaseView.EtaSharedOn's own doc comment.
	if bestCaseEta != nil {
		s := bestCaseEta.Format("2006-01-02")
		cv.BestCaseFixEta = &s
	}
	if mostLikelyEta != nil {
		s := mostLikelyEta.Format("2006-01-02")
		cv.MostLikelyFixEta = &s
	}
	if worstCaseEta != nil {
		s := worstCaseEta.Format("2006-01-02")
		cv.WorstCaseFixEta = &s
	}
	cv.EtaSharedOn = etaSharedOn
	// case_state_enum/case_issue_type_enum are UPPER_SNAKE_CASE while the
	// domain values are lowercase; case_severity_enum's 'S0'..'S4' labels
	// have no case-only relationship to the domain value at all -- see
	// caseSeverityToEnum's own comment.
	if severity != nil {
		s := caseSeverityFromEnum[*severity]
		cv.Severity = &s
	}
	if issueType != nil {
		it := domain.CaseIssueType(strings.ToLower(*issueType))
		cv.IssueType = &it
	}
	if state != nil {
		st := domain.CaseState(strings.ToLower(*state))
		cv.State = &st
	}
	if cause != nil {
		c := domain.CaseCause(*cause)
		cv.Cause = &c
	}
	// caseLikeCloseNotesColumn genuinely is "close notes" (the only such
	// column this schema has -- "case"/engagement/service_request/
	// security_report_analysis/announcement all name it close_notes, not
	// resolution_notes) and belongs on CaseView.CloseNotes, not
	// CaseView.ResolutionNotes -- a distinct field on the ServiceNow data
	// source (sn_case_service.go sets both from two separate upstream
	// fields) that this schema has no separate column for at all, so it's
	// correctly left nil here rather than double-filled from the same
	// value.
	if closeNotes != nil {
		cv.CloseNotes = closeNotes
	}
	if resolutionCode != nil {
		rc := caseResolutionCodeFromEnum[*resolutionCode]
		cv.ResolutionCode = &rc
	}
	// A freshly created case has NULL current_escalation_level/is_escalated
	// (case creation sets neither), but ServiceNow's own case response always
	// carries a real value for both from the moment a case exists -- level
	// "0"/EL0, isEscalated false -- confirmed live: GET /cases/{id} silently
	// omitted both keys entirely for such a case instead. Default NULL to
	// that same "never escalated" state here, matching the semantic
	// SearchCases' own escalation filter already gives NULL (this file's
	// "escalation (isEmpty / isNotEmpty)" comment: "A row with no ... has no
	// escalation, so it satisfies isEmpty").
	if escalationLevel != nil {
		el := caseEscalationLevelFromEnum(*escalationLevel)
		cv.EscalationLevel = &el
	} else {
		el := caseEscalationLevelFromEnum("EL0")
		cv.EscalationLevel = &el
	}
	isEscalatedOrFalse := isEscalated != nil && *isEscalated
	cv.IsEscalated = &isEscalatedOrFalse
	cv.ResolvedOn = resolvedOn
	if caseType != nil {
		lower := strings.ToLower(*caseType)
		cv.Type = &lower
	}
	// project_id/deployment_id/deployed_product_id (and deployed_product.
	// product_id) are all nullable on work_item (migration 0021) -- a case
	// with no project or deployment linked is a valid state, same as
	// SearchCases already treats it (see that query's own comment). These
	// were previously INNER joins, which meant a case missing any of them
	// came back zero rows here (misreported as 404 "case not found") while
	// still appearing fine in SearchCases's result list.
	if projID != nil {
		cv.ProjectDetails = &domain.EntityRef{ID: *projID, Name: stringOrEmpty(projName)}
	}
	if depID != nil {
		cv.DeploymentDetails = &domain.EntityRef{ID: *depID, Name: stringOrEmpty(depName)}
	}
	if dpID != nil {
		dpRef := &domain.DeployedProductRef{ID: dpID, DisplayName: dpDisplayName}
		if prodID != nil {
			dpRef.Product = &domain.EntityRef{ID: *prodID, Name: stringOrEmpty(prodName)}
		}
		cv.DeployedProductDetails = dpRef
	}
	if accountID != nil {
		name := ""
		if accountName != nil {
			name = *accountName
		}
		// Type (support tier) is account.support_tier (migration 0101,
		// BASIC/ENTERPRISE), same column GetProjectByID already reads into
		// ProjectAccountRef.Tier. CreTeam/SreTeam are account.cre_team_id/
		// sre_team_id, real FKs into "group" -- see accountSelectColumns' own
		// comment in account_repo.go for the same join, added for the
		// dedicated /accounts endpoint.
		accountRef := &domain.AccountRef{ID: *accountID, Name: name, Type: stringOrEmpty(accountTier)}
		if creTeamID != nil {
			accountRef.CreTeam = &domain.EntityRef{ID: *creTeamID, Name: stringOrEmpty(creTeamName)}
		}
		if sreTeamID != nil {
			accountRef.SreTeam = &domain.EntityRef{ID: *sreTeamID, Name: stringOrEmpty(sreTeamName)}
		}
		cv.AccountDetails = accountRef
	}
	if workState != nil {
		ws := domain.CaseWorkState(strings.ToLower(*workState))
		cv.WorkState = &ws
	}
	// work_item.created_by is a free-text VARCHAR (an email), not a UUID FK
	// -- resolved by email match against "user" for a real id/name when
	// possible, same pattern as deployment_repo.go's own created_by fix.
	name := ""
	if creatorName != nil {
		name = *creatorName
	}
	id2 := ""
	if creatorID != nil {
		id2 = *creatorID
	}
	cv.CreatedBy = domain.NewUserReference(id2, creatorEmail, name)
	if aeID != nil {
		aName := ""
		if aeName != nil {
			aName = *aeName
		}
		cv.AssignedEngineer = domain.NewUserReference(*aeID, stringOrEmpty(aeEmail), aName)
	}
	if ackID != nil {
		ackNameStr := ""
		if ackName != nil {
			ackNameStr = *ackName
		}
		cv.AcknowledgedBy = &domain.AssignedEngineerRef{ID: *ackID, Name: ackNameStr, Email: ackEmail}
	}
	if pcID != nil {
		// work_item.parent_id (migration 0039) is a generic self-reference
		// across every work_item type, not case-specific -- unlike
		// RelatedCase below, Type reflects the parent's own real type
		// rather than being hardcoded, so a non-case parent isn't
		// misrepresented as one.
		var t *string
		if pcType != nil {
			lower := strings.ToLower(*pcType)
			t = &lower
		}
		cv.ParentCase = &domain.CaseNumberRef{ID: *pcID, Number: *pcNum, Type: t}
	}
	if rcID != nil {
		// "case".related_case_id (migration 0041) is a foreign key into
		// "case" specifically, so a resolved related record is always
		// another case.
		cv.RelatedCase = &domain.CaseNumberRef{ID: *rcID, Number: *rcNum, Type: &parentRefTypeCase}
	}
	watchers, err := fetchCaseWatchers(ctx, r.db, id)
	if err != nil {
		return domain.CaseView{}, err
	}
	cv.WatchList = watchers

	// Mirrors sn_case_service.go's GetCaseByID: a tags lookup failure must
	// not fail the whole case read (see CaseView.Tags' own doc comment) --
	// cv.Tags is left nil and the failure is logged instead.
	tags, err := fetchCaseTags(ctx, r.db, id)
	if err != nil {
		slog.WarnContext(ctx, "get case: case tags lookup failed", "caseId", id, "error", err)
	} else {
		cv.Tags = tags
	}
	return cv, nil
}

// fetchCaseTags reads the tags currently attached to the case (== work_item)
// identified by caseID, via work_item_tag joined to tag -- the same rows
// AddCaseTag/RemoveCaseTag write. There is no case-scoped "list tags" route
// on this data source (only POST .../tags and DELETE .../tags/{tagId}), so
// GetCaseByID is the only place a caller ever sees a case's current tag set;
// unlike sn_case_service.go's listCaseTags, which calls a real case-scoped
// ServiceNow endpoint, this reads work_item_tag directly.
func fetchCaseTags(ctx context.Context, q rowsQuerier, caseID string) ([]domain.Tag, error) {
	rows, err := q.Query(ctx, `
		SELECT t.id, t.name
		FROM work_item_tag wit
		JOIN tag t ON t.id = wit.tag_id
		WHERE wit.work_item_id = $1
		ORDER BY t.name`, caseID)
	if err != nil {
		return nil, fmt.Errorf("fetch case tags: %w", err)
	}
	defer rows.Close()

	var tags []domain.Tag
	for rows.Next() {
		tag, err := scanTag(rows)
		if err != nil {
			return nil, fmt.Errorf("fetch case tags: scan: %w", err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fetch case tags: %w", err)
	}
	return tags, nil
}

// caseCommentTypeEnum maps a domain.CommentType to its comment_type_enum
// label (migration 0040: APPROVAL_HISTORY, COMMENT, WORK_NOTE) for the
// case-scoped comment methods below. Mirrors commentTypeToEnum in
// internal/service/comment_service.go -- kept as a small local map rather
// than importing the service package (repository must not depend on
// service, per this repo's own layering convention).
var caseCommentTypeEnum = map[domain.CommentType]string{
	domain.CommentTypeComment:  "COMMENT",
	domain.CommentTypeWorkNote: "WORK_NOTE",
	domain.CommentTypeActivity: "APPROVAL_HISTORY",
}

var caseCommentEnumType = map[string]domain.CommentType{
	"COMMENT":          domain.CommentTypeComment,
	"WORK_NOTE":        domain.CommentTypeWorkNote,
	"APPROVAL_HISTORY": domain.CommentTypeActivity,
}

// CreateCaseComment implements CaseRepository. createdOn is nil for an
// ordinary, caller-authored comment (created_on binds to NOW()); mirroring
// a comment ServiceNow already created at a known past time (see
// caseService.mirrorInitialSNComments) passes its own timestamp instead,
// so SearchCaseComments' "ORDER BY created_on DESC" reflects the real
// chronology rather than when the mirror step happened to run.
func (r *caseRepo) CreateCaseComment(ctx context.Context, req domain.CreateCaseCommentRequest, createdOn *time.Time) (domain.CaseComment, error) {
	// APPROVAL_HISTORY only ever arises from ServiceNow's own audit trail,
	// never a caller-authored comment -- see commentService.CreateComment's
	// identical restriction in the generic comment path.
	if req.Type == domain.CommentTypeActivity {
		return domain.CaseComment{}, &apierror.ValidationError{Msg: `type "activity" is not writable through this endpoint`}
	}
	typeEnum, ok := caseCommentTypeEnum[req.Type]
	if !ok {
		return domain.CaseComment{}, &apierror.ValidationError{Msg: "type contains invalid value: " + string(req.Type)}
	}

	// comment.work_item_id references work_item(id) -- INSERT ... SELECT
	// confirms the work item exists in the same round trip, RETURNING zero
	// rows (rather than a hard-to-attribute FK error) when it doesn't. This
	// checks work_item, not the narrower "case" subtype table: this method
	// backs every case-family comment (case, announcement, engagement,
	// service_request, security_report_analysis all share this same
	// endpoint), and only "case" rows have a matching "case" subtype row --
	// an announcement's own type-specific row lives in the "announcement"
	// table instead. Scoping this existence check to "case" specifically
	// made commenting on any non-"case" work item impossible regardless of
	// whether it genuinely existed (reported live: posting an update to a
	// real, existing ANNOUNCEMENT case always failed with "case not found").
	//
	// COALESCE($5, NOW()) rather than two separate query strings: $5 is a
	// nil *time.Time (pgx sends SQL NULL) for the ordinary path, or a real
	// timestamp for the mirror path.
	const query = `
		INSERT INTO comment (id, created_on, created_by, type, work_item_id, content)
		SELECT gen_random_uuid(), COALESCE($5, NOW()), $1, $2::comment_type_enum, w.id, $4
		FROM work_item w
		WHERE w.id = $3
		RETURNING id, work_item_id, type, content, created_by, created_on`

	var c domain.CaseComment
	var typeRaw, createdByEmail string
	err := r.db.QueryRow(ctx, query,
		req.CreatedBy, typeEnum, req.CaseID, req.Content, createdOn,
	).Scan(&c.ID, &c.CaseID, &typeRaw, &c.Content, &createdByEmail, &c.CreatedOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CaseComment{}, &apierror.NotFoundError{Msg: "case not found"}
	}
	if err != nil {
		return domain.CaseComment{}, fmt.Errorf("create case comment: %w", err)
	}
	c.Type = caseCommentEnumType[typeRaw]
	// comment.created_by is a free-text VARCHAR (an email, by this data
	// source's own convention -- see caseService.CreateCaseComment), not a
	// UUID FK, so the reference carries no id here, matching this file's
	// other email-only CreatedBy references (e.g. SearchCaseView.CreatedBy).
	c.CreatedBy = domain.NewUserReference("", createdByEmail, "")
	return c, nil
}

// SearchCaseComments implements CaseRepository.
func (r *caseRepo) SearchCaseComments(ctx context.Context, req domain.SearchCaseCommentsRequest) ([]domain.CaseComment, int, error) {
	args := []any{req.CaseID}
	typeFilter := ""
	if req.Filters != nil && req.Filters.Type != nil {
		args = append(args, caseCommentTypeEnum[*req.Filters.Type])
		typeFilter = fmt.Sprintf(" AND cc.type = $%d::comment_type_enum", len(args))
	}

	countQuery := `SELECT COUNT(*) FROM comment cc WHERE cc.work_item_id = $1` + typeFilter
	// LEFT JOIN "user" by email match: comment.created_by is a free-text
	// VARCHAR (see CreateCaseComment above), not a FK, so a real user id/name
	// is only available when it happens to match a known user's email.
	dataQuery := fmt.Sprintf(`
		SELECT cc.id, cc.work_item_id, cc.type, cc.content, cc.created_by, cc.created_on,
		       u.id, COALESCE(u.name, NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''))
		FROM comment cc
		LEFT JOIN "user" u ON LOWER(u.email) = LOWER(cc.created_by)
		WHERE cc.work_item_id = $1%s
		ORDER BY cc.created_on DESC, cc.id
		LIMIT $%d OFFSET $%d`, typeFilter, len(args)+1, len(args)+2)

	dataArgs := append(args, req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var comments []domain.CaseComment

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count case comments: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query case comments: %w", err)
		}
		defer rows.Close()

		result := make([]domain.CaseComment, 0, req.Pagination.Limit)
		for rows.Next() {
			var c domain.CaseComment
			var typeRaw, authorEmail string
			var authorID, authorName *string
			if err := rows.Scan(
				&c.ID, &c.CaseID, &typeRaw, &c.Content, &authorEmail, &c.CreatedOn,
				&authorID, &authorName,
			); err != nil {
				return fmt.Errorf("scan case comment: %w", err)
			}
			c.Type = caseCommentEnumType[typeRaw]
			if authorID != nil {
				name := ""
				if authorName != nil {
					name = *authorName
				}
				c.CreatedBy = domain.NewUserReference(*authorID, authorEmail, name)
			} else {
				c.CreatedBy = domain.NewUserReference("", authorEmail, "")
			}
			result = append(result, c)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate case comments: %w", err)
		}
		comments = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return comments, total, nil
}

// updateCaseQuery is shared by both branches of UpdateCase below. case.id IS
// work_item.id (migration 0023), so this updates both tables in one round
// trip via a CTE: "case" carries state/severity/work_state/closed_on,
// work_item carries everything else (including updated_on, bumped
// unconditionally). The work_item UPDATE's "AND EXISTS (SELECT 1 FROM
// updated_case)" guard means it only actually touches a row when the case
// update did -- so a nonexistent id updates nothing anywhere and the final
// join returns zero rows, not a partial update.
// updateCaseQuery's $2/$3/$4 arrive already converted to their real enum
// labels by UpdateCase below (state/work_state upper-cased, severity mapped
// through caseSeverityToEnum) -- case_state_enum's "CLOSED" is what $2 = ”
// compares a non-empty state against here, not the domain value "closed".
// $5/$6/$7 (resolutionCode/cause/closeNotes) are only ever non-empty/non-nil
// when req.State is also set to closed/solution_proposed -- CaseService.
// UpdateCase enforces that gate before this query ever runs, mirroring
// sn_case_service.go's own identical restriction ("resolutionCode, cause,
// and closeNotes are only allowed when state is closed or
// solution_proposed"). closeNotes uses COALESCE rather than the ”-sentinel
// trick the other five use, since "" is itself a meaningful value to write
// there (clearing existing notes), not a stand-in for "not provided" --
// unlike an enum column, where ” is never a valid domain value anyway.
const updateCaseQuery = `
	WITH updated_case AS (
		UPDATE "case"
		SET state           = CASE WHEN $2 <> '' THEN $2::case_state_enum ELSE state END,
		    severity        = CASE WHEN $3 <> '' THEN $3::case_severity_enum ELSE severity END,
		    work_state      = CASE WHEN $4 <> '' THEN $4::case_work_state_enum ELSE work_state END,
		    closed_on       = CASE WHEN $2 = 'CLOSED' THEN NOW() WHEN $2 <> '' AND $2 <> 'CLOSED' THEN NULL ELSE closed_on END,
		    resolution_code = CASE WHEN $5 <> '' THEN $5::case_resolution_code_enum ELSE resolution_code END,
		    cause           = CASE WHEN $6 <> '' THEN $6::case_cause_enum ELSE cause END,
		    close_notes     = COALESCE($7, close_notes)
		WHERE id = $1
		RETURNING id, severity, issue_type, state, work_state, closed_on
	),
	updated_work_item AS (
		UPDATE work_item
		SET updated_on = NOW()
		WHERE id = $1 AND EXISTS (SELECT 1 FROM updated_case)
		RETURNING id, number, wso2_id, created_by, project_id, deployment_id, deployed_product_id,
		          subject, description, created_on, updated_on
	)
	SELECT uwi.id, uwi.number, uwi.wso2_id, uwi.created_by, uwi.project_id, uwi.deployment_id, uwi.deployed_product_id,
	       uwi.subject, uwi.description,
	       uc.severity::TEXT, uc.issue_type::TEXT, uc.state::TEXT, uc.work_state::TEXT,
	       uwi.created_on, uwi.updated_on, uc.closed_on
	FROM updated_work_item uwi
	JOIN updated_case uc ON uc.id = uwi.id`

const updateSecurityReportAnalysisQuery = `
	WITH updated_sra AS (
		UPDATE security_report_analysis
		SET state           = CASE WHEN $2 <> '' THEN $2::security_report_analysis_state_enum ELSE state END,
		    closed_on       = CASE WHEN $2 = 'CLOSED' THEN NOW() WHEN $2 <> '' AND $2 <> 'CLOSED' THEN NULL ELSE closed_on END,
		    cause           = CASE WHEN $3 <> '' THEN $3::security_report_analysis_cause_enum ELSE cause END,
		    close_notes     = COALESCE($4, close_notes),
		    work_state      = CASE WHEN $5 <> '' THEN $5::case_work_state_enum ELSE work_state END,
		    resolution_code = CASE WHEN $6 <> '' THEN $6::case_resolution_code_enum ELSE resolution_code END
		WHERE id = $1
		RETURNING id, state, work_state, closed_on
	),
	updated_work_item AS (
		UPDATE work_item
		SET updated_on = NOW()
		WHERE id = $1 AND EXISTS (SELECT 1 FROM updated_sra)
		RETURNING id, number, wso2_id, created_by, project_id, deployment_id, deployed_product_id,
		          subject, description, created_on, updated_on
	)
	SELECT uwi.id, uwi.number, uwi.wso2_id, uwi.created_by, uwi.project_id, uwi.deployment_id, uwi.deployed_product_id,
	       uwi.subject, uwi.description,
	       NULL::TEXT, NULL::TEXT, usra.state::TEXT, usra.work_state::TEXT,
	       uwi.created_on, uwi.updated_on, usra.closed_on
	FROM updated_work_item uwi
	JOIN updated_sra usra ON usra.id = uwi.id`

const updateServiceRequestQuery = `
	WITH updated_sr AS (
		UPDATE service_request
		SET state           = CASE WHEN $2 <> '' THEN $2::service_request_state_enum ELSE state END,
		    closed_on       = CASE WHEN $2 = 'CLOSED' THEN NOW() WHEN $2 <> '' AND $2 <> 'CLOSED' THEN NULL ELSE closed_on END,
		    cause           = CASE WHEN $3 <> '' THEN $3::service_request_cause_enum ELSE cause END,
		    close_notes     = COALESCE($4, close_notes),
		    work_state      = CASE WHEN $5 <> '' THEN $5::case_work_state_enum ELSE work_state END,
		    resolution_code = CASE WHEN $6 <> '' THEN $6::case_resolution_code_enum ELSE resolution_code END
		WHERE id = $1
		RETURNING id, state, work_state, closed_on
	),
	updated_work_item AS (
		UPDATE work_item
		SET updated_on = NOW()
		WHERE id = $1 AND EXISTS (SELECT 1 FROM updated_sr)
		RETURNING id, number, wso2_id, created_by, project_id, deployment_id, deployed_product_id,
		          subject, description, created_on, updated_on
	)
	SELECT uwi.id, uwi.number, uwi.wso2_id, uwi.created_by, uwi.project_id, uwi.deployment_id, uwi.deployed_product_id,
	       uwi.subject, uwi.description,
	       NULL::TEXT, NULL::TEXT, usr.state::TEXT, usr.work_state::TEXT,
	       uwi.created_on, uwi.updated_on, usr.closed_on
	FROM updated_work_item uwi
	JOIN updated_sr usr ON usr.id = uwi.id`

const updateEngagementQuery = `
	WITH updated_eng AS (
		UPDATE engagement
		SET state           = CASE WHEN $2 <> '' THEN $2::engagement_state_enum ELSE state END,
		    closed_on       = CASE WHEN $2 = 'CLOSED' THEN NOW() WHEN $2 <> '' AND $2 <> 'CLOSED' THEN NULL ELSE closed_on END,
		    cause           = CASE WHEN $3 <> '' THEN $3::engagement_cause_enum ELSE cause END,
		    close_notes     = COALESCE($4, close_notes),
		    work_state      = CASE WHEN $5 <> '' THEN $5::case_work_state_enum ELSE work_state END,
		    resolution_code = CASE WHEN $6 <> '' THEN $6::case_resolution_code_enum ELSE resolution_code END
		WHERE id = $1
		RETURNING id, state, work_state, closed_on
	),
	updated_work_item AS (
		UPDATE work_item
		SET updated_on = NOW()
		WHERE id = $1 AND EXISTS (SELECT 1 FROM updated_eng)
		RETURNING id, number, wso2_id, created_by, project_id, deployment_id, deployed_product_id,
		          subject, description, created_on, updated_on
	)
	SELECT uwi.id, uwi.number, uwi.wso2_id, uwi.created_by, uwi.project_id, uwi.deployment_id, uwi.deployed_product_id,
	       uwi.subject, uwi.description,
	       NULL::TEXT, NULL::TEXT, ueng.state::TEXT, ueng.work_state::TEXT,
	       uwi.created_on, uwi.updated_on, ueng.closed_on
	FROM updated_work_item uwi
	JOIN updated_eng ueng ON ueng.id = uwi.id`

const updateAnnouncementQuery = `
	WITH updated_ann AS (
		UPDATE announcement
		SET state       = CASE WHEN $2 <> '' THEN $2::announcement_state_enum ELSE state END,
		    closed_on   = CASE WHEN $2 = 'CLOSE' THEN NOW() WHEN $2 <> '' AND $2 <> 'CLOSE' THEN NULL ELSE closed_on END,
		    cause       = CASE WHEN $3 <> '' THEN $3::announcement_cause_enum ELSE cause END,
		    close_notes = COALESCE($4, close_notes)
		WHERE id = $1
		RETURNING id, state, closed_on
	),
	updated_work_item AS (
		UPDATE work_item
		SET updated_on = NOW()
		WHERE id = $1 AND EXISTS (SELECT 1 FROM updated_ann)
		RETURNING id, number, wso2_id, created_by, project_id, deployment_id, deployed_product_id,
		          subject, description, created_on, updated_on
	)
	SELECT uwi.id, uwi.number, uwi.wso2_id, uwi.created_by, uwi.project_id, uwi.deployment_id, uwi.deployed_product_id,
	       uwi.subject, uwi.description,
	       NULL::TEXT, NULL::TEXT,
	       CASE WHEN uann.state::TEXT = 'CLOSE' THEN 'CLOSED' ELSE uann.state::TEXT END,
	       NULL::TEXT,
	       uwi.created_on, uwi.updated_on, uann.closed_on
	FROM updated_work_item uwi
	JOIN updated_ann uann ON uann.id = uwi.id`

// validateUpdateCaseFieldsForType rejects fields the work item's type has no
// column for, and restricts announcement state to "open" or "closed".
//
// severity is "case"-only. workState and resolutionCode are on every
// case-like type except announcement (migration 0184), matching ServiceNow,
// where u_work_state and resolution_code live on the one case table that
// holds all of these types.
func validateUpdateCaseFieldsForType(workItemType string, req domain.UpdateCaseRequest, state string) error {
	if workItemType != "CASE" && req.Severity != nil {
		return &apierror.ValidationError{Msg: "severity is only supported for cases"}
	}
	if workItemType == "ANNOUNCEMENT" {
		if req.WorkState != nil {
			return &apierror.ValidationError{Msg: "workState is not supported for announcements"}
		}
		if req.ResolutionCode != nil {
			return &apierror.ValidationError{Msg: "resolutionCode is not supported for announcements"}
		}
		if state != "" && state != "OPEN" && state != "CLOSED" {
			return &apierror.ValidationError{Msg: "announcements only support state open or closed"}
		}
	}
	return nil
}

// caseLikeExtensionUpdate returns the update statement and its arguments for
// a non-"case" case-like work item, plus a label for error messages. "case"
// itself goes through updateCaseQuery, which also handles severity.
func caseLikeExtensionUpdate(workItemType string, req domain.UpdateCaseRequest, state, workState, resolutionCode, cause string) (query string, args []any, label string, ok bool) {
	switch workItemType {
	case "SECURITY_REPORT_ANALYSIS":
		return updateSecurityReportAnalysisQuery, []any{req.ID, state, cause, req.CloseNotes, workState, resolutionCode}, "security report analysis", true
	case "SERVICE_REQUEST":
		return updateServiceRequestQuery, []any{req.ID, state, cause, req.CloseNotes, workState, resolutionCode}, "service request", true
	case "ENGAGEMENT":
		return updateEngagementQuery, []any{req.ID, state, cause, req.CloseNotes, workState, resolutionCode}, "engagement", true
	case "ANNOUNCEMENT":
		annState := state
		if annState == "CLOSED" {
			annState = "CLOSE"
		}
		return updateAnnouncementQuery, []any{req.ID, annState, cause, req.CloseNotes}, "announcement", true
	}
	return "", nil, "", false
}

// scanUpdatedCase is shared by both branches of UpdateCase below.
func scanUpdatedCase(row pgx.Row) (domain.Case, error) {
	var c domain.Case
	var internalID *string
	// deploymentID/deployedProductID: normally NOT NULL by the time a case
	// is read back here, but CreateCaseFromServiceNow's own existingRefOrNil
	// can legitimately write NULL for either when the SN-first case's
	// deployment/deployed product isn't yet mirrored in Postgres (see that
	// method's own doc comment) -- a *string scan avoids the same
	// "cannot scan NULL into *string" panic this file already guards
	// against for internalID/severity/etc, confirmed live the first time
	// this path could actually return NULL here.
	var deploymentID, deployedProductID *string
	// projectID/description: work_item.project_id and .description are both
	// nullable, and synced data has NULLs in each (8,414 and 510 case-like
	// rows on the staging copy checked) -- scanned straight into
	// domain.Case's plain strings, every update to such a row failed with
	// "cannot scan NULL into *string" after the write had already happened.
	var projectID, description *string
	var severity, issueType, state, workStateRaw *string
	if err := row.Scan(
		&c.ID, &c.Number, &internalID, &c.CreatedBy,
		&projectID, &deploymentID, &deployedProductID,
		&c.Subject, &description, &severity, &issueType, &state, &workStateRaw,
		&c.CreatedOn, &c.UpdatedOn, &c.ClosedOn,
	); err != nil {
		return domain.Case{}, err
	}
	c.InternalID = stringOrEmpty(internalID)
	c.ProjectID = stringOrEmpty(projectID)
	c.Description = stringOrEmpty(description)
	c.DeploymentID = stringOrEmpty(deploymentID)
	c.DeployedProductID = stringOrEmpty(deployedProductID)
	if severity != nil {
		s := caseSeverityFromEnum[*severity]
		c.Severity = &s
	}
	if issueType != nil {
		it := domain.CaseIssueType(strings.ToLower(*issueType))
		c.IssueType = &it
	}
	if state != nil {
		st := domain.CaseState(strings.ToLower(*state))
		c.State = &st
	}
	if workStateRaw != nil {
		ws := domain.CaseWorkState(strings.ToLower(*workStateRaw))
		c.WorkState = &ws
	}
	return c, nil
}

// UpdateCase implements CaseRepository.
func (r *caseRepo) UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, error) {
	state := ""
	if req.State != nil {
		state = strings.ToUpper(string(*req.State))
	}
	severity := ""
	if req.Severity != nil {
		severity = caseSeverityToEnum[*req.Severity]
	}
	workState := ""
	if req.WorkState != nil {
		workState = strings.ToUpper(string(*req.WorkState))
	}
	resolutionCode := ""
	if req.ResolutionCode != nil {
		enumVal, ok := caseResolutionCodeToEnum[*req.ResolutionCode]
		if !ok {
			return domain.Case{}, nil, &apierror.ValidationError{Msg: "resolutionCode contains invalid value: " + string(*req.ResolutionCode)}
		}
		resolutionCode = enumVal
	}
	cause := ""
	if req.Cause != nil {
		cause = string(*req.Cause)
	}

	var workItemType string
	err := r.db.QueryRow(ctx, `SELECT type::TEXT FROM work_item WHERE id = $1 AND type = ANY(`+caseLikeWorkItemTypes+`)`, req.ID).Scan(&workItemType)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Case{}, nil, &apierror.NotFoundError{Msg: "case not found"}
	}
	if err != nil {
		return domain.Case{}, nil, fmt.Errorf("update case: read type: %w", err)
	}

	if err := validateUpdateCaseFieldsForType(workItemType, req, state); err != nil {
		return domain.Case{}, nil, err
	}

	// state=closed: a case cannot close while any child case (work_item.parent_id
	// pointing at it; the case detail's "Child cases" list) is still open. The
	// case detail's link dialog states this rule; nothing on this data source
	// enforced it.
	if state == "CLOSED" {
		var openChild string
		err := r.db.QueryRow(ctx, `
			SELECT wi.number
			FROM work_item wi
			LEFT JOIN "case" c ON c.id = wi.id
			`+caseLikeJoins+`
			WHERE wi.parent_id = $1::uuid
			  AND wi.type = ANY(`+caseLikeWorkItemTypes+`)
			  AND COALESCE(`+caseLikeStateColumn+`, '') <> 'CLOSED'
			LIMIT 1`, req.ID).Scan(&openChild)
		if err == nil {
			return domain.Case{}, nil, &apierror.ConflictError{Msg: "Cannot close this case while child case " + openChild + " is still open"}
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return domain.Case{}, nil, fmt.Errorf("update case: check open child cases: %w", err)
		}
	}

	switch workItemType {
	case "CASE":
		// workState=ongoing: an engineer may hold only one ONGOING case. The
		// mirrored data source enforces the same rule, so accepting a second one
		// here would only ever surface later as a failed mirror write. Checked in
		// a transaction serialized per assignee.
		if req.WorkState != nil && workState == "ONGOING" {
			return r.updateCaseEnforcingOneOngoing(ctx, req, workItemType, state, severity, workState, resolutionCode, cause)
		}

		// req.Severity == nil: severity can't change, so there's nothing to
		// race on — skip the transaction/lock overhead entirely.
		if req.Severity == nil {
			c, err := scanUpdatedCase(r.db.QueryRow(ctx, updateCaseQuery, req.ID, state, severity, workState, resolutionCode, cause, req.CloseNotes))
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.Case{}, nil, &apierror.NotFoundError{Msg: "case not found"}
			}
			if err != nil {
				return domain.Case{}, nil, fmt.Errorf("update case: %w", err)
			}
			return c, c.Severity, nil
		}

		// req.Severity != nil: lock the row first so the previous severity this
		// returns is accurate even under a concurrent update to the same case —
		// see this method's own interface doc comment for why that matters.
		var c domain.Case
		var previousSeverityRaw *string
		err := r.db.InTx(ctx, func(tx pgx.Tx) error {
			if err := tx.QueryRow(ctx, `SELECT severity::TEXT FROM "case" WHERE id = $1 FOR UPDATE`, req.ID).Scan(&previousSeverityRaw); err != nil {
				return err
			}
			var txErr error
			c, txErr = scanUpdatedCase(tx.QueryRow(ctx, updateCaseQuery, req.ID, state, severity, workState, resolutionCode, cause, req.CloseNotes))
			if txErr != nil {
				return txErr
			}

			// Recompute time-card billability only on a genuine LOW/S4 boundary
			// crossing (not every severity change) -- see
			// recomputeTimeCardsBillable's own doc comment for why this runs
			// inside this same transaction, under the row lock just taken
			// above, rather than as a separate call after commit. Best-effort:
			// logged, never allowed to roll back a severity change that
			// otherwise succeeded.
			oldLow := previousSeverityRaw != nil && caseSeverityFromEnum[*previousSeverityRaw] == domain.CaseSeverityLow
			newLow := c.Severity != nil && *c.Severity == domain.CaseSeverityLow
			if oldLow != newLow {
				if _, err := recomputeTimeCardsBillable(ctx, tx, req.ID, newLow); err != nil {
					slog.ErrorContext(ctx, "update case: recompute time cards billable failed", "caseId", req.ID, "error", err)
				}
			}
			return nil
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Case{}, nil, &apierror.NotFoundError{Msg: "case not found"}
		}
		if err != nil {
			return domain.Case{}, nil, fmt.Errorf("update case: %w", err)
		}
		var previousSeverity *domain.CaseSeverity
		if previousSeverityRaw != nil {
			s := caseSeverityFromEnum[*previousSeverityRaw]
			previousSeverity = &s
		}
		return c, previousSeverity, nil

	default:
		query, args, label, ok := caseLikeExtensionUpdate(workItemType, req, state, workState, resolutionCode, cause)
		if !ok {
			return domain.Case{}, nil, &apierror.ValidationError{Msg: fmt.Sprintf("unsupported work item type: %s", workItemType)}
		}
		// Same one-Ongoing-per-engineer rule as "case": ServiceNow keeps the
		// work state of every case-like type in one field on one table.
		if req.WorkState != nil && workState == "ONGOING" {
			return r.updateCaseEnforcingOneOngoing(ctx, req, workItemType, state, severity, workState, resolutionCode, cause)
		}
		c, err := scanUpdatedCase(r.db.QueryRow(ctx, query, args...))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Case{}, nil, &apierror.NotFoundError{Msg: "case not found"}
		}
		if err != nil {
			return domain.Case{}, nil, fmt.Errorf("update case: %s: %w", label, err)
		}
		return c, nil, nil
	}
}

// CreateCaseAttachment implements CaseRepository.
func (r *caseRepo) CreateCaseAttachment(ctx context.Context, req domain.CreateAttachmentRequest) (domain.Attachment, error) {
	const query = `
		INSERT INTO case_attachment (case_id, storage_key, filename, mime_type, size_bytes, description, uploaded_by, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, case_id, storage_key, filename, mime_type, size_bytes, description, uploaded_by, created_on, status`

	var (
		a            domain.Attachment
		storageKey   string
		uploadedByID string
	)
	err := r.db.QueryRow(ctx, query,
		req.ReferenceID, req.StorageKey, req.Name, req.Type, req.SizeBytes, req.Description, req.CreatedBy, req.Status,
	).Scan(
		&a.ID, &a.ReferenceID, &storageKey, &a.Name, &a.Type, &a.SizeBytes, &a.Description,
		&uploadedByID, &a.CreatedOn, &a.Status,
	)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503": // foreign_key_violation — case_id or uploaded_by does not exist
				return domain.Attachment{}, &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
			case "23514": // check_violation — e.g. size_bytes <= 0 or an invalid status
				return domain.Attachment{}, &apierror.ValidationError{Msg: pgErr.Message}
			}
		}
		return domain.Attachment{}, fmt.Errorf("create case attachment: %w", err)
	}
	a.ReferenceType = domain.ReferenceTypeCase
	a.StorageKey = &storageKey
	// The insert returns only the uploader's id; email and display name would
	// need a further join, so the reference carries the id alone, same as
	// CreateCaseComment above.
	a.CreatedBy = domain.NewUserReference(uploadedByID, "", "")
	return a, nil
}

// ConfirmCaseAttachment implements CaseRepository.
func (r *caseRepo) ConfirmCaseAttachment(ctx context.Context, id string) (domain.Attachment, error) {
	const query = `
		UPDATE case_attachment
		SET status = 'complete', updated_on = NOW()
		WHERE id = $1 AND status = 'pending'
		RETURNING id, case_id, storage_key, filename, mime_type, size_bytes, description, uploaded_by, created_on, status`

	var (
		a            domain.Attachment
		storageKey   string
		uploadedByID string
	)
	err := r.db.QueryRow(ctx, query, id).Scan(
		&a.ID, &a.ReferenceID, &storageKey, &a.Name, &a.Type, &a.SizeBytes, &a.Description,
		&uploadedByID, &a.CreatedOn, &a.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Attachment{}, &apierror.ConflictError{Msg: "attachment is not pending (it may already be confirmed, or was confirmed/deleted concurrently)"}
	}
	if err != nil {
		return domain.Attachment{}, fmt.Errorf("confirm case attachment: %w", err)
	}
	a.ReferenceType = domain.ReferenceTypeCase
	a.StorageKey = &storageKey
	a.CreatedBy = domain.NewUserReference(uploadedByID, "", "")
	return a, nil
}

// SearchCaseAttachments implements CaseRepository.
//
// Both queries filter to status = 'complete' -- see the doc comment on
// CaseService.SearchCaseAttachments for why pending (still-uploading) rows
// are excluded from the default list/search response rather than shown with
// a visible status.
func (r *caseRepo) SearchCaseAttachments(ctx context.Context, caseID string, pagination domain.Pagination) ([]domain.Attachment, int, error) {
	const countQuery = `SELECT COUNT(*) FROM case_attachment WHERE case_id = $1 AND status = 'complete'`
	const dataQuery = `
		SELECT ca.id, ca.case_id, ca.filename, ca.mime_type, ca.size_bytes, ca.description,
		       u.id, u.email, COALESCE(u.name, NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''), '') AS full_name,
		       ca.created_on, ca.storage_key, ca.status
		FROM case_attachment ca
		JOIN "user" u ON u.id = ca.uploaded_by
		WHERE ca.case_id = $1 AND ca.status = 'complete'
		ORDER BY ca.created_on DESC, ca.id
		LIMIT $2 OFFSET $3`

	var total int
	var attachments []domain.Attachment

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, caseID).Scan(&total); err != nil {
			return fmt.Errorf("count case attachments: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, caseID, pagination.Limit, pagination.Offset)
		if err != nil {
			return fmt.Errorf("query case attachments: %w", err)
		}
		defer rows.Close()

		result := make([]domain.Attachment, 0, pagination.Limit)
		for rows.Next() {
			var (
				a                         domain.Attachment
				uploaderID, uploaderEmail string
				uploaderName, storageKey  string
			)
			if err := rows.Scan(
				&a.ID, &a.ReferenceID, &a.Name, &a.Type, &a.SizeBytes, &a.Description,
				&uploaderID, &uploaderEmail, &uploaderName, &a.CreatedOn, &storageKey, &a.Status,
			); err != nil {
				return fmt.Errorf("scan case attachment: %w", err)
			}
			a.ReferenceType = domain.ReferenceTypeCase
			a.CreatedBy = domain.NewUserReference(uploaderID, uploaderEmail, uploaderName)
			a.StorageKey = &storageKey
			result = append(result, a)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate case attachments: %w", err)
		}
		attachments = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return attachments, total, nil
}

// GetCaseAttachmentByID implements CaseRepository.
//
// Unlike SearchCaseAttachments, this intentionally does not filter by status:
// a direct id lookup must return a 'pending' row too, since that's how the
// confirm step resolves the row it's about to transition, and how an
// uploader can check on their own in-flight upload. See the doc comment on
// CaseService.SearchCaseAttachments for the read-path status decision.
func (r *caseRepo) GetCaseAttachmentByID(ctx context.Context, id string) (domain.Attachment, error) {
	const query = `
		SELECT ca.id, ca.case_id, ca.filename, ca.mime_type, ca.size_bytes, ca.description,
		       u.id, u.email, COALESCE(u.name, NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''), '') AS full_name,
		       ca.created_on, ca.storage_key, ca.status
		FROM case_attachment ca
		JOIN "user" u ON u.id = ca.uploaded_by
		WHERE ca.id = $1`

	var (
		a                         domain.Attachment
		uploaderID, uploaderEmail string
		uploaderName, storageKey  string
	)
	err := r.db.QueryRow(ctx, query, id).Scan(
		&a.ID, &a.ReferenceID, &a.Name, &a.Type, &a.SizeBytes, &a.Description,
		&uploaderID, &uploaderEmail, &uploaderName, &a.CreatedOn, &storageKey, &a.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Attachment{}, &apierror.NotFoundError{Msg: "attachment not found"}
	}
	if err != nil {
		return domain.Attachment{}, fmt.Errorf("get case attachment by id: %w", err)
	}
	a.ReferenceType = domain.ReferenceTypeCase
	a.CreatedBy = domain.NewUserReference(uploaderID, uploaderEmail, uploaderName)
	a.StorageKey = &storageKey
	return a, nil
}

// DeleteCaseAttachment implements CaseRepository.
func (r *caseRepo) DeleteCaseAttachment(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM case_attachment WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete case attachment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &apierror.NotFoundError{Msg: "attachment not found"}
	}
	return nil
}

// UpdateCaseAttachmentName implements CaseRepository.
func (r *caseRepo) UpdateCaseAttachmentName(ctx context.Context, id, name, updatedBy string) (time.Time, error) {
	const query = `
		UPDATE case_attachment
		SET filename = $2, updated_on = NOW(), updated_by = $3
		WHERE id = $1
		RETURNING updated_on`

	var updatedOn time.Time
	err := r.db.QueryRow(ctx, query, id, name, updatedBy).Scan(&updatedOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, &apierror.NotFoundError{Msg: "attachment not found"}
	}
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return time.Time{}, &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
		}
		return time.Time{}, fmt.Errorf("update case attachment name: %w", err)
	}
	return updatedOn, nil
}

// caseLikeStateSortColumn ranks caseLikeStateColumn's label by
// case_state_enum's own declared order (migration 0023: WORK_IN_PROGRESS,
// AWAITING_INFO, SOLUTION_PROPOSED, CLOSED, OPEN, WAITING_ON_WSO2, REOPENED)
// rather than sorting the label as text. Native Postgres enum comparison
// (what bare c.state ordered by before this existed) already followed that
// declared order for CASE rows; caseLikeStateColumn casts to TEXT so every
// case-like type can share one expression, but ORDER BY on that TEXT would
// sort alphabetically instead -- e.g. AWAITING_INFO before CLOSED and
// WORK_IN_PROGRESS, a real change to the customer-facing sort order. This
// preserves the original order while still applying to every case-like type,
// not just CASE.
const caseLikeStateSortColumn = `CASE ` + caseLikeStateColumn + `
	WHEN 'WORK_IN_PROGRESS' THEN 0
	WHEN 'AWAITING_INFO' THEN 1
	WHEN 'SOLUTION_PROPOSED' THEN 2
	WHEN 'CLOSED' THEN 3
	WHEN 'OPEN' THEN 4
	WHEN 'WAITING_ON_WSO2' THEN 5
	WHEN 'REOPENED' THEN 6
	END`

// pgSortColMap maps domain CaseSortField values to Postgres column expressions.
// State uses caseLikeStateSortColumn, not a bare "case" column, for the same
// reason the state filter uses caseLikeStateColumn (see that const's own doc
// comment): "case" is a LEFT JOIN here, so a service_request/engagement/
// security_report_analysis/announcement row's own c.state is always NULL,
// and sorting on it left every non-CASE row unsorted by state. Severity stays
// c.severity -- it's genuinely case-only, unlike state.
var pgSortColMap = map[domain.CaseSortField]string{
	domain.CaseSortFieldCreatedOn: "wi.created_on",
	domain.CaseSortFieldUpdatedOn: "wi.updated_on",
	domain.CaseSortFieldSeverity:  "c.severity",
	domain.CaseSortFieldState:     caseLikeStateSortColumn,
}

// onboardingStatusLabels maps a projectOnboardingStatus filter value (keyed by
// its normalized form, see onboardingStatusEnumLabels) to
// onboarding_status_enum. The filter's vocabulary is ServiceNow's choice labels
// ("In-Progress", "Not-Applicable", "OnHold"), which are not the enum's spelling,
// so values are compared with case, hyphens, underscores and spaces ignored.
var onboardingStatusLabels, onboardingStatusNames = buildOnboardingStatusTables([][2]string{
	{"Cancelled", "CANCELLED"},
	{"Completed", "COMPLETED"},
	{"Expired", "EXPIRED"},
	{"In-Progress", "IN_PROGRESS"},
	{"Not-Applicable", "NOT_APPLICABLE"},
	{"Not-Started", "NOT_STARTED"},
	{"On-Hold", "ON_HOLD"},
})

// buildOnboardingStatusTables derives the normalized-key lookup and the
// display names for error messages from one {name, label} list.
func buildOnboardingStatusTables(pairs [][2]string) (map[string]string, []string) {
	labels := make(map[string]string, len(pairs))
	names := make([]string, 0, len(pairs))
	for _, p := range pairs {
		labels[onboardingStatusKeyStripper.Replace(strings.ToLower(p[0]))] = p[1]
		names = append(names, p[0])
	}
	return labels, names
}

// lowerAll returns a lower-cased copy of values.
func lowerAll(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = strings.ToLower(v)
	}
	return out
}

var onboardingStatusKeyStripper = strings.NewReplacer("-", "", "_", "", " ", "")

// onboardingStatusEnumLabels translates field's onboarding status filter values
// to onboarding_status_enum labels. An unknown value is a ValidationError
// rather than a silent no-match: for notIn that would widen the result set.
func onboardingStatusEnumLabels(field string, values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	for _, v := range values {
		label, ok := onboardingStatusLabels[onboardingStatusKeyStripper.Replace(strings.ToLower(strings.TrimSpace(v)))]
		if !ok {
			return nil, apierror.InvalidValue(field, v, "onboarding status", onboardingStatusNames)
		}
		out = append(out, label)
	}
	return out, nil
}

// caseSearchJoins is the join set buildCaseSearchWhere's predicates refer to.
const caseSearchJoins = `LEFT JOIN "case" c ON c.id = wi.id
		 ` + caseLikeJoins + `
		 LEFT JOIN project p ON p.id = wi.project_id
		 LEFT JOIN account a ON a.id = wi.account_id
		 LEFT JOIN "group" cre ON cre.id = a.cre_team_id
		 LEFT JOIN "group" sre ON sre.id = a.sre_team_id
		 LEFT JOIN deployment d ON d.id = wi.deployment_id
		 LEFT JOIN deployed_product dp ON dp.id = wi.deployed_product_id
		 LEFT JOIN product prod ON prod.id = dp.product_id
		 LEFT JOIN product_version pv ON pv.id = dp.version_id
		 LEFT JOIN "user" ae ON ae.id = wi.assigned_to_id
		 LEFT JOIN work_item pw ON pw.id = wi.parent_id
		 LEFT JOIN "case" rc ON rc.id = c.related_case_id
		 LEFT JOIN work_item rc_wi ON rc_wi.id = rc.id`

// buildCaseSearchWhere renders the WHERE clause (and its bound arguments) shared by
// SearchCases and AggregateCases. It expects the joins in caseSearchJoins.
func buildCaseSearchWhere(req domain.SearchCasesRequest, scope SearchScope) (string, []any, int, error) {
	filterArgs := []any{}
	argIdx := 1

	where := "WHERE 1=1"

	// No Go-side project AUTHORIZATION here any more (the viewerProjectHint
	// below is a redundant planner hint, not a security check) -- work_item's own RLS policy
	// (migration 0147) already applies the identical is_project_member check
	// to every statement SearchCases/AggregateCases stamp with
	// WithCallerIdentity, including this one. A scoped caller asking for a project outside their own access
	// gets zero rows from Postgres itself, never someone else's data,
	// regardless of what req.Parsed.ProjectIDs (below) additionally asks
	// for.

	// See announcementVisibilityLeakGuard's own doc comment. Shared by the
	// data, COUNT and aggregate queries, which all use this WHERE.
	where += " AND " + announcementLeakGuardFor(scope)
	// Planner hint for external callers only (see viewerProjectHint): lets
	// Postgres use idx_work_item_project_id instead of scanning all of
	// work_item; RLS above remains the authorization boundary.
	where += viewerProjectHint("wi", scope)

	// Fields shared with anyOf branches are built by one function so the two
	// cannot drift apart (see caseFieldPredicates for the column notes).
	fieldPreds, fieldArgs, nextIdx, err := caseFieldPredicates(caseFieldSet{
		Types: req.Parsed.Types, ProjectIDs: req.Parsed.ProjectIDs, DeploymentIDs: req.Parsed.DeploymentIDs,
		AssignedUserIDs: req.Parsed.AssignedUserIDs, States: req.Parsed.States, Severities: req.Parsed.Severities,
		IssueTypes: req.Parsed.IssueTypes, EngagementTypes: req.Parsed.EngagementTypes, WorkStates: req.Parsed.WorkStates,
		EscalationLevels: req.Parsed.EscalationLevels, Tags: req.Parsed.Tags, ExcludeTags: req.Parsed.ExcludeTags,
		DefaultTypes: true,
	}, argIdx)
	if err != nil {
		return "", nil, argIdx, err
	}
	for _, pred := range fieldPreds {
		where += " AND " + pred
	}
	filterArgs = append(filterArgs, fieldArgs...)
	argIdx = nextIdx

	if len(req.Parsed.ExcludeProjectIDs) > 0 {
		// project_id is on work_item ("case" has no such column), and a case with
		// no project is not in any excluded project, so it satisfies notIn.
		where += fmt.Sprintf(" AND (wi.project_id IS NULL OR wi.project_id <> ALL($%d::uuid[]))", argIdx)
		filterArgs = append(filterArgs, req.Parsed.ExcludeProjectIDs)
		argIdx++
	}

	if len(req.Parsed.CreatedBy) > 0 {
		// work_item.created_by is already a free-text email (not a UUID FK
		// needing a join) -- see this file's other created_by fixes.
		where += fmt.Sprintf(" AND wi.created_by = ANY($%d)", argIdx)
		filterArgs = append(filterArgs, req.Parsed.CreatedBy)
		argIdx++
	}

	// parentId: child cases of this case/incident, via the generic
	// work_item.parent_id self-reference (migration 0039) -- the same
	// column GetCaseByID's own ParentCase resolves in the other direction.
	// Not part of caseFieldPredicates: rejectUnsupportedOrGroupFields already
	// refuses parentId inside an anyOf branch on every data source, so this
	// only ever needs to apply at the top level.
	if req.Parsed.ParentID != nil {
		where += fmt.Sprintf(" AND wi.parent_id = $%d::uuid", argIdx)
		filterArgs = append(filterArgs, *req.Parsed.ParentID)
		argIdx++
	}

	// number/internalId: exact-match filters parsed by case_filters.go (see
	// ParsedCaseFilters.Number/InternalID's own doc comments) but never
	// actually applied here until now -- every by-number/by-internal-id
	// lookup (SPL's postgresSplCaseClient.resolveCaseByNumber chief among
	// them, since entity-service's GET /cases/{id} only accepts the internal
	// UUID) silently ignored this filter and fell through to whatever the
	// sort/limit happened to pick, ordinarily the single most-recently-
	// created case overall regardless of the requested number.
	if req.Parsed.Number != nil {
		where += fmt.Sprintf(" AND wi.number = $%d", argIdx)
		filterArgs = append(filterArgs, *req.Parsed.Number)
		argIdx++
	}
	if req.Parsed.InternalID != nil {
		where += fmt.Sprintf(" AND wi.wso2_id = $%d", argIdx)
		filterArgs = append(filterArgs, *req.Parsed.InternalID)
		argIdx++
	}

	if req.Parsed.ClosedStartDate != nil {
		where += fmt.Sprintf(" AND c.closed_on >= $%d", argIdx)
		filterArgs = append(filterArgs, req.Parsed.ClosedStartDate)
		argIdx++
	}
	if req.Parsed.ClosedEndDate != nil {
		where += fmt.Sprintf(" AND c.closed_on <= $%d", argIdx)
		filterArgs = append(filterArgs, req.Parsed.ClosedEndDate)
		argIdx++
	}
	// resolvedOn: "case".resolved_on (migration 0023). Case-only, like closedOn.
	if req.Parsed.ResolvedStartDate != nil {
		where += fmt.Sprintf(" AND c.resolved_on >= $%d", argIdx)
		filterArgs = append(filterArgs, req.Parsed.ResolvedStartDate)
		argIdx++
	}
	if req.Parsed.ResolvedEndDate != nil {
		where += fmt.Sprintf(" AND c.resolved_on <= $%d", argIdx)
		filterArgs = append(filterArgs, req.Parsed.ResolvedEndDate)
		argIdx++
	}
	// state notIn: a row with no state satisfies it (the inverse of state in).
	if len(req.Parsed.ExcludeStates) > 0 {
		states := make([]string, len(req.Parsed.ExcludeStates))
		for i, st := range req.Parsed.ExcludeStates {
			states[i] = strings.ToUpper(string(st))
		}
		where += fmt.Sprintf(" AND ("+caseLikeStateColumn+" IS NULL OR "+caseLikeStateColumn+" <> ALL($%d::text[]))", argIdx)
		filterArgs = append(filterArgs, states)
		argIdx++
	}
	// projectType: matched on project_type.name (p is the LEFT JOIN above).
	if len(req.Parsed.ProjectTypeNames) > 0 {
		where += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM project_type pt WHERE pt.id = p.project_type_id AND pt.name = ANY($%d::text[]))", argIdx)
		filterArgs = append(filterArgs, req.Parsed.ProjectTypeNames)
		argIdx++
	}
	// slaBreached: at least one SLA row of the case has breached. NOTE: the
	// ServiceNow side restricts this to ten named SLA definitions; this
	// matches any breached row until that list is mapped onto sla_policy.
	if req.Parsed.HasBreachedSLA != nil {
		if *req.Parsed.HasBreachedSLA {
			where += " AND EXISTS (SELECT 1 FROM sla bs WHERE bs.work_item_id = wi.id AND bs.has_breached IS TRUE)"
		} else {
			where += " AND NOT EXISTS (SELECT 1 FROM sla bs WHERE bs.work_item_id = wi.id AND bs.has_breached IS TRUE)"
		}
	}
	if req.Parsed.StartCreatedDate != nil {
		where += fmt.Sprintf(" AND wi.created_on >= $%d", argIdx)
		filterArgs = append(filterArgs, req.Parsed.StartCreatedDate)
		argIdx++
	}
	if req.Parsed.EndCreatedDate != nil {
		where += fmt.Sprintf(" AND wi.created_on <= $%d", argIdx)
		filterArgs = append(filterArgs, req.Parsed.EndCreatedDate)
		argIdx++
	}
	if req.Parsed.StartUpdatedDate != nil {
		where += fmt.Sprintf(" AND wi.updated_on >= $%d", argIdx)
		filterArgs = append(filterArgs, req.Parsed.StartUpdatedDate)
		argIdx++
	}
	if req.Parsed.EndUpdatedDate != nil {
		where += fmt.Sprintf(" AND wi.updated_on <= $%d", argIdx)
		filterArgs = append(filterArgs, req.Parsed.EndUpdatedDate)
		argIdx++
	}

	// projectOnboardingStatus: the parent project's onboarding_status (p is
	// the LEFT JOIN below). A case whose project has no status set (NULL)
	// satisfies notIn -- "not in progress" is true of it -- but never in.
	if len(req.Parsed.ProjectOnboardingStatuses) > 0 {
		labels, err := onboardingStatusEnumLabels("projectOnboardingStatus", req.Parsed.ProjectOnboardingStatuses)
		if err != nil {
			return "", nil, argIdx, err
		}
		where += fmt.Sprintf(" AND p.onboarding_status = ANY($%d::text[]::onboarding_status_enum[])", argIdx)
		filterArgs = append(filterArgs, labels)
		argIdx++
	}
	if len(req.Parsed.ExcludeProjectOnboardingStatuses) > 0 {
		labels, err := onboardingStatusEnumLabels("projectOnboardingStatus", req.Parsed.ExcludeProjectOnboardingStatuses)
		if err != nil {
			return "", nil, argIdx, err
		}
		where += fmt.Sprintf(" AND (p.onboarding_status IS NULL OR p.onboarding_status <> ALL($%d::text[]::onboarding_status_enum[]))", argIdx)
		filterArgs = append(filterArgs, labels)
		argIdx++
	}

	// taskSLABusinessElapsedPercent: matches a case with at least one SLA row
	// whose business_elapsed_percentage is within the bound(s). Both bounds
	// apply to the SAME row (a range like 75..100 means one SLA in that range,
	// not one row >= 75 and another <= 100), which is why this is a single
	// EXISTS. Any SLA row counts, whatever its stage -- the contract of
	// domain.TaskSLAFilter. The percentage is uncapped (long-overdue SLAs
	// climb far past 100), and 0 is a real bound, hence the nil checks.
	if f := req.Parsed.TaskSLAFilter; f != nil && (f.MinBusinessElapsedPercent != nil || f.MaxBusinessElapsedPercent != nil) {
		slaWhere := "tsla.work_item_id = wi.id"
		if f.MinBusinessElapsedPercent != nil {
			slaWhere += fmt.Sprintf(" AND tsla.business_elapsed_percentage >= $%d::numeric", argIdx)
			filterArgs = append(filterArgs, *f.MinBusinessElapsedPercent)
			argIdx++
		}
		if f.MaxBusinessElapsedPercent != nil {
			slaWhere += fmt.Sprintf(" AND tsla.business_elapsed_percentage <= $%d::numeric", argIdx)
			filterArgs = append(filterArgs, *f.MaxBusinessElapsedPercent)
			argIdx++
		}
		where += " AND EXISTS (SELECT 1 FROM sla tsla WHERE " + slaWhere + ")"
	}

	// product: matched on the deployed product's own catalog name (prod is
	// the LEFT JOIN below) -- exact match against the same prod.name value
	// SearchCases already selects into each row's ProductName.
	if len(req.Parsed.ProductNames) > 0 {
		where += fmt.Sprintf(" AND prod.name = ANY($%d::text[])", argIdx)
		filterArgs = append(filterArgs, req.Parsed.ProductNames)
		argIdx++
	}

	// creTeam/sreTeam: the parent account's CRE/SRE-owning "group" (a and
	// cre/sre are the LEFT JOINs below) -- the same account.cre_team_id/
	// sre_team_id -> "group" path GetCaseByID already resolves for its own
	// CreTeam/SreTeam fields.
	if len(req.Parsed.CreTeamIDs) > 0 {
		where += fmt.Sprintf(" AND cre.id = ANY($%d::uuid[])", argIdx)
		filterArgs = append(filterArgs, req.Parsed.CreTeamIDs)
		argIdx++
	}
	if len(req.Parsed.SreTeamIDs) > 0 {
		where += fmt.Sprintf(" AND sre.id = ANY($%d::uuid[])", argIdx)
		filterArgs = append(filterArgs, req.Parsed.SreTeamIDs)
		argIdx++
	}

	// escalation (isEmpty / isNotEmpty): whether the case itself carries an active
	// escalation, matched on "case".is_escalated -- the flag the case detail
	// exposes as isEscalated. A row with no "case" row (a non-case work item) has
	// no escalation, so it satisfies isEmpty.
	if req.Parsed.HasActiveEscalation != nil {
		if *req.Parsed.HasActiveEscalation {
			where += " AND c.is_escalated IS TRUE"
		} else {
			where += " AND c.is_escalated IS NOT TRUE"
		}
	}

	// anyOf: each branch is the AND of its own fields, the branches are OR'd, and
	// the whole is ANDed with everything above. A branch with no fields would
	// match everything, so it is rendered TRUE rather than dropped.
	if len(req.Parsed.OrGroups) > 0 {
		branches := make([]string, 0, len(req.Parsed.OrGroups))
		for _, g := range req.Parsed.OrGroups {
			preds, branchArgs, next, err := caseFieldPredicates(caseFieldSetFromGroup(g), argIdx)
			if err != nil {
				return "", nil, argIdx, err
			}
			filterArgs = append(filterArgs, branchArgs...)
			argIdx = next
			if len(preds) == 0 {
				branches = append(branches, "TRUE")
				continue
			}
			branches = append(branches, "("+strings.Join(preds, " AND ")+")")
		}
		where += " AND (" + strings.Join(branches, " OR ") + ")"
	}

	if req.Filters.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(req.Filters.SearchQuery)
		pattern := "%" + escaped + "%"
		where += fmt.Sprintf(` AND (wi.subject ILIKE $%d ESCAPE '\' OR wi.number ILIKE $%d ESCAPE '\' OR wi.wso2_id ILIKE $%d ESCAPE '\')`, argIdx, argIdx, argIdx)
		filterArgs = append(filterArgs, pattern)
		argIdx++
	}

	return where, filterArgs, argIdx, nil
}

// SearchCases implements CaseRepository.
func (r *caseRepo) SearchCases(ctx context.Context, req domain.SearchCasesRequest, scope SearchScope) ([]domain.SearchCaseView, int, error) {
	// WithCallerIdentity from the explicit scope parameter -- see
	// GetCaseByID's own identical stamp for why this doesn't rely on
	// whatever identity ctx already carries.
	ctx = WithCallerIdentity(ctx, scope)
	where, filterArgs, argIdx, err := buildCaseSearchWhere(req, scope)
	if err != nil {
		return nil, 0, err
	}

	sortCol := pgSortColMap[req.SortBy.Field]
	sortDir := string(req.SortBy.Order)

	// Deployment/deployed-product/product are LEFT joins here (unlike
	// GetCaseByID's INNER joins): SearchCases can return non-case work_item
	// types too (service_request, engagement, security_report_analysis),
	// and "announcement" rows have no deployment/deployed-product at all
	// (see validateCreateCaseRequest) -- an INNER join would silently drop
	// them from every search result.
	joins := caseSearchJoins

	countQuery := "SELECT COUNT(*) FROM work_item wi " + joins + " " + where

	// The page is chosen first, by an inner query that selects only wi.id,
	// and the display columns are joined onto just those rows afterwards.
	// Every join in caseSearchJoins is on a primary key, so the inner query
	// returns exactly the rows the single-level form did, in the same order
	// (same ORDER BY, wi.id tie-break), while Postgres drops the joins the
	// WHERE and ORDER BY do not reference. Under RLS this matters: the
	// policies on the joined tables stop the planner from limiting rows
	// before joining, so a deep page used to join every matching row first.
	dataQuery := fmt.Sprintf(
		`SELECT wi.id, wi.number, wi.wso2_id,
		        wi.type::TEXT, wi.subject, wi.description, c.severity::TEXT, c.issue_type::TEXT, `+caseLikeStateColumn+`,
		        eng.type::TEXT, `+caseLikeWorkStateColumn+`, c.current_escalation_level::TEXT, wi.created_on, wi.updated_on,
		        wi.created_by,
		        p.id, p.name,
		        d.id, d.name,
		        dp.id, prod.name || COALESCE(' ' || pv.version, ''),
		        prod.id, prod.name,
		        ae.id, COALESCE(ae.name, NULLIF(TRIM(CONCAT_WS(' ', ae.first_name, ae.last_name)), '')), ae.email,
		        pw.id, pw.number,
		        rc_wi.id, rc_wi.number
		 FROM (SELECT wi.id FROM work_item wi %s %s
		       ORDER BY %s %s NULLS LAST, wi.id
		       LIMIT $%d OFFSET $%d) page
		 JOIN work_item wi ON wi.id = page.id %s
		 ORDER BY %s %s NULLS LAST, wi.id`,
		joins, where, sortCol, sortDir, argIdx, argIdx+1,
		joins, sortCol, sortDir,
	)
	dataArgs := append(append([]any{}, filterArgs...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var cases []domain.SearchCaseView

	eg, egCtx := errgroup.WithContext(ctx)

	// COUNT and SELECT each go through Scoped independently (rather than
	// sharing one transaction) specifically so they can still run
	// concurrently on separate pool connections, same as before this change
	// -- a pgx.Tx is bound to a single connection, so one shared transaction
	// across both goroutines would have serialized them. Scoped.QueryRow/
	// Query each set the caller's identity (read from egCtx, stamped above)
	// as their own implicit one-statement transaction, so there is no
	// explicit tx/setCallerIdentity call needed here any more.
	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, filterArgs...).Scan(&total); err != nil {
			return fmt.Errorf("count cases: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query cases: %w", err)
		}
		defer rows.Close()

		result := make([]domain.SearchCaseView, 0, req.Pagination.Limit)
		for rows.Next() {
			var cv domain.SearchCaseView
			var internalID *string
			var caseType, subject string
			var description *string
			var severity, issueType, engagementType, workState, state, escalationLevel *string
			var createdAt, updatedAt time.Time
			var aeID, aeName, aeEmail *string
			var pcID, pcNumber *string
			var rcID, rcNumber *string
			var prodID, prodName *string
			var projID, projName *string
			var depID, depName *string
			var dpID, dpName *string
			var creatorEmail string
			if err := rows.Scan(
				&cv.ID, &cv.Number, &internalID,
				&caseType, &subject, &description, &severity, &issueType, &state,
				&engagementType, &workState, &escalationLevel, &createdAt, &updatedAt,
				&creatorEmail,
				&projID, &projName,
				&depID, &depName,
				&dpID, &dpName,
				&prodID, &prodName,
				&aeID, &aeName, &aeEmail,
				&pcID, &pcNumber,
				&rcID, &rcNumber,
			); err != nil {
				return fmt.Errorf("scan case: %w", err)
			}
			cv.InternalID = stringOrEmpty(internalID)
			if projID != nil {
				cv.Project = &domain.EntityRef{ID: *projID, Name: stringOrEmpty(projName)}
			}
			if depID != nil {
				cv.Deployment = &domain.EntityRef{ID: *depID, Name: stringOrEmpty(depName)}
			}
			if dpID != nil {
				cv.DeployedProduct = &domain.EntityRef{ID: *dpID, Name: stringOrEmpty(dpName)}
			}
			cv.Type = strings.ToLower(caseType)
			cv.Subject = &subject
			cv.Description = description
			// case_severity_enum/case_issue_type_enum/case_work_state_enum/
			// engagement_type_enum are all UPPER_SNAKE_CASE (case_severity_enum
			// is 'S0'..'S4', an entirely different label set -- see
			// caseSeverityToEnum's own comment); the domain values these
			// response fields carry are lowercase.
			if severity != nil {
				mapped := string(caseSeverityFromEnum[*severity])
				cv.Severity = &mapped
			}
			if issueType != nil {
				lower := strings.ToLower(*issueType)
				cv.IssueType = &lower
			}
			if engagementType != nil {
				lower := strings.ToLower(*engagementType)
				cv.EngagementType = &lower
			}
			if workState != nil {
				lower := strings.ToLower(*workState)
				cv.WorkState = &lower
			}
			if state != nil {
				lower := strings.ToLower(*state)
				cv.State = &lower
			}
			if escalationLevel != nil {
				el := caseEscalationLevelFromEnum(*escalationLevel)
				cv.EscalationLevel = &el
			}
			cv.CreatedOn = createdAt.UTC().Format(time.RFC3339)
			cv.UpdatedOn = updatedAt.UTC().Format(time.RFC3339)
			if prodID != nil {
				cv.Product = &domain.EntityRef{ID: *prodID, Name: stringOrEmpty(prodName)}
			}
			// The search projection carries only the creator's email, no id or
			// display name, so the canonical reference keeps a null id.
			cv.CreatedBy = domain.NewUserReference("", creatorEmail, "")
			if aeID != nil {
				cv.AssignedEngineer = domain.NewUserReference(*aeID, stringOrEmpty(aeEmail), stringOrEmpty(aeName))
			}
			if pcID != nil {
				cv.ParentCase = &domain.EntityRef{ID: *pcID, Name: stringOrEmpty(pcNumber)}
			}
			if rcID != nil {
				cv.RelatedCase = &domain.EntityRef{ID: *rcID, Name: stringOrEmpty(rcNumber)}
			}
			result = append(result, cv)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate cases: %w", err)
		}
		cases = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return cases, total, nil
}

// rowsQuerier is satisfied by both *Scoped and pgx.Tx, letting
// fetchCaseWatchers run either directly against Scoped (GetCaseByID) or
// inside an existing transaction (SetCaseWatchList), without duplicating the
// query.
type rowsQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// fetchCaseWatchers reads the resolved watch list for the case (== work_item)
// identified by caseID, newest-added-last ordering is not available (see
// work_item_watcher's own migration comment: no per-row audit trail to sort
// by), so results are ordered by user_name for a stable, deterministic
// response instead.
func fetchCaseWatchers(ctx context.Context, q rowsQuerier, caseID string) ([]domain.WatchListUser, error) {
	// locked mirrors AccountDefaultWatcherEmails' own four stakeholder
	// columns (minus the email resolution), joined live rather than
	// cross-checked against a snapshot -- see
	// WatchListUser.Locked's own doc comment on why that's deliberate.
	// LEFT JOINs throughout so a case with no project, or a project with no
	// account, still returns every watcher with locked=false rather than
	// zero rows (an INNER JOIN here would silently drop every watcher on
	// such a case, the same class of false-empty-result bug this file's own
	// "Case-like work_item types" fixes already guard against elsewhere).
	rows, err := q.Query(ctx, `
		SELECT u.id, u.user_name, COALESCE(u.name, CONCAT_WS(' ', u.first_name, u.last_name)), u.email,
		       COALESCE(u.id = acct.technical_owner_id, false)
		           OR COALESCE(u.id = acct.secondary_technical_owner_id, false)
		           OR COALESCE(u.id = acct.account_manager_id, false)
		           OR COALESCE(u.id = acct.renewal_account_manager_id, false) AS locked
		FROM work_item_watcher w
		JOIN "user" u ON u.id = w.user_id
		LEFT JOIN work_item wi ON wi.id = w.work_item_id
		LEFT JOIN project p ON p.id = wi.project_id
		LEFT JOIN account acct ON acct.id = p.account_id
		WHERE w.work_item_id = $1
		ORDER BY u.user_name`, caseID)
	if err != nil {
		return nil, fmt.Errorf("query case watch list: %w", err)
	}
	defer rows.Close()

	var watchers []domain.WatchListUser
	for rows.Next() {
		var id, userName, name string
		var email *string
		var locked bool
		if err := rows.Scan(&id, &userName, &name, &email, &locked); err != nil {
			return nil, fmt.Errorf("scan case watcher: %w", err)
		}
		watchers = append(watchers, domain.WatchListUser{
			ID:       id,
			UserName: userName,
			Name:     name,
			Email:    stringOrEmpty(email),
			Locked:   locked,
			// User.ID is always null by contract -- see WatchListUser.User's
			// own doc comment ("its id is always null: a watch-list entry is
			// not guaranteed to point at a user record"). Pass "" rather
			// than id so NewUserReference omits it, even though this
			// particular row is known to resolve to a real user.
			User: domain.NewUserReference("", stringOrEmpty(email), name),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate case watch list: %w", err)
	}
	return watchers, nil
}

// SetCaseWatchList implements CaseRepository.
func (r *caseRepo) SetCaseWatchList(ctx context.Context, caseID string, userIDs []string, callerEmail string) ([]domain.WatchListUser, time.Time, error) {
	var updatedOn time.Time
	var watchers []domain.WatchListUser
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `UPDATE work_item SET updated_on = NOW(), updated_by = $2 WHERE id = $1 RETURNING updated_on`, caseID, callerEmail).Scan(&updatedOn); err != nil {
			return fmt.Errorf("touch work_item for watch list update: %w", err)
		}

		if _, err := tx.Exec(ctx, `DELETE FROM work_item_watcher WHERE work_item_id = $1`, caseID); err != nil {
			return fmt.Errorf("clear case watch list: %w", err)
		}

		for _, userID := range userIDs {
			if _, err := tx.Exec(ctx,
				`INSERT INTO work_item_watcher (id, work_item_id, user_id) VALUES (gen_random_uuid(), $1, $2)`,
				caseID, userID,
			); err != nil {
				if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
					return &apierror.ValidationError{Msg: "one or more watch list user IDs do not exist: " + pgErr.Detail}
				}
				return fmt.Errorf("insert case watcher: %w", err)
			}
		}

		var txErr error
		watchers, txErr = fetchCaseWatchers(ctx, tx, caseID)
		return txErr
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, time.Time{}, &apierror.NotFoundError{Msg: "case not found"}
	}
	if err != nil {
		return nil, time.Time{}, err
	}
	return watchers, updatedOn, nil
}

// AccountDefaultWatcherEmails implements CaseRepository.
func (r *caseRepo) AccountDefaultWatcherEmails(ctx context.Context, projectID string) ([]string, error) {
	var towEmail, stowEmail, amEmail, ramEmail *string
	err := r.db.QueryRow(ctx, `
		SELECT tow.email, stow.email, am.email, ram.email
		FROM project p
		JOIN account a ON a.id = p.account_id
		LEFT JOIN "user" tow ON tow.id = a.technical_owner_id
		LEFT JOIN "user" stow ON stow.id = a.secondary_technical_owner_id
		LEFT JOIN "user" am ON am.id = a.account_manager_id
		LEFT JOIN "user" ram ON ram.id = a.renewal_account_manager_id
		WHERE p.id = $1`, projectID,
	).Scan(&towEmail, &stowEmail, &amEmail, &ramEmail)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("account default watcher emails: %w", err)
	}

	emails := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	for _, email := range []*string{towEmail, stowEmail, amEmail, ramEmail} {
		if email == nil || *email == "" {
			continue
		}
		if _, dup := seen[*email]; dup {
			continue
		}
		seen[*email] = struct{}{}
		emails = append(emails, *email)
	}
	return emails, nil
}

// GetCaseEtaSharedOn implements CaseRepository.
func (r *caseRepo) GetCaseEtaSharedOn(ctx context.Context, caseID string) (*time.Time, error) {
	var etaSharedOn *time.Time
	err := r.db.QueryRow(ctx, `SELECT eta_shared_on FROM work_item WHERE id = $1`, caseID).Scan(&etaSharedOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get case eta shared on: %w", err)
	}
	return etaSharedOn, nil
}

// ProjectContactEmailsByRole implements CaseRepository.
func (r *caseRepo) ProjectContactEmailsByRole(ctx context.Context, projectID, role string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT pc.email
		FROM project_contact pc
		JOIN project_contact_group pcg ON pcg.project_contact_id = pc.id
		JOIN project_group_role pgr ON pgr.project_group_id = pcg.project_group_id
		JOIN project_role pr ON pr.id = pgr.project_role_id
		WHERE pc.project_id = $1
		  AND pr.role = $2::project_role_enum
		  AND (pc.state IS NULL OR pc.state <> 'DEACTIVATED'::project_contact_state_enum)`,
		projectID, role,
	)
	if err != nil {
		return nil, fmt.Errorf("project contact emails by role: %w", err)
	}
	defer rows.Close()

	var emails []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, fmt.Errorf("project contact emails by role: scan: %w", err)
		}
		emails = append(emails, email)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("project contact emails by role: %w", err)
	}
	return emails, nil
}

// updateCaseAssigneeQuery atomically applies the no-op check inside the
// UPDATE itself (WHERE assigned_to_id IS DISTINCT FROM $2) rather than a
// separate pre-write read -- two concurrent calls assigning the same case to
// the same engineer can't both see "unchanged" and neither can both see
// "changed", since only one of them can actually win the WHERE clause. The
// outer SELECT reads work_item AFTER the CTE runs (same "modify, then report
// the post-write row" idiom updateCaseQuery/acknowledgeCaseQuery already use
// in this file), so it reports the just-written updated_on on a genuine
// change and the pre-existing one on a no-op.
const updateCaseAssigneeQuery = `
	WITH updated AS (
		UPDATE work_item
		SET assigned_to_id = $2, updated_on = NOW(), updated_by = $3
		WHERE id = $1 AND assigned_to_id IS DISTINCT FROM $2
		RETURNING id
	)
	SELECT wi.updated_on, (updated.id IS NOT NULL) AS changed
	FROM work_item wi
	LEFT JOIN updated ON updated.id = wi.id
	WHERE wi.id = $1`

// UpdateCaseAssignee implements CaseRepository.
func (r *caseRepo) UpdateCaseAssignee(ctx context.Context, caseID string, userID *string, callerEmail string) (time.Time, bool, error) {
	var updatedOn time.Time
	var changed bool
	err := r.db.QueryRow(ctx, updateCaseAssigneeQuery, caseID, userID, callerEmail).Scan(&updatedOn, &changed)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, &apierror.NotFoundError{Msg: "case not found"}
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("update case assignee: %w", err)
	}
	return updatedOn, changed, nil
}

// MarkCaseFixIssued implements CaseRepository.
//
// Writes work_item.fix_issued_on, not a new column -- this column already
// exists in the base schema (migration 0021_work_item_table) and is the
// csm-sync-service mapping's designated target for ServiceNow's
// u_fix_issued (configs/mappings/sn_customerservice_case.yaml), confirmed
// against that file directly. A separate "case".fix_issued column was
// briefly added and then dropped once this was found -- see lesson 124.
func (r *caseRepo) MarkCaseFixIssued(ctx context.Context, caseID string) (time.Time, bool, error) {
	var fixIssued time.Time
	err := r.db.QueryRow(ctx,
		`UPDATE work_item SET fix_issued_on = NOW() WHERE id = $1 AND fix_issued_on IS NULL RETURNING fix_issued_on`,
		caseID,
	).Scan(&fixIssued)
	if err == nil {
		return fixIssued, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, fmt.Errorf("mark case fix issued: %w", err)
	}

	// No row updated: either the case doesn't exist, or fix_issued_on was
	// already set (first-write-wins no-op) -- distinguish the two with a
	// follow-up read.
	err = r.db.QueryRow(ctx, `SELECT fix_issued_on FROM work_item WHERE id = $1`, caseID).Scan(&fixIssued)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, &apierror.NotFoundError{Msg: "case not found"}
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("mark case fix issued: read existing: %w", err)
	}
	return fixIssued, true, nil
}

// txQuerier is the QueryRow+Exec subset both pgx.Tx and *Scoped satisfy --
// lets recomputeTimeCardsBillable/addCaseTagTx's own patch-tag check run
// against either a transaction already in progress or (in principle) the
// plain pool, without duplicating the SQL.
type txQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// caseHasPatchTag reports whether caseID currently carries a tag named
// "patch" (case/whitespace-insensitive -- tag.name has no normalization of
// its own), via q so the check can run inside an already-open transaction
// that has the case row locked.
func caseHasPatchTag(ctx context.Context, q txQuerier, caseID string) (bool, error) {
	var hasPatch bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM work_item_tag wit
			JOIN tag t ON t.id = wit.tag_id
			WHERE wit.work_item_id = $1 AND LOWER(TRIM(t.name)) = 'patch'
		)`, caseID).Scan(&hasPatch)
	if err != nil {
		return false, fmt.Errorf("check patch tag: %w", err)
	}
	return hasPatch, nil
}

// recomputeTimeCardsBillable sets every time_card row under caseID to
// isLow && !hasPatchTag -- "entering LOW/S4 severity makes time cards
// billable, unless a 'patch' tag overrides it back to non-billable" (WSO2
// still covers a patch under support even on an otherwise best-efforts S4
// case) -- in one UPDATE, via q. Called from inside the SAME transaction
// that already holds a `SELECT ... FOR UPDATE` lock on the case row
// (UpdateCase's severity branch, addCaseTagTx's "patch" branch below), so
// two concurrent writers that could otherwise race on this (a severity
// change and a tag add, or two overlapping severity changes) are
// serialized by Postgres's own row lock instead: whichever transaction
// commits last is also the one whose fresh-within-that-transaction read of
// severity/tags determines the final state, so an older transition can
// never land after a newer one already did. Errors are returned to the
// caller to log (best-effort, never meant to abort the transaction this
// runs inside -- see each call site's own handling).
func recomputeTimeCardsBillable(ctx context.Context, q txQuerier, caseID string, isLow bool) (int64, error) {
	isBillable := false
	if isLow {
		hasPatch, err := caseHasPatchTag(ctx, q, caseID)
		if err != nil {
			return 0, err
		}
		isBillable = !hasPatch
	}
	tag, err := q.Exec(ctx, `UPDATE time_card SET is_billable = $1 WHERE case_id = $2`, isBillable, caseID)
	if err != nil {
		return 0, fmt.Errorf("set time cards billable for case: %w", err)
	}
	return tag.RowsAffected(), nil
}

// UpdateCaseParent implements CaseRepository.
func (r *caseRepo) UpdateCaseParent(ctx context.Context, caseID, parentID, callerEmail string) (time.Time, error) {
	var updatedOn time.Time
	err := r.db.QueryRow(ctx,
		`UPDATE work_item SET parent_id = $2::uuid, updated_on = NOW(), updated_by = $3 WHERE id = $1 RETURNING updated_on`,
		caseID, parentID, callerEmail,
	).Scan(&updatedOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, &apierror.NotFoundError{Msg: "case not found"}
	}
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return time.Time{}, &apierror.ValidationError{Msg: "parentId does not exist: " + pgErr.Detail}
		}
		return time.Time{}, fmt.Errorf("update case parent: %w", err)
	}
	return updatedOn, nil
}

// RecordCaseFieldChangeActivity implements CaseRepository.
func (r *caseRepo) RecordCaseFieldChangeActivity(ctx context.Context, caseID, fieldName, oldValue, newValue, actorEmail string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO work_item_activity (id, created_on, created_by, work_item_id, field_name, old_value, new_value, user_email)
		 VALUES (gen_random_uuid(), NOW(), $1, $2, $3, $4, $5, $1)`,
		actorEmail, caseID, fieldName, oldValue, newValue,
	)
	if err != nil {
		return fmt.Errorf("record case field change activity: %w", err)
	}
	return nil
}

// AcknowledgeCase implements CaseRepository. The claim itself
// (work_item.acknowledged_by_user_id IS NULL) and the read of whoever ends up
// holding it run in one transaction with a row lock, so two concurrent
// Acknowledge calls on the same case can't both believe they were first.
func (r *caseRepo) AcknowledgeCase(ctx context.Context, caseID, actorID, actorEmail string) (bool, domain.AssignedEngineerRef, string, time.Time, error) {
	var alreadyAcknowledged bool
	var number string
	var updatedOn time.Time
	var ackID, ackName, ackEmail *string
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		var existingAckID *string
		if err := tx.QueryRow(ctx, `SELECT acknowledged_by_user_id FROM work_item WHERE id = $1 FOR UPDATE`, caseID).Scan(&existingAckID); err != nil {
			return fmt.Errorf("acknowledge case: lock work_item: %w", err)
		}

		alreadyAcknowledged = existingAckID != nil
		if !alreadyAcknowledged {
			if _, err := tx.Exec(ctx,
				`UPDATE work_item SET acknowledged_by_user_id = $2, updated_on = NOW(), updated_by = $3 WHERE id = $1`,
				caseID, actorID, actorEmail,
			); err != nil {
				return fmt.Errorf("acknowledge case: claim: %w", err)
			}
		}

		// Read back from the row regardless of which branch above ran, so the
		// acknowledger's name/email always come from the same "user" join --
		// whoever holds the claim now, not necessarily this call's actor.
		if err := tx.QueryRow(ctx, `
			SELECT wi.number, wi.updated_on, u.id, COALESCE(u.name, NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), '')), u.email
			FROM work_item wi
			LEFT JOIN "user" u ON u.id = wi.acknowledged_by_user_id
			WHERE wi.id = $1`, caseID,
		).Scan(&number, &updatedOn, &ackID, &ackName, &ackEmail); err != nil {
			return fmt.Errorf("acknowledge case: read back: %w", err)
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, domain.AssignedEngineerRef{}, "", time.Time{}, &apierror.NotFoundError{Msg: "case not found"}
	}
	if err != nil {
		return false, domain.AssignedEngineerRef{}, "", time.Time{}, err
	}

	ackBy := domain.AssignedEngineerRef{ID: stringOrEmpty(ackID), Name: stringOrEmpty(ackName), Email: ackEmail}
	return alreadyAcknowledged, ackBy, number, updatedOn, nil
}

// UpdateCaseFields implements CaseRepository. "case" is updated first (if it
// has any columns to touch), so a nonexistent id or a related_case_id
// foreign-key violation is caught before work_item's own row is touched at
// all -- if req names no "case" column, work_item's own UPDATE...WHERE
// alone still correctly reports not-found via zero rows.
func (r *caseRepo) UpdateCaseFields(ctx context.Context, req domain.UpdateCaseRequest, actorID, actorEmail string) (time.Time, error) {
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (time.Time, error) {
		return updateCaseFieldsTx(ctx, tx, req, actorID, actorEmail)
	})
}

// updateCaseFieldsTx is UpdateCaseFields' body, extracted so it can run
// inside r.db.InTx's closure.
func updateCaseFieldsTx(ctx context.Context, tx pgx.Tx, req domain.UpdateCaseRequest, actorID, actorEmail string) (time.Time, error) {
	// resolutionCode/cause/closeNotes are deliberately NOT handled here, even
	// though they also live on "case" -- sn_case_service.go's own UpdateCase
	// only allows them alongside state (and only when transitioning to
	// closed/solution_proposed), so they're written by CaseRepository.
	// UpdateCase's own "case" branch, not this generic field bundle. issueType
	// is ALSO not handled here for the mirror-image reason: on that same SN
	// contract it's accepted only as part of a type transfer ("type" itself
	// has no Postgres implementation), never as a standalone field.
	var caseSets []string
	caseArgs := []any{req.ID}
	idx := 2
	if req.RelatedCaseID != nil {
		caseSets = append(caseSets, fmt.Sprintf("related_case_id = $%d::uuid", idx))
		caseArgs = append(caseArgs, *req.RelatedCaseID)
		idx++
	}
	if len(caseSets) > 0 {
		tag, err := tx.Exec(ctx, `UPDATE "case" SET `+strings.Join(caseSets, ", ")+` WHERE id = $1`, caseArgs...)
		if err != nil {
			if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
				return time.Time{}, &apierror.ValidationError{Msg: "relatedCaseId does not exist: " + pgErr.Detail}
			}
			return time.Time{}, fmt.Errorf("update case fields: case: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return time.Time{}, &apierror.NotFoundError{Msg: "case not found"}
		}
	}

	// work_item.updated_on/updated_by are bumped unconditionally, matching
	// every sibling UpdateCase branch, even when only "case" columns above
	// changed.
	wiSets := []string{"updated_on = NOW()", "updated_by = $2"}
	wiArgs := []any{req.ID, actorEmail}
	widx := 3
	if req.Subject != nil {
		wiSets = append(wiSets, fmt.Sprintf("subject = $%d", widx))
		wiArgs = append(wiArgs, *req.Subject)
		widx++
	}
	if req.Description != nil {
		wiSets = append(wiSets, fmt.Sprintf("description = $%d", widx))
		wiArgs = append(wiArgs, *req.Description)
		widx++
	}
	if req.DeploymentID != nil {
		wiSets = append(wiSets, fmt.Sprintf("deployment_id = $%d::uuid", widx))
		wiArgs = append(wiArgs, *req.DeploymentID)
		widx++
	}
	if req.DeployedProductID != nil {
		wiSets = append(wiSets, fmt.Sprintf("deployed_product_id = $%d::uuid", widx))
		wiArgs = append(wiArgs, *req.DeployedProductID)
		widx++
	}
	if req.BestCaseFixEta != nil {
		wiSets = append(wiSets, fmt.Sprintf("best_case_eta = $%d::date", widx))
		wiArgs = append(wiArgs, *req.BestCaseFixEta)
		widx++
	}
	if req.MostLikelyFixEta != nil {
		wiSets = append(wiSets, fmt.Sprintf("most_likely_eta = $%d::date", widx))
		wiArgs = append(wiArgs, *req.MostLikelyFixEta)
		widx++
	}
	if req.WorstCaseFixEta != nil {
		wiSets = append(wiSets, fmt.Sprintf("worst_case_eta = $%d::date", widx))
		wiArgs = append(wiArgs, *req.WorstCaseFixEta)
		widx++
	}
	if req.WorkaroundProvided != nil {
		if *req.WorkaroundProvided {
			wiSets = append(wiSets, fmt.Sprintf("workaround_provided_on = NOW(), workaround_provided_by_user_id = $%d::uuid", widx))
			wiArgs = append(wiArgs, actorID)
			widx++
		} else {
			// Recalling the workaround clears both fields -- see
			// domain.UpdateCaseRequest.WorkaroundProvided's own doc comment:
			// unlike Acknowledge, false is a meaningful, accepted value here.
			wiSets = append(wiSets, "workaround_provided_on = NULL, workaround_provided_by_user_id = NULL")
		}
	}

	var updatedOn time.Time
	err := tx.QueryRow(ctx, `UPDATE work_item SET `+strings.Join(wiSets, ", ")+` WHERE id = $1 RETURNING updated_on`, wiArgs...).Scan(&updatedOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, &apierror.NotFoundError{Msg: "case not found"}
	}
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return time.Time{}, &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
		}
		return time.Time{}, fmt.Errorf("update case fields: work_item: %w", err)
	}

	return updatedOn, nil
}

// scanTag scans a single (id, name) row into a domain.Tag. tag has no
// "color" column (migration 0026), unlike ServiceNow's label table, so
// Color is always nil for a Postgres-sourced tag.
func scanTag(row interface{ Scan(...any) error }) (domain.Tag, error) {
	var t domain.Tag
	err := row.Scan(&t.ID, &t.Label)
	return t, err
}

// AddCaseTag implements CaseRepository.
func (r *caseRepo) AddCaseTag(ctx context.Context, caseID, label, callerEmail string) (domain.Tag, error) {
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (domain.Tag, error) {
		return addCaseTagTx(ctx, tx, caseID, label, callerEmail)
	})
}

// addCaseTagTx is AddCaseTag's body, extracted so it can run inside
// r.db.InTx's closure.
func addCaseTagTx(ctx context.Context, tx pgx.Tx, caseID, label, callerEmail string) (domain.Tag, error) {
	// Find or create the tag by name, case-insensitively. tag.name has no
	// UNIQUE constraint (migration 0026), so this can race with a
	// concurrent AddCaseTag for the same never-before-seen label and
	// produce two rows with the same name -- a cosmetic duplicate (each
	// still links correctly via its own id), not a correctness bug, and not
	// fixable here without a schema change (out of scope).
	tag, err := scanTag(tx.QueryRow(ctx, `SELECT id, name FROM tag WHERE LOWER(name) = LOWER($1) LIMIT 1`, label))
	if errors.Is(err, pgx.ErrNoRows) {
		tag, err = scanTag(tx.QueryRow(ctx,
			`INSERT INTO tag (id, created_on, updated_on, created_by, updated_by, name)
			 VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, $2)
			 RETURNING id, name`, callerEmail, label))
	}
	if err != nil {
		return domain.Tag{}, fmt.Errorf("find or create tag: %w", err)
	}

	// Idempotent attach: a second AddCaseTag for a label already on this
	// case returns the existing tag rather than erroring or duplicating the
	// work_item_tag row. work_item_tag has no UNIQUE constraint on
	// (work_item_id, tag_id) (migration 0026), so this is guarded with
	// "AND NOT EXISTS" rather than "ON CONFLICT DO NOTHING", which would
	// need one to match against.
	_, err = tx.Exec(ctx, `
		INSERT INTO work_item_tag (id, created_on, updated_on, created_by, updated_by, work_item_id, tag_id)
		SELECT gen_random_uuid(), NOW(), NOW(), $1, $1, wi.id, $3
		FROM work_item wi
		WHERE wi.id = $2
		  AND NOT EXISTS (SELECT 1 FROM work_item_tag wit WHERE wit.work_item_id = $2 AND wit.tag_id = $3)`,
		callerEmail, caseID, tag.ID)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return domain.Tag{}, &apierror.NotFoundError{Msg: "case not found"}
		}
		return domain.Tag{}, fmt.Errorf("attach tag to case: %w", err)
	}

	// The INSERT ... SELECT above silently inserts zero rows (rather than
	// erroring) when caseID doesn't reference an existing work_item, since
	// the FROM work_item WHERE wi.id = $2 clause just matches nothing. Check
	// caseID separately so that case is reported as a ValidationError
	// instead of a misleadingly successful response.
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM work_item WHERE id = $1)`, caseID).Scan(&exists); err != nil {
		return domain.Tag{}, fmt.Errorf("verify case exists: %w", err)
	}
	if !exists {
		return domain.Tag{}, &apierror.NotFoundError{Msg: "case not found"}
	}

	// A case tagged "patch" while at LOW/S4 severity should have its time
	// cards non-billable regardless of the normal "entering LOW makes time
	// cards billable" rule -- WSO2 still covers a patch under support even
	// for an otherwise best-efforts S4 case. Locks the same case row
	// UpdateCase's own severity branch locks (SELECT ... FOR UPDATE on
	// "case"), so a concurrent severity change is serialized against this
	// tag add rather than racing it -- see recomputeTimeCardsBillable's own
	// doc comment. Only acts when the case is currently LOW: adding "patch"
	// at any other severity does nothing immediately, and is picked up the
	// next time the case's severity actually crosses into LOW (that
	// transition's own recompute checks for this tag fresh, every time).
	// Best-effort: logged, never allowed to fail the tag attach that
	// already succeeded above.
	if strings.EqualFold(strings.TrimSpace(label), "patch") {
		var severityRaw *string
		err := tx.QueryRow(ctx, `SELECT severity::TEXT FROM "case" WHERE id = $1 FOR UPDATE`, caseID).Scan(&severityRaw)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			slog.ErrorContext(ctx, "add case tag: lock case for patch billable override failed", "caseId", caseID, "error", err)
		} else if severityRaw != nil && caseSeverityFromEnum[*severityRaw] == domain.CaseSeverityLow {
			if _, err := recomputeTimeCardsBillable(ctx, tx, caseID, true); err != nil {
				slog.ErrorContext(ctx, "add case tag: recompute time cards billable failed", "caseId", caseID, "error", err)
			}
		}
	}

	return tag, nil
}

// RemoveCaseTag implements CaseRepository.
func (r *caseRepo) RemoveCaseTag(ctx context.Context, caseID, tagID, _ string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM work_item_tag WHERE work_item_id = $1 AND tag_id = $2`, caseID, tagID)
	if err != nil {
		return fmt.Errorf("remove case tag: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &apierror.NotFoundError{Msg: "tag not found on this case"}
	}
	return nil
}

// SetCaseTagSNSysID implements CaseRepository. Returns *apierror.NotFoundError
// if the (caseID, tagID) attachment no longer exists -- e.g. RemoveCaseTag
// deleted it concurrently, in the gap between AddCaseTag's mirror creating
// the ServiceNow tag and this call persisting its sys_id back -- so the
// AddCaseTag writeback callback can react by cleaning up the now-orphaned
// ServiceNow tag instead of silently discarding its id.
func (r *caseRepo) SetCaseTagSNSysID(ctx context.Context, caseID, tagID, snSysID string) error {
	result, err := r.db.Exec(ctx,
		`UPDATE work_item_tag SET sn_sys_id = $1 WHERE work_item_id = $2 AND tag_id = $3`,
		snSysID, caseID, tagID)
	if err != nil {
		return fmt.Errorf("set case tag sn sys id: %w", err)
	}
	if result.RowsAffected() == 0 {
		return &apierror.NotFoundError{Msg: "tag not found on this case"}
	}
	return nil
}

// GetCaseTagSNSysID implements CaseRepository.
func (r *caseRepo) GetCaseTagSNSysID(ctx context.Context, caseID, tagID string) (*string, error) {
	var snSysID *string
	err := r.db.QueryRow(ctx,
		`SELECT sn_sys_id FROM work_item_tag WHERE work_item_id = $1 AND tag_id = $2`,
		caseID, tagID,
	).Scan(&snSysID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &apierror.NotFoundError{Msg: "tag not found on this case"}
	}
	if err != nil {
		return nil, fmt.Errorf("get case tag sn sys id: %w", err)
	}
	return snSysID, nil
}

// SearchTags implements CaseRepository.
func (r *caseRepo) SearchTags(ctx context.Context, searchQuery, _ string, limit int) ([]domain.Tag, error) {
	where := "WHERE 1=1"
	args := []any{}
	if searchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(searchQuery)
		args = append(args, "%"+escaped+"%")
		where += " AND name ILIKE $1 ESCAPE '\\'"
	}
	args = append(args, limit)

	query := fmt.Sprintf(`SELECT id, name FROM tag %s ORDER BY created_on DESC, id LIMIT $%d`, where, len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search tags: %w", err)
	}
	defer rows.Close()

	tags := make([]domain.Tag, 0, limit)
	for rows.Next() {
		t, err := scanTag(rows)
		if err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}
	return tags, nil
}

// caseActivityFieldChangeLabel renders work_item_activity.field_name (a raw
// column identifier, e.g. "assigned_to_id") as a human-readable label (e.g.
// "Assigned To Id") -- the same space-separated title case convention
// taskSlaStageDisplay (task_sla_repo.go) already uses for a raw enum label,
// applied here to a raw column name instead. No field-name -> display-label
// mapping exists anywhere else in this schema to defer to.
// caseActivityFieldChangeLabelOverrides holds field_name -> display label
// pairs where the generic space-separated-title-case rendering below reads
// badly: an "_id" suffix is natural in a column name but not in a label a
// person reads ("Assigned To Id"), and English keeps an assignment's own
// preposition lowercase ("Assigned to", not "Assigned To").
var caseActivityFieldChangeLabelOverrides = map[string]string{
	"assigned_to_id":          "Assigned to",
	"acknowledged_by_user_id": "Acknowledged by",
	"parent_id":               "Parent case",
}

func caseActivityFieldChangeLabel(fieldName string) string {
	if label, ok := caseActivityFieldChangeLabelOverrides[fieldName]; ok {
		return label
	}
	words := strings.Split(fieldName, "_")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// scanCaseActivity scans one row of SearchCaseActivities' dataQuery
// (id, kind, content, created_on, email, first_name, last_name, name,
// comment_type, file_name, content_type, size_bytes, field_name, old_value,
// new_value) into a domain.CaseActivity, dispatching on kind the same way
// the query's three UNION ALL branches are discriminated.
func scanCaseActivity(row interface{ Scan(...any) error }) (domain.CaseActivity, error) {
	var (
		id, kind, content                string
		createdOn                        time.Time
		email, firstName, lastName, name *string
		commentTypeRaw                   *string
		fileName, contentType            *string
		sizeBytes                        *int64
		fieldName, oldValue, newValue    *string
	)
	if err := row.Scan(&id, &kind, &content, &createdOn, &email, &firstName, &lastName, &name, &commentTypeRaw, &fileName, &contentType, &sizeBytes, &fieldName, &oldValue, &newValue); err != nil {
		return domain.CaseActivity{}, err
	}
	a := domain.CaseActivity{
		ID:                 id,
		Content:            content,
		CreatedOn:          createdOn,
		CreatedByFirstName: stringOrEmpty(firstName),
		CreatedByLastName:  stringOrEmpty(lastName),
	}
	// CreatedBy.ID is always null on this feed by contract -- see
	// CaseActivity's own doc comment. Name resolves the same way every
	// other read in this service does: "user".name first, falling back to
	// first_name+last_name only if name is unset.
	a.CreatedBy = domain.NewUserReference("", stringOrEmpty(email), stringOrEmpty(name))
	switch kind {
	case "comment":
		a.Type = domain.ActivityTypeComment
		if commentTypeRaw != nil {
			if ct, ok := caseCommentEnumType[*commentTypeRaw]; ok {
				a.CommentType = &ct
			}
		}
	case "attachment":
		a.Type = domain.ActivityTypeAttachment
		a.FileName = stringOrEmpty(fileName)
		a.ContentType = stringOrEmpty(contentType)
		if sizeBytes != nil {
			a.SizeBytes = int(*sizeBytes)
		}
		// DownloadURL is deliberately left empty: this service builds no
		// portal links or absolute URLs to itself (see CLAUDE.md's Event
		// Hub section for the same "no portal base URL" posture) -- a
		// caller resolves the actual bytes via GET /attachments/{id}/content.
	case "field_change":
		a.Type = domain.ActivityTypeFieldChange
		field := stringOrEmpty(fieldName)
		a.Changes = []domain.FieldChange{{
			Field:         field,
			FieldLabel:    caseActivityFieldChangeLabel(field),
			PreviousValue: stringOrEmpty(oldValue),
			NewValue:      stringOrEmpty(newValue),
		}}
	}
	return a, nil
}

// SearchCaseActivities implements CaseRepository.
//
// includeFieldChanges gates a third UNION ALL branch over work_item_activity
// (migration 0055) -- previously there was no field-change audit table in
// this schema at all, so SearchCaseActivitiesRequest.IncludeFieldChanges had
// no effect. Each work_item_activity row is one single field mutation (no
// grouping key -- e.g. a shared timestamp -- is confirmed to bundle several
// simultaneous field changes into one activity entry the way a ServiceNow
// journal entry might), so each row becomes its own CaseActivity with a
// single-element Changes slice, rather than guessing at a bundling rule.
func (r *caseRepo) SearchCaseActivities(ctx context.Context, req domain.SearchCaseActivitiesRequest) ([]domain.CaseActivity, int, error) {
	// Confirm req.CaseID is actually a case-like work item before reading
	// its activity feed -- comment/case_attachment/work_item_activity are
	// all keyed by the generic work_item_id with no type filter of their
	// own, so without this check a caller could pass any other work_item's
	// UUID (a change request, incident, ...) through this endpoint and read
	// that record's comments/attachments/field changes instead. Same class
	// of gap as IncidentRepository.SearchIncidentActivities' own fix.
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM work_item WHERE id = $1 AND type = ANY(`+caseLikeWorkItemTypes+`))`, req.CaseID).Scan(&exists); err != nil {
		return nil, 0, fmt.Errorf("check case exists: %w", err)
	}
	if !exists {
		return nil, 0, &apierror.NotFoundError{Msg: "case not found"}
	}

	includeFieldChanges := req.IncludeFieldChanges != nil && *req.IncludeFieldChanges

	countQuery := `
		SELECT
			(SELECT COUNT(*) FROM comment WHERE work_item_id = $1) +
			(SELECT COUNT(*) FROM case_attachment WHERE case_id = $1 AND status = 'complete')`
	if includeFieldChanges {
		countQuery += ` + (SELECT COUNT(*) FROM work_item_activity WHERE work_item_id = $1)`
	}

	// UNION ALL merges the tables into one timeline. Comment/field-change
	// rows resolve their (free-text VARCHAR) author by email match against
	// "user"; attachment rows join it directly, since case_attachment.
	// uploaded_by is a real UUID FK (migration 0106) -- see this file's
	// other created_by fixes for why they differ.
	//
	// The comment/field-change branches' email joins are each wrapped in
	// their own DISTINCT ON subquery: "user".email has no unique constraint
	// (migration 0002 only makes user_name UNIQUE), so two user rows
	// sharing an address would otherwise fan a single row out into more than
	// one activity entry, while countQuery above still counts it once --
	// putting the page's rows and its total out of sync.
	dataQuery := `
		WITH activity AS (
			SELECT
				c.id, 'comment' AS kind, c.content, c.created_on AS created_on,
				c.email, c.first_name, c.last_name, c.name, c.type::text AS comment_type,
				NULL::text AS file_name, NULL::text AS content_type, NULL::bigint AS size_bytes,
				NULL::text AS field_name, NULL::text AS old_value, NULL::text AS new_value
			FROM (
				SELECT DISTINCT ON (cm.id)
					cm.id, cm.content, cm.created_on, cm.created_by AS email,
					u1.first_name, u1.last_name,
					COALESCE(u1.name, NULLIF(TRIM(CONCAT_WS(' ', u1.first_name, u1.last_name)), '')) AS name,
					cm.type
				FROM comment cm
				LEFT JOIN "user" u1 ON LOWER(u1.email) = LOWER(cm.created_by)
				WHERE cm.work_item_id = $1
				ORDER BY cm.id, u1.id
			) c

			UNION ALL

			SELECT
				a.id, 'attachment' AS kind, COALESCE(a.description, '') AS content, a.created_on AS created_on,
				u2.email, u2.first_name, u2.last_name,
				COALESCE(u2.name, NULLIF(TRIM(CONCAT_WS(' ', u2.first_name, u2.last_name)), '')) AS name,
				NULL::text AS comment_type,
				a.filename, a.mime_type, a.size_bytes,
				NULL::text AS field_name, NULL::text AS old_value, NULL::text AS new_value
			FROM case_attachment a
			JOIN "user" u2 ON u2.id = a.uploaded_by
			WHERE a.case_id = $1 AND a.status = 'complete'`
	if includeFieldChanges {
		dataQuery += `

			UNION ALL

			SELECT
				fc.id, 'field_change' AS kind, '' AS content, fc.created_on AS created_on,
				fc.email, fc.first_name, fc.last_name, fc.name,
				NULL::text AS comment_type,
				NULL::text AS file_name, NULL::text AS content_type, NULL::bigint AS size_bytes,
				fc.field_name, fc.old_value, fc.new_value
			FROM (
				SELECT DISTINCT ON (wa.id)
					wa.id, wa.created_on, wa.user_email AS email,
					u3.first_name, u3.last_name,
					COALESCE(u3.name, NULLIF(TRIM(CONCAT_WS(' ', u3.first_name, u3.last_name)), '')) AS name,
					wa.field_name, wa.old_value, wa.new_value
				FROM work_item_activity wa
				LEFT JOIN "user" u3 ON LOWER(u3.email) = LOWER(wa.user_email)
				WHERE wa.work_item_id = $1
				ORDER BY wa.id, u3.id
			) fc`
	}
	dataQuery += `
		)
		SELECT id, kind, content, created_on, email, first_name, last_name, name, comment_type, file_name, content_type, size_bytes, field_name, old_value, new_value
		FROM activity
		ORDER BY created_on DESC, id
		LIMIT $2 OFFSET $3`

	var total int
	var activity []domain.CaseActivity

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, req.CaseID).Scan(&total); err != nil {
			return fmt.Errorf("count case activities: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, req.CaseID, req.Pagination.Limit, req.Pagination.Offset)
		if err != nil {
			return fmt.Errorf("query case activities: %w", err)
		}
		defer rows.Close()

		result := make([]domain.CaseActivity, 0, req.Pagination.Limit)
		for rows.Next() {
			a, err := scanCaseActivity(rows)
			if err != nil {
				return fmt.Errorf("scan case activity: %w", err)
			}
			result = append(result, a)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate case activities: %w", err)
		}
		activity = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return activity, total, nil
}

// updateCaseEnforcingOneOngoing is UpdateCase's workState=ONGOING path, for
// every type in workStateWorkItemTypes. It takes a transaction-scoped
// advisory lock keyed on the work item's assignee (two concurrent requests
// for two different work items of the same engineer share no row to lock),
// rejects the update with a ConflictError naming the engineer's other ONGOING
// work item of any of those types, then runs the type's normal update in the
// same transaction. An unassigned work item has no engineer to conflict with
// and proceeds.
func (r *caseRepo) updateCaseEnforcingOneOngoing(ctx context.Context, req domain.UpdateCaseRequest, workItemType, state, severity, workState, resolutionCode, cause string) (domain.Case, *domain.CaseSeverity, error) {
	query, args, label := updateCaseQuery, []any{req.ID, state, severity, workState, resolutionCode, cause, req.CloseNotes}, "case"
	if workItemType != "CASE" {
		var ok bool
		query, args, label, ok = caseLikeExtensionUpdate(workItemType, req, state, workState, resolutionCode, cause)
		if !ok {
			return domain.Case{}, nil, &apierror.ValidationError{Msg: fmt.Sprintf("unsupported work item type: %s", workItemType)}
		}
	}

	// r.db.InTx (Scoped) sets the caller identity, commits on a nil return
	// and rolls back on any error, exactly the lifecycle the hand-written
	// Begin/Commit it replaces had.
	var c domain.Case
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		var assignee *string
		err := tx.QueryRow(ctx, `SELECT assigned_to_id::TEXT FROM work_item WHERE id = $1 AND type = ANY(`+workStateWorkItemTypes+`)`, req.ID).Scan(&assignee)
		if errors.Is(err, pgx.ErrNoRows) {
			return &apierror.NotFoundError{Msg: "case not found"}
		}
		if err != nil {
			return fmt.Errorf("update case: read assignee: %w", err)
		}
		if assignee != nil {
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('case-ongoing:' || $1::TEXT, 0))`, *assignee); err != nil {
				return fmt.Errorf("update case: lock assignee: %w", err)
			}
			var otherNumber string
			err := tx.QueryRow(ctx, `
				SELECT wi.number
				FROM work_item wi
				LEFT JOIN "case" c ON c.id = wi.id
				LEFT JOIN engagement eng ON eng.id = wi.id
				LEFT JOIN service_request sr ON sr.id = wi.id
				LEFT JOIN security_report_analysis sra ON sra.id = wi.id
				WHERE wi.assigned_to_id = $1::uuid AND wi.id <> $2::uuid
				  AND wi.type = ANY(`+workStateWorkItemTypes+`)
				  AND `+caseLikeWorkStateColumn+` = 'ONGOING'
				  AND COALESCE(c.state::TEXT, eng.state::TEXT, sr.state::TEXT, sra.state::TEXT) <> 'CLOSED'
				ORDER BY wi.updated_on DESC LIMIT 1`, *assignee, req.ID).Scan(&otherNumber)
			if err == nil {
				return &apierror.ConflictError{Msg: "Cannot set work state to Ongoing: the assigned engineer already has an Ongoing case: " + otherNumber}
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("update case: check ongoing cases: %w", err)
			}
		}

		var scanErr error
		c, scanErr = scanUpdatedCase(tx.QueryRow(ctx, query, args...))
		if scanErr != nil {
			return fmt.Errorf("update case: %s: %w", label, scanErr)
		}
		return nil
	})
	if err != nil {
		return domain.Case{}, nil, err
	}
	return c, c.Severity, nil
}

// caseAggregateColumns maps an aggregate/group-by field to the SQL expression
// buckets are keyed on (over caseSearchJoins) and the expression the bucket
// label is read from.
var caseAggregateColumns = map[string]struct{ key, label string }{
	"state":          {caseLikeStateColumn, caseLikeStateColumn},
	"severity":       {"c.severity::TEXT", "c.severity::TEXT"},
	"type":           {"wi.type::TEXT", "wi.type::TEXT"},
	"engagementType": {"eng.type::TEXT", "eng.type::TEXT"},
	"issueType":      {"c.issue_type::TEXT", "c.issue_type::TEXT"},
	"workState":      {caseLikeWorkStateColumn, caseLikeWorkStateColumn},
	"account":        {"wi.account_id::TEXT", "a.name"},
}

// AggregateCases implements CaseRepository. It shares buildCaseSearchWhere with
// SearchCases, so a bucket count always agrees with the same search's total.
func (r *caseRepo) AggregateCases(ctx context.Context, req domain.SearchCasesRequest, groupBy string, scope SearchScope) ([]domain.AggregateBucket, error) {
	col, ok := caseAggregateColumns[groupBy]
	if !ok {
		return nil, &apierror.ValidationError{Msg: "groupBy contains invalid value: " + groupBy}
	}
	// Same explicit stamp as SearchCases: the WHERE below carries the
	// announcement leak guard and project hint, and RLS does the rest.
	ctx = WithCallerIdentity(ctx, scope)
	where, args, _, err := buildCaseSearchWhere(req, scope)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		SELECT %s AS bucket_key, MAX(%s) AS bucket_label, COUNT(*) AS bucket_count
		FROM work_item wi %s %s AND %s IS NOT NULL
		GROUP BY bucket_key
		ORDER BY bucket_count DESC, bucket_key`, col.key, col.label, caseSearchJoins, where, col.key)
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("aggregate cases: %w", err)
	}
	defer rows.Close()

	buckets := []domain.AggregateBucket{}
	for rows.Next() {
		var key string
		var label *string
		var count int
		if err := rows.Scan(&key, &label, &count); err != nil {
			return nil, fmt.Errorf("scan case bucket: %w", err)
		}
		k, l := aggregateBucketKey(groupBy, key), stringOrEmpty(label)
		if l == "" || groupBy != "account" {
			l = k
		}
		buckets = append(buckets, domain.AggregateBucket{Key: k, Label: l, Count: count})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate case buckets: %w", err)
	}
	return buckets, nil
}

// aggregateBucketKey converts a stored enum label to the domain value the
// case search contract uses (lower-case, severity via caseSeverityFromEnum);
// account keys are UUIDs and pass through.
func aggregateBucketKey(groupBy, raw string) string {
	switch groupBy {
	case "account":
		return raw
	case "severity":
		if v, ok := caseSeverityFromEnum[raw]; ok {
			return string(v)
		}
	}
	return strings.ToLower(raw)
}
