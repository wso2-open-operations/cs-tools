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

//
// Card-level React Query hooks (a case's cards, create, decide), backed by
// the real csm-portal-backend endpoints:
//
//   POST  /time-cards/search   list for a case (client-filtered, see below)
//   POST  /time-cards          create (already `submitted` — no draft step)
//   PATCH /time-cards/{id}     accept/reject { state, leadComment }
//

import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeCreateTimeCardPayload,
  BeTimeCardMutationResponse,
} from "@api/backend/types";
import type {
  CreateTimeCardInput,
  CsmTimeCard,
} from "@features/csm-timecards/types/timeCards";
import {
  invalidateTimecards,
  mapTimeCard,
  searchTimeCards,
  useDecideCard,
} from "@features/csm-timecards/api/useTimeSheets";

/**
 * Time cards logged on a single case, newest first. There is no `caseId`
 * search filter on the backend — `POST /time-cards/search` also requires a
 * non-empty `filters.projectIds` to return anything at all (confirmed live;
 * an unscoped search always returns `total: 0` despite the OpenAPI spec
 * documenting `projectIds` as optional) — so this scopes the search to the
 * case's own project and filters the case's cards out client-side.
 */
export function useCaseTimeCards(
  caseId: string | undefined,
  projectId: string | undefined,
): UseQueryResult<CsmTimeCard[], Error> {
  const api = useBackendApi();
  return useQuery<CsmTimeCard[], Error>({
    queryKey: [ApiQueryKeys.CASE_TIME_CARDS_SEARCH, caseId ?? "", projectId ?? ""],
    queryFn: async (): Promise<CsmTimeCard[]> => {
      if (!caseId || !projectId) return [];
      const all = await searchTimeCards(api, { projectIds: [projectId] });
      return all.filter((c) => c.caseId === caseId);
    },
    enabled: !!caseId && !!projectId,
    staleTime: 5_000,
  });
}

/**
 * Create a time card on the signed-in engineer's behalf. The card is created
 * already `submitted` — the backend has no draft/pending step.
 */
export function usePostTimeCard(): UseMutationResult<
  CsmTimeCard,
  Error,
  CreateTimeCardInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();
  return useMutation<CsmTimeCard, Error, CreateTimeCardInput>({
    mutationFn: async (input): Promise<CsmTimeCard> => {
      const payload: BeCreateTimeCardPayload = {
        caseId: input.caseId,
        projectId: input.projectId,
        date: input.date,
        approverIds: [input.approver.id],
        isBillable: input.billable,
        issueComplexity: input.issueComplexity,
        workLogComment: input.workLogComment,
        // The backend's hour fields are integers (confirmed live: a 0.5
        // value is rejected with 400) even though the form logs quarter-hour
        // increments — round each bucket to the nearest whole hour here, at
        // the API boundary, so the FE keeps its finer-grained display.
        timeAnalyzing: Math.round(input.breakdown.analysisDebugging),
        timeSettingUp: Math.round(input.breakdown.settingUp),
        timeReproducingDebugging: Math.round(input.breakdown.reproduce),
        timeProvidingSolution: Math.round(input.breakdown.providingSolution),
        timePatching: Math.round(input.breakdown.answering),
      };
      const res = await api.post<BeCreateTimeCardPayload, BeTimeCardMutationResponse>(
        "/time-cards",
        payload,
      );
      return mapTimeCard(res.timeCard);
    },
    onSuccess: () => invalidateTimecards(queryClient),
  });
}

/**
 * Accept or reject a single time card (approver/admin). An alias of
 * {@link useDecideCard} — the case-panel and the Approvals-queue decide
 * flows were identical mutations with two independently drifting
 * `onSuccess` invalidation lists; this keeps a single implementation so
 * they can't drift again.
 */
export const useDecideTimeCard = useDecideCard;
