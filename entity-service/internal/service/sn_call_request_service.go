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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snCallRequestCreatePayload mirrors POST /call-requests in the SN integration service.
type snCallRequestCreatePayload struct {
	CaseID          string   `json:"caseId"`
	Reason          string   `json:"reason"`
	UTCTimes        []string `json:"utcTimes"`
	DurationMinutes int      `json:"durationInMinutes"`
}

// snCallRequestState is the raw choice-list state object from the SN integration
// service. Its id arrives as either an integer choice-list key or a string;
// toDomainCallRequestState normalizes it to the domain string enum.
type snCallRequestState struct {
	ID    json.RawMessage `json:"id"`
	Label string          `json:"label"`
}

// snCallRequestCreateResponse mirrors the SN integration service POST /call-requests response.
type snCallRequestCreateResponse struct {
	Message     string `json:"message"`
	CallRequest struct {
		ID        string             `json:"id"`
		CreatedOn string             `json:"createdOn"`
		CreatedBy string             `json:"createdBy"`
		State     snCallRequestState `json:"state"`
	} `json:"callRequest"`
}

// snCallRequestSearchPayload mirrors POST /call-requests/search in the SN integration service.
type snCallRequestSearchPayload struct {
	CaseID     string                      `json:"caseId"`
	Filters    *snCallRequestSearchFilters `json:"filters,omitempty"`
	Pagination snProjectPagination         `json:"pagination"`
}

type snCallRequestSearchFilters struct {
	StateKeys []int `json:"stateKeys,omitempty"`
}

// snCallRequestsSearchAllPayload mirrors POST /call-requests/search-all in the SN
// integration service (standalone, not case-scoped).
type snCallRequestsSearchAllPayload struct {
	Filters    *snCallRequestsSearchAllFilters `json:"filters,omitempty"`
	SortBy     *snCallRequestSort              `json:"sortBy,omitempty"`
	Pagination snProjectPagination             `json:"pagination"`
}

type snCallRequestsSearchAllFilters struct {
	AssignedUserIDs   []string `json:"assignedUserIds,omitempty"`
	StateKeys         []int    `json:"stateKeys,omitempty"`
	CaseStates        []int    `json:"caseStates,omitempty"`
	ExcludeCaseStates []int    `json:"excludeCaseStates,omitempty"`
	AssignmentTeamIDs []string `json:"assignmentTeamIds,omitempty"`
}

type snCallRequestSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

// snCallRequestsResponse mirrors the SN integration service POST /call-requests/search response.
type snCallRequestsResponse struct {
	CallRequests []snCallRequest `json:"callRequests"`
	TotalRecords int             `json:"totalRecords"`
	Offset       int             `json:"offset"`
	Limit        int             `json:"limit"`
}

type snCallRequest struct {
	ID                 string               `json:"id"`
	Number             string               `json:"number"`
	Case               snCallRequestCaseRef `json:"case"`
	Reason             *string              `json:"reason"`
	PreferredTimes     []string             `json:"preferredTimes"`
	DurationMin        int                  `json:"durationMin"`
	ScheduleTime       *string              `json:"scheduleTime"`
	MeetingLink        *string              `json:"meetingLink"`
	CreatedOn          string               `json:"createdOn"`
	UpdatedOn          string               `json:"updatedOn"`
	State              snCallRequestState   `json:"state"`
	CancellationReason *string              `json:"cancellationReason,omitempty"`
	Assignee           *string              `json:"assignee,omitempty"`
	Notes              *string              `json:"notes,omitempty"`
	Plan               *string              `json:"plan,omitempty"`
	Attendees          *string              `json:"attendees,omitempty"`
	ActionItems        *string              `json:"actionItems,omitempty"`
	ActualDurationMin  *int                 `json:"actualDurationMin,omitempty"`
}

type snCallRequestCaseRef struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Number *string `json:"number,omitempty"`
}

// snCallRequestUpdatePayload mirrors PATCH /call-requests/{id} in the SN integration service.
type snCallRequestUpdatePayload struct {
	StateKey           int      `json:"stateKey"`
	CancellationReason *string  `json:"cancellationReason,omitempty"`
	UTCTimes           []string `json:"utcTimes,omitempty"`
	DurationMinutes    *int     `json:"durationInMinutes,omitempty"`
	MeetingDate        *string  `json:"meetingDate,omitempty"`
	Assignee           *string  `json:"assignee,omitempty"`
	Notes              *string  `json:"notes,omitempty"`
	Plan               *string  `json:"plan,omitempty"`
	Attendees          *string  `json:"attendees,omitempty"`
	ActionItems        *string  `json:"actionItems,omitempty"`
	ActualDurationMin  *int     `json:"actualDurationMin,omitempty"`
}

// callRequestStateToKey maps domain CallRequestStateType strings to the ServiceNow integer choice-list key.
var callRequestStateToKey = map[domain.CallRequestStateType]int{
	domain.CallRequestStatePendingOnCustomer: 1,
	domain.CallRequestStatePendingOnWSO2:     2,
	domain.CallRequestStateScheduled:         3,
	domain.CallRequestStateCustomerRejected:  4,
	domain.CallRequestStateWSO2Rejected:      5,
	domain.CallRequestStateCanceled:          6,
	domain.CallRequestStateNotesPending:      7,
	domain.CallRequestStateConcluded:         8,
}

// callRequestKeyToState is the inverse of callRequestStateToKey: it maps the
// ServiceNow integer choice-list key back to the domain string enum. This keeps
// the numeric stateKey confined to this adapter so callers only ever see the
// string enum.
var callRequestKeyToState = map[int]domain.CallRequestStateType{
	1: domain.CallRequestStatePendingOnCustomer,
	2: domain.CallRequestStatePendingOnWSO2,
	3: domain.CallRequestStateScheduled,
	4: domain.CallRequestStateCustomerRejected,
	5: domain.CallRequestStateWSO2Rejected,
	6: domain.CallRequestStateCanceled,
	7: domain.CallRequestStateNotesPending,
	8: domain.CallRequestStateConcluded,
}

// toDomainCallRequestState normalizes a raw SN state into the domain state, whose
// id is always the string enum key. If the raw id is an integer choice-list key
// it is mapped via callRequestKeyToState; if it is already a string it is passed
// through unchanged. This is the only place the numeric key crosses into a view.
func toDomainCallRequestState(s snCallRequestState) domain.CallRequestState {
	out := domain.CallRequestState{Label: s.Label}
	if len(s.ID) == 0 {
		return out
	}
	var key int
	if err := json.Unmarshal(s.ID, &key); err == nil {
		if state, ok := callRequestKeyToState[key]; ok {
			out.ID = string(state)
		}
		return out
	}
	var str string
	if err := json.Unmarshal(s.ID, &str); err == nil {
		out.ID = str
	}
	return out
}

// validCallRequestStates is the set of accepted CallRequestStateType values.
var validCallRequestStates = map[domain.CallRequestStateType]struct{}{
	domain.CallRequestStatePendingOnCustomer: {},
	domain.CallRequestStatePendingOnWSO2:     {},
	domain.CallRequestStateScheduled:         {},
	domain.CallRequestStateCustomerRejected:  {},
	domain.CallRequestStateWSO2Rejected:      {},
	domain.CallRequestStateCanceled:          {},
	domain.CallRequestStateNotesPending:      {},
	domain.CallRequestStateConcluded:         {},
}

// snCallRequestUpdateResponse mirrors the SN integration service PATCH /call-requests/{id} response.
type snCallRequestUpdateResponse struct {
	Message     string `json:"message"`
	CallRequest struct {
		ID        string `json:"id"`
		UpdatedOn string `json:"updatedOn"`
		UpdatedBy string `json:"updatedBy"`
	} `json:"callRequest"`
}

type snCallRequestService struct {
	client *integrationservice.Client
}

// NewServiceNowCallRequestService constructs a CallRequestService backed by the SN integration service.
func NewServiceNowCallRequestService(client *integrationservice.Client) CallRequestService {
	return &snCallRequestService{client: client}
}

// CreateCallRequest implements CallRequestService.
func (s *snCallRequestService) CreateCallRequest(ctx context.Context, req domain.CreateCallRequestRequest) (domain.CreateCallRequestResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	if req.CaseID == "" {
		return domain.CreateCallRequestResponse{}, &apierror.ValidationError{Msg: "caseId is required"}
	}
	if err := validateUUIDs("caseId", []string{req.CaseID}); err != nil {
		return domain.CreateCallRequestResponse{}, err
	}
	if req.Reason == "" {
		return domain.CreateCallRequestResponse{}, &apierror.ValidationError{Msg: "reason is required"}
	}
	if len(req.UTCTimes) == 0 {
		return domain.CreateCallRequestResponse{}, &apierror.ValidationError{Msg: "utcTimes must not be empty"}
	}
	if req.DurationMinutes <= 0 {
		return domain.CreateCallRequestResponse{}, &apierror.ValidationError{Msg: "durationInMinutes must be positive"}
	}

	payload := snCallRequestCreatePayload{
		CaseID:          uuidToSysid(req.CaseID),
		Reason:          req.Reason,
		UTCTimes:        req.UTCTimes,
		DurationMinutes: req.DurationMinutes,
	}
	raw, err := s.client.Post(ctx, "/call-requests", token, payload)
	if err != nil {
		return domain.CreateCallRequestResponse{}, err
	}

	var snResp snCallRequestCreateResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.CreateCallRequestResponse{}, fmt.Errorf("sn call requests: parse create response: %w", err)
	}

	var resp domain.CreateCallRequestResponse
	resp.Message = snResp.Message
	resp.CallRequest.ID = sysidToUUID(snResp.CallRequest.ID)
	resp.CallRequest.CreatedOn = snResp.CallRequest.CreatedOn
	resp.CallRequest.CreatedBy = snResp.CallRequest.CreatedBy
	resp.CallRequest.State = toDomainCallRequestState(snResp.CallRequest.State)
	return resp, nil
}

// SearchCallRequests implements CallRequestService.
func (s *snCallRequestService) SearchCallRequests(ctx context.Context, req domain.SearchCallRequestsRequest) (domain.SearchCallRequestsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCallRequestsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	if req.CaseID == "" {
		return domain.SearchCallRequestsResponse{}, &apierror.ValidationError{Msg: "caseId is required"}
	}
	if err := validateUUIDs("caseId", []string{req.CaseID}); err != nil {
		return domain.SearchCallRequestsResponse{}, err
	}

	payload := snCallRequestSearchPayload{
		CaseID:     uuidToSysid(req.CaseID),
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}
	if req.Filters != nil && len(req.Filters.States) > 0 {
		keys := make([]int, 0, len(req.Filters.States))
		for _, s := range req.Filters.States {
			if _, ok := validCallRequestStates[s]; !ok {
				return domain.SearchCallRequestsResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid state %q", s)}
			}
			keys = append(keys, callRequestStateToKey[s])
		}
		payload.Filters = &snCallRequestSearchFilters{StateKeys: keys}
	}

	raw, err := s.client.Post(ctx, "/call-requests/search", token, payload)
	if err != nil {
		return domain.SearchCallRequestsResponse{}, err
	}

	var snResp snCallRequestsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchCallRequestsResponse{}, fmt.Errorf("sn call requests: parse search response: %w", err)
	}

	views := mapSNCallRequestsToViews(snResp.CallRequests)

	total := snResp.TotalRecords
	return domain.SearchCallRequestsResponse{
		CallRequests: views,
		Total:        total,
		Limit:        req.Pagination.Limit,
		Offset:       req.Pagination.Offset,
	}, nil
}

// mapSNCallRequestsToViews converts raw SN call request records to domain views.
// Shared by SearchCallRequests (case-scoped) and SearchAllCallRequests (cross-case) --
// both call the same underlying SN response shape.
func mapSNCallRequestsToViews(crs []snCallRequest) []domain.CallRequestView {
	views := make([]domain.CallRequestView, 0, len(crs))
	for _, cr := range crs {
		views = append(views, domain.CallRequestView{
			ID:     sysidToUUID(cr.ID),
			Number: cr.Number,
			Case: domain.CallRequestCaseRef{
				ID:     sysidToUUID(cr.Case.ID),
				Name:   cr.Case.Name,
				Number: cr.Case.Number,
			},
			Reason:             cr.Reason,
			PreferredTimes:     cr.PreferredTimes,
			DurationMin:        cr.DurationMin,
			ScheduleTime:       cr.ScheduleTime,
			MeetingLink:        cr.MeetingLink,
			CreatedOn:          cr.CreatedOn,
			UpdatedOn:          cr.UpdatedOn,
			State:              toDomainCallRequestState(cr.State),
			CancellationReason: cr.CancellationReason,
			Assignee:           cr.Assignee,
			Notes:              cr.Notes,
			Plan:               cr.Plan,
			Attendees:          cr.Attendees,
			ActionItems:        cr.ActionItems,
			ActualDurationMin:  cr.ActualDurationMin,
		})
	}
	return views
}

// validCallRequestSortField is the set of accepted CallRequestSortField values.
var validCallRequestSortField = map[domain.CallRequestSortField]bool{
	domain.CallRequestSortFieldCreatedOn:    true,
	domain.CallRequestSortFieldUpdatedOn:    true,
	domain.CallRequestSortFieldScheduleTime: true,
}

// validCallRequestSortOrder is the set of accepted CallRequestSortOrder values.
var validCallRequestSortOrder = map[domain.CallRequestSortOrder]bool{
	domain.CallRequestSortOrderAsc:  true,
	domain.CallRequestSortOrderDesc: true,
}

// SearchAllCallRequests implements CallRequestService.
func (s *snCallRequestService) SearchAllCallRequests(ctx context.Context, req domain.SearchAllCallRequestsRequest) (domain.SearchCallRequestsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCallRequestsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	if err := validateUUIDs("filters.assignedUserIds", req.Filters.AssignedUserIDs); err != nil {
		return domain.SearchCallRequestsResponse{}, err
	}
	if err := validateUUIDs("filters.assignmentTeamIds", req.Filters.AssignmentTeamIDs); err != nil {
		return domain.SearchCallRequestsResponse{}, err
	}
	// domainStatesToSNIDs silently skips unrecognized values, which omitempty
	// then drops from the SN payload entirely -- validate up front so an
	// unrecognized value errors instead of silently widening the result set,
	// mirroring the case search's States/ExcludeStates validation.
	for _, st := range req.Filters.CaseStates {
		if !validCaseState[st] {
			return domain.SearchCallRequestsResponse{}, &apierror.ValidationError{Msg: "filters.caseStates contains invalid value: " + string(st)}
		}
	}
	// Same reasoning as CaseStates above, and it matters more here: an
	// exclusion value that got silently dropped would widen the result set
	// rather than narrow it, which is the harder failure to notice.
	for _, st := range req.Filters.ExcludeCaseStates {
		if !validCaseState[st] {
			return domain.SearchCallRequestsResponse{}, &apierror.ValidationError{Msg: "filters.excludeCaseStates contains invalid value: " + string(st)}
		}
	}

	if req.SortBy.Field == "" {
		req.SortBy.Field = domain.CallRequestSortFieldUpdatedOn
	} else if !validCallRequestSortField[req.SortBy.Field] {
		return domain.SearchCallRequestsResponse{}, &apierror.ValidationError{Msg: "sortBy.field must be one of: createdOn, updatedOn, scheduleTime"}
	}
	if req.SortBy.Order == "" {
		req.SortBy.Order = domain.CallRequestSortOrderDesc
	} else if !validCallRequestSortOrder[req.SortBy.Order] {
		return domain.SearchCallRequestsResponse{}, &apierror.ValidationError{Msg: "sortBy.order must be one of: asc, desc"}
	}

	payload := snCallRequestsSearchAllPayload{
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
		SortBy: &snCallRequestSort{
			Field: string(req.SortBy.Field),
			Order: string(req.SortBy.Order),
		},
	}

	var filters snCallRequestsSearchAllFilters
	hasFilters := false
	if len(req.Filters.AssignedUserIDs) > 0 {
		filters.AssignedUserIDs = uuidsToSysids(req.Filters.AssignedUserIDs)
		hasFilters = true
	}
	if len(req.Filters.States) > 0 {
		keys := make([]int, 0, len(req.Filters.States))
		for _, st := range req.Filters.States {
			if _, ok := validCallRequestStates[st]; !ok {
				return domain.SearchCallRequestsResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid state %q", st)}
			}
			keys = append(keys, callRequestStateToKey[st])
		}
		filters.StateKeys = keys
		hasFilters = true
	}
	if len(req.Filters.CaseStates) > 0 {
		filters.CaseStates = domainStatesToSNIDs(req.Filters.CaseStates)
		hasFilters = true
	}
	if len(req.Filters.ExcludeCaseStates) > 0 {
		filters.ExcludeCaseStates = domainStatesToSNIDs(req.Filters.ExcludeCaseStates)
		hasFilters = true
	}
	if len(req.Filters.AssignmentTeamIDs) > 0 {
		filters.AssignmentTeamIDs = uuidsToSysids(req.Filters.AssignmentTeamIDs)
		hasFilters = true
	}
	if hasFilters {
		payload.Filters = &filters
	}

	raw, err := s.client.Post(ctx, "/call-requests/search-all", token, payload)
	if err != nil {
		return domain.SearchCallRequestsResponse{}, err
	}

	var snResp snCallRequestsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchCallRequestsResponse{}, fmt.Errorf("sn call requests: parse search-all response: %w", err)
	}

	return domain.SearchCallRequestsResponse{
		CallRequests: mapSNCallRequestsToViews(snResp.CallRequests),
		Total:        snResp.TotalRecords,
		Offset:       req.Pagination.Offset,
		Limit:        req.Pagination.Limit,
	}, nil
}

// UpdateCallRequest implements CallRequestService.
func (s *snCallRequestService) UpdateCallRequest(ctx context.Context, req domain.UpdateCallRequestRequest) (domain.UpdateCallRequestResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.UpdateCallRequestResponse{}, err
	}

	if _, ok := validCallRequestStates[req.State]; !ok {
		return domain.UpdateCallRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid state %q", req.State)}
	}
	if req.DurationMinutes != nil && *req.DurationMinutes <= 0 {
		return domain.UpdateCallRequestResponse{}, &apierror.ValidationError{Msg: "durationInMinutes must be positive"}
	}
	if req.UTCTimes != nil && len(req.UTCTimes) == 0 {
		return domain.UpdateCallRequestResponse{}, &apierror.ValidationError{Msg: "utcTimes must not be empty when provided"}
	}
	if req.MeetingDate != nil {
		if _, err := time.Parse(time.RFC3339, *req.MeetingDate); err != nil {
			return domain.UpdateCallRequestResponse{}, &apierror.ValidationError{Msg: "meetingDate must be a valid RFC3339 timestamp"}
		}
	}
	if req.ActualDurationMin != nil && *req.ActualDurationMin <= 0 {
		return domain.UpdateCallRequestResponse{}, &apierror.ValidationError{Msg: "actualDurationMin must be positive"}
	}
	// State-specific required fields (mirrors the backing data source's per-state rules).
	switch req.State {
	case domain.CallRequestStateScheduled:
		if req.MeetingDate == nil || strings.TrimSpace(*req.MeetingDate) == "" {
			return domain.UpdateCallRequestResponse{}, &apierror.ValidationError{Msg: "meetingDate is required when state is scheduled"}
		}
		if req.DurationMinutes == nil {
			return domain.UpdateCallRequestResponse{}, &apierror.ValidationError{Msg: "durationInMinutes is required when state is scheduled"}
		}
	case domain.CallRequestStateConcluded:
		if req.Notes == nil || strings.TrimSpace(*req.Notes) == "" {
			return domain.UpdateCallRequestResponse{}, &apierror.ValidationError{Msg: "notes is required when state is concluded"}
		}
	}

	sysid := uuidToSysid(req.ID)

	if req.CaseID != "" {
		if err := validateUUIDs("caseId", []string{req.CaseID}); err != nil {
			return domain.UpdateCallRequestResponse{}, err
		}
		if err := s.verifyCallRequestBelongsToCase(ctx, token, uuidToSysid(req.CaseID), sysid); err != nil {
			return domain.UpdateCallRequestResponse{}, err
		}
	}

	payload := snCallRequestUpdatePayload{
		StateKey:           callRequestStateToKey[req.State],
		CancellationReason: req.CancellationReason,
		UTCTimes:           req.UTCTimes,
		DurationMinutes:    req.DurationMinutes,
		MeetingDate:        req.MeetingDate,
		Assignee:           req.Assignee,
		Notes:              req.Notes,
		Plan:               req.Plan,
		Attendees:          req.Attendees,
		ActionItems:        req.ActionItems,
		ActualDurationMin:  req.ActualDurationMin,
	}
	raw, err := s.client.Patch(ctx, fmt.Sprintf("/call-requests/%s", sysid), token, payload)
	if err != nil {
		return domain.UpdateCallRequestResponse{}, err
	}

	var snResp snCallRequestUpdateResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.UpdateCallRequestResponse{}, fmt.Errorf("sn call requests: parse update response: %w", err)
	}

	var resp domain.UpdateCallRequestResponse
	resp.Message = snResp.Message
	resp.CallRequest.ID = sysidToUUID(snResp.CallRequest.ID)
	resp.CallRequest.UpdatedOn = snResp.CallRequest.UpdatedOn
	resp.CallRequest.UpdatedBy = snResp.CallRequest.UpdatedBy
	return resp, nil
}

// verifyCallRequestBelongsToCase pages through all call requests for caseSysid
// and returns nil if callRequestSysid is found, or NotFoundError if exhausted.
func (s *snCallRequestService) verifyCallRequestBelongsToCase(ctx context.Context, token, caseSysid, callRequestSysid string) error {
	const pageSize = 50
	offset := 0
	for {
		payload := snCallRequestSearchPayload{
			CaseID:     caseSysid,
			Pagination: snProjectPagination{Limit: pageSize, Offset: offset},
		}
		raw, err := s.client.Post(ctx, "/call-requests/search", token, payload)
		if err != nil {
			return err
		}
		var resp snCallRequestsResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return fmt.Errorf("sn call requests: verify ownership: parse response: %w", err)
		}
		for _, cr := range resp.CallRequests {
			if cr.ID == callRequestSysid {
				return nil
			}
		}
		if offset+len(resp.CallRequests) >= resp.TotalRecords {
			break
		}
		offset += pageSize
	}
	return &apierror.NotFoundError{Msg: "call request not found for this case"}
}
