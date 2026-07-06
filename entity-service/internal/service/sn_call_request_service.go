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

// snCallRequestCreateResponse mirrors the SN integration service POST /call-requests response.
type snCallRequestCreateResponse struct {
	Message     string `json:"message"`
	CallRequest struct {
		ID        string                  `json:"id"`
		CreatedOn string                  `json:"createdOn"`
		CreatedBy string                  `json:"createdBy"`
		State     domain.CallRequestState `json:"state"`
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

// snCallRequestsResponse mirrors the SN integration service POST /call-requests/search response.
type snCallRequestsResponse struct {
	CallRequests []snCallRequest `json:"callRequests"`
	TotalRecords int             `json:"totalRecords"`
	Offset       int             `json:"offset"`
	Limit        int             `json:"limit"`
}

type snCallRequest struct {
	ID                 string                  `json:"id"`
	Number             string                  `json:"number"`
	Case               snCallRequestCaseRef    `json:"case"`
	Reason             *string                 `json:"reason"`
	PreferredTimes     []string                `json:"preferredTimes"`
	DurationMin        int                     `json:"durationMin"`
	ScheduleTime       *string                 `json:"scheduleTime"`
	MeetingLink        *string                 `json:"meetingLink"`
	CreatedOn          string                  `json:"createdOn"`
	UpdatedOn          string                  `json:"updatedOn"`
	State              domain.CallRequestState `json:"state"`
	CancellationReason *string                 `json:"cancellationReason,omitempty"`
	Assignee           *string                 `json:"assignee,omitempty"`
	Notes              *string                 `json:"notes,omitempty"`
	Plan               *string                 `json:"plan,omitempty"`
	Attendees          *string                 `json:"attendees,omitempty"`
	ActionItems        *string                 `json:"actionItems,omitempty"`
	ActualDurationMin  *int                    `json:"actualDurationMin,omitempty"`
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

// snCallRequestSchedulePayload mirrors POST /call-requests/{id}/schedule in the SN integration service.
type snCallRequestSchedulePayload struct {
	MeetingDate     string `json:"meetingDate"`
	DurationMinutes int    `json:"durationInMinutes"`
	Assignee        string `json:"assignee,omitempty"`
}

// snCallRequestRejectPayload mirrors POST /call-requests/{id}/reject in the SN integration service.
type snCallRequestRejectPayload struct {
	Reason string `json:"reason,omitempty"`
}

// snCallRequestNotesPayload mirrors POST /call-requests/{id}/notes in the SN integration service.
type snCallRequestNotesPayload struct {
	Notes             string `json:"notes"`
	Plan              string `json:"plan,omitempty"`
	Attendees         string `json:"attendees,omitempty"`
	ActionItems       string `json:"actionItems,omitempty"`
	ActualDurationMin *int   `json:"actualDurationMin,omitempty"`
}

// snCallRequestActionResponse mirrors the SN integration service response for the
// schedule/reject/notes agent actions. MeetingLink and ScheduleTime are only set
// by the schedule action.
type snCallRequestActionResponse struct {
	Message     string `json:"message"`
	CallRequest struct {
		ID           string                  `json:"id"`
		UpdatedOn    string                  `json:"updatedOn"`
		State        domain.CallRequestState `json:"state"`
		MeetingLink  *string                 `json:"meetingLink,omitempty"`
		ScheduleTime *string                 `json:"scheduleTime,omitempty"`
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
	if token == "" {
		return domain.CreateCallRequestResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

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
	resp.CallRequest.State = snResp.CallRequest.State
	return resp, nil
}

// SearchCallRequests implements CallRequestService.
func (s *snCallRequestService) SearchCallRequests(ctx context.Context, req domain.SearchCallRequestsRequest) (domain.SearchCallRequestsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCallRequestsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchCallRequestsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

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

	views := make([]domain.CallRequestView, 0, len(snResp.CallRequests))
	for _, cr := range snResp.CallRequests {
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
			State:              cr.State,
			CancellationReason: cr.CancellationReason,
			Assignee:           cr.Assignee,
			Notes:              cr.Notes,
			Plan:               cr.Plan,
			Attendees:          cr.Attendees,
			ActionItems:        cr.ActionItems,
			ActualDurationMin:  cr.ActualDurationMin,
		})
	}

	total := snResp.TotalRecords
	return domain.SearchCallRequestsResponse{
		CallRequests: views,
		Total:        total,
		Limit:        req.Pagination.Limit,
		Offset:       req.Pagination.Offset,
	}, nil
}

// UpdateCallRequest implements CallRequestService.
func (s *snCallRequestService) UpdateCallRequest(ctx context.Context, req domain.UpdateCallRequestRequest) (domain.UpdateCallRequestResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.UpdateCallRequestResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

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

// ScheduleCallRequest implements CallRequestService.
func (s *snCallRequestService) ScheduleCallRequest(ctx context.Context, req domain.ScheduleCallRequestRequest) (domain.CallRequestActionResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.CallRequestActionResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.CallRequestActionResponse{}, err
	}
	if req.MeetingDate == "" {
		return domain.CallRequestActionResponse{}, &apierror.ValidationError{Msg: "meetingDate is required"}
	}
	if _, err := time.Parse(time.RFC3339, req.MeetingDate); err != nil {
		return domain.CallRequestActionResponse{}, &apierror.ValidationError{Msg: "meetingDate must be a valid RFC3339 timestamp"}
	}
	if req.DurationMinutes < 15 || req.DurationMinutes > 240 {
		return domain.CallRequestActionResponse{}, &apierror.ValidationError{Msg: "durationInMinutes must be between 15 and 240"}
	}

	sysid := uuidToSysid(req.ID)
	payload := snCallRequestSchedulePayload{
		MeetingDate:     req.MeetingDate,
		DurationMinutes: req.DurationMinutes,
		Assignee:        req.Assignee,
	}
	raw, err := s.client.Post(ctx, fmt.Sprintf("/call-requests/%s/schedule", sysid), token, payload)
	if err != nil {
		return domain.CallRequestActionResponse{}, err
	}
	return parseCallRequestActionResponse(raw, "schedule")
}

// RejectCallRequest implements CallRequestService.
func (s *snCallRequestService) RejectCallRequest(ctx context.Context, req domain.RejectCallRequestRequest) (domain.CallRequestActionResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.CallRequestActionResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.CallRequestActionResponse{}, err
	}

	sysid := uuidToSysid(req.ID)
	payload := snCallRequestRejectPayload{Reason: req.Reason}
	raw, err := s.client.Post(ctx, fmt.Sprintf("/call-requests/%s/reject", sysid), token, payload)
	if err != nil {
		return domain.CallRequestActionResponse{}, err
	}
	return parseCallRequestActionResponse(raw, "reject")
}

// SendCallRequestNotes implements CallRequestService.
func (s *snCallRequestService) SendCallRequestNotes(ctx context.Context, req domain.SendCallRequestNotesRequest) (domain.CallRequestActionResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.CallRequestActionResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.CallRequestActionResponse{}, err
	}
	if req.Notes == "" {
		return domain.CallRequestActionResponse{}, &apierror.ValidationError{Msg: "notes is required"}
	}
	if req.ActualDurationMin != nil && *req.ActualDurationMin <= 0 {
		return domain.CallRequestActionResponse{}, &apierror.ValidationError{Msg: "actualDurationMin must be positive"}
	}

	sysid := uuidToSysid(req.ID)
	payload := snCallRequestNotesPayload{
		Notes:             req.Notes,
		Plan:              req.Plan,
		Attendees:         req.Attendees,
		ActionItems:       req.ActionItems,
		ActualDurationMin: req.ActualDurationMin,
	}
	raw, err := s.client.Post(ctx, fmt.Sprintf("/call-requests/%s/notes", sysid), token, payload)
	if err != nil {
		return domain.CallRequestActionResponse{}, err
	}
	return parseCallRequestActionResponse(raw, "notes")
}

// parseCallRequestActionResponse unmarshals an agent-action response from the SN
// integration service and maps the sysid back to a UUID for the caller.
func parseCallRequestActionResponse(raw json.RawMessage, action string) (domain.CallRequestActionResponse, error) {
	var snResp snCallRequestActionResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.CallRequestActionResponse{}, fmt.Errorf("sn call requests: parse %s response: %w", action, err)
	}

	var resp domain.CallRequestActionResponse
	resp.Message = snResp.Message
	resp.CallRequest.ID = sysidToUUID(snResp.CallRequest.ID)
	resp.CallRequest.UpdatedOn = snResp.CallRequest.UpdatedOn
	resp.CallRequest.State = snResp.CallRequest.State
	resp.CallRequest.MeetingLink = snResp.CallRequest.MeetingLink
	resp.CallRequest.ScheduleTime = snResp.CallRequest.ScheduleTime
	return resp, nil
}
