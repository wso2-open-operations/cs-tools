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

// Package service is declared in interfaces.go.
package service

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

var uuidRE = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// emailRE matches the Ballerina `Email` constraint used by the Customer Portal
// backend (`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`).
var emailRE = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// validateEmail returns a ValidationError unless email is present and matches
// emailRE. Used where a caller-supplied address is documented as
// `format: email` in openapi.yaml and would otherwise be forwarded upstream
// unchecked — a schema constraint the service does not enforce is not a
// constraint.
func validateEmail(email string) error {
	if email == "" {
		return &apierror.ValidationError{Msg: "email is required"}
	}
	if !emailRE.MatchString(email) {
		return &apierror.ValidationError{Msg: "email is not a valid email address"}
	}
	return nil
}

// validateUUIDs returns a ValidationError if any element of ids is not a valid UUID.
func validateUUIDs(field string, ids []string) error {
	for _, id := range ids {
		if !uuidRE.MatchString(id) {
			return &apierror.ValidationError{Msg: fmt.Sprintf("%s contains invalid UUID: %q", field, id)}
		}
	}
	return nil
}

// derefSeverity/derefState dereference domain.CaseView/Case's now-optional
// Severity/State (nil in practice for most real Postgres cases, but always
// set on the ServiceNow data source) to their plain zero-valued type, for a
// caller (map lookup, string conversion, equality check) that predates
// those fields becoming optional and only ever runs against the
// ServiceNow-backed path where a nil is not actually expected.
func derefSeverity(s *domain.CaseSeverity) domain.CaseSeverity {
	if s == nil {
		return ""
	}
	return *s
}

func derefState(s *domain.CaseState) domain.CaseState {
	if s == nil {
		return ""
	}
	return *s
}

func derefWorkState(s *domain.CaseWorkState) domain.CaseWorkState {
	if s == nil {
		return ""
	}
	return *s
}

// validateDateRange enforces the same rules as the Ballerina reference's
// shared validateDateRange helper: both dates must be exactly 10 characters
// in YYYY-MM-DD format, startDate must be strictly before endDate, and the
// span between them must not exceed one year.
func validateDateRange(startDate, endDate string) error {
	if len(startDate) != 10 || len(endDate) != 10 ||
		startDate[4:5] != "-" || startDate[7:8] != "-" ||
		endDate[4:5] != "-" || endDate[7:8] != "-" {
		return &apierror.ValidationError{Msg: "invalid date format. Expected YYYY-MM-DD"}
	}

	startYear, errSY := strconv.Atoi(startDate[0:4])
	startMonth, errSM := strconv.Atoi(startDate[5:7])
	startDay, errSD := strconv.Atoi(startDate[8:10])
	endYear, errEY := strconv.Atoi(endDate[0:4])
	endMonth, errEM := strconv.Atoi(endDate[5:7])
	endDay, errED := strconv.Atoi(endDate[8:10])
	if errSY != nil || errSM != nil || errSD != nil || errEY != nil || errEM != nil || errED != nil {
		return &apierror.ValidationError{Msg: "invalid date format. Expected YYYY-MM-DD"}
	}

	if startDate >= endDate {
		return &apierror.ValidationError{Msg: "endDate must be after startDate"}
	}

	yearDiff := endYear - startYear
	if yearDiff > 1 || (yearDiff == 1 && (endMonth > startMonth || (endMonth == startMonth && endDay > startDay))) {
		return &apierror.ValidationError{Msg: "date range must not exceed 1 year"}
	}

	return nil
}

const (
	defaultLimit = 20
	// maxLimit is 50 because the backing data source rejects anything above 50 with
	// an opaque validation error. Capping here, at the single choke point every
	// search normalizes through, means a new search cannot silently reintroduce the
	// mismatch: it gets a named error naming the limit instead of a downstream 400.
	maxLimit          = 50
	maxSearchQueryLen = 200

	defaultUserLimit = 10
	maxUserLimit     = 50
)

// normalizePagination applies defaults and clamps to p in-place.
// Returns a ValidationError if the limit exceeds maxLimit.
func normalizePagination(p *domain.Pagination) error {
	if p.Limit <= 0 {
		p.Limit = defaultLimit
	}
	if p.Limit > maxLimit {
		return &apierror.ValidationError{Msg: fmt.Sprintf("limit cannot exceed %d", maxLimit)}
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return nil
}

// normalizeUserPagination applies user-search-specific defaults (limit 10, max 50).
func normalizeUserPagination(p *domain.Pagination) error {
	if p.Limit <= 0 {
		p.Limit = defaultUserLimit
	}
	if p.Limit > maxUserLimit {
		return &apierror.ValidationError{Msg: "limit cannot exceed 50"}
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return nil
}

// validateSearchQuery returns a ValidationError if q exceeds the character limit.
func validateSearchQuery(q string) error {
	if utf8.RuneCountInString(q) > maxSearchQueryLen {
		return &apierror.ValidationError{Msg: "searchQuery cannot exceed 200 characters"}
	}
	return nil
}

type userService struct {
	repo repository.UserRepository
}

// NewUserService constructs a UserService backed by the given repository.
func NewUserService(repo repository.UserRepository) UserService {
	return &userService{repo: repo}
}

// GetUser implements UserService.
func (s *userService) GetUser(ctx context.Context, id string) (domain.UserDetail, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.UserDetail{}, err
	}
	u, err := s.repo.GetUserDetail(ctx, id)
	if err != nil {
		return domain.UserDetail{}, err
	}
	if u.Roles, err = s.repo.GetUserRoles(ctx, id); err != nil {
		return domain.UserDetail{}, err
	}
	if u.Groups, err = s.repo.GetUserGroups(ctx, id); err != nil {
		return domain.UserDetail{}, err
	}
	// Project access is a customer concept: staff have no project-contact rows.
	if u.UserType == domain.UserTypeCustomer && u.Email != "" {
		if u.ProjectAccess, err = s.repo.GetUserProjectAccess(ctx, u.Email); err != nil {
			return domain.UserDetail{}, err
		}
	}
	return u, nil
}

// SearchUsers implements UserService.
func (s *userService) SearchUsers(ctx context.Context, req domain.SearchUsersRequest) (domain.SearchUsersResponse, error) {
	if err := normalizeUserPagination(&req.Pagination); err != nil {
		return domain.SearchUsersResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchUsersResponse{}, err
	}
	if err := validateUUIDs("userIds", req.Filters.UserIDs); err != nil {
		return domain.SearchUsersResponse{}, err
	}
	if err := validateUUIDs("groupIds", req.Filters.GroupIDs); err != nil {
		return domain.SearchUsersResponse{}, err
	}
	if req.SortBy.Field != "" && !validUserSortField[req.SortBy.Field] {
		return domain.SearchUsersResponse{}, &apierror.ValidationError{Msg: "sortBy.field contains invalid value: " + string(req.SortBy.Field)}
	}
	if req.SortBy.Order != "" && req.SortBy.Field == "" {
		return domain.SearchUsersResponse{}, &apierror.ValidationError{Msg: "sortBy.order requires sortBy.field to be set"}
	}
	if req.SortBy.Order != "" && !validUserSortOrder[req.SortBy.Order] {
		return domain.SearchUsersResponse{}, &apierror.ValidationError{Msg: "sortBy.order contains invalid value: " + string(req.SortBy.Order)}
	}
	if len(req.Filters.UserNames) > 50 {
		return domain.SearchUsersResponse{}, &apierror.ValidationError{Msg: "userNames cannot contain more than 50 values"}
	}
	if len(req.Filters.Emails) > 50 {
		return domain.SearchUsersResponse{}, &apierror.ValidationError{Msg: "emails cannot contain more than 50 values"}
	}
	if len(req.Filters.RoleIDs) > 50 {
		return domain.SearchUsersResponse{}, &apierror.ValidationError{Msg: "roleIds cannot contain more than 50 values"}
	}

	users, total, err := s.repo.SearchUsers(ctx, req)
	if err != nil {
		return domain.SearchUsersResponse{}, err
	}

	return domain.SearchUsersResponse{
		Users:   users,
		Total:   total,
		Limit:   req.Pagination.Limit,
		Offset:  req.Pagination.Offset,
		HasMore: req.Pagination.Offset+len(users) < total,
	}, nil
}

// GetMe implements UserService.
//
// The Postgres data source has no JWT validation of its own (that happens at
// the BFF), so the caller's identity is resolved the same way the rest of
// this package resolves an acting user from a forwarded token: decode the
// (already-validated) x-user-id-token JWT's email claim and look up the
// matching row. See case_service.go's identical pattern for CreateCase /
// CreateCaseComment.
//
// Postgres has role/user_role tables (migrations 000004/000006 -- see
// SearchUsers' roleIds filter, which does query them) and no group-membership
// table at all. GetMe doesn't resolve either here: Roles is left empty rather
// than queried, since no caller has asked for it on this path yet, and Groups
// is always empty because there is genuinely nothing to resolve it from —
// the frontend's team/role resolution is simply a no-op for this data source
// today.
func (s *userService) GetMe(ctx context.Context) (domain.GetUserMeResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.GetUserMeResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return domain.GetUserMeResponse{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return domain.GetUserMeResponse{}, err
	}
	roles, err := s.repo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return domain.GetUserMeResponse{}, err
	}
	// GetUserMeResponse.Groups's own doc comment: best-effort, empty rather
	// than a failed request when the lookup errors.
	groups, err := s.repo.GetUserGroups(ctx, user.ID)
	if err != nil {
		groups = []domain.UserGroupRef{}
	}

	firstName := user.FirstName
	return domain.GetUserMeResponse{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: &firstName,
		LastName:  user.LastName,
		TimeZone:  user.Timezone,
		Roles:     roles,
		Groups:    groups,
	}, nil
}

// GetUsersByIDs implements UserService.
func (s *userService) GetUsersByIDs(ctx context.Context, ids []string) (domain.GetUsersByIDsResponse, error) {
	if len(ids) == 0 {
		return domain.GetUsersByIDsResponse{Users: []domain.User{}}, nil
	}
	users, err := s.repo.GetUsersByIDs(ctx, ids)
	if err != nil {
		return domain.GetUsersByIDsResponse{}, err
	}
	return domain.GetUsersByIDsResponse{Users: users}, nil
}

// CreateUser implements UserService.
func (s *userService) CreateUser(ctx context.Context, req domain.CreateUserRequest) (domain.User, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.User{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	actor, err := emailFromJWT(token)
	if err != nil {
		return domain.User{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}

	if err := validateEmail(req.Email); err != nil {
		return domain.User{}, err
	}
	if strings.TrimSpace(req.FirstName) == "" && strings.TrimSpace(req.LastName) == "" {
		return domain.User{}, &apierror.ValidationError{Msg: "firstName or lastName is required"}
	}
	if len(req.Roles) > 50 {
		return domain.User{}, &apierror.ValidationError{Msg: "roles cannot contain more than 50 values"}
	}

	return s.repo.CreateUser(ctx, req, actor)
}

