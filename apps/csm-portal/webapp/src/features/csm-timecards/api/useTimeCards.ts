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

/** A case's time cards from the first `BE_MAX_PAGE_LIMIT` cards in its
 * project, plus whether that one page fell short of the project's full
 * `total` — some of the case's cards may not have been fetched if so. No
 * pagination UI here (unlike the three tabs on `/time-cards`) — a single
 * case logging more than a page's worth of time is not expected. */
export interface CaseTimeCardsResult {
  cards: CsmTimeCard[];
  truncated: boolean;
}

/**
 * Time cards logged on a single case, newest first. `filters.caseId` is
 * documented in `openapi.yaml` and genuinely implemented end-to-end in
 * `entity-service` (`sn_time_card_service.go` forwards it straight through
 * to ServiceNow) — but confirmed live to be non-functional in practice: a
 * search scoped by nothing but a case's own id returns `total: 0`
 * unconditionally, even seconds/minutes/an hour after creating a card
 * against that exact case, and even though the very same project-scoped
 * search independently proves cards with that exact `case.id` exist (7 of
 * 10 cards in the project used to verify this). `userId` and `approverId`
 * were separately confirmed to work correctly (see {@link useMyTimeSheets}
 * and {@link useApprovalQueue} in `useTimeSheets.ts`) — this is specific to
 * `caseId`. Do not switch this back to `filters.caseId` without re-confirming
 * live first. Scopes to the case's own project instead (also requires
 * `filters.projectIds` to be non-empty — see {@link searchTimeCards}) and
 * filters the case's cards out client-side.
 */
export function useCaseTimeCards(
  caseId: string | undefined,
  projectId: string | undefined,
): UseQueryResult<CaseTimeCardsResult, Error> {
  const api = useBackendApi();
  return useQuery<CaseTimeCardsResult, Error>({
    queryKey: [ApiQueryKeys.CASE_TIME_CARDS_SEARCH, caseId ?? "", projectId ?? ""],
    queryFn: async (): Promise<CaseTimeCardsResult> => {
      if (!caseId || !projectId) return { cards: [], truncated: false };
      const { cards, total } = await searchTimeCards(api, { projectIds: [projectId] });
      return {
        cards: cards.filter((c) => c.caseId === caseId),
        truncated: cards.length < total,
      };
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
        // The backend's time fields are whole minutes (confirmed:
        // entity-service's own validation helper is named
        // `nonNegativeMinutes`, and a real pre-existing card's `totalTime` of
        // 150 only makes sense as ~2.5h of work in one entry, not 150 hours)
        // — the form collects minutes directly now, so this is a defensive
        // round only, not a unit conversion.
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
