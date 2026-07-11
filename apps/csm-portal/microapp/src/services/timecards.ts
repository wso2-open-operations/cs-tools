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

// React Query options + mutations for weekly time sheets and approvals, backed
// by the real csm-portal-backend endpoints:
//
//   POST  /time-cards/search   my sheets / all / approval queue (paged, client-grouped into weeks)
//   PATCH /time-cards/{id}     approve / reject a card
//
// The backend has no "sheet" concept and no bulk endpoint — weekly sheets are a
// pure client-side grouping of individual cards. Time is in whole MINUTES.
//
// Pagination: the backend caps a page at TIME_CARD_MAX_PAGE_LIMIT (50) cards, so
// each tab uses an infinite query that pages by offset (the mobile equivalent of
// the webapp's page/rowsPerPage controls). Cards from every loaded page are
// flattened, then grouped into weekly sheets in one pass — so a week that spans a
// page boundary stays a single sheet, rather than being split per page.

import { infiniteQueryOptions, type InfiniteData } from "@tanstack/react-query";
import { TIME_CARDS_SEARCH_ENDPOINT, TIME_CARD_ENDPOINT } from "@config/endpoints";
import type {
  BeSearchTimeCardsPayload,
  BeSearchTimeCardsResponse,
  BeTimeCardMutationResponse,
  BeTimeCardState,
  BeUpdateTimeCardPayload,
  CsmTimeCard,
  CsmTimeSheet,
  TimeCardDecisionInput,
} from "@src/types";
import { toTimeCard } from "@src/types";
import {
  EMPTY_TIMECARD_FILTERS,
  groupCardsByUserIntoSheets,
  groupIntoSheets,
  TIME_CARD_MAX_PAGE_LIMIT,
  type EngineerOption,
  type TimeCardFilters,
} from "@utils/timecard";
import apiClient from "./apiClient";

// Server-side filters honored by the search endpoint. `states` is filtered
// client-side (combining it with a large `projectIds` array 500s the backend),
// and there is deliberately no `caseId` (non-functional live — see the DTO).
interface TimeCardSearchFilters {
  projectIds?: string[];
  userId?: string;
  approverId?: string;
  states?: BeTimeCardState[];
  from?: string;
  to?: string;
}

// One page of cards, plus what the infinite query needs to fetch the next one.
interface TimeCardPage {
  cards: CsmTimeCard[];
  total: number;
  /** Offset to request for the following page. */
  nextOffset: number;
  hasMore: boolean;
}

// The grouped, paginated view a tab renders: weekly sheets built from every
// loaded page, `total` (the backend's count for the whole scope), `loaded`
// (cards fetched so far — drives the "X of Y" progress line), and the option
// lists for the client-side work-item / engineer filters (derived from the
// loaded cards, before those two filters are applied).
export interface TimeSheetsView {
  sheets: CsmTimeSheet[];
  total: number;
  loaded: number;
  availableWorkItems: string[];
  availableEngineers: EngineerOption[];
}

const EMPTY_PAGE: TimeCardPage = { cards: [], total: 0, nextOffset: 0, hasMore: false };

// Fetch one page of cards from `POST /time-cards/search` at `offset`. Scope as
// precisely as possible at the source (userId / approverId) rather than fetching
// broadly and filtering client-side. `limit` is capped at TIME_CARD_MAX_PAGE_LIMIT.
async function searchTimeCardsPage(filters: TimeCardSearchFilters, offset: number): Promise<TimeCardPage> {
  const payload: BeSearchTimeCardsPayload = {
    filters: {
      ...(filters.projectIds?.length ? { projectIds: filters.projectIds } : {}),
      ...(filters.userId ? { userId: filters.userId } : {}),
      ...(filters.approverId ? { approverId: filters.approverId } : {}),
      ...(filters.from ? { startDate: filters.from } : {}),
      ...(filters.to ? { endDate: filters.to } : {}),
    },
    pagination: { limit: TIME_CARD_MAX_PAGE_LIMIT, offset },
  };
  const { data } = await apiClient.post<BeSearchTimeCardsResponse>(TIME_CARDS_SEARCH_ENDPOINT, payload);
  const raw = (data.timeCards ?? []).map(toTimeCard);
  // Advance the cursor by the *raw* page size (before the client-side `states`
  // filter), so paging stays aligned with the backend's own offset even when the
  // filter drops cards from a page.
  const nextOffset = data.offset + raw.length;
  const cards = filters.states?.length ? raw.filter((c) => filters.states!.includes(c.state)) : raw;
  return { cards, total: data.total, nextOffset, hasMore: raw.length > 0 && nextOffset < data.total };
}

// Flatten every loaded page's cards, derive the client-filter option lists from
// them, apply the client-side work-item / engineer filters, then group what's
// left once. `group` builds the weekly sheets appropriately for the tab (own
// week grouping vs per-user).
function toView(
  data: InfiniteData<TimeCardPage, number>,
  filters: TimeCardFilters,
  group: (cards: CsmTimeCard[]) => CsmTimeSheet[],
): TimeSheetsView {
  const cards = data.pages.flatMap((p) => p.cards);

  // Options come from the *unfiltered* loaded cards so every choice stays visible.
  const availableWorkItems = [...new Set(cards.map((c) => c.caseNumber).filter((n) => n && n !== "—"))].sort();
  const engineersById = new Map<string, string>();
  for (const c of cards) {
    if (c.userId) engineersById.set(c.userId, c.userName);
  }
  const availableEngineers = [...engineersById.entries()]
    .map(([id, name]) => ({ id, name }))
    .sort((a, b) => a.name.localeCompare(b.name));

  const filtered = cards.filter(
    (c) =>
      (filters.workItems.length === 0 || filters.workItems.includes(c.caseNumber)) &&
      (filters.engineers.length === 0 || filters.engineers.includes(c.userId)),
  );

  return {
    sheets: group(filtered),
    total: data.pages[0]?.total ?? cards.length,
    loaded: cards.length,
    availableWorkItems,
    availableEngineers,
  };
}

const getNextPageParam = (last: TimeCardPage): number | undefined => (last.hasMore ? last.nextOffset : undefined);

// Approve or reject a single card. The portal only ever sends the
// state-transition form (never editable fields).
async function decideCard(decision: TimeCardDecisionInput): Promise<CsmTimeCard> {
  const payload: BeUpdateTimeCardPayload = {
    state: decision.state,
    ...(decision.leadComment ? { leadComment: decision.leadComment } : {}),
  };
  const { data } = await apiClient.patch<BeTimeCardMutationResponse>(
    TIME_CARD_ENDPOINT(encodeURIComponent(decision.cardId)),
    payload,
  );
  return toTimeCard(data.timeCard);
}

export const timecards = {
  // The signed-in user's own cards, grouped into weekly sheets (newest first).
  // Scoped server-side by `userId` so the page reflects just this user's cards.
  // Root key ["timecards"] so one invalidate refreshes every view after a write.
  mySheets: (meId: string | null, meName: string, filters: TimeCardFilters = EMPTY_TIMECARD_FILTERS) =>
    infiniteQueryOptions({
      queryKey: ["timecards", "mine", meId, filters],
      queryFn: ({ pageParam }) =>
        meId
          ? searchTimeCardsPage(
              {
                userId: meId,
                projectIds: filters.projects.length ? filters.projects.map((p) => p.id) : undefined,
                states: filters.states.length ? filters.states : undefined,
                from: filters.from || undefined,
                to: filters.to || undefined,
              },
              pageParam,
            )
          : Promise.resolve(EMPTY_PAGE),
      initialPageParam: 0,
      getNextPageParam,
      enabled: !!meId,
      staleTime: 5_000,
      select: (data) => toView(data, filters, (cards) => groupIntoSheets(cards, meId ?? "", meName)),
    }),

  // Every visible user's cards (own included), grouped per-user — a read-only
  // "see everything" view with no state restriction or ownership exclusion.
  all: (meId: string | null, filters: TimeCardFilters = EMPTY_TIMECARD_FILTERS) =>
    infiniteQueryOptions({
      queryKey: ["timecards", "all", meId, filters],
      queryFn: ({ pageParam }) =>
        meId
          ? searchTimeCardsPage(
              {
                projectIds: filters.projects.length ? filters.projects.map((p) => p.id) : undefined,
                states: filters.states.length ? filters.states : undefined,
                from: filters.from || undefined,
                to: filters.to || undefined,
              },
              pageParam,
            )
          : Promise.resolve(EMPTY_PAGE),
      initialPageParam: 0,
      getNextPageParam,
      enabled: !!meId,
      staleTime: 5_000,
      select: (data) => toView(data, filters, groupCardsByUserIntoSheets),
    }),

  // Other engineers' sheets containing a card awaiting the signed-in approver's
  // decision. Scoped server-side by `approverId` + client-filtered to `submitted`
  // so the queue only holds cards this user can actually decide. Own cards are
  // excluded client-side as a defense-in-depth self-approval guard. The queue is
  // always `submitted`, so the state filter doesn't apply here — only the date range.
  approvalQueue: (meId: string | null, filters: TimeCardFilters = EMPTY_TIMECARD_FILTERS) =>
    infiniteQueryOptions({
      queryKey: ["timecards", "approvals", meId, filters],
      queryFn: ({ pageParam }) =>
        meId
          ? searchTimeCardsPage(
              {
                approverId: meId,
                projectIds: filters.projects.length ? filters.projects.map((p) => p.id) : undefined,
                states: ["submitted"],
                from: filters.from || undefined,
                to: filters.to || undefined,
              },
              pageParam,
            )
          : Promise.resolve(EMPTY_PAGE),
      initialPageParam: 0,
      getNextPageParam,
      enabled: !!meId,
      staleTime: 5_000,
      select: (data) =>
        toView(data, filters, (cards) => groupCardsByUserIntoSheets(cards.filter((c) => c.userId !== meId))),
    }),

  decide: decideCard,
};
