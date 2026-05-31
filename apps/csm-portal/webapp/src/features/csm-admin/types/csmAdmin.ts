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

export type CsmUserStatus = "Active" | "Suspended" | "Invited";

export interface CsmUser {
  id: string;
  name: string;
  email: string;
  status: CsmUserStatus;
  /** Asgardeo / IdP id; informational only in the UI. */
  externalId: string;
  /** Role ids assigned to the user. */
  roleIds: string[];
  /** Group ids the user belongs to. */
  groupIds: string[];
  /** ISO date of last login (or invitation). */
  lastActiveAt: string;
}

export interface CsmRole {
  id: string;
  name: string;
  description: string;
  /** Permission ids granted by this role. */
  permissionIds: string[];
  /** True for product-defined roles that should not be deleted. Renaming /
   *  re-permissioning a built-in is allowed; the mocks just flag it. */
  builtIn: boolean;
}

export interface CsmGroup {
  id: string;
  name: string;
  description: string;
  /** User ids that are members of the group. */
  memberIds: string[];
  /** Role ids granted to every member of the group. */
  roleIds: string[];
}

export type CsmPermissionCategory =
  | "cases"
  | "projects"
  | "accounts"
  | "engagements"
  | "updates"
  | "security"
  | "time_cards"
  | "admin";

export interface CsmPermission {
  id: string;
  /** Human-readable label (e.g. "View all cases"). */
  name: string;
  /** One-line description for the assignment UI. */
  description: string;
  category: CsmPermissionCategory;
}
