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
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type announcementRequestService struct {
	repo repository.AnnouncementRequestRepository
}

// NewAnnouncementRequestService constructs an AnnouncementRequestService
// backed by the given repository.
func NewAnnouncementRequestService(repo repository.AnnouncementRequestRepository) AnnouncementRequestService {
	return &announcementRequestService{repo: repo}
}

// CreateDraft implements AnnouncementRequestService.
func (s *announcementRequestService) CreateDraft(ctx context.Context, req domain.CreateAnnouncementRequestRequest) (domain.AnnouncementRequest, error) {
	if req.Kind != domain.AnnouncementRequestKindCustomer && req.Kind != domain.AnnouncementRequestKindEOL {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "kind must be \"customer\" or \"eol\""}
	}
	if strings.TrimSpace(req.CreatedBy) == "" {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "createdBy is required"}
	}
	return s.repo.Create(ctx, req)
}

// Get implements AnnouncementRequestService.
func (s *announcementRequestService) Get(ctx context.Context, id string) (domain.AnnouncementRequest, error) {
	if strings.TrimSpace(id) == "" {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "id is required"}
	}
	return s.repo.Get(ctx, id)
}

// Search implements AnnouncementRequestService.
func (s *announcementRequestService) Search(ctx context.Context, req domain.SearchAnnouncementRequestsRequest) (domain.SearchAnnouncementRequestsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchAnnouncementRequestsResponse{}, err
	}
	if req.State != nil && !isValidAnnouncementRequestState(*req.State) {
		return domain.SearchAnnouncementRequestsResponse{}, &apierror.ValidationError{Msg: "state must be one of: draft, pending_approval, approved, published"}
	}

	requests, total, err := s.repo.Search(ctx, req)
	if err != nil {
		return domain.SearchAnnouncementRequestsResponse{}, err
	}
	return domain.SearchAnnouncementRequestsResponse{
		Requests: requests,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(requests) < total,
	}, nil
}

// Update implements AnnouncementRequestService.
//
// What actually happens is entirely decided by the row's *current* state,
// not by anything the caller chooses:
//   - draft: a plain field update, no state change.
//   - pending_approval: the same field update, but also reverts to draft
//     as one atomic side effect (clearing the dry-run record and the
//     frozen audience snapshot) — the content is out for real review over
//     email, so a silent change under the reviewer isn't safe, and the old
//     dry run no longer describes whatever's about to be re-submitted.
//   - approved: subject/description/security-flag may still be updated in
//     place with no state change and no audience change — a human has
//     already said yes over email, so this is a deliberate trade-off,
//     accepted explicitly: a post-approval edit is not re-verified against
//     a fresh dry run before Publish. AudienceDefinition is rejected here
//     (the approved snapshot must never silently change).
//   - published: rejected — nothing about a published request is editable
//     through this entity; the real cases it fanned out into are the
//     record from this point on.
func (s *announcementRequestService) Update(ctx context.Context, id string, req domain.UpdateAnnouncementRequestRequest) (domain.AnnouncementRequest, error) {
	if strings.TrimSpace(req.ActorID) == "" {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "actorId is required"}
	}

	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.AnnouncementRequest{}, err
	}

	switch current.State {
	case domain.AnnouncementRequestStateDraft:
		return s.repo.Update(ctx, id, domain.AnnouncementRequestStateDraft, req)
	case domain.AnnouncementRequestStatePendingApproval:
		return s.repo.RevertToDraft(ctx, id, req)
	case domain.AnnouncementRequestStateApproved:
		if len(req.AudienceDefinition) > 0 {
			return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "audience cannot be changed on an approved request — the approved audience snapshot is frozen"}
		}
		return s.repo.Update(ctx, id, domain.AnnouncementRequestStateApproved, req)
	default: // published
		return domain.AnnouncementRequest{}, &apierror.ConflictError{Msg: "a published announcement request cannot be edited"}
	}
}

// RecordDryRun implements AnnouncementRequestService. Allowed only from
// draft — recording a dry run against a request that's already left draft
// makes no sense (pending_approval/approved already snapshot a specific
// version, and a published request is done).
func (s *announcementRequestService) RecordDryRun(ctx context.Context, id string, req domain.RecordAnnouncementDryRunRequest) (domain.AnnouncementRequest, error) {
	if strings.TrimSpace(req.CaseID) == "" {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "caseId is required"}
	}
	if strings.TrimSpace(req.ActorID) == "" {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "actorId is required"}
	}

	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.AnnouncementRequest{}, err
	}
	if current.State != domain.AnnouncementRequestStateDraft {
		return domain.AnnouncementRequest{}, &apierror.ConflictError{Msg: "a dry run can only be recorded while the request is in draft, not " + string(current.State)}
	}
	return s.repo.RecordDryRun(ctx, id, req)
}

// Submit implements AnnouncementRequestService. Rejects unless the current
// state is draft and a dry run has already been recorded — the dry-run case
// is the only preview the approver ever sees (there is no other rendering
// surface in this slice), so submitting without one would send an approval
// request for content nobody has actually looked at rendered.
func (s *announcementRequestService) Submit(ctx context.Context, id string, req domain.SubmitAnnouncementRequestRequest) (domain.AnnouncementRequest, error) {
	if strings.TrimSpace(req.ActorID) == "" {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "actorId is required"}
	}
	if len(req.ResolvedProjectIDs) == 0 {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "resolvedProjectIds must not be empty"}
	}

	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.AnnouncementRequest{}, err
	}
	if current.State != domain.AnnouncementRequestStateDraft {
		return domain.AnnouncementRequest{}, &apierror.ConflictError{Msg: "only a draft can be submitted for approval, not " + string(current.State)}
	}
	if current.DryRunCaseID == nil {
		return domain.AnnouncementRequest{}, &apierror.ConflictError{Msg: "a dry run must be recorded before submitting for approval"}
	}
	return s.repo.Submit(ctx, id, req)
}

// Approve implements AnnouncementRequestService. Rejects unless the current
// state is pending_approval. There is deliberately no approver-role check
// here — the real approval decision already happened over email, outside
// this service; this call only records that whoever is working the
// request says it's been approved.
func (s *announcementRequestService) Approve(ctx context.Context, id, actorID string) (domain.AnnouncementRequest, error) {
	if strings.TrimSpace(actorID) == "" {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "actorId is required"}
	}

	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.AnnouncementRequest{}, err
	}
	if current.State != domain.AnnouncementRequestStatePendingApproval {
		return domain.AnnouncementRequest{}, &apierror.ConflictError{Msg: "only a request pending approval can be approved, not " + string(current.State)}
	}
	return s.repo.Approve(ctx, id, actorID)
}

// MarkPublished implements AnnouncementRequestService. Rejects unless the
// current state is approved. This never creates the real per-project cases
// itself — the caller (the webapp's own publish flow) does that fan-out
// exactly as it already does today; this call only records that it
// happened, by whom, and when.
func (s *announcementRequestService) MarkPublished(ctx context.Context, id, actorID string) (domain.AnnouncementRequest, error) {
	if strings.TrimSpace(actorID) == "" {
		return domain.AnnouncementRequest{}, &apierror.ValidationError{Msg: "actorId is required"}
	}

	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.AnnouncementRequest{}, err
	}
	if current.State != domain.AnnouncementRequestStateApproved {
		return domain.AnnouncementRequest{}, &apierror.ConflictError{Msg: "only an approved request can be published, not " + string(current.State)}
	}
	return s.repo.MarkPublished(ctx, id, actorID)
}

func isValidAnnouncementRequestState(s domain.AnnouncementRequestState) bool {
	switch s {
	case domain.AnnouncementRequestStateDraft,
		domain.AnnouncementRequestStatePendingApproval,
		domain.AnnouncementRequestStateApproved,
		domain.AnnouncementRequestStatePublished:
		return true
	default:
		return false
	}
}
