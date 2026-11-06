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

package dto

import (
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// This file translates between entity-service's raw enum vocabulary
// (UPPER_SNAKE_CASE, matching its Postgres enum columns) and the exact
// strings the Verification feature's requirements doc (§4 data model)
// specifies for the frontend: addedReason as "auto-closed"/"manual",
// previousStatus as "Solution Proposed"/"Awaiting Info", and recordType as
// the record's display name ("Security Report", not "SECURITY_REPORT_ANALYSIS").
// Same pattern as case_enum_mapping.go and friends -- a small,
// hand-maintained table at the one place this backend builds/reads the
// enum, so entity-service's own vocabulary never leaks to the frontend.

var pendingVerificationAddedReasonToPortal = map[string]string{
	"AUTO_CLOSED": "auto-closed",
	"MANUAL":      "manual",
}

var pendingVerificationAddedReasonToEntity = map[string]string{
	"auto-closed": "AUTO_CLOSED",
	"manual":      "MANUAL",
}

var pendingVerificationPreviousStatusToPortal = map[string]string{
	"SOLUTION_PROPOSED": "Solution Proposed",
	"AWAITING_INFO":     "Awaiting Info",
}

var pendingVerificationPreviousStatusToEntity = map[string]string{
	"Solution Proposed": "SOLUTION_PROPOSED",
	"Awaiting Info":     "AWAITING_INFO",
}

// pendingVerificationRecordTypeToPortal mirrors the six work_item_type_enum
// values this feature is scoped to (§2 of the requirements doc) -- entity-service's
// own validPendingVerificationRecordType map lists the same six.
var pendingVerificationRecordTypeToPortal = map[string]string{
	"CASE":                     "Case",
	"ENGAGEMENT":               "Engagement",
	"SECURITY_REPORT_ANALYSIS": "Security Report",
	"SERVICE_REQUEST":          "Service Request",
	"ANNOUNCEMENT":             "Announcement",
	"CHANGE_REQUEST":           "Change Request",
}

var pendingVerificationRecordTypeToEntity = map[string]string{
	"Case":            "CASE",
	"Engagement":      "ENGAGEMENT",
	"Security Report": "SECURITY_REPORT_ANALYSIS",
	"Service Request": "SERVICE_REQUEST",
	"Announcement":    "ANNOUNCEMENT",
	"Change Request":  "CHANGE_REQUEST",
}

// PendingVerificationCreateRequest is the portal's request shape for
// POST /pending-verifications. AddedBy is never accepted here -- entity-service
// resolves it from the caller's own token, the same way it already refuses
// a client-supplied value.
type PendingVerificationCreateRequest struct {
	WorkItemID     string  `json:"workItemId"`
	AddedReason    string  `json:"addedReason"`
	PreviousStatus *string `json:"previousStatus,omitempty"`
	Note           *string `json:"note,omitempty"`
}

// BuildEntityCreatePendingVerificationRequest converts the portal's create
// request into entity-service's request shape. An addedReason/previousStatus
// value this backend doesn't recognise is passed through unchanged rather
// than dropped silently -- entity-service's own enum validation then rejects
// it with a 400 naming the bad value, instead of this layer masking a typo
// as some other field being "missing".
func BuildEntityCreatePendingVerificationRequest(req PendingVerificationCreateRequest) entity.CreatePendingVerificationRequest {
	out := entity.CreatePendingVerificationRequest{
		WorkItemID:  req.WorkItemID,
		AddedReason: mapOrPassthrough(req.AddedReason, pendingVerificationAddedReasonToEntity),
		Note:        req.Note,
	}
	if req.PreviousStatus != nil {
		mapped := mapOrPassthrough(*req.PreviousStatus, pendingVerificationPreviousStatusToEntity)
		out.PreviousStatus = &mapped
	}
	return out
}

// mapOrPassthrough looks up v in m, returning v unchanged if it has no entry
// -- see BuildEntityCreatePendingVerificationRequest's own doc comment for why.
func mapOrPassthrough(v string, m map[string]string) string {
	if mapped, ok := m[v]; ok {
		return mapped
	}
	return v
}

// PendingVerificationView is the portal's response shape for a single
// pending-verification entry. Field names match the requirements doc's §4
// data model exactly (caseId/caseTitle/addedAt), even for a non-Case record
// -- the doc uses these names generically across every in-scope record type.
// caseId is the record's human-readable number (e.g. "CS0441731"), for
// display only -- workItemId is the actual work_item UUID, for navigating
// back into the record's own detail page.
type PendingVerificationView struct {
	ID             string     `json:"id"`
	WorkItemID     string     `json:"workItemId"`
	CaseID         string     `json:"caseId"`
	InternalID     *string    `json:"internalId,omitempty"`
	CaseTitle      string     `json:"caseTitle"`
	Severity       *string    `json:"severity,omitempty"`
	RecordType     string     `json:"recordType"`
	AddedReason    string     `json:"addedReason"`
	PreviousStatus *string    `json:"previousStatus,omitempty"`
	AddedAt        time.Time  `json:"addedAt"`
	AddedBy        string     `json:"addedBy"`
	VerifiedAt     *time.Time `json:"verifiedAt,omitempty"`
	VerifiedBy     *string    `json:"verifiedBy,omitempty"`
	Note           *string    `json:"note,omitempty"`
}

// MapPendingVerification builds the portal response from entity-service's
// PendingVerification.
func MapPendingVerification(pv entity.PendingVerification) PendingVerificationView {
	view := PendingVerificationView{
		ID:          pv.ID,
		WorkItemID:  pv.WorkItemID,
		CaseID:      pv.RecordNumber,
		InternalID:  pv.InternalID,
		CaseTitle:   pv.RecordTitle,
		Severity:    pv.Severity,
		RecordType:  mapOrPassthrough(pv.RecordType, pendingVerificationRecordTypeToPortal),
		AddedReason: mapOrPassthrough(pv.AddedReason, pendingVerificationAddedReasonToPortal),
		AddedAt:     pv.AddedOn,
		AddedBy:     pv.AddedBy,
		VerifiedAt:  pv.VerifiedOn,
		VerifiedBy:  pv.VerifiedBy,
		Note:        pv.Note,
	}
	if pv.PreviousStatus != nil {
		mapped := mapOrPassthrough(*pv.PreviousStatus, pendingVerificationPreviousStatusToPortal)
		view.PreviousStatus = &mapped
	}
	return view
}

// PendingVerificationCreateResponse is the portal's response for
// POST /pending-verifications.
type PendingVerificationCreateResponse struct {
	Message             string                  `json:"message"`
	PendingVerification PendingVerificationView `json:"pendingVerification"`
}

// MapPendingVerificationCreate builds the portal response from entity-service's
// CreatePendingVerificationResponse.
func MapPendingVerificationCreate(r entity.CreatePendingVerificationResponse) PendingVerificationCreateResponse {
	return PendingVerificationCreateResponse{
		Message:             r.Message,
		PendingVerification: MapPendingVerification(r.PendingVerification),
	}
}

// PendingVerificationSearchFilters is the portal's filter shape for
// POST /pending-verifications/search. RecordTypes takes the doc's display
// labels ("Security Report", not "SECURITY_REPORT_ANALYSIS") -- translated
// to entity-service's enum vocabulary by BuildEntitySearchPendingVerificationsRequest.
// VerifiedOnly, when true, overrides IncludeVerified and narrows to verified
// rows only, deduped to the single most-recently-verified row per work item
// -- entity-service's own SearchPendingVerificationsFilters.VerifiedOnly doc
// comment has the full reasoning (this is what backs the list page's
// Verified tab: one card per case, not one per historical verification
// round -- the full multi-round history stays available, undeduped, via
// IncludeVerified=true).
type PendingVerificationSearchFilters struct {
	WorkItemID  *string  `json:"workItemId,omitempty"`
	RecordTypes []string `json:"recordTypes,omitempty"`
	// AddedReason takes the doc's display values ("auto-closed"/"manual"),
	// translated to entity-service's enum vocabulary below -- backs the
	// Support page's Auto-closed/Manual stat boxes, which link straight into
	// a pre-filtered view of the list.
	AddedReason     *string `json:"addedReason,omitempty"`
	SearchQuery     string  `json:"searchQuery,omitempty"`
	IncludeVerified bool    `json:"includeVerified,omitempty"`
	VerifiedOnly    bool    `json:"verifiedOnly,omitempty"`
}

// PendingVerificationSearchRequest is the portal's request shape for
// POST /pending-verifications/search. ProjectID is required -- the Pending
// Verification list is always project-scoped (§9 of the requirements doc).
type PendingVerificationSearchRequest struct {
	ProjectID  string                           `json:"projectId"`
	Pagination entity.Pagination                `json:"pagination"`
	Filters    PendingVerificationSearchFilters `json:"filters"`
}

// BuildEntitySearchPendingVerificationsRequest converts the portal's search
// request into entity-service's request shape.
func BuildEntitySearchPendingVerificationsRequest(req PendingVerificationSearchRequest) entity.SearchPendingVerificationsRequest {
	workItemTypes := make([]string, 0, len(req.Filters.RecordTypes))
	for _, t := range req.Filters.RecordTypes {
		workItemTypes = append(workItemTypes, mapOrPassthrough(t, pendingVerificationRecordTypeToEntity))
	}
	var addedReason *string
	if req.Filters.AddedReason != nil {
		mapped := mapOrPassthrough(*req.Filters.AddedReason, pendingVerificationAddedReasonToEntity)
		addedReason = &mapped
	}
	return entity.SearchPendingVerificationsRequest{
		Pagination: req.Pagination,
		Filters: entity.PendingVerificationSearchFilters{
			ProjectID:       req.ProjectID,
			WorkItemID:      req.Filters.WorkItemID,
			WorkItemTypes:   workItemTypes,
			AddedReason:     addedReason,
			SearchQuery:     req.Filters.SearchQuery,
			IncludeVerified: req.Filters.IncludeVerified,
			VerifiedOnly:    req.Filters.VerifiedOnly,
		},
	}
}

// PendingVerificationTypeCountView is the portal's shape for one row of the
// type-filter dropdown's per-type counts.
type PendingVerificationTypeCountView struct {
	RecordType string `json:"recordType"`
	Count      int    `json:"count"`
}

// PendingVerificationSearchResponse is the portal's response for
// POST /pending-verifications/search. TotalRecords (not Total) matches the
// frontend's shared pagination envelope -- see backend-v2's CLAUDE.md
// ("All pagination responses use totalRecords").
type PendingVerificationSearchResponse struct {
	PendingVerifications []PendingVerificationView          `json:"pendingVerifications"`
	TotalRecords         int                                `json:"totalRecords"`
	AutoClosedCount      int                                `json:"autoClosedCount"`
	ManualCount          int                                `json:"manualCount"`
	TypeCounts           []PendingVerificationTypeCountView `json:"typeCounts"`
	Limit                int                                `json:"limit"`
	Offset               int                                `json:"offset"`
	HasMore              bool                               `json:"hasMore"`
}

// MapPendingVerificationSearch builds the portal response from entity-service's
// SearchPendingVerificationsResponse.
func MapPendingVerificationSearch(r entity.SearchPendingVerificationsResponse) PendingVerificationSearchResponse {
	entries := make([]PendingVerificationView, 0, len(r.PendingVerifications))
	for _, pv := range r.PendingVerifications {
		entries = append(entries, MapPendingVerification(pv))
	}
	typeCounts := make([]PendingVerificationTypeCountView, 0, len(r.TypeCounts))
	for _, tc := range r.TypeCounts {
		typeCounts = append(typeCounts, PendingVerificationTypeCountView{
			RecordType: mapOrPassthrough(tc.RecordType, pendingVerificationRecordTypeToPortal),
			Count:      tc.Count,
		})
	}
	return PendingVerificationSearchResponse{
		PendingVerifications: entries,
		TotalRecords:         r.Total,
		AutoClosedCount:      r.AutoClosedCount,
		ManualCount:          r.ManualCount,
		TypeCounts:           typeCounts,
		Limit:                r.Limit,
		Offset:               r.Offset,
		HasMore:              r.HasMore,
	}
}

// PendingVerificationVerifyResponse is the portal's response for
// PATCH /pending-verifications/{id}.
type PendingVerificationVerifyResponse struct {
	Message             string                  `json:"message"`
	PendingVerification PendingVerificationView `json:"pendingVerification"`
}

// MapPendingVerificationVerify builds the portal response from entity-service's
// VerifyPendingVerificationResponse.
func MapPendingVerificationVerify(r entity.VerifyPendingVerificationResponse) PendingVerificationVerifyResponse {
	return PendingVerificationVerifyResponse{
		Message:             r.Message,
		PendingVerification: MapPendingVerification(r.PendingVerification),
	}
}
