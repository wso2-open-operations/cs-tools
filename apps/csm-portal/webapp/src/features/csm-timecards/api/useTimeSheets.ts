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
// React Query hooks for time cards and approvals, backed by the real
// csm-portal-backend endpoints:
//
//   POST  /time-cards/search   my cards / all cards / approval queue
//   PATCH /time-cards/{id}     approve / reject a card
//
// The backend has no bulk (approve/reject a whole batch) endpoint, no
// delegation, and no reports/aggregates endpoint — those features from the
// earlier FE-first mock are not available here. Cards come back flat; any
// visual grouping (by case or by engineer) is done client-side in
// `TimeCardsTable`, not here — see `timeCardGrouping.ts`.
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
  BeSearchTimeCardsFilters,
  BeSearchTimeCardsPayload,
  BeSearchTimeCardsResponse,
  BeTimeCardState,
  BeTimeCardView,
  BeUpdateTimeCardPayload,
  BeTimeCardMutationResponse,
} from "@api/backend/types";
import type {
  CsmTimeCard,
  TimeCardDecisionInput,
  TimeCardSearchFilters,
} from "@features/csm-timecards/types/timeCards";

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
 *
 * `workDate` falls back to the deprecated `createdOn` so a backend that hasn't
 * rolled out the new field yet still yields a usable date — the two currently
 * read the same underlying value, so the fallback is a no-op in practice.
 * `approvedBy` / `rejectionReason` are mutually exclusive: only one of them is
 * ever populated (see {@link CsmTimeCard}).
 */
export function mapTimeCard(v: BeTimeCardView): CsmTimeCard {
  return {
    id: v.id,
    caseId: v.case?.id ?? "",
    caseNumber: v.case?.number || v.case?.name || "—",
    projectId: v.project?.id ?? "",
    projectName: v.project?.name ?? "—",
    workDate: v.workDate || v.createdOn,
    userId: v.user?.id ?? "",
    userName: v.user?.name ?? "—",
    state: v.state,
    billable: v.hasBillable,
    totalMinutes: v.totalTime,
    approvedById: v.approvedBy?.id,
    approvedByName: v.approvedBy?.name,
    rejectionReason: v.rejectionReason ?? undefined,
  };
}

/** Zero-indexed page + page size, mirroring MUI `TablePagination`'s own
 * `page`/`rowsPerPage` convention (see `CsmTimeCardsPage.tsx`). */
export interface TimeCardPagination {
  page: number;
  rowsPerPage: number;
}

/** Result of {@link searchTimeCards}: the cards on the requested page, and
 * `total` — the backend's count for the whole (filtered) scope, driving
 * `TablePagination`'s `count` directly. */
export interface TimeCardSearchResult {
  cards: CsmTimeCard[];
  total: number;
}

/**
 * Search against `POST /time-cards/search`, fetching exactly the requested
 * page (defaults to the first `BE_MAX_PAGE_LIMIT` cards when `pagination` is
 * omitted). `projectIds`, `caseId`, `userId`/`userIds`, `approverId`,
 * `states`, and `from`/`to` are all real server-side filters — every one of
 * them is forwarded on the wire and the response is returned as-is, with no
 * client-side re-filtering. Callers should still scope as precisely as
 * possible at the source (see {@link useMyTimeCards}, {@link useApprovalQueue}
 * below) so `total` and the page's contents reflect exactly what's shown.
 * `limit` is capped at `BE_MAX_PAGE_LIMIT` — the backend rejects anything
 * above that with a generic 400 despite the OpenAPI spec documenting up to
 * 100 (confirmed live).
 *
 * `caseId` and `states` were previously avoided here — `caseId` alone used to
 * return `total: 0` unconditionally, and `states` combined with a large
 * `projectIds` scope used to 500 — both fixed upstream on the entity-service
 * data source (see PR #1133); this now forwards them directly instead of the
 * removed caseId-via-project / states-client-filter-and-walk workarounds.
 *
 * This used to page through the *entire* scope internally (up to 1,000
 * cards) before returning, so every view always had "everything". That was
 * confirmed live to take 30-60+ seconds on a few-hundred-record scope
 * (sequential page-by-page requests), and — with three tabs each doing
 * their own such walk — was slow enough to sometimes fail outright. Real,
 * caller-driven pagination replaces that: callers ask for one page at a
 * time via `pagination`, and `total` lets them drive page controls.
 */
export async function searchTimeCards(
  api: BackendApi,
  filters?: TimeCardSearchFilters,
  pagination?: TimeCardPagination,
): Promise<TimeCardSearchResult> {
  const limit = Math.min(pagination?.rowsPerPage ?? BE_MAX_PAGE_LIMIT, BE_MAX_PAGE_LIMIT);
  const wireFilters: BeSearchTimeCardsFilters = {
    ...(filters?.caseId ? { caseId: filters.caseId } : {}),
    ...(filters?.projectIds?.length ? { projectIds: filters.projectIds } : {}),
    ...(filters?.userId ? { userId: filters.userId } : {}),
    ...(filters?.userIds?.length ? { userIds: filters.userIds } : {}),
    ...(filters?.approverId ? { approverId: filters.approverId } : {}),
    ...(filters?.states?.length ? { states: filters.states as BeTimeCardState[] } : {}),
    ...(filters?.from ? { startDate: filters.from } : {}),
    ...(filters?.to ? { endDate: filters.to } : {}),
  };
  const payload: BeSearchTimeCardsPayload = {
    filters: wireFilters,
    pagination: { limit, offset: (pagination?.page ?? 0) * limit },
  };
  const res = await api.post<BeSearchTimeCardsPayload, BeSearchTimeCardsResponse>(
    "/time-cards/search",
    payload,
  );
  return { cards: (res.timeCards ?? []).map(mapTimeCard), total: res.total };
}

/**
 * The signed-in user's own cards on the requested page, newest work date
 * first — grouping (by case or by engineer) is a display concern now, done in
 * `TimeCardsTable`, not here. Scoped server-side by `userId` (on top of the
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
export function useMyTimeCards(
  enabled: boolean,
  filters: TimeCardSearchFilters | undefined,
  pagination: TimeCardPagination,
): UseQueryResult<TimeCardSearchResult, Error> {
  const api = useBackendApi();
  const me = useCurrentEngineer();
  return useQuery<TimeCardSearchResult, Error>({
    queryKey: [ApiQueryKeys.TIME_SHEETS_SEARCH, "mine", me.id, filters, pagination],
    queryFn: async (): Promise<TimeCardSearchResult> => {
      if (!me.id) return { cards: [], total: 0 };
      return searchTimeCards(api, { ...filters, userId: me.id }, pagination);
    },
    enabled: enabled && !!me.id,
    staleTime: 5_000,
  });
}

/**
 * Other engineers' cards on the requested page awaiting the signed-in
 * approver's decision. Scoped server-side by `approverId` (on top of the
 * existing `projectIds` default-scope) so the queue only ever contains cards
 * the signed-in user is actually eligible to decide — previously this
 * fetched every submitted card in scope regardless of who could approve it,
 * so clicking Approve/Reject on a card the viewer wasn't the assigned
 * approver for 403'd. `approverId` alone is sufficient for self-exclusion
 * too: the backend excludes the caller's own cards from an
 * `approverId`-scoped search unconditionally, so no client-side "exclude
 * myself" filtering (or an `excludeUserId` wire field) is needed here. Like
 * {@link useMyTimeCards}, no project scope is required (the search runs
 * unscoped when no project filter is picked), and `enabled` should be gated on
 * this tab actually being active (see the note on {@link useMyTimeCards}).
 */
export function useApprovalQueue(
  enabled: boolean,
  filters: TimeCardSearchFilters | undefined,
  pagination: TimeCardPagination,
): UseQueryResult<TimeCardSearchResult, Error> {
  const api = useBackendApi();
  const me = useCurrentEngineer();
  return useQuery<TimeCardSearchResult, Error>({
    queryKey: [ApiQueryKeys.TIME_CARD_APPROVAL_QUEUE, me.id, filters, pagination],
    queryFn: async (): Promise<TimeCardSearchResult> => {
      if (!me.id) return { cards: [], total: 0 };
      return searchTimeCards(api, { ...filters, approverId: me.id, states: ["submitted"] }, pagination);
    },
    enabled: enabled && !!me.id,
    staleTime: 5_000,
  });
}

/**
 * Every visible user's cards on the requested page, own included — unlike
 * {@link useMyTimeCards} (self only) and {@link useApprovalQueue} (others'
 * submitted cards only, for deciding), this is a read-only "see everything"
 * view: no state restriction, no ownership exclusion. Runs unscoped when no
 * project filter is picked (the backend returns the caller's full
 * entitlement), and `enabled` should be gated on this tab actually being
 * active (see the note on {@link useMyTimeCards} for why).
 */
export function useAllTimeCards(
  enabled: boolean,
  filters: TimeCardSearchFilters | undefined,
  pagination: TimeCardPagination,
): UseQueryResult<TimeCardSearchResult, Error> {
  const api = useBackendApi();
  const me = useCurrentEngineer();
  return useQuery<TimeCardSearchResult, Error>({
    queryKey: [ApiQueryKeys.TIME_CARD_ALL, filters, pagination],
    queryFn: (): Promise<TimeCardSearchResult> => searchTimeCards(api, filters, pagination),
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
