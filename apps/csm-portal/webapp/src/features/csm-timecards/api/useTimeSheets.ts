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
} from "@features/csm-timecards/types/timeCards";
import { groupIntoSheets } from "@features/csm-timecards/utils/timeSheetGrouping";

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
    ApiQueryKeys.TIME_CARD_ALL,
  ]) {
    void queryClient.invalidateQueries({ queryKey: [key] });
  }
}

/**
 * Map the backend's `TimeCardView` to the portal's `CsmTimeCard`. `totalTime`
 * is already whole minutes on the wire (see `usePostTimeCard`'s note on why),
 * which is also the unit the portal displays throughout — a direct
 * passthrough, no conversion needed.
 */
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
    totalMinutes: v.totalTime,
    approvedById: v.approvedBy?.id,
    approvedByName: v.approvedBy?.name,
  };
}

/** Zero-indexed page + page size, mirroring MUI `TablePagination`'s own
 * `page`/`rowsPerPage` convention (see `CsmTimeCardsPage.tsx`). */
export interface TimeCardPagination {
  page: number;
  rowsPerPage: number;
}

/** Result of {@link searchTimeCards}: the cards on the requested page, and
 * `total` — the backend's count for the whole scope (all states, all
 * cards), *not* the count after `filters.states` client-side filtering
 * (see the note on that below) — so a caller filtering further should treat
 * `total` as "how big is the underlying page-able scope", not "how many of
 * these match my filter". */
export interface TimeCardSearchResult {
  cards: CsmTimeCard[];
  total: number;
}

/**
 * Search against `POST /time-cards/search`, fetching exactly the requested
 * page (defaults to the first `BE_MAX_PAGE_LIMIT` cards when `pagination`
 * is omitted). `projectIds`, `userId`, `approverId`, and `from`/`to` are all
 * real server-side filters, confirmed live (not just by reading
 * `entity-service`'s `sn_time_card_service.go` — see the note below on
 * `caseId`, which looked equally real in code but isn't). Callers should
 * scope as precisely as possible at the source (see {@link useMyTimeSheets},
 * {@link useApprovalQueue} below) rather than fetching a broad page and
 * filtering client-side — the latter makes `total` and the page's contents
 * diverge from what's actually shown. `limit` is capped at
 * `BE_MAX_PAGE_LIMIT` — the backend rejects anything above that with a
 * generic 400 despite the OpenAPI spec documenting up to 100 (confirmed
 * live).
 *
 * `filters.caseId` is deliberately *not* forwarded, unlike the others,
 * despite existing in `openapi.yaml` and being genuinely implemented
 * end-to-end in `entity-service` — confirmed live to be non-functional: a
 * search scoped only by a case's own id returns `total: 0` unconditionally,
 * even though the exact same cards are provably present and correctly
 * tagged with that `case.id` when the search is instead scoped by
 * `projectIds` (see {@link useCaseTimeCards} in `useTimeCards.ts`, which
 * scopes by project and filters to the case client-side instead). Don't
 * re-add it without re-confirming live first.
 *
 * This used to page through the *entire* scope internally (up to 1,000
 * cards) before returning, so every view always had "everything". That was
 * confirmed live to take 30-60+ seconds on a few-hundred-record scope
 * (sequential page-by-page requests), and — with three tabs each doing
 * their own such walk — was slow enough to sometimes fail outright. Real,
 * caller-driven pagination replaces that: callers ask for one page at a
 * time via `pagination`, and `total` lets them drive page controls.
 *
 * `filters.states` is deliberately filtered client-side, never sent as
 * `filters.states` on the wire: confirmed live that the backend 500s
 * ("Failed to search time cards.") when `states` is combined with a large
 * `projectIds` array (reproduced: 2 or 10 project ids + `states` → 200; the
 * full ~88-project default scope + `states` → 500; that same full scope
 * *without* `states` → 200). Since {@link useApprovalQueue} always needs a
 * "submitted" filter and still defaults `projectIds` to every visible
 * project (alongside its more precise `approverId` scope), sending `states`
 * server-side would risk the same 500 for any user with enough visible
 * projects — worse than not filtering at all. One consequence of filtering
 * client-side over a single server page: a page can legitimately come back
 * with fewer matching cards than `rowsPerPage` (or none at all) even though
 * `total` says there's more data — the states filter just happened to
 * exclude most/all of that particular page.
 */
export async function searchTimeCards(
  api: BackendApi,
  filters?: TimeCardSearchFilters,
  pagination?: TimeCardPagination,
): Promise<TimeCardSearchResult> {
  const limit = Math.min(pagination?.rowsPerPage ?? BE_MAX_PAGE_LIMIT, BE_MAX_PAGE_LIMIT);
  const payload: BeSearchTimeCardsPayload = {
    filters: {
      ...(filters?.projectIds?.length ? { projectIds: filters.projectIds } : {}),
      ...(filters?.userId ? { userId: filters.userId } : {}),
      ...(filters?.approverId ? { approverId: filters.approverId } : {}),
      ...(filters?.from ? { startDate: filters.from } : {}),
      ...(filters?.to ? { endDate: filters.to } : {}),
    },
    pagination: { limit, offset: (pagination?.page ?? 0) * limit },
  };
  const res = await api.post<BeSearchTimeCardsPayload, BeSearchTimeCardsResponse>(
    "/time-cards/search",
    payload,
  );
  const cards = (res.timeCards ?? []).map(mapTimeCard);
  return {
    cards: filters?.states?.length
      ? cards.filter((c) => (filters.states as BeTimeCardState[]).includes(c.state))
      : cards,
    total: res.total,
  };
}

/** Weekly sheets on the requested page, plus `total` — see the note on
 * {@link TimeCardSearchResult}, the same caveat about client-side-filtered
 * scopes applies here. */
export interface TimeSheetsResult {
  sheets: CsmTimeSheet[];
  total: number;
}

/** Groups cards by `userId` into per-user weekly sheets, newest first —
 * shared by {@link useApprovalQueue} and {@link useAllTimeCards}, which only
 * differ in which cards they pass in (others' submitted cards vs everyone's). */
function groupCardsByUserIntoSheets(cards: CsmTimeCard[]): CsmTimeSheet[] {
  const byUser = new Map<string, CsmTimeCard[]>();
  for (const c of cards) {
    const bucket = byUser.get(c.userId);
    if (bucket) bucket.push(c);
    else byUser.set(c.userId, [c]);
  }
  return [...byUser.entries()].flatMap(([userId, userCards]) =>
    groupIntoSheets(userCards, userId, userCards[0]?.userName ?? "—"),
  );
}

/**
 * The signed-in user's own cards on the requested page, grouped into weekly
 * sheets, newest first. Scoped server-side by `userId` (on top of the
 * existing `projectIds` default-scope) so `total` and the fetched page both
 * reflect just this user's cards — paginating over a project-wide page and
 * filtering to "mine" client-side would make `total` and the page size
 * meaningless here (a page of 20 project-wide cards could easily contain
 * none of the signed-in user's own). No project scope is required: with no
 * project filter picked the search runs unscoped and the backend returns
 * every card the caller is entitled to, bounded here to the signed-in user
 * by `userId` (see `CsmTimeCardsPage.tsx`).
 *
 * `enabled` should be gated on the owning tab actually being active (see
 * `CsmTimeCardsPage.tsx`) — confirmed live: with this, {@link useAllTimeCards}
 * and {@link useApprovalQueue} all fetching eagerly regardless of which tab
 * is shown was enough concurrent load to make some fail outright or never
 * settle.
 */
export function useMyTimeSheets(
  enabled: boolean,
  filters: TimeCardSearchFilters | undefined,
  pagination: TimeCardPagination,
): UseQueryResult<TimeSheetsResult, Error> {
  const api = useBackendApi();
  const me = useCurrentEngineer();
  return useQuery<TimeSheetsResult, Error>({
    queryKey: [ApiQueryKeys.TIME_SHEETS_SEARCH, "mine", me.id, filters, pagination],
    queryFn: async (): Promise<TimeSheetsResult> => {
      if (!me.id) return { sheets: [], total: 0 };
      const { cards, total } = await searchTimeCards(
        api,
        { ...filters, userId: me.id },
        pagination,
      );
      return { sheets: groupIntoSheets(cards, me.id, me.name), total };
    },
    enabled: enabled && !!me.id,
    staleTime: 5_000,
  });
}

/**
 * Other engineers' sheets on the requested page containing a card awaiting
 * the signed-in approver's decision. Scoped server-side by `approverId` (on
 * top of the existing `projectIds` default-scope) so the queue only ever
 * contains cards the signed-in user is actually eligible to decide —
 * previously this fetched every submitted card in scope regardless of who
 * could approve it, so clicking Approve/Reject on a card the viewer wasn't
 * the assigned approver for 403'd. Still excludes the signed-in user's own
 * cards client-side as a defense-in-depth self-approval guard: a card
 * created before `LogTimeCardDialog` started excluding the submitter from
 * the approver picker could still name themselves as an eligible approver.
 * Like {@link useMyTimeSheets}, no project scope is required (the search runs
 * unscoped when no project filter is picked), and `enabled` should be gated on
 * this tab actually being active (see the note on {@link useMyTimeSheets}).
 */
export function useApprovalQueue(
  enabled: boolean,
  filters: TimeCardSearchFilters | undefined,
  pagination: TimeCardPagination,
): UseQueryResult<TimeSheetsResult, Error> {
  const api = useBackendApi();
  const me = useCurrentEngineer();
  return useQuery<TimeSheetsResult, Error>({
    queryKey: [ApiQueryKeys.TIME_CARD_APPROVAL_QUEUE, me.id, filters, pagination],
    queryFn: async (): Promise<TimeSheetsResult> => {
      if (!me.id) return { sheets: [], total: 0 };
      const { cards, total } = await searchTimeCards(
        api,
        { ...filters, approverId: me.id, states: ["submitted"] },
        pagination,
      );
      const others = cards.filter((c) => c.userId !== me.id);
      return { sheets: groupCardsByUserIntoSheets(others), total };
    },
    enabled: enabled && !!me.id,
    staleTime: 5_000,
  });
}

/**
 * Every visible user's sheets on the requested page, own included — unlike
 * {@link useMyTimeSheets} (self only) and {@link useApprovalQueue} (others'
 * submitted cards only, for deciding), this is a read-only "see everything"
 * view: no state restriction, no ownership exclusion. Runs unscoped when no
 * project filter is picked (the backend returns the caller's full
 * entitlement), and `enabled` should be gated on this tab actually being
 * active (see the note on {@link useMyTimeSheets} for why).
 */
export function useAllTimeCards(
  enabled: boolean,
  filters: TimeCardSearchFilters | undefined,
  pagination: TimeCardPagination,
): UseQueryResult<TimeSheetsResult, Error> {
  const api = useBackendApi();
  const me = useCurrentEngineer();
  return useQuery<TimeSheetsResult, Error>({
    queryKey: [ApiQueryKeys.TIME_CARD_ALL, filters, pagination],
    queryFn: async (): Promise<TimeSheetsResult> => {
      const { cards, total } = await searchTimeCards(api, filters, pagination);
      return { sheets: groupCardsByUserIntoSheets(cards), total };
    },
    enabled: enabled && !!me.id,
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
