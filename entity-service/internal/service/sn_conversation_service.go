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
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snConversationsResponse mirrors the Choreo POST /conversations/search response.
type snConversationsResponse struct {
	Conversations []snConversation `json:"conversations"`
	TotalRecords  int              `json:"totalRecords"`
	Offset        int              `json:"offset"`
	Limit         int              `json:"limit"`
}

type snConversation struct {
	ID             *string                 `json:"id"`
	Number         *string                 `json:"number"`
	InitialMessage *string                 `json:"initialMessage"`
	MessageCount   int                     `json:"messageCount"`
	Project        *snEntityRef            `json:"project"`
	Case           *snEntityRef            `json:"case"`
	State          *snConversationIntLabel `json:"state"`
	CreatedOn      string                  `json:"createdOn"`
	CreatedBy      string                  `json:"createdBy"`
}

type snEntityRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type snConversationIntLabel struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

// snConversationSearchPayload is the Choreo POST /conversations/search request body.
type snConversationSearchPayload struct {
	Filters    snConversationFilters `json:"filters,omitempty"`
	SortBy     *snConversationSort   `json:"sortBy,omitempty"`
	Pagination snProjectPagination   `json:"pagination"`
}

type snConversationSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

type snConversationFilters struct {
	ProjectIDs  []string `json:"projectIds,omitempty"`
	StateKeys   []int    `json:"stateKeys,omitempty"`
	SearchQuery string   `json:"searchQuery,omitempty"`
	Number      string   `json:"number,omitempty"`
	CreatedByMe bool     `json:"createdByMe,omitempty"`
	CreatedBy   []string `json:"createdBy,omitempty"`
}

// snConversationStateKeyMap maps domain ConversationState enums to SN numeric state keys.
// Covers all 6 states (used by UpdateConversation and to interpret
// GetConversation's state, which may be any of them), though search filters
// (validConversationState) only ever accept ACTIVE/RESOLVED.
var snConversationStateKeyMap = map[domain.ConversationState]int{
	domain.ConversationStateOpen:      1,
	domain.ConversationStateActive:    2,
	domain.ConversationStateResolved:  3,
	domain.ConversationStateConverted: 4,
	domain.ConversationStateAbandoned: 5,
	domain.ConversationStateClosed:    6,
}

// snConversationStateLabelMap maps SN numeric state keys to domain enum strings.
var snConversationStateLabelMap = map[int]string{
	1: "OPEN",
	2: "ACTIVE",
	3: "RESOLVED",
	4: "CONVERTED",
	5: "ABANDONED",
	6: "CLOSED",
}

var validConversationState = map[domain.ConversationState]bool{
	domain.ConversationStateOpen:      true,
	domain.ConversationStateActive:    true,
	domain.ConversationStateResolved:  true,
	domain.ConversationStateConverted: true,
	domain.ConversationStateAbandoned: true,
	domain.ConversationStateClosed:    true,
}

// validConversationUpdateState is the full transition allow-list PATCH
// /conversations/{id} enforces — every state a conversation can be moved to,
// not just the two the search endpoint allows filtering by.
var validConversationUpdateState = map[domain.ConversationState]bool{
	domain.ConversationStateActive:    true,
	domain.ConversationStateResolved:  true,
	domain.ConversationStateConverted: true,
	domain.ConversationStateAbandoned: true,
	domain.ConversationStateClosed:    true,
}

var validConversationSortField = map[domain.ConversationSortField]bool{
	domain.ConversationSortFieldCreatedOn: true,
	domain.ConversationSortFieldUpdatedOn: true,
}

var validConversationSortOrder = map[domain.ConversationSortOrder]bool{
	domain.ConversationSortOrderAsc:  true,
	domain.ConversationSortOrderDesc: true,
}

// conversationDefaultLimit and conversationMaxLimit mirror the ServiceNow
// Choreo /conversations/search endpoint's own pagination bounds.
const (
	conversationDefaultLimit = 20
	conversationMaxLimit     = 50
)

// normalizeConversationPagination applies conversation-search-specific defaults
// (limit 20, max 50) matching the downstream Choreo endpoint's own constraints.
func normalizeConversationPagination(p *domain.Pagination) error {
	if p.Limit <= 0 {
		p.Limit = conversationDefaultLimit
	}
	if p.Limit > conversationMaxLimit {
		return &apierror.ValidationError{Msg: "limit cannot exceed 50"}
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return nil
}

// maxConversationCreatedByEntries and maxConversationCreatedByEntryLen bound the
// initiator (createdBy) filter before it reaches the backing integration. Length
// caps only, not email-format validation -- mirrors validateExactNumber's
// deliberate "don't hardcode format assumptions this layer shouldn't own" style.
const (
	maxConversationCreatedByEntries  = 20
	maxConversationCreatedByEntryLen = 254
)

// validateConversationCreatedBy checks an optional initiator-email filter list.
func validateConversationCreatedBy(emails []string) error {
	if len(emails) > maxConversationCreatedByEntries {
		return &apierror.ValidationError{Msg: fmt.Sprintf("createdBy cannot contain more than %d entries", maxConversationCreatedByEntries)}
	}
	for _, e := range emails {
		if utf8.RuneCountInString(e) > maxConversationCreatedByEntryLen {
			return &apierror.ValidationError{Msg: fmt.Sprintf("createdBy entry %q exceeds %d characters", e, maxConversationCreatedByEntryLen)}
		}
	}
	return nil
}

// conversationCreatedByRef builds a UserReference from the conversation payload's
// single createdBy string. The upstream integration carries only one identity
// field here (unlike case search, which supplies separate email and full-name
// fields), so there is no full name to populate: the value is treated as an
// email when it looks like one, otherwise as a display name.
func conversationCreatedByRef(createdBy string) *domain.UserReference {
	if strings.Contains(createdBy, "@") {
		return domain.NewUserReference("", createdBy, "")
	}
	return domain.NewUserReference("", "", createdBy)
}

type snConversationService struct {
	client *integrationservice.Client
}

// NewServiceNowConversationService constructs a ConversationService backed by the Choreo API.
func NewServiceNowConversationService(client *integrationservice.Client) ConversationService {
	return &snConversationService{client: client}
}

func (s *snConversationService) SearchConversations(ctx context.Context, req domain.SearchConversationsRequest) (domain.SearchConversationsResponse, error) {
	if err := normalizeConversationPagination(&req.Pagination); err != nil {
		return domain.SearchConversationsResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchConversationsResponse{}, err
	}
	if err := validateExactNumber("number", req.Filters.Number); err != nil {
		return domain.SearchConversationsResponse{}, err
	}
	if err := validateConversationCreatedBy(req.Filters.CreatedBy); err != nil {
		return domain.SearchConversationsResponse{}, err
	}
	if req.SortBy.Field != "" && !validConversationSortField[req.SortBy.Field] {
		return domain.SearchConversationsResponse{}, &apierror.ValidationError{Msg: "sortBy.field contains invalid value: " + string(req.SortBy.Field)}
	}
	if req.SortBy.Order != "" && !validConversationSortOrder[req.SortBy.Order] {
		return domain.SearchConversationsResponse{}, &apierror.ValidationError{Msg: "sortBy.order contains invalid value: " + string(req.SortBy.Order)}
	}
	for _, st := range req.Filters.States {
		if !validConversationState[st] {
			return domain.SearchConversationsResponse{}, &apierror.ValidationError{Msg: "states contains invalid value: " + string(st)}
		}
	}
	if err := validateUUIDs("projectIds", req.Filters.ProjectIDs); err != nil {
		return domain.SearchConversationsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	var snSortBy *snConversationSort
	if req.SortBy.Field != "" {
		order := string(req.SortBy.Order)
		if order == "" {
			order = "desc"
		}
		snSortBy = &snConversationSort{Field: string(req.SortBy.Field), Order: order}
	}

	stateKeys := make([]int, 0, len(req.Filters.States))
	for _, st := range req.Filters.States {
		stateKeys = append(stateKeys, snConversationStateKeyMap[st])
	}

	payload := snConversationSearchPayload{
		Filters: snConversationFilters{
			ProjectIDs:  uuidsToSysids(req.Filters.ProjectIDs),
			StateKeys:   stateKeys,
			SearchQuery: req.Filters.SearchQuery,
			Number:      stringPtrValue(req.Filters.Number),
			CreatedByMe: req.Filters.CreatedByMe,
			CreatedBy:   req.Filters.CreatedBy,
		},
		SortBy:     snSortBy,
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/conversations/search", token, payload)
	if err != nil {
		return domain.SearchConversationsResponse{}, err
	}

	var snResp snConversationsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchConversationsResponse{}, fmt.Errorf("sn conversations: parse response: %w", err)
	}

	views := make([]domain.SearchConversationView, 0, len(snResp.Conversations))
	for _, c := range snResp.Conversations {
		view := domain.SearchConversationView{
			Number:         c.Number,
			InitialMessage: c.InitialMessage,
			MessageCount:   c.MessageCount,
			CreatedOn:      c.CreatedOn,
			CreatedBy:      conversationCreatedByRef(c.CreatedBy),
		}
		if c.ID != nil && *c.ID != "" {
			id := sysidToUUID(*c.ID)
			view.ID = &id
		}
		if c.Project != nil {
			view.Project = &domain.EntityRef{ID: sysidToUUID(c.Project.ID), Name: c.Project.Name}
		}
		if c.Case != nil {
			view.Case = &domain.EntityRef{ID: sysidToUUID(c.Case.ID), Name: c.Case.Name}
		}
		if c.State != nil {
			if label, ok := snConversationStateLabelMap[c.State.ID]; ok {
				view.State = &label
			}
		}
		views = append(views, view)
	}

	return domain.SearchConversationsResponse{
		Conversations: views,
		Total:         snResp.TotalRecords,
		Limit:         req.Pagination.Limit,
		Offset:        req.Pagination.Offset,
	}, nil
}

// snConversationDetails mirrors the Choreo GET /conversations/{id} response
// (Ballerina's ConversationResponse, which inclusion-copies every field of
// Conversation and adds updatedOn/updatedBy — flattened here).
type snConversationDetails struct {
	ID             string                  `json:"id"`
	Number         *string                 `json:"number"`
	InitialMessage *string                 `json:"initialMessage"`
	MessageCount   int                     `json:"messageCount"`
	CreatedOn      string                  `json:"createdOn"`
	CreatedBy      string                  `json:"createdBy"`
	Project        *snEntityRef            `json:"project"`
	Case           *snEntityRef            `json:"case"`
	State          *snConversationIntLabel `json:"state"`
	UpdatedOn      string                  `json:"updatedOn"`
	UpdatedBy      string                  `json:"updatedBy"`
}

func (s *snConversationService) GetConversation(ctx context.Context, id string) (domain.ConversationDetails, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.ConversationDetails{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/conversations/"+uuidToSysid(id), token)
	if err != nil {
		return domain.ConversationDetails{}, err
	}

	var snResp snConversationDetails
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.ConversationDetails{}, fmt.Errorf("sn get conversation: parse response: %w", err)
	}

	details := domain.ConversationDetails{
		ID:             sysidToUUID(snResp.ID),
		Number:         snResp.Number,
		InitialMessage: snResp.InitialMessage,
		MessageCount:   snResp.MessageCount,
		CreatedOn:      snResp.CreatedOn,
		CreatedBy:      snResp.CreatedBy,
		UpdatedOn:      snResp.UpdatedOn,
		UpdatedBy:      snResp.UpdatedBy,
	}
	if snResp.Project != nil {
		details.Project = &domain.EntityRef{ID: sysidToUUID(snResp.Project.ID), Name: snResp.Project.Name}
	}
	if snResp.Case != nil {
		details.Case = &domain.EntityRef{ID: sysidToUUID(snResp.Case.ID), Name: snResp.Case.Name}
	}
	if snResp.State != nil {
		if label, ok := snConversationStateLabelMap[snResp.State.ID]; ok {
			details.State = &label
		}
	}

	return details, nil
}

// snConversationCreatePayload mirrors the Choreo POST /conversations request body.
type snConversationCreatePayload struct {
	ProjectID      string `json:"projectId"`
	InitialMessage string `json:"initialMessage"`
}

type snCreatedConversation struct {
	ID        string                 `json:"id"`
	Number    string                 `json:"number"`
	CreatedBy string                 `json:"createdBy"`
	CreatedOn string                 `json:"createdOn"`
	State     snConversationIntLabel `json:"state"`
}

// snConversationCreateResponse mirrors the Choreo POST /conversations response.
type snConversationCreateResponse struct {
	Message      string                `json:"message"`
	Conversation snCreatedConversation `json:"conversation"`
}

func (s *snConversationService) CreateConversation(ctx context.Context, req domain.CreateConversationRequest) (domain.CreateConversationResponse, error) {
	if err := validateUUIDs("projectId", []string{req.ProjectID}); err != nil {
		return domain.CreateConversationResponse{}, err
	}
	if req.InitialMessage == "" {
		return domain.CreateConversationResponse{}, &apierror.ValidationError{Msg: "initialMessage is required"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snConversationCreatePayload{
		ProjectID:      uuidToSysid(req.ProjectID),
		InitialMessage: req.InitialMessage,
	}

	raw, err := s.client.Post(ctx, "/conversations", token, payload)
	if err != nil {
		return domain.CreateConversationResponse{}, err
	}

	var snResp snConversationCreateResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.CreateConversationResponse{}, fmt.Errorf("sn create conversation: parse response: %w", err)
	}

	state := snConversationStateLabelMap[snResp.Conversation.State.ID]
	var statePtr *string
	if state != "" {
		statePtr = &state
	}

	return domain.CreateConversationResponse{
		Message: snResp.Message,
		Conversation: domain.CreatedConversation{
			ID:        sysidToUUID(snResp.Conversation.ID),
			Number:    snResp.Conversation.Number,
			CreatedBy: snResp.Conversation.CreatedBy,
			CreatedOn: snResp.Conversation.CreatedOn,
			State:     statePtr,
		},
	}, nil
}

// snConversationUpdatePayload mirrors the Choreo PATCH /conversations/{id} request body.
type snConversationUpdatePayload struct {
	StateKey int `json:"stateKey"`
}

type snUpdatedConversation struct {
	ID        string                 `json:"id"`
	Number    *string                `json:"number"`
	UpdatedOn string                 `json:"updatedOn"`
	UpdatedBy string                 `json:"updatedBy"`
	State     snConversationIntLabel `json:"state"`
}

// snConversationUpdateResponse mirrors the Choreo PATCH /conversations/{id} response.
type snConversationUpdateResponse struct {
	Message      string                `json:"message"`
	Conversation snUpdatedConversation `json:"conversation"`
}

func (s *snConversationService) UpdateConversation(ctx context.Context, id string, req domain.UpdateConversationRequest) (domain.UpdateConversationResponse, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.UpdateConversationResponse{}, err
	}
	if !validConversationUpdateState[req.State] {
		return domain.UpdateConversationResponse{}, &apierror.ValidationError{Msg: "state contains invalid value: " + string(req.State)}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snConversationUpdatePayload{StateKey: snConversationStateKeyMap[req.State]}

	raw, err := s.client.Patch(ctx, "/conversations/"+uuidToSysid(id), token, payload)
	if err != nil {
		return domain.UpdateConversationResponse{}, err
	}

	var snResp snConversationUpdateResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.UpdateConversationResponse{}, fmt.Errorf("sn update conversation: parse response: %w", err)
	}

	state := snConversationStateLabelMap[snResp.Conversation.State.ID]
	var statePtr *string
	if state != "" {
		statePtr = &state
	}

	return domain.UpdateConversationResponse{
		Message: snResp.Message,
		Conversation: domain.UpdatedConversation{
			ID:        sysidToUUID(snResp.Conversation.ID),
			Number:    snResp.Conversation.Number,
			UpdatedOn: snResp.Conversation.UpdatedOn,
			UpdatedBy: snResp.Conversation.UpdatedBy,
			State:     statePtr,
		},
	}, nil
}
