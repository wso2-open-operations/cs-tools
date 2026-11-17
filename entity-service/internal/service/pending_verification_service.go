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
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// validPendingVerificationAddedReason mirrors the DB's
// pending_verification_added_reason_enum; add an entry here whenever a new
// enum value is added there.
var validPendingVerificationAddedReason = map[domain.PendingVerificationAddedReason]bool{
	domain.PendingVerificationAddedReasonAutoClosed: true,
	domain.PendingVerificationAddedReasonManual:     true,
}

// validPendingVerificationPreviousStatus mirrors the DB's
// pending_verification_previous_status_enum.
var validPendingVerificationPreviousStatus = map[domain.PendingVerificationPreviousStatus]bool{
	domain.PendingVerificationPreviousStatusSolutionProposed: true,
	domain.PendingVerificationPreviousStatusAwaitingInfo:     true,
	domain.PendingVerificationPreviousStatusWorkInProgress:   true,
}

// validPendingVerificationRecordType narrows filters.workItemTypes to CASE,
// the only type CreatePendingVerification actually ever accepts (product
// decision, 2026-10-06). The requirements doc's §2 scope table originally
// named five more types (Engagement, Security Report, Service Request,
// Announcement, Change Request), but the feature was since scoped down to
// Case only before any code accepting those other types was built -- and
// buildSearchWhere's own query unconditionally ANDs wi.type = 'CASE' onto
// every search regardless of this filter, so a caller sending e.g.
// workItemTypes: ["ENGAGEMENT"] used to pass this validation and then
// silently get an empty result forever, with no way to tell "no matches"
// apart from "this filter value doesn't do anything here". Rejecting the
// other five outright is honest about what's actually implemented.
var validPendingVerificationRecordType = map[string]bool{
	"CASE": true,
}

// addedBy/verifiedBy always come from resolveCallerEmail (account_contact_service.go's
// shared helper) — this service has no other identity layer, and a
// client-supplied value in the request body would let one caller attribute
// an entry to someone else.

type pendingVerificationService struct {
	repo   repository.PendingVerificationRepository
	access AccessService
}

// NewPendingVerificationService constructs a PendingVerificationService
// backed by Postgres. access is used to require an internal caller for
// AUTO_CLOSED entries -- see CreatePendingVerification's own comment.
func NewPendingVerificationService(repo repository.PendingVerificationRepository, access AccessService) PendingVerificationService {
	return &pendingVerificationService{repo: repo, access: access}
}

// CreatePendingVerification implements PendingVerificationService.
func (s *pendingVerificationService) CreatePendingVerification(ctx context.Context, req domain.CreatePendingVerificationRequest) (domain.CreatePendingVerificationResponse, error) {
	if err := validateUUIDs("workItemId", []string{req.WorkItemID}); err != nil {
		return domain.CreatePendingVerificationResponse{}, err
	}
	if !validPendingVerificationAddedReason[req.AddedReason] {
		return domain.CreatePendingVerificationResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("addedReason must be one of auto-closed/manual, got %q", req.AddedReason)}
	}

	// previousStatus is required iff addedReason is auto-closed — mirrors
	// the table's own chk_previous_status_matches_reason CHECK constraint,
	// checked here too so the caller gets a clear 400 instead of a raw
	// constraint-violation error.
	isAutoClosed := req.AddedReason == domain.PendingVerificationAddedReasonAutoClosed
	if isAutoClosed && req.PreviousStatus == nil {
		return domain.CreatePendingVerificationResponse{}, &apierror.ValidationError{Msg: "previousStatus is required when addedReason is auto-closed"}
	}
	if !isAutoClosed && req.PreviousStatus != nil {
		return domain.CreatePendingVerificationResponse{}, &apierror.ValidationError{Msg: "previousStatus must not be set when addedReason is manual"}
	}
	if req.PreviousStatus != nil && !validPendingVerificationPreviousStatus[*req.PreviousStatus] {
		return domain.CreatePendingVerificationResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("previousStatus must be one of Solution Proposed/Awaiting Info/Work In Progress, got %q", *req.PreviousStatus)}
	}

	// AUTO_CLOSED claims the system itself closed the case -- only an
	// internal caller may create one, or a registered customer could forge a
	// false system-generated auto-closure entry (migration 100025's RLS
	// policy only checks project membership, never added_reason). MANUAL
	// stays open to any project member, gated below by the usual
	// membership/feature checks only.
	if isAutoClosed {
		if err := RequireInternalCaller(ctx, s.access, "only internal users may add an auto-closed entry to Pending Verification"); err != nil {
			return domain.CreatePendingVerificationResponse{}, err
		}
	}

	// The feature is scoped to Case only (product decision, 2026-10-06) --
	// read through the same Scoped/RLS-protected lookup every other caller
	// uses, so a work item this caller can't even see fails exactly like one
	// that doesn't exist, rather than leaking its existence via a different
	// error shape.
	workItemType, projectID, caseState, err := s.repo.GetWorkItemTypeAndProject(ctx, req.WorkItemID)
	if err != nil {
		return domain.CreatePendingVerificationResponse{}, err
	}
	if workItemType != "CASE" {
		return domain.CreatePendingVerificationResponse{}, &apierror.ValidationError{Msg: "workItemId must reference a Case -- Pending Verification is scoped to cases only"}
	}
	// "Add to Verification" only ever applies to a closed case -- this used
	// to be enforced only by the webapp (VerificationActionButtons' own
	// isRecordClosed), so any other caller could add an open case straight
	// through the API.
	if caseState == nil || *caseState != "CLOSED" {
		return domain.CreatePendingVerificationResponse{}, &apierror.ValidationError{Msg: "workItemId must reference a closed Case -- Pending Verification can only be added once the case is closed"}
	}

	// RLS enforces project membership, not whether the project has this
	// feature turned on at all -- without this, a read-only project member
	// could still add an entry on a project where the feature is disabled.
	if enabled, err := s.repo.GetProjectVerificationEnabled(ctx, projectID); err != nil {
		return domain.CreatePendingVerificationResponse{}, err
	} else if !enabled {
		return domain.CreatePendingVerificationResponse{}, &apierror.ForbiddenError{Msg: "Pending Verification is not enabled for this project"}
	}

	email, err := resolveCallerEmail(ctx)
	if err != nil {
		return domain.CreatePendingVerificationResponse{}, err
	}
	req.AddedBy = email

	pv, err := s.repo.Create(ctx, req)
	if err != nil {
		return domain.CreatePendingVerificationResponse{}, err
	}

	return domain.CreatePendingVerificationResponse{
		Message:             "Added to Pending Verification",
		PendingVerification: pv,
	}, nil
}

// SearchPendingVerifications implements PendingVerificationService.
func (s *pendingVerificationService) SearchPendingVerifications(ctx context.Context, req domain.SearchPendingVerificationsRequest) (domain.SearchPendingVerificationsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchPendingVerificationsResponse{}, err
	}
	if err := validateUUIDs("filters.projectId", []string{req.Filters.ProjectID}); err != nil {
		return domain.SearchPendingVerificationsResponse{}, err
	}
	if req.Filters.WorkItemID != nil {
		if err := validateUUIDs("filters.workItemId", []string{*req.Filters.WorkItemID}); err != nil {
			return domain.SearchPendingVerificationsResponse{}, err
		}
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchPendingVerificationsResponse{}, err
	}
	for _, t := range req.Filters.WorkItemTypes {
		if !validPendingVerificationRecordType[t] {
			return domain.SearchPendingVerificationsResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("filters.workItemTypes contains an unsupported record type: %q", t)}
		}
	}
	if req.Filters.AddedReason != nil && !validPendingVerificationAddedReason[*req.Filters.AddedReason] {
		return domain.SearchPendingVerificationsResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("filters.addedReason must be one of auto-closed/manual, got %q", *req.Filters.AddedReason)}
	}

	// See CreatePendingVerification's own comment: RLS alone does not check
	// whether the project has this feature turned on.
	if enabled, err := s.repo.GetProjectVerificationEnabled(ctx, req.Filters.ProjectID); err != nil {
		return domain.SearchPendingVerificationsResponse{}, err
	} else if !enabled {
		return domain.SearchPendingVerificationsResponse{}, &apierror.ForbiddenError{Msg: "Pending Verification is not enabled for this project"}
	}

	return s.repo.Search(ctx, req)
}

// VerifyPendingVerification implements PendingVerificationService.
func (s *pendingVerificationService) VerifyPendingVerification(ctx context.Context, req domain.VerifyPendingVerificationRequest) (domain.VerifyPendingVerificationResponse, error) {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.VerifyPendingVerificationResponse{}, err
	}

	// See CreatePendingVerification's own comment: RLS alone does not check
	// whether the project has this feature turned on. This request only
	// carries the entry's own id, so its project has to be resolved first.
	projectID, err := s.repo.GetProjectIDForEntry(ctx, req.ID)
	if err != nil {
		return domain.VerifyPendingVerificationResponse{}, err
	}
	if enabled, err := s.repo.GetProjectVerificationEnabled(ctx, projectID); err != nil {
		return domain.VerifyPendingVerificationResponse{}, err
	} else if !enabled {
		return domain.VerifyPendingVerificationResponse{}, &apierror.ForbiddenError{Msg: "Pending Verification is not enabled for this project"}
	}

	email, err := resolveCallerEmail(ctx)
	if err != nil {
		return domain.VerifyPendingVerificationResponse{}, err
	}
	req.VerifiedBy = email

	pv, err := s.repo.Verify(ctx, req)
	if err != nil {
		return domain.VerifyPendingVerificationResponse{}, err
	}

	return domain.VerifyPendingVerificationResponse{
		Message:             fmt.Sprintf("Marked as verified — %s has been removed from your pending verification list.", pv.RecordNumber),
		PendingVerification: pv,
	}, nil
}
