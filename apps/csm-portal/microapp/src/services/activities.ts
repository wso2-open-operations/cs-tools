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

import { queryOptions } from "@tanstack/react-query";
import { CASE_ACTIVITIES_SEARCH_ENDPOINT } from "@config/endpoints";
import type { CaseActivitiesSearchPayloadDto, CaseActivitiesSearchResponseDto, CaseActivityEntryDto } from "@src/types";
import { toCaseAuditEntry, type CaseAuditEntry } from "@src/types";
import apiClient from "./apiClient";

// Matches the app's other search endpoints (see COMMENTS_PAGE_LIMIT in services/cases.ts) — the
// live upstream rejects a page size above 50 despite what openapi.yaml's declared max claims.
const ACTIVITIES_PAGE_LIMIT = 50;

// Safety cap mirroring CASES_SEARCH_MAX_PAGES in services/cases.ts — bounds the loop below rather
// than trusting `hasMore` to eventually go false.
const ACTIVITIES_MAX_PAGES = 20;

// This endpoint also returns `comment`/`attachment` typed entries, but those lanes keep reading
// from their own dedicated endpoints (cases.comments / attachments.forCase) — only field_change
// entries are consumed here, mirroring the webapp's useCsmCaseActivities.ts (see the note on
// CaseActivityEntryDto). Unlike the webapp (which reads a single capped page), this pages through
// the full result set — a case with more than one page of history would otherwise silently lose
// its older field-change entries from the feed.
const searchCaseActivities = async (caseId: string): Promise<CaseAuditEntry[]> => {
  const entries: CaseActivityEntryDto[] = [];
  for (let page = 0; page < ACTIVITIES_MAX_PAGES; page += 1) {
    const payload: CaseActivitiesSearchPayloadDto = {
      pagination: { offset: page * ACTIVITIES_PAGE_LIMIT, limit: ACTIVITIES_PAGE_LIMIT },
      includeFieldChanges: true,
    };
    const { data } = await apiClient.post<CaseActivitiesSearchResponseDto>(
      CASE_ACTIVITIES_SEARCH_ENDPOINT(caseId),
      payload,
    );
    const batch = data.activity ?? [];
    entries.push(...batch);
    if (batch.length < ACTIVITIES_PAGE_LIMIT || !data.hasMore) break;
  }
  return entries.filter((a) => a.type === "field_change").map(toCaseAuditEntry);
};

export const activities = {
  forCase: (caseId: string) =>
    queryOptions({
      queryKey: ["case", caseId, "activities"],
      queryFn: () => searchCaseActivities(caseId),
    }),
};
