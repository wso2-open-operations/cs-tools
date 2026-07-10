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

import type {
  CaseState,
  Severity,
} from "@features/csm-dashboard/types/abtDashboard";

/**
 * Filter state for the announcements list. All arrays are multi-select and
 * empty by default, so the list shows every state and severity across all
 * projects until the user narrows it. `search` matches subject / number.
 */
export interface AnnouncementFilters {
  search: string;
  states: CaseState[];
  severities: Severity[];
  /** Project ids (the project filter is id-based). */
  projectIds: string[];
}

export const DEFAULT_ANNOUNCEMENT_FILTERS: AnnouncementFilters = {
  search: "",
  states: [],
  severities: [],
  projectIds: [],
};

/**
 * A single announcement row for the list. Announcements are stored as cases of
 * `type: "announcement"` and read back through `POST /cases/search`, so the row
 * is a trimmed projection of the case search view — only the fields the
 * read-only list renders. (Targeting/audience metadata isn't available until
 * the dedicated announcement backend lands — see digiops-cs#2053.)
 */
export interface CsmAnnouncementRow {
  id: string;
  number?: string;
  subject: string;
  /** Owning project display name, or "—" when cross-customer / unset. */
  projectName: string;
  state?: CaseState;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
}

/** Paginated announcements list, mirroring the backend search envelope. */
export interface CsmAnnouncementsListResponse {
  announcements: CsmAnnouncementRow[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}
