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
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// CallRequestCreateResponse is the portal's response for POST /call-requests.
type CallRequestCreateResponse struct {
	ID        string `json:"id"`
	CreatedOn string `json:"createdOn"`
	State     string `json:"state"`
}

// MapCallRequestCreate builds the portal response from entity-service's CreateCallRequestResponse.
func MapCallRequestCreate(r entity.CreateCallRequestResponse) CallRequestCreateResponse {
	state := r.CallRequest.State.Label
	if state == "" {
		state = r.CallRequest.State.ID
	}
	return CallRequestCreateResponse{
		ID:        r.CallRequest.ID,
		CreatedOn: r.CallRequest.CreatedOn,
		State:     state,
	}
}

// CallRequestStateInfo holds the state of a call request.
type CallRequestStateInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// CallRequestCase is a compact reference to the case a call request belongs to.
type CallRequestCase struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Number *string `json:"number,omitempty"`
}

// CallRequestSummary is one item of the portal's response for
// POST /cases/{caseId}/call-requests/search.
type CallRequestSummary struct {
	ID             string               `json:"id"`
	Number         string               `json:"number"`
	Case           CallRequestCase      `json:"case"`
	Reason         *string              `json:"reason,omitempty"`
	PreferredTimes []string             `json:"preferredTimes,omitempty"`
	DurationMin    int                  `json:"durationMin"`
	ScheduleTime   *string              `json:"scheduleTime,omitempty"`
	MeetingLink    *string              `json:"meetingLink,omitempty"`
	CreatedOn      string               `json:"createdOn"`
	UpdatedOn      string               `json:"updatedOn"`
	State          CallRequestStateInfo `json:"state"`
}

// CallRequestSearchFilters holds the optional filter criteria for
// POST /cases/{caseId}/call-requests/search — shaped to match the
// frontend's request body (useGetCallRequests.ts's bodyObj.filters), which
// sends numeric ServiceNow choice-list keys (stateKeys), not
// entity-service's own string enum (see callRequestStateKeyToEnum for the
// translation).
type CallRequestSearchFilters struct {
	StateKeys []int `json:"stateKeys,omitempty"`
}

// CallRequestSearchRequest is the portal's request body for
// POST /cases/{caseId}/call-requests/search.
type CallRequestSearchRequest struct {
	Filters    CallRequestSearchFilters `json:"filters"`
	Pagination entity.Pagination        `json:"pagination"`
}

// toDashedID converts an identifier (either a dashed UUID or a 32-hex sysid)
// to a canonical lowercase 8-4-4-4-12 dashed UUID string expected by entity-service.
func toDashedID(id string) string {
	clean := strings.ToLower(strings.ReplaceAll(id, "-", ""))
	if len(clean) == 32 {
		return clean[0:8] + "-" + clean[8:12] + "-" + clean[12:16] + "-" + clean[16:20] + "-" + clean[20:32]
	}
	return strings.ToLower(id)
}

// BuildEntitySearchCallRequestsRequest translates the portal's request into
// entity-service's SearchCallRequestsRequest. caseID (the {caseId} path
// parameter) is always forced, never taken from the request body — the
// frontend's request body carries only filters/pagination. Unrecognized
// state keys are silently skipped (same behavior as
// caseIDsToEnums/BuildEntitySearchCasesRequest).
func BuildEntitySearchCallRequestsRequest(caseID string, req CallRequestSearchRequest) entity.SearchCallRequestsRequest {
	states := make([]string, 0, len(req.Filters.StateKeys))
	for _, key := range req.Filters.StateKeys {
		if enum, ok := callRequestStateKeyToEnum[key]; ok {
			states = append(states, enum)
		}
	}
	var filters *entity.SearchCallRequestsFilters
	if len(states) > 0 {
		filters = &entity.SearchCallRequestsFilters{States: states}
	}
	return entity.SearchCallRequestsRequest{
		CaseID:     toDashedID(caseID),
		Filters:    filters,
		Pagination: req.Pagination,
	}
}

// SearchCallRequestsResponse is the portal's response for
// POST /cases/{caseId}/call-requests/search. TotalRecords (not Total) to
// match the frontend's shared pagination envelope — see
// PaginationResponse in apps/customer-portal/webapp/src/types/common.ts.
type SearchCallRequestsResponse struct {
	CallRequests []CallRequestSummary `json:"callRequests"`
	TotalRecords int                  `json:"totalRecords"`
	Offset       int                  `json:"offset"`
	Limit        int                  `json:"limit"`
}

// MapSearchCallRequests builds the portal response from entity-service's SearchCallRequestsResponse.
func MapSearchCallRequests(r entity.SearchCallRequestsResponse) SearchCallRequestsResponse {
	items := make([]CallRequestSummary, 0, len(r.CallRequests))
	for _, v := range r.CallRequests {
		items = append(items, CallRequestSummary{
			ID:             v.ID,
			Number:         v.Number,
			Case:           CallRequestCase{ID: v.Case.ID, Name: v.Case.Name, Number: v.Case.Number},
			Reason:         v.Reason,
			PreferredTimes: v.PreferredTimes,
			DurationMin:    v.DurationMin,
			ScheduleTime:   v.ScheduleTime,
			MeetingLink:    v.MeetingLink,
			CreatedOn:      v.CreatedOn,
			UpdatedOn:      v.UpdatedOn,
			State:          CallRequestStateInfo{ID: v.State.ID, Label: v.State.Label},
		})
	}
	return SearchCallRequestsResponse{
		CallRequests: items,
		TotalRecords: r.Total,
		Offset:       r.Offset,
		Limit:        r.Limit,
	}
}

// CallRequestUpdateRequest is the portal's request shape for
// PATCH /cases/{caseId}/call-requests/{id} — a deliberately restricted
// subset of entity-service's UpdateCallRequestRequest. Excluded fields are
// agent-side actions entity-service itself documents as "set when an
// engineer schedules or concludes the call" — MeetingDate, Assignee, Notes,
// Plan, Attendees, ActionItems, ActualDurationMin — none of which a customer
// should be able to set on their own call request. StateKey (not State):
// the frontend was built against the old Ballerina backend and still sends
// ServiceNow's numeric choice-list key, not entity-service's own string
// enum — see callRequestStateKeyToEnum for the translation. Reason is
// excluded too: the frontend's PatchCallRequest carries an optional reason
// field, but entity-service's UpdateCallRequestRequest has no equivalent —
// there's no code path to forward it to, so it's silently dropped rather
// than worked around.
type CallRequestUpdateRequest struct {
	StateKey           int      `json:"stateKey"`
	CancellationReason *string  `json:"cancellationReason,omitempty"`
	UTCTimes           []string `json:"utcTimes,omitempty"`
	DurationMinutes    *int     `json:"durationInMinutes,omitempty"`
}

// BuildEntityUpdateCallRequestRequest converts the portal's restricted update
// request into entity-service's full request shape, leaving every excluded
// (agent-side) field nil. An unrecognized StateKey (including the zero value
// for an absent field) translates to an empty State, which entity-service's
// own validCallRequestStates check then rejects with a 400 — this backend
// doesn't duplicate that validation itself.
func BuildEntityUpdateCallRequestRequest(req CallRequestUpdateRequest) entity.UpdateCallRequestRequest {
	return entity.UpdateCallRequestRequest{
		State:              callRequestStateKeyToEnum[req.StateKey],
		CancellationReason: req.CancellationReason,
		UTCTimes:           req.UTCTimes,
		DurationMinutes:    req.DurationMinutes,
	}
}

// CallRequestUpdateResponse is the portal's response for PATCH /call-requests/{id}.
// Deliberately excludes entity-service's UpdatedBy (internal actor identity),
// consistent with the other update responses in this package.
type CallRequestUpdateResponse struct {
	ID        string `json:"id"`
	UpdatedOn string `json:"updatedOn"`
}

// MapCallRequestUpdate builds the portal response from entity-service's UpdateCallRequestResponse.
func MapCallRequestUpdate(r entity.UpdateCallRequestResponse) CallRequestUpdateResponse {
	return CallRequestUpdateResponse{
		ID:        r.CallRequest.ID,
		UpdatedOn: r.CallRequest.UpdatedOn,
	}
}
