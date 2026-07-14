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
	ID             *string                  `json:"id"`
	Number         *string                  `json:"number"`
	InitialMessage *string                  `json:"initialMessage"`
	MessageCount   int                      `json:"messageCount"`
	Project        *snConversationEntityRef `json:"project"`
	Case           *snConversationEntityRef `json:"case"`
	State          *snConversationIntLabel  `json:"state"`
	CreatedOn      string                   `json:"createdOn"`
	CreatedBy      string                   `json:"createdBy"`
}

type snConversationEntityRef struct {
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
	CreatedByMe bool     `json:"createdByMe,omitempty"`
}

// snConversationStateKeyMap maps domain ConversationState enums to SN numeric state keys.
var snConversationStateKeyMap = map[domain.ConversationState]int{
	domain.ConversationStateActive:   2,
	domain.ConversationStateResolved: 3,
}

// snConversationStateLabelMap maps SN numeric state keys to domain enum strings.
var snConversationStateLabelMap = map[int]string{
	2: "ACTIVE",
	3: "RESOLVED",
}

var validConversationState = map[domain.ConversationState]bool{
	domain.ConversationStateActive:   true,
	domain.ConversationStateResolved: true,
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
	if token == "" {
		return domain.SearchConversationsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

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
			CreatedByMe: req.Filters.CreatedByMe,
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
			CreatedBy:      c.CreatedBy,
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
