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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeCaseActivitiesSearchPayload,
  BeCaseActivitiesSearchResponse,
  BeCaseActivityEntry,
} from "@api/backend/types";
import type { CaseAuditEntry } from "@features/csm-cases/types/csmCases";

/** Page size used when loading the field-change lane. Capped by the BE; see BE_MAX_PAGE_LIMIT. */
const ACTIVITIES_PAGE_LIMIT = BE_MAX_PAGE_LIMIT;

/** Best display name off an activity entry's flat author fields. */
function activityAuthorName(entry: BeCaseActivityEntry): string {
  const full = entry.createdByFullName?.trim();
  if (full) return full;
  const composed = [entry.createdByFirstName, entry.createdByLastName]
    .filter((p) => p && p.trim())
    .join(" ")
    .trim();
  if (composed) return composed;
  return entry.createdBy?.trim() || "Unknown";
}

/** Map one backend `field_change` activity entry onto a {@link CaseAuditEntry}. */
export function auditEntryFromBeActivity(
  entry: BeCaseActivityEntry,
): CaseAuditEntry {
  return {
    id: entry.id,
    kind: "field_change",
    actor: activityAuthorName(entry),
    createdAt: entry.createdOn,
    changes: (entry.changes ?? []).map((c) => ({
      field: c.field,
      fieldLabel: c.fieldLabel,
      previousValue: c.previousValue,
      newValue: c.newValue,
    })),
  };
}

/**
 * Load the audited field/state-change lane for a case. In LIVE mode calls
 * `POST /cases/{id}/activities/search` with a single wide page (limit capped
 * at BE_MAX_PAGE_LIMIT) and `includeFieldChanges: true`, then filters the
 * response down to `type === "field_change"` entries — this endpoint also
 * returns `comment`/`attachment` entries, but those lanes keep reading from
 * their existing hooks (`useGetCsmCaseComments` / `useGetCsmCaseAttachments`),
 * so they are ignored here to avoid a second, divergent read path. Notably
 * this endpoint excludes work notes, so it must never replace the comments
 * hook.
 */
export function useGetCsmCaseActivities(
  caseId: string | undefined,
): UseQueryResult<CaseAuditEntry[], Error> {
  const api = useBackendApi();

  return useQuery<CaseAuditEntry[], Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_ACTIVITIES, caseId ?? ""],
    queryFn: async (): Promise<CaseAuditEntry[]> => {
      if (!caseId) return [];

      const payload: BeCaseActivitiesSearchPayload = {
        pagination: { offset: 0, limit: ACTIVITIES_PAGE_LIMIT },
        includeFieldChanges: true,
      };
      const response = await api.post<
        BeCaseActivitiesSearchPayload,
        BeCaseActivitiesSearchResponse
      >(`/cases/${encodeURIComponent(caseId)}/activities/search`, payload);
      return (response.activity ?? [])
        .filter((a) => a.type === "field_change")
        .map(auditEntryFromBeActivity);
    },
    enabled: !!caseId,
    staleTime: 10_000,
  });
}
