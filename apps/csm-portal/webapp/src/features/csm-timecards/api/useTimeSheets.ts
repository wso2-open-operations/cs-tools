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
// React Query hooks for weekly time sheets and approvals, backed by the real
// csm-portal-backend endpoints:
//
//   POST  /time-cards/search   my sheets / approval queue (client-grouped into weeks)
//   PATCH /time-cards/{id}     approve / reject a card
//
// The backend has no "sheet" concept, no bulk (approve/reject/submit a whole
// week) endpoint, no delegation, and no reports/aggregates endpoint — those
// features from the earlier FE-first mock are not available here. Weekly
// sheets are a pure client-side grouping of individual cards for display.
//

import {
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { resolveUserInfo } from "@utils/userClaims";
import { useGetUsersMe } from "@features/settings/api/useGetUsersMe";
import { useBackendApi, type BackendApi } from "@api/backend/client";
import type {
  BeSearchTimeCardsPayload,
  BeSearchTimeCardsResponse,
  BeTimeCardState,
  BeTimeCardView,
  BeUpdateTimeCardPayload,
  BeTimeCardMutationResponse,
} from "@api/backend/types";
import type {
  CsmTimeCard,
  CsmTimeSheet,
  TimeCardDecisionInput,
  TimeCardSearchFilters,
  TimeSheetState,
} from "@features/csm-timecards/types/timeCards";
import { weekEndOf, weekStartOf } from "@features/csm-timecards/utils/timeSheetWeek";
import { roundHours } from "@features/csm-timecards/utils/timeCardTotals";

/**
 * The signed-in engineer's stable identity, resolved from `GET /users/me`.
 * `id` is the entity-service UUID — the same stable identifier the platform
 * uses across all services. Display name is built from firstName + lastName
 * returned by the entity service, falling back to ID-token values while the
 * query is in flight. `id` is `undefined` until a real identity resolves —
 * never a synthetic placeholder — so callers must gate on it before running
 * queries/mutations that key off it (a placeholder id would silently let
 * "my cards" / "not my cards" filtering match nothing, or everything).
 */
export function useCurrentEngineer(): { id: string | undefined; name: string } {
  const { data: me } = useGetUsersMe();
  const info = resolveUserInfo(useIdTokenClaims());
  const displayName =
    [me?.firstName, me?.lastName].filter(Boolean).join(" ") || info.fullName;
  return { id: me?.id ?? me?.email ?? info.email, name: displayName };
}

/** Invalidate every time-card query so all views refresh after a write. */
export function invalidateTimecards(queryClient: QueryClient): void {
  for (const key of [
    ApiQueryKeys.TIME_CARDS_SEARCH,
    ApiQueryKeys.CASE_TIME_CARDS_SEARCH,
    ApiQueryKeys.TIME_SHEETS_SEARCH,
    ApiQueryKeys.TIME_CARD_APPROVAL_QUEUE,
  ]) {
    void queryClient.invalidateQueries({ queryKey: [key] });
  }
}

/** Map the backend's `TimeCardView` to the portal's `CsmTimeCard`. */
export function mapTimeCard(v: BeTimeCardView): CsmTimeCard {
  return {
    id: v.id,
    caseId: v.case?.id ?? "",
    caseNumber: v.case?.number || v.case?.name || "—",
    projectId: v.project?.id ?? "",
    projectName: v.project?.name ?? "—",
    createdOn: v.createdOn,
    userId: v.user?.id ?? "",
    userName: v.user?.name ?? "—",
    state: v.state,
    billable: v.hasBillable,
    totalHours: v.totalTime,
    approvedById: v.approvedBy?.id,
    approvedByName: v.approvedBy?.name,
  };
}

/**
 * Raw search against `POST /time-cards/search`, mapped to `CsmTimeCard[]`.
 * The backend only supports `projectIds` / date range server-side — there is
 * no `caseId` or `engineerId` filter, so case- and user-scoping happen
 * client-side over the returned page (see {@link useCaseTimeCards},
 * {@link useMyTimeSheets}, {@link useApprovalQueue} below). Fetches
 * `BE_MAX_PAGE_LIMIT` cards (the backend rejects `limit` above that with a
 * generic 400 despite the OpenAPI spec documenting up to 100 — confirmed
 * live); a dataset larger than that isn't fully covered yet — this is a
 * known limitation until search grows pagination support here.
 *
 * `filters.states` is deliberately filtered client-side, never sent as
 * `filters.states` on the wire: confirmed live that the backend 500s
 * ("Failed to search time cards.") when `states` is combined with a large
 * `projectIds` array (reproduced: 2 or 10 project ids + `states` → 200; the
 * full ~88-project default scope + `states` → 500; that same full scope
 * *without* `states` → 200). Since {@link useApprovalQueue} always needs a
 * "submitted" filter and now defaults to every visible project, sending
 * `states` server-side would make the Approvals queue fail outright for any
 * user with enough visible projects — worse than not filtering at all.
 */
export async function searchTimeCards(
  api: BackendApi,
  filters?: TimeCardSearchFilters,
): Promise<CsmTimeCard[]> {
  const payload: BeSearchTimeCardsPayload = {
    filters: {
      ...(filters?.projectIds?.length ? { projectIds: filters.projectIds } : {}),
      ...(filters?.from ? { startDate: filters.from } : {}),
      ...(filters?.to ? { endDate: filters.to } : {}),
    },
    pagination: { limit: BE_MAX_PAGE_LIMIT, offset: 0 },
  };
  const res = await api.post<BeSearchTimeCardsPayload, BeSearchTimeCardsResponse>(
    "/time-cards/search",
    payload,
  );
  const cards = (res.timeCards ?? []).map(mapTimeCard);
  return filters?.states?.length
    ? cards.filter((c) => (filters.states as BeTimeCardState[]).includes(c.state))
    : cards;
}

/** Roll a week's cards up into a single display status. */
function sheetStatus(cards: CsmTimeCard[]): TimeSheetState {
  if (cards.some((c) => c.state === "rejected")) return "rejected";
  if (cards.every((c) => c.state === "approved" || c.state === "processed")) {
    return "approved";
  }
  return "submitted";
}

/** Group a flat list of cards into weekly sheets (Mon–Sun), newest first. */
function groupIntoSheets(
  cards: CsmTimeCard[],
  userId: string,
  userName: string,
): CsmTimeSheet[] {
  const byWeek = new Map<string, CsmTimeCard[]>();
  for (const c of cards) {
    const wk = weekStartOf(c.createdOn);
    const bucket = byWeek.get(wk);
    if (bucket) bucket.push(c);
    else byWeek.set(wk, [c]);
  }
  const sheets: CsmTimeSheet[] = [];
  for (const [weekStart, weekCards] of byWeek) {
    weekCards.sort((a, b) => b.createdOn.localeCompare(a.createdOn));
    sheets.push({
      id: `${userId}:${weekStart}`,
      userId,
      userName,
      weekStart,
      weekEnd: weekEndOf(weekStart),
      state: sheetStatus(weekCards),
      cards: weekCards,
      totalHours: roundHours(weekCards.reduce((sum, c) => sum + c.totalHours, 0)),
    });
  }
  return sheets.sort((a, b) => b.weekStart.localeCompare(a.weekStart));
}

/**
 * The signed-in user's own cards, grouped into weekly sheets, newest first.
 * `filters.projectIds` must be non-empty — the backend requires it to return
 * anything (see {@link searchTimeCards}) — so this stays disabled until the
 * caller has resolved a real project scope (defaulted to every visible
 * project when the user hasn't picked one — see `CsmTimeCardsPage.tsx`).
 */
export function useMyTimeSheets(
  filters?: TimeCardSearchFilters,
): UseQueryResult<CsmTimeSheet[], Error> {
  const api = useBackendApi();
  const me = useCurrentEngineer();
  return useQuery<CsmTimeSheet[], Error>({
    queryKey: [ApiQueryKeys.TIME_SHEETS_SEARCH, "mine", me.id, filters],
    queryFn: async (): Promise<CsmTimeSheet[]> => {
      if (!me.id) return [];
      const all = await searchTimeCards(api, filters);
      return groupIntoSheets(
        all.filter((c) => c.userId === me.id),
        me.id,
        me.name,
      );
    },
    enabled: !!me.id && !!filters?.projectIds?.length,
    staleTime: 5_000,
  });
}

/**
 * Other engineers' sheets containing a card awaiting the signed-in
 * approver's decision. There's no server-side "approval queue" endpoint or
 * engineer filter, so this fetches submitted cards (scoped by `filters`) and
 * excludes the signed-in user's own on the client. Like {@link useMyTimeSheets},
 * this requires `filters.projectIds` to be non-empty to get anything back.
 */
export function useApprovalQueue(
  enabled: boolean,
  filters?: TimeCardSearchFilters,
): UseQueryResult<CsmTimeSheet[], Error> {
  const api = useBackendApi();
  const me = useCurrentEngineer();
  return useQuery<CsmTimeSheet[], Error>({
    queryKey: [ApiQueryKeys.TIME_CARD_APPROVAL_QUEUE, me.id, filters],
    queryFn: async (): Promise<CsmTimeSheet[]> => {
      if (!me.id) return [];
      const submitted = await searchTimeCards(api, {
        ...filters,
        states: ["submitted"],
      });
      const others = submitted.filter((c) => c.userId !== me.id);
      const byUser = new Map<string, CsmTimeCard[]>();
      for (const c of others) {
        const bucket = byUser.get(c.userId);
        if (bucket) bucket.push(c);
        else byUser.set(c.userId, [c]);
      }
      return [...byUser.entries()].flatMap(([userId, cards]) =>
        groupIntoSheets(cards, userId, cards[0]?.userName ?? "—"),
      );
    },
    enabled: enabled && !!me.id && !!filters?.projectIds?.length,
    staleTime: 5_000,
  });
}

/** Approve or reject a single card. */
export function useDecideCard(): UseMutationResult<
  CsmTimeCard,
  Error,
  TimeCardDecisionInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();
  return useMutation<CsmTimeCard, Error, TimeCardDecisionInput>({
    mutationFn: async (decision): Promise<CsmTimeCard> => {
      const payload: BeUpdateTimeCardPayload = {
        state: decision.state,
        ...(decision.leadComment ? { leadComment: decision.leadComment } : {}),
      };
      const res = await api.patch<BeUpdateTimeCardPayload, BeTimeCardMutationResponse>(
        `/time-cards/${encodeURIComponent(decision.cardId)}`,
        payload,
      );
      return mapTimeCard(res.timeCard);
    },
    onSuccess: () => invalidateTimecards(queryClient),
  });
}
