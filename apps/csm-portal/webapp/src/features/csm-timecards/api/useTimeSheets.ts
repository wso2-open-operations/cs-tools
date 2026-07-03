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
 * uses across all services, and the same identifier `card.userId` compares
 * against. Display name is built from firstName + lastName returned by the
 * entity service, falling back to ID-token values while the query is in
 * flight. `id` is `undefined` until the real UUID resolves — deliberately
 * *not* falling back to email: the ID-token email is available immediately,
 * so an email fallback would make the "wait for a real id" gate never
 * actually wait, and "my cards" filtering (`card.userId === id`) would
 * compare a UUID against an email and match nothing until `GET /users/me`
 * resolves (permanently, if it errors).
 */
export function useCurrentEngineer(): { id: string | undefined; name: string } {
  const { data: me } = useGetUsersMe();
  const info = resolveUserInfo(useIdTokenClaims());
  const displayName =
    [me?.firstName, me?.lastName].filter(Boolean).join(" ") || info.fullName;
  return { id: me?.id, name: displayName };
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
 * Page cap for {@link searchTimeCards} — `BE_MAX_PAGE_LIMIT` (50) per page,
 * so 20 pages covers up to 1,000 cards per scoped search before giving up
 * and reporting `truncated: true`. Mirrors the same bounded-pagination
 * shape as `projectOptionsQueryOptions` in `useProjectOptions.ts`.
 */
const TIME_CARDS_MAX_PAGES = 20;

/** Result of {@link searchTimeCards}: the cards found, and whether the
 * `TIME_CARDS_MAX_PAGES` safety cap was hit before the results ran out (in
 * which case some cards in scope were not fetched). */
export interface TimeCardSearchResult {
  cards: CsmTimeCard[];
  truncated: boolean;
}

/**
 * Search against `POST /time-cards/search`, paginating through every page
 * up to {@link TIME_CARDS_MAX_PAGES}. The backend only supports
 * `projectIds` server-side (also `startDate`/`endDate`, but nothing in the
 * UI exposes a date-range filter, so this never sends them) — there is no
 * `caseId` or `engineerId` filter, so case- and user-scoping happen
 * client-side over the fetched cards (see {@link useCaseTimeCards},
 * {@link useMyTimeSheets}, {@link useApprovalQueue} below). `limit` per
 * page is capped at `BE_MAX_PAGE_LIMIT` — the backend rejects anything
 * above that with a generic 400 despite the OpenAPI spec documenting up to
 * 100 (confirmed live).
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
): Promise<TimeCardSearchResult> {
  const baseFilters = {
    ...(filters?.projectIds?.length ? { projectIds: filters.projectIds } : {}),
  };

  const all: BeTimeCardView[] = [];
  let truncated = false;
  let offset = 0;
  for (let page = 0; page < TIME_CARDS_MAX_PAGES; page += 1) {
    const payload: BeSearchTimeCardsPayload = {
      filters: baseFilters,
      pagination: { limit: BE_MAX_PAGE_LIMIT, offset },
    };
    let res: BeSearchTimeCardsResponse;
    try {
      res = await api.post<BeSearchTimeCardsPayload, BeSearchTimeCardsResponse>(
        "/time-cards/search",
        payload,
      );
    } catch (err) {
      // The first page failing is a real error — surface it as before (empty
      // scope, auth issue, etc). A *later* page failing is different, and
      // confirmed live: the backend parses a whole page's worth of records
      // in one shot, so a single malformed record (e.g. a non-numeric
      // totalTime) fails the *entire* page, discarding every valid record
      // alongside it. Reported upstream (correlation IDs on file) — until
      // it's fixed there, degrade to what was already fetched rather than
      // losing an otherwise-successful search to one bad record several
      // pages in.
      if (page === 0) throw err;
      truncated = true;
      break;
    }
    const pageCards = res.timeCards ?? [];
    all.push(...pageCards);
    if (pageCards.length < BE_MAX_PAGE_LIMIT) break;
    offset += BE_MAX_PAGE_LIMIT;
    if (page === TIME_CARDS_MAX_PAGES - 1) truncated = true;
  }

  const cards = all.map(mapTimeCard);
  return {
    cards: filters?.states?.length
      ? cards.filter((c) => (filters.states as BeTimeCardState[]).includes(c.state))
      : cards,
    truncated,
  };
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

/** Weekly sheets plus whether {@link searchTimeCards} hit its page cap —
 * i.e. some cards in scope weren't fetched and aren't reflected below. */
export interface TimeSheetsResult {
  sheets: CsmTimeSheet[];
  truncated: boolean;
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
): UseQueryResult<TimeSheetsResult, Error> {
  const api = useBackendApi();
  const me = useCurrentEngineer();
  return useQuery<TimeSheetsResult, Error>({
    queryKey: [ApiQueryKeys.TIME_SHEETS_SEARCH, "mine", me.id, filters],
    queryFn: async (): Promise<TimeSheetsResult> => {
      if (!me.id) return { sheets: [], truncated: false };
      const { cards, truncated } = await searchTimeCards(api, filters);
      return {
        sheets: groupIntoSheets(
          cards.filter((c) => c.userId === me.id),
          me.id,
          me.name,
        ),
        truncated,
      };
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
): UseQueryResult<TimeSheetsResult, Error> {
  const api = useBackendApi();
  const me = useCurrentEngineer();
  return useQuery<TimeSheetsResult, Error>({
    queryKey: [ApiQueryKeys.TIME_CARD_APPROVAL_QUEUE, me.id, filters],
    queryFn: async (): Promise<TimeSheetsResult> => {
      if (!me.id) return { sheets: [], truncated: false };
      const { cards, truncated } = await searchTimeCards(api, {
        ...filters,
        states: ["submitted"],
      });
      const others = cards.filter((c) => c.userId !== me.id);
      const byUser = new Map<string, CsmTimeCard[]>();
      for (const c of others) {
        const bucket = byUser.get(c.userId);
        if (bucket) bucket.push(c);
        else byUser.set(c.userId, [c]);
      }
      return {
        sheets: [...byUser.entries()].flatMap(([userId, userCards]) =>
          groupIntoSheets(userCards, userId, userCards[0]?.userName ?? "—"),
        ),
        truncated,
      };
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
