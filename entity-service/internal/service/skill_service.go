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
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// SkillService is the skills module: the catalogue, each engineer's own skills
// profile, the manager's view of everybody's, and the admin's catalogue edits.
//
// Who may do what:
//   - any internal caller reads the catalogue (levels, active categories and
//     skills);
//   - an internal user with a user record reads and changes only their own
//     profile, which is always the caller's, and requests skills;
//   - every other task is a capability (domain.SkillCapability): the directory
//     (everybody's skills), reviewing skill requests, and managing the
//     catalogue (skills and categories). Each is held by a CSM admin and by the
//     roles configured for it (AUTH_SKILL_DIRECTORY_ROLES,
//     AUTH_SKILL_REQUEST_REVIEWER_ROLES, AUTH_SKILL_CATALOGUE_MANAGER_ROLES).
//     Code checks the capability, never a role name, so who does what is
//     decided by configuration.
//
// A CSM admin is a CSM Portal admin: a caller whose token's "roles" claim holds
// one of AUTH_ADMIN_ROLES, as for the rota access tab, so the two can never
// disagree about who an admin is.
type SkillService interface {
	ListLevelTypes(ctx context.Context) (domain.SkillLevelTypesResponse, error)
	ListCategories(ctx context.Context) (domain.SkillCategoriesResponse, error)
	SearchSkills(ctx context.Context, req domain.SearchSkillsRequest) (domain.SearchSkillsResponse, error)
	GetSkill(ctx context.Context, id string) (domain.Skill, error)

	GetMySkills(ctx context.Context) (domain.MySkillsResponse, error)
	AddMySkill(ctx context.Context, req domain.AddMySkillRequest) (domain.UserSkill, error)
	SetMySkillLevel(ctx context.Context, skillID string, req domain.SetMySkillLevelRequest) (domain.UserSkill, error)
	RemoveMySkill(ctx context.Context, skillID string) error
	GetMySkillHistory(ctx context.Context, req domain.SkillHistoryRequest) (domain.SkillHistoryResponse, error)

	SearchUserSkills(ctx context.Context, req domain.SearchUserSkillsRequest) (domain.SearchUserSkillsResponse, error)

	CreateSkill(ctx context.Context, req domain.CreateSkillRequest) (domain.Skill, error)
	UpdateSkill(ctx context.Context, id string, req domain.UpdateSkillRequest) (domain.Skill, error)

	SkillManagementService
}

// SkillCapabilityRoles is the configured role list of each capability, on top
// of the admin roles. An empty list means admins only.
type SkillCapabilityRoles map[domain.SkillCapability][]string

type skillService struct {
	repo       repository.SkillRepository
	access     AccessService
	adminRoles []string
	capRoles   SkillCapabilityRoles
}

// NewSkillService constructs a SkillService. adminRoles is AUTH_ADMIN_ROLES;
// capRoles adds roles per capability (nil: admins only).
func NewSkillService(repo repository.SkillRepository, access AccessService, adminRoles []string, capRoles SkillCapabilityRoles) SkillService {
	return &skillService{repo: repo, access: access, adminRoles: adminRoles, capRoles: capRoles}
}

const (
	// The catalogue is a few hundred entries at most, so its page is wide.
	skillDefaultLimit = 50
	skillMaxLimit     = 200
	// History is read a page at a time.
	skillHistoryDefaultLimit = 20
	skillHistoryMaxLimit     = 100

	skillNameMax        = 255
	skillKeywordsMax    = 500
	skillDescriptionMax = 4000
	// A filter list longer than this is a mistake, not a filter.
	skillFilterMax = 200
)

// skillPage applies the module's paging rules: a limit of zero or less is the
// default, one above max is refused, an offset below zero is zero.
func skillPage(p *domain.Pagination, def, max int) error {
	if p.Limit <= 0 {
		p.Limit = def
	}
	if p.Limit > max {
		return &apierror.ValidationError{Msg: fmt.Sprintf("limit cannot exceed %d", max)}
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return nil
}

// requireInternal lets only internal callers through, and says who is asking.
func (s *skillService) requireInternal(ctx context.Context) (auth.Identity, error) {
	if err := RequireInternalCaller(ctx, s.access, "skills are only available to internal staff"); err != nil {
		return auth.Identity{}, err
	}
	return auth.IdentityFromContext(ctx), nil
}

// requireMe resolves the caller's own platform user: internal, with a user
// token, and an active internal user record found by that token's email. The
// identity provider's own user id is never compared with a platform id.
func (s *skillService) requireMe(ctx context.Context) (userID, email string, err error) {
	id, err := s.requireInternal(ctx)
	if err != nil {
		return "", "", err
	}
	if id.UserEmail == "" {
		return "", "", &apierror.ForbiddenError{Msg: "a skills profile needs a user token, not a service credential"}
	}
	userID, ok, err := s.repo.ResolveUserID(ctx, id.UserEmail)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", &apierror.ForbiddenError{Msg: "no active internal user record for the caller"}
	}
	return userID, id.UserEmail, nil
}

// holds reports whether the identity holds a capability: an admin role, or a
// role configured for that capability. Roles are matched exactly.
func (s *skillService) holds(id auth.Identity, c domain.SkillCapability) bool {
	return holdsAnyRole(id.Roles, s.adminRoles) || holdsAnyRole(id.Roles, s.capRoles[c])
}

// capabilityRefusals is what a caller without the capability is told.
var capabilityRefusals = map[domain.SkillCapability]string{
	domain.SkillCapDirectory:       "you don't have access to the skill directory",
	domain.SkillCapReviewRequests:  "you don't have access to review skill requests",
	domain.SkillCapManageCatalogue: "you don't have access to change the skill catalogue",
}

// requireCapability is the gate for a capability, and returns the actor's
// email. A service credential holds none: every gated task needs a person.
func (s *skillService) requireCapability(ctx context.Context, c domain.SkillCapability) (string, error) {
	id, err := s.requireInternal(ctx)
	if err != nil {
		return "", err
	}
	if id.UserEmail == "" {
		return "", &apierror.ForbiddenError{Msg: "this needs a user token, not a service credential"}
	}
	if !s.holds(id, c) {
		return "", &apierror.ForbiddenError{Msg: capabilityRefusals[c]}
	}
	return id.UserEmail, nil
}

// requireManager is the gate for reading everybody's skills.
func (s *skillService) requireManager(ctx context.Context) error {
	_, err := s.requireCapability(ctx, domain.SkillCapDirectory)
	return err
}

// requireAdmin is the gate for catalogue edits, and returns the actor.
func (s *skillService) requireAdmin(ctx context.Context) (string, error) {
	return s.requireCapability(ctx, domain.SkillCapManageCatalogue)
}

// managesCatalogue is whether the caller (already known to be internal) may
// see what only catalogue managers see: inactive entries and holder counts.
func (s *skillService) managesCatalogue(ctx context.Context) bool {
	id := auth.IdentityFromContext(ctx)
	return id.UserEmail != "" && s.holds(id, domain.SkillCapManageCatalogue)
}

// ListLevelTypes implements SkillService.
func (s *skillService) ListLevelTypes(ctx context.Context) (domain.SkillLevelTypesResponse, error) {
	if _, err := s.requireInternal(ctx); err != nil {
		return domain.SkillLevelTypesResponse{}, err
	}
	types, err := s.repo.ListLevelTypes(ctx)
	if err != nil {
		return domain.SkillLevelTypesResponse{}, err
	}
	return domain.SkillLevelTypesResponse{LevelTypes: types}, nil
}

// ListCategories implements SkillService.
func (s *skillService) ListCategories(ctx context.Context) (domain.SkillCategoriesResponse, error) {
	if _, err := s.requireInternal(ctx); err != nil {
		return domain.SkillCategoriesResponse{}, err
	}
	cats, err := s.repo.ListCategories(ctx, s.managesCatalogue(ctx))
	if err != nil {
		return domain.SkillCategoriesResponse{}, err
	}
	return domain.SkillCategoriesResponse{Categories: cats}, nil
}

// SearchSkills implements SkillService.
func (s *skillService) SearchSkills(ctx context.Context, req domain.SearchSkillsRequest) (domain.SearchSkillsResponse, error) {
	if _, err := s.requireInternal(ctx); err != nil {
		return domain.SearchSkillsResponse{}, err
	}
	if err := skillPage(&req.Pagination, skillDefaultLimit, skillMaxLimit); err != nil {
		return domain.SearchSkillsResponse{}, err
	}
	if err := validateSkillIDList("categoryIds", req.CategoryIDs); err != nil {
		return domain.SearchSkillsResponse{}, err
	}
	if req.LevelTypeID != "" {
		if err := validateUUIDs("levelTypeId", []string{req.LevelTypeID}); err != nil {
			return domain.SearchSkillsResponse{}, err
		}
	}
	// Inactive skills and holder counts are a catalogue manager's view: the
	// flag is ignored for anyone else, so inactive entries never reach them.
	manager := s.managesCatalogue(ctx)
	req.IncludeInactive = req.IncludeInactive && manager
	req.WithHolderCounts = manager
	skills, total, err := s.repo.SearchSkills(ctx, req)
	if err != nil {
		return domain.SearchSkillsResponse{}, err
	}
	return domain.SearchSkillsResponse{Skills: skills, Total: total, Limit: req.Pagination.Limit, Offset: req.Pagination.Offset}, nil
}

// GetSkill implements SkillService.
func (s *skillService) GetSkill(ctx context.Context, id string) (domain.Skill, error) {
	if _, err := s.requireInternal(ctx); err != nil {
		return domain.Skill{}, err
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.Skill{}, err
	}
	skill, err := s.repo.GetSkill(ctx, id)
	if err != nil {
		return domain.Skill{}, err
	}
	if skill == nil {
		return domain.Skill{}, &apierror.NotFoundError{Msg: "skill not found"}
	}
	return *skill, nil
}

// GetMySkills implements SkillService.
func (s *skillService) GetMySkills(ctx context.Context) (domain.MySkillsResponse, error) {
	userID, _, err := s.requireMe(ctx)
	if err != nil {
		return domain.MySkillsResponse{}, err
	}
	skills, err := s.repo.ListUserSkills(ctx, userID)
	if err != nil {
		return domain.MySkillsResponse{}, err
	}
	return domain.MySkillsResponse{UserID: userID, Skills: skills}, nil
}

// checkLevelInScale is the rule the database cannot enforce: a person's level
// must be a rung of the scale of that skill. levelID is already a UUID.
func (s *skillService) checkLevelInScale(ctx context.Context, skill *domain.Skill, levelID string) error {
	typeID, ok, err := s.repo.LevelTypeOfLevel(ctx, levelID)
	if err != nil {
		return err
	}
	if !ok {
		return &apierror.ValidationError{Msg: "levelId is not a skill level"}
	}
	if skill.LevelTypeID == nil {
		return &apierror.ConflictError{Msg: "this skill has no level scale, so no level can be set on it"}
	}
	if typeID != *skill.LevelTypeID {
		return &apierror.ValidationError{Msg: "levelId is not a level of this skill's scale"}
	}
	return nil
}

// loadSkill reads the skill a profile write names.
func (s *skillService) loadSkill(ctx context.Context, skillID string) (*domain.Skill, error) {
	if err := validateUUIDs("skillId", []string{skillID}); err != nil {
		return nil, err
	}
	skill, err := s.repo.GetSkill(ctx, skillID)
	if err != nil {
		return nil, err
	}
	if skill == nil {
		return nil, &apierror.NotFoundError{Msg: "skill not found"}
	}
	return skill, nil
}

const inheritedRefusal = "this skill is inherited from a group and cannot be changed here"

// AddMySkill implements SkillService.
func (s *skillService) AddMySkill(ctx context.Context, req domain.AddMySkillRequest) (domain.UserSkill, error) {
	userID, actor, err := s.requireMe(ctx)
	if err != nil {
		return domain.UserSkill{}, err
	}
	if strings.TrimSpace(req.LevelID) == "" {
		return domain.UserSkill{}, &apierror.ValidationError{Msg: "levelId is required"}
	}
	if err := validateUUIDs("levelId", []string{req.LevelID}); err != nil {
		return domain.UserSkill{}, err
	}
	req.LevelID = strings.ToLower(req.LevelID)
	skill, err := s.loadSkill(ctx, req.SkillID)
	if err != nil {
		return domain.UserSkill{}, err
	}
	if !skill.Active {
		return domain.UserSkill{}, &apierror.ConflictError{Msg: inactiveSkillRefusal}
	}
	if err := s.checkLevelInScale(ctx, skill, req.LevelID); err != nil {
		return domain.UserSkill{}, err
	}
	res, err := s.repo.AddUserSkill(ctx, userID, skill.ID, req.LevelID, actor)
	if err != nil {
		return domain.UserSkill{}, err
	}
	switch res.Outcome {
	case domain.SkillOutcomeAlreadyHeld:
		return domain.UserSkill{}, &apierror.ConflictError{Msg: "this skill is already on your profile: change its level instead"}
	case domain.SkillOutcomeInherited:
		return domain.UserSkill{}, &apierror.ConflictError{Msg: inheritedRefusal}
	case domain.SkillOutcomeSkillInactive:
		return domain.UserSkill{}, &apierror.ConflictError{Msg: inactiveSkillRefusal}
	case domain.SkillOutcomeLevelNotInScale, domain.SkillOutcomeNotFound:
		return domain.UserSkill{}, levelScaleRaceError(res.Outcome)
	}
	return rowOrError(res)
}

// SetMySkillLevel implements SkillService. Setting the level already held
// changes nothing and writes no history.
func (s *skillService) SetMySkillLevel(ctx context.Context, skillID string, req domain.SetMySkillLevelRequest) (domain.UserSkill, error) {
	userID, actor, err := s.requireMe(ctx)
	if err != nil {
		return domain.UserSkill{}, err
	}
	if strings.TrimSpace(req.LevelID) == "" {
		return domain.UserSkill{}, &apierror.ValidationError{Msg: "levelId is required"}
	}
	if err := validateUUIDs("levelId", []string{req.LevelID}); err != nil {
		return domain.UserSkill{}, err
	}
	req.LevelID = strings.ToLower(req.LevelID)
	skill, err := s.loadSkill(ctx, skillID)
	if err != nil {
		return domain.UserSkill{}, err
	}
	if err := s.checkLevelInScale(ctx, skill, req.LevelID); err != nil {
		return domain.UserSkill{}, err
	}
	res, err := s.repo.SetUserSkillLevel(ctx, userID, skill.ID, req.LevelID, actor)
	if err != nil {
		return domain.UserSkill{}, err
	}
	switch res.Outcome {
	case domain.SkillOutcomeNotHeld:
		return domain.UserSkill{}, &apierror.NotFoundError{Msg: "this skill is not on your profile"}
	case domain.SkillOutcomeInherited:
		return domain.UserSkill{}, &apierror.ConflictError{Msg: inheritedRefusal}
	case domain.SkillOutcomeLevelNotInScale, domain.SkillOutcomeNotFound:
		return domain.UserSkill{}, levelScaleRaceError(res.Outcome)
	}
	return rowOrError(res)
}

// inactiveSkillRefusal is the 409 for adding a skill that is not active, whether
// the service saw it or the write's own transaction did.
const inactiveSkillRefusal = "this skill is inactive and cannot be added to a profile"

// levelScaleRaceError answers the refusals only the write's own transaction can
// find, because the skill changed after the request was checked: its scale was
// swapped (409: the request was valid, the state it assumed is gone, and asking
// again with a level of the new scale can succeed), or the skill is gone (404).
func levelScaleRaceError(o domain.SkillOutcome) error {
	if o == domain.SkillOutcomeNotFound {
		return &apierror.NotFoundError{Msg: "skill not found"}
	}
	return &apierror.ConflictError{Msg: "this skill's level scale changed while the request was running: choose a level of its current scale"}
}

// rowOrError is the profile row a successful write returns.
func rowOrError(res domain.SkillWriteResult) (domain.UserSkill, error) {
	if res.Row == nil {
		return domain.UserSkill{}, fmt.Errorf("skill profile write left no row to return")
	}
	return *res.Row, nil
}

// RemoveMySkill implements SkillService.
func (s *skillService) RemoveMySkill(ctx context.Context, skillID string) error {
	userID, actor, err := s.requireMe(ctx)
	if err != nil {
		return err
	}
	if err := validateUUIDs("skillId", []string{skillID}); err != nil {
		return err
	}
	skillID = strings.ToLower(skillID)
	res, err := s.repo.RemoveUserSkill(ctx, userID, skillID, actor)
	if err != nil {
		return err
	}
	switch res.Outcome {
	case domain.SkillOutcomeNotHeld:
		return &apierror.NotFoundError{Msg: "this skill is not on your profile"}
	case domain.SkillOutcomeInherited:
		return &apierror.ConflictError{Msg: inheritedRefusal}
	}
	return nil
}

// GetMySkillHistory implements SkillService.
func (s *skillService) GetMySkillHistory(ctx context.Context, req domain.SkillHistoryRequest) (domain.SkillHistoryResponse, error) {
	userID, _, err := s.requireMe(ctx)
	if err != nil {
		return domain.SkillHistoryResponse{}, err
	}
	if req.SkillID != "" {
		if err := validateUUIDs("skillId", []string{req.SkillID}); err != nil {
			return domain.SkillHistoryResponse{}, err
		}
	}
	if err := skillPage(&req.Pagination, skillHistoryDefaultLimit, skillHistoryMaxLimit); err != nil {
		return domain.SkillHistoryResponse{}, err
	}
	rows, total, err := s.repo.ListUserSkillHistory(ctx, userID, req.SkillID, req.Pagination.Limit, req.Pagination.Offset)
	if err != nil {
		return domain.SkillHistoryResponse{}, err
	}
	return domain.SkillHistoryResponse{History: rows, Total: total, Limit: req.Pagination.Limit, Offset: req.Pagination.Offset}, nil
}

// validateSkillIDList checks a filter list of UUIDs.
func validateSkillIDList(field string, ids []string) error {
	if len(ids) > skillFilterMax {
		return &apierror.ValidationError{Msg: fmt.Sprintf("%s cannot hold more than %d ids", field, skillFilterMax)}
	}
	return validateUUIDs(field, ids)
}

// SearchUserSkills implements SkillService.
func (s *skillService) SearchUserSkills(ctx context.Context, req domain.SearchUserSkillsRequest) (domain.SearchUserSkillsResponse, error) {
	if err := s.requireManager(ctx); err != nil {
		return domain.SearchUserSkillsResponse{}, err
	}
	if err := skillPage(&req.Pagination, skillDefaultLimit, skillMaxLimit); err != nil {
		return domain.SearchUserSkillsResponse{}, err
	}
	for field, ids := range map[string][]string{"userIds": req.UserIDs, "skillIds": req.SkillIDs, "levelIds": req.LevelIDs} {
		if err := validateSkillIDList(field, ids); err != nil {
			return domain.SearchUserSkillsResponse{}, err
		}
	}
	if req.MinLevelValue != nil && *req.MinLevelValue < 0 {
		return domain.SearchUserSkillsResponse{}, &apierror.ValidationError{Msg: "minLevelValue cannot be negative"}
	}
	if len(req.TeamKeys) > skillFilterMax {
		return domain.SearchUserSkillsResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("teamKeys cannot hold more than %d keys", skillFilterMax)}
	}
	// Team keys are stored lower case, and a trailing blank is not a team.
	var keys []string
	for _, k := range req.TeamKeys {
		if k = strings.ToLower(strings.TrimSpace(k)); k != "" {
			keys = append(keys, k)
		}
	}
	req.TeamKeys = keys
	items, total, err := s.repo.SearchUserSkills(ctx, req)
	if err != nil {
		return domain.SearchUserSkillsResponse{}, err
	}
	return domain.SearchUserSkillsResponse{Items: items, Total: total, Limit: req.Pagination.Limit, Offset: req.Pagination.Offset}, nil
}

// cleanSkillText trims an optional text field and enforces its length. A nil
// stays nil (not given); an empty result is kept as "" (clear it).
func cleanSkillText(field string, in *string, max int) (*string, error) {
	if in == nil {
		return nil, nil
	}
	v := strings.TrimSpace(*in)
	if len([]rune(v)) > max {
		return nil, &apierror.ValidationError{Msg: fmt.Sprintf("%s cannot be longer than %d characters", field, max)}
	}
	return &v, nil
}

// cleanSkillName trims a name and requires it.
func cleanSkillName(in string) (string, error) {
	v := strings.TrimSpace(in)
	if v == "" {
		return "", &apierror.ValidationError{Msg: "name is required"}
	}
	if len([]rune(v)) > skillNameMax {
		return "", &apierror.ValidationError{Msg: fmt.Sprintf("name cannot be longer than %d characters", skillNameMax)}
	}
	return v, nil
}

// cleanCategoryIDs validates the ids, drops repeats, and checks they exist. A
// category the skill is newly filed under must be an active leaf; one it is
// already filed under (keep) may stay, so editing a skill filed under a parent
// or a since-deactivated category (migrated data) is not refused.
func (s *skillService) cleanCategoryIDs(ctx context.Context, ids []string, keep []domain.SkillCategoryRef) ([]string, error) {
	if err := validateSkillIDList("categoryIds", ids); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []string{}
	for _, id := range ids {
		id = strings.ToLower(id)
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	ok, err := s.repo.CategoriesExist(ctx, out)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, &apierror.ValidationError{Msg: "categoryIds names a category that does not exist"}
	}
	kept := map[string]bool{}
	for _, c := range keep {
		kept[strings.ToLower(c.ID)] = true
	}
	var added []string
	for _, id := range out {
		if !kept[id] {
			added = append(added, id)
		}
	}
	if ok, err = s.repo.CategoriesAreActiveLeaves(ctx, added); err != nil {
		return nil, err
	}
	if !ok {
		return nil, &apierror.ValidationError{Msg: "a skill is filed only under an active category with no sub-categories"}
	}
	return out, nil
}

func (s *skillService) requireLevelType(ctx context.Context, id string) error {
	if err := validateUUIDs("levelTypeId", []string{id}); err != nil {
		return err
	}
	ok, err := s.repo.LevelTypeExists(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return &apierror.ValidationError{Msg: "levelTypeId is not a skill level scale"}
	}
	return nil
}

// skillWriteError turns a catalogue write's outcome into the caller's refusal.
func skillWriteError(res domain.SkillWriteResult) error {
	switch res.Outcome {
	case domain.SkillOutcomeNotFound:
		return &apierror.NotFoundError{Msg: "skill not found"}
	case domain.SkillOutcomeNameTaken:
		return &apierror.ConflictError{Msg: "another skill already has this name"}
	case domain.SkillOutcomeLevelsInUse:
		return &apierror.ConflictError{Msg: "people hold levels of this skill, so its level scale cannot be changed"}
	}
	if res.Outcome != domain.SkillOutcomeDone {
		return fmt.Errorf("unexpected skill write outcome %d", res.Outcome)
	}
	if res.Skill == nil {
		return fmt.Errorf("skill write left no skill to return")
	}
	return nil
}

// CreateSkill implements SkillService.
func (s *skillService) CreateSkill(ctx context.Context, req domain.CreateSkillRequest) (domain.Skill, error) {
	actor, err := s.requireAdmin(ctx)
	if err != nil {
		return domain.Skill{}, err
	}
	if req.Name, err = cleanSkillName(req.Name); err != nil {
		return domain.Skill{}, err
	}
	if req.Description, err = cleanSkillText("description", req.Description, skillDescriptionMax); err != nil {
		return domain.Skill{}, err
	}
	if req.Keywords, err = cleanSkillText("keywords", req.Keywords, skillKeywordsMax); err != nil {
		return domain.Skill{}, err
	}
	if strings.TrimSpace(req.LevelTypeID) == "" {
		return domain.Skill{}, &apierror.ValidationError{Msg: "levelTypeId is required"}
	}
	if err := s.requireLevelType(ctx, req.LevelTypeID); err != nil {
		return domain.Skill{}, err
	}
	req.LevelTypeID = strings.ToLower(req.LevelTypeID)
	if req.CategoryIDs, err = s.cleanCategoryIDs(ctx, req.CategoryIDs, nil); err != nil {
		return domain.Skill{}, err
	}
	res, err := s.repo.CreateSkill(ctx, req, actor)
	if err != nil {
		return domain.Skill{}, err
	}
	if err := skillWriteError(res); err != nil {
		return domain.Skill{}, err
	}
	return *res.Skill, nil
}

// UpdateSkill implements SkillService. Deactivating a skill (active=false)
// keeps it in the catalogue and on the profiles that hold it; it only stops it
// being added to more.
func (s *skillService) UpdateSkill(ctx context.Context, id string, req domain.UpdateSkillRequest) (domain.Skill, error) {
	actor, err := s.requireAdmin(ctx)
	if err != nil {
		return domain.Skill{}, err
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.Skill{}, err
	}
	id = strings.ToLower(id)
	if req.Name == nil && req.Description == nil && req.Keywords == nil && req.Active == nil &&
		req.LevelTypeID == nil && req.CategoryIDs == nil {
		return domain.Skill{}, &apierror.ValidationError{Msg: "give at least one field to change"}
	}
	if req.Name != nil {
		name, err := cleanSkillName(*req.Name)
		if err != nil {
			return domain.Skill{}, err
		}
		req.Name = &name
	}
	if req.Description, err = cleanSkillText("description", req.Description, skillDescriptionMax); err != nil {
		return domain.Skill{}, err
	}
	if req.Keywords, err = cleanSkillText("keywords", req.Keywords, skillKeywordsMax); err != nil {
		return domain.Skill{}, err
	}
	if req.LevelTypeID != nil {
		if err := s.requireLevelType(ctx, *req.LevelTypeID); err != nil {
			return domain.Skill{}, err
		}
		lt := strings.ToLower(*req.LevelTypeID)
		req.LevelTypeID = &lt
	}
	if req.CategoryIDs != nil {
		current, err := s.repo.GetSkill(ctx, id)
		if err != nil {
			return domain.Skill{}, err
		}
		var keep []domain.SkillCategoryRef
		if current != nil {
			keep = current.Categories
		}
		ids, err := s.cleanCategoryIDs(ctx, *req.CategoryIDs, keep)
		if err != nil {
			return domain.Skill{}, err
		}
		req.CategoryIDs = &ids
	}
	res, err := s.repo.UpdateSkill(ctx, id, req, actor)
	if err != nil {
		return domain.Skill{}, err
	}
	if err := skillWriteError(res); err != nil {
		return domain.Skill{}, err
	}
	return *res.Skill, nil
}
