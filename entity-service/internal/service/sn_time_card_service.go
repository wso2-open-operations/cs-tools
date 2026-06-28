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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snTimeCardStateLabelToSN maps domain TimeCardState to the SN label string.
var snTimeCardStateLabelToSN = map[domain.TimeCardState]string{
	domain.TimeCardStateApproved: "Approved",
}

// validTimeCardStates is used to validate incoming state values.
var validTimeCardStates = map[domain.TimeCardState]struct{}{
	domain.TimeCardStateApproved: {},
}

// snTimeCardStateLabelToDomain maps SN state labels (lowercased) to domain strings.
var snTimeCardStateLabelToDomain = map[string]string{
	"approved": string(domain.TimeCardStateApproved),
}

type snTimeCardSearchPayload struct {
	Filters    snTimeCardFilters   `json:"filters,omitempty"`
	Pagination snProjectPagination `json:"pagination"`
}

type snTimeCardFilters struct {
	ProjectIDs []string `json:"projectIds,omitempty"`
	StartDate  string   `json:"startDate,omitempty"`
	EndDate    string   `json:"endDate,omitempty"`
	States     []string `json:"states,omitempty"`
}

type snTimeCardsResponse struct {
	TimeCards    []snTimeCard `json:"timeCards"`
	TotalRecords int          `json:"totalRecords"`
	Limit        int          `json:"limit"`
	Offset       int          `json:"offset"`
}

type snTimeCard struct {
	ID          string              `json:"id"`
	TotalTime   float64             `json:"totalTime"`
	CreatedOn   string              `json:"createdOn"`
	HasBillable bool                `json:"hasBillable"`
	State       *snTimeCardLabel    `json:"state"`
	User        *snTimeCardRef      `json:"user"`
	ApprovedBy  *snTimeCardRef      `json:"approvedBy"`
	Project     *snTimeCardRef      `json:"project"`
	Case        *snTimeCardCaseRef  `json:"case"`
}

type snTimeCardLabel struct {
	Label string `json:"label"`
}

type snTimeCardRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type snTimeCardCaseRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Number string `json:"number"`
}

func snTimeCardToView(tc snTimeCard) domain.TimeCardView {
	view := domain.TimeCardView{
		ID:          sysidToUUID(tc.ID),
		TotalTime:   tc.TotalTime,
		CreatedOn:   tc.CreatedOn,
		HasBillable: tc.HasBillable,
	}

	if tc.State != nil {
		label := tc.State.Label
		if mapped, ok := snTimeCardStateLabelToDomain[strings.ToLower(label)]; ok {
			view.State = &mapped
		} else {
			view.State = &label
		}
	}

	if tc.User != nil {
		view.User = &domain.TimeCardRef{ID: sysidToUUID(tc.User.ID), Name: tc.User.Name}
	}
	if tc.ApprovedBy != nil {
		view.ApprovedBy = &domain.TimeCardRef{ID: sysidToUUID(tc.ApprovedBy.ID), Name: tc.ApprovedBy.Name}
	}
	if tc.Project != nil {
		view.Project = &domain.TimeCardRef{ID: sysidToUUID(tc.Project.ID), Name: tc.Project.Name}
	}
	if tc.Case != nil {
		view.Case = &domain.TimeCardCaseRef{
			ID:     sysidToUUID(tc.Case.ID),
			Name:   tc.Case.Name,
			Number: tc.Case.Number,
		}
	}

	return view
}

type snTimeCardService struct {
	client *integrationservice.Client
}

// NewServiceNowTimeCardService constructs a TimeCardService backed by the Choreo API.
func NewServiceNowTimeCardService(client *integrationservice.Client) TimeCardService {
	return &snTimeCardService{client: client}
}

func (s *snTimeCardService) SearchTimeCards(ctx context.Context, req domain.SearchTimeCardsRequest) (domain.SearchTimeCardsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchTimeCardsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchTimeCardsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	payload := snTimeCardSearchPayload{
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	if req.Filters != nil {
		for _, state := range req.Filters.States {
			if _, ok := validTimeCardStates[state]; !ok {
				return domain.SearchTimeCardsResponse{}, &apierror.ValidationError{Msg: "states contains invalid value: " + string(state)}
			}
		}
		if err := validateUUIDs("projectIds", req.Filters.ProjectIDs); err != nil {
			return domain.SearchTimeCardsResponse{}, err
		}

		snStates := make([]string, 0, len(req.Filters.States))
		for _, state := range req.Filters.States {
			snStates = append(snStates, snTimeCardStateLabelToSN[state])
		}

		payload.Filters = snTimeCardFilters{
			ProjectIDs: uuidsToSysids(req.Filters.ProjectIDs),
			States:     snStates,
		}
		if req.Filters.StartDate != nil {
			payload.Filters.StartDate = *req.Filters.StartDate
		}
		if req.Filters.EndDate != nil {
			payload.Filters.EndDate = *req.Filters.EndDate
		}
	}

	raw, err := s.client.Post(ctx, "/time-cards/search", token, payload)
	if err != nil {
		return domain.SearchTimeCardsResponse{}, err
	}

	var snResp snTimeCardsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchTimeCardsResponse{}, fmt.Errorf("sn time cards: parse response: %w", err)
	}

	views := make([]domain.TimeCardView, 0, len(snResp.TimeCards))
	for _, tc := range snResp.TimeCards {
		views = append(views, snTimeCardToView(tc))
	}

	return domain.SearchTimeCardsResponse{
		TimeCards: views,
		Total:     snResp.TotalRecords,
		Limit:     req.Pagination.Limit,
		Offset:    req.Pagination.Offset,
	}, nil
}
