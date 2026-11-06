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

/** A level of a scale. `value` orders the levels of one scale. */
export interface SkillLevel {
  id: string;
  name: string;
  value?: number | null;
  description?: string | null;
  color?: string | null;
}

/** A level scale (for example Beginner to Expert) a skill is measured on. */
export interface SkillLevelType {
  id: string;
  name: string;
  description?: string | null;
  levels: SkillLevel[];
}

export interface SkillLevelTypesResponse {
  levelTypes: SkillLevelType[];
}

/** One node of the flat category list; the tree is built client-side. */
export interface SkillCategory {
  id: string;
  name: string;
  code?: string | null;
  categoryPath?: string | null;
  isLowestCategory: boolean;
  parentId?: string | null;
}

export interface SkillCategoriesResponse {
  categories: SkillCategory[];
}

export interface SkillCategoryRef {
  id: string;
  name: string;
  categoryPath?: string | null;
}

export interface Skill {
  id: string;
  name: string;
  description?: string | null;
  keywords?: string | null;
  active: boolean;
  levelTypeId?: string | null;
  categories: SkillCategoryRef[];
  createdOn: string;
  updatedOn: string;
}

export interface SearchSkillsRequest {
  searchQuery?: string;
  categoryIds?: string[];
  levelTypeId?: string;
  includeInactive?: boolean;
  pagination?: { limit?: number; offset?: number };
}

export interface SearchSkillsResponse {
  skills: Skill[];
  total: number;
  limit: number;
  offset: number;
}

export interface CreateSkillRequest {
  name: string;
  description?: string;
  keywords?: string;
  levelTypeId: string;
  categoryIds?: string[];
  active?: boolean;
}

/** Fields left out are unchanged. `description`/`keywords` are cleared with "". */
export interface UpdateSkillRequest {
  name?: string;
  description?: string;
  keywords?: string;
  active?: boolean;
  levelTypeId?: string;
  categoryIds?: string[];
}

/** One row of the caller's own profile. */
export interface UserSkill {
  skillId: string;
  skillName: string;
  skillActive: boolean;
  categories: SkillCategoryRef[];
  levelTypeId?: string | null;
  level?: SkillLevel | null;
  /** True when the row comes from a group; the person cannot change it. */
  inherited: boolean;
  inheritedFromGroupId?: string | null;
  updatedOn: string;
}

export interface MySkillsResponse {
  userId: string;
  skills: UserSkill[];
}

export interface SkillHistoryEntry {
  id: string;
  skillId: string;
  skillName: string;
  levelFrom?: SkillLevel | null;
  levelTo?: SkillLevel | null;
  changedBy?: string | null;
  changedOn: string;
}

export interface SkillHistoryResponse {
  history: SkillHistoryEntry[];
  total: number;
  limit: number;
  offset: number;
}

export interface SearchUserSkillsRequest {
  searchQuery?: string;
  userIds?: string[];
  skillIds?: string[];
  levelIds?: string[];
  minLevelValue?: number;
  teamKeys?: string[];
  pagination?: { limit?: number; offset?: number };
}

export interface UserSkillRow {
  userId: string;
  userName?: string | null;
  userEmail?: string | null;
  teamKeys: string[];
  skillId: string;
  skillName: string;
  level?: SkillLevel | null;
  inherited: boolean;
  updatedOn: string;
}

export interface SearchUserSkillsResponse {
  items: UserSkillRow[];
  total: number;
  limit: number;
  offset: number;
}
