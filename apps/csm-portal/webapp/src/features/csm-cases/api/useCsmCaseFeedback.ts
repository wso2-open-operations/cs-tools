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
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeCaseFeedback,
  BeCaseFeedbackSearchFilters,
  BeCaseFeedbackSearchResponse,
} from "@api/backend/types";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";
import type { CaseFeedbackEntry } from "@features/csm-cases/types/csmCases";
import { normalizeBackendTimestamp } from "@utils/dateTime";

function feedbackEntryFromBe(f: BeCaseFeedback): CaseFeedbackEntry {
  return {
    id: f.instanceId,
    rating: f.rating,
    ratingLabel: f.ratingLabel,
    comment: f.comment,
    // ServiceNow returns submittedAt as a raw space-separated timestamp
    // ("2026-08-17 06:17:56"), not ISO 8601 like every other activity-feed
    // entry's own timestamp. CaseActivitiesFeed's compareFeedEntries sorts by
    // a plain string compare of each entry's `at` — a raw space-separated
    // string sorts BEFORE a same-day ISO "T"-separated one purely because
    // " " < "T" in ASCII, regardless of actual time-of-day, which put real
    // feedback entries out of chronological order in the feed. Normalize to
    // ISO 8601 UTC here, at the API boundary, same as every other entry.
    submittedAt: normalizeBackendTimestamp(f.submittedAt) ?? f.submittedAt,
    submitterName: f.submitterName,
    submitterEmail: f.submitterEmail,
  };
}

/**
 * Loads any Case Feedback survey submissions for a single case, for the case
 * detail page's activity feed. Case Feedback is a CSAT survey submitted by
 * the customer, typically only once a case is closed — an open case will
 * almost always resolve to an empty list, which is expected, not an error.
 *
 * Reuses `WIDGET_RESOURCE_CONFIG.case_feedback`'s endpoint rather than
 * hardcoding the path, so this stays in sync with that config's own source
 * of truth for the endpoint name.
 */
export function useGetCsmCaseFeedback(
  caseId: string | undefined,
): UseQueryResult<CaseFeedbackEntry[], Error> {
  const api = useBackendApi();

  return useQuery<CaseFeedbackEntry[], Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_FEEDBACK, caseId ?? ""],
    queryFn: async (): Promise<CaseFeedbackEntry[]> => {
      if (!caseId) return [];

      const payload = {
        filters: { caseId } satisfies BeCaseFeedbackSearchFilters,
        page: 1,
        pageSize: 20,
      };
      const response = await api.post<
        typeof payload,
        BeCaseFeedbackSearchResponse
      >(WIDGET_RESOURCE_CONFIG.case_feedback.searchEndpoint, payload);
      return (response.results ?? []).map(feedbackEntryFromBe);
    },
    enabled: !!caseId,
    staleTime: 10_000,
  });
}
