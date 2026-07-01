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
// React Query hooks for weekly time sheets, approvals, delegation and reports.
// FE-first: backed by `timeCardStore`. Contract the BFF should expose later:
//
//   POST  /time-sheets/search                 my weekly sheets
//   POST  /time-sheets/{user}/{week}/submit    submit a week
//   POST  /time-sheets/{user}/{week}/approve   approve remaining
//   POST  /time-sheets/{user}/{week}/recall    recall approved
//   PATCH /time-cards/{id}                      decide / recall / process / edit
//   GET   /time-cards/approvals                 approval queue
//   GET/PUT /time-cards/delegation             approver delegation
//   GET   /time-cards/reports                   aggregates
//

import {
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { resolveUserInfo } from "@utils/userClaims";
import { useGetUsersMe } from "@features/settings/api/useGetUsersMe";
import type {
  ApproverDelegation,
  CsmTimeCard,
  CsmTimeSheet,
  TimeCardDecisionInput,
  TimecardReports,
  TimeCardSearchFilters,
} from "@features/csm-timecards/types/timeCards";
import {
  activeDelegationFor,
  approveSheet,
  autoGenerateCards,
  clearDelegation,
  computeReports,
  copyPreviousWeek,
  decideTimeCard,
  deleteCard,
  ensureDemoCardsForUser,
  listApprovalQueue,
  listMyTimeSheets,
  processCard,
  recallCard,
  recallSheet,
  rejectSheet,
  setDelegation,
  submitSheet,
  updateCard,
} from "@features/csm-timecards/api/timeCardStore";

/**
 * The signed-in engineer's stable identity.
 * `id` is sourced from `GET /users/me` (platform-owned data) so it is
 * consistent with the BFF's own user resolution and avoids keying time-card
 * data against whichever ID-token claim (`sub` vs email) happens to be
 * populated.  Falls back to the ID-token email while the query loads.
 * Phase 2: once `/users/me` returns `id` and `displayName`, replace the
 * token-derived name below with those fields.
 */
export function useCurrentEngineer(): { id: string; name: string } {
  const { data: me } = useGetUsersMe();
  const info = resolveUserInfo(useIdTokenClaims());
  return { id: me?.email ?? info.email ?? "me", name: info.fullName };
}

/** Invalidate every time-card/-sheet query so all views refresh after a write. */
export function invalidateTimecards(queryClient: QueryClient): void {
  for (const key of [
    ApiQueryKeys.TIME_CARDS_SEARCH,
    ApiQueryKeys.CASE_TIME_CARDS_SEARCH,
    ApiQueryKeys.TIME_SHEETS_SEARCH,
    ApiQueryKeys.TIME_CARD_APPROVAL_QUEUE,
    ApiQueryKeys.TIME_CARD_REPORTS,
    ApiQueryKeys.TIME_CARD_DELEGATION,
  ]) {
    void queryClient.invalidateQueries({ queryKey: [key] });
  }
}

/** The signed-in user's weekly time sheets, newest first. */
export function useMyTimeSheets(
  filters?: TimeCardSearchFilters,
): UseQueryResult<CsmTimeSheet[], Error> {
  const me = useCurrentEngineer();
  return useQuery<CsmTimeSheet[], Error>({
    queryKey: [ApiQueryKeys.TIME_SHEETS_SEARCH, "mine", me.id, filters],
    queryFn: async (): Promise<CsmTimeSheet[]> => {
      ensureDemoCardsForUser(me.id, me.name);
      return listMyTimeSheets(me.id, filters);
    },
    staleTime: 5_000,
  });
}

/** Submitted sheets awaiting the signed-in approver/admin (excludes own). */
export function useApprovalQueue(
  enabled: boolean,
  filters?: TimeCardSearchFilters,
): UseQueryResult<CsmTimeSheet[], Error> {
  const me = useCurrentEngineer();
  return useQuery<CsmTimeSheet[], Error>({
    queryKey: [ApiQueryKeys.TIME_CARD_APPROVAL_QUEUE, me.id, filters],
    queryFn: async (): Promise<CsmTimeSheet[]> => listApprovalQueue(me.id, filters),
    enabled,
    staleTime: 5_000,
  });
}

/** Submit a user's week (their pending/rejected/recalled cards → submitted). */
export function useSubmitSheet(): UseMutationResult<
  void,
  Error,
  { userId: string; weekStart: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ userId, weekStart }) => submitSheet(userId, weekStart),
    onSuccess: () => invalidateTimecards(queryClient),
  });
}

/** Copy last week's tasks into a target week as zero-hour pending cards. */
export function useCopyPreviousWeek(): UseMutationResult<
  number,
  Error,
  { weekStart: string }
> {
  const me = useCurrentEngineer();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ weekStart }) => copyPreviousWeek(me.id, weekStart),
    onSuccess: () => invalidateTimecards(queryClient),
  });
}

/** Generate zero-hour pending cards from the user's assigned tasks (mock). */
export function useAutoGenerate(): UseMutationResult<
  number,
  Error,
  { weekStart: string }
> {
  const me = useCurrentEngineer();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ weekStart }) =>
      autoGenerateCards(me.id, me.name, weekStart),
    onSuccess: () => invalidateTimecards(queryClient),
  });
}

/** Approve every still-submitted card in a sheet. */
export function useApproveSheet(): UseMutationResult<
  void,
  Error,
  { userId: string; weekStart: string; comment?: string }
> {
  const me = useCurrentEngineer();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ userId, weekStart, comment }) =>
      approveSheet(userId, weekStart, me.name, comment),
    onSuccess: () => invalidateTimecards(queryClient),
  });
}

/** Reject every still-submitted card in a sheet. */
export function useRejectSheet(): UseMutationResult<
  void,
  Error,
  { userId: string; weekStart: string; comment?: string }
> {
  const me = useCurrentEngineer();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ userId, weekStart, comment }) =>
      rejectSheet(userId, weekStart, me.name, comment),
    onSuccess: () => invalidateTimecards(queryClient),
  });
}

/** Recall every approved card in a sheet. */
export function useRecallSheet(): UseMutationResult<
  void,
  Error,
  { userId: string; weekStart: string }
> {
  const me = useCurrentEngineer();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ userId, weekStart }) =>
      recallSheet(userId, weekStart, me.name),
    onSuccess: () => invalidateTimecards(queryClient),
  });
}

/** Approve or reject a single card (within a submitted sheet). */
export function useDecideCard(): UseMutationResult<
  CsmTimeCard,
  Error,
  TimeCardDecisionInput
> {
  const me = useCurrentEngineer();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (decision) => decideTimeCard(decision, me.name),
    onSuccess: () => invalidateTimecards(queryClient),
  });
}

/** Recall a single approved card. */
export function useRecallCard(): UseMutationResult<CsmTimeCard, Error, string> {
  const me = useCurrentEngineer();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (cardId) => recallCard(cardId, me.name),
    onSuccess: () => invalidateTimecards(queryClient),
  });
}

/** Mark an approved card as processed (admin). */
export function useProcessCard(): UseMutationResult<CsmTimeCard, Error, string> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (cardId) => processCard(cardId),
    onSuccess: () => invalidateTimecards(queryClient),
  });
}

/** Edit an editable card's fields (owner or admin). */
export function useUpdateCard(): UseMutationResult<
  CsmTimeCard,
  Error,
  { cardId: string; patch: Parameters<typeof updateCard>[1] }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ cardId, patch }) => updateCard(cardId, patch),
    onSuccess: () => invalidateTimecards(queryClient),
  });
}

/** Delete an editable card (owner or admin); records a deletion audit entry. */
export function useDeleteCard(): UseMutationResult<void, Error, string> {
  const me = useCurrentEngineer();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (cardId) => deleteCard(cardId, me.name),
    onSuccess: () => invalidateTimecards(queryClient),
  });
}

/** The signed-in approver's active delegation, if any. */
export function useDelegation(): UseQueryResult<
  ApproverDelegation | null,
  Error
> {
  const me = useCurrentEngineer();
  return useQuery<ApproverDelegation | null, Error>({
    queryKey: [ApiQueryKeys.TIME_CARD_DELEGATION, me.id],
    queryFn: async () => activeDelegationFor(me.id) ?? null,
    staleTime: 5_000,
  });
}

/** Set or clear the signed-in approver's delegation. */
export function useSetDelegation(): UseMutationResult<
  void,
  Error,
  Omit<ApproverDelegation, "approverId"> | null
> {
  const me = useCurrentEngineer();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input) => {
      if (input === null) clearDelegation(me.id);
      else setDelegation({ approverId: me.id, ...input });
    },
    onSuccess: () => invalidateTimecards(queryClient),
  });
}

/** Aggregated report figures across all time cards. */
export function useTimecardReports(
  enabled: boolean,
  filter?: { from?: string; to?: string },
): UseQueryResult<TimecardReports, Error> {
  return useQuery<TimecardReports, Error>({
    queryKey: [ApiQueryKeys.TIME_CARD_REPORTS, filter?.from, filter?.to],
    queryFn: async () => computeReports(filter),
    enabled,
    staleTime: 5_000,
  });
}
