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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// publishCaseCreatedTimeout bounds publishCaseCreated's enrichment
// (GetCaseByID) + publish, so a slow ServiceNow or Event Hub round trip
// can't consume all of the request's own deadline (middleware.Timeout) —
// see publishCaseCreated's doc comment for why this runs synchronously
// rather than detached.
const publishCaseCreatedTimeout = 5 * time.Second

// publishCommentAddedTimeout bounds publishCommentAdded's enrichment
// (GetCaseByID plus a bounded SearchCaseComments lookup for the comment
// author's resolved display name) + publish — same reasoning as
// publishCaseCreatedTimeout.
const publishCommentAddedTimeout = 5 * time.Second

// publishStatusChangedTimeout bounds two separate things that share the
// same reasoning as publishCaseCreatedTimeout: UpdateCase's own pre-PATCH
// GetCaseByID enrichment/no-op check, and publishStatusChanged's own
// Publish call afterward.
const publishStatusChangedTimeout = 5 * time.Second

// publishCaseAssignedTimeout bounds the same two things as
// publishStatusChangedTimeout, for UpdateCase's assigneeEmail path instead
// of its state path.
const publishCaseAssignedTimeout = 5 * time.Second

// publishCaseAcknowledgedTimeout bounds publishCaseAcknowledged's own
// GetCaseByID enrichment + publish call — see publishCaseCreatedTimeout's
// doc comment for why.
const publishCaseAcknowledgedTimeout = 5 * time.Second

// publishSeverityChangedTimeout bounds publishSeverityChanged's own publish
// call — see publishStatusChangedTimeout's doc comment for why. Unlike
// publishCaseAcknowledgedTimeout, this doesn't need to also bound a
// GetCaseByID call: UpdateCase's own pre-PATCH enrichment (mirroring its
// status-change block) already supplies the "before" CaseView this publish
// needs.
const publishSeverityChangedTimeout = 5 * time.Second

// applyResponseSLATimeout bounds applyResponseSLAOnComment's own author
// resolution (SearchCaseComments) + role lookup (SearchUsers) +
// SetSLAClockTierReached calls — same reasoning as publishCaseCreatedTimeout,
// though this one isn't a publish at all (see that function's own doc
// comment for why it's independent of s.publisher/Event Hub entirely).
const applyResponseSLATimeout = 5 * time.Second

// applyCaseStateSLATimeout bounds applyCaseStateSLAEffects' own
// Pause/Resume/SetSLAClockTierReached calls — same reasoning as
// applyResponseSLATimeout.
const applyCaseStateSLATimeout = 5 * time.Second

// applyCustomerReplyTimeout bounds applyCustomerReplyStateTransition's own
// GetCaseByID + author resolution + role lookup + UpdateCase calls — same
// reasoning as applyResponseSLATimeout.
const applyCustomerReplyTimeout = 5 * time.Second

// watchListEmails extracts the non-empty emails from a case's watch list —
// Recipients for every case.* event this file publishes is the case's
// WatchList emails only (an explicit, deliberate decision — this service
// has no other notion of who should be emailed for a case; see
// publishCaseCreated's own doc comment).
func watchListUserEmails(watchList []domain.WatchListUser) []string {
	recipients := make([]string, 0, len(watchList))
	for _, w := range watchList {
		if w.Email != "" {
			recipients = append(recipients, w.Email)
		}
	}
	return recipients
}

// caseProductName returns cv's deployed product's display name (e.g. "WSO2
// API Manager"), "" when the case has no deployed product. Shared by every
// publisher that needs it for a case.* payload's Product field (Google
// Chat space routing key and, for case.created, also a display line — see
// events.CaseCreatedPayload.Product's own doc comment).
func caseProductName(cv domain.CaseView) string {
	if cv.DeployedProductDetails != nil && cv.DeployedProductDetails.Product != nil {
		return cv.DeployedProductDetails.Product.Name
	}
	return ""
}

// caseTeamName returns cv's account's CRE team display name (e.g. "Team
// Nova"), "" when the case has no account or the account has no CRE team.
// Shared by every publisher that needs it for a case.* payload's Team
// field — see events.CaseCreatedPayload.Team's own doc comment.
//
// Reads cv.AccountDetails.CreTeam, which GetCaseByID already resolves from
// the case's own embedded account object (see snCaseAccount's doc comment)
// — no extra lookup needed, unlike a fresh GET /accounts/{id} call. As of
// this field's introduction, that embedded object's creTeam/sreTeam are
// documented ("not yet available in the backing service") as not
// guaranteed to be populated by the ServiceNow integration yet, even
// though the standalone accounts endpoint does return them — this helper
// simply passes through whatever GetCaseByID resolved, so Team may come
// back empty in practice until that catches up. If it does, the fix is on
// the backing service, not here.
func caseTeamName(cv domain.CaseView) string {
	if cv.AccountDetails != nil && cv.AccountDetails.CreTeam != nil {
		return cv.AccountDetails.CreTeam.Name
	}
	return ""
}

// wso2EmailDomain is WSO2's own corporate domain — mirrors
// apps/csm-portal/backend's own wso2EmailDomain constant (see that
// package's user_external_account.go).
const wso2EmailDomain = "@wso2.com"

// filterWso2Emails returns only the emails in emails on wso2EmailDomain,
// preserving order. Used for CommentTypeWorkNote (an internal note, never
// meant for a customer's eyes) — a case's watch list can contain both
// internal and customer emails, so publishing an internal note's
// case.comment_added event with the full, unfiltered watch list would
// notify customer watchers about a note that was never meant for them,
// regardless of how that watch list was populated.
func filterWso2Emails(emails []string) []string {
	filtered := make([]string, 0, len(emails))
	for _, e := range emails {
		if strings.HasSuffix(strings.ToLower(e), wso2EmailDomain) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// snCasesResponse mirrors the Choreo POST /cases/search response.
type snCasesResponse struct {
	Cases        []snCase `json:"cases"`
	TotalRecords int      `json:"totalRecords"`
	Offset       int      `json:"offset"`
	Limit        int      `json:"limit"`
}

type snCase struct {
	ID                string                 `json:"id"`
	InternalID        string                 `json:"internalId"`
	Number            string                 `json:"number"`
	Title             string                 `json:"title"`
	Description       string                 `json:"description"`
	CreatedOn         string                 `json:"createdOn"`
	UpdatedOn         *string                `json:"updatedOn"`
	CreatedBy         string                 `json:"createdBy"`
	CreatedByFullName string                 `json:"createdByFullName"`
	Project           snCaseProjectRef       `json:"project"`
	Deployment        snCaseEntityRef        `json:"deployment"`
	DeployedProduct   snCaseDeployedProduct  `json:"deployedProduct"`
	Product           *snCaseEntityRef       `json:"product"`
	State             *snCaseState           `json:"state"`
	WorkState         *snCaseLabel           `json:"workState"`
	Severity          *snCaseLabel           `json:"severity"`
	IssueType         *snCaseIssueType       `json:"issueType"`
	EngagementType    *snCaseLabel           `json:"engagementType"`
	CaseType          *snCaseEntityRef       `json:"caseType"`
	Catalog           *snCaseEntityRef       `json:"catalog"`
	CatalogItem       *snCaseEntityRef       `json:"catalogItem"`
	AssignedTeam      *snCaseEntityRef       `json:"assignedTeam"`
	Conversation      *snCaseEntityRef       `json:"conversation"`
	AssignedEngineer  *snAssignedEngineerRef `json:"assignedEngineer"`
	AcknowledgedBy    *snAssignedEngineerRef `json:"acknowledgedBy"`
	// WorkaroundProvidedOn/WorkaroundProvidedBy mirror AcknowledgedBy's shape:
	// populated on the Choreo GET /cases/{id} response, null until the workaround
	// is marked provided (and cleared again on recall).
	WorkaroundProvidedOn  *string                     `json:"workaroundProvidedOn"`
	WorkaroundProvidedBy  *snAssignedEngineerRef      `json:"workaroundProvidedBy"`
	ParentCase            *snCaseRef                  `json:"parentCase"`
	RelatedCase           *snCaseRef                  `json:"relatedCase"`
	Account               *snCaseAccount              `json:"account"`
	LinkedServiceRequests []snLinkedServiceRequestRef `json:"linkedServiceRequests"`
	// Variables carries the answers to the catalog item's questions on a service
	// request, keyed as `variables` upstream. Present on the Choreo GET /cases/{id}
	// response for service-request cases only; absent for every other case type, so
	// this must tolerate absence.
	Variables []snCaseVariableAnswer `json:"variables"`
	// ChangeRequests carries the change requests raised from this case, keyed as
	// `changeRequests` upstream. Only populated for service-request cases.
	//
	// Deprecated: the upstream `changeRequests` field is filtered to a subset of change
	// request states by the backing service. Case mapping below reads ChangeRequestsAll
	// instead, which is unfiltered. This field is kept only because it is still present
	// on the upstream response; nothing in this service reads it.
	ChangeRequests []snLinkedChangeRequestRef `json:"changeRequests"`
	// ChangeRequestsAll carries the same change requests as ChangeRequests but unfiltered by
	// state, keyed as `changeRequestsAll` upstream. Same item shape as `changeRequests`. Case
	// mapping reads this field so linked change requests in New/Assess/Authorize states are
	// no longer silently dropped before they reach the outward `linkedChangeRequests` field.
	ChangeRequestsAll []snLinkedChangeRequestRef `json:"changeRequestsAll"`
	ResolutionCode    *struct {
		ID    json.Number `json:"id"`
		Label string      `json:"label"`
	} `json:"resolutionCode"`
	Cause *struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	} `json:"cause"`
	ResolutionNotes *string `json:"resolutionNotes"`
	ResolvedOn      *string `json:"resolvedOn"`
	// WatchList carries the watchers on the case (four SN glide_lists collapsed into one
	// list by Ballerina). Confirmed present on the Choreo GET /cases/{id} response
	// (the corresponding field in the backing service's case response).
	WatchList []snWatchListUser `json:"watchList"`
	// AutoclosureStep/AutoclosureStateTime surface ServiceNow's real staged auto-closure
	// sequence (u_autoclosure_step / u_autoclosure_state_time), confirmed live against a
	// held case on the dev tenant. Ballerina's
	// Case/CaseResponse carry matching autoclosureStep/autoclosureStateTime fields (not
	// yet available in the backing service -- the closest field it exposes today,
	// hasAutoClosed, means something different: case has already been auto-closed, not
	// "where the case sits in the auto-closure sequence").
	AutoclosureStep *string `json:"autoclosureStep"`
	// AutoclosureStateTime is when the auto-closure sequence next advances (e.g. the
	// "eligible again after" date for a held case).
	AutoclosureStateTime *string `json:"autoclosureStateTime"`
	// BestCaseFixEta is the internal-only best-case fix-commitment date
	// (u_best_case_fix_eta). Populated on the Choreo GET /cases/{id} response
	// unconditionally, and on POST /cases/search rows only when the search
	// request set includeExtendedFields — nil otherwise, so this must tolerate
	// absence.
	BestCaseFixEta *string `json:"bestCaseFixEta"`
	// MostLikelyFixEta is the internal-only most-likely fix-commitment date
	// (u_most_likely_fix_eta). Populated on the Choreo GET /cases/{id} response
	// unconditionally, and on POST /cases/search rows only when the search
	// request set includeExtendedFields — nil otherwise, so this must tolerate
	// absence.
	MostLikelyFixEta *string `json:"mostLikelyFixEta"`
	// WorstCaseFixEta is the internal-only worst-case fix-commitment date
	// (u_worst_case_fix_eta). Populated on the Choreo GET /cases/{id} response
	// unconditionally, and on POST /cases/search rows only when the search
	// request set includeExtendedFields — nil otherwise, so this must tolerate
	// absence.
	WorstCaseFixEta *string `json:"worstCaseFixEta"`
	// Fields the Ballerina entity-service declares on CaseResponse that were
	// not declared here, so encoding/json discarded them. All nullable: an
	// absent key stays nil rather than becoming a zero value.
	SLAResponseTime       *string          `json:"slaResponseTime"`
	ClosedBy              *snCaseEntityRef `json:"closedBy"`
	HasAutoClosed         *bool            `json:"hasAutoClosed"`
	EngagementStartDate   *string          `json:"engagementStartDate"`
	EngagementEndDate     *string          `json:"engagementEndDate"`
	EngagementPaymentType *snCaseLabel     `json:"engagementPaymentType"`
	// Duration/EscalationLevel/IsEscalated are response fields Choreo already
	// sends. Note escalationLevel appears twice in this file for different
	// purposes: as a []string request filter on the search payload, and as this
	// single choice-list value on the response. Only the filter was declared, so
	// encoding/json discarded all three on the way back and the portal had no
	// escalation state or duration to render.
	Duration        *string                `json:"duration"`
	EscalationLevel *snCaseEscalationLevel `json:"escalationLevel"`
	IsEscalated     *bool                  `json:"isEscalated"`
}

// snCaseVariableAnswer mirrors one answered catalog-item question on a service
// request as the backing service returns it (CaseResponse.variables). Distinct
// from snCaseVariable, which is the {id, value} shape a case create submits.
type snCaseVariableAnswer struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// snWatchListUser mirrors the watch-list user shape ServiceNow/Ballerina returns on both
// the case detail read (CaseResponse.watchList) and the update-case response
// (snUpdateCaseResponse.Case.WatchList below).
type snWatchListUser struct {
	ID       string  `json:"id"`
	UserName string  `json:"userName"`
	Name     *string `json:"name"`
	Email    *string `json:"email"`
}

type snCaseEntityRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// snCaseProjectRef is the case's project reference. Key is the project's short
// human-readable key (e.g. "TESTQUERYSUB"); it is only present on search rows when
// the search request set includeExtendedFields, so it must tolerate absence.
type snCaseProjectRef struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Key  *string `json:"key"`
}

type snCaseDeployedProduct struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

type snCaseRef struct {
	ID     string  `json:"id"`
	Number string  `json:"number"`
	Type   *string `json:"type"`
}

type snLinkedServiceRequestRef struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	Name   string `json:"name"`
}

type snLinkedChangeRequestRef struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	Name   string `json:"name"`
}

type snAssignedEngineerRef struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Email *string `json:"email"`
}

type snCaseAccount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	// CreTeam and SreTeam resolve the account's CRE/SRE group refs. Per the project
	// owner's resolution, SN's u_integration_cs_team maps to CreTeam and u_sre_team maps
	// to SreTeam -- both refs to sys_user_group. Ballerina's Case/CaseResponse.account
	// gained matching fields, but they are not yet available in the backing service.
	CreTeam *snCaseEntityRef `json:"creTeam"`
	SreTeam *snCaseEntityRef `json:"sreTeam"`
}

type snCaseState struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

type snCaseLabel struct {
	Label string `json:"label"`
}

// snCaseEscalationLevel is the escalation-level choice list on a case response,
// e.g. {"id": "0", "label": "EL0"}. snCaseLabel cannot be reused because it
// carries no id, and the id is the part that round-trips into the
// escalationLevel request filter. json.Number because Choreo has sent choice ids
// both quoted and bare elsewhere in this API.
type snCaseEscalationLevel struct {
	ID    json.Number `json:"id"`
	Label string      `json:"label"`
}

type snCaseIssueType struct {
	ID    json.Number `json:"id"`
	Label string      `json:"label"`
}

// snCaseSearchPayload is the Choreo POST /cases/search request body.
type snCaseSearchPayload struct {
	Filters    snCaseFilters       `json:"filters,omitempty"`
	SortBy     *snCaseSort         `json:"sortBy,omitempty"`
	Pagination snProjectPagination `json:"pagination"`
	// IncludeExtendedFields opts a search row into the account reference, project
	// key, and fix-ETA fields (see snCase's Account/Project/BestCaseFixEta/
	// MostLikelyFixEta/WorstCaseFixEta doc comments). Left unset (false) on the
	// group-count fan-out in searchCasesGroupCount, which reads only
	// totalRecords and never touches a row's fields, to keep that call as
	// lightweight as it is today. SearchCases itself always sets this to true.
	IncludeExtendedFields bool `json:"includeExtendedFields,omitempty"`
}

type snCaseSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

// snCaseTypeMap maps domain case type strings to the ServiceNow caseType values.
var snCaseTypeMap = map[string]string{
	"case":                     "default_case",
	"service_request":          "service_request",
	"security_report_analysis": "security_report_analysis",
	"announcement":             "announcement",
	"engagement":               "engagement",
	"hosting":                  "hosting",
	"hosting_query":            "hosting_query",
	"hosting_task":             "hosting_task",
}

// snEscalationLevelToDomain reduces Choreo's escalation-level choice list to the
// plain nullable string this service exposes for enum-valued fields.
//
// It keeps the level id ("0".."5", the same vocabulary validEscalationLevel
// accepts on the request side) rather than the display label ("EL0"), so the
// value round-trips: a caller can feed what it reads back into the
// escalationLevel filter. Building the {id, label} pair the portal renders is
// the BFF's job, not this service's.
//
// An id outside the known set yields nil rather than passing an unrecognised
// value through, matching how the other snXxxToEnum helpers here behave.
func snEscalationLevelToDomain(l *snCaseEscalationLevel) *string {
	if l == nil {
		return nil
	}
	id := strings.TrimSpace(l.ID.String())
	if id == "" || !validEscalationLevel[id] {
		return nil
	}
	return &id
}

// snCaseTypeSysidMap maps ServiceNow caseType sysids to domain case type values.
var snCaseTypeSysidMap = map[string]string{
	"8d4b87bd1b18f010cb6898aebd4bcb59": "case",
	"0d5b8fbd1b18f010cb6898aebd4bcba5": "case",
	"5aeff1201b74c210264c997a234bcb54": "service_request",
	"ab36479047ccf510a0a29cd3846d43ee": "security_report_analysis",
	"3b8b43311b58f010cb6898aebd4bcb8f": "announcement",
	"8f8fc2c41b0bd550d64e64a2604bcb38": "engagement",
	"bfa1473c1bbcb410cb6898aebd4bcb52": "hosting",
	"80810ff81bbcb410cb6898aebd4bcb3c": "hosting_query",
	"f46103f81bbcb410cb6898aebd4bcb27": "hosting_task",
}

// snCaseTypeToDomain converts a SN caseType entity ref to the domain type string.
// Maps by sysid; defaults to "case" when caseType is null or the sysid is unrecognised.
func snCaseTypeToDomain(ct *snCaseEntityRef) *string {
	domainType := "case"
	if ct != nil {
		if mapped, ok := snCaseTypeSysidMap[ct.ID]; ok {
			domainType = mapped
		}
	}
	return &domainType
}

// snParentCaseTypeMap maps the raw ServiceNow parent/related-case type value --
// derived by CaseUtils.js's _mapCaseDetails from the parent record's sys_class_name
// (sn_customerservice_case -> "case", incident -> "incident", change_request ->
// "change_request", problem -> "problem") -- to the API's public CaseNumberRef.type
// enum. SN already emits these exact literal strings, so this is effectively an
// allow-list guarding against an unrecognised or future upstream value, kept as a
// map (rather than a set) to match this file's other snXTypeMap conventions.
var snParentCaseTypeMap = map[string]string{
	"case":           "case",
	"incident":       "incident",
	"change_request": "change_request",
	"problem":        "problem",
}

// snParentCaseTypeToDomain maps a parent/related case reference's raw SN type
// value to the domain enum, returning nil for nil or unrecognised values so an
// unsupported ServiceNow task type never leaks onto the API surface.
func snParentCaseTypeToDomain(raw *string) *string {
	if raw == nil {
		return nil
	}
	if mapped, ok := snParentCaseTypeMap[*raw]; ok {
		return &mapped
	}
	return nil
}

func domainTypeKeysToSN(typeKeys []string) []string {
	result := make([]string, 0, len(typeKeys))
	for _, t := range typeKeys {
		if sn, ok := snCaseTypeMap[t]; ok {
			result = append(result, sn)
		}
	}
	return result
}

// snSortFieldMap maps domain CaseSortField values to SN field names.
// validEscalationLevel is the set of real escalation level ids confirmed live
// against wso2sndev (EL0 "not escalated" through EL5 "CEO").
var validEscalationLevel = map[string]bool{
	"0": true, "1": true, "2": true, "3": true, "4": true, "5": true,
}

var snSortFieldMap = map[domain.CaseSortField]string{
	domain.CaseSortFieldCreatedOn: "createdOn",
	domain.CaseSortFieldUpdatedOn: "updatedOn",
	domain.CaseSortFieldSeverity:  "severity",
	domain.CaseSortFieldState:     "state",
}

// caseGroupByFieldValues enumerates the case-search fields SearchCases can
// group results by, each backed by a small fixed enum, and every value in
// that enum (in stable display order). Grouping is implemented as a fan-out
// of one limit=1 SearchCases call per value (cheap: SN computes the matching
// total regardless of page size) rather than a ServiceNow-side aggregate
// query, so no SN/Ballerina change is needed -- see
// case-search-filter-dsl-migration for why this stays a Go-entity-service-only
// capability. Free-text fields (e.g. tag) have no fixed value set and are not
// groupable.
//
// The enumeration is also the limit of what a grouped response can see: the
// grouped SearchCasesResponse.Total is the sum of these buckets, so a case
// whose field value is absent from this map is counted nowhere and does not
// reach Total. Keep these lists in step with the backing data source's value
// sets, and see the SearchCasesResponse doc comment for the caller-facing
// consequence.
var caseGroupByFieldValues = map[string][]string{
	"state":          {"open", "work_in_progress", "waiting_on_wso2", "awaiting_info", "reopened", "solution_proposed", "closed"},
	"severity":       {"catastrophic", "critical", "high", "medium", "low"},
	"type":           {"case", "service_request", "security_report_analysis", "announcement", "engagement"},
	"engagementType": {"migration", "consultancy", "new_feature_improvement", "follow_up", "onboarding"},
	"issueType":      {"error", "partial_outage", "performance_degradation", "question", "security_or_compliance", "total_outage"},
	"workState":      {"ongoing", "paused"},
}

type snCaseFilters struct {
	CaseTypes          []string `json:"caseTypes"`
	SearchQuery        string   `json:"searchQuery,omitempty"`
	ProjectIDs         []string `json:"projectIds,omitempty"`
	DeploymentIDs      []string `json:"deploymentIds,omitempty"`
	DeployedProductIDs []string `json:"deployedProductIds,omitempty"`
	StateKeys          []int    `json:"stateKeys,omitempty"`
	// ExcludeStates is the inverse of StateKeys: cases whose state is none of
	// these. It carries the same numeric state keys StateKeys does, produced by
	// the same domainStatesToSNIDs conversion, because that is the form
	// ServiceNow's `state` column is queried in -- a list of state names would
	// match nothing. See domain.ParsedCaseFilters.ExcludeStates.
	//
	// Deployment ordering: the downstream service must be deployed before this
	// one. Its search-filter record is closed, so an undeclared key is
	// rejected outright rather than ignored -- if this service ships first, a
	// search carrying a state exclusion fails loudly instead of quietly
	// returning an unfiltered result set. That is the intended direction: a
	// dropped exclusion widens the result set, which is the harder failure to
	// notice. omitempty keeps the key off the wire entirely unless a
	// `state notIn` filter was actually requested, so no search that does not
	// use one is affected by the ordering.
	ExcludeStates      []int    `json:"excludeStateKeys,omitempty"`
	SeverityKeys       []int    `json:"severityKeys,omitempty"`
	IssueTypeKeys      []int    `json:"issueTypeKeys,omitempty"`
	EngagementTypeKeys []int    `json:"engagementTypeKeys,omitempty"`
	ClosedStartDate    string   `json:"closedStartDate,omitempty"`
	ClosedEndDate      string   `json:"closedEndDate,omitempty"`
	ResolvedStartDate  string   `json:"resolvedStartDate,omitempty"`
	ResolvedEndDate    string   `json:"resolvedEndDate,omitempty"`
	StartCreatedDate   string   `json:"startCreatedDate,omitempty"`
	EndCreatedDate     string   `json:"endCreatedDate,omitempty"`
	StartUpdatedDate   string   `json:"startUpdatedDate,omitempty"`
	EndUpdatedDate     string   `json:"endUpdatedDate,omitempty"`
	CreatedBy          []string `json:"createdBy,omitempty"`
	CreatedByMe        bool     `json:"createdByMe,omitempty"`
	WorkStateKeys      []int    `json:"workStateKeys,omitempty"`
	AssignedUserIDs    []string `json:"assignedUserIds,omitempty"`
	ProductNames       []string `json:"productNames,omitempty"`
	// Tags: filters cases by attached free-text label. Works end-to-end now
	// (ServiceNow's CaseUtils.searchCases honors filters.tags; Ballerina's
	// CaseSearchFilters forwards it).
	Tags []string `json:"tags,omitempty"`
	// ExcludeTags: inverse of Tags, also works end-to-end (ServiceNow's
	// CaseUtils.searchCases honors filters.excludeTags; Ballerina's CaseSearchFilters
	// declares it). See domain.ParsedCaseFilters.ExcludeTags.
	ExcludeTags []string `json:"excludeTags,omitempty"`
	// ParentID: see domain.SearchCasesFilters.ParentID doc comment.
	ParentID string `json:"parentId,omitempty"`
	// Number: see domain.ParsedCaseFilters.Number doc comment. Exact
	// match against ServiceNow's `number` column -- not part of the free-text
	// SearchQuery scan.
	Number string `json:"number,omitempty"`
	// InternalID: see domain.ParsedCaseFilters.InternalID doc comment. Exact
	// match against ServiceNow's `u_wso2_case_id` column -- not part of the
	// free-text SearchQuery scan.
	InternalID                       string   `json:"internalId,omitempty"`
	ProjectOnboardingStatuses        []string `json:"projectOnboardingStatuses,omitempty"`
	ExcludeProjectOnboardingStatuses []string `json:"excludeProjectOnboardingStatuses,omitempty"`
	// ProjectTypeNames carries project-type NAMES (e.g. "Subscription"), not
	// sys_ids: CaseUtils.searchCases matches project.u_project_type.u_name.
	// It goes out under its own key, "projectTypes", rather than reusing the
	// old id-based "projectTypeIds": that key is typed as a 32-hex-character
	// sys_id array in the Ballerina contract, so a name sent through it is
	// rejected outright by request validation. "projectTypeIds" has no
	// remaining producer or consumer on either portal and is being retired
	// from the contract alongside this change.
	ProjectTypeNames []string `json:"projectTypes,omitempty"`
	// CreTeamIDs and SreTeamIDs go out under the wire keys the Ballerina/SN
	// contract already uses (integrationCsTeamIds, sreTeamIds) -- only the Go
	// domain naming changed, not the wire protocol.
	CreTeamIDs []string `json:"integrationCsTeamIds,omitempty"`
	SreTeamIDs []string `json:"sreTeamIds,omitempty"`
	// AccountIDs: see domain.ParsedCaseFilters.AccountIDs doc comment.
	AccountIDs           []string `json:"accountIds,omitempty"`
	Unassigned           bool     `json:"unassigned,omitempty"`
	ResolutionNotesEmpty bool     `json:"resolutionNotesEmpty,omitempty"`
	// TaskSLAFilter: SN-side join on Task SLA table, filtering by businessElapsedPercent
	// range. Confined to SN adapter per vendor-neutral boundary; see
	// domain.TaskSLAFilter doc comment.
	TaskSLAFilter *snTaskSLAFilter `json:"taskSLAFilter,omitempty"`
	// EscalationLevels: see domain.SearchCasesFilters.EscalationLevels doc comment.
	EscalationLevels []string `json:"escalationLevel,omitempty"`
	// IsEscalated: a *bool (not bool) so that an explicit false is still sent on
	// the wire -- omitempty on a pointer only drops a nil, not a false value.
	IsEscalated *bool `json:"isEscalated,omitempty"`
	// SlaBreached: see domain.ParsedCaseFilters.HasBreachedSLA doc comment. A
	// *bool for the same reason as IsEscalated above -- an explicit false must
	// still reach ServiceNow, not be silently dropped.
	SlaBreached *bool `json:"slaBreached,omitempty"`
	// AccountEscalationActive: see domain.ParsedCaseFilters.HasActiveAccountEscalation
	// doc comment. A *bool for the same reason as IsEscalated above.
	AccountEscalationActive *bool `json:"accountEscalationActive,omitempty"`
	// OrGroups is the ServiceNow wire field name, deliberately unchanged:
	// ServiceNow's CaseUtils Script Include reads "orGroups" and silently
	// ignores JSON keys it does not recognise (returning an unfiltered count
	// rather than erroring), so this key must NOT be renamed to track the
	// public API's domain.SearchCasesFilters.AnyOf. See that doc comment for
	// the semantics.
	OrGroups []snCaseFilterGroup `json:"orGroups,omitempty"`
}

// snTaskSLAFilter represents the Task SLA filter for SN's POST /cases/search request.
// snTaskSLAFilter is the SN wire shape of the Task-SLA business-elapsed-percent
// bounds. The bounds are pointers, not plain ints: 0 is a legitimate bound
// (`lte 0` means "no elapsed SLA time at all"), and with a plain int the
// omitempty tag would strip an explicit 0 from the payload, leaving ServiceNow
// with no bound and silently widening the result set. nil means "no bound".
type snTaskSLAFilter struct {
	MinBusinessElapsedPercent *int `json:"minBusinessElapsedPercent,omitempty"`
	MaxBusinessElapsedPercent *int `json:"maxBusinessElapsedPercent,omitempty"`
}

// buildSNTaskSLAFilter converts a domain.TaskSLAFilter into the SN wire shape,
// or nil if no filter was supplied. Bounds are copied by value into fresh
// pointers so the payload never aliases the caller's parsed filters.
func buildSNTaskSLAFilter(f *domain.TaskSLAFilter) *snTaskSLAFilter {
	if f == nil {
		return nil
	}
	sn := &snTaskSLAFilter{}
	if f.MinBusinessElapsedPercent != nil {
		minPct := *f.MinBusinessElapsedPercent
		sn.MinBusinessElapsedPercent = &minPct
	}
	if f.MaxBusinessElapsedPercent != nil {
		maxPct := *f.MaxBusinessElapsedPercent
		sn.MaxBusinessElapsedPercent = &maxPct
	}
	return sn
}

// snStateIDMap maps domain CaseState enums to SN numeric state IDs.
var snStateIDMap = map[domain.CaseState]int{
	domain.CaseStateOpen:             1,
	domain.CaseStateWorkInProgress:   10,
	domain.CaseStateAwaitingInfo:     18,
	domain.CaseStateWaitingOnWSO2:    1003,
	domain.CaseStateReopened:         1006,
	domain.CaseStateSolutionProposed: 6,
	domain.CaseStateClosed:           3,
}

// snSeverityIDMap maps domain CaseSeverity enums to SN numeric severity IDs.
var snSeverityIDMap = map[domain.CaseSeverity]int{
	domain.CaseSeverityCatastrophic: 14,
	domain.CaseSeverityCritical:     10,
	domain.CaseSeverityHigh:         11,
	domain.CaseSeverityMedium:       12,
	domain.CaseSeverityLow:          13,
}

// snIssueTypeIDMap maps domain CaseIssueType enums to SN numeric issue-type IDs.
var snIssueTypeIDMap = map[domain.CaseIssueType]int{
	domain.CaseIssueTypeTotalOutage:            1,
	domain.CaseIssueTypePartialOutage:          2,
	domain.CaseIssueTypePerformanceDegradation: 3,
	domain.CaseIssueTypeQuestion:               4,
	domain.CaseIssueTypeSecurityOrCompliance:   5,
	domain.CaseIssueTypeError:                  6,
}

// snEngagementTypeIDMap maps domain EngagementType enums to SN numeric engagement-type IDs.
var snEngagementTypeIDMap = map[domain.EngagementType]int{
	domain.EngagementTypeMigration:             1,
	domain.EngagementTypeConsultancy:           2,
	domain.EngagementTypeNewFeatureImprovement: 3,
	domain.EngagementTypeFollowUp:              4,
	domain.EngagementTypeOnboarding:            5,
}

// snEngagementPaymentTypeIDMap maps domain EngagementPaymentType enums to SN numeric
// engagement-payment-type IDs.
var snEngagementPaymentTypeIDMap = map[domain.EngagementPaymentType]int{
	domain.EngagementPaymentTypePaid: 1,
	domain.EngagementPaymentTypeFOC:  2,
}

func domainStatesToSNIDs(states []domain.CaseState) []int {
	ids := make([]int, 0, len(states))
	for _, s := range states {
		if id, ok := snStateIDMap[s]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func domainSeveritiesToSNIDs(severities []domain.CaseSeverity) []int {
	ids := make([]int, 0, len(severities))
	for _, s := range severities {
		if id, ok := snSeverityIDMap[s]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func domainIssueTypesToSNIDs(issueTypes []domain.CaseIssueType) []int {
	ids := make([]int, 0, len(issueTypes))
	for _, it := range issueTypes {
		if id, ok := snIssueTypeIDMap[it]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func domainWorkStatesToSNIDs(workStates []domain.CaseWorkState) []int {
	ids := make([]int, 0, len(workStates))
	for _, ws := range workStates {
		if id, ok := snWorkStateIDMap[ws]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func domainEngagementTypesToSNIDs(engTypes []domain.EngagementType) []int {
	ids := make([]int, 0, len(engTypes))
	for _, et := range engTypes {
		if id, ok := snEngagementTypeIDMap[et]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func formatSNDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(snCreatedOnLayout)
}

// snUtcDateTimeLayout is the ISO 8601 UTC wire format ("UtcDateTimeString" in the
// integration service's OpenAPI contract, e.g. servicenow:CaseSearchFilters'
// startCreatedDate/endCreatedDate/closedStartDate/closedEndDate) that case- and
// change-request-search date-range filters must be sent in. Distinct from
// snCreatedOnLayout (space-separated, "YYYY-MM-DD HH:MM:SS"): that layout is what SN
// itself returns for a record's own created_on/updated_on/resolved_on fields, not what
// the integration service's search-filter contract accepts for a caller-supplied date
// range. Sending the space-separated layout here 400s with a payload-schema pattern
// validation error.
const snUtcDateTimeLayout = "2006-01-02T15:04:05Z"

// formatSNDateTimeUTC renders a date-range search-filter bound in the integration
// service's UtcDateTimeString format. See snUtcDateTimeLayout for why this differs
// from formatSNDate.
func formatSNDateTimeUTC(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(snUtcDateTimeLayout)
}

// snDateOnlyLayout is the date-only wire format the integration service expects for
// fields whose contract carries a date with no time component.
const snDateOnlyLayout = "2006-01-02"

// formatSNDateOnly renders a date-only value. The auto-closure hold is a date, not a
// datetime: the integration service constrains it to YYYY-MM-DD, and sending a
// datetime fails payload binding before the request ever reaches the data source.
func formatSNDateOnly(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(snDateOnlyLayout)
}

// validateDateOnly rejects a wire value that is not a strict "YYYY-MM-DD" date. Used
// for fields (bestCaseFixEta, mostLikelyFixEta, worstCaseFixEta) whose contract with
// the integration service is a plain date string, not an RFC3339 datetime — unlike
// AutocloseHoldUntil, these arrive as *string already, so there is no JSON-time-based
// parse to lean on; the format must be checked explicitly before it is forwarded.
func validateDateOnly(field, value string) error {
	if _, err := time.Parse(snDateOnlyLayout, value); err != nil {
		return &apierror.ValidationError{Msg: field + " must be a date in YYYY-MM-DD format"}
	}
	return nil
}

type snCaseService struct {
	client     *integrationservice.Client
	pgFallback CaseService
	// publisher is nil when Event Hub is not configured (see
	// config.Config.EventHubBroker) — every call site must check before
	// using it. See publishCaseCreated.
	publisher EventPublisherService
	// slaClocks/userSvc/supportEngineerRole back the SLA-tracking additions
	// in this file (publishCaseCreated's registration step,
	// applyResponseSLAOnComment, applyCaseStateSLAEffects) — see each
	// function's own doc comment. slaClocks is nil when this service is
	// running with DATA_SOURCE=servicenow and no database configured (see
	// config.Config.HasDatabase/routes.go's own comment on slaClockService)
	// — every call site must check before using it, same as publisher.
	// userSvc is only ever consulted once supportEngineerRole is non-empty
	// (see applyResponseSLAOnComment), so it is never dereferenced while
	// nil in practice. supportEngineerRole may be "" (see
	// config.Config.SupportEngineerRole's own doc comment), which
	// applyResponseSLAOnComment treats as "can't confirm engineer
	// authorship, skip" rather than an error.
	slaClocks           SLAClockService
	userSvc             SNUserService
	supportEngineerRole string
	// customerRoles backs applyCustomerReplyStateTransition — see that
	// function's own doc comment and config.Config.CustomerRoles'. May be
	// empty (unconfigured), treated the same "can't confirm authorship,
	// skip" way supportEngineerRole == "" is.
	customerRoles []string
}

// NewSNCaseService constructs a CaseService that delegates SearchCases to the
// Choreo API and all write/read-by-id operations to pgFallback. publisher may
// be nil (see snCaseService.publisher's doc comment).
func NewServiceNowCaseService(client *integrationservice.Client, pgFallback CaseService, publisher EventPublisherService, slaClocks SLAClockService, userSvc SNUserService, supportEngineerRole string, customerRoles []string) CaseService {
	return &snCaseService{
		client:              client,
		pgFallback:          pgFallback,
		publisher:           publisher,
		slaClocks:           slaClocks,
		userSvc:             userSvc,
		supportEngineerRole: supportEngineerRole,
		customerRoles:       customerRoles,
	}
}

// snIssueTypeID maps domain CaseIssueType to the ServiceNow issue-type choice-list value.
var snIssueTypeID = map[domain.CaseIssueType]int{
	domain.CaseIssueTypeTotalOutage:            1,
	domain.CaseIssueTypePartialOutage:          2,
	domain.CaseIssueTypePerformanceDegradation: 3,
	domain.CaseIssueTypeQuestion:               4,
	domain.CaseIssueTypeSecurityOrCompliance:   5,
	domain.CaseIssueTypeError:                  6,
}

type snCreateCasePayload struct {
	Type                  string             `json:"type"`
	ProjectID             string             `json:"projectId"`
	DeploymentID          string             `json:"deploymentId,omitempty"`
	DeployedProductID     string             `json:"deployedProductId,omitempty"`
	Title                 string             `json:"title,omitempty"`
	Description           string             `json:"description,omitempty"`
	SeverityKey           int                `json:"severityKey,omitempty"`
	IssueTypeKey          int                `json:"issueTypeKey,omitempty"`
	EngagementType        int                `json:"engagementType,omitempty"`
	EngagementPaymentType int                `json:"engagementPaymentType,omitempty"`
	CatalogID             string             `json:"catalogId,omitempty"`
	CatalogItemID         string             `json:"catalogItemId,omitempty"`
	Variables             []snCaseVariable   `json:"variables,omitempty"`
	RelatedCaseID         string             `json:"relatedCaseId,omitempty"`
	ConversationID        string             `json:"conversationId,omitempty"`
	WatchList             []string           `json:"watchList,omitempty"`
	Attachments           []snCaseAttachment `json:"attachments,omitempty"`
}

type snCaseVariable struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

type snCaseAttachment struct {
	Name string `json:"name"`
	File string `json:"file"`
}

type snCreateCaseResponse struct {
	Message string `json:"message"`
	Case    struct {
		ID         string       `json:"id"`
		InternalID string       `json:"internalId"`
		Number     string       `json:"number"`
		CreatedBy  string       `json:"createdBy"`
		CreatedOn  string       `json:"createdOn"`
		State      *snCaseState `json:"state"`
	} `json:"case"`
}

func (s *snCaseService) CreateCase(ctx context.Context, req domain.CreateCaseRequest) (domain.CreateCaseResponse, error) {
	if err := validateCreateCaseRequest(&req); err != nil {
		return domain.CreateCaseResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	snType, ok := snCaseTypeMap[req.Type]
	if !ok {
		return domain.CreateCaseResponse{}, &apierror.ValidationError{Msg: "type contains invalid value: " + req.Type}
	}

	if err := validateUUIDs("projectId", []string{req.ProjectID}); err != nil {
		return domain.CreateCaseResponse{}, err
	}
	// Announcements have no deployment/deployed-product concept, so these
	// fields are left empty rather than validated as UUIDs.
	if req.Type != "announcement" {
		if err := validateUUIDs("deploymentId", []string{req.DeploymentID}); err != nil {
			return domain.CreateCaseResponse{}, err
		}
		if err := validateUUIDs("deployedProductId", []string{req.DeployedProductID}); err != nil {
			return domain.CreateCaseResponse{}, err
		}
	}

	payload := snCreateCasePayload{
		Type:              snType,
		ProjectID:         uuidToSysid(req.ProjectID),
		DeploymentID:      uuidToSysid(req.DeploymentID),
		DeployedProductID: uuidToSysid(req.DeployedProductID),
	}

	switch req.Type {
	case "case":
		payload.Title = req.Subject
		payload.Description = req.Description
		payload.SeverityKey = snSeverityIDMap[req.Severity]
		payload.IssueTypeKey = snIssueTypeID[req.IssueType]
	case "service_request":
		if err := validateUUIDs("catalogId", []string{req.CatalogID}); err != nil {
			return domain.CreateCaseResponse{}, err
		}
		if err := validateUUIDs("catalogItemId", []string{req.CatalogItemID}); err != nil {
			return domain.CreateCaseResponse{}, err
		}
		payload.CatalogID = uuidToSysid(req.CatalogID)
		payload.CatalogItemID = uuidToSysid(req.CatalogItemID)
		if len(req.Variables) > 0 {
			vars := make([]snCaseVariable, 0, len(req.Variables))
			for i, v := range req.Variables {
				if err := validateUUIDs(fmt.Sprintf("variables[%d].id", i), []string{v.ID}); err != nil {
					return domain.CreateCaseResponse{}, err
				}
				vars = append(vars, snCaseVariable{ID: uuidToSysid(v.ID), Value: v.Value})
			}
			payload.Variables = vars
		}
	case "security_report_analysis":
		payload.Title = req.Subject
		payload.Description = req.Description
		if len(req.Attachments) > 0 {
			atts := make([]snCaseAttachment, 0, len(req.Attachments))
			for _, a := range req.Attachments {
				atts = append(atts, snCaseAttachment{Name: a.Name, File: a.File})
			}
			payload.Attachments = atts
		}
	case "engagement":
		payload.Title = req.Subject
		payload.Description = req.Description
		payload.EngagementType = snEngagementTypeIDMap[req.EngagementType]
		payload.EngagementPaymentType = snEngagementPaymentTypeIDMap[req.EngagementPaymentType]
	case "announcement":
		payload.Title = req.Subject
		payload.Description = req.Description
	}

	if len(req.WatchList) > 0 {
		// The backing service's case-create payload declares the watch list as
		// email addresses, not user ids, so the incoming platform UUIDs are
		// resolved to emails first.
		emails, err := watchListEmails(ctx, s.client, token, "watchList", req.WatchList)
		if err != nil {
			return domain.CreateCaseResponse{}, err
		}
		payload.WatchList = emails
	}
	if req.RelatedCaseID != "" {
		if err := validateUUIDs("relatedCaseId", []string{req.RelatedCaseID}); err != nil {
			return domain.CreateCaseResponse{}, err
		}
		payload.RelatedCaseID = uuidToSysid(req.RelatedCaseID)
	}
	if req.ConversationID != "" {
		if err := validateUUIDs("conversationId", []string{req.ConversationID}); err != nil {
			return domain.CreateCaseResponse{}, err
		}
		payload.ConversationID = uuidToSysid(req.ConversationID)
	}

	raw, err := s.client.Post(ctx, "/cases", token, payload)
	if err != nil {
		return domain.CreateCaseResponse{}, err
	}

	var snResp snCreateCaseResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.CreateCaseResponse{}, fmt.Errorf("sn create case: parse response: %w", err)
	}

	createdOn, err := parseSNDateTime(ctx, "sn create case", "createdOn", snResp.Case.CreatedOn)
	if err != nil {
		return domain.CreateCaseResponse{}, fmt.Errorf("sn create case: parse createdOn %q: %w", snResp.Case.CreatedOn, err)
	}

	stateLabel := ""
	if snResp.Case.State != nil {
		stateLabel = snResp.Case.State.Label
	}

	resp := domain.CreateCaseResponse{
		Message: snResp.Message,
		Case: domain.CreateCaseDetails{
			ID:         sysidToUUID(snResp.Case.ID),
			InternalID: snResp.Case.InternalID,
			Number:     snResp.Case.Number,
			CreatedBy:  snResp.Case.CreatedBy,
			CreatedOn:  createdOn,
			State:      stateLabel,
		},
	}
	s.publishCaseCreated(ctx, req, resp.Case.ID)
	return resp, nil
}

// publishCaseCreated best-effort publishes a case.created event for a newly
// created case. It re-fetches the case via GetCaseByID rather than building
// the payload from snCreateCaseResponse/req alone: the create response
// carries only a handful of fields (see snCreateCaseResponse), while
// GetCaseByID's own SN response already resolves the reporter's display name,
// the project's name, and each watcher's email — exactly what
// events.CaseCreatedPayload needs and req/snCreateCaseResponse don't have.
//
// Recipients is the case's WatchList emails only (per explicit decision —
// this service has no other notion of "who should be emailed" for a case).
// A case created with no watchers is a real, expected state (watchers are
// often added after creation), not an error — publishing is silently skipped
// rather than sending a payload csm-notification-service's events.Validate
// would reject anyway for an empty recipients list.
//
// Runs synchronously (not detached/async like apps/csm-portal/backend's own
// publishAsync) so no goroutine-draining hook is needed on this service's
// shutdown path — publishCaseCreatedTimeout bounds the added latency instead.
// Any failure (enrichment or publish) is logged and does not fail CreateCase
// itself: the case already exists in ServiceNow by this point, and a
// notification-side hiccup must not be reported to the caller as a failed
// case creation.
func (s *snCaseService) publishCaseCreated(ctx context.Context, req domain.CreateCaseRequest, caseID string) {
	if s.publisher == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, publishCaseCreatedTimeout)
	defer cancel()

	cv, err := s.GetCaseByID(ctx, caseID)
	if err != nil {
		// Not logging err itself: it can carry a raw ServiceNow response
		// (potentially including response-body content), and this service's
		// own convention is to log only ids and sanitised summaries (see
		// CLAUDE.md's Security section).
		slog.ErrorContext(ctx, "sn create case: enrich case for case.created publish failed", "caseId", caseID)
		return
	}

	// SLA-clock registration runs here — sharing this function's own
	// s.publisher==nil guard (registration is inherently Kafka-based, same
	// as every publish in this file) and its GetCaseByID fetch above — but
	// deliberately BEFORE the watcher-count check below: that check only
	// gates the case.created *email*, and SLA tracking must happen
	// regardless of whether the case has watchers to notify.
	s.publishSLAClockRegister(ctx, cv, req, caseID)

	recipients := watchListUserEmails(cv.WatchList)
	if len(recipients) == 0 {
		slog.InfoContext(ctx, "sn create case: case.created not published, case has no watchers to email", "caseId", caseID)
		return
	}

	reporterName := ""
	if cv.CreatedBy != nil {
		reporterName = cv.CreatedBy.Name
	}
	product := caseProductName(cv)

	payload, err := json.Marshal(events.CaseCreatedPayload{
		ReporterName: reporterName,
		ProjectName:  cv.ProjectDetails.Name,
		ProjectID:    cv.ProjectDetails.ID,
		CaseID:       caseID,
		CaseNumber:   cv.Number,
		WSO2CaseID:   cv.InternalID,
		CaseTitle:    cv.Subject,
		CaseType:     strings.ToUpper(req.Type),
		Priority:     strings.ToUpper(string(cv.Severity)),
		Product:      product,
		Team:         caseTeamName(cv),
		CreatedAt:    cv.CreatedOn.Format(time.RFC3339),
		Description:  cv.Description,
		Recipients:   recipients,
	})
	if err != nil {
		slog.ErrorContext(ctx, "sn create case: encode case.created payload failed", "caseId", caseID, "error", err)
		return
	}
	if err := s.publisher.Publish(ctx, events.TypeCaseCreated, caseID, payload); err != nil {
		// Not logging err itself — see publishIncidentCreated's matching log
		// line for why (same reasoning: a raw Event Hub client error, and
		// the full error is already durably recorded in
		// event_publish_failures by Publish itself).
		slog.ErrorContext(ctx, "sn create case: publish case.created failed", "caseId", caseID)
	}
}

// publishSLAClockRegister best-effort publishes sla.clock.register for a
// newly created case's applicable SLA clocks — called from
// publishCaseCreated (see its own doc comment for why this runs before that
// function's watcher-count check, sharing its s.publisher==nil guard and
// its GetCaseByID enrichment instead of a second fetch).
//
// Which clock types get registered, and their durations, come from
// sla_policy.go's slaDurations, keyed by cv.Severity — LOW severity's entry
// only has a "response" duration (WSO2's support policy defines no fixed
// Workaround/Resolution SLA for it, "best efforts"), so only that one clock
// gets registered for a LOW-severity case. A severity with no policy entry
// at all (shouldn't happen given domain.CaseSeverity's own fixed set, but
// defensively handled rather than assumed) skips publishing entirely,
// logged as a warning rather than silently registering nothing.
//
// Durations are encoded as Go duration strings (time.Duration.String(),
// e.g. "24h0m0s") — csm-notification-service's slaengine parses them back
// via time.ParseDuration and adds them to CaseCreatedAt (the case's actual
// creation time, not publish/consume-time "now" — see
// events.SLAClockRegisterPayload.CaseCreatedAt's own doc comment) to
// compute each clock's actual due timestamp; this function never computes
// an absolute due time itself. AvoidWeekendDueDate carries sla_policy.go's
// slaAvoidWeekendClockTypes for cv.Severity — see that map's own doc
// comment for what it's for.
//
// Like every other publish in this file, a failed Publish call below is
// only durably recorded in event_publish_failures (searchable/resolvable),
// not automatically retried or reconciled — see
// EventPublisherService.Publish's own KNOWN GAP doc comment. That's an
// existing, accepted limitation shared by every case.*/incident.* event
// this service publishes, not something specific to SLA registration.
func (s *snCaseService) publishSLAClockRegister(ctx context.Context, cv domain.CaseView, req domain.CreateCaseRequest, caseID string) {
	durations, ok := slaDurations[cv.Severity]
	if !ok || len(durations) == 0 {
		slog.WarnContext(ctx, "sn create case: sla.clock.register not published, no SLA duration policy for severity", "caseId", caseID, "severity", cv.Severity)
		return
	}
	durationStrings := make(map[string]string, len(durations))
	for clockType, d := range durations {
		durationStrings[clockType] = d.String()
	}

	payload, err := json.Marshal(events.SLAClockRegisterPayload{
		CaseID:              caseID,
		Durations:           durationStrings,
		CaseCreatedAt:       cv.CreatedOn.Format(time.RFC3339),
		AvoidWeekendDueDate: slaAvoidWeekendClockTypes[cv.Severity],
		CaseNumber:          cv.Number,
		WSO2CaseID:          cv.InternalID,
		CaseTitle:           cv.Subject,
		CaseType:            strings.ToUpper(req.Type),
		Product:             caseProductName(cv),
		Team:                caseTeamName(cv),
		Priority:            strings.ToUpper(string(cv.Severity)),
		State:               strings.ToUpper(string(cv.State)),
	})
	if err != nil {
		slog.ErrorContext(ctx, "sn create case: encode sla.clock.register payload failed", "caseId", caseID, "error", err)
		return
	}
	if err := s.publisher.Publish(ctx, events.TypeSLAClockRegister, caseID, payload); err != nil {
		// Not logging err itself — see publishCaseCreated's matching log
		// line for why.
		slog.ErrorContext(ctx, "sn create case: publish sla.clock.register failed", "caseId", caseID)
	}
}

// resolveCommentAuthorSearchLimit bounds resolveCommentAuthor's lookup —
// the new comment is essentially certain to be within this many of the
// case's most recent comments regardless of SearchCaseComments' own sort
// order (undocumented, not controllable by this service), since it was
// just created moments before this call runs.
const resolveCommentAuthorSearchLimit = 20

// resolveCommentAuthor looks up commentID's resolved author (name, email,
// and — when ServiceNow's comment record carried one — id) via
// SearchCaseComments — see publishCommentAdded's doc comment for why this
// re-fetch is needed at all. Returns nil if the comment isn't found in the
// first resolveCommentAuthorSearchLimit results, or if the search itself
// fails; either way the caller logs and skips its own reaction rather than
// proceeding with a fabricated or missing author. Used by both
// publishCommentAdded (author display name, for the comment-added email)
// and applyResponseSLAOnComment (author email, to look up their
// ServiceNow role).
func (s *snCaseService) resolveCommentAuthor(ctx context.Context, caseID, commentID string) *domain.UserReference {
	pagination := domain.Pagination{Limit: resolveCommentAuthorSearchLimit}
	if err := normalizePagination(&pagination); err != nil {
		return nil
	}
	resp, err := s.SearchCaseComments(ctx, domain.SearchCaseCommentsRequest{
		CaseID:     caseID,
		Pagination: pagination,
	})
	if err != nil {
		return nil
	}
	for _, c := range resp.Comments {
		if c.ID == commentID && c.CreatedBy != nil {
			return c.CreatedBy
		}
	}
	return nil
}

// publishCommentAdded best-effort publishes a case.comment_added event
// after a new comment is created. Recipients is the case's WatchList
// emails only — see publishCaseCreated's own doc comment for why, and why
// an empty list silently skips publishing rather than sending a payload
// csm-notification-service's events.Validate would reject anyway. When
// req.Type is CommentTypeWorkNote (an internal note — never meant for a
// customer to see), Recipients is filtered down to wso2.com addresses
// only via filterWso2Emails, regardless of who else is on the case's
// watch list.
//
// events.CommentAddedPayload.Name requires the comment author's resolved
// display name, which snCreateCommentResponse doesn't carry (only a raw
// CreatedBy string, unresolved — see snCreateCommentResponse) — every
// other place in this file that needs a resolved author name gets it from
// a GET/search response, never a bare create-acknowledgment response, so
// this re-fetches via resolveCommentAuthorName (SearchCaseComments)
// instead of trusting the create response, mirroring publishCaseCreated's
// own "re-fetch rather than trust the create response" precedent. If the
// author name can't be resolved that way, publishing is skipped (logged)
// rather than sending an event with an empty or fabricated name — see
// resolveCommentAuthorName's own doc comment for when that happens.
//
// Runs synchronously, bounded by publishCommentAddedTimeout — see
// publishCaseCreated's own doc comment for why (same reasoning).
func (s *snCaseService) publishCommentAdded(ctx context.Context, req domain.CreateCaseCommentRequest, commentID string) {
	if s.publisher == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, publishCommentAddedTimeout)
	defer cancel()

	cv, err := s.GetCaseByID(ctx, req.CaseID)
	if err != nil {
		slog.ErrorContext(ctx, "sn create comment: enrich case for case.comment_added publish failed", "caseId", req.CaseID)
		return
	}

	recipients := watchListUserEmails(cv.WatchList)
	if req.Type == domain.CommentTypeWorkNote {
		recipients = filterWso2Emails(recipients)
	}
	if len(recipients) == 0 {
		slog.InfoContext(ctx, "sn create comment: case.comment_added not published, case has no watchers to email", "caseId", req.CaseID)
		return
	}

	author := s.resolveCommentAuthor(ctx, req.CaseID, commentID)
	if author == nil || author.Name == "" {
		slog.InfoContext(ctx, "sn create comment: case.comment_added not published, could not resolve comment author's display name", "caseId", req.CaseID)
		return
	}

	payload, err := json.Marshal(events.CommentAddedPayload{
		Name:           author.Name,
		ProjectID:      cv.ProjectDetails.ID,
		CaseID:         req.CaseID,
		CaseNumber:     cv.Number,
		WSO2CaseID:     cv.InternalID,
		CaseTitle:      cv.Subject,
		CaseComment:    req.Content,
		CommentID:      commentID,
		IsInternalNote: req.Type == domain.CommentTypeWorkNote,
		Recipients:     recipients,
	})
	if err != nil {
		slog.ErrorContext(ctx, "sn create comment: encode case.comment_added payload failed", "caseId", req.CaseID, "error", err)
		return
	}
	if err := s.publisher.Publish(ctx, events.TypeCommentAdded, req.CaseID, payload); err != nil {
		// Not logging err itself — see publishCaseCreated's matching log
		// line for why.
		slog.ErrorContext(ctx, "sn create comment: publish case.comment_added failed", "caseId", req.CaseID)
	}
}

// applyResponseSLAOnComment marks the case's "response" SLA clock complete
// (all three tiers — 50/75/100 — claimed at once via
// SetSLAClockTierReached, idempotently) when the new comment is a
// customer-visible reply (req.Type == domain.CommentTypeComment — work
// notes and system activity entries don't count as a response) from a
// user holding s.supportEngineerRole.
//
// This is a pure in-process DB operation, deliberately NOT gated on
// s.publisher — unlike every publishXxx function in this file, it never
// touches Event Hub at all, so a deployment without Event Hub configured
// must not lose SLA tracking as a side effect. Claiming all three tiers at
// once (not just "100") is what suppresses a later, spurious breach alert:
// when csm-notification-service's slaengine eventually reaches the
// wake-index entries this clock's registration created for 50%/75%/100%
// elapsed, its own SetTierReachedIfUnset call will see each already
// claimed and skip publishing — the exact same alreadyReached mechanism
// that already prevents a duplicate real breach alert, reused here for
// "this was satisfied early, not breached" instead. Calling this on every
// qualifying comment (not just literally the first) is intentional and
// harmless: SetSLAClockTierReached is itself idempotent, so only the
// first call for a given tier actually claims it.
//
// entity-service has no auth/identity layer of its own (the
// x-user-id-token it forwards is opaque), so "is this comment's author a
// support engineer" can't be answered from anything in this request — it's
// answered by resolving the comment's author (resolveCommentAuthor, the
// same lookup publishCommentAdded already needs for its own display name)
// and checking their ServiceNow role via s.userSvc.SearchUsers, filtered
// by the author's email. s.supportEngineerRole being "" (unconfigured — see
// config.Config.SupportEngineerRole's own doc comment) means this can
// never be confirmed, so this skips entirely rather than guessing.
func (s *snCaseService) applyResponseSLAOnComment(ctx context.Context, req domain.CreateCaseCommentRequest, commentID string) {
	if req.Type != domain.CommentTypeComment || s.supportEngineerRole == "" || s.slaClocks == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, applyResponseSLATimeout)
	defer cancel()

	author := s.resolveCommentAuthor(ctx, req.CaseID, commentID)
	if author == nil || author.Email == "" {
		slog.InfoContext(ctx, "sn create comment: response SLA not evaluated, could not resolve comment author's email", "caseId", req.CaseID)
		return
	}

	usersResp, err := s.userSvc.SearchUsers(ctx, domain.SearchUsersRequest{
		Pagination: domain.Pagination{Limit: 1},
		Filters:    domain.SearchUsersFilters{Emails: []string{author.Email}},
	})
	if err != nil {
		slog.ErrorContext(ctx, "sn create comment: response SLA not evaluated, user role lookup failed", "caseId", req.CaseID)
		return
	}

	isSupportEngineer := false
	for _, u := range usersResp.Users {
		if slices.Contains(u.Roles, s.supportEngineerRole) {
			isSupportEngineer = true
			break
		}
	}
	if !isSupportEngineer {
		return
	}

	for _, tier := range []string{"50", "75", "100"} {
		if _, err := s.slaClocks.SetSLAClockTierReached(ctx, req.CaseID, slaClockTypeResponse, tier, domain.SetSLAClockTierRequest{Status: domain.SLATierStatusReached}); err != nil {
			logSLAClockOpFailed(ctx, "sn create comment: mark response sla clock tier reached failed", req.CaseID, slaClockTypeResponse, err)
		}
	}
}

// applyCustomerReplyStateTransition moves a case back to Waiting on WSO2
// when a customer replies while it's Awaiting Info or Solution Proposed —
// WSO2 was waiting on the customer, and a reply means it's WSO2's turn to
// act again. A pure in-process call to s.UpdateCase (not a raw ServiceNow
// PATCH of its own), so it gets publishStatusChanged/applyCaseStateSLAEffects
// for free — in particular, applyCaseStateSLAEffects' own "any state other
// than Awaiting Info/Solution Proposed/Closed resumes both Workaround and
// Resolution" default case is exactly the right side effect here, with no
// duplicated logic. Calling UpdateCase with a State equal to the case's
// current one (a race with some other concurrent state change) is a
// harmless no-op there — see UpdateCase's own pre-PATCH equality check.
//
// KNOWN GAP: the read here (this function's own GetCaseByID) and the write
// (the UpdateCase call below, which does its own separate GetCaseByID
// purely to decide whether to publish case.status_changed — see that
// function's own pre-PATCH block) are not atomic. If the case is moved to
// some OTHER state (e.g. Closed) in the window between this function's read
// and UpdateCase's PATCH, this still unconditionally sends
// State: WaitingOnWSO2 — silently reopening a case that was just closed,
// and resuming SLA clocks applyCaseStateSLAEffects had just paused for
// Closed. This is not unique to this function: every UpdateCase caller that
// sets State/Severity/AssigneeEmail (publishStatusChanged/
// publishSeverityChanged/publishCaseAssigned's own pre-PATCH guards) has the
// identical read-then-PATCH race window, since ServiceNow is this service's
// sole source of truth (no local row/version to condition on) and
// s.client.Patch has no optimistic-concurrency mechanism (no ETag/version/
// sys_mod_count precondition) to send even if this function wanted one.
// Closing this needs the underlying Choreo/ServiceNow integration to expose
// a conditional update — a real, cross-team dependency, not a quick fix
// here, so it's flagged rather than worked around with a partial guard that
// wouldn't close the actual window anyway.
//
// Requires its own GetCaseByID call: nothing in CreateCaseComment's own flow
// surfaces the case's current state today (publishCommentAdded fetches one
// for its own, separate purpose, but never returns or shares it, and is
// itself skipped when s.publisher is nil — not something this can rely on).
//
// Same role-lookup mechanism as applyResponseSLAOnComment (this service has
// no auth/identity layer of its own, so "is this comment's author a
// customer" is answered by resolving the author and checking their
// ServiceNow role via s.userSvc.SearchUsers), but against a configurable
// LIST of roles (s.customerRoles, config.Config.CustomerRoles) rather than
// a single one — an organisation can have more than one customer-facing
// role. Skips entirely, rather than guessing, when s.customerRoles is empty
// (unconfigured — see config.Config.CustomerRoles' own doc comment).
func (s *snCaseService) applyCustomerReplyStateTransition(ctx context.Context, req domain.CreateCaseCommentRequest, commentID string) {
	if req.Type != domain.CommentTypeComment || len(s.customerRoles) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, applyCustomerReplyTimeout)
	defer cancel()

	cv, err := s.GetCaseByID(ctx, req.CaseID)
	if err != nil {
		slog.ErrorContext(ctx, "sn create comment: customer reply state transition not evaluated, get case failed", "caseId", req.CaseID)
		return
	}
	if cv.State != domain.CaseStateAwaitingInfo && cv.State != domain.CaseStateSolutionProposed {
		return
	}

	author := s.resolveCommentAuthor(ctx, req.CaseID, commentID)
	if author == nil || author.Email == "" {
		slog.InfoContext(ctx, "sn create comment: customer reply state transition not evaluated, could not resolve comment author's email", "caseId", req.CaseID)
		return
	}

	usersResp, err := s.userSvc.SearchUsers(ctx, domain.SearchUsersRequest{
		Pagination: domain.Pagination{Limit: 1},
		Filters:    domain.SearchUsersFilters{Emails: []string{author.Email}},
	})
	if err != nil {
		slog.ErrorContext(ctx, "sn create comment: customer reply state transition not evaluated, user role lookup failed", "caseId", req.CaseID)
		return
	}

	isCustomer := false
	for _, u := range usersResp.Users {
		if slices.ContainsFunc(u.Roles, func(r string) bool { return slices.Contains(s.customerRoles, r) }) {
			isCustomer = true
			break
		}
	}
	if !isCustomer {
		return
	}

	waitingOnWSO2 := domain.CaseStateWaitingOnWSO2
	if _, err := s.UpdateCase(ctx, domain.UpdateCaseRequest{ID: req.CaseID, State: &waitingOnWSO2}); err != nil {
		slog.ErrorContext(ctx, "sn create comment: move case to waiting on wso2 after customer reply failed", "caseId", req.CaseID)
	}
}

// publishStatusChanged best-effort publishes a case.status_changed event
// after UpdateCase changes a case's state. newStatus is the raw SN state
// label (e.g. "Open", "Work In Progress") rather than domain.CaseState's
// own enum conversion — mirrors CreateCase's own stateLabel handling, and
// is always available whenever snResp.Case.State is non-nil, unlike the
// enum conversion (snCaseStateLabelToEnum), which silently leaves the
// domain value unset on a label it doesn't recognize.
//
// before is the case as it was fetched by UpdateCase *before* issuing the
// PATCH — the caller has already used it to confirm the case's state is
// actually transitioning (a caller re-PATCHing the current state must not
// trigger a false "status changed" notification to every watcher; see
// UpdateCase's own comment at that fetch), and this reuses the exact same
// enrichment for Recipients/ProjectID rather than issuing a second
// GetCaseByID call after the PATCH: neither value depends on the update
// that just happened.
//
// Recipients is the case's WatchList emails only — see publishCaseCreated's
// own doc comment for why, and why an empty list silently skips
// publishing.
//
// Runs synchronously, bounded by publishStatusChangedTimeout — see
// publishCaseCreated's own doc comment for why (same reasoning).
func (s *snCaseService) publishStatusChanged(ctx context.Context, caseID, newStatus string, before domain.CaseView) {
	if s.publisher == nil || newStatus == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, publishStatusChangedTimeout)
	defer cancel()

	recipients := watchListUserEmails(before.WatchList)
	if len(recipients) == 0 {
		slog.InfoContext(ctx, "sn update case: case.status_changed not published, case has no watchers to email", "caseId", caseID)
		return
	}

	payload, err := json.Marshal(events.StatusChangedPayload{
		ProjectID:  before.ProjectDetails.ID,
		CaseID:     caseID,
		CaseNumber: before.Number,
		WSO2CaseID: before.InternalID,
		CaseTitle:  before.Subject,
		NewStatus:  newStatus,
		Recipients: recipients,
	})
	if err != nil {
		slog.ErrorContext(ctx, "sn update case: encode case.status_changed payload failed", "caseId", caseID, "error", err)
		return
	}
	if err := s.publisher.Publish(ctx, events.TypeStatusChanged, caseID, payload); err != nil {
		// Not logging err itself — see publishCaseCreated's matching log
		// line for why.
		slog.ErrorContext(ctx, "sn update case: publish case.status_changed failed", "caseId", caseID)
	}
}

// logSLAClockOpFailed logs an SLA clock mutation's failure — at Info, not
// Error, when it's a *apierror.NotFoundError: that specifically means no
// such clock was ever registered for this case (e.g. a LOW-severity case,
// which never gets a "workaround"/"resolution" clock at all — see
// sla_policy.go's slaDurations), an expected, harmless outcome on every
// state transition for such a case, not a real failure worth alerting on.
// Anything else logs at Error, matching every other best-effort publish
// helper's own failure logging in this file.
func logSLAClockOpFailed(ctx context.Context, msg, caseID, clockType string, err error) {
	var notFound *apierror.NotFoundError
	if errors.As(err, &notFound) {
		slog.InfoContext(ctx, msg+": no such sla clock registered", "caseId", caseID, "clockType", clockType)
		return
	}
	slog.ErrorContext(ctx, msg, "caseId", caseID, "clockType", clockType)
}

// applyCaseStateSLAEffects pauses/resumes/completes the case's "workaround"
// and "resolution" SLA clocks in reaction to a state-changing PATCH — a
// pure in-process DB operation, deliberately independent of
// publishStatusChanged/s.publisher (see this function's call site in
// UpdateCase for why: pause/resume/completion must keep working even in a
// deployment that hasn't configured Event Hub, since nothing here actually
// needs it). Skips entirely when s.slaClocks is nil — a deployment running
// DATA_SOURCE=servicenow with no database configured (see
// config.Config.HasDatabase) has nowhere to store SLA clocks at all.
//
//   - CaseStateAwaitingInfo/CaseStateSolutionProposed: pause both clocks —
//     the case is waiting on the customer, not actively being worked, so
//     neither should keep counting toward a breach.
//   - CaseStateClosed: resume both (so paused_at doesn't stay stuck at a
//     non-null value on a row that's now done), then complete "resolution"
//     the same way applyResponseSLAOnComment completes "response" — claim
//     all three tiers (50/75/100) at once via SetSLAClockTierReached, which
//     suppresses any later wake-index entry for it from firing a spurious
//     breach alert (see that function's own doc comment for the mechanism).
//     "workaround" is only paused here, not completed:
//     TODO: workaround SLA has no completion trigger wired up yet — it
//     needs a "workaround provided" signal that doesn't exist anywhere in
//     this domain model today, distinct from the case simply closing.
//     Pausing on close is a stopgap so it stops counting/alerting past
//     closure, not a substitute for real completion.
//   - Anything else (e.g. back to CaseStateWorkInProgress): resume both —
//     the case is active again.
//
// Every step is idempotent (Pause/Resume/SetSLAClockTierReached all are),
// so no no-op pre-check is needed here: a caller re-PATCHing the case's own
// current state just redundantly re-applies the same effect.
func (s *snCaseService) applyCaseStateSLAEffects(ctx context.Context, caseID string, state domain.CaseState) {
	if s.slaClocks == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, applyCaseStateSLATimeout)
	defer cancel()

	switch state {
	case domain.CaseStateAwaitingInfo, domain.CaseStateSolutionProposed:
		if _, err := s.slaClocks.Pause(ctx, caseID, slaClockTypeWorkaround); err != nil {
			logSLAClockOpFailed(ctx, "sn update case: pause workaround sla clock failed", caseID, slaClockTypeWorkaround, err)
		}
		if _, err := s.slaClocks.Pause(ctx, caseID, slaClockTypeResolution); err != nil {
			logSLAClockOpFailed(ctx, "sn update case: pause resolution sla clock failed", caseID, slaClockTypeResolution, err)
		}
	case domain.CaseStateClosed:
		if _, err := s.slaClocks.Resume(ctx, caseID, slaClockTypeResolution); err != nil {
			logSLAClockOpFailed(ctx, "sn update case: resume resolution sla clock failed", caseID, slaClockTypeResolution, err)
		}
		for _, tier := range []string{"50", "75", "100"} {
			if _, err := s.slaClocks.SetSLAClockTierReached(ctx, caseID, slaClockTypeResolution, tier, domain.SetSLAClockTierRequest{Status: domain.SLATierStatusReached}); err != nil {
				logSLAClockOpFailed(ctx, "sn update case: mark resolution sla clock tier reached failed", caseID, slaClockTypeResolution, err)
			}
		}
		// workaround: paused, not completed — see this function's own doc
		// comment TODO above.
		if _, err := s.slaClocks.Pause(ctx, caseID, slaClockTypeWorkaround); err != nil {
			logSLAClockOpFailed(ctx, "sn update case: pause workaround sla clock failed", caseID, slaClockTypeWorkaround, err)
		}
	default:
		if _, err := s.slaClocks.Resume(ctx, caseID, slaClockTypeWorkaround); err != nil {
			logSLAClockOpFailed(ctx, "sn update case: resume workaround sla clock failed", caseID, slaClockTypeWorkaround, err)
		}
		if _, err := s.slaClocks.Resume(ctx, caseID, slaClockTypeResolution); err != nil {
			logSLAClockOpFailed(ctx, "sn update case: resume resolution sla clock failed", caseID, slaClockTypeResolution, err)
		}
	}
}

// publishSeverityChanged best-effort publishes a case.severity_changed
// event after UpdateCase changes a case's severity — called only when the
// PATCH's own req.Severity was set AND actually differs from the case's
// prior severity (the same no-op guard as publishStatusChanged's own call
// site: a caller re-PATCHing the case's current severity must not send
// every watcher a false "severity changed" notification). Unlike
// publishCaseAcknowledged, this has both an email reaction (Recipients,
// same watch-list-emails audience as publishStatusChanged/
// publishCaseAssigned) and a Google Chat alert (Product, same
// caseProductName(before) reasoning as publishCaseCreated/
// publishCaseAcknowledged) — csm-notification-service's dispatch package
// decides how to route each.
//
// before is the case as it was fetched by UpdateCase *before* issuing the
// PATCH — same reuse-the-enrichment reasoning as publishStatusChanged's own
// doc comment.
//
// Recipients is the case's WatchList emails only — see publishCaseCreated's
// own doc comment for why, and why an empty list silently skips publishing
// (both the email AND the Chat alert — csm-notification-service has no
// separate "chat only, no recipients" event type for this the way
// case.acknowledged is; a severity change with no watchers has nobody to
// notify by design).
//
// Runs synchronously, bounded by publishSeverityChangedTimeout — see
// publishCaseCreated's own doc comment for why (same reasoning).
func (s *snCaseService) publishSeverityChanged(ctx context.Context, caseID, oldSeverity, newSeverity string, before domain.CaseView) {
	if s.publisher == nil || newSeverity == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, publishSeverityChangedTimeout)
	defer cancel()

	recipients := watchListUserEmails(before.WatchList)
	if len(recipients) == 0 {
		slog.InfoContext(ctx, "sn update case: case.severity_changed not published, case has no watchers to email", "caseId", caseID)
		return
	}

	payload, err := json.Marshal(events.SeverityChangedPayload{
		ProjectID:   before.ProjectDetails.ID,
		CaseID:      caseID,
		CaseNumber:  before.Number,
		WSO2CaseID:  before.InternalID,
		CaseTitle:   before.Subject,
		OldSeverity: strings.ToUpper(oldSeverity),
		NewSeverity: strings.ToUpper(newSeverity),
		Product:     caseProductName(before),
		Team:        caseTeamName(before),
		Recipients:  recipients,
	})
	if err != nil {
		slog.ErrorContext(ctx, "sn update case: encode case.severity_changed payload failed", "caseId", caseID, "error", err)
		return
	}
	if err := s.publisher.Publish(ctx, events.TypeSeverityChanged, caseID, payload); err != nil {
		// Not logging err itself — see publishCaseCreated's matching log
		// line for why.
		slog.ErrorContext(ctx, "sn update case: publish case.severity_changed failed", "caseId", caseID)
	}
}

// publishCaseAssigned best-effort publishes a case.assigned event after
// UpdateCase changes a case's assignee. assigneeName/assigneeEmail
// identify who the case is now assigned *to* — not who performed the
// assignment: this service has no inbound identity layer (the
// x-user-id-token header it forwards is opaque, not decodable), so it has
// no way to resolve who made the request; the new assignee's own
// email is directly available on req.AssigneeEmail with no extra lookup,
// and assigneeName is ServiceNow's own resolved display name for that
// assignee from the PATCH response, falling back to the email if
// ServiceNow's response didn't carry one.
//
// before is the case as it was fetched by UpdateCase *before* issuing the
// PATCH — same reuse-the-enrichment reasoning as publishStatusChanged's
// own doc comment (the caller has already used it to confirm the assignee
// actually changed).
//
// Recipients is the case's WatchList emails only — see publishCaseCreated's
// own doc comment for why, and why an empty list silently skips
// publishing.
//
// Runs synchronously, bounded by publishCaseAssignedTimeout — see
// publishCaseCreated's own doc comment for why (same reasoning).
func (s *snCaseService) publishCaseAssigned(ctx context.Context, caseID, assigneeName, assigneeEmail string, before domain.CaseView) {
	if s.publisher == nil || assigneeEmail == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, publishCaseAssignedTimeout)
	defer cancel()

	recipients := watchListUserEmails(before.WatchList)
	if len(recipients) == 0 {
		slog.InfoContext(ctx, "sn update case: case.assigned not published, case has no watchers to email", "caseId", caseID)
		return
	}

	payload, err := json.Marshal(events.CaseAssignedPayload{
		AssigneeName:  assigneeName,
		AssigneeEmail: assigneeEmail,
		ProjectID:     before.ProjectDetails.ID,
		CaseID:        caseID,
		CaseNumber:    before.Number,
		WSO2CaseID:    before.InternalID,
		CaseTitle:     before.Subject,
		Recipients:    recipients,
	})
	if err != nil {
		slog.ErrorContext(ctx, "sn update case: encode case.assigned payload failed", "caseId", caseID, "error", err)
		return
	}
	if err := s.publisher.Publish(ctx, events.TypeCaseAssigned, caseID, payload); err != nil {
		// Not logging err itself — see publishCaseCreated's matching log
		// line for why.
		slog.ErrorContext(ctx, "sn update case: publish case.assigned failed", "caseId", caseID)
	}
}

// publishCaseAcknowledged best-effort publishes a case.acknowledged event
// after UpdateCase's Acknowledge path successfully claims the case for the
// first time — never for a repeat Acknowledge:true call that succeeded
// without changing anything (AlreadyAcknowledged true; see the call site's
// own comment). Chat-only: there is no email reaction for an
// acknowledgment, unlike every other publisher in this file, so there's no
// Recipients/watch-list concept here at all.
//
// Re-fetches the case via GetCaseByID rather than trusting the PATCH
// response for CaseNumber/WSO2CaseID/Severity — snUpdateCaseResponse's
// acknowledge path only ever echoes Number/AlreadyAcknowledged/
// AcknowledgedBy, none of which cover what csm-notification-service's Chat
// alert needs to display (mirrors publishCaseCreated's own "re-fetch
// rather than trust a narrow create/update response" precedent).
//
// Runs synchronously, bounded by publishCaseAcknowledgedTimeout — see
// publishCaseCreated's own doc comment for why (same reasoning).
func (s *snCaseService) publishCaseAcknowledged(ctx context.Context, caseID, acknowledgerName string) {
	if s.publisher == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, publishCaseAcknowledgedTimeout)
	defer cancel()

	cv, err := s.GetCaseByID(ctx, caseID)
	if err != nil {
		slog.ErrorContext(ctx, "sn update case: enrich case for case.acknowledged publish failed", "caseId", caseID)
		return
	}
	product := caseProductName(cv)

	payload, err := json.Marshal(events.CaseAcknowledgedPayload{
		CaseID:           caseID,
		CaseNumber:       cv.Number,
		WSO2CaseID:       cv.InternalID,
		Severity:         strings.ToUpper(string(cv.Severity)),
		Product:          product,
		Team:             caseTeamName(cv),
		AcknowledgerName: acknowledgerName,
	})
	if err != nil {
		slog.ErrorContext(ctx, "sn update case: encode case.acknowledged payload failed", "caseId", caseID, "error", err)
		return
	}
	if err := s.publisher.Publish(ctx, events.TypeCaseAcknowledged, caseID, payload); err != nil {
		// Not logging err itself — see publishCaseCreated's matching log
		// line for why.
		slog.ErrorContext(ctx, "sn update case: publish case.acknowledged failed", "caseId", caseID)
	}
}

func (s *snCaseService) GetCaseByID(ctx context.Context, id string) (domain.CaseView, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/cases/"+uuidToSysid(id), token)
	if err != nil {
		return domain.CaseView{}, err
	}

	var c snCase
	if err := json.Unmarshal(raw, &c); err != nil {
		return domain.CaseView{}, fmt.Errorf("sn get case: parse response: %w", err)
	}

	createdOn, err := parseSNDateTime(ctx, "sn get case", "createdOn", c.CreatedOn)
	if err != nil {
		return domain.CaseView{}, fmt.Errorf("sn get case: parse createdOn %q: %w", c.CreatedOn, err)
	}
	updatedOn := createdOn
	if c.UpdatedOn != nil && *c.UpdatedOn != "" {
		updatedOn, err = parseSNDateTime(ctx, "sn get case", "updatedOn", *c.UpdatedOn)
		if err != nil {
			return domain.CaseView{}, fmt.Errorf("sn get case: parse updatedOn %q: %w", *c.UpdatedOn, err)
		}
	}

	state, err := snCaseStateLabelToEnum(c.State)
	if err != nil {
		return domain.CaseView{}, fmt.Errorf("sn get case %q: %w", c.ID, err)
	}

	cv := domain.CaseView{
		ID:              sysidToUUID(c.ID),
		Number:          c.Number,
		InternalID:      c.InternalID,
		Subject:         c.Title,
		Duration:        c.Duration,
		EscalationLevel: snEscalationLevelToDomain(c.EscalationLevel),
		IsEscalated:     c.IsEscalated,
		Description:     c.Description,
		Severity:        snSeverityToSeverity(c.Severity),
		IssueType:       snIssueTypeToEnum(c.IssueType),
		State:           state,
		WorkState:       snWorkStateLabelToEnum(c.WorkState),
		Type:            snCaseTypeToDomain(c.CaseType),
		EngagementType:  snLabelStr(c.EngagementType),
		CreatedOn:       createdOn,
		UpdatedOn:       updatedOn,
		// The case read carries no id for the creator, only the email and full
		// name, so the canonical reference is emitted with a null id.
		CreatedBy:      domain.NewUserReference("", c.CreatedBy, c.CreatedByFullName),
		ProjectDetails: domain.EntityRef{ID: sysidToUUID(c.Project.ID), Name: c.Project.Name},
	}

	if depID := sysidToUUID(c.Deployment.ID); depID != "" {
		cv.DeploymentDetails = &domain.EntityRef{ID: depID, Name: c.Deployment.Name}
	}
	if dpID := sysidToUUID(c.DeployedProduct.ID); dpID != "" {
		displayName := strings.TrimSpace(c.DeployedProduct.Name + " " + c.DeployedProduct.Version)
		cv.DeployedProductDetails = &domain.DeployedProductRef{
			ID:          &dpID,
			DisplayName: &displayName,
		}
	}
	if c.Product != nil {
		if id := sysidToUUID(c.Product.ID); id != "" {
			// The catalogue product hangs off the deployed product. A case can
			// name one without naming a deployed instance, so emit the
			// reference with null id/displayName rather than dropping it.
			if cv.DeployedProductDetails == nil {
				cv.DeployedProductDetails = &domain.DeployedProductRef{}
			}
			cv.DeployedProductDetails.Product = &domain.EntityRef{ID: id, Name: c.Product.Name}
		}
	}
	if c.Catalog != nil {
		if id := sysidToUUID(c.Catalog.ID); id != "" {
			cv.Catalog = &domain.EntityRef{ID: id, Name: c.Catalog.Name}
		}
	}
	if c.CatalogItem != nil {
		if id := sysidToUUID(c.CatalogItem.ID); id != "" {
			cv.CatalogItem = &domain.EntityRef{ID: id, Name: c.CatalogItem.Name}
		}
	}
	if c.AssignedTeam != nil {
		if id := sysidToUUID(c.AssignedTeam.ID); id != "" {
			cv.AssignedTeam = &domain.EntityRef{ID: id, Name: c.AssignedTeam.Name}
		}
	}
	if c.Conversation != nil {
		if id := sysidToUUID(c.Conversation.ID); id != "" {
			cv.Conversation = &domain.EntityRef{ID: id, Name: c.Conversation.Name}
		}
	}
	if c.AssignedEngineer != nil {
		cv.AssignedEngineer = domain.NewUserReference(sysidToUUID(c.AssignedEngineer.ID), snStr(c.AssignedEngineer.Email), c.AssignedEngineer.Name)
	}
	if c.AcknowledgedBy != nil {
		cv.AcknowledgedBy = &domain.AssignedEngineerRef{ID: sysidToUUID(c.AcknowledgedBy.ID), Name: c.AcknowledgedBy.Name, Email: c.AcknowledgedBy.Email}
	}
	if c.WorkaroundProvidedOn != nil && *c.WorkaroundProvidedOn != "" {
		workaroundProvidedOn, err := parseSNDateTime(ctx, "sn get case", "workaroundProvidedOn", *c.WorkaroundProvidedOn)
		if err != nil {
			return domain.CaseView{}, fmt.Errorf("sn get case: parse workaroundProvidedOn %q: %w", *c.WorkaroundProvidedOn, err)
		}
		cv.WorkaroundProvidedOn = &workaroundProvidedOn
	}
	if c.WorkaroundProvidedBy != nil {
		cv.WorkaroundProvidedBy = &domain.AssignedEngineerRef{ID: sysidToUUID(c.WorkaroundProvidedBy.ID), Name: c.WorkaroundProvidedBy.Name, Email: c.WorkaroundProvidedBy.Email}
	}
	if c.ParentCase != nil {
		cv.ParentCase = &domain.CaseNumberRef{ID: sysidToUUID(c.ParentCase.ID), Number: c.ParentCase.Number, Type: snParentCaseTypeToDomain(c.ParentCase.Type)}
	}
	if c.RelatedCase != nil {
		cv.RelatedCase = &domain.CaseNumberRef{ID: sysidToUUID(c.RelatedCase.ID), Number: c.RelatedCase.Number, Type: snParentCaseTypeToDomain(c.RelatedCase.Type)}
	}
	if c.Account != nil {
		cv.AccountDetails = &domain.AccountRef{ID: sysidToUUID(c.Account.ID), Name: c.Account.Name, Type: c.Account.Type}
		// CRE/SRE team (see snCaseAccount.CreTeam/SreTeam doc comment) pass through once
		// the backing service exposes the matching fields; until then these are nil.
		if c.Account.CreTeam != nil {
			if id := sysidToUUID(c.Account.CreTeam.ID); id != "" {
				cv.AccountDetails.CreTeam = &domain.EntityRef{ID: id, Name: c.Account.CreTeam.Name}
			}
		}
		if c.Account.SreTeam != nil {
			if id := sysidToUUID(c.Account.SreTeam.ID); id != "" {
				cv.AccountDetails.SreTeam = &domain.EntityRef{ID: id, Name: c.Account.SreTeam.Name}
			}
		}
	}
	if len(c.LinkedServiceRequests) > 0 {
		lsr := make([]domain.LinkedServiceRequestRef, 0, len(c.LinkedServiceRequests))
		for _, r := range c.LinkedServiceRequests {
			lsr = append(lsr, domain.LinkedServiceRequestRef{ID: sysidToUUID(r.ID), Number: r.Number, Name: r.Name})
		}
		cv.LinkedServiceRequests = lsr
	}
	if len(c.ChangeRequestsAll) > 0 {
		lcr := make([]domain.LinkedChangeRequestRef, 0, len(c.ChangeRequestsAll))
		for _, r := range c.ChangeRequestsAll {
			// An absent upstream subject becomes null, not "" — see the note on
			// LinkedChangeRequestRef.Name.
			var name *string
			if r.Name != "" {
				name = strPtr(r.Name)
			}
			lcr = append(lcr, domain.LinkedChangeRequestRef{ID: sysidToUUID(r.ID), Number: r.Number, Name: name})
		}
		cv.LinkedChangeRequests = lcr
	}
	if len(c.Variables) > 0 {
		vars := make([]domain.CaseVariable, 0, len(c.Variables))
		for _, v := range c.Variables {
			// Order is the backing data source's own question order; it carries
			// meaning on the request form, so it is passed through untouched.
			vars = append(vars, domain.CaseVariable{Name: v.Name, Value: v.Value})
		}
		cv.Variables = vars
	}
	if c.ResolutionCode != nil {
		if rc, ok := snResolutionCodeByID[c.ResolutionCode.ID.String()]; ok {
			cv.ResolutionCode = &rc
		}
	}
	if c.Cause != nil {
		if cause, ok := snCauseByID[c.Cause.ID]; ok {
			cv.Cause = &cause
		}
	}
	cv.ResolutionNotes = c.ResolutionNotes
	if c.ResolvedOn != nil && *c.ResolvedOn != "" {
		resolvedOn, err := parseSNDateTime(ctx, "sn get case", "resolvedOn", *c.ResolvedOn)
		if err != nil {
			return domain.CaseView{}, fmt.Errorf("sn get case: parse resolvedOn %q: %w", *c.ResolvedOn, err)
		}
		cv.ResolvedOn = &resolvedOn
	}
	if len(c.WatchList) > 0 {
		wl := make([]domain.WatchListUser, 0, len(c.WatchList))
		for _, u := range c.WatchList {
			wlu := domain.WatchListUser{ID: sysidToUUID(u.ID), UserName: u.UserName}
			if u.Name != nil {
				wlu.Name = *u.Name
			}
			if u.Email != nil {
				wlu.Email = *u.Email
			}
			// A watch-list entry's own id is not necessarily a sys_user sys_id
			// (the list collapses several upstream glide_lists), so the
			// canonical reference keeps a null id rather than risk a non-user id.
			wlu.User = domain.NewUserReference("", wlu.Email, wlu.Name)
			wl = append(wl, wlu)
		}
		cv.WatchList = wl
	}
	// AutoclosureStep/AutoclosureStateTime pass through once the backing service exposes
	// the matching fields (see snCase field doc comments); until then these are nil.
	cv.AutoclosureStep = c.AutoclosureStep
	if c.AutoclosureStateTime != nil && *c.AutoclosureStateTime != "" {
		autoclosureStateTime, err := parseSNDateTime(ctx, "sn get case", "autoclosureStateTime", *c.AutoclosureStateTime)
		if err != nil {
			return domain.CaseView{}, fmt.Errorf("sn get case: parse autoclosureStateTime %q: %w", *c.AutoclosureStateTime, err)
		}
		cv.AutoclosureStateTime = &autoclosureStateTime
	}
	// BestCaseFixEta/MostLikelyFixEta/WorstCaseFixEta are already date-only
	// "YYYY-MM-DD" strings on both sides, so no parsing/reformatting is
	// needed — pass through once Ballerina's matching read fields (see
	// snCase doc comments) land; until then these are always nil.
	if c.BestCaseFixEta != nil && *c.BestCaseFixEta != "" {
		cv.BestCaseFixEta = c.BestCaseFixEta
	}
	if c.MostLikelyFixEta != nil && *c.MostLikelyFixEta != "" {
		cv.MostLikelyFixEta = c.MostLikelyFixEta
	}
	if c.WorstCaseFixEta != nil && *c.WorstCaseFixEta != "" {
		cv.WorstCaseFixEta = c.WorstCaseFixEta
	}

	// Pass-through of the fields declared alongside the fix-ETA group above.
	// Refs get sysidToUUID applied like every other inbound ID; the date fields
	// are already date-only strings on both sides, so no reformatting.
	// AcknowledgedBy is not re-assigned here — already populated above with the
	// richer AssignedEngineerRef (including Email).
	cv.SLAResponseTime = c.SLAResponseTime
	cv.HasAutoClosed = c.HasAutoClosed
	cv.EngagementStartDate = c.EngagementStartDate
	cv.EngagementEndDate = c.EngagementEndDate
	if c.ClosedBy != nil {
		cv.ClosedBy = &domain.EntityRef{ID: sysidToUUID(c.ClosedBy.ID), Name: c.ClosedBy.Name}
	}
	if c.EngagementPaymentType != nil && c.EngagementPaymentType.Label != "" {
		cv.EngagementPaymentType = &c.EngagementPaymentType.Label
	}
	// EscalationLevel: the single-case GET path (this function) was missing
	// this assignment -- only SearchCases populated it. Case detail needs it
	// too (the escalation widget renders off GET /cases/{id}).
	cv.EscalationLevel = snEscalationLevelToDomain(c.EscalationLevel)

	// The Choreo GET /cases/{id} response (snCase above) still has no inline tags field,
	// so the case's current tags are fetched separately via the case-scoped
	// GET /cases/{id}/tags resource. A failure here must not fail the whole case read
	// (see CaseView.Tags doc comment): cv.Tags is left nil and the failure is logged.
	tags, err := s.listCaseTags(ctx, id)
	if err != nil {
		slog.WarnContext(ctx, "sn get case: case tags lookup failed", "caseId", id, "error", err)
	} else {
		cv.Tags = tags
	}

	return cv, nil
}

type snCreateCommentPayload struct {
	ReferenceID   string `json:"referenceId"`
	ReferenceType string `json:"referenceType"`
	Type          string `json:"type"`
	Content       string `json:"content"`
	// CreatedBy is omitted unless a caller explicitly overrides the author —
	// ServiceNow then falls back to resolving it from the caller's token, which
	// is what the case-comment path here relies on. See
	// domain.CreateCommentRequest.CreatedBy.
	CreatedBy string `json:"createdBy,omitempty"`
}

type snCreateCommentResponse struct {
	Message string `json:"message"`
	Comment struct {
		ID        string `json:"id"`
		CreatedOn string `json:"createdOn"`
		CreatedBy string `json:"createdBy"`
	} `json:"comment"`
}

func (s *snCaseService) CreateCaseComment(ctx context.Context, req domain.CreateCaseCommentRequest) (domain.CreateCaseCommentResponse, error) {
	if !validCommentType[req.Type] {
		return domain.CreateCaseCommentResponse{}, &apierror.ValidationError{Msg: "type contains invalid value: " + string(req.Type)}
	}
	if req.Content == "" {
		return domain.CreateCaseCommentResponse{}, &apierror.ValidationError{Msg: "content is required"}
	}
	if req.Type == domain.CommentTypeActivity {
		return domain.CreateCaseCommentResponse{}, &apierror.ValidationError{Msg: "type 'activity' is not supported for ServiceNow"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	snType := snCommentTypeMap[req.Type]

	payload := snCreateCommentPayload{
		ReferenceID:   uuidToSysid(req.CaseID),
		ReferenceType: "case",
		Type:          snType,
		Content:       req.Content,
	}

	raw, err := s.client.Post(ctx, "/comments", token, payload)
	if err != nil {
		return domain.CreateCaseCommentResponse{}, err
	}

	var snResp snCreateCommentResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.CreateCaseCommentResponse{}, fmt.Errorf("sn create comment: parse response: %w", err)
	}

	createdOn, err := parseSNDateTime(ctx, "sn create comment", "createdOn", snResp.Comment.CreatedOn)
	if err != nil {
		return domain.CreateCaseCommentResponse{}, fmt.Errorf("sn create comment: parse createdOn %q: %w", snResp.Comment.CreatedOn, err)
	}

	result := domain.CreateCaseCommentResponse{
		Message: snResp.Message,
		Comment: domain.CaseCommentDetail{
			ID:        sysidToUUID(snResp.Comment.ID),
			CreatedOn: createdOn,
			CreatedBy: snResp.Comment.CreatedBy,
		},
	}
	s.publishCommentAdded(ctx, req, result.Comment.ID)
	s.applyResponseSLAOnComment(ctx, req, result.Comment.ID)
	s.applyCustomerReplyStateTransition(ctx, req, result.Comment.ID)
	return result, nil
}

type snCommentFilters struct {
	Type string `json:"type,omitempty"`
}

type snSearchCommentsPayload struct {
	ReferenceID   string              `json:"referenceId"`
	ReferenceType string              `json:"referenceType"`
	Filters       *snCommentFilters   `json:"filters,omitempty"`
	Pagination    snProjectPagination `json:"pagination"`
}

type snComment struct {
	ID                 string `json:"id"`
	ReferenceID        string `json:"referenceId"`
	Content            string `json:"content"`
	Type               string `json:"type"`
	CreatedOn          string `json:"createdOn"`
	CreatedBy          string `json:"createdBy"`
	CreatedByFirstName string `json:"createdByFirstName"`
	CreatedByLastName  string `json:"createdByLastName"`
	CreatedByFullName  string `json:"createdByFullName"`
	// CreatedByUser is the author's resolved sys_user record: nil when the
	// author is not a real user (e.g. "system") and when the ServiceNow side
	// predates the field. See snUserRef.
	CreatedByUser *snUserRef `json:"createdByUser"`
	// Inline attachments are the images embedded in a comment body. Declared
	// here because ServiceNow sends them and this struct previously dropped
	// them; shape follows the Ballerina entity-service InlineAttachment record.
	HasInlineAttachments bool                 `json:"hasInlineAttachments"`
	InlineAttachments    []snInlineAttachment `json:"inlineAttachments"`
}

// snInlineAttachment is an image embedded in a comment body, as ServiceNow
// returns it. IDs are sysids and are converted with sysidToUUID on the way out.
type snInlineAttachment struct {
	ID          string `json:"id"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	DownloadURL string `json:"downloadUrl"`
	CreatedOn   string `json:"createdOn"`
	CreatedBy   string `json:"createdBy"`
}

type snSearchCommentsResponse struct {
	Comments     []snComment `json:"comments"`
	Offset       int         `json:"offset"`
	Limit        int         `json:"limit"`
	TotalRecords int         `json:"totalRecords"`
}

var snCommentTypeMap = map[domain.CommentType]string{
	domain.CommentTypeComment:  "comments",
	domain.CommentTypeWorkNote: "work_notes",
}

// snCommentTypeToCommentType maps the SN API type string back to the domain enum.
var snCommentTypeToCommentType = map[string]domain.CommentType{
	"comments":   domain.CommentTypeComment,
	"work_notes": domain.CommentTypeWorkNote,
	"activity":   domain.CommentTypeActivity,
}

func (s *snCaseService) SearchCaseComments(ctx context.Context, req domain.SearchCaseCommentsRequest) (domain.SearchCaseCommentsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCaseCommentsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snSearchCommentsPayload{
		ReferenceID:   uuidToSysid(req.CaseID),
		ReferenceType: "case",
		Pagination:    snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}
	if req.Filters != nil && req.Filters.Type != nil {
		snType, ok := snCommentTypeMap[*req.Filters.Type]
		if !ok {
			return domain.SearchCaseCommentsResponse{}, &apierror.ValidationError{
				Msg: "filters.type is not supported for ServiceNow: " + string(*req.Filters.Type),
			}
		}
		payload.Filters = &snCommentFilters{Type: snType}
	}

	raw, err := s.client.Post(ctx, "/comments/search", token, payload)
	if err != nil {
		return domain.SearchCaseCommentsResponse{}, err
	}

	var snResp snSearchCommentsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchCaseCommentsResponse{}, fmt.Errorf("sn search comments: parse response: %w", err)
	}

	comments := make([]domain.CaseComment, 0, len(snResp.Comments))
	for _, c := range snResp.Comments {
		createdAt, err := parseSNDateTime(ctx, "sn search comments", "createdOn", c.CreatedOn)
		if err != nil {
			return domain.SearchCaseCommentsResponse{}, fmt.Errorf("sn search comments: parse createdOn %q: %w", c.CreatedOn, err)
		}
		var commentType domain.CommentType
		switch c.Type {
		case "comments", "comment":
			commentType = domain.CommentTypeComment
		case "work_notes", "work_note":
			commentType = domain.CommentTypeWorkNote
		case "activity":
			commentType = domain.CommentTypeActivity
		default:
			commentType = domain.CommentTypeComment
		}
		// Inline attachments: sysid -> UUID like every other inbound ID, and the
		// same createdOn layout. A parse failure on one image must not fail the
		// whole comment page, so a bad timestamp leaves that entry's CreatedOn zero.
		var inlineAttachments []domain.InlineAttachment
		for _, ia := range c.InlineAttachments {
			entry := domain.InlineAttachment{
				ID:          sysidToUUID(ia.ID),
				FileName:    ia.FileName,
				ContentType: ia.ContentType,
				DownloadURL: ia.DownloadURL,
				CreatedBy:   ia.CreatedBy,
			}
			// Left nil for an empty or unparseable value: a zero time would render as
			// "0001-01-01T00:00:00Z" and read as a genuine timestamp.
			if ia.CreatedOn != "" {
				if parsed, err := time.Parse(snCreatedOnLayout, ia.CreatedOn); err == nil {
					entry.CreatedOn = &parsed
				}
			}
			inlineAttachments = append(inlineAttachments, entry)
		}
		comments = append(comments, domain.CaseComment{
			ID:                   sysidToUUID(c.ID),
			CaseID:               sysidToUUID(c.ReferenceID),
			Type:                 commentType,
			Content:              c.Content,
			CreatedBy:            snUserReference(c.CreatedByUser, c.CreatedBy, c.CreatedByFullName),
			CreatedOn:            createdAt,
			HasInlineAttachments: c.HasInlineAttachments,
			InlineAttachments:    inlineAttachments,
		})
	}

	total := snResp.TotalRecords
	return domain.SearchCaseCommentsResponse{
		Comments: comments,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(comments) < total,
	}, nil
}

type snUpdateCasePayload struct {
	StateKey     *int `json:"stateKey,omitempty"`
	SeverityKey  *int `json:"severityKey,omitempty"`
	WorkStateKey *int `json:"workStateKey,omitempty"`
	// Type transfers the case to another type --
	// same string values as the create payload's own Type field (see
	// snCaseTypeMap). EngagementType/EngagementPaymentType (int keys, only
	// meaningful with Type "engagement") and CatalogID/CatalogItemID/Variables
	// (only meaningful with Type "service_request") are its companions, mirroring
	// snCreateCasePayload's own field set for those two type-specific shapes.
	Type                  *string          `json:"type,omitempty"`
	EngagementType        *int             `json:"engagementType,omitempty"`
	EngagementPaymentType *int             `json:"engagementPaymentType,omitempty"`
	IssueTypeKey          *int             `json:"issueTypeKey,omitempty"`
	CatalogID             *string          `json:"catalogId,omitempty"`
	CatalogItemID         *string          `json:"catalogItemId,omitempty"`
	Variables             []snCaseVariable `json:"variables,omitempty"`
	// WatchList replaces the whole list, so an explicitly empty list must still be
	// sent to clear it rather than be omitted -- hence the pointer.
	WatchList     *[]string `json:"watchList,omitempty"`
	AssigneeEmail *string   `json:"assigneeEmail,omitempty"`
	// Acknowledge claims the case for the calling engineer, first-write-wins. Only
	// true is ever sent -- there is no unacknowledge -- and the backing service keeps
	// it mutually exclusive with every other field in this payload.
	Acknowledge    *bool   `json:"acknowledge,omitempty"`
	ResolutionCode *int    `json:"resolutionCode,omitempty"`
	Cause          *string `json:"cause,omitempty"`
	CloseNotes     *string `json:"closeNotes,omitempty"`
	// ParentID writes the native task.parent field. Confirmed already supported by the
	// backing service's case-update payload and its validation, which already accept it
	// as an exactly-one-field option -- fully wired.
	ParentID *string `json:"parentId,omitempty"`
	// RelatedCaseID writes the looser, non-hierarchical u_related_case cross-link.
	// A matching field on the backing service's case-update payload and its validation
	// exists, but is not yet available in the backing service.
	RelatedCaseID *string `json:"relatedCaseId,omitempty"`
	// AutocloseHoldUntil places the case on hold in ServiceNow's staged auto-closure
	// sequence, internally setting u_autoclosure_step = ON_HOLD and
	// u_autoclosure_state_time = this date together. A matching field on the backing
	// service's case-update payload exists, but is not yet available in the backing
	// service.
	AutocloseHoldUntil *string `json:"autocloseHoldUntil,omitempty"`
	// Title/Description/DeploymentID/DeployedProductID as PATCH-time fields (previously
	// the backing service's case-update payload only supported these at create time, via
	// the case-create payload) -- not yet available in the backing service.
	Title             *string `json:"title,omitempty"`
	Description       *string `json:"description,omitempty"`
	DeploymentID      *string `json:"deploymentId,omitempty"`
	DeployedProductID *string `json:"deployedProductId,omitempty"`
	// BestCaseFixEta writes the internal-only best-case fix-commitment date
	// (u_best_case_fix_eta) as a date-only "YYYY-MM-DD" string. Confirmed live
	// against the existing case-update resource — same single-field PATCH
	// pathway as AutocloseHoldUntil/Title/Description above.
	BestCaseFixEta *string `json:"bestCaseFixEta,omitempty"`
	// MostLikelyFixEta writes the internal-only most-likely fix-commitment
	// date (u_most_likely_fix_eta) as a date-only "YYYY-MM-DD" string. Same
	// pathway as BestCaseFixEta above.
	MostLikelyFixEta *string `json:"mostLikelyFixEta,omitempty"`
	// WorstCaseFixEta writes the internal-only worst-case fix-commitment date
	// (u_worst_case_fix_eta) as a date-only "YYYY-MM-DD" string. Same pathway
	// as BestCaseFixEta above.
	WorstCaseFixEta *string `json:"worstCaseFixEta,omitempty"`
	// AddPublicComment/Product/PublicTicket mirror ServiceNow's "Share Fix ETA"
	// CWF action: when AddPublicComment is true, ServiceNow posts a customer-visible
	// comment built from Product/PublicTicket/the 3 ETA dates, in addition to writing
	// the ETA fields themselves.
	AddPublicComment *bool   `json:"addPublicComment,omitempty"`
	Product          *string `json:"product,omitempty"`
	PublicTicket     *string `json:"publicTicket,omitempty"`
	// WorkaroundProvided marks (true) or recalls (false) the case's workaround
	// (u_workaround_provided/u_workaround_provided_by). Same combinable pathway as
	// the fix-ETA fields above -- not mutually exclusive with anything.
	WorkaroundProvided *bool `json:"workaroundProvided,omitempty"`
}

// snResolutionStates are the state keys that allow resolution fields.
var snResolutionStates = map[int]bool{
	3: true, // closed
	6: true, // solution_proposed
}

// snResolutionCodeKey maps domain CaseResolutionCode enums to the ServiceNow integer keys.
var snResolutionCodeKey = map[domain.CaseResolutionCode]int{
	domain.CaseResolutionCodeSolvedFixedBySupportGuidanceProvided: 1,
	domain.CaseResolutionCodeSolvedFixedByClosingRelatedIncident:  16,
	domain.CaseResolutionCodeSolvedFixedByClosingRelatedRDTicket:  17,
	domain.CaseResolutionCodeSolvedWorkaroundProvided:             3,
	domain.CaseResolutionCodeSolvedByCustomer:                     4,
	domain.CaseResolutionCodeConsideredForRoadmap:                 18,
	domain.CaseResolutionCodeInconclusiveOutOfScope:               5,
	domain.CaseResolutionCodeInconclusiveCannotReproduce:          6,
	domain.CaseResolutionCodeInconclusiveNoWorkaround:             7,
	domain.CaseResolutionCodeDuplicateIssue:                       8,
	domain.CaseResolutionCodeVoidedCanceled:                       9,
	domain.CaseResolutionCodeOnHold:                               19,
	domain.CaseResolutionCodeConsideredForRoadmapAlt:              20,
	domain.CaseResolutionCodeSolvedFixedTheIssue:                  21,
	domain.CaseResolutionCodeSolvedWorkaroundProvidedAlt:          22,
	domain.CaseResolutionCodeSolvedByContributor:                  27,
	domain.CaseResolutionCodeSolvedByNovera:                       51,
	domain.CaseResolutionCodeAbruptlyClosedDueToNonResponsiveness: 52,
}

// snResolutionCodeByID maps ServiceNow resolution code id strings to domain CaseResolutionCode enums.
var snResolutionCodeByID = func() map[string]domain.CaseResolutionCode {
	m := make(map[string]domain.CaseResolutionCode, len(snResolutionCodeKey))
	for k, v := range snResolutionCodeKey {
		m[fmt.Sprintf("%d", v)] = k
	}
	return m
}()

// snCauseKey maps domain CaseCause enums to the ServiceNow integer choice
// values for sn_customerservice_case.cause on the PROD tenant (wso2),
// verified via sys_choice. Unlike resolution codes' scattered keys, prod's
// cause choice list happens to number sequentially 1-25 in picklist order.
//
// The DEV tenant (wso2sndev) configures this same field with the label text
// as its stored value instead of an integer — a real cross-tenant
// inconsistency, not a bug in this mapping. This map targets prod, the only
// tenant live customer traffic reaches; DEV-tenant testing of the cause
// field will not round-trip correctly against this mapping.
var snCauseKey = map[domain.CaseCause]int{
	domain.CaseCauseSolutionArchitecture:          1,
	domain.CaseCauseDeploymentArchitecture:        2,
	domain.CaseCauseUserErrorConfiguration:        3,
	domain.CaseCauseUserErrorProductConcept:       4,
	domain.CaseCauseUserErrorRuntime:              5,
	domain.CaseCauseUserErrorRecommendation:       6,
	domain.CaseCauseCustomizationLimitation:       7,
	domain.CaseCauseCustomizationBug:              8,
	domain.CaseCauseDocumentationGap:              9,
	domain.CaseCauseDocumentationError:            10,
	domain.CaseCauseProductLimitation:             11,
	domain.CaseCauseProductBug:                    12,
	domain.CaseCauseProductRegression:             13,
	domain.CaseCauseProductMigration:              14,
	domain.CaseCauseInfrastructureDatabase:        15,
	domain.CaseCauseInfrastructureOS:              16,
	domain.CaseCauseInfrastructureNetwork:         17,
	domain.CaseCauseInfrastructureJDK:             18,
	domain.CaseCauseInfrastructureLDAP:            19,
	domain.CaseCauseInfrastructureLoadBalancer:    20,
	domain.CaseCauseInfrastructureIAAS:            21,
	domain.CaseCauseInfrastructureExternalProduct: 22,
	domain.CaseCauseInfrastructureProxy:           23,
	domain.CaseCauseInfrastructureOther:           24,
	domain.CaseCauseUnknown:                       25,
}

// snCauseByID maps ServiceNow cause choice-value strings (the SN "cause"
// field's id, e.g. "12") to domain CaseCause enums.
var snCauseByID = func() map[string]domain.CaseCause {
	m := make(map[string]domain.CaseCause, len(snCauseKey))
	for k, v := range snCauseKey {
		m[strconv.Itoa(v)] = k
	}
	return m
}()

// snWorkStateIDMap maps domain CaseWorkState enums to SN numeric work state IDs.
var snWorkStateIDMap = map[domain.CaseWorkState]int{
	domain.CaseWorkStateOngoing: 1,
	domain.CaseWorkStatePaused:  2,
}

type snUpdateCaseResponse struct {
	Message string `json:"message"`
	Case    struct {
		ID        string           `json:"id"`
		UpdatedOn string           `json:"updatedOn"`
		UpdatedBy string           `json:"updatedBy"`
		State     *snCaseState     `json:"state"`
		Severity  *snCaseLabel     `json:"severity"`
		Type      *snCaseEntityRef `json:"type"`
		WorkState *snCaseLabel     `json:"workState"`
		WatchList []struct {
			ID       string `json:"id"`
			UserName string `json:"userName"`
			Name     string `json:"name"`
			Email    string `json:"email"`
		} `json:"watchList"`
		AssignedTo *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"assignedTo"`
		ResolutionCode *struct {
			ID    json.Number `json:"id"`
			Label string      `json:"label"`
		} `json:"resolutionCode"`
		Cause *struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"cause"`
		CloseNotes *string    `json:"closeNotes"`
		ResolvedOn *string    `json:"resolvedOn"`
		ParentCase *snCaseRef `json:"parentCase"`
		// BestCaseFixEta/MostLikelyFixEta/WorstCaseFixEta echo back the
		// updated date-only estimate when that field was the one PATCHed —
		// see snUpdateCasePayload doc comments.
		BestCaseFixEta   *string `json:"bestCaseFixEta"`
		MostLikelyFixEta *string `json:"mostLikelyFixEta"`
		WorstCaseFixEta  *string `json:"worstCaseFixEta"`
		// Number/AlreadyAcknowledged/AcknowledgedBy come back only on the
		// acknowledge path -- see snUpdateCasePayload.Acknowledge.
		Number              string                 `json:"number"`
		AlreadyAcknowledged *bool                  `json:"alreadyAcknowledged"`
		AcknowledgedBy      *snAssignedEngineerRef `json:"acknowledgedBy"`
	} `json:"case"`
}

func (s *snCaseService) UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error) {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.UpdateCaseResponse{}, err
	}

	hasResolutionFields := req.ResolutionCode != nil || req.Cause != nil || req.CloseNotes != nil

	// exclusiveCount covers the role-gated fields with complex side effects
	// (state transitions, assignment, watch list, parent linkage) -- SN keeps
	// these mutually exclusive of each other and of every other field.
	exclusiveCount := 0
	if req.State != nil {
		exclusiveCount++
	}
	// Severity is a *companion* of Type when a type transfer is requested: transferring to
	// "case" needs a severity, which is also what selects Incident vs Query at the backing
	// data source. In that one combination it does not take an exclusive slot of its own.
	// Every other severity request is unchanged.
	severityIsTypeCompanion := req.Severity != nil && req.Type != nil
	if req.Severity != nil && !severityIsTypeCompanion {
		exclusiveCount++
	}
	if req.WorkState != nil {
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
	if req.Type != nil {
		exclusiveCount++
	}
	// combinableCount covers plain field writes with no cross-field side
	// effects -- SN now accepts any subset of these together in one PATCH.
	combinableCount := 0
	if req.RelatedCaseID != nil {
		combinableCount++
	}
	if req.AutocloseHoldUntil != nil {
		combinableCount++
	}
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
	// AddPublicComment/Product/PublicTicket are meaningful only alongside the
	// fix-ETA trio above (see the addPublicComment handling below), so they
	// belong in the same combinable bucket -- otherwise `type` (or any other
	// exclusive field) plus a bare `product`/`publicTicket` with no fix-ETA
	// date would pass this gate and then be silently dropped later, never
	// validated or forwarded.
	if req.AddPublicComment != nil {
		combinableCount++
	}
	if req.Product != nil {
		combinableCount++
	}
	if req.PublicTicket != nil {
		combinableCount++
	}
	if req.WorkaroundProvided != nil {
		combinableCount++
	}
	const fieldList = "state, severity, workState, watchList, assigneeEmail, parentId, acknowledge, type, " +
		"relatedCaseId, autocloseHoldUntil, subject, description, deploymentId, deployedProductId, " +
		"bestCaseFixEta, mostLikelyFixEta, worstCaseFixEta, or workaroundProvided"
	if exclusiveCount == 0 && combinableCount == 0 {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "at least one of " + fieldList + " must be provided"}
	}
	if exclusiveCount > 1 || (exclusiveCount == 1 && combinableCount > 0) {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "state, severity, workState, watchList, assigneeEmail, parentId, acknowledge, and type cannot be combined with each other or with any other field in the same request"}
	}
	if hasResolutionFields && req.State == nil {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "resolutionCode, cause, and closeNotes are only allowed when state is also provided"}
	}
	if req.Type == nil && (req.EngagementType != nil || req.EngagementPaymentType != nil || req.CatalogID != nil || req.CatalogItemID != nil || len(req.Variables) > 0 || req.IssueType != nil) {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "engagementType, engagementPaymentType, issueType, catalogId, catalogItemId, and variables are only allowed when type is also provided"}
	}
	if req.AddPublicComment == nil && (req.Product != nil || req.PublicTicket != nil) {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "product and publicTicket are only allowed when addPublicComment is also provided"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snUpdateCasePayload{}
	if req.State != nil {
		if !validCaseState[*req.State] {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "state contains invalid value: " + string(*req.State)}
		}
		id, ok := snStateIDMap[*req.State]
		if !ok {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "state " + string(*req.State) + " is not supported by ServiceNow"}
		}
		payload.StateKey = &id
		if hasResolutionFields && !snResolutionStates[id] {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "resolutionCode, cause, and closeNotes are only allowed when state is closed or solution_proposed"}
		}
		if req.ResolutionCode != nil {
			key, ok := snResolutionCodeKey[*req.ResolutionCode]
			if !ok {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "resolutionCode contains invalid value: " + string(*req.ResolutionCode)}
			}
			payload.ResolutionCode = &key
		}
		if req.Cause != nil {
			key, ok := snCauseKey[*req.Cause]
			if !ok {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "cause contains invalid value: " + string(*req.Cause)}
			}
			val := strconv.Itoa(key)
			payload.Cause = &val
		}
		payload.CloseNotes = req.CloseNotes
	}
	if req.Severity != nil {
		if !validCaseSeverity[*req.Severity] {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "severity contains invalid value: " + string(*req.Severity)}
		}
		id, ok := snSeverityIDMap[*req.Severity]
		if !ok {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "severity " + string(*req.Severity) + " is not supported by ServiceNow"}
		}
		payload.SeverityKey = &id
	}
	if req.Type != nil {
		if !validCaseType[*req.Type] {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "type contains invalid value: " + *req.Type}
		}
		snType, ok := snCaseTypeMap[*req.Type]
		if !ok {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "type " + *req.Type + " is not supported by ServiceNow"}
		}
		payload.Type = &snType
		switch *req.Type {
		case "engagement":
			if req.EngagementType == nil {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "engagementType is required when type is \"engagement\""}
			}
			if req.EngagementPaymentType == nil {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "engagementPaymentType is required when type is \"engagement\""}
			}
			if req.CatalogID != nil || req.CatalogItemID != nil || len(req.Variables) > 0 {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "catalogId, catalogItemId, and variables are only accepted when type is \"service_request\""}
			}
			if req.IssueType != nil {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "issueType is only accepted when type is \"case\""}
			}
			if req.Severity != nil {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "severity may only accompany type when type is \"case\""}
			}
			if !validEngagementType[*req.EngagementType] {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "engagementType contains invalid value: " + string(*req.EngagementType)}
			}
			if !validEngagementPaymentType[*req.EngagementPaymentType] {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "engagementPaymentType contains invalid value: " + string(*req.EngagementPaymentType)}
			}
			id := snEngagementTypeIDMap[*req.EngagementType]
			payload.EngagementType = &id
			paymentID := snEngagementPaymentTypeIDMap[*req.EngagementPaymentType]
			payload.EngagementPaymentType = &paymentID
		case "service_request":
			if req.CatalogID == nil || req.CatalogItemID == nil {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "catalogId and catalogItemId are required when type is \"service_request\""}
			}
			// The backing data source requires at least one variable: a service request with no
			// variable values has no request detail, and renders with an empty category in the
			// customer-facing portal.
			if len(req.Variables) == 0 {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "variables must contain at least one entry when type is \"service_request\""}
			}
			if req.EngagementType != nil || req.EngagementPaymentType != nil {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "engagementType and engagementPaymentType are only accepted when type is \"engagement\""}
			}
			if req.Severity != nil {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "severity may only accompany type when type is \"case\""}
			}
			if err := validateUUIDs("catalogId", []string{*req.CatalogID}); err != nil {
				return domain.UpdateCaseResponse{}, err
			}
			if err := validateUUIDs("catalogItemId", []string{*req.CatalogItemID}); err != nil {
				return domain.UpdateCaseResponse{}, err
			}
			catalogSysid := uuidToSysid(*req.CatalogID)
			catalogItemSysid := uuidToSysid(*req.CatalogItemID)
			payload.CatalogID = &catalogSysid
			payload.CatalogItemID = &catalogItemSysid
			if len(req.Variables) > 0 {
				vars := make([]snCaseVariable, 0, len(req.Variables))
				for i, v := range req.Variables {
					if err := validateUUIDs(fmt.Sprintf("variables[%d].id", i), []string{v.ID}); err != nil {
						return domain.UpdateCaseResponse{}, err
					}
					vars = append(vars, snCaseVariable{ID: uuidToSysid(v.ID), Value: v.Value})
				}
				payload.Variables = vars
			}
		case "case":
			if req.EngagementType != nil || req.EngagementPaymentType != nil || req.CatalogID != nil || req.CatalogItemID != nil || len(req.Variables) > 0 {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "engagementType and engagementPaymentType are only accepted when type is \"engagement\"; catalogId, catalogItemId, and variables are only accepted when type is \"service_request\""}
			}
			// Both are mandatory at the backing data source: severity selects Incident vs Query,
			// and issue type is the classification those records carry.
			if req.Severity == nil {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "severity is required when type is \"case\""}
			}
			if req.IssueType == nil {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "issueType is required when type is \"case\""}
			}
			if !validCaseIssueType[*req.IssueType] {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "issueType contains invalid value: " + string(*req.IssueType)}
			}
			issueTypeID, ok := snIssueTypeIDMap[*req.IssueType]
			if !ok {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "issueType " + string(*req.IssueType) + " is not supported by ServiceNow"}
			}
			payload.IssueTypeKey = &issueTypeID
		default:
			if req.EngagementType != nil || req.EngagementPaymentType != nil || req.CatalogID != nil || req.CatalogItemID != nil || len(req.Variables) > 0 {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "engagementType and engagementPaymentType are only accepted when type is \"engagement\"; catalogId, catalogItemId, and variables are only accepted when type is \"service_request\""}
			}
			if req.IssueType != nil {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "issueType is only accepted when type is \"case\""}
			}
			if req.Severity != nil {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "severity may only accompany type when type is \"case\""}
			}
		}
	}
	if req.WorkState != nil {
		if !validCaseWorkState[*req.WorkState] {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "workState contains invalid value: " + string(*req.WorkState)}
		}
		id, ok := snWorkStateIDMap[*req.WorkState]
		if !ok {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "workState " + string(*req.WorkState) + " is not supported by ServiceNow"}
		}
		payload.WorkStateKey = &id
	}
	if req.WatchList != nil {
		// As on create, the backing service's case-update payload declares the
		// watch list as email addresses, and it replaces the whole list, so an
		// explicitly empty list must still be sent to clear it rather than be
		// skipped.
		emails, err := watchListEmails(ctx, s.client, token, "watchList", *req.WatchList)
		if err != nil {
			return domain.UpdateCaseResponse{}, err
		}
		payload.WatchList = &emails
	}
	if req.AssigneeEmail != nil {
		payload.AssigneeEmail = req.AssigneeEmail
	}
	if req.Acknowledge != nil {
		// Reject false here rather than forwarding it: acknowledgement is
		// first-write-wins with no unacknowledge path, so false has no meaning, and a
		// caller sending it is asking for something this API cannot do.
		if !*req.Acknowledge {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "acknowledge must be true; unacknowledging a case is not supported"}
		}
		payload.Acknowledge = req.Acknowledge
	}
	if req.ParentID != nil {
		if err := validateUUIDs("parentId", []string{*req.ParentID}); err != nil {
			return domain.UpdateCaseResponse{}, err
		}
		sysid := uuidToSysid(*req.ParentID)
		payload.ParentID = &sysid
	}
	if req.RelatedCaseID != nil {
		if err := validateUUIDs("relatedCaseId", []string{*req.RelatedCaseID}); err != nil {
			return domain.UpdateCaseResponse{}, err
		}
		sysid := uuidToSysid(*req.RelatedCaseID)
		payload.RelatedCaseID = &sysid
	}
	if req.AutocloseHoldUntil != nil {
		holdUntil := formatSNDateOnly(req.AutocloseHoldUntil)
		payload.AutocloseHoldUntil = &holdUntil
	}
	if req.Subject != nil {
		if *req.Subject == "" {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "subject cannot be empty"}
		}
		payload.Title = req.Subject
	}
	if req.Description != nil {
		if *req.Description == "" {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "description cannot be empty"}
		}
		payload.Description = req.Description
	}
	if req.DeploymentID != nil {
		if err := validateUUIDs("deploymentId", []string{*req.DeploymentID}); err != nil {
			return domain.UpdateCaseResponse{}, err
		}
		sysid := uuidToSysid(*req.DeploymentID)
		payload.DeploymentID = &sysid
	}
	if req.DeployedProductID != nil {
		if err := validateUUIDs("deployedProductId", []string{*req.DeployedProductID}); err != nil {
			return domain.UpdateCaseResponse{}, err
		}
		sysid := uuidToSysid(*req.DeployedProductID)
		payload.DeployedProductID = &sysid
	}
	if req.BestCaseFixEta != nil {
		if err := validateDateOnly("bestCaseFixEta", *req.BestCaseFixEta); err != nil {
			return domain.UpdateCaseResponse{}, err
		}
		payload.BestCaseFixEta = req.BestCaseFixEta
	}
	if req.MostLikelyFixEta != nil {
		if err := validateDateOnly("mostLikelyFixEta", *req.MostLikelyFixEta); err != nil {
			return domain.UpdateCaseResponse{}, err
		}
		payload.MostLikelyFixEta = req.MostLikelyFixEta
	}
	if req.WorstCaseFixEta != nil {
		if err := validateDateOnly("worstCaseFixEta", *req.WorstCaseFixEta); err != nil {
			return domain.UpdateCaseResponse{}, err
		}
		payload.WorstCaseFixEta = req.WorstCaseFixEta
	}
	if req.AddPublicComment != nil {
		hasAnyFixEta := req.BestCaseFixEta != nil || req.MostLikelyFixEta != nil || req.WorstCaseFixEta != nil
		if !hasAnyFixEta {
			return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "addPublicComment requires at least one of bestCaseFixEta, mostLikelyFixEta, or worstCaseFixEta"}
		}
		if *req.AddPublicComment {
			if req.Product == nil || *req.Product == "" {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "product is required when addPublicComment is true"}
			}
			if req.PublicTicket == nil || *req.PublicTicket == "" {
				return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "publicTicket is required when addPublicComment is true"}
			}
		}
		payload.AddPublicComment = req.AddPublicComment
		payload.Product = req.Product
		payload.PublicTicket = req.PublicTicket
	}
	if req.WorkaroundProvided != nil {
		payload.WorkaroundProvided = req.WorkaroundProvided
	}

	// A state-change PATCH is only worth publishing case.status_changed for
	// if the state is actually transitioning — a caller re-PATCHing the
	// case's own current state (a no-op as far as ServiceNow is concerned)
	// must not send every watcher a false "status changed" notification.
	// Fetching the case's state now, before the PATCH below, both answers
	// that question and supplies publishStatusChanged's own enrichment
	// (ProjectID/Recipients) — a single GetCaseByID call either way, just
	// moved earlier instead of adding a second one after the PATCH. Bounded
	// by its own derived context (publishStatusChangedTimeout), same as
	// every other publish helper's enrichment call, so a slow ServiceNow
	// round trip here can't eat into this request's own deadline — unlike
	// those helpers, this one sits in UpdateCase's critical path (before
	// the PATCH, not fired off after success), so a failure here is caught
	// and simply skips publishing rather than failing UpdateCase itself.
	var caseBeforeUpdate domain.CaseView
	publishStatusChange := false
	if req.State != nil && s.publisher != nil {
		enrichCtx, cancel := context.WithTimeout(ctx, publishStatusChangedTimeout)
		cv, err := s.GetCaseByID(enrichCtx, req.ID)
		cancel()
		switch {
		case err != nil:
			slog.ErrorContext(ctx, "sn update case: enrich case for case.status_changed publish failed", "caseId", req.ID)
		case cv.State == *req.State:
			slog.InfoContext(ctx, "sn update case: case.status_changed not published, state is unchanged", "caseId", req.ID)
		default:
			caseBeforeUpdate = cv
			publishStatusChange = true
		}
	}

	// Same reasoning as the state-change block above, applied to severity
	// instead: a caller re-PATCHing the case's current severity (a no-op as
	// far as ServiceNow is concerned) must not send every watcher a false
	// "severity changed" notification. req.State and req.Severity are
	// mutually exclusive per request (see exclusiveCount above), so this and
	// the block above never both fire for the same call.
	var caseBeforeSeverity domain.CaseView
	publishSeverityChange := false
	if req.Severity != nil && s.publisher != nil {
		enrichCtx, cancel := context.WithTimeout(ctx, publishSeverityChangedTimeout)
		cv, err := s.GetCaseByID(enrichCtx, req.ID)
		cancel()
		switch {
		case err != nil:
			slog.ErrorContext(ctx, "sn update case: enrich case for case.severity_changed publish failed", "caseId", req.ID)
		case cv.Severity == *req.Severity:
			slog.InfoContext(ctx, "sn update case: case.severity_changed not published, severity is unchanged", "caseId", req.ID)
		default:
			caseBeforeSeverity = cv
			publishSeverityChange = true
		}
	}

	// Same reasoning as the state-change block above, applied to
	// assigneeEmail instead: a caller re-PATCHing the case's current
	// assignee (a no-op as far as ServiceNow is concerned) must not send
	// every watcher a false "case assigned" notification. req.State and
	// req.AssigneeEmail are mutually exclusive per request (see
	// exclusiveCount above), so this and the block above never both fire
	// for the same call.
	var caseBeforeAssign domain.CaseView
	publishCaseAssign := false
	if req.AssigneeEmail != nil && s.publisher != nil {
		enrichCtx, cancel := context.WithTimeout(ctx, publishCaseAssignedTimeout)
		cv, err := s.GetCaseByID(enrichCtx, req.ID)
		cancel()
		switch {
		case err != nil:
			slog.ErrorContext(ctx, "sn update case: enrich case for case.assigned publish failed", "caseId", req.ID)
		case cv.AssignedEngineer != nil && strings.EqualFold(cv.AssignedEngineer.Email, *req.AssigneeEmail):
			slog.InfoContext(ctx, "sn update case: case.assigned not published, assignee is unchanged", "caseId", req.ID)
		default:
			caseBeforeAssign = cv
			publishCaseAssign = true
		}
	}

	raw, err := s.client.Patch(ctx, "/cases/"+uuidToSysid(req.ID), token, payload)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}

	var snResp snUpdateCaseResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.UpdateCaseResponse{}, fmt.Errorf("sn update case: parse response: %w", err)
	}

	updatedOn, err := parseSNDateTime(ctx, "sn update case", "updatedOn", snResp.Case.UpdatedOn)
	if err != nil {
		return domain.UpdateCaseResponse{}, fmt.Errorf("sn update case: parse updatedOn %q: %w", snResp.Case.UpdatedOn, err)
	}

	resp := domain.UpdateCaseResponse{
		Message: snResp.Message,
		Case: domain.UpdatedCase{
			ID:        sysidToUUID(snResp.Case.ID),
			UpdatedOn: updatedOn,
			UpdatedBy: snResp.Case.UpdatedBy,
		},
	}

	if snResp.Case.State != nil {
		state, err := snCaseStateLabelToEnum(snResp.Case.State)
		if err == nil {
			resp.Case.State = state
		}
	}
	if snResp.Case.Severity != nil {
		resp.Case.Severity = snSeverityToSeverity(snResp.Case.Severity)
	}
	if snResp.Case.Type != nil {
		if t := snCaseTypeToDomain(snResp.Case.Type); t != nil {
			resp.Case.Type = *t
		}
	}
	resp.Case.WorkState = snWorkStateLabelToEnum(snResp.Case.WorkState)
	if snResp.Case.AssignedTo != nil {
		resp.Case.AssignedTo = &domain.AssignedEngineerRef{
			ID:   sysidToUUID(snResp.Case.AssignedTo.ID),
			Name: snResp.Case.AssignedTo.Name,
		}
		resp.Case.AssignedToUser = domain.NewUserReference(resp.Case.AssignedTo.ID, "", resp.Case.AssignedTo.Name)
	}
	if req.Acknowledge != nil {
		// Number and alreadyAcknowledged are only meaningful on the acknowledge path,
		// so they are echoed only when this request was one. alreadyAcknowledged is
		// copied verbatim rather than defaulted: if the backing service omits it, the
		// caller sees an absent field instead of a fabricated false.
		resp.Case.Number = snResp.Case.Number
		resp.Case.AlreadyAcknowledged = snResp.Case.AlreadyAcknowledged
		if snResp.Case.AcknowledgedBy != nil {
			resp.Case.AcknowledgedBy = &domain.AssignedEngineerRef{
				ID:    sysidToUUID(snResp.Case.AcknowledgedBy.ID),
				Name:  snResp.Case.AcknowledgedBy.Name,
				Email: snResp.Case.AcknowledgedBy.Email,
			}
		}
	}
	if len(snResp.Case.WatchList) > 0 {
		wl := make([]domain.WatchListUser, 0, len(snResp.Case.WatchList))
		for _, u := range snResp.Case.WatchList {
			wl = append(wl, domain.WatchListUser{
				ID:       sysidToUUID(u.ID),
				UserName: u.UserName,
				Name:     u.Name,
				Email:    u.Email,
				// Null id by design, as on the case read above.
				User: domain.NewUserReference("", u.Email, u.Name),
			})
		}
		resp.Case.WatchList = wl
	}
	if snResp.Case.ResolutionCode != nil {
		if rc, ok := snResolutionCodeByID[snResp.Case.ResolutionCode.ID.String()]; ok {
			resp.Case.ResolutionCode = &rc
		}
	}
	if snResp.Case.Cause != nil {
		if c, ok := snCauseByID[snResp.Case.Cause.ID]; ok {
			resp.Case.Cause = &c
		}
	}
	resp.Case.CloseNotes = snResp.Case.CloseNotes
	if snResp.Case.ParentCase != nil {
		resp.Case.ParentCase = &domain.CaseNumberRef{ID: sysidToUUID(snResp.Case.ParentCase.ID), Number: snResp.Case.ParentCase.Number, Type: snParentCaseTypeToDomain(snResp.Case.ParentCase.Type)}
	}
	if snResp.Case.ResolvedOn != nil {
		resolvedOn, err := parseSNDateTime(ctx, "sn update case", "resolvedOn", *snResp.Case.ResolvedOn)
		if err != nil {
			return domain.UpdateCaseResponse{}, fmt.Errorf("sn update case: parse resolvedAt %q: %w", *snResp.Case.ResolvedOn, err)
		}
		resp.Case.ResolvedOn = &resolvedOn
	}
	// BestCaseFixEta/MostLikelyFixEta/WorstCaseFixEta echo back verbatim — the
	// wire and domain types are both date-only "YYYY-MM-DD" strings, so no
	// parsing/reformatting is needed here.
	if snResp.Case.BestCaseFixEta != nil && *snResp.Case.BestCaseFixEta != "" {
		resp.Case.BestCaseFixEta = snResp.Case.BestCaseFixEta
	}
	if snResp.Case.MostLikelyFixEta != nil && *snResp.Case.MostLikelyFixEta != "" {
		resp.Case.MostLikelyFixEta = snResp.Case.MostLikelyFixEta
	}
	if snResp.Case.WorstCaseFixEta != nil && *snResp.Case.WorstCaseFixEta != "" {
		resp.Case.WorstCaseFixEta = snResp.Case.WorstCaseFixEta
	}

	if publishStatusChange && snResp.Case.State != nil {
		s.publishStatusChanged(ctx, req.ID, snResp.Case.State.Label, caseBeforeUpdate)
	}
	// Deliberately independent of publishStatusChange (which is also gated
	// on s.publisher != nil) — see applyCaseStateSLAEffects' own doc
	// comment for why pause/resume/completion must not depend on Event Hub
	// being configured. req.State != nil alone (not the no-op-detecting
	// publishStatusChange flag) is enough: Pause/Resume/
	// SetSLAClockTierReached are all idempotent, so a caller re-PATCHing
	// the case's own current state just redundantly re-applies the same
	// effect harmlessly.
	if req.State != nil && snResp.Case.State != nil {
		s.applyCaseStateSLAEffects(ctx, req.ID, resp.Case.State)
	}
	if publishCaseAssign {
		assigneeName := *req.AssigneeEmail
		if snResp.Case.AssignedTo != nil && snResp.Case.AssignedTo.Name != "" {
			assigneeName = snResp.Case.AssignedTo.Name
		}
		s.publishCaseAssigned(ctx, req.ID, assigneeName, *req.AssigneeEmail, caseBeforeAssign)
	}
	// AlreadyAcknowledged distinguishes a genuine first-time claim from a
	// repeat Acknowledge:true call that succeeded without changing anything
	// (see UpdateCaseRequest.Acknowledge's own doc comment) — only the
	// former is a real event worth a Chat alert.
	if req.Acknowledge != nil && *req.Acknowledge && (resp.Case.AlreadyAcknowledged == nil || !*resp.Case.AlreadyAcknowledged) && resp.Case.AcknowledgedBy != nil {
		s.publishCaseAcknowledged(ctx, req.ID, resp.Case.AcknowledgedBy.Name)
	}
	// resp.Case.Severity != caseBeforeSeverity.Severity is a second guard on
	// top of publishSeverityChange itself: that flag only confirms the
	// PATCH *request* asked for a different severity than the pre-PATCH
	// GetCaseByID observed — it says nothing about what the PATCH response
	// actually echoes back. If ServiceNow's response reports the
	// pre-update severity (e.g. a stale echo), publishing anyway would
	// send a false case.severity_changed event with identical old/new
	// values.
	if publishSeverityChange && resp.Case.Severity != "" && resp.Case.Severity != caseBeforeSeverity.Severity {
		s.publishSeverityChanged(ctx, req.ID, string(caseBeforeSeverity.Severity), string(resp.Case.Severity), caseBeforeSeverity)
	}

	return resp, nil
}

type snCreateAttachmentPayload struct {
	ReferenceID   string  `json:"referenceId"`
	ReferenceType string  `json:"referenceType"`
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	File          string  `json:"file"`
	Description   *string `json:"description,omitempty"`
}

type snCreateAttachmentResponse struct {
	Message    string `json:"message"`
	Attachment struct {
		ID          string `json:"id"`
		SizeBytes   int    `json:"sizeBytes"`
		CreatedOn   string `json:"createdOn"`
		CreatedBy   string `json:"createdBy"`
		DownloadURL string `json:"downloadUrl"`
	} `json:"attachment"`
}

const maxAttachmentBytes = 10 * 1024 * 1024 // 10 MB decoded

func (s *snCaseService) CreateCaseAttachment(ctx context.Context, req domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error) {
	if req.Name == "" {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "name is required"}
	}
	if req.Type == "" {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "type is required"}
	}
	if req.File == "" {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "file is required"}
	}

	// file must be a data URI: data:<mime>;base64,<encoded>
	const dataURIPrefix = "data:"
	const base64Marker = ";base64,"
	if !strings.HasPrefix(req.File, dataURIPrefix) {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "file must be a base64 data URI (e.g. data:image/png;base64,...)"}
	}
	markerIdx := strings.Index(req.File, base64Marker)
	if markerIdx == -1 {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "file must be a base64 data URI (e.g. data:image/png;base64,...)"}
	}
	rawBase64 := req.File[markerIdx+len(base64Marker):]

	// Early size guard: decoded size ≈ 3/4 of base64 length. Reject before allocating.
	if len(rawBase64)*3/4 > maxAttachmentBytes {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "file exceeds maximum allowed size of 10 MB"}
	}

	decoded, err := base64.StdEncoding.DecodeString(rawBase64)
	if err != nil {
		// try URL-safe variant
		decoded, err = base64.URLEncoding.DecodeString(rawBase64)
		if err != nil {
			return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "file contains invalid base64 data"}
		}
	}
	if len(decoded) > maxAttachmentBytes {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "file exceeds maximum allowed size of 10 MB"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	if err := validateUUIDs("referenceId", []string{req.ReferenceID}); err != nil {
		return domain.CreateAttachmentResponse{}, err
	}
	if _, ok := validReferenceTypes[req.ReferenceType]; !ok {
		return domain.CreateAttachmentResponse{}, &apierror.ValidationError{Msg: "referenceType is invalid: " + string(req.ReferenceType)}
	}

	payload := snCreateAttachmentPayload{
		ReferenceID:   uuidToSysid(req.ReferenceID),
		ReferenceType: string(req.ReferenceType),
		Name:          req.Name,
		Type:          req.Type,
		File:          rawBase64,
		Description:   req.Description,
	}

	raw, err := s.client.Post(ctx, "/attachments", token, payload)
	if err != nil {
		return domain.CreateAttachmentResponse{}, err
	}

	var snResp snCreateAttachmentResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.CreateAttachmentResponse{}, fmt.Errorf("sn create attachment: parse response: %w", err)
	}

	createdOn, err := time.Parse(snCreatedOnLayout, snResp.Attachment.CreatedOn)
	if err != nil {
		return domain.CreateAttachmentResponse{}, fmt.Errorf("sn create attachment: parse createdOn %q: %w", snResp.Attachment.CreatedOn, err)
	}

	return domain.CreateAttachmentResponse{
		Message: snResp.Message,
		Attachment: domain.AttachmentDetail{
			ID:          sysidToUUID(snResp.Attachment.ID),
			SizeBytes:   snResp.Attachment.SizeBytes,
			CreatedOn:   createdOn,
			CreatedBy:   snResp.Attachment.CreatedBy,
			DownloadURL: &snResp.Attachment.DownloadURL,
			Status:      domain.AttachmentStatusComplete,
		},
	}, nil
}

// ConfirmCaseAttachment implements CaseService. ServiceNow's /attachments API
// only ever returns fully-uploaded files -- there is no pending/in-progress
// upload state to confirm -- so this is a CSM-native (Postgres) data
// source-only operation. See caseService.ConfirmCaseAttachment.
func (s *snCaseService) ConfirmCaseAttachment(_ context.Context, _ string) (domain.ConfirmAttachmentResponse, error) {
	return domain.ConfirmAttachmentResponse{}, &apierror.ServiceUnavailableError{Msg: "confirming an attachment is only supported for the CSM-native data source"}
}

var validReferenceTypes = map[domain.ReferenceType]struct{}{
	domain.ReferenceTypeCase:          {},
	domain.ReferenceTypeConversation:  {},
	domain.ReferenceTypeChangeRequest: {},
	domain.ReferenceTypeDeployment:    {},
	domain.ReferenceTypeIncident:      {},
}

type snSearchAttachmentsPayload struct {
	ReferenceID   string              `json:"referenceId"`
	ReferenceType string              `json:"referenceType"`
	Pagination    snProjectPagination `json:"pagination"`
}

type snAttachment struct {
	ID                string  `json:"id"`
	ReferenceID       string  `json:"referenceId"`
	Name              string  `json:"name"`
	Type              string  `json:"type"`
	SizeBytes         int     `json:"sizeBytes"`
	Description       *string `json:"description"`
	CreatedBy         string  `json:"createdBy"`
	CreatedByFullName string  `json:"createdByFullName"`
	// CreatedByUser is the uploader's resolved sys_user record: nil when the
	// uploader is not a real user and when the ServiceNow side predates the
	// field. See snUserRef.
	CreatedByUser *snUserRef `json:"createdByUser"`
	CreatedOn     string     `json:"createdOn"`
	DownloadURL   *string    `json:"downloadUrl"`
	PreviewURL    *string    `json:"previewUrl"`
}

type snSearchAttachmentsResponse struct {
	Attachments  []snAttachment `json:"attachments"`
	TotalRecords int            `json:"totalRecords"`
	Offset       int            `json:"offset"`
	Limit        int            `json:"limit"`
}

func (s *snCaseService) SearchCaseAttachments(ctx context.Context, req domain.SearchAttachmentsRequest) (domain.SearchAttachmentsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchAttachmentsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	if err := validateUUIDs("referenceId", []string{req.ReferenceID}); err != nil {
		return domain.SearchAttachmentsResponse{}, err
	}
	if _, ok := validReferenceTypes[req.ReferenceType]; !ok {
		return domain.SearchAttachmentsResponse{}, &apierror.ValidationError{Msg: "referenceType is invalid: " + string(req.ReferenceType)}
	}

	payload := snSearchAttachmentsPayload{
		ReferenceID:   uuidToSysid(req.ReferenceID),
		ReferenceType: string(req.ReferenceType),
		Pagination:    snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/attachments/search", token, payload)
	if err != nil {
		return domain.SearchAttachmentsResponse{}, err
	}

	var snResp snSearchAttachmentsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchAttachmentsResponse{}, fmt.Errorf("sn search attachments: parse response: %w", err)
	}

	attachments := make([]domain.Attachment, 0, len(snResp.Attachments))
	for _, a := range snResp.Attachments {
		createdOn, err := time.Parse(snCreatedOnLayout, a.CreatedOn)
		if err != nil {
			return domain.SearchAttachmentsResponse{}, fmt.Errorf("sn search attachments: parse createdOn %q: %w", a.CreatedOn, err)
		}
		attachments = append(attachments, domain.Attachment{
			ID:            sysidToUUID(a.ID),
			ReferenceID:   sysidToUUID(a.ReferenceID),
			ReferenceType: req.ReferenceType,
			Name:          a.Name,
			Type:          a.Type,
			SizeBytes:     a.SizeBytes,
			Description:   a.Description,
			CreatedBy:     snUserReference(a.CreatedByUser, a.CreatedBy, a.CreatedByFullName),
			CreatedOn:     createdOn,
			DownloadURL:   a.DownloadURL,
			PreviewURL:    a.PreviewURL,
			Status:        domain.AttachmentStatusComplete,
		})
	}

	total := snResp.TotalRecords
	return domain.SearchAttachmentsResponse{
		Attachments: attachments,
		Total:       total,
		Limit:       req.Pagination.Limit,
		Offset:      req.Pagination.Offset,
		HasMore:     req.Pagination.Offset+len(attachments) < total,
	}, nil
}

type snSearchActivitiesPayload struct {
	Pagination          snProjectPagination `json:"pagination"`
	IncludeFieldChanges *bool               `json:"includeFieldChanges,omitempty"`
}

type snFieldChange struct {
	Field         string `json:"field"`
	FieldLabel    string `json:"fieldLabel"`
	PreviousValue string `json:"previousValue"`
	NewValue      string `json:"newValue"`
}

type snActivity struct {
	ID                 string          `json:"id"`
	Type               string          `json:"type"`
	Content            string          `json:"content"`
	CreatedOn          string          `json:"createdOn"`
	CreatedBy          string          `json:"createdBy"`
	CreatedByFirstName string          `json:"createdByFirstName"`
	CreatedByLastName  string          `json:"createdByLastName"`
	CreatedByFullName  string          `json:"createdByFullName"`
	CommentType        string          `json:"commentType"`
	FileName           string          `json:"fileName"`
	ContentType        string          `json:"contentType"`
	SizeBytes          int             `json:"sizeBytes"`
	DownloadURL        string          `json:"downloadUrl"`
	Changes            []snFieldChange `json:"changes"`
}

type snSearchActivitiesResponse struct {
	Activity     []snActivity `json:"activity"`
	Offset       int          `json:"offset"`
	Limit        int          `json:"limit"`
	TotalRecords int          `json:"totalRecords"`
}

// mapSNActivitiesToDomain converts a raw ServiceNow activity list into the domain
// representation shared by the case and incident activity feeds -- an activity entry
// (comment, attachment, or field change) is not inherently case-specific.
func mapSNActivitiesToDomain(raw []snActivity) ([]domain.CaseActivity, error) {
	activities := make([]domain.CaseActivity, 0, len(raw))
	for _, a := range raw {
		createdOn, err := time.Parse(snCreatedOnLayout, a.CreatedOn)
		if err != nil {
			return nil, fmt.Errorf("parse createdOn %q: %w", a.CreatedOn, err)
		}
		activity := domain.CaseActivity{
			ID:                 sysidToUUID(a.ID),
			Type:               domain.ActivityType(a.Type),
			Content:            a.Content,
			CreatedOn:          createdOn,
			CreatedByFirstName: a.CreatedByFirstName,
			CreatedByLastName:  a.CreatedByLastName,
			// The activity feed carries no user id for the actor, so the
			// canonical reference is emitted with a null id.
			CreatedBy: domain.NewUserReference("", a.CreatedBy, a.CreatedByFullName),
		}
		switch domain.ActivityType(a.Type) {
		case domain.ActivityTypeComment:
			var ct domain.CommentType
			switch a.CommentType {
			case "comments", "comment":
				ct = domain.CommentTypeComment
			case "work_notes", "work_note":
				ct = domain.CommentTypeWorkNote
			case "activity":
				ct = domain.CommentTypeActivity
			default:
				ct = domain.CommentTypeComment
			}
			activity.CommentType = &ct
		case domain.ActivityTypeAttachment:
			activity.FileName = a.FileName
			activity.ContentType = a.ContentType
			activity.SizeBytes = a.SizeBytes
			activity.DownloadURL = a.DownloadURL
		case domain.ActivityTypeFieldChange:
			changes := make([]domain.FieldChange, 0, len(a.Changes))
			for _, ch := range a.Changes {
				changes = append(changes, domain.FieldChange{
					Field:         ch.Field,
					FieldLabel:    ch.FieldLabel,
					PreviousValue: ch.PreviousValue,
					NewValue:      ch.NewValue,
				})
			}
			activity.Changes = changes
		}
		activities = append(activities, activity)
	}
	return activities, nil
}

func (s *snCaseService) SearchCaseActivities(ctx context.Context, req domain.SearchCaseActivitiesRequest) (domain.SearchCaseActivitiesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCaseActivitiesResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	if err := validateUUIDs("id", []string{req.CaseID}); err != nil {
		return domain.SearchCaseActivitiesResponse{}, err
	}

	payload := snSearchActivitiesPayload{
		Pagination:          snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
		IncludeFieldChanges: req.IncludeFieldChanges,
	}

	raw, err := s.client.Post(ctx, "/cases/"+uuidToSysid(req.CaseID)+"/activities/search", token, payload)
	if err != nil {
		return domain.SearchCaseActivitiesResponse{}, err
	}

	var snResp snSearchActivitiesResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchCaseActivitiesResponse{}, fmt.Errorf("sn search activities: parse response: %w", err)
	}

	activities, err := mapSNActivitiesToDomain(snResp.Activity)
	if err != nil {
		return domain.SearchCaseActivitiesResponse{}, fmt.Errorf("sn search activities: %w", err)
	}

	total := snResp.TotalRecords
	return domain.SearchCaseActivitiesResponse{
		Activity: activities,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(activities) < total,
	}, nil
}

func (s *snCaseService) GetCaseAttachmentContent(ctx context.Context, attachmentID string) ([]byte, string, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	resp, err := s.client.GetBinary(ctx, "/attachments/"+uuidToSysid(attachmentID)+"/content", token)
	if err != nil {
		return nil, "", err
	}

	return resp.Body, resp.ContentType, nil
}

func (s *snCaseService) DeleteCaseAttachment(ctx context.Context, req domain.DeleteAttachmentRequest) (domain.DeleteAttachmentResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Delete(ctx, "/attachments/"+uuidToSysid(req.AttachmentID), token)
	if err != nil {
		return domain.DeleteAttachmentResponse{}, err
	}

	var snResp struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.DeleteAttachmentResponse{}, fmt.Errorf("sn delete attachment: parse response: %w", err)
	}

	return domain.DeleteAttachmentResponse{Message: snResp.Message}, nil
}

// snAttachmentDetails mirrors the Choreo GET /attachments/{id} response
// (Ballerina's AttachmentResponse -- every Attachment field plus content).
// The response does carry createdByUser/createdByFullName, but this lookup
// does not resolve them: createdByUser arrives null, unlike the search path
// (see snAttachment), so CreatedBy stays the bare string below. Do not
// "align" this decode with the search path's UserReference shape on the
// assumption the object is populated here -- it is not. referenceType is
// genuinely absent from this response.
type snAttachmentDetails struct {
	ID          string  `json:"id"`
	ReferenceID string  `json:"referenceId"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	SizeBytes   int     `json:"sizeBytes"`
	Description *string `json:"description"`
	CreatedBy   string  `json:"createdBy"`
	CreatedOn   string  `json:"createdOn"`
	DownloadURL *string `json:"downloadUrl"`
	PreviewURL  *string `json:"previewUrl"`
	Content     string  `json:"content"`
}

func (s *snCaseService) GetAttachmentByID(ctx context.Context, id string) (domain.AttachmentDetails, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.AttachmentDetails{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/attachments/"+uuidToSysid(id), token)
	if err != nil {
		return domain.AttachmentDetails{}, err
	}

	var snResp snAttachmentDetails
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.AttachmentDetails{}, fmt.Errorf("sn get attachment: parse response: %w", err)
	}

	createdOn, err := time.Parse(snCreatedOnLayout, snResp.CreatedOn)
	if err != nil {
		return domain.AttachmentDetails{}, fmt.Errorf("sn get attachment: parse createdOn %q: %w", snResp.CreatedOn, err)
	}

	// ReferenceType is left nil (serialized as JSON null): the upstream
	// attachment-details response carries no reference type (see
	// snAttachmentDetails), and there is no way to derive one from the id
	// alone. Callers must fail closed on it.
	return domain.AttachmentDetails{
		ID:          sysidToUUID(snResp.ID),
		ReferenceID: sysidToUUID(snResp.ReferenceID),
		Name:        snResp.Name,
		Type:        snResp.Type,
		SizeBytes:   snResp.SizeBytes,
		Description: snResp.Description,
		CreatedBy:   snResp.CreatedBy,
		CreatedOn:   createdOn,
		DownloadURL: snResp.DownloadURL,
		PreviewURL:  snResp.PreviewURL,
		Content:     &snResp.Content,
		Status:      domain.AttachmentStatusComplete,
	}, nil
}

// validateAttachmentUpdate mirrors the Ballerina reference's
// validateAttachmentUpdatePayload exactly: referenceType must be case or
// deployment; case requires name and forbids description; deployment
// requires at least one of name or description.
// rawDescriptionPresent reports whether a json.RawMessage description carries
// an actual value. Description is RawMessage so "absent", "explicitly null",
// and "a value" stay distinguishable; only the last counts as present here.
func rawDescriptionPresent(d json.RawMessage) bool {
	if len(d) == 0 {
		return false // absent -- the caller said nothing about description
	}
	if string(d) == "null" {
		// An explicit null is an instruction to clear the description, so it
		// counts as "provided" for the at-least-one-field rule. Collapsing it
		// into "absent" would make clearing a description impossible.
		return true
	}
	var s string
	if err := json.Unmarshal(d, &s); err == nil {
		return strings.TrimSpace(s) != ""
	}
	return true
}

func validateAttachmentUpdate(req domain.UpdateAttachmentRequest) error {
	if req.ReferenceType != domain.ReferenceTypeCase && req.ReferenceType != domain.ReferenceTypeDeployment {
		return &apierror.ValidationError{Msg: fmt.Sprintf("invalid reference type %q. Only 'case' and 'deployment' are allowed", req.ReferenceType)}
	}
	if req.ReferenceType == domain.ReferenceTypeCase {
		if len(req.Description) > 0 {
			return &apierror.ValidationError{Msg: "description field is not allowed for case reference type"}
		}
		if req.Name == nil || strings.TrimSpace(*req.Name) == "" {
			return &apierror.ValidationError{Msg: "name field is required for case reference type"}
		}
	}
	if req.ReferenceType == domain.ReferenceTypeDeployment {
		hasName := req.Name != nil && strings.TrimSpace(*req.Name) != ""
		hasDescription := rawDescriptionPresent(req.Description)
		if !hasName && !hasDescription {
			return &apierror.ValidationError{Msg: "at least one field (name or description) must be provided for deployment reference type"}
		}
	}
	return nil
}

// snUpdateAttachmentPayload mirrors the Choreo PATCH /attachments/{id} request body.
type snUpdateAttachmentPayload struct {
	ReferenceID   string          `json:"referenceId"`
	ReferenceType string          `json:"referenceType"`
	Name          *string         `json:"name,omitempty"`
	Description   json.RawMessage `json:"description,omitempty"`
}

// snUpdateAttachmentResponse mirrors the Choreo PATCH /attachments/{id} response.
type snUpdateAttachmentResponse struct {
	Message    string `json:"message"`
	Attachment struct {
		ID        string `json:"id"`
		UpdatedOn string `json:"updatedOn"`
		UpdatedBy string `json:"updatedBy"`
	} `json:"attachment"`
}

// UpdateAttachment implements CaseService.UpdateAttachment for the ServiceNow data source.
func (s *snCaseService) UpdateAttachment(ctx context.Context, req domain.UpdateAttachmentRequest) (domain.UpdateAttachmentResponse, error) {
	if err := validateUUIDs("id", []string{req.AttachmentID}); err != nil {
		return domain.UpdateAttachmentResponse{}, err
	}
	if err := validateUUIDs("referenceId", []string{req.ReferenceID}); err != nil {
		return domain.UpdateAttachmentResponse{}, err
	}
	if err := validateAttachmentUpdate(req); err != nil {
		return domain.UpdateAttachmentResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snUpdateAttachmentPayload{
		ReferenceID:   uuidToSysid(req.ReferenceID),
		ReferenceType: string(req.ReferenceType),
		Name:          req.Name,
	}
	if len(req.Description) > 0 {
		payload.Description = req.Description
	}

	raw, err := s.client.Patch(ctx, "/attachments/"+uuidToSysid(req.AttachmentID), token, payload)
	if err != nil {
		return domain.UpdateAttachmentResponse{}, err
	}

	var snResp snUpdateAttachmentResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.UpdateAttachmentResponse{}, fmt.Errorf("sn update attachment: parse response: %w", err)
	}

	updatedOn, err := time.Parse(snCreatedOnLayout, snResp.Attachment.UpdatedOn)
	if err != nil {
		return domain.UpdateAttachmentResponse{}, fmt.Errorf("sn update attachment: parse updatedOn %q: %w", snResp.Attachment.UpdatedOn, err)
	}

	return domain.UpdateAttachmentResponse{
		Message: snResp.Message,
		Attachment: domain.UpdatedAttachment{
			ID:        sysidToUUID(snResp.Attachment.ID),
			UpdatedOn: updatedOn,
			UpdatedBy: snResp.Attachment.UpdatedBy,
		},
	}, nil
}

// snCaseFeedbackEmojiRef mirrors the emoji reference embedded in the Choreo
// GET /cases/{id}/feedback response.
type snCaseFeedbackEmojiRef struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	SelectedImage string `json:"selectedImage"`
}

// snCaseFeedbackGetResponse mirrors the Choreo GET /cases/{id}/feedback response.
type snCaseFeedbackGetResponse struct {
	ID                string                 `json:"id"`
	Emoji             snCaseFeedbackEmojiRef `json:"emoji"`
	Chips             []string               `json:"chips"`
	AssessmentID      string                 `json:"assessmentId"`
	CreatedBy         string                 `json:"createdBy"`
	CreatedOn         string                 `json:"createdOn"`
	AdditionalComment *string                `json:"additionalComment"`
}

func (s *snCaseService) GetCaseFeedback(ctx context.Context, id string) (domain.CaseEmojiFeedback, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.CaseEmojiFeedback{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/cases/"+uuidToSysid(id)+"/feedback", token)
	if err != nil {
		return domain.CaseEmojiFeedback{}, err
	}

	var snResp snCaseFeedbackGetResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.CaseEmojiFeedback{}, fmt.Errorf("sn get case feedback: parse response: %w", err)
	}

	chips := make([]string, 0, len(snResp.Chips))
	for _, c := range snResp.Chips {
		chips = append(chips, sysidToUUID(c))
	}

	return domain.CaseEmojiFeedback{
		ID: sysidToUUID(snResp.ID),
		Emoji: domain.CaseFeedbackEmojiRef{
			ID:            sysidToUUID(snResp.Emoji.ID),
			Name:          snResp.Emoji.Name,
			SelectedImage: snResp.Emoji.SelectedImage,
		},
		ChipIDs:           chips,
		AssessmentID:      sysidToUUID(snResp.AssessmentID),
		CreatedBy:         snResp.CreatedBy,
		CreatedOn:         snResp.CreatedOn,
		AdditionalComment: snResp.AdditionalComment,
	}, nil
}

// snSubmitCaseFeedbackPayload mirrors the Choreo POST /cases/{id}/feedback request body.
type snSubmitCaseFeedbackPayload struct {
	EmojiID           string   `json:"emojiId"`
	ChipIDs           []string `json:"chipIds,omitempty"`
	AdditionalComment *string  `json:"additionalComment,omitempty"`
}

// snCaseFeedbackResult mirrors the Choreo CaseFeedbackResult shape.
type snCaseFeedbackResult struct {
	ID           string `json:"id"`
	AssessmentID string `json:"assessmentId"`
	CaseID       string `json:"caseId"`
	CreatedBy    string `json:"createdBy"`
	CreatedOn    string `json:"createdOn"`
}

// snSubmitCaseFeedbackResponse mirrors the Choreo POST /cases/{id}/feedback response.
type snSubmitCaseFeedbackResponse struct {
	Message  string               `json:"message"`
	Feedback snCaseFeedbackResult `json:"feedback"`
}

func (s *snCaseService) SubmitCaseFeedback(ctx context.Context, id string, req domain.SubmitCaseFeedbackRequest) (domain.SubmitCaseFeedbackResponse, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.SubmitCaseFeedbackResponse{}, err
	}
	if err := validateUUIDs("emojiId", []string{req.EmojiID}); err != nil {
		return domain.SubmitCaseFeedbackResponse{}, err
	}
	if err := validateUUIDs("chipIds", req.ChipIDs); err != nil {
		return domain.SubmitCaseFeedbackResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snSubmitCaseFeedbackPayload{
		EmojiID:           uuidToSysid(req.EmojiID),
		ChipIDs:           uuidsToSysids(req.ChipIDs),
		AdditionalComment: req.AdditionalComment,
	}

	raw, err := s.client.Post(ctx, "/cases/"+uuidToSysid(id)+"/feedback", token, payload)
	if err != nil {
		return domain.SubmitCaseFeedbackResponse{}, err
	}

	var snResp snSubmitCaseFeedbackResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SubmitCaseFeedbackResponse{}, fmt.Errorf("sn submit case feedback: parse response: %w", err)
	}

	return domain.SubmitCaseFeedbackResponse{
		Message: snResp.Message,
		Feedback: domain.CaseFeedbackResult{
			ID:           sysidToUUID(snResp.Feedback.ID),
			AssessmentID: sysidToUUID(snResp.Feedback.AssessmentID),
			CaseID:       sysidToUUID(snResp.Feedback.CaseID),
			CreatedBy:    snResp.Feedback.CreatedBy,
			CreatedOn:    snResp.Feedback.CreatedOn,
		},
	}, nil
}

// validateOrGroupEnums validates one AnyOf branch's enum-valued fields
// against the same validXxx maps the top-level (AND-only) filters use,
// mirroring that validation exactly (unrecognized values would otherwise be
// silently dropped by domainStatesToSNIDs/etc's omitempty behavior).
func validateOrGroupEnums(i int, group domain.CaseFilterGroup) error {
	for _, t := range group.Types {
		if _, ok := snCaseTypeMap[t]; !ok {
			return &apierror.ValidationError{Msg: fmt.Sprintf("anyOf[%d]: type contains invalid value: %s", i, t)}
		}
	}
	for _, st := range group.States {
		if !validCaseState[st] {
			return &apierror.ValidationError{Msg: fmt.Sprintf("anyOf[%d]: state contains invalid value: %s", i, st)}
		}
	}
	for _, sv := range group.Severities {
		if !validCaseSeverity[sv] {
			return &apierror.ValidationError{Msg: fmt.Sprintf("anyOf[%d]: severity contains invalid value: %s", i, sv)}
		}
	}
	for _, it := range group.IssueTypes {
		if !validCaseIssueType[it] {
			return &apierror.ValidationError{Msg: fmt.Sprintf("anyOf[%d]: issueType contains invalid value: %s", i, it)}
		}
	}
	for _, et := range group.EngagementTypes {
		if !validEngagementType[et] {
			return &apierror.ValidationError{Msg: fmt.Sprintf("anyOf[%d]: engagementType contains invalid value: %s", i, et)}
		}
	}
	for _, ws := range group.WorkStates {
		if ws != domain.CaseWorkStateOngoing && ws != domain.CaseWorkStatePaused {
			return &apierror.ValidationError{Msg: fmt.Sprintf("anyOf[%d]: workState contains invalid value: %s", i, ws)}
		}
	}
	for _, lvl := range group.EscalationLevels {
		if !validEscalationLevel[lvl] {
			return &apierror.ValidationError{Msg: fmt.Sprintf("anyOf[%d]: escalationLevel contains invalid value: %s", i, lvl)}
		}
	}
	if err := validateUUIDs(fmt.Sprintf("anyOf[%d].assignedUserId", i), group.AssignedUserIDs); err != nil {
		return err
	}
	return nil
}

// snCaseFilterGroup is one ANDed branch of an snCaseFilters.OrGroups entry --
// see domain.CaseFilterGroup doc comment for which fields are supported here.
type snCaseFilterGroup struct {
	CaseTypes          []string `json:"caseTypes,omitempty"`
	StateKeys          []int    `json:"stateKeys,omitempty"`
	SeverityKeys       []int    `json:"severityKeys,omitempty"`
	IssueTypeKeys      []int    `json:"issueTypeKeys,omitempty"`
	EngagementTypeKeys []int    `json:"engagementTypeKeys,omitempty"`
	WorkStateKeys      []int    `json:"workStateKeys,omitempty"`
	ProjectIDs         []string `json:"projectIds,omitempty"`
	DeploymentIDs      []string `json:"deploymentIds,omitempty"`
	AssignedUserIDs    []string `json:"assignedUserIds,omitempty"`
	EscalationLevels   []string `json:"escalationLevel,omitempty"`
	Tags               []string `json:"tags,omitempty"`
	ExcludeTags        []string `json:"excludeTags,omitempty"`
}

// buildSNCaseFilterGroups maps each domain.CaseFilterGroup branch into its SN
// wire shape, reusing the exact same domainXxxToSNIDs/uuidsToSysids
// conversions the top-level filters use.
func buildSNCaseFilterGroups(groups []domain.CaseFilterGroup) []snCaseFilterGroup {
	if len(groups) == 0 {
		return nil
	}
	result := make([]snCaseFilterGroup, 0, len(groups))
	for _, g := range groups {
		result = append(result, snCaseFilterGroup{
			CaseTypes:          domainTypeKeysToSN(g.Types),
			StateKeys:          domainStatesToSNIDs(g.States),
			SeverityKeys:       domainSeveritiesToSNIDs(g.Severities),
			IssueTypeKeys:      domainIssueTypesToSNIDs(g.IssueTypes),
			EngagementTypeKeys: domainEngagementTypesToSNIDs(g.EngagementTypes),
			WorkStateKeys:      domainWorkStatesToSNIDs(g.WorkStates),
			ProjectIDs:         uuidsToSysids(g.ProjectIDs),
			DeploymentIDs:      uuidsToSysids(g.DeploymentIDs),
			AssignedUserIDs:    uuidsToSysids(g.AssignedUserIDs),
			EscalationLevels:   g.EscalationLevels,
			Tags:               g.Tags,
			ExcludeTags:        g.ExcludeTags,
		})
	}
	return result
}

// buildSNCaseFilters maps a domain.ParsedCaseFilters (already translated from
// the generic filters array by ParseCaseFieldFilters) into the exact same
// snCaseFilters payload SearchCases has always sent to the backing service --
// this function changes what feeds the mapping, not the mapping itself.
func buildSNCaseFilters(parsed domain.ParsedCaseFilters, searchQuery string) snCaseFilters {
	// An empty/omitted Types filter means "no type restriction" -- SN's own case
	// search (CaseUtils._resolveCaseTypeIds + applyFilters) already treats an
	// empty caseTypes list this way, skipping the type query entirely. Defaulting
	// it to ["default_case"] here silently excluded every non-"case" type (most
	// importantly service_request) from callers that search across all types,
	// e.g. the "does this engineer already have another ongoing work item"
	// pre-check before starting a new case.
	snCaseTypes := domainTypeKeysToSN(parsed.Types)

	return snCaseFilters{
		CaseTypes:                        snCaseTypes,
		SearchQuery:                      searchQuery,
		ProjectIDs:                       uuidsToSysids(parsed.ProjectIDs),
		DeploymentIDs:                    uuidsToSysids(parsed.DeploymentIDs),
		StateKeys:                        domainStatesToSNIDs(parsed.States),
		ExcludeStates:                    domainStatesToSNIDs(parsed.ExcludeStates),
		SeverityKeys:                     domainSeveritiesToSNIDs(parsed.Severities),
		IssueTypeKeys:                    domainIssueTypesToSNIDs(parsed.IssueTypes),
		EngagementTypeKeys:               domainEngagementTypesToSNIDs(parsed.EngagementTypes),
		ClosedStartDate:                  formatSNDateTimeUTC(parsed.ClosedStartDate),
		ClosedEndDate:                    formatSNDateTimeUTC(parsed.ClosedEndDate),
		ResolvedStartDate:                formatSNDateTimeUTC(parsed.ResolvedStartDate),
		ResolvedEndDate:                  formatSNDateTimeUTC(parsed.ResolvedEndDate),
		StartCreatedDate:                 formatSNDateTimeUTC(parsed.StartCreatedDate),
		EndCreatedDate:                   formatSNDateTimeUTC(parsed.EndCreatedDate),
		StartUpdatedDate:                 formatSNDateTimeUTC(parsed.StartUpdatedDate),
		EndUpdatedDate:                   formatSNDateTimeUTC(parsed.EndUpdatedDate),
		CreatedBy:                        parsed.CreatedBy,
		CreatedByMe:                      parsed.CreatedByMe,
		WorkStateKeys:                    domainWorkStatesToSNIDs(parsed.WorkStates),
		AssignedUserIDs:                  uuidsToSysids(parsed.AssignedUserIDs),
		ProductNames:                     parsed.ProductNames,
		Tags:                             parsed.Tags,
		ExcludeTags:                      parsed.ExcludeTags,
		ParentID:                         snParentIDFilter(parsed.ParentID),
		Number:                           stringPtrValue(parsed.Number),
		InternalID:                       stringPtrValue(parsed.InternalID),
		ProjectOnboardingStatuses:        parsed.ProjectOnboardingStatuses,
		ExcludeProjectOnboardingStatuses: parsed.ExcludeProjectOnboardingStatuses,
		ProjectTypeNames:                 parsed.ProjectTypeNames,
		CreTeamIDs:                       uuidsToSysids(parsed.CreTeamIDs),
		SreTeamIDs:                       uuidsToSysids(parsed.SreTeamIDs),
		AccountIDs:                       uuidsToSysids(parsed.AccountIDs),
		Unassigned:                       parsed.Unassigned,
		ResolutionNotesEmpty:             parsed.ResolutionNotesEmpty,
		TaskSLAFilter:                    buildSNTaskSLAFilter(parsed.TaskSLAFilter),
		EscalationLevels:                 parsed.EscalationLevels,
		IsEscalated:                      parsed.HasActiveEscalation,
		SlaBreached:                      parsed.HasBreachedSLA,
		AccountEscalationActive:          parsed.HasActiveAccountEscalation,
		OrGroups:                         buildSNCaseFilterGroups(parsed.OrGroups),
	}
}

// SearchCases implements CaseService by calling the Choreo POST /cases/search endpoint.
func (s *snCaseService) SearchCases(ctx context.Context, req domain.SearchCasesRequest) (domain.SearchCasesResponse, error) {
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
	req.Parsed = parsed

	orGroups, err := ParseCaseFieldFilterGroups(req.Filters.AnyOf)
	if err != nil {
		return domain.SearchCasesResponse{}, err
	}
	req.Parsed.OrGroups = orGroups

	if req.Parsed.ClosedEndDate != nil && req.Parsed.ClosedStartDate != nil &&
		req.Parsed.ClosedEndDate.Before(*req.Parsed.ClosedStartDate) {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "closedOn: lte value must not be before gte value"}
	}
	if req.Parsed.ResolvedEndDate != nil && req.Parsed.ResolvedStartDate != nil &&
		req.Parsed.ResolvedEndDate.Before(*req.Parsed.ResolvedStartDate) {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "resolvedOn: lte value must not be before gte value"}
	}
	if req.Parsed.EndCreatedDate != nil && req.Parsed.StartCreatedDate != nil &&
		req.Parsed.EndCreatedDate.Before(*req.Parsed.StartCreatedDate) {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "createdOn: lte value must not be before gte value"}
	}
	if req.Parsed.EndUpdatedDate != nil && req.Parsed.StartUpdatedDate != nil &&
		req.Parsed.EndUpdatedDate.Before(*req.Parsed.StartUpdatedDate) {
		return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "updatedOn: lte value must not be before gte value"}
	}

	for _, ws := range req.Parsed.WorkStates {
		if ws != domain.CaseWorkStateOngoing && ws != domain.CaseWorkStatePaused {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "workState contains invalid value: " + string(ws)}
		}
	}
	if err := validateUUIDs("assignedUserId", req.Parsed.AssignedUserIDs); err != nil {
		return domain.SearchCasesResponse{}, err
	}
	if req.Parsed.ParentID != nil {
		if err := validateUUIDs("parentId", []string{*req.Parsed.ParentID}); err != nil {
			return domain.SearchCasesResponse{}, err
		}
	}
	// projectType values are free-text project-type names (no UUID validation);
	// creTeam/sreTeam values are still UUIDs.
	if err := validateUUIDs("creTeam", req.Parsed.CreTeamIDs); err != nil {
		return domain.SearchCasesResponse{}, err
	}
	if err := validateUUIDs("sreTeam", req.Parsed.SreTeamIDs); err != nil {
		return domain.SearchCasesResponse{}, err
	}
	if err := validateUUIDs("accountId", req.Parsed.AccountIDs); err != nil {
		return domain.SearchCasesResponse{}, err
	}

	var snSortBy *snCaseSort
	if req.SortBy.Field != "" {
		snField, ok := snSortFieldMap[req.SortBy.Field]
		if !ok {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "sortBy.field " + string(req.SortBy.Field) + " is not supported by ServiceNow"}
		}
		order := string(req.SortBy.Order)
		if order == "" {
			order = "desc"
		}
		snSortBy = &snCaseSort{Field: snField, Order: order}
	}

	for _, t := range req.Parsed.Types {
		if _, ok := snCaseTypeMap[t]; !ok {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "type contains invalid value: " + t}
		}
	}
	// domainStatesToSNIDs/domainSeveritiesToSNIDs/domainIssueTypesToSNIDs/
	// domainEngagementTypesToSNIDs silently skip unrecognized values, which
	// omitempty then drops from the SN payload entirely -- validate up front
	// so an unrecognized value errors instead of silently widening the result
	// set. Validated against the same validXxx maps the Postgres backend uses
	// (confirmed to cover the exact same value sets as the snXxxIDMap maps
	// this backend converts through).
	for _, st := range req.Parsed.States {
		if !validCaseState[st] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "state contains invalid value: " + string(st)}
		}
	}
	// Same reasoning as States above, and it matters more here: an exclusion
	// value that got silently dropped would widen the result set rather than
	// narrow it, which is the harder failure to notice.
	for _, st := range req.Parsed.ExcludeStates {
		if !validCaseState[st] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "state (notIn) contains invalid value: " + string(st)}
		}
	}
	for _, sv := range req.Parsed.Severities {
		if !validCaseSeverity[sv] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "severity contains invalid value: " + string(sv)}
		}
	}
	for _, it := range req.Parsed.IssueTypes {
		if !validCaseIssueType[it] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "issueType contains invalid value: " + string(it)}
		}
	}
	for _, et := range req.Parsed.EngagementTypes {
		if !validEngagementType[et] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "engagementType contains invalid value: " + string(et)}
		}
	}
	for _, lvl := range req.Parsed.EscalationLevels {
		if !validEscalationLevel[lvl] {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "escalationLevel contains invalid value: " + lvl}
		}
	}
	for i, group := range req.Parsed.OrGroups {
		if err := validateOrGroupEnums(i, group); err != nil {
			return domain.SearchCasesResponse{}, err
		}
	}

	if req.GroupBy != "" {
		values, ok := caseGroupByFieldValues[req.GroupBy]
		if !ok {
			return domain.SearchCasesResponse{}, &apierror.ValidationError{Msg: "groupBy must be one of: state, severity, type, engagementType, issueType, workState"}
		}
		// One limit=1 search per bucket, run concurrently: grouping by state is
		// seven independent round trips to ServiceNow and serialising them made
		// the widget wait for the sum of all seven latencies. Results are written
		// into a pre-sized slice by index, never appended from a goroutine, so the
		// group order stays the caseGroupByFieldValues display order regardless of
		// completion order. errgroup's derived context is passed to every bucket
		// call, so the first failure cancels the rest.
		//
		// The limit is 4: the widest group (state) has seven buckets, and this is
		// a fan-out against a single shared downstream (the ServiceNow integration
		// service) that other requests also use. Four keeps the worst case to two
		// waves while bounding the burst a single request can impose on that
		// downstream; unbounded would let one grouped search open seven concurrent
		// upstream connections.
		const groupCountConcurrency = 4

		groups := make([]domain.CaseGroup, len(values))
		counts := make([]int, len(values))
		g, gCtx := errgroup.WithContext(ctx)
		g.SetLimit(groupCountConcurrency)
		for i, v := range values {
			i, v := i, v
			g.Go(func() error {
				count, err := s.searchCasesGroupCount(gCtx, token, req.GroupBy, v, req.Parsed, req.Filters.SearchQuery)
				if err != nil {
					return err
				}
				counts[i] = count
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return domain.SearchCasesResponse{}, err
		}
		total := 0
		for i, v := range values {
			groups[i] = domain.CaseGroup{Key: v, Count: counts[i]}
			total += counts[i]
		}
		return domain.SearchCasesResponse{
			Groups: groups,
			Total:  total,
			Limit:  req.Pagination.Limit,
			Offset: req.Pagination.Offset,
		}, nil
	}

	snFilters := buildSNCaseFilters(req.Parsed, req.Filters.SearchQuery)

	payload := snCaseSearchPayload{
		Filters:    snFilters,
		SortBy:     snSortBy,
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
		// Requests the account reference, project key, and fix-ETA fields on every
		// row (see snCaseSearchPayload.IncludeExtendedFields doc comment).
		IncludeExtendedFields: true,
	}

	raw, err := s.client.Post(ctx, "/cases/search", token, payload)
	if err != nil {
		return domain.SearchCasesResponse{}, err
	}

	var snResp snCasesResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchCasesResponse{}, fmt.Errorf("sn cases: parse response: %w", err)
	}

	views := make([]domain.SearchCaseView, 0, len(snResp.Cases))
	for _, c := range snResp.Cases {
		title := c.Title
		description := c.Description
		severityLabel := snSeverityLabelStr(c.Severity)
		issueTypeLabel := snIssueTypeLabelStr(c.IssueType)
		workStateLabel := snWorkStateLabelStr(c.WorkState)
		engagementTypeLabel := snLabelStr(c.EngagementType)

		stateLabel := ""
		if c.State != nil {
			stateLabel = c.State.Label
		}
		caseTypeDomain := ""
		if t := snCaseTypeToDomain(c.CaseType); t != nil {
			caseTypeDomain = *t
		}

		updatedOn := c.CreatedOn
		if c.UpdatedOn != nil && *c.UpdatedOn != "" {
			updatedOn = *c.UpdatedOn
		}

		cv := domain.SearchCaseView{
			ID:         sysidToUUID(c.ID),
			Number:     c.Number,
			InternalID: c.InternalID,
			CreatedOn:  c.CreatedOn,
			UpdatedOn:  updatedOn,
			// The case search carries no id for the creator, only the email and
			// full name, so the canonical reference is emitted with a null id.
			CreatedBy:      domain.NewUserReference("", c.CreatedBy, c.CreatedByFullName),
			Subject:        &title,
			Description:    &description,
			IssueType:      issueTypeLabel,
			State:          stateLabel,
			Severity:       severityLabel,
			EngagementType: engagementTypeLabel,
			WorkState:      workStateLabel,
			Type:           caseTypeDomain,
			Project:        domain.EntityRef{ID: sysidToUUID(c.Project.ID), Name: c.Project.Name},
			ProjectKey:     c.Project.Key,
			// BestCaseFixEta/MostLikelyFixEta/WorstCaseFixEta are already date-only
			// "YYYY-MM-DD" strings on both sides, so no parsing/reformatting is
			// needed — nil when includeExtendedFields data is absent from a row.
			BestCaseFixEta:   c.BestCaseFixEta,
			MostLikelyFixEta: c.MostLikelyFixEta,
			WorstCaseFixEta:  c.WorstCaseFixEta,
			EscalationLevel:  snEscalationLevelToDomain(c.EscalationLevel),
		}
		if c.Account != nil {
			cv.AccountDetails = &domain.AccountRef{ID: sysidToUUID(c.Account.ID), Name: c.Account.Name, Type: c.Account.Type}
		}
		if depID := sysidToUUID(c.Deployment.ID); depID != "" {
			cv.Deployment = &domain.EntityRef{ID: depID, Name: c.Deployment.Name}
		}
		if dpID := sysidToUUID(c.DeployedProduct.ID); dpID != "" {
			cv.DeployedProduct = &domain.EntityRef{ID: dpID, Name: strings.TrimSpace(c.DeployedProduct.Name + " " + c.DeployedProduct.Version)}
		}
		if c.Product != nil {
			if id := sysidToUUID(c.Product.ID); id != "" {
				cv.Product = &domain.EntityRef{ID: id, Name: c.Product.Name}
			}
		}
		if c.Catalog != nil {
			if id := sysidToUUID(c.Catalog.ID); id != "" {
				cv.Catalog = &domain.EntityRef{ID: id, Name: c.Catalog.Name}
			}
		}
		if c.CatalogItem != nil {
			if id := sysidToUUID(c.CatalogItem.ID); id != "" {
				cv.CatalogItem = &domain.EntityRef{ID: id, Name: c.CatalogItem.Name}
			}
		}
		if c.AssignedTeam != nil {
			if id := sysidToUUID(c.AssignedTeam.ID); id != "" {
				cv.AssignedTeam = &domain.EntityRef{ID: id, Name: c.AssignedTeam.Name}
			}
		}
		if c.Conversation != nil {
			cv.Conversation = &domain.EntityRef{ID: sysidToUUID(c.Conversation.ID), Name: c.Conversation.Name}
		}
		if c.AssignedEngineer != nil {
			cv.AssignedEngineer = domain.NewUserReference(sysidToUUID(c.AssignedEngineer.ID), snStr(c.AssignedEngineer.Email), c.AssignedEngineer.Name)
		}
		if c.ParentCase != nil {
			cv.ParentCase = &domain.EntityRef{ID: sysidToUUID(c.ParentCase.ID), Name: c.ParentCase.Number}
		}
		if c.RelatedCase != nil {
			cv.RelatedCase = &domain.EntityRef{ID: sysidToUUID(c.RelatedCase.ID), Name: c.RelatedCase.Number}
		}
		views = append(views, cv)
	}

	return domain.SearchCasesResponse{
		Cases:  views,
		Total:  snResp.TotalRecords,
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
	}, nil
}

// snCaseAggregatePayload is the Choreo POST /cases/aggregate request body.
type snCaseAggregatePayload struct {
	Filters   snCaseFilters `json:"filters,omitempty"`
	GroupBy   string        `json:"groupBy"`
	MaxGroups int           `json:"maxGroups,omitempty"`
}

// AggregateCases implements CaseService by calling the Choreo POST
// /cases/aggregate endpoint: a single server-side aggregation over the
// requested field (e.g. account), capped to the top MaxGroups buckets with
// the remainder folded into AggregateResponse.OthersCount. This is distinct
// from SearchCases' own GroupBy, which only supports small fixed-enum
// fields and computes each bucket as a separate client-side search.
//
// Filter parsing and validation mirror SearchCases exactly -- same
// ParseCaseFieldFilters/ParseCaseFieldFilterGroups calls, same enum and
// range checks -- so a request that would be rejected by search is rejected
// here too, rather than silently reaching ServiceNow with a narrower filter
// set than the caller intended.
func (s *snCaseService) AggregateCases(ctx context.Context, req domain.AggregateCasesRequest) (domain.AggregateResponse, error) {
	if req.GroupBy == "" {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy is required"}
	}
	if !validCaseAggregateField[req.GroupBy] {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy contains invalid value: " + req.GroupBy}
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.AggregateResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	callerEmail, callerEmailErr := resolveCaseFilterCallerEmail(token)
	parsed, err := ParseCaseFieldFilters(req.Filters.Filters, callerEmail, callerEmailErr, time.Now().UTC())
	if err != nil {
		return domain.AggregateResponse{}, err
	}

	orGroups, err := ParseCaseFieldFilterGroups(req.Filters.AnyOf)
	if err != nil {
		return domain.AggregateResponse{}, err
	}
	parsed.OrGroups = orGroups

	if parsed.ClosedEndDate != nil && parsed.ClosedStartDate != nil &&
		parsed.ClosedEndDate.Before(*parsed.ClosedStartDate) {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "closedOn: lte value must not be before gte value"}
	}
	if parsed.ResolvedEndDate != nil && parsed.ResolvedStartDate != nil &&
		parsed.ResolvedEndDate.Before(*parsed.ResolvedStartDate) {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "resolvedOn: lte value must not be before gte value"}
	}
	if parsed.EndCreatedDate != nil && parsed.StartCreatedDate != nil &&
		parsed.EndCreatedDate.Before(*parsed.StartCreatedDate) {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "createdOn: lte value must not be before gte value"}
	}
	if parsed.EndUpdatedDate != nil && parsed.StartUpdatedDate != nil &&
		parsed.EndUpdatedDate.Before(*parsed.StartUpdatedDate) {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "updatedOn: lte value must not be before gte value"}
	}
	for _, ws := range parsed.WorkStates {
		if ws != domain.CaseWorkStateOngoing && ws != domain.CaseWorkStatePaused {
			return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "workState contains invalid value: " + string(ws)}
		}
	}
	if err := validateUUIDs("assignedUserId", parsed.AssignedUserIDs); err != nil {
		return domain.AggregateResponse{}, err
	}
	if parsed.ParentID != nil {
		if err := validateUUIDs("parentId", []string{*parsed.ParentID}); err != nil {
			return domain.AggregateResponse{}, err
		}
	}
	if err := validateUUIDs("creTeam", parsed.CreTeamIDs); err != nil {
		return domain.AggregateResponse{}, err
	}
	if err := validateUUIDs("sreTeam", parsed.SreTeamIDs); err != nil {
		return domain.AggregateResponse{}, err
	}
	if err := validateUUIDs("accountId", parsed.AccountIDs); err != nil {
		return domain.AggregateResponse{}, err
	}
	for _, t := range parsed.Types {
		if _, ok := snCaseTypeMap[t]; !ok {
			return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "type contains invalid value: " + t}
		}
	}
	for _, st := range parsed.States {
		if !validCaseState[st] {
			return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "state contains invalid value: " + string(st)}
		}
	}
	for _, st := range parsed.ExcludeStates {
		if !validCaseState[st] {
			return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "state (notIn) contains invalid value: " + string(st)}
		}
	}
	for _, sv := range parsed.Severities {
		if !validCaseSeverity[sv] {
			return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "severity contains invalid value: " + string(sv)}
		}
	}
	for _, it := range parsed.IssueTypes {
		if !validCaseIssueType[it] {
			return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "issueType contains invalid value: " + string(it)}
		}
	}
	for _, et := range parsed.EngagementTypes {
		if !validEngagementType[et] {
			return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "engagementType contains invalid value: " + string(et)}
		}
	}
	for _, lvl := range parsed.EscalationLevels {
		if !validEscalationLevel[lvl] {
			return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "escalationLevel contains invalid value: " + lvl}
		}
	}
	for i, group := range parsed.OrGroups {
		if err := validateOrGroupEnums(i, group); err != nil {
			return domain.AggregateResponse{}, err
		}
	}

	snFilters := buildSNCaseFilters(parsed, req.Filters.SearchQuery)

	payload := snCaseAggregatePayload{
		Filters:   snFilters,
		GroupBy:   req.GroupBy,
		MaxGroups: req.MaxGroups,
	}

	raw, err := s.client.Post(ctx, "/cases/aggregate", token, payload)
	if err != nil {
		return domain.AggregateResponse{}, err
	}

	var resp domain.AggregateResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return domain.AggregateResponse{}, fmt.Errorf("sn cases: parse aggregate response: %w", err)
	}
	// "account" is the only ID-valued field in validCaseAggregateField; SN
	// returns its bucket keys as raw sys_ids, so convert them to this
	// platform's UUIDs before returning. Every other allowed field (state,
	// severity, type) is a plain enum and is left as-is.
	if req.GroupBy == "account" {
		for i := range resp.Groups {
			resp.Groups[i].Key = sysidToUUID(resp.Groups[i].Key)
		}
	}
	return resp, nil
}

// searchCasesGroupCount returns the total matching-record count for one
// groupBy bucket: parsed's existing filters, with any prior filter on
// groupByField replaced by exactly value (a groupBy request describes the
// whole breakdown of that field, not a further-narrowed slice of it). Only
// limit=1 is requested since the count comes from SN's totalRecords
// regardless of page size -- used by SearchCases's groupBy fan-out.
func (s *snCaseService) searchCasesGroupCount(ctx context.Context, token, groupByField, value string, parsed domain.ParsedCaseFilters, searchQuery string) (int, error) {
	switch groupByField {
	case "state":
		parsed.States = []domain.CaseState{domain.CaseState(value)}
	case "severity":
		parsed.Severities = []domain.CaseSeverity{domain.CaseSeverity(value)}
	case "type":
		parsed.Types = []string{value}
	case "engagementType":
		parsed.EngagementTypes = []domain.EngagementType{domain.EngagementType(value)}
	case "issueType":
		parsed.IssueTypes = []domain.CaseIssueType{domain.CaseIssueType(value)}
	case "workState":
		parsed.WorkStates = []domain.CaseWorkState{domain.CaseWorkState(value)}
	}

	snFilters := buildSNCaseFilters(parsed, searchQuery)
	payload := snCaseSearchPayload{
		Filters:    snFilters,
		Pagination: snProjectPagination{Limit: 1, Offset: 0},
	}
	raw, err := s.client.Post(ctx, "/cases/search", token, payload)
	if err != nil {
		return 0, err
	}
	var snResp snCasesResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return 0, fmt.Errorf("sn cases: parse grouped response: %w", err)
	}
	return snResp.TotalRecords, nil
}

// snSeverityLabelStr returns the raw SN severity label string, or nil if absent.
func snSeverityLabelStr(s *snCaseLabel) *string {
	if s == nil {
		return nil
	}
	return &s.Label
}

// snIssueTypeLabelStr returns the raw SN issue-type label string, or nil if absent.
func snIssueTypeLabelStr(it *snCaseIssueType) *string {
	if it == nil {
		return nil
	}
	return &it.Label
}

// snLabelStr returns the label of an snCaseLabel, or nil if absent.
func snLabelStr(l *snCaseLabel) *string {
	if l == nil {
		return nil
	}
	return &l.Label
}

// snWorkStateLabelStr normalizes an SN work-state label to the lowercased
// domain enum value ("ongoing"/"paused") as a string, or nil when absent or
// unrecognised. The search view carries workState as a plain string, but it
// must match the enum the detail endpoint returns (GET /cases/{id} maps via
// snWorkStateLabelToEnum) so both paths agree on casing. Returning the raw SN
// label here (e.g. "Ongoing") breaks clients that gate on the lowercased value.
func snWorkStateLabelStr(ws *snCaseLabel) *string {
	e := snWorkStateLabelToEnum(ws)
	if e == nil {
		return nil
	}
	s := string(*e)
	return &s
}

// snCaseStateMap maps SN state labels (lowercased) to domain CaseState enums.
var snCaseStateMap = map[string]domain.CaseState{
	"open":              domain.CaseStateOpen,
	"work in progress":  domain.CaseStateWorkInProgress,
	"waiting on wso2":   domain.CaseStateWaitingOnWSO2,
	"awaiting info":     domain.CaseStateAwaitingInfo,
	"reopened":          domain.CaseStateWaitingOnWSO2,
	"solution proposed": domain.CaseStateSolutionProposed,
	"closed":            domain.CaseStateClosed,
}

func snCaseStateLabelToEnum(state *snCaseState) (domain.CaseState, error) {
	if state == nil {
		return domain.CaseStateOpen, nil
	}
	if v, ok := snCaseStateMap[strings.ToLower(state.Label)]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unknown case state %q from ServiceNow", state.Label)
}

// snSeverityLabel extracts the priority word from SN severity labels like
// "Low (P4)", "2 - High", "3 - Moderate" → "low", "high", "medium".
var snSeverityLabelMap = map[string]domain.CaseSeverity{
	"catastrophic": domain.CaseSeverityCatastrophic,
	"critical":     domain.CaseSeverityCritical,
	"high":         domain.CaseSeverityHigh,
	"moderate":     domain.CaseSeverityMedium,
	"medium":       domain.CaseSeverityMedium,
	"low":          domain.CaseSeverityLow,
}

func snSeverityToSeverity(severity *snCaseLabel) domain.CaseSeverity {
	if severity == nil {
		return ""
	}
	// Labels arrive as e.g. "Low (P4)" or "2 - High"; scan words for a known priority.
	for _, word := range strings.Fields(severity.Label) {
		if p, ok := snSeverityLabelMap[strings.ToLower(strings.Trim(word, "(),"))]; ok {
			return p
		}
	}
	return ""
}

func snIssueTypeToEnum(issueType *snCaseIssueType) domain.CaseIssueType {
	if issueType == nil {
		return ""
	}
	// SN sends issueType.name as the human label e.g. "Error", "Total Outage".
	it := domain.CaseIssueType(strings.ToLower(strings.ReplaceAll(issueType.Label, " ", "_")))
	if validCaseIssueType[it] {
		return it
	}
	return ""
}

func snWorkStateLabelToEnum(ws *snCaseLabel) *domain.CaseWorkState {
	if ws == nil {
		return nil
	}
	v := domain.CaseWorkState(strings.ToLower(ws.Label))
	switch v {
	case domain.CaseWorkStateOngoing, domain.CaseWorkStatePaused:
		return &v
	default:
		return nil
	}
}

// snAddTagPayload is the Choreo POST /cases/{id}/tags request body. SN's tagging is
// the generic platform label/label_entry mechanism (table-agnostic, not a case
// column), backed by the sys_label / label_entry tables scoped to
// reference_table="sn_customerservice_case".
type snAddTagPayload struct {
	Label string `json:"label"`
}

// snTag mirrors the Choreo tag shape.
type snTag struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	Color *string `json:"color"`
}

type snAddTagResponse struct {
	Message string `json:"message"`
	Tag     snTag  `json:"tag"`
}

// AddCaseTag attaches a free-text label to the case identified by caseID.
func (s *snCaseService) AddCaseTag(ctx context.Context, caseID, label string) (domain.Tag, error) {
	if err := validateUUIDs("id", []string{caseID}); err != nil {
		return domain.Tag{}, err
	}
	if strings.TrimSpace(label) == "" {
		return domain.Tag{}, &apierror.ValidationError{Msg: "label is required"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snAddTagPayload{Label: label}
	raw, err := s.client.Post(ctx, "/cases/"+uuidToSysid(caseID)+"/tags", token, payload)
	if err != nil {
		return domain.Tag{}, err
	}

	var snResp snAddTagResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.Tag{}, fmt.Errorf("sn add case tag: parse response: %w", err)
	}

	return domain.Tag{
		ID:    sysidToUUID(snResp.Tag.ID),
		Label: snResp.Tag.Label,
		Color: snResp.Tag.Color,
	}, nil
}

// RemoveCaseTag removes the tag identified by tagID from the case identified by caseID.
func (s *snCaseService) RemoveCaseTag(ctx context.Context, caseID, tagID string) error {
	if err := validateUUIDs("id", []string{caseID}); err != nil {
		return err
	}
	if err := validateUUIDs("tagId", []string{tagID}); err != nil {
		return err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	_, err := s.client.Delete(ctx, "/cases/"+uuidToSysid(caseID)+"/tags/"+uuidToSysid(tagID), token)
	return err
}

// snSearchTagsFilters holds the filter fields of the Choreo POST /tags/search request body.
// CaseID scopes the search to labels already used on one case; the entity service never sets
// it (nothing upstream consumes the case-scoped variant), but it is part of the wire contract.
type snSearchTagsFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
	CaseID      string `json:"caseId,omitempty"`
}

// snSearchTagsPayload is the Choreo POST /tags/search request body.
type snSearchTagsPayload struct {
	Filters snSearchTagsFilters `json:"filters"`
	Limit   int                 `json:"limit,omitempty"`
}

// snSearchTagsResponse mirrors the Choreo POST /tags/search response, and the shape of the
// case-scoped GET /cases/{id}/tags response consumed by listCaseTags below.
type snSearchTagsResponse struct {
	Tags []snTag `json:"tags"`
}

// listCaseTags returns the tags currently attached to the case identified by caseID, via the
// case-scoped GET /cases/{id}/tags resource.
func (s *snCaseService) listCaseTags(ctx context.Context, caseID string) ([]domain.Tag, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/cases/"+uuidToSysid(caseID)+"/tags", token)
	if err != nil {
		return nil, err
	}

	var snResp snSearchTagsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return nil, fmt.Errorf("sn list case tags: parse response: %w", err)
	}

	tags := make([]domain.Tag, 0, len(snResp.Tags))
	for _, t := range snResp.Tags {
		tags = append(tags, domain.Tag{
			ID:    sysidToUUID(t.ID),
			Label: t.Label,
			Color: t.Color,
		})
	}
	return tags, nil
}

// SearchTags returns the tags (not scoped to any single case) whose label matches query, for
// FE autocomplete when attaching a tag to a case. SN's tagging is the generic platform label
// mechanism (table-agnostic, not a case column), backed by the sys_label table (optionally
// scoped to labels used against reference_table="sn_customerservice_case" label_entry rows).
func (s *snCaseService) SearchTags(ctx context.Context, req domain.SearchTagsRequest) ([]domain.Tag, error) {
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return nil, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snSearchTagsPayload{
		Filters: snSearchTagsFilters{SearchQuery: req.Filters.SearchQuery},
	}
	if req.Limit > 0 {
		payload.Limit = req.Limit
	}

	raw, err := s.client.Post(ctx, "/tags/search", token, payload)
	if err != nil {
		return nil, err
	}

	var snResp snSearchTagsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return nil, fmt.Errorf("sn search tags: parse response: %w", err)
	}

	tags := make([]domain.Tag, 0, len(snResp.Tags))
	for _, t := range snResp.Tags {
		tags = append(tags, domain.Tag{
			ID:    sysidToUUID(t.ID),
			Label: t.Label,
			Color: t.Color,
		})
	}
	return tags, nil
}
