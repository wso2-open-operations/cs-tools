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
	"log"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snUserMeResponse mirrors the Choreo GET /users/me response.
type snUserMeResponse struct {
	ID        string   `json:"id"`
	Email     string   `json:"email"`
	FirstName *string  `json:"firstName"`
	LastName  string   `json:"lastName"`
	TimeZone  *string  `json:"timeZone"`
	UserType  string   `json:"userType"`
	Roles     []string `json:"roles"`
}

// snGroupMembersSearchPayload is the Choreo POST group-members/search request body.
type snGroupMembersSearchPayload struct {
	Filters snGroupMembersFilters `json:"filters"`
}

type snGroupMembersFilters struct {
	GroupIDs   []string `json:"groupIds,omitempty"`
	GroupNames []string `json:"groupNames,omitempty"`
	UserID     string   `json:"userId,omitempty"`
}

// snGroupMembersSearchResponse mirrors the Choreo POST group-members/search response.
type snGroupMembersSearchResponse struct {
	Memberships  []snGroupMembership `json:"memberships"`
	TotalRecords int                 `json:"totalRecords"`
}

type snGroupMembership struct {
	UserID    string `json:"userId"`
	GroupID   string `json:"groupId"`
	GroupName string `json:"groupName"`
}

// snPatchUserMePayload is the Choreo PATCH /users/me request body.
type snPatchUserMePayload struct {
	TimeZone string `json:"timeZone"`
}

// snPatchUserMeUpdated is the user sub-object in the Choreo PATCH /users/me response.
type snPatchUserMeUpdated struct {
	ID        string `json:"id"`
	UpdatedBy string `json:"updatedBy"`
	UpdatedOn string `json:"updatedOn"`
}

// snPatchUserMeResponse mirrors the Choreo PATCH /users/me response.
type snPatchUserMeResponse struct {
	Message string               `json:"message"`
	User    snPatchUserMeUpdated `json:"user"`
}

// snUsersResponse mirrors the Choreo POST /users/search response.
type snUsersResponse struct {
	Users        []snUser `json:"users"`
	TotalRecords int      `json:"totalRecords"`
	Offset       int      `json:"offset"`
	Limit        int      `json:"limit"`
}

type snUser struct {
	ID          string   `json:"id"`
	UserName    string   `json:"userName"`
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	TimeZone    *string  `json:"timeZone"`
	MobilePhone *string  `json:"mobilePhone"`
	UserType    string   `json:"userType"`
	Active      bool     `json:"active"`
	LockedOut   bool     `json:"lockedOut"`
	CreatedOn   string   `json:"createdOn"`
	UpdatedOn   string   `json:"updatedOn"`
	Roles       []string `json:"roles"`
}

// snUserSearchPayload is the Choreo POST /users/search request body.
type snUserSearchPayload struct {
	Filters    snUserFilters       `json:"filters,omitempty"`
	SortBy     *snUserSort         `json:"sortBy,omitempty"`
	Pagination snProjectPagination `json:"pagination"`
}

type snUserFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
	// RoleNames is sent under the backing data source's open, unconstrained role
	// filter rather than its closed "roles" enum -- the role catalogue is
	// configuration-driven in the caller (the portal backend's directory package)
	// and not limited to that closed set, so a role outside it (e.g.
	// timecard_approver) must not be sent under "roles", which the upstream layer
	// rejects at request binding for values it doesn't recognize.
	RoleNames []string `json:"roleNames,omitempty"`
	UserNames []string `json:"userNames,omitempty"`
	Emails    []string `json:"emails,omitempty"`
	UserIDs   []string `json:"userIds,omitempty"`
	Active    *bool    `json:"active,omitempty"`
}

// snProjectContactRowsPayload is the Choreo POST project-contacts/search request body.
type snProjectContactRowsPayload struct {
	Filters snProjectContactRowsFilters `json:"filters"`
}

type snProjectContactRowsFilters struct {
	Email string `json:"email"`
}

// snProjectContactRowsResponse mirrors the Choreo POST project-contacts/search response.
type snProjectContactRowsResponse struct {
	Contacts     []snProjectContactRow `json:"contacts"`
	TotalRecords int                   `json:"totalRecords"`
}

type snProjectContactRow struct {
	ProjectID              string   `json:"projectId"`
	ProjectName            string   `json:"projectName"`
	ProjectKey             string   `json:"projectKey"`
	ContactEmail           string   `json:"contactEmail"`
	CustomerContactPresent bool     `json:"customerContactPresent"`
	CustomerContactEmail   string   `json:"customerContactEmail"`
	RegistrationState      string   `json:"registrationState"`
	NotificationsEnabled   bool     `json:"notificationsEnabled"`
	Roles                  []string `json:"roles"`
	GrantsCaseAccess       bool     `json:"grantsCaseAccess"`
}

// snUserIDFilterLimit caps how many ids one search may pass upstream. The upstream builds
// an IN clause from them, and an over-long encoded query is silently truncated there.
const snUserIDFilterLimit = 200

const (
	// snGroupIDFilterLimit caps the groupIds filter. Each id widens the membership query
	// that resolves the filter to a user-id set, and that set then feeds the same IN
	// clause snUserIDFilterLimit protects.
	snGroupIDFilterLimit = 50
	// snGroupNameFilterLimit caps the groupNames filter, which widens the membership
	// query exactly as groupIds does.
	snGroupNameFilterLimit = 50
)

type snUserSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
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
	// The ABT team registry is deployment configuration installed at startup
	// (domain.SetAbtTeams), not something this service fetches.
	return &snUserService{client: client}
}

func (s *snUserService) SearchUsers(ctx context.Context, req domain.SearchUsersRequest) (domain.SearchSNUsersResponse, error) {
	if err := normalizeUserPagination(&req.Pagination); err != nil {
		return domain.SearchSNUsersResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchSNUsersResponse{}, err
	}
	if len(req.Filters.RoleIDs) > 20 {
		return domain.SearchSNUsersResponse{}, &apierror.ValidationError{Msg: "roleIds cannot contain more than 20 values"}
	}
	if len(req.Filters.UserNames) > 50 {
		return domain.SearchSNUsersResponse{}, &apierror.ValidationError{Msg: "userNames cannot contain more than 50 values"}
	}
	if len(req.Filters.Emails) > 50 {
		return domain.SearchSNUsersResponse{}, &apierror.ValidationError{Msg: "emails cannot contain more than 50 values"}
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

	if len(req.Filters.UserIDs) > snUserIDFilterLimit {
		return domain.SearchSNUsersResponse{}, &apierror.ValidationError{
			Msg: fmt.Sprintf("userIds cannot contain more than %d values", snUserIDFilterLimit)}
	}
	// Reject malformed ids here rather than letting uuidToSysid pass them through
	// unchanged: upstream answers a bogus id with an opaque error, or an empty page that
	// looks like a legitimate "no such user".
	if err := validateUUIDs("userIds", req.Filters.UserIDs); err != nil {
		return domain.SearchSNUsersResponse{}, err
	}
	if len(req.Filters.GroupIDs) > snGroupIDFilterLimit {
		return domain.SearchSNUsersResponse{}, &apierror.ValidationError{
			Msg: fmt.Sprintf("groupIds cannot contain more than %d values", snGroupIDFilterLimit)}
	}
	if err := validateUUIDs("groupIds", req.Filters.GroupIDs); err != nil {
		return domain.SearchSNUsersResponse{}, err
	}
	if len(req.Filters.GroupNames) > snGroupNameFilterLimit {
		return domain.SearchSNUsersResponse{}, &apierror.ValidationError{
			Msg: fmt.Sprintf("groupNames cannot contain more than %d values", snGroupNameFilterLimit)}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	// Group and team membership cannot be expressed as a user-search filter upstream --
	// the data source cannot join users against group membership in one query. Resolve
	// both to a user-ID set here and intersect with any explicit userIds, so paging and
	// totals still come from the single upstream call rather than being recomputed here.
	userIDs, err := s.resolveMembershipUserIDs(ctx, token, req.Filters)
	if err != nil {
		return domain.SearchSNUsersResponse{}, err
	}
	if userIDs != nil && len(userIDs) == 0 {
		// A membership filter was supplied and matched nobody. Returning the unfiltered
		// page here would be a silent lie, so return an empty page instead.
		return domain.SearchSNUsersResponse{
			Users:  []domain.SNUser{},
			Total:  0,
			Limit:  req.Pagination.Limit,
			Offset: req.Pagination.Offset,
		}, nil
	}

	roles := make([]string, len(req.Filters.RoleIDs))
	for i, r := range req.Filters.RoleIDs {
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
			RoleNames:   roles,
			UserNames:   req.Filters.UserNames,
			Emails:      req.Filters.Emails,
			UserIDs:     userIDs,
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
			ID:          sysidToUUID(u.ID),
			UserName:    u.UserName,
			Name:        u.Name,
			Email:       u.Email,
			TimeZone:    u.TimeZone,
			MobilePhone: u.MobilePhone,
			UserType:    domain.UserType(u.UserType),
			Active:      u.Active,
			LockedOut:   u.LockedOut,
			CreatedOn:   u.CreatedOn,
			UpdatedOn:   u.UpdatedOn,
			Roles:       roles,
		})
	}

	return domain.SearchSNUsersResponse{
		Users:  users,
		Total:  snResp.TotalRecords,
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
	}, nil
}

// resolveMembershipUserIDs turns the groupIds/teamIds/userIds filters into the single
// user-id list the upstream search understands.
//
// nil means "do not constrain by id". An empty non-nil slice means a membership filter was
// supplied and matched nobody, which the caller must render as an empty page.
func (s *snUserService) resolveMembershipUserIDs(
	ctx context.Context, token string, filters domain.SearchUsersFilters,
) ([]string, error) {
	explicit := make([]string, 0, len(filters.UserIDs))
	for _, id := range filters.UserIDs {
		explicit = append(explicit, uuidToSysid(id))
	}

	if len(filters.GroupIDs) == 0 && len(filters.GroupNames) == 0 {
		if len(explicit) == 0 {
			return nil, nil
		}
		return explicit, nil
	}

	groupIDs := make([]string, 0, len(filters.GroupIDs))
	for _, id := range filters.GroupIDs {
		groupIDs = append(groupIDs, uuidToSysid(id))
	}

	// Group names arrive already resolved: the team registry that maps a team key to a
	// group name is the caller's configuration, not this service's, because the backing
	// group ids differ between environments while the names do not.
	members, err := s.searchGroupMemberships(ctx, token, groupIDs, filters.GroupNames, "")
	if err != nil {
		return nil, err
	}

	memberIDs := make([]string, 0, len(members))
	seen := make(map[string]struct{}, len(members))
	for _, m := range members {
		if m.UserID == "" {
			continue
		}
		if _, dup := seen[m.UserID]; dup {
			continue
		}
		seen[m.UserID] = struct{}{}
		memberIDs = append(memberIDs, m.UserID)
	}

	if len(explicit) == 0 {
		return memberIDs, nil
	}

	// Both supplied: intersect, so the filters compose instead of one silently winning.
	intersection := make([]string, 0, len(explicit))
	for _, id := range explicit {
		if _, ok := seen[id]; ok {
			intersection = append(intersection, id)
		}
	}
	return intersection, nil
}

// searchGroupMemberships calls the upstream group-membership search. At least one of
// groupIDs, groupNames or userSysID must be set; passing only userSysID returns every
// group that user belongs to.
func (s *snUserService) searchGroupMemberships(
	ctx context.Context, token string, groupIDs, groupNames []string, userSysID string,
) ([]snGroupMembership, error) {
	payload := snGroupMembersSearchPayload{
		Filters: snGroupMembersFilters{
			GroupIDs:   groupIDs,
			GroupNames: groupNames,
			UserID:     userSysID,
		},
	}

	raw, err := s.client.Post(ctx, "/group-members/search", token, payload)
	if err != nil {
		return nil, err
	}

	var resp snGroupMembersSearchResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("sn users: parse group membership response: %w", err)
	}
	return resp.Memberships, nil
}

// GetUser handles GET /users/{id}.
//
// Built on the search's userIds filter because there is no get-by-id upstream. That filter
// also lifts the active-only default, so a deactivated user is still returned -- which
// matters, since "this user is deactivated" is often the answer the caller wants.
func (s *snUserService) GetUser(ctx context.Context, id string) (domain.SNUserDetail, error) {
	if id == "" {
		return domain.SNUserDetail{}, &apierror.ValidationError{Msg: "id is required"}
	}
	// uuidToSysid returns a non-canonical id unchanged, so validate before converting:
	// otherwise a malformed id is forwarded upstream, and reporting it as "required"
	// would send the caller looking for a missing parameter they did supply.
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.SNUserDetail{}, err
	}
	sysID := uuidToSysid(id)

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snUserSearchPayload{
		Filters:    snUserFilters{UserIDs: []string{sysID}},
		Pagination: snProjectPagination{Limit: 1, Offset: 0},
	}

	raw, err := s.client.Post(ctx, "/users/search", token, payload)
	if err != nil {
		return domain.SNUserDetail{}, err
	}

	var snResp snUsersResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SNUserDetail{}, fmt.Errorf("sn users: parse response: %w", err)
	}
	if len(snResp.Users) == 0 {
		return domain.SNUserDetail{}, &apierror.NotFoundError{Msg: "user not found"}
	}

	u := snResp.Users[0]
	roles := u.Roles
	if roles == nil {
		roles = []string{}
	}

	detail := domain.SNUserDetail{
		SNUser: domain.SNUser{
			ID:          sysidToUUID(u.ID),
			UserName:    u.UserName,
			Name:        u.Name,
			Email:       u.Email,
			TimeZone:    u.TimeZone,
			MobilePhone: u.MobilePhone,
			UserType:    domain.UserType(u.UserType),
			Active:      u.Active,
			LockedOut:   u.LockedOut,
			CreatedOn:   u.CreatedOn,
			UpdatedOn:   u.UpdatedOn,
			Roles:       roles,
		},
	}

	// The enrichments are best-effort: each degrades to empty on upstream failure rather
	// than failing the whole profile, matching how the caller's own team is resolved on
	// GET /users/me.
	//
	// Group membership is internal-only: the upstream ACL rejects this lookup for external
	// (customer) users, so skip the call entirely rather than making a request known to fail.
	if detail.UserType == domain.UserTypeExternal {
		detail.Groups = []domain.UserGroupRef{}
		detail.ProjectAccess = s.resolveProjectAccess(ctx, token, u.Email)
	} else {
		detail.Groups = s.resolveUserGroups(ctx, token, sysID)
	}

	return detail, nil
}

// resolveUserGroups lists every group the user belongs to.
//
// Which of those groups are teams is deliberately not decided here: the team registry is
// the caller's configuration, so the caller maps these names to teams. Reporting the raw
// membership keeps this service free of organisation vocabulary it would otherwise have to
// be told about on every deploy.
func (s *snUserService) resolveUserGroups(
	ctx context.Context, token, userSysID string,
) []domain.UserGroupRef {
	groups := []domain.UserGroupRef{}

	members, err := s.searchGroupMemberships(ctx, token, nil, nil, userSysID)
	if err != nil {
		log.Printf("sn users: group membership lookup for user failed: %v", err)
		return groups
	}

	for _, m := range members {
		groups = append(groups, domain.UserGroupRef{ID: sysidToUUID(m.GroupID), Name: m.GroupName})
	}
	return groups
}

// resolveProjectAccess lists the user's project-contact rows, including the ones the
// upstream access rule hides, so a caller can see why a contact cannot reach their cases.
func (s *snUserService) resolveProjectAccess(
	ctx context.Context, token, email string,
) []domain.UserProjectAccess {
	access := []domain.UserProjectAccess{}
	if email == "" {
		return access
	}

	payload := snProjectContactRowsPayload{Filters: snProjectContactRowsFilters{Email: email}}

	raw, err := s.client.Post(ctx, "/project-contacts/search", token, payload)
	if err != nil {
		log.Printf("sn users: project contact lookup failed: %v", err)
		return access
	}

	var resp snProjectContactRowsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		log.Printf("sn users: parse project contact response failed: %v", err)
		return access
	}

	for _, c := range resp.Contacts {
		access = append(access, domain.UserProjectAccess{
			ProjectID:            sysidToUUID(c.ProjectID),
			ProjectName:          c.ProjectName,
			ProjectKey:           c.ProjectKey,
			ContactEmail:         c.ContactEmail,
			ContactRecordPresent: c.CustomerContactPresent,
			ContactRecordEmail:   c.CustomerContactEmail,
			RegistrationState:    c.RegistrationState,
			NotificationsEnabled: c.NotificationsEnabled,
			Roles:                c.Roles,
			GrantsCaseAccess:     c.GrantsCaseAccess,
		})
	}
	return access
}

// GetMe handles GET /users/me.
//
// Group membership is internal-only, exactly as it is for GetUser: the upstream ACL rejects
// this lookup for external (customer) users, so skip the call entirely for them rather than
// making a request known to fail.
func (s *snUserService) GetMe(ctx context.Context) (domain.GetUserMeResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/users/me", token)
	if err != nil {
		return domain.GetUserMeResponse{}, err
	}

	var snResp snUserMeResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.GetUserMeResponse{}, fmt.Errorf("sn users: parse get-me response: %w", err)
	}

	roles := snResp.Roles
	if roles == nil {
		roles = []string{}
	}

	userType := domain.UserType(snResp.UserType)

	resp := domain.GetUserMeResponse{
		ID:        sysidToUUID(snResp.ID),
		Email:     snResp.Email,
		FirstName: snResp.FirstName,
		LastName:  snResp.LastName,
		TimeZone:  snResp.TimeZone,
		UserType:  userType,
		Roles:     roles,
	}

	if userType == domain.UserTypeExternal {
		resp.Groups = []domain.UserGroupRef{}
	} else {
		resp.Groups = s.resolveUserGroups(ctx, token, snResp.ID)
	}

	return resp, nil
}

func (s *snUserService) PatchMe(ctx context.Context, req domain.PatchUserMeRequest) (domain.PatchUserMeResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	// phone is accepted but not stored on this data source: callers update the
	// identity provider first and then send phone here too, so rejecting it would
	// fail the request after the phone was already saved and leave a combined
	// timeZone update unapplied. A phone-only request is therefore a no-op.
	if req.TimeZone == "" {
		if req.Phone != nil {
			return domain.PatchUserMeResponse{Message: "phone is not stored by this data source; no changes applied"}, nil
		}
		return domain.PatchUserMeResponse{}, &apierror.ValidationError{Msg: "timeZone is required"}
	}

	raw, err := s.client.Patch(ctx, "/users/me", token, snPatchUserMePayload{TimeZone: req.TimeZone})
	if err != nil {
		return domain.PatchUserMeResponse{}, err
	}

	var snResp snPatchUserMeResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.PatchUserMeResponse{}, fmt.Errorf("sn users: parse patch-me response: %w", err)
	}

	return domain.PatchUserMeResponse{
		Message: snResp.Message,
		User: domain.PatchUserMeUpdated{
			ID:        sysidToUUID(snResp.User.ID),
			UpdatedBy: snResp.User.UpdatedBy,
			UpdatedOn: snResp.User.UpdatedOn,
		},
	}, nil
}
