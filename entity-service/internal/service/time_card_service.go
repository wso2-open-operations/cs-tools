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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

var validTimeCardState = map[domain.TimeCardState]bool{
	domain.TimeCardStatePending:   true,
	domain.TimeCardStateSubmitted: true,
	domain.TimeCardStateApproved:  true,
	domain.TimeCardStateRejected:  true,
	domain.TimeCardStateProcessed: true,
	domain.TimeCardStateRecalled:  true,
}

type timeCardService struct {
	repo     repository.TimeCardRepository
	userRepo repository.UserRepository
	// snWriteback/snMirror back CreateTimeCard's best-effort, asynchronous
	// ServiceNow mirror write under DATA_SOURCE=postgres-servicenow-dual-write
	// -- both nil in every other mode. Set only via
	// NewTimeCardServiceWithSNWriteback.
	//
	// UpdateTimeCard/DeleteTimeCard are deliberately NOT mirrored here, same
	// reasoning as callRequestService's own doc comment on why
	// UpdateCallRequest isn't mirrored: CreateTimeCard is Postgres-first --
	// time_card.id is a plain Postgres-generated UUID with no ServiceNow
	// counterpart stored anywhere (no column on time_card holds one -- see
	// migration 000039), unlike case/change_request/incident whose CREATE is
	// ServiceNow-first under this data source. uuidToSysid(that id) would not
	// resolve to the real ServiceNow record, so a mirrored update/delete
	// would either permanently 404 or risk colliding with an unrelated
	// ServiceNow record.
	snWriteback *SNWritebackDispatcher
	snMirror    TimeCardService
}

// NewTimeCardService constructs a TimeCardService backed by Postgres.
func NewTimeCardService(repo repository.TimeCardRepository, userRepo repository.UserRepository) TimeCardService {
	return &timeCardService{repo: repo, userRepo: userRepo}
}

// NewTimeCardServiceWithSNWriteback is NewTimeCardService plus the wiring
// DATA_SOURCE=postgres-servicenow-dual-write needs: CreateTimeCard dispatches
// a best-effort, asynchronous ServiceNow mirror write onto mirror after the
// Postgres write commits -- see CreateTimeCard's own doc comment, and
// timeCardService's own doc comment on why Update/DeleteTimeCard are not
// mirrored. A separate constructor rather than extending NewTimeCardService's
// own signature, same reasoning as NewCaseServiceWithSNWriteback's own doc
// comment.
func NewTimeCardServiceWithSNWriteback(repo repository.TimeCardRepository, userRepo repository.UserRepository, dispatcher *SNWritebackDispatcher, mirror TimeCardService) TimeCardService {
	return &timeCardService{repo: repo, userRepo: userRepo, snWriteback: dispatcher, snMirror: mirror}
}

// currentUserID resolves the caller's user id from their x-user-id-token --
// this data source has no other notion of who is calling, the same
// mechanism caseService.CreateCaseComment uses to attribute a case comment.
func (s *timeCardService) currentUserID(ctx context.Context) (string, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return "", &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	return user.ID, nil
}

func normalizeTimeCardSort(sort *domain.TimeCardSort) error {
	if sort.Field == "" {
		sort.Field = domain.TimeCardSortFieldUpdatedOn
	}
	if sort.Field != domain.TimeCardSortFieldUpdatedOn && sort.Field != domain.TimeCardSortFieldWorkDate {
		return &apierror.ValidationError{Msg: "sortBy.field contains invalid value: " + string(sort.Field)}
	}
	if sort.Order == "" {
		sort.Order = domain.TimeCardSortOrderDesc
	}
	if sort.Order != domain.TimeCardSortOrderAsc && sort.Order != domain.TimeCardSortOrderDesc {
		return &apierror.ValidationError{Msg: "sortBy.order contains invalid value: " + string(sort.Order)}
	}
	return nil
}

func validateTimeCardDate(field, value string) error {
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return &apierror.ValidationError{Msg: field + " must be in YYYY-MM-DD format"}
	}
	return nil
}

func validateTimeCardFilters(f *domain.SearchTimeCardsFilters) error {
	if f == nil {
		return nil
	}
	for _, st := range f.States {
		if !validTimeCardState[st] {
			return &apierror.ValidationError{Msg: "states contains invalid value: " + string(st)}
		}
	}

	ids := append([]string{}, f.ProjectIDs...)
	ids = append(ids, f.UserIDs...)
	if f.CaseID != nil {
		ids = append(ids, *f.CaseID)
	}
	if f.UserID != nil {
		ids = append(ids, *f.UserID)
	}
	if f.ApproverID != nil {
		ids = append(ids, *f.ApproverID)
	}
	if f.ApprovedByID != nil {
		ids = append(ids, *f.ApprovedByID)
	}
	if len(ids) > 0 {
		if err := validateUUIDs("filters", ids); err != nil {
			return err
		}
	}

	if f.StartDate != nil {
		if err := validateTimeCardDate("startDate", *f.StartDate); err != nil {
			return err
		}
	}
	if f.EndDate != nil {
		if err := validateTimeCardDate("endDate", *f.EndDate); err != nil {
			return err
		}
	}
	return nil
}

func validateTimeCardMinutes(values ...int) error {
	for _, v := range values {
		if v < 0 {
			return &apierror.ValidationError{Msg: "time values must not be negative"}
		}
	}
	return nil
}

// SearchTimeCards implements TimeCardService.
func (s *timeCardService) SearchTimeCards(ctx context.Context, req domain.SearchTimeCardsRequest) (domain.SearchTimeCardsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchTimeCardsResponse{}, err
	}
	if err := normalizeTimeCardSort(&req.SortBy); err != nil {
		return domain.SearchTimeCardsResponse{}, err
	}
	if err := validateTimeCardFilters(req.Filters); err != nil {
		return domain.SearchTimeCardsResponse{}, err
	}

	// Requires a valid, authenticated caller (same minimum bar as every
	// write on this service) but does not yet scope results to what that
	// caller specifically owns/approves/manages -- entity-service has no
	// authorization model to build that against today. callerEmail is
	// threaded to the repository layer for that future decision, the same
	// deliberate, deferred-not-missing posture as
	// AccountContactRepository/ProjectContactRepository's own callerEmail
	// parameter.
	callerEmail, err := resolveCallerEmail(ctx)
	if err != nil {
		return domain.SearchTimeCardsResponse{}, err
	}

	views, total, err := s.repo.SearchTimeCards(ctx, req, callerEmail)
	if err != nil {
		return domain.SearchTimeCardsResponse{}, err
	}

	return domain.SearchTimeCardsResponse{
		TimeCards: views,
		Total:     total,
		Limit:     req.Pagination.Limit,
		Offset:    req.Pagination.Offset,
	}, nil
}

// SearchCaseTimeCards implements TimeCardService.
func (s *timeCardService) SearchCaseTimeCards(ctx context.Context, req domain.SearchTimeCardsRequest) (domain.SearchCaseTimeCardsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCaseTimeCardsResponse{}, err
	}
	if err := validateTimeCardFilters(req.Filters); err != nil {
		return domain.SearchCaseTimeCardsResponse{}, err
	}

	// See SearchTimeCards' identical comment: requires authentication, does
	// not yet scope results to it.
	callerEmail, err := resolveCallerEmail(ctx)
	if err != nil {
		return domain.SearchCaseTimeCardsResponse{}, err
	}

	summaries, total, err := s.repo.SearchCaseTimeCards(ctx, req, callerEmail)
	if err != nil {
		return domain.SearchCaseTimeCardsResponse{}, err
	}

	return domain.SearchCaseTimeCardsResponse{
		Cases:  summaries,
		Total:  total,
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
	}, nil
}

// CreateTimeCard implements TimeCardService.
func (s *timeCardService) CreateTimeCard(ctx context.Context, req domain.CreateTimeCardRequest) (domain.TimeCardMutationResponse, error) {
	ids := []string{req.CaseID}
	if req.ProjectID != "" {
		ids = append(ids, req.ProjectID)
	}
	ids = append(ids, req.ApproverIDs...)
	if err := validateUUIDs("id", ids); err != nil {
		return domain.TimeCardMutationResponse{}, err
	}
	if err := validateTimeCardDate("date", req.Date); err != nil {
		return domain.TimeCardMutationResponse{}, err
	}
	if err := validateTimeCardMinutes(req.TimeAnalyzing, req.TimeSettingUp, req.TimeReproducingDebugging, req.TimeProvidingSolution, req.TimePatching); err != nil {
		return domain.TimeCardMutationResponse{}, err
	}

	userID, err := s.currentUserID(ctx)
	if err != nil {
		return domain.TimeCardMutationResponse{}, err
	}

	view, err := s.repo.CreateTimeCard(ctx, req, userID)
	if err != nil {
		return domain.TimeCardMutationResponse{}, err
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only (snWriteback/snMirror are both nil otherwise -- see
	// timeCardService's own doc comment). Postgres has already committed by
	// this point; the ServiceNow-side id this mirror creates is deliberately
	// discarded (never written back onto the Postgres row) -- see
	// timeCardService's own doc comment for why Update/DeleteTimeCard cannot
	// use it later anyway.
	if s.snWriteback != nil {
		mirrorReq := req
		s.snWriteback.Dispatch(ctx, "time_card", view.ID, "create",
			map[string]any{"caseId": req.CaseID, "projectId": req.ProjectID, "date": req.Date},
			func(writeCtx context.Context) error {
				_, err := s.snMirror.CreateTimeCard(writeCtx, mirrorReq)
				return err
			},
		)
	}

	return domain.TimeCardMutationResponse{Message: "Time card created successfully", TimeCard: &view}, nil
}

// UpdateTimeCard implements TimeCardService.
func (s *timeCardService) UpdateTimeCard(ctx context.Context, req domain.UpdateTimeCardRequest) (domain.TimeCardMutationResponse, error) {
	ids := append([]string{req.ID}, req.ApproverIDs...)
	if err := validateUUIDs("id", ids); err != nil {
		return domain.TimeCardMutationResponse{}, err
	}

	actorID, err := s.currentUserID(ctx)
	if err != nil {
		return domain.TimeCardMutationResponse{}, err
	}

	if req.State != nil {
		if *req.State != domain.TimeCardStateApproved && *req.State != domain.TimeCardStateRejected {
			return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "state must be \"approved\" or \"rejected\""}
		}
		if *req.State == domain.TimeCardStateRejected && (req.LeadComment == nil || *req.LeadComment == "") {
			return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "leadComment is required when rejecting"}
		}
		view, err := s.repo.TransitionTimeCardState(ctx, req.ID, *req.State, req.LeadComment, actorID)
		if err != nil {
			return domain.TimeCardMutationResponse{}, err
		}
		return domain.TimeCardMutationResponse{Message: "Time card updated successfully", TimeCard: &view}, nil
	}

	if req.Date != nil {
		if err := validateTimeCardDate("date", *req.Date); err != nil {
			return domain.TimeCardMutationResponse{}, err
		}
	}
	minutes := []int{}
	for _, v := range []*int{req.TimeAnalyzing, req.TimeSettingUp, req.TimeReproducingDebugging, req.TimeProvidingSolution, req.TimePatching} {
		if v != nil {
			minutes = append(minutes, *v)
		}
	}
	if err := validateTimeCardMinutes(minutes...); err != nil {
		return domain.TimeCardMutationResponse{}, err
	}

	view, err := s.repo.UpdateTimeCardFields(ctx, req, actorID)
	if err != nil {
		return domain.TimeCardMutationResponse{}, err
	}
	return domain.TimeCardMutationResponse{Message: "Time card updated successfully", TimeCard: &view}, nil
}

// DeleteTimeCard implements TimeCardService.
func (s *timeCardService) DeleteTimeCard(ctx context.Context, req domain.DeleteTimeCardRequest) (domain.DeleteTimeCardResponse, error) {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.DeleteTimeCardResponse{}, err
	}

	userID, err := s.currentUserID(ctx)
	if err != nil {
		return domain.DeleteTimeCardResponse{}, err
	}

	if err := s.repo.DeleteTimeCard(ctx, req.ID, userID); err != nil {
		return domain.DeleteTimeCardResponse{}, err
	}

	return domain.DeleteTimeCardResponse{Message: "Time card deleted successfully"}, nil
}
