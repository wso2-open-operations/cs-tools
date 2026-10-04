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
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
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
	// snWriteback/snMirror back CreateTimeCard's, UpdateTimeCard's, and
	// DeleteTimeCard's best-effort, asynchronous ServiceNow mirror writes
	// under DATA_SOURCE=postgres-servicenow-dual-write -- both nil in every
	// other mode. Set only via NewTimeCardServiceWithSNWriteback.
	//
	// Update/DeleteTimeCard's mirrors need an id mapping CreateTimeCard
	// didn't used to record: unlike case/change_request/incident (whose
	// CREATE is ServiceNow-first under this data source, so their Postgres
	// id IS the real ServiceNow sys_id round-tripped through sysidToUUID),
	// CreateTimeCard is Postgres-first -- time_card.id is a plain
	// Postgres-generated UUID with no ServiceNow counterpart. Migration
	// 000088 adds time_card.sn_sys_id for exactly this: CreateTimeCard's own
	// mirror success path now persists it (best-effort, asynchronously --
	// see that method below), and Update/DeleteTimeCard's mirrors look it up
	// before dispatching, skipping silently (not erroring) when it is still
	// NULL.
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
	email, err := callerEmail(ctx)
	if err != nil {
		return "", err
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
	// this point. On success, the ServiceNow-side id this mirror creates is
	// persisted back onto the Postgres row (migration 0135's
	// time_card.sn_sys_id) -- itself a second best-effort, asynchronous
	// write: if it fails, the row simply has no id yet, the same "not yet
	// mirrorable" state Update/DeleteTimeCard's mirrors already tolerate.
	if s.snWriteback != nil {
		mirrorReq := req
		pgID := view.ID
		s.snWriteback.Dispatch(ctx, "time_card", pgID, "create",
			map[string]any{"caseId": req.CaseID, "projectId": req.ProjectID, "date": req.Date},
			func(writeCtx context.Context) error {
				snResp, err := s.snMirror.CreateTimeCard(writeCtx, mirrorReq)
				if err != nil {
					return err
				}
				if snResp.TimeCard == nil {
					slog.WarnContext(writeCtx, "sn writeback: time card create mirror succeeded but returned no time card to read its sys_id from",
						"timeCardId", pgID)
					return nil
				}
				snSysID := uuidToSysid(snResp.TimeCard.ID)
				if setErr := s.repo.SetTimeCardSNSysID(writeCtx, pgID, snSysID); setErr != nil {
					slog.WarnContext(writeCtx, "sn writeback: time card created in ServiceNow but persisting its sys_id back onto Postgres failed -- update/delete mirrors for this row will keep skipping until this is fixed",
						"timeCardId", pgID, "error", setErr)
				}
				return nil
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
		s.dispatchTimeCardUpdateMirror(ctx, req)
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
	s.dispatchTimeCardUpdateMirror(ctx, req)
	return domain.TimeCardMutationResponse{Message: "Time card updated successfully", TimeCard: &view}, nil
}

// dispatchTimeCardUpdateMirror dispatches UpdateTimeCard's best-effort
// ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write only
// (a no-op when snWriteback is nil). Called from both of UpdateTimeCard's
// return paths (state transition and field edit) with the same, unmodified
// req -- snMirror.UpdateTimeCard already rejects combining a state
// transition with field edits (see its own doc comment), the same mutual
// exclusion the Postgres path above already enforces, so req needs no
// branch-specific handling here. The SN sys_id lookup happens inside the
// dispatched closure, after any earlier-queued job for this same time card
// (e.g. the CREATE mirror that persists it) has already applied -- the
// dispatcher serializes jobs per entity key precisely so this ordering
// holds.
func (s *timeCardService) dispatchTimeCardUpdateMirror(ctx context.Context, req domain.UpdateTimeCardRequest) {
	if s.snWriteback == nil {
		return
	}
	mirrorReq := req
	s.snWriteback.Dispatch(ctx, "time_card", req.ID, "update",
		map[string]any{"id": req.ID, "state": req.State},
		func(writeCtx context.Context) error {
			snSysID, lookupErr := s.repo.GetTimeCardSNSysID(writeCtx, req.ID)
			if lookupErr != nil {
				return lookupErr
			}
			if snSysID == nil || *snSysID == "" {
				slog.InfoContext(writeCtx, "sn writeback: time card update mirror skipped, no ServiceNow mapping stored yet", "timeCardId", req.ID)
				return nil
			}
			mirrorReq.ID = sysidToUUID(*snSysID)
			_, err := s.snMirror.UpdateTimeCard(writeCtx, mirrorReq)
			return err
		},
	)
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

	// The SN sys_id must be read BEFORE the Postgres delete below: DeleteTimeCard
	// removes the row entirely, taking time_card.sn_sys_id with it, so this is
	// the last point at which it can be looked up. Postgres deletion is
	// authoritative and must proceed regardless of whether the row happens to
	// have a ServiceNow mapping yet: a genuinely successful lookup that finds
	// no mapping is skipped silently below, but a real lookup error is
	// recorded as a failed writeback (sn_writeback_failures) instead of being
	// discarded.
	var snSysID *string
	var snLookupErr error
	if s.snWriteback != nil {
		snSysID, snLookupErr = s.repo.GetTimeCardSNSysID(ctx, req.ID)
	}

	if err := s.repo.DeleteTimeCard(ctx, req.ID, userID); err != nil {
		return domain.DeleteTimeCardResponse{}, err
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only. Postgres has already committed (the row is gone) by this point;
	// skip silently (not an error) if no ServiceNow mapping was ever stored.
	if s.snWriteback != nil && snLookupErr != nil {
		lookupErr := snLookupErr
		s.snWriteback.Dispatch(ctx, "time_card", req.ID, "delete",
			map[string]any{"id": req.ID},
			func(context.Context) error {
				return lookupErr
			},
		)
	} else if s.snWriteback != nil && snSysID != nil && *snSysID != "" {
		mirrorReq := domain.DeleteTimeCardRequest{ID: sysidToUUID(*snSysID)}
		s.snWriteback.Dispatch(ctx, "time_card", req.ID, "delete",
			map[string]any{"id": req.ID},
			func(writeCtx context.Context) error {
				_, err := s.snMirror.DeleteTimeCard(writeCtx, mirrorReq)
				return err
			},
		)
	}

	return domain.DeleteTimeCardResponse{Message: "Time card deleted successfully"}, nil
}
