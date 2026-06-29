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

// snUsersResponse mirrors the Choreo POST /users/search response.
type snUsersResponse struct {
	Users        []snUser `json:"users"`
	TotalRecords int      `json:"totalRecords"`
	Offset       int      `json:"offset"`
	Limit        int      `json:"limit"`
}

type snUser struct {
	ID        string   `json:"id"`
	UserName  string   `json:"userName"`
	Name      string   `json:"name"`
	Email     string   `json:"email"`
	TimeZone  *string  `json:"timeZone"`
	Active    bool     `json:"active"`
	CreatedOn string   `json:"createdOn"`
	UpdatedOn string   `json:"updatedOn"`
	Roles     []string `json:"roles"`
}

// snUserSearchPayload is the Choreo POST /users/search request body.
type snUserSearchPayload struct {
	Filters    snUserFilters       `json:"filters,omitempty"`
	SortBy     *snUserSort         `json:"sortBy,omitempty"`
	Pagination snProjectPagination `json:"pagination"`
}

type snUserFilters struct {
	SearchQuery string   `json:"searchQuery,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	UserNames   []string `json:"userNames,omitempty"`
	Emails      []string `json:"emails,omitempty"`
	Active      *bool    `json:"active,omitempty"`
}

type snUserSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

var validUserRole = map[domain.UserRole]bool{
	domain.UserRoleInternal:      true,
	domain.UserRoleAgent:         true,
	domain.UserRoleAdmin:         true,
	domain.UserRoleCommenter:     true,
	domain.UserRoleExternal:      true,
	domain.UserRoleCustomer:      true,
	domain.UserRoleCustomerAdmin: true,
	domain.UserRolePartner:       true,
	domain.UserRolePartnerAdmin:  true,
}

var validUserSortField = map[domain.UserSortField]bool{
	domain.UserSortFieldName:      true,
	domain.UserSortFieldCreatedOn: true,
	domain.UserSortFieldUpdatedOn: true,
}

var validUserSortOrder = map[domain.UserSortOrder]bool{
	domain.UserSortOrderAsc:  true,
	domain.UserSortOrderDesc: true,
}

type snUserService struct {
	client *integrationservice.Client
}

// NewServiceNowUserService constructs an SNUserService backed by the Choreo API.
func NewServiceNowUserService(client *integrationservice.Client) SNUserService {
	return &snUserService{client: client}
}

func (s *snUserService) SearchUsers(ctx context.Context, req domain.SearchUsersRequest) (domain.SearchSNUsersResponse, error) {
	if err := normalizeUserPagination(&req.Pagination); err != nil {
		return domain.SearchSNUsersResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchSNUsersResponse{}, err
	}
	if len(req.Filters.Roles) > 20 {
		return domain.SearchSNUsersResponse{}, &apierror.ValidationError{Msg: "roles cannot contain more than 20 values"}
	}
	if len(req.Filters.UserNames) > 50 {
		return domain.SearchSNUsersResponse{}, &apierror.ValidationError{Msg: "userNames cannot contain more than 50 values"}
	}
	if len(req.Filters.Emails) > 50 {
		return domain.SearchSNUsersResponse{}, &apierror.ValidationError{Msg: "emails cannot contain more than 50 values"}
	}
	for _, role := range req.Filters.Roles {
		if !validUserRole[role] {
			return domain.SearchSNUsersResponse{}, &apierror.ValidationError{Msg: "roles contains invalid value: " + string(role)}
		}
	}
	if req.SortBy.Field != "" && !validUserSortField[req.SortBy.Field] {
		return domain.SearchSNUsersResponse{}, &apierror.ValidationError{Msg: "sortBy.field contains invalid value: " + string(req.SortBy.Field)}
	}
	if req.SortBy.Order != "" && req.SortBy.Field == "" {
		return domain.SearchSNUsersResponse{}, &apierror.ValidationError{Msg: "sortBy.order requires sortBy.field to be set"}
	}
	if req.SortBy.Order != "" && !validUserSortOrder[req.SortBy.Order] {
		return domain.SearchSNUsersResponse{}, &apierror.ValidationError{Msg: "sortBy.order contains invalid value: " + string(req.SortBy.Order)}
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchSNUsersResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	roles := make([]string, len(req.Filters.Roles))
	for i, r := range req.Filters.Roles {
		roles[i] = string(r)
	}

	var snSortBy *snUserSort
	if req.SortBy.Field != "" {
		order := string(req.SortBy.Order)
		if order == "" {
			order = "asc"
		}
		snSortBy = &snUserSort{Field: string(req.SortBy.Field), Order: order}
	}

	payload := snUserSearchPayload{
		Filters: snUserFilters{
			SearchQuery: req.Filters.SearchQuery,
			Roles:       roles,
			UserNames:   req.Filters.UserNames,
			Emails:      req.Filters.Emails,
			Active:      req.Filters.Active,
		},
		SortBy:     snSortBy,
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/users/search", token, payload)
	if err != nil {
		return domain.SearchSNUsersResponse{}, err
	}

	var snResp snUsersResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchSNUsersResponse{}, fmt.Errorf("sn users: parse response: %w", err)
	}

	users := make([]domain.SNUser, 0, len(snResp.Users))
	for _, u := range snResp.Users {
		roles := u.Roles
		if roles == nil {
			roles = []string{}
		}
		users = append(users, domain.SNUser{
			ID:        sysidToUUID(u.ID),
			UserName:  u.UserName,
			Name:      u.Name,
			Email:     u.Email,
			TimeZone:  u.TimeZone,
			Active:    u.Active,
			CreatedOn: u.CreatedOn,
			UpdatedOn: u.UpdatedOn,
			Roles:     roles,
		})
	}

	return domain.SearchSNUsersResponse{
		Users:  users,
		Total:  snResp.TotalRecords,
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
	}, nil
}
