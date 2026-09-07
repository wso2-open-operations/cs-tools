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
	"regexp"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// timeCardService is the native (Postgres) TimeCardService. Unlike the
// ServiceNow implementation, ServiceNow enforces no authorization for us here,
// so the state machine and the "who may do what" rules are enforced in Go:
//   - a card is created in the submitted state by its submitter;
//   - only the submitter may edit or delete their own card, and only while it
//     is still submitted;
//   - only an eligible approver (never the submitter) may approve or reject a
//     submitted card.
//
// These rules mirror the documented ServiceNow behavior; where the exact rule
// wasn't recoverable from the SN definition it's marked DECISION and is safe to
// tune.
type timeCardService struct {
	repo     repository.TimeCardRepository
	userRepo repository.UserRepository
}

// NewTimeCardService constructs the native TimeCardService.
func NewTimeCardService(repo repository.TimeCardRepository, userRepo repository.UserRepository) TimeCardService {
	return &timeCardService{repo: repo, userRepo: userRepo}
}

var timeCardDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

var validTimeCardSortField = map[domain.TimeCardSortField]bool{
	domain.TimeCardSortFieldUpdatedOn: true,
	domain.TimeCardSortFieldWorkDate:  true,
}

var validTimeCardState = map[domain.TimeCardState]bool{
	domain.TimeCardStatePending:   true,
	domain.TimeCardStateSubmitted: true,
	domain.TimeCardStateApproved:  true,
	domain.TimeCardStateRejected:  true,
	domain.TimeCardStateProcessed: true,
	domain.TimeCardStateRecalled:  true,
}

// resolveTimeCardActor resolves the authenticated caller to a domain.User.
func (s *timeCardService) resolveActor(ctx context.Context) (domain.User, error) {
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

// SearchTimeCards implements TimeCardService.
func (s *timeCardService) SearchTimeCards(ctx context.Context, req domain.SearchTimeCardsRequest) (domain.SearchTimeCardsResponse, error) {
	if req.SortBy.Field != "" && !validTimeCardSortField[req.SortBy.Field] {
		return domain.SearchTimeCardsResponse{}, &apierror.ValidationError{Msg: "sortBy.field contains invalid value: " + string(req.SortBy.Field)}
	}
	if req.SortBy.Order != "" && req.SortBy.Order != domain.TimeCardSortOrderAsc && req.SortBy.Order != domain.TimeCardSortOrderDesc {
		return domain.SearchTimeCardsResponse{}, &apierror.ValidationError{Msg: "sortBy.order contains invalid value: " + string(req.SortBy.Order)}
	}
	if req.Filters != nil {
		for _, st := range req.Filters.States {
			if !validTimeCardState[st] {
				return domain.SearchTimeCardsResponse{}, &apierror.ValidationError{Msg: "states contains invalid value: " + string(st)}
			}
		}
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchTimeCardsResponse{}, err
	}
	cards, total, err := s.repo.SearchTimeCards(ctx, req)
	if err != nil {
		return domain.SearchTimeCardsResponse{}, err
	}
	return domain.SearchTimeCardsResponse{
		TimeCards: cards,
		Total:     total,
		Limit:     req.Pagination.Limit,
		Offset:    req.Pagination.Offset,
	}, nil
}

// SearchCaseTimeCards implements TimeCardService. The by-case rollup is a
// ServiceNow-only capability; the native path does not provide it.
func (s *timeCardService) SearchCaseTimeCards(_ context.Context, _ domain.SearchTimeCardsRequest) (domain.SearchCaseTimeCardsResponse, error) {
	return domain.SearchCaseTimeCardsResponse{}, &apierror.ServiceUnavailableError{Msg: "case time-card rollup is only supported for the ServiceNow data source"}
}

func nonNegativeTimeBucket(field string, v int) error {
	if v < 0 {
		return &apierror.ValidationError{Msg: field + " must not be negative"}
	}
	return nil
}

// CreateTimeCard implements TimeCardService.
func (s *timeCardService) CreateTimeCard(ctx context.Context, req domain.CreateTimeCardRequest) (domain.TimeCardMutationResponse, error) {
	if req.CaseID == "" {
		return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "caseId is required"}
	}
	if req.ProjectID == "" {
		return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "projectId is required"}
	}
	if req.Date == "" || !timeCardDateRe.MatchString(req.Date) {
		return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "date is required in YYYY-MM-DD format"}
	}
	if len(req.ApproverIDs) == 0 {
		return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "approverIds must not be empty"}
	}
	for field, v := range map[string]int{
		"timeAnalyzing": req.TimeAnalyzing, "timeSettingUp": req.TimeSettingUp,
		"timeReproducingDebugging": req.TimeReproducingDebugging,
		"timeProvidingSolution":    req.TimeProvidingSolution, "timePatching": req.TimePatching,
	} {
		if err := nonNegativeTimeBucket(field, v); err != nil {
			return domain.TimeCardMutationResponse{}, err
		}
	}
	actor, err := s.resolveActor(ctx)
	if err != nil {
		return domain.TimeCardMutationResponse{}, err
	}
	// DECISION (verify vs SN): a submitter may not list themselves as an approver.
	for _, aid := range req.ApproverIDs {
		if aid == actor.ID {
			return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "the submitter cannot be their own approver"}
		}
	}
	card, err := s.repo.CreateTimeCard(ctx, req, actor.ID)
	if err != nil {
		return domain.TimeCardMutationResponse{}, err
	}
	return domain.TimeCardMutationResponse{Message: "Time card created successfully", TimeCard: &card}, nil
}

// UpdateTimeCard implements TimeCardService. It is either a field edit (by the
// submitter, while submitted) or a state transition to approved/rejected (by an
// eligible approver) -- never both in one request.
func (s *timeCardService) UpdateTimeCard(ctx context.Context, req domain.UpdateTimeCardRequest) (domain.TimeCardMutationResponse, error) {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.TimeCardMutationResponse{}, err
	}
	isEdit := req.Date != nil || req.IsBillable != nil || req.IssueComplexity != nil ||
		req.WorkLogComment != nil || req.ApproverIDs != nil ||
		req.TimeAnalyzing != nil || req.TimeSettingUp != nil || req.TimeReproducingDebugging != nil ||
		req.TimeProvidingSolution != nil || req.TimePatching != nil
	if req.State == nil && !isEdit && req.LeadComment == nil {
		return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "no fields to update"}
	}
	if req.State != nil && isEdit {
		return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "a state transition cannot be combined with field edits"}
	}

	actor, err := s.resolveActor(ctx)
	if err != nil {
		return domain.TimeCardMutationResponse{}, err
	}
	current, err := s.repo.GetTimeCardByID(ctx, req.ID)
	if err != nil {
		return domain.TimeCardMutationResponse{}, err
	}

	if req.State != nil {
		return s.transition(ctx, req, actor, current)
	}

	// Field edit: submitter only, submitted only.
	if current.User == nil || current.User.ID != actor.ID {
		return domain.TimeCardMutationResponse{}, &apierror.ForbiddenError{Msg: "only the submitter can edit their time card"}
	}
	if current.State == nil || *current.State != string(domain.TimeCardStateSubmitted) {
		return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "only submitted time cards can be edited"}
	}
	for field, v := range map[string]*int{
		"timeAnalyzing": req.TimeAnalyzing, "timeSettingUp": req.TimeSettingUp,
		"timeReproducingDebugging": req.TimeReproducingDebugging,
		"timeProvidingSolution":    req.TimeProvidingSolution, "timePatching": req.TimePatching,
	} {
		if v != nil {
			if err := nonNegativeTimeBucket(field, *v); err != nil {
				return domain.TimeCardMutationResponse{}, err
			}
		}
	}
	card, err := s.repo.UpdateTimeCardFields(ctx, req)
	if err != nil {
		return domain.TimeCardMutationResponse{}, err
	}
	return domain.TimeCardMutationResponse{Message: "Time card updated successfully", TimeCard: &card}, nil
}

// transition handles the approve/reject path of UpdateTimeCard.
func (s *timeCardService) transition(ctx context.Context, req domain.UpdateTimeCardRequest, actor domain.User, current domain.TimeCardView) (domain.TimeCardMutationResponse, error) {
	if *req.State != domain.TimeCardStateApproved && *req.State != domain.TimeCardStateRejected {
		return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "state must be approved or rejected"}
	}
	if current.State == nil || *current.State != string(domain.TimeCardStateSubmitted) {
		return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "only submitted time cards can be approved or rejected"}
	}
	// approver must be an eligible approver of this card, and never its submitter
	if current.User != nil && current.User.ID == actor.ID {
		return domain.TimeCardMutationResponse{}, &apierror.ForbiddenError{Msg: "a submitter cannot approve or reject their own time card"}
	}
	eligible := false
	for _, a := range current.Approvers {
		if a.ID == actor.ID {
			eligible = true
			break
		}
	}
	if !eligible {
		return domain.TimeCardMutationResponse{}, &apierror.ForbiddenError{Msg: "only an eligible approver can approve or reject this time card"}
	}
	// DECISION (verify vs SN): on reject, leadComment carries the rejection reason.
	card, err := s.repo.TransitionTimeCard(ctx, req.ID, *req.State, actor.ID, req.LeadComment)
	if err != nil {
		return domain.TimeCardMutationResponse{}, err
	}
	return domain.TimeCardMutationResponse{Message: "Time card " + string(*req.State), TimeCard: &card}, nil
}

// DeleteTimeCard implements TimeCardService. Submitter only, while submitted.
func (s *timeCardService) DeleteTimeCard(ctx context.Context, req domain.DeleteTimeCardRequest) (domain.DeleteTimeCardResponse, error) {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.DeleteTimeCardResponse{}, err
	}
	actor, err := s.resolveActor(ctx)
	if err != nil {
		return domain.DeleteTimeCardResponse{}, err
	}
	current, err := s.repo.GetTimeCardByID(ctx, req.ID)
	if err != nil {
		return domain.DeleteTimeCardResponse{}, err
	}
	if current.User == nil || current.User.ID != actor.ID {
		return domain.DeleteTimeCardResponse{}, &apierror.ForbiddenError{Msg: "only the submitter can delete their time card"}
	}
	if current.State == nil || *current.State != string(domain.TimeCardStateSubmitted) {
		return domain.DeleteTimeCardResponse{}, &apierror.ValidationError{Msg: "only submitted time cards can be deleted"}
	}
	if err := s.repo.DeleteTimeCard(ctx, req.ID); err != nil {
		return domain.DeleteTimeCardResponse{}, err
	}
	return domain.DeleteTimeCardResponse{Message: "Time card deleted successfully"}, nil
}
