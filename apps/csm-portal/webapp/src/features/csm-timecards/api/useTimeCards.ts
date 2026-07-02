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
// Card-level React Query hooks (a case's cards, create, decide). FE-first:
// backed by `timeCardStore`. Weekly sheets/approvals/reports live in
// `useTimeSheets`. API contract the BFF should expose:
//
//   POST   /cases/{id}/time-cards            create
//   POST   /cases/{id}/time-cards/search     list for a case
//   PATCH  /time-cards/{cardId}              accept/reject { state, leadComment }
//

import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import type {
  CreateTimeCardInput,
  CsmTimeCard,
  TimeCardDecisionInput,
} from "@features/csm-timecards/types/timeCards";
import {
  createTimeCard,
  decideTimeCard,
  listCaseTimeCards,
} from "@features/csm-timecards/api/timeCardStore";
import {
  invalidateTimecards,
  useCurrentEngineer,
} from "@features/csm-timecards/api/useTimeSheets";

/** Time cards logged on a single case, newest first. */
export function useCaseTimeCards(
  caseId: string | undefined,
): UseQueryResult<CsmTimeCard[], Error> {
  return useQuery<CsmTimeCard[], Error>({
    queryKey: [ApiQueryKeys.CASE_TIME_CARDS_SEARCH, caseId ?? ""],
    queryFn: async (): Promise<CsmTimeCard[]> => {
      if (!caseId) return [];
      // TODO(backend): api.post(`/cases/${caseId}/time-cards/search`, payload)
      return listCaseTimeCards(caseId);
    },
    enabled: !!caseId,
    staleTime: 5_000,
  });
}

/** Create a time card on the signed-in engineer's behalf. */
export function usePostTimeCard(): UseMutationResult<
  CsmTimeCard,
  Error,
  CreateTimeCardInput
> {
  const me = useCurrentEngineer();
  const queryClient = useQueryClient();
  return useMutation<CsmTimeCard, Error, CreateTimeCardInput>({
    mutationFn: async (input): Promise<CsmTimeCard> => {
      // TODO(backend): api.post(`/cases/${input.caseId}/time-cards`, payload)
      return createTimeCard(input, me);
    },
    onSuccess: () => invalidateTimecards(queryClient),
  });
}

/** Accept or reject a single time card (approver/admin). */
export function useDecideTimeCard(): UseMutationResult<
  CsmTimeCard,
  Error,
  TimeCardDecisionInput
> {
  const me = useCurrentEngineer();
  const queryClient = useQueryClient();
  return useMutation<CsmTimeCard, Error, TimeCardDecisionInput>({
    mutationFn: async (decision): Promise<CsmTimeCard> => {
      // TODO(backend): api.patch(`/time-cards/${decision.cardId}`, { state, leadComment })
      return decideTimeCard(decision, me.name);
    },
    onSuccess: () => invalidateTimecards(queryClient),
  });
}
