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
  /** The scale new skills and skill requests use. */
  isDefault?: boolean;
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
  /** Inactive categories are only sent to a catalogue manager. */
  active?: boolean;
  description?: string | null;
  /** Active skills filed directly under it. */
  skillCount?: number;
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
  /** How many people hold it; only sent to a catalogue manager. */
  holderCount?: number;
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

export type SkillHistoryAction = "ADDED" | "LEVEL_CHANGED" | "REMOVED" | "READDED" | "";

export interface SkillHistoryEntry {
  id: string;
  /** Empty only for a migrated row that recorded neither level. */
  action?: SkillHistoryAction;
  /** Set when an approved skill request made the change. */
  skillRequestId?: string | null;
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

/** GET /skills/me/capabilities: which gated tasks the caller holds. */
export interface SkillCapabilities {
  canViewDirectory: boolean;
  canReviewRequests: boolean;
  canManageCatalogue: boolean;
  pendingRequestCount?: number;
}

export interface CreateSkillCategoryRequest {
  name: string;
  parentId?: string;
  description?: string;
}

export interface UpdateSkillCategoryRequest {
  name?: string;
  description?: string;
  active?: boolean;
}

export type SkillRequestStatus = "PENDING" | "APPROVED" | "REJECTED" | "WITHDRAWN";

export type SkillAutoAddNote =
  | "ADDED"
  | "READDED"
  | "ALREADY_HELD"
  | "LEVEL_ADJUSTED"
  | "SKIPPED_INACTIVE_USER"
  | "SKIPPED_NO_SCALE";

export interface SkillPersonRef {
  id: string;
  name?: string | null;
  email?: string | null;
}

export interface SkillRef {
  id: string;
  name: string;
  active: boolean;
}

export interface SkillRequest {
  id: string;
  requestedName: string;
  description?: string | null;
  category?: SkillCategoryRef | null;
  suggestedCategory?: string | null;
  requestedLevel?: SkillLevel | null;
  status: SkillRequestStatus;
  requestedBy: SkillPersonRef;
  createdOn: string;
  reviewedBy?: SkillPersonRef | null;
  reviewedOn?: string | null;
  /** The approval comment or the rejection reason. */
  reviewComment?: string | null;
  resolution?: "CREATED" | "REACTIVATED" | "EXISTING" | null;
  resolvedSkill?: SkillRef | null;
  autoAddedLevel?: SkillLevel | null;
  autoAddNote?: SkillAutoAddNote | null;
}

export interface SkillRequestsResponse {
  requests: SkillRequest[];
  total: number;
  limit: number;
  offset: number;
}

export interface CheckSkillRequestResponse {
  exactMatch?: SkillRef | null;
  similar: SkillRef[];
  pendingRequest?: SkillRequest | null;
}

export interface CreateSkillRequestRequest {
  name: string;
  categoryId?: string;
  suggestedCategory?: string;
  description?: string;
  levelId: string;
}

export type ApproveMode = "create" | "reactivate" | "existing";

export interface ApproveSkillRequestRequest {
  mode: ApproveMode;
  name?: string;
  categoryId?: string;
  description?: string;
  keywords?: string;
  skillId?: string;
  levelId?: string;
  comment?: string;
}

export interface SkillCriterion {
  skillId: string;
  /** 0 = any level. */
  minLevelValue?: number;
}

export interface SearchSkillDirectoryRequest {
  searchQuery?: string;
  teamKeys?: string[];
  categoryIds?: string[];
  criteria?: SkillCriterion[];
  match?: "ALL" | "ANY";
  pagination?: { limit?: number; offset?: number };
}

export interface SkillDirectoryPerson {
  userId: string;
  userName?: string | null;
  userEmail?: string | null;
  teamKeys: string[];
  matchedCount: number;
  skills: UserSkill[];
}

export interface SearchSkillDirectoryResponse {
  items: SkillDirectoryPerson[];
  total: number;
  limit: number;
  offset: number;
}

export interface UserSkillProfileResponse {
  userId: string;
  userName?: string | null;
  userEmail?: string | null;
  teamKeys: string[];
  skills: UserSkill[];
}
