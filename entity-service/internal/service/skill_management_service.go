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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SkillManagementService is the skill management revamp: capabilities, the
// category admin, skill requests and the people directory.
type SkillManagementService interface {
	GetMyCapabilities(ctx context.Context) (domain.SkillCapabilities, error)

	CreateCategory(ctx context.Context, req domain.CreateSkillCategoryRequest) (domain.SkillCategory, error)
	UpdateCategory(ctx context.Context, id string, req domain.UpdateSkillCategoryRequest) (domain.SkillCategory, error)

	CheckSkillRequest(ctx context.Context, req domain.CheckSkillRequestRequest) (domain.CheckSkillRequestResponse, error)
	CreateSkillRequest(ctx context.Context, req domain.CreateSkillRequestRequest) (domain.SkillRequest, error)
	ListMySkillRequests(ctx context.Context) (domain.SkillRequestsResponse, error)
	WithdrawSkillRequest(ctx context.Context, id string) (domain.SkillRequest, error)
	SearchSkillRequests(ctx context.Context, req domain.SearchSkillRequestsRequest) (domain.SkillRequestsResponse, error)
	ApproveSkillRequest(ctx context.Context, id string, req domain.ApproveSkillRequestRequest) (domain.SkillRequest, error)
	RejectSkillRequest(ctx context.Context, id string, req domain.RejectSkillRequestRequest) (domain.SkillRequest, error)

	SearchSkillDirectory(ctx context.Context, req domain.SearchSkillDirectoryRequest) (domain.SearchSkillDirectoryResponse, error)
	GetUserSkillProfile(ctx context.Context, userID string) (domain.UserSkillProfileResponse, error)
	GetUserSkillHistory(ctx context.Context, userID string, req domain.SkillHistoryRequest) (domain.SkillHistoryResponse, error)
}

const (
	skillCategoryDescriptionMax = 2000
	skillRequestReasonMax       = 2000
	// A directory search with more criteria than this is a mistake.
	skillCriteriaMax = 20
	// "My requests" is short: the newest this many.
	mySkillRequestsLimit = 50
	// The directory lists each person with their skills, so its page is narrow.
	skillDirectoryDefaultLimit = 25
	skillDirectoryMaxLimit     = 100
)

// normalizeSkillName is what "the same name" means for requests: lower case,
// trimmed, inner whitespace collapsed.
func normalizeSkillName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}

// GetMyCapabilities implements SkillManagementService. Any internal caller may
// ask; a service credential holds none.
func (s *skillService) GetMyCapabilities(ctx context.Context) (domain.SkillCapabilities, error) {
	id, err := s.requireInternal(ctx)
	if err != nil {
		return domain.SkillCapabilities{}, err
	}
	if id.UserEmail == "" {
		return domain.SkillCapabilities{}, nil
	}
	caps := domain.SkillCapabilities{
		CanViewDirectory:   s.holds(id, domain.SkillCapDirectory),
		CanReviewRequests:  s.holds(id, domain.SkillCapReviewRequests),
		CanManageCatalogue: s.holds(id, domain.SkillCapManageCatalogue),
	}
	if caps.CanReviewRequests {
		n, err := s.repo.CountPendingSkillRequests(ctx)
		if err != nil {
			return domain.SkillCapabilities{}, err
		}
		caps.PendingRequestCount = &n
	}
	return caps, nil
}

// --- categories ---

func categoryWriteError(outcome domain.SkillOutcome) error {
	switch outcome {
	case domain.SkillOutcomeCategoryNotFound:
		return &apierror.NotFoundError{Msg: "category not found"}
	case domain.SkillOutcomeNameTaken:
		return &apierror.ConflictError{Msg: "another category under the same parent already has this name"}
	case domain.SkillOutcomeCategoryInvalid:
		return &apierror.ConflictError{Msg: "a sub-category goes under an active category that has no skills filed directly under it, and an active category can't sit under an inactive one"}
	case domain.SkillOutcomeCategoryInUse:
		return &apierror.ConflictError{Msg: "this category still has active sub-categories or skills: move or deactivate them first"}
	}
	return fmt.Errorf("unexpected category write outcome %d", outcome)
}

// CreateCategory implements SkillManagementService.
func (s *skillService) CreateCategory(ctx context.Context, req domain.CreateSkillCategoryRequest) (domain.SkillCategory, error) {
	actor, err := s.requireCapability(ctx, domain.SkillCapManageCatalogue)
	if err != nil {
		return domain.SkillCategory{}, err
	}
	if req.Name, err = cleanSkillName(req.Name); err != nil {
		return domain.SkillCategory{}, err
	}
	if strings.Contains(req.Name, strings.TrimSpace(categoryPathSeparator)) {
		return domain.SkillCategory{}, &apierror.ValidationError{Msg: "a category name cannot contain \"/\""}
	}
	if req.Description, err = cleanSkillText("description", req.Description, skillCategoryDescriptionMax); err != nil {
		return domain.SkillCategory{}, err
	}
	if req.ParentID != nil {
		if err := validateUUIDs("parentId", []string{*req.ParentID}); err != nil {
			return domain.SkillCategory{}, err
		}
		p := strings.ToLower(*req.ParentID)
		req.ParentID = &p
	}
	cat, outcome, err := s.repo.CreateCategory(ctx, req, actor)
	if err != nil {
		return domain.SkillCategory{}, err
	}
	if outcome != domain.SkillOutcomeDone {
		return domain.SkillCategory{}, categoryWriteError(outcome)
	}
	return *cat, nil
}

// categoryPathSeparator matches the repository's stored path separator.
const categoryPathSeparator = " / "

// UpdateCategory implements SkillManagementService.
func (s *skillService) UpdateCategory(ctx context.Context, id string, req domain.UpdateSkillCategoryRequest) (domain.SkillCategory, error) {
	actor, err := s.requireCapability(ctx, domain.SkillCapManageCatalogue)
	if err != nil {
		return domain.SkillCategory{}, err
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.SkillCategory{}, err
	}
	if req.Name == nil && req.Description == nil && req.Active == nil {
		return domain.SkillCategory{}, &apierror.ValidationError{Msg: "give at least one field to change"}
	}
	if req.Name != nil {
		name, err := cleanSkillName(*req.Name)
		if err != nil {
			return domain.SkillCategory{}, err
		}
		if strings.Contains(name, strings.TrimSpace(categoryPathSeparator)) {
			return domain.SkillCategory{}, &apierror.ValidationError{Msg: "a category name cannot contain \"/\""}
		}
		req.Name = &name
	}
	if req.Description, err = cleanSkillText("description", req.Description, skillCategoryDescriptionMax); err != nil {
		return domain.SkillCategory{}, err
	}
	cat, outcome, err := s.repo.UpdateCategory(ctx, strings.ToLower(id), req, actor)
	if err != nil {
		return domain.SkillCategory{}, err
	}
	if outcome != domain.SkillOutcomeDone {
		return domain.SkillCategory{}, categoryWriteError(outcome)
	}
	return *cat, nil
}

// --- skill requests ---

// CheckSkillRequest implements SkillManagementService: any internal user (the
// requester while typing, and a reviewer judging a request).
func (s *skillService) CheckSkillRequest(ctx context.Context, req domain.CheckSkillRequestRequest) (domain.CheckSkillRequestResponse, error) {
	if _, err := s.requireInternal(ctx); err != nil {
		return domain.CheckSkillRequestResponse{}, err
	}
	norm := normalizeSkillName(req.Name)
	if len([]rune(norm)) > skillNameMax {
		return domain.CheckSkillRequestResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("name cannot be longer than %d characters", skillNameMax)}
	}
	resp, err := s.repo.CheckSkillRequest(ctx, norm)
	if err != nil {
		return domain.CheckSkillRequestResponse{}, err
	}
	// Only a catalogue manager is told about inactive skills by name.
	if !s.managesCatalogue(ctx) {
		var similar []domain.SkillRef
		for _, sk := range resp.Similar {
			if sk.Active {
				similar = append(similar, sk)
			}
		}
		if similar == nil {
			similar = []domain.SkillRef{}
		}
		resp.Similar = similar
	}
	return resp, nil
}

// CreateSkillRequest implements SkillManagementService.
func (s *skillService) CreateSkillRequest(ctx context.Context, req domain.CreateSkillRequestRequest) (domain.SkillRequest, error) {
	userID, actor, err := s.requireMe(ctx)
	if err != nil {
		return domain.SkillRequest{}, err
	}
	if req.Name, err = cleanSkillName(req.Name); err != nil {
		return domain.SkillRequest{}, err
	}
	if req.Description, err = cleanSkillText("description", req.Description, skillDescriptionMax); err != nil {
		return domain.SkillRequest{}, err
	}
	if req.SuggestedCategory, err = cleanSkillText("suggestedCategory", req.SuggestedCategory, skillNameMax); err != nil {
		return domain.SkillRequest{}, err
	}
	if req.CategoryID != nil && strings.TrimSpace(*req.CategoryID) == "" {
		req.CategoryID = nil
	}
	if req.CategoryID != nil {
		if err := validateUUIDs("categoryId", []string{*req.CategoryID}); err != nil {
			return domain.SkillRequest{}, err
		}
		c := strings.ToLower(*req.CategoryID)
		req.CategoryID = &c
		ok, err := s.repo.CategoriesAreActiveLeaves(ctx, []string{c})
		if err != nil {
			return domain.SkillRequest{}, err
		}
		if !ok {
			return domain.SkillRequest{}, &apierror.ValidationError{Msg: "categoryId is not an active category without sub-categories"}
		}
	}
	if strings.TrimSpace(req.LevelID) == "" {
		return domain.SkillRequest{}, &apierror.ValidationError{Msg: "levelId is required: it is the level the skill is added at when the request is approved"}
	}
	if err := validateUUIDs("levelId", []string{req.LevelID}); err != nil {
		return domain.SkillRequest{}, err
	}
	req.LevelID = strings.ToLower(req.LevelID)
	scale, ok, err := s.repo.DefaultLevelTypeID(ctx)
	if err != nil {
		return domain.SkillRequest{}, err
	}
	levelScale, found, err := s.repo.LevelTypeOfLevel(ctx, req.LevelID)
	if err != nil {
		return domain.SkillRequest{}, err
	}
	if !found || (ok && levelScale != scale) {
		return domain.SkillRequest{}, &apierror.ValidationError{Msg: "levelId is not a level of the default scale"}
	}
	out, outcome, err := s.repo.CreateSkillRequest(ctx, userID, normalizeSkillName(req.Name), req, actor)
	if err != nil {
		return domain.SkillRequest{}, err
	}
	switch outcome {
	case domain.SkillOutcomeDone:
		return *out, nil
	case domain.SkillOutcomeSkillExists:
		return domain.SkillRequest{}, &apierror.ConflictError{Msg: "this skill is already in the catalogue: add it to your skills instead"}
	case domain.SkillOutcomePendingExists:
		if out != nil && out.RequestedBy.ID == userID {
			return domain.SkillRequest{}, &apierror.ConflictError{Msg: "you have already requested this skill: it is waiting for review"}
		}
		return domain.SkillRequest{}, &apierror.ConflictError{Msg: "someone has already requested this skill: it is waiting for review"}
	}
	return domain.SkillRequest{}, fmt.Errorf("unexpected skill request outcome %d", outcome)
}

// ListMySkillRequests implements SkillManagementService.
func (s *skillService) ListMySkillRequests(ctx context.Context) (domain.SkillRequestsResponse, error) {
	userID, _, err := s.requireMe(ctx)
	if err != nil {
		return domain.SkillRequestsResponse{}, err
	}
	rows, total, err := s.repo.ListSkillRequests(ctx, userID, nil, mySkillRequestsLimit, 0)
	if err != nil {
		return domain.SkillRequestsResponse{}, err
	}
	return domain.SkillRequestsResponse{Requests: rows, Total: total, Limit: mySkillRequestsLimit}, nil
}

// requestDecisionError turns a decision's outcome into the caller's refusal.
func requestDecisionError(outcome domain.SkillOutcome, req *domain.SkillRequest) error {
	switch outcome {
	case domain.SkillOutcomeNotFound:
		return &apierror.NotFoundError{Msg: "skill request not found"}
	case domain.SkillOutcomeRequestNotPending:
		msg := "this request is no longer pending"
		if req != nil {
			msg = fmt.Sprintf("this request is already %s", strings.ToLower(string(req.Status)))
			if req.ReviewedBy != nil {
				who := req.ReviewedBy.ID
				if req.ReviewedBy.Name != nil && *req.ReviewedBy.Name != "" {
					who = *req.ReviewedBy.Name
				} else if req.ReviewedBy.Email != nil {
					who = *req.ReviewedBy.Email
				}
				msg += " by " + who
			}
		}
		return &apierror.ConflictError{Msg: msg}
	case domain.SkillOutcomeNameTaken:
		return &apierror.ConflictError{Msg: "another skill already has this name: approve it as an existing skill instead"}
	case domain.SkillOutcomeNoDefaultScale:
		return &apierror.ConflictError{Msg: "no level scale is marked as the default, so a new skill can't be created from a request"}
	case domain.SkillOutcomeCategoryInvalid:
		return &apierror.ValidationError{Msg: "categoryId is not an active category without sub-categories"}
	case domain.SkillOutcomeSkillNotInactive:
		return &apierror.ConflictError{Msg: "this skill is already active: approve it as an existing skill instead"}
	case domain.SkillOutcomeSkillNotActive:
		return &apierror.ConflictError{Msg: "this skill is inactive: reactivate it instead"}
	case domain.SkillOutcomeLevelNotInScale:
		return &apierror.ValidationError{Msg: "levelId is not a level of the approved skill's scale"}
	}
	return fmt.Errorf("unexpected skill request outcome %d", outcome)
}

// WithdrawSkillRequest implements SkillManagementService: the requester's own
// pending request only.
func (s *skillService) WithdrawSkillRequest(ctx context.Context, id string) (domain.SkillRequest, error) {
	userID, actor, err := s.requireMe(ctx)
	if err != nil {
		return domain.SkillRequest{}, err
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.SkillRequest{}, err
	}
	out, outcome, err := s.repo.WithdrawSkillRequest(ctx, strings.ToLower(id), userID, actor)
	if err != nil {
		return domain.SkillRequest{}, err
	}
	if outcome != domain.SkillOutcomeDone {
		return domain.SkillRequest{}, requestDecisionError(outcome, out)
	}
	return *out, nil
}

// SearchSkillRequests implements SkillManagementService.
func (s *skillService) SearchSkillRequests(ctx context.Context, req domain.SearchSkillRequestsRequest) (domain.SkillRequestsResponse, error) {
	if _, err := s.requireCapability(ctx, domain.SkillCapReviewRequests); err != nil {
		return domain.SkillRequestsResponse{}, err
	}
	if err := skillPage(&req.Pagination, skillDefaultLimit, skillMaxLimit); err != nil {
		return domain.SkillRequestsResponse{}, err
	}
	for _, st := range req.Statuses {
		switch st {
		case domain.SkillRequestPending, domain.SkillRequestApproved, domain.SkillRequestRejected, domain.SkillRequestWithdrawn:
		default:
			return domain.SkillRequestsResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("unknown status %q", st)}
		}
	}
	rows, total, err := s.repo.ListSkillRequests(ctx, "", req.Statuses, req.Pagination.Limit, req.Pagination.Offset)
	if err != nil {
		return domain.SkillRequestsResponse{}, err
	}
	return domain.SkillRequestsResponse{Requests: rows, Total: total, Limit: req.Pagination.Limit, Offset: req.Pagination.Offset}, nil
}

// reviewer is the reviewing caller's email and platform user id.
func (s *skillService) reviewer(ctx context.Context) (string, string, error) {
	actor, err := s.requireCapability(ctx, domain.SkillCapReviewRequests)
	if err != nil {
		return "", "", err
	}
	userID, ok, err := s.repo.ResolveUserID(ctx, actor)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", &apierror.ForbiddenError{Msg: "no active internal user record for the caller"}
	}
	return actor, userID, nil
}

// ApproveSkillRequest implements SkillManagementService.
func (s *skillService) ApproveSkillRequest(ctx context.Context, id string, req domain.ApproveSkillRequestRequest) (domain.SkillRequest, error) {
	actor, reviewerID, err := s.reviewer(ctx)
	if err != nil {
		return domain.SkillRequest{}, err
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.SkillRequest{}, err
	}
	id = strings.ToLower(id)
	current, err := s.repo.GetSkillRequest(ctx, id)
	if err != nil {
		return domain.SkillRequest{}, err
	}
	if current == nil {
		return domain.SkillRequest{}, &apierror.NotFoundError{Msg: "skill request not found"}
	}
	a := domain.SkillApproval{RequestID: id, Mode: req.Mode, ReviewerID: reviewerID, Actor: actor}
	if a.Comment, err = cleanSkillText("comment", req.Comment, skillRequestReasonMax); err != nil {
		return domain.SkillRequest{}, err
	}
	if req.LevelID != nil && strings.TrimSpace(*req.LevelID) != "" {
		if err := validateUUIDs("levelId", []string{*req.LevelID}); err != nil {
			return domain.SkillRequest{}, err
		}
		l := strings.ToLower(*req.LevelID)
		a.LevelID = &l
	}
	if req.CategoryID != nil && strings.TrimSpace(*req.CategoryID) != "" {
		if err := validateUUIDs("categoryId", []string{*req.CategoryID}); err != nil {
			return domain.SkillRequest{}, err
		}
		c := strings.ToLower(*req.CategoryID)
		a.CategoryID = &c
	}
	switch req.Mode {
	case domain.SkillApproveCreate:
		name := current.RequestedName
		if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
			name = *req.Name
		}
		if a.Name, err = cleanSkillName(name); err != nil {
			return domain.SkillRequest{}, err
		}
		if a.CategoryID == nil && current.Category != nil {
			a.CategoryID = &current.Category.ID
		}
		if a.CategoryID == nil {
			return domain.SkillRequest{}, &apierror.ValidationError{Msg: "choose a category for the new skill"}
		}
		desc := req.Description
		if desc == nil {
			desc = current.Description
		}
		if a.Description, err = cleanSkillText("description", desc, skillDescriptionMax); err != nil {
			return domain.SkillRequest{}, err
		}
		if a.Keywords, err = cleanSkillText("keywords", req.Keywords, skillKeywordsMax); err != nil {
			return domain.SkillRequest{}, err
		}
	case domain.SkillApproveReactivate, domain.SkillApproveExisting:
		if req.SkillID == nil || strings.TrimSpace(*req.SkillID) == "" {
			return domain.SkillRequest{}, &apierror.ValidationError{Msg: "skillId is required for mode " + req.Mode}
		}
		if err := validateUUIDs("skillId", []string{*req.SkillID}); err != nil {
			return domain.SkillRequest{}, err
		}
		a.SkillID = strings.ToLower(*req.SkillID)
		if req.Mode == domain.SkillApproveExisting {
			a.CategoryID = nil
		}
	default:
		return domain.SkillRequest{}, &apierror.ValidationError{Msg: `mode must be "create", "reactivate" or "existing"`}
	}
	out, outcome, err := s.repo.ApproveSkillRequest(ctx, a)
	if err != nil {
		return domain.SkillRequest{}, err
	}
	if outcome != domain.SkillOutcomeDone {
		return domain.SkillRequest{}, requestDecisionError(outcome, out)
	}
	return *out, nil
}

// RejectSkillRequest implements SkillManagementService. A reason is required
// and shown to the requester.
func (s *skillService) RejectSkillRequest(ctx context.Context, id string, req domain.RejectSkillRequestRequest) (domain.SkillRequest, error) {
	actor, reviewerID, err := s.reviewer(ctx)
	if err != nil {
		return domain.SkillRequest{}, err
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.SkillRequest{}, err
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return domain.SkillRequest{}, &apierror.ValidationError{Msg: "a reason is required: the requester sees it"}
	}
	if len([]rune(reason)) > skillRequestReasonMax {
		return domain.SkillRequest{}, &apierror.ValidationError{Msg: fmt.Sprintf("reason cannot be longer than %d characters", skillRequestReasonMax)}
	}
	out, outcome, err := s.repo.RejectSkillRequest(ctx, strings.ToLower(id), reviewerID, reason, actor)
	if err != nil {
		return domain.SkillRequest{}, err
	}
	if outcome != domain.SkillOutcomeDone {
		return domain.SkillRequest{}, requestDecisionError(outcome, out)
	}
	return *out, nil
}

// --- directory ---

// SearchSkillDirectory implements SkillManagementService.
func (s *skillService) SearchSkillDirectory(ctx context.Context, req domain.SearchSkillDirectoryRequest) (domain.SearchSkillDirectoryResponse, error) {
	if _, err := s.requireCapability(ctx, domain.SkillCapDirectory); err != nil {
		return domain.SearchSkillDirectoryResponse{}, err
	}
	if err := skillPage(&req.Pagination, skillDirectoryDefaultLimit, skillDirectoryMaxLimit); err != nil {
		return domain.SearchSkillDirectoryResponse{}, err
	}
	switch strings.ToUpper(strings.TrimSpace(req.Match)) {
	case "", "ALL":
		req.Match = "ALL"
	case "ANY":
		req.Match = "ANY"
	default:
		return domain.SearchSkillDirectoryResponse{}, &apierror.ValidationError{Msg: `match must be "ALL" or "ANY"`}
	}
	if len(req.Criteria) > skillCriteriaMax {
		return domain.SearchSkillDirectoryResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("criteria cannot hold more than %d skills", skillCriteriaMax)}
	}
	// One criterion per skill: a repeat keeps the stricter minimum.
	index := map[string]int{}
	var crit []domain.SkillCriterion
	for _, c := range req.Criteria {
		if err := validateUUIDs("criteria.skillId", []string{c.SkillID}); err != nil {
			return domain.SearchSkillDirectoryResponse{}, err
		}
		if c.MinLevelValue < 0 {
			return domain.SearchSkillDirectoryResponse{}, &apierror.ValidationError{Msg: "minLevelValue cannot be negative"}
		}
		c.SkillID = strings.ToLower(c.SkillID)
		if i, ok := index[c.SkillID]; ok {
			if c.MinLevelValue > crit[i].MinLevelValue {
				crit[i].MinLevelValue = c.MinLevelValue
			}
			continue
		}
		index[c.SkillID] = len(crit)
		crit = append(crit, c)
	}
	req.Criteria = crit
	if err := validateSkillIDList("categoryIds", req.CategoryIDs); err != nil {
		return domain.SearchSkillDirectoryResponse{}, err
	}
	if len(req.TeamKeys) > skillFilterMax {
		return domain.SearchSkillDirectoryResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("teamKeys cannot hold more than %d keys", skillFilterMax)}
	}
	var keys []string
	for _, k := range req.TeamKeys {
		if k = strings.ToLower(strings.TrimSpace(k)); k != "" {
			keys = append(keys, k)
		}
	}
	req.TeamKeys = keys
	items, total, err := s.repo.SearchSkillDirectory(ctx, req)
	if err != nil {
		return domain.SearchSkillDirectoryResponse{}, err
	}
	return domain.SearchSkillDirectoryResponse{Items: items, Total: total, Limit: req.Pagination.Limit, Offset: req.Pagination.Offset}, nil
}

// GetUserSkillProfile implements SkillManagementService.
func (s *skillService) GetUserSkillProfile(ctx context.Context, userID string) (domain.UserSkillProfileResponse, error) {
	if _, err := s.requireCapability(ctx, domain.SkillCapDirectory); err != nil {
		return domain.UserSkillProfileResponse{}, err
	}
	if err := validateUUIDs("userId", []string{userID}); err != nil {
		return domain.UserSkillProfileResponse{}, err
	}
	p, err := s.repo.GetUserSkillProfile(ctx, strings.ToLower(userID))
	if err != nil {
		return domain.UserSkillProfileResponse{}, err
	}
	if p == nil {
		return domain.UserSkillProfileResponse{}, &apierror.NotFoundError{Msg: "no internal user with this id"}
	}
	return *p, nil
}

// GetUserSkillHistory implements SkillManagementService.
func (s *skillService) GetUserSkillHistory(ctx context.Context, userID string, req domain.SkillHistoryRequest) (domain.SkillHistoryResponse, error) {
	if _, err := s.requireCapability(ctx, domain.SkillCapDirectory); err != nil {
		return domain.SkillHistoryResponse{}, err
	}
	if err := validateUUIDs("userId", []string{userID}); err != nil {
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
	rows, total, err := s.repo.ListUserSkillHistory(ctx, strings.ToLower(userID), req.SkillID, req.Pagination.Limit, req.Pagination.Offset)
	if err != nil {
		return domain.SkillHistoryResponse{}, err
	}
	return domain.SkillHistoryResponse{History: rows, Total: total, Limit: req.Pagination.Limit, Offset: req.Pagination.Offset}, nil
}
