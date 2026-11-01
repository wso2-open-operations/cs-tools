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

package domain

import "time"

// SkillLevel is one rung of a level scale: "Beginner", "Intermediate",
// "Expert". Value orders the rungs within a scale.
type SkillLevel struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Value       *int    `json:"value"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
}

// SkillLevelType is a level scale: the set of levels a skill is rated on.
type SkillLevelType struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description *string      `json:"description"`
	Levels      []SkillLevel `json:"levels"`
}

// SkillLevelTypesResponse is the body of GET /skills/levels.
type SkillLevelTypesResponse struct {
	LevelTypes []SkillLevelType `json:"levelTypes"`
}

// SkillCategory is a node of the skill category tree. CategoryPath is the
// stored display path of the node; IsLowestCategory marks a leaf, the only
// kind skills are filed under.
type SkillCategory struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Code             *string `json:"code"`
	CategoryPath     *string `json:"categoryPath"`
	IsLowestCategory bool    `json:"isLowestCategory"`
	ParentID         *string `json:"parentId"`
}

// SkillCategoriesResponse is the body of GET /skills/categories.
type SkillCategoriesResponse struct {
	Categories []SkillCategory `json:"categories"`
}

// SkillCategoryRef is a category as listed on a skill.
type SkillCategoryRef struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	CategoryPath *string `json:"categoryPath"`
}

// Skill is one entry of the skill catalogue.
type Skill struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description *string            `json:"description"`
	Keywords    *string            `json:"keywords"`
	Active      bool               `json:"active"`
	LevelTypeID *string            `json:"levelTypeId"`
	Categories  []SkillCategoryRef `json:"categories"`
	CreatedOn   time.Time          `json:"createdOn"`
	UpdatedOn   time.Time          `json:"updatedOn"`
}

// SearchSkillsRequest is the body of POST /skills/search. Every filter is
// optional and they combine with AND. Inactive skills are left out unless
// includeInactive is set.
type SearchSkillsRequest struct {
	SearchQuery     string     `json:"searchQuery"`
	CategoryIDs     []string   `json:"categoryIds"`
	LevelTypeID     string     `json:"levelTypeId"`
	IncludeInactive bool       `json:"includeInactive"`
	Pagination      Pagination `json:"pagination"`
}

// SearchSkillsResponse is the body of POST /skills/search.
type SearchSkillsResponse struct {
	Skills []Skill `json:"skills"`
	Total  int     `json:"total"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
}

// CreateSkillRequest is the body of POST /skills.
type CreateSkillRequest struct {
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	Keywords    *string  `json:"keywords"`
	LevelTypeID string   `json:"levelTypeId"`
	CategoryIDs []string `json:"categoryIds"`
	// Active defaults to true when absent.
	Active *bool `json:"active"`
}

// UpdateSkillRequest is the body of PATCH /skills/{id}. An absent field is
// left as it is. Description and Keywords are cleared with an empty string.
// CategoryIDs replaces the skill's category links as a whole when present
// (an empty list removes them all).
type UpdateSkillRequest struct {
	Name        *string   `json:"name"`
	Description *string   `json:"description"`
	Keywords    *string   `json:"keywords"`
	Active      *bool     `json:"active"`
	LevelTypeID *string   `json:"levelTypeId"`
	CategoryIDs *[]string `json:"categoryIds"`
}

// UserSkill is one skill on somebody's profile.
type UserSkill struct {
	SkillID     string             `json:"skillId"`
	SkillName   string             `json:"skillName"`
	SkillActive bool               `json:"skillActive"`
	Categories  []SkillCategoryRef `json:"categories"`
	LevelTypeID *string            `json:"levelTypeId"`
	Level       *SkillLevel        `json:"level"`
	// Inherited rows come from a group and cannot be changed by the person.
	Inherited            bool      `json:"inherited"`
	InheritedFromGroupID *string   `json:"inheritedFromGroupId"`
	UpdatedOn            time.Time `json:"updatedOn"`
}

// MySkillsResponse is the body of GET /skills/me.
type MySkillsResponse struct {
	UserID string      `json:"userId"`
	Skills []UserSkill `json:"skills"`
}

// AddMySkillRequest is the body of POST /skills/me.
type AddMySkillRequest struct {
	SkillID string `json:"skillId"`
	LevelID string `json:"levelId"`
}

// SetMySkillLevelRequest is the body of PUT /skills/me/{skillId}.
type SetMySkillLevelRequest struct {
	LevelID string `json:"levelId"`
}

// SkillHistoryEntry is one level change on somebody's profile. LevelFrom is
// nil for the first time the skill was set, LevelTo is nil when it was
// removed.
type SkillHistoryEntry struct {
	ID        string      `json:"id"`
	SkillID   string      `json:"skillId"`
	SkillName string      `json:"skillName"`
	LevelFrom *SkillLevel `json:"levelFrom"`
	LevelTo   *SkillLevel `json:"levelTo"`
	ChangedBy *string     `json:"changedBy"`
	ChangedOn time.Time   `json:"changedOn"`
}

// SkillHistoryRequest filters GET /skills/me/history.
type SkillHistoryRequest struct {
	SkillID    string
	Pagination Pagination
}

// SkillHistoryResponse is the body of GET /skills/me/history, newest first.
type SkillHistoryResponse struct {
	History []SkillHistoryEntry `json:"history"`
	Total   int                 `json:"total"`
	Limit   int                 `json:"limit"`
	Offset  int                 `json:"offset"`
}

// SearchUserSkillsRequest is the body of POST /skills/users/search: the
// manager's view of everybody's skills. One result row is one person's one
// skill. A row must satisfy every filter given. Only active internal staff
// and their active skills are listed.
type SearchUserSkillsRequest struct {
	SearchQuery string   `json:"searchQuery"`
	UserIDs     []string `json:"userIds"`
	SkillIDs    []string `json:"skillIds"`
	LevelIDs    []string `json:"levelIds"`
	// MinLevelValue keeps rows whose level's value is at least this.
	MinLevelValue *int       `json:"minLevelValue"`
	TeamKeys      []string   `json:"teamKeys"`
	Pagination    Pagination `json:"pagination"`
}

// UserSkillRow is one person's one skill in the manager's view.
type UserSkillRow struct {
	UserID    string      `json:"userId"`
	UserName  *string     `json:"userName"`
	UserEmail *string     `json:"userEmail"`
	TeamKeys  []string    `json:"teamKeys"`
	SkillID   string      `json:"skillId"`
	SkillName string      `json:"skillName"`
	Level     *SkillLevel `json:"level"`
	Inherited bool        `json:"inherited"`
	UpdatedOn time.Time   `json:"updatedOn"`
}

// SearchUserSkillsResponse is the body of POST /skills/users/search.
type SearchUserSkillsResponse struct {
	Items  []UserSkillRow `json:"items"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// SkillOutcome is what a repository write found. The service turns it into
// the caller's answer, as the rota writes do.
type SkillOutcome int

const (
	// SkillOutcomeDone: the change was written.
	SkillOutcomeDone SkillOutcome = iota
	// SkillOutcomeNotHeld: the person has no such skill on their profile.
	SkillOutcomeNotHeld
	// SkillOutcomeAlreadyHeld: the person already has the skill.
	SkillOutcomeAlreadyHeld
	// SkillOutcomeInherited: the row comes from a group and is not theirs to change.
	SkillOutcomeInherited
	// SkillOutcomeUnchanged: the level asked for is the one already held.
	SkillOutcomeUnchanged
	// SkillOutcomeNotFound: the skill does not exist.
	SkillOutcomeNotFound
	// SkillOutcomeNameTaken: another skill already has this name.
	SkillOutcomeNameTaken
	// SkillOutcomeLevelsInUse: the level scale cannot change while people hold levels of it.
	SkillOutcomeLevelsInUse
	// SkillOutcomeLevelNotInScale: when the write took its lock, the level was not
	// on the skill's scale. The service checked it just before, so the scale was
	// changed in between.
	SkillOutcomeLevelNotInScale
	// SkillOutcomeSkillInactive: when an add took its lock, the skill was inactive.
	// The service checked it just before, so it was deactivated in between.
	SkillOutcomeSkillInactive
)

// SkillWriteResult is a skill write's outcome and, for a profile write, the
// row as it stands.
type SkillWriteResult struct {
	Outcome SkillOutcome
	Skill   *Skill
	Row     *UserSkill
}
