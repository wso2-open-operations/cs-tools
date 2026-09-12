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
// earlier FE-first mock are not available here. Bulk approve (see
// `useBulkApproveCards` below) is a frontend-only approximation: N parallel
// calls to the same single-card PATCH, not a real batch request. Cards come
// back flat; any visual grouping (by case or by engineer) is done
// client-side in `TimeCardsTable`, not here — see `timeCardGrouping.ts`.
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
import { BackendApiError, useBackendApi, type BackendApi } from "@api/backend/client";
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

/** Invalidate every time-card query so all views refresh after a write. Also
 * drops the persisted {@link RecentApprover} cache (see
 * {@link clearPersistedRecentApprovers}) — this is the one existing hub
 * already called after every card create/decide/bulk-approve mutation, so
 * reusing it here (rather than wiring a create-only success handler into
 * `usePostTimeCard`, a different file) keeps this a same-file change. Decide
 * and bulk-approve don't actually change *who the signed-in engineer picked
 * as an approver*, so those two calls make this drop the cache one mutation
 * too many — an accepted, harmless over-invalidation (one extra background
 * refetch next dialog open) rather than a second invalidation path. */
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
  clearPersistedRecentApprovers();
}

/** Wire `issueComplexity` values the entity-service is confirmed to return
 * (see `IssueComplexity`) — anything else (an older/renamed value) maps to
 * `undefined` rather than a bogus cast, since the edit form's `<select>`
 * would otherwise silently show nothing selected for an unrecognized value. */
const KNOWN_ISSUE_COMPLEXITIES: readonly string[] = ["N/A", "Low", "Medium", "High"];

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
 *
 * `breakdown`/`issueComplexity` are only set when the wire response actually
 * carries the per-activity minute fields — confirmed live to be present on
 * every real card, but a defensive per-field fallback (rather than assuming
 * the whole group is always there) costs nothing and matches how the rest of
 * this mapper already treats every other optional wire field.
 */
export function mapTimeCard(v: BeTimeCardView): CsmTimeCard {
  const hasBreakdown =
    v.timeAnalyzing !== undefined ||
    v.timeSettingUp !== undefined ||
    v.timeReproducingDebugging !== undefined ||
    v.timeProvidingSolution !== undefined ||
    v.timePatching !== undefined;
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
    approvers: v.approvers,
    workLogComment: v.workLogComment ?? undefined,
    breakdown: hasBreakdown
      ? {
          analysisDebugging: v.timeAnalyzing ?? 0,
          reproduce: v.timeReproducingDebugging ?? 0,
          settingUp: v.timeSettingUp ?? 0,
          providingSolution: v.timeProvidingSolution ?? 0,
          answering: v.timePatching ?? 0,
        }
      : undefined,
    issueComplexity:
      v.issueComplexity && KNOWN_ISSUE_COMPLEXITIES.includes(v.issueComplexity)
        ? (v.issueComplexity as CsmTimeCard["issueComplexity"])
        : undefined,
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
 *
 * `sortBy` is an escape hatch for callers that need the server to select
 * (not just order) by a specific field before applying `pagination` — e.g.
 * {@link useRecentApprovers}, where the top `RECENT_APPROVERS_LOOKBACK`
 * cards must actually be the most recent by `workDate`, not just sorted
 * after the fact within whatever page the default order happened to return.
 */
export async function searchTimeCards(
  api: BackendApi,
  filters?: TimeCardSearchFilters,
  pagination?: TimeCardPagination,
  sortBy?: BeSearchTimeCardsPayload["sortBy"],
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
    ...(sortBy ? { sortBy } : {}),
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

/** A previously-selected approver, reduced from the signed-in engineer's own
 * recent time-card submissions (see {@link useRecentApprovers}) — deliberately
 * the same `{ id, name }` shape as {@link TimeCardApprover} since it feeds the
 * exact same approver-picker candidate rendering in `LogTimeCardDialog`. */
export interface RecentApprover {
  id: string;
  name: string;
}

/** How many distinct recent approvers {@link useRecentApprovers} surfaces —
 * enough to cover an engineer's usual handful of team leads without turning
 * the "before you type" list into a full roster. */
const RECENT_APPROVERS_MAX = 4;

/** How many of the signed-in engineer's own most-recent cards to look at when
 * deriving {@link RecentApprover}s — a small page is enough since the same
 * few approvers repeat constantly; this is "who did I pick before", not a
 * complete history. */
const RECENT_APPROVERS_LOOKBACK = 20;

/** localStorage key prefix for the persisted {@link RecentApprover} cache,
 * one entry per signed-in engineer id (see {@link recentApproversStorageKey})
 * so switching accounts on a shared browser profile never surfaces another
 * engineer's picks. Versioned so a future change to the stored shape can
 * invalidate old entries outright rather than needing a migration. */
const RECENT_APPROVERS_STORAGE_PREFIX = "csm-portal:time-cards:recent-approvers:v1:";

function recentApproversStorageKey(engineerId: string): string {
  return `${RECENT_APPROVERS_STORAGE_PREFIX}${engineerId}`;
}

/** Backstop TTL for the persisted cache, well beyond a normal work session —
 * this only matters when the *server-side* answer to "who did I pick before"
 * has drifted without this browser seeing a create/decide/bulk-approve
 * mutation to invalidate on (e.g. the same engineer logged a card from a
 * different device or browser profile). 24h bounds that drift to at most one
 * stale day without forcing a re-fetch on every single dialog open, which
 * would defeat the point of caching this at all. */
const RECENT_APPROVERS_TTL_MS = 24 * 60 * 60 * 1000;

interface PersistedRecentApprovers {
  approvers: RecentApprover[];
  cachedAt: number;
}

function isRecentApproverArray(value: unknown): value is RecentApprover[] {
  return (
    Array.isArray(value) &&
    value.every(
      (v) =>
        !!v &&
        typeof v === "object" &&
        typeof (v as RecentApprover).id === "string" &&
        typeof (v as RecentApprover).name === "string",
    )
  );
}

/** Reads the persisted {@link RecentApprover} list for `engineerId`, or
 * `undefined` if there's no entry, the entry is older than
 * {@link RECENT_APPROVERS_TTL_MS}, or the stored shape doesn't parse as
 * expected (e.g. a stale format from an older build, or storage tampered with
 * outside this app) — any of those is treated identically to a cache miss,
 * never a crash. Wrapped in try/catch: private browsing or a storage quota
 * error must fall back to the always-fetch behavior, not break the dialog. */
function readPersistedRecentApprovers(engineerId: string): RecentApprover[] | undefined {
  try {
    const raw = window.localStorage.getItem(recentApproversStorageKey(engineerId));
    if (!raw) return undefined;
    const parsed = JSON.parse(raw) as Partial<PersistedRecentApprovers> | null;
    if (
      !parsed ||
      typeof parsed.cachedAt !== "number" ||
      !isRecentApproverArray(parsed.approvers)
    ) {
      return undefined;
    }
    const now = Date.now();
    if (
      !Number.isFinite(parsed.cachedAt) ||
      parsed.cachedAt > now ||
      now - parsed.cachedAt > RECENT_APPROVERS_TTL_MS
    ) {
      return undefined;
    }
    return parsed.approvers;
  } catch {
    return undefined;
  }
}

function writePersistedRecentApprovers(engineerId: string, approvers: RecentApprover[]): void {
  try {
    const payload: PersistedRecentApprovers = { approvers, cachedAt: Date.now() };
    window.localStorage.setItem(recentApproversStorageKey(engineerId), JSON.stringify(payload));
  } catch {
    // Storage disabled/full/private browsing — nothing to persist, next
    // dialog open just falls back to fetching live, same as today.
  }
}

/** Drops every persisted {@link RecentApprover} entry (all engineer ids, not
 * just the signed-in one) — cheap and safe since at most a handful of
 * accounts ever share one browser profile, and this runs from
 * {@link invalidateTimecards}, which has no engineer id of its own to scope
 * a single-key removal to. */
export function clearPersistedRecentApprovers(): void {
  try {
    const keys: string[] = [];
    for (let i = 0; i < window.localStorage.length; i++) {
      const key = window.localStorage.key(i);
      if (key?.startsWith(RECENT_APPROVERS_STORAGE_PREFIX)) keys.push(key);
    }
    for (const key of keys) window.localStorage.removeItem(key);
  } catch {
    // Nothing persisted to begin with, or storage unavailable — no-op.
  }
}

/**
 * The signed-in engineer's own most-recently-submitted-to approvers, most
 * recent first, deduped by id, capped at {@link RECENT_APPROVERS_MAX} —
 * powers "Log time"'s approver picker showing previously-picked approvers
 * before the engineer types anything (digiops-cs#2839), instead of requiring
 * the same search every time.
 *
 * Derived entirely from the existing `POST /time-cards/search` endpoint
 * (`userId`-scoped to the signed-in engineer, no `states` filter — an
 * approved, rejected, or still-`submitted` card all equally answer "who did I
 * pick before") rather than a new backend endpoint or local storage: no state
 * restriction is needed here, this is about the approver pick, not the
 * outcome. `/time-cards/search`'s own default ordering isn't documented, so
 * results are explicitly re-sorted here by `workDate` descending rather than
 * assumed to already be newest-first.
 *
 * `enabled` should be gated to create-mode only (see `LogTimeCardDialog`) —
 * the approver field is read-only once editing, so there's nothing for this
 * list to feed there.
 *
 * Persisted to `localStorage` (see {@link readPersistedRecentApprovers} /
 * {@link writePersistedRecentApprovers}), keyed by the signed-in engineer's
 * id, so opening "Log time" doesn't re-run this search every single time —
 * this list only actually changes when the engineer submits a new card with
 * an approver (see {@link invalidateTimecards}, the sole invalidation path),
 * so a fresh-enough persisted entry is used as-is with no network call at
 * all, surviving page reloads and new sessions. A fresh cache is fed straight
 * back as `initialData`; only a cache miss or a stale entry lets the query
 * actually run, and every successful fetch re-persists its result.
 */
export function useRecentApprovers(enabled: boolean): UseQueryResult<RecentApprover[], Error> {
  const api = useBackendApi();
  const me = useCurrentEngineer();
  const cached = me.id ? readPersistedRecentApprovers(me.id) : undefined;
  return useQuery<RecentApprover[], Error>({
    queryKey: [ApiQueryKeys.TIME_SHEETS_SEARCH, "recent-approvers", me.id],
    queryFn: async (): Promise<RecentApprover[]> => {
      if (!me.id) return [];
      // sortBy: workDate desc so the page itself is the most-recent-by-workDate
      // cards — the server's default order (updatedOn desc) selects a
      // different, and possibly non-overlapping, top-N.
      const { cards } = await searchTimeCards(
        api,
        { userId: me.id },
        { page: 0, rowsPerPage: RECENT_APPROVERS_LOOKBACK },
        { field: "workDate", order: "desc" },
      );
      const newestFirst = [...cards].sort((a, b) =>
        (b.workDate || "").localeCompare(a.workDate || ""),
      );
      const seen = new Set<string>();
      const recents: RecentApprover[] = [];
      outer: for (const card of newestFirst) {
        for (const approver of card.approvers ?? []) {
          // Defensive only: nothing should let an engineer submit a card
          // approved by themself, mirroring the same self-exclusion already
          // applied to live search candidates in LogTimeCardDialog.
          if (approver.id === me.id || seen.has(approver.id)) continue;
          seen.add(approver.id);
          recents.push({ id: approver.id, name: approver.name });
          if (recents.length >= RECENT_APPROVERS_MAX) break outer;
        }
      }
      writePersistedRecentApprovers(me.id, recents);
      return recents;
    },
    enabled: enabled && !!me.id && !cached,
    initialData: cached,
    staleTime: 30_000,
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

/** Outcome of {@link useBulkApproveCards}: which cards actually got approved
 * versus which ones failed, each with its own reason — a bulk request is
 * fanned out as N independent single-card PATCHes (see the module doc
 * comment: the backend has no batch endpoint), so a partial result, not just
 * all-or-nothing, is the normal case. E.g. another approver decides one of
 * the selected cards first, or the caller turns out not to be an eligible
 * approver for one specific card (see `useDecideCard`'s own note on this same
 * 403 behavior). */
export interface BulkApproveResult {
  succeededIds: string[];
  failed: { cardId: string; message: string }[];
}

/**
 * Approve several cards at once. Fires one `PATCH /time-cards/{id}` per card
 * in parallel (`Promise.allSettled`, never rejecting the mutation itself, so
 * a failure on one card never stops the others from completing) and
 * aggregates the outcome into {@link BulkApproveResult} for the caller to
 * report. Approve only, not reject — a bulk reject would need a per-card
 * reason (the backend requires a non-empty `leadComment` to reject, see
 * `TimeCardReviewDialog`), which doesn't multiplex across an arbitrary
 * selection the way a single "approved" state does.
 */
export function useBulkApproveCards(): UseMutationResult<
  BulkApproveResult,
  Error,
  string[]
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();
  return useMutation<BulkApproveResult, Error, string[]>({
    mutationFn: async (cardIds): Promise<BulkApproveResult> => {
      const payload: BeUpdateTimeCardPayload = { state: "approved" };
      const outcomes = await Promise.allSettled(
        cardIds.map((cardId) =>
          api.patch<BeUpdateTimeCardPayload, BeTimeCardMutationResponse>(
            `/time-cards/${encodeURIComponent(cardId)}`,
            payload,
          ),
        ),
      );
      const result: BulkApproveResult = { succeededIds: [], failed: [] };
      outcomes.forEach((outcome, i) => {
        const cardId = cardIds[i];
        if (outcome.status === "fulfilled") {
          result.succeededIds.push(cardId);
        } else {
          const err = outcome.reason;
          const message =
            err instanceof BackendApiError && err.status < 500 && err.message
              ? err.message
              : "Could not approve this time card.";
          result.failed.push({ cardId, message });
        }
      });
      return result;
    },
    // Some cards may have been approved even when others in the same batch
    // failed, so every view still needs a refresh regardless of `failed`'s
    // length -- this only runs when mutationFn itself doesn't throw, which
    // (per Promise.allSettled above) it never does.
    onSuccess: () => invalidateTimecards(queryClient),
  });
}
