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

import { beStateFromUi } from "@api/backend/mappers";
import type { AnnouncementFilters } from "@features/csm-announcements/types/csmAnnouncements";

/**
 * One row of the Announcements tab's grouped registry (Phase 3 "registry
 * groups by batch" backlog item) — see `useSearchAnnouncementRegistry`'s own
 * doc comment for the full "why". `kind` decides which of the two field
 * groups below is populated:
 *
 * - `"batch"`: a published announcement request — every case it created
 *   collapses into this one row (`projectCount` of them).
 * - `"case"`: an announcement-type case with no known owning request
 *   (anything sent before this workflow existed, or via any other path) —
 *   shown individually, exactly as the old flat list did.
 */
/** One project's own case within a "batch" row. */
export interface AnnouncementRegistryCaseMember {
  caseId: string;
  caseNumber: string;
  wso2CaseId: string;
  projectName: string;
}

export interface AnnouncementRegistryRow {
  kind: "batch" | "case";
  subject: string;
  createdBy?: string;
  createdOn?: string;
  updatedOn?: string;

  /** Set for kind="batch" only. */
  announcementRequestId?: string;
  /** Set for kind="batch" only. */
  projectCount?: number;
  /**
   * Set for kind="batch" only — a bare legacy case (kind="case") has no
   * owning request to read this from, and is never displayed as security
   * even if its own case happens to carry the tag (the backend has no cheap
   * way to know that for a whole page of rows — see the backend's own doc
   * comment on this field).
   */
  isSecurityAnnouncement?: boolean;
  /** Every member case this batch's request published, one per project. Set for kind="batch" only. */
  cases?: AnnouncementRegistryCaseMember[];

  /** Set for kind="case" only. */
  caseId?: string;
  /** Set for kind="case" only. */
  caseNumber?: string;
  /** Project-scoped WSO2 case reference (case internalId). Set for kind="case" only. */
  wso2CaseId?: string;
  /** The underlying case's own state. Set for kind="case" only. */
  state?: string;
  /** Set for kind="case" only. */
  projectName?: string;
}

export interface SearchAnnouncementRegistryPayload {
  search?: string;
  states?: string[];
  projectIds?: string[];
  pagination: { offset: number; limit: number };
}

export interface SearchAnnouncementRegistryResponse {
  rows: AnnouncementRegistryRow[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

/** Builds a SearchAnnouncementRegistryPayload from the tab's existing AnnouncementFilters shape. */
export function registryPayloadFromFilters(
  filters: AnnouncementFilters,
  offset: number,
  limit: number,
): SearchAnnouncementRegistryPayload {
  const payload: SearchAnnouncementRegistryPayload = { pagination: { offset, limit } };
  const search = filters.search.trim();
  if (search.length > 0) payload.search = search;
  if (filters.states.length > 0) payload.states = filters.states.map(beStateFromUi);
  if (filters.projectIds.length > 0) payload.projectIds = filters.projectIds;
  return payload;
}
