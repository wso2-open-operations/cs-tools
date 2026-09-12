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

import { useCallback, useMemo, useState, type ChangeEvent, type JSX } from "react";
import {
  AdapterDateFns,
  Box,
  DatePickers,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  MenuItem,
  Paper,
  Switch,
  Tab,
  Tabs,
  TablePagination,
  TextField,
  Typography,
  Button,
} from "@wso2/oxygen-ui";
import { ChevronDown, ChevronUp, Download, ListFilter, X } from "@wso2/oxygen-ui-icons-react";

// The plain (responsive) DatePicker switches to a mobile dialog (title bar +
// Cancel/OK) below the sm breakpoint — this is a desktop-only portal page, so
// DesktopDatePicker is used directly to always get the inline popup calendar.
const { DesktopDatePicker: DatePicker, LocalizationProvider } = DatePickers;

/** "YYYY-MM-DD" to a local-midnight Date (avoids the UTC-parse day-shift
 * `new Date(dateString)` can cause depending on the viewer's timezone). */
function parseDateOnly(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  return Number.isNaN(date.getTime()) ? null : date;
}

/** Local-midnight Date back to "YYYY-MM-DD", matching TimeCardSearchFilters'
 * `from`/`to` wire format. */
function formatDateOnly(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}
import {
  useAllTimeCards,
  useApprovalQueue,
  useBulkApproveCards,
  useCurrentEngineer,
  useDecideCard,
  useMyTimeCards,
  type TimeCardPagination,
} from "@features/csm-timecards/api/useTimeSheets";
import { useDeleteTimeCard, useUpdateTimeCard } from "@features/csm-timecards/api/useTimeCards";
import AsyncProjectMultiSelect from "@features/csm-cases/components/AsyncProjectMultiSelect";
import { BackendApiError } from "@api/backend/client";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { TIME_CARD_STATE_META } from "@features/csm-timecards/constants/timeCardConstants";
import RefreshButton from "@components/RefreshButton";
import { useTimecardRole } from "@features/csm-timecards/hooks/useTimecardRole";
import TimeCardsTable from "@features/csm-timecards/components/TimeCardsTable";
import TimeCardReviewDialog from "@features/csm-timecards/components/TimeCardReviewDialog";
import BulkApproveDialog from "@features/csm-timecards/components/BulkApproveDialog";
import LogTimeCardDialog from "@features/csm-timecards/components/LogTimeCardDialog";
import SearchableMultiSelect from "@components/SearchableMultiSelect";
import AsyncUserIdMultiSelect from "@features/csm-cases/components/AsyncUserIdMultiSelect";
import { INTERNAL_USER_ROLES } from "@features/csm-users/types/csmUsers";
import { exportTimeCardsCsv } from "@features/csm-timecards/utils/timeCardCsvExport";
import { cardActions, type TimecardAction, type TimecardRoleCtx } from "@features/csm-timecards/utils/timeSheetState";
import type { TimeCardGroupBy } from "@features/csm-timecards/utils/timeCardGrouping";
import type {
  CsmTimeCard,
  TimeCardSearchFilters,
  TimeCardState,
} from "@features/csm-timecards/types/timeCards";

/** Distinct case numbers present in whatever cards are currently loaded for
 * one tab — the option list for the work-item filter. There's no
 * case-number search endpoint for time cards, so — unlike the Engineer
 * filter (see `AsyncUserIdMultiSelect` below, which searches the real users
 * directory) — this only ever offers case numbers actually present on the
 * current page, not every case the viewer could filter by. */
function workItemOptionsFrom(cards: CsmTimeCard[] | undefined): string[] {
  return Array.from(new Set((cards ?? []).map((c) => c.caseNumber)));
}

/** `[projectId, projectName]` pairs present in a batch of cards — building
 * block for the page-level accumulating project-name cache below. Each
 * `CsmTimeCard` already carries both fields, so this is a free derivation,
 * not a new fetch. */
function projectNamesIn(cards: CsmTimeCard[] | undefined): [string, string][] {
  return (cards ?? []).map((c) => [c.projectId, c.projectName]);
}

/** `[userId, userName]` pairs present in a batch of cards — same role as
 * {@link projectNamesIn}, but for the Engineer filter's chip-label cache
 * (see `engineerNameCache` below): the filter itself now searches the real
 * users directory via `AsyncUserIdMultiSelect`, but a selected engineer's
 * name still needs a source before that search has run against the id
 * already in the filter (e.g. right after switching tabs). */
function engineerNamesIn(cards: CsmTimeCard[] | undefined): [string, string][] {
  return (cards ?? []).map((c) => [c.userId, c.userName]);
}

const DEFAULT_ROWS_PER_PAGE = 20;
// Top option is the backend's max page limit; larger requests are rejected.
const ROWS_PER_PAGE_OPTIONS = [10, 20, BE_MAX_PAGE_LIMIT];

// Static role contexts for `TimeCardsTable`'s `roleFor` — constant regardless
// of which card is being rendered, unlike the "All" tab's (see allRoleFor in
// the component, which depends on the signed-in user's id per card).
const mineRole = (): TimecardRoleCtx => ({ isOwner: true, isApprover: false, isAdmin: false });
const approvalsRoleFor =
  (isAdmin: boolean) =>
  (): TimecardRoleCtx => ({ isOwner: false, isApprover: true, isAdmin });

/** Page + rows-per-page state for one tab's `TablePagination`, following the
 * same shape/convention as `CsmUsersPage.tsx` and friends. Each tab gets its
 * own instance so switching tabs doesn't disturb another tab's position. */
function usePagination(): {
  pagination: TimeCardPagination;
  onPageChange: (event: unknown, newPage: number) => void;
  onRowsPerPageChange: (e: ChangeEvent<HTMLInputElement>) => void;
  setPage: (page: number) => void;
} {
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  return {
    pagination: { page, rowsPerPage },
    onPageChange: (_, newPage) => setPage(newPage),
    onRowsPerPageChange: (e) => {
      setRowsPerPage(parseInt(e.target.value, 10));
      setPage(0);
    },
    setPage,
  };
}

type TabId = "mine" | "all" | "approvals";

/**
 * Time cards workspace. Three tabs: **My time sheets** (own cards only),
 * **All** (everyone's cards, read only — visibility, not action), and
 * **Approvals** (approver/admin: approve/reject a submitted card, or select
 * several and approve them together). Logging a *new* card still only
 * happens from a case's Time tracking tab (this page has no case context to
 * log against) — but editing an own still-`submitted` card is available
 * from here too (My time sheets / All), via the same Edit action
 * `TimeCardsTable` offers there. There's no delegation or reports — the
 * backend has no endpoints for those (see the module-level notes in
 * `types/timeCards.ts`); bulk approve is a frontend-only fan-out over the
 * same single-card endpoint (see `useBulkApproveCards`), not a real batch
 * request.
 */
export default function CsmTimeCardsPage(): JSX.Element {
  const role = useTimecardRole();
  const me = useCurrentEngineer();
  const { showError } = useErrorBanner();
  const { showSuccess } = useSuccessBanner();
  const [tab, setTab] = useState<TabId>("mine");
  const activeTab: TabId = tab === "approvals" && !role.isApprover ? "mine" : tab;
  // Stable per-render so re-renders while the page is open don't shift the
  // exported filename's date mid-session.
  const todayStamp = useMemo(() => new Date().toISOString().slice(0, 10), []);

  // Carries the action the user already picked (clicking Approve vs Reject in
  // the list), so the dialog reflects that one decision instead of asking
  // again — see TimeCardReviewDialog's `action` prop.
  const [review, setReview] = useState<{ card: CsmTimeCard; action: TimecardAction } | null>(null);
  // The card open in the edit dialog, if any — this page has no case
  // context of its own, so LogTimeCardDialog's caseId/caseNumber/projectId/
  // projectName all come from the card being edited (see the render below).
  const [editingCard, setEditingCard] = useState<CsmTimeCard | null>(null);
  const updateTimeCard = useUpdateTimeCard();
  // The card pending a delete confirmation, if any — mirrors
  // CsmCaseDetailPage's own attachment-delete `pendingDelete` pattern.
  const [pendingDelete, setPendingDelete] = useState<CsmTimeCard | null>(null);
  const deleteTimeCard = useDeleteTimeCard();

  // Bulk-approve selection — Approvals tab only. Holds card ids rather than
  // whole cards so a background refetch (refresh button, or the queue
  // shrinking after a decision) can't leave this holding stale card objects;
  // `selectedApprovalCards` below re-derives the live card list from
  // whatever ids are still actually present on the current page.
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [bulkConfirmOpen, setBulkConfirmOpen] = useState(false);
  const bulkApprove = useBulkApproveCards();
  const clearSelection = (): void => setSelectedIds(new Set());

  // How the table clusters its rows — a display-only choice (see
  // `groupTimeCards`), independent of the server-side filters below. Only
  // toggleable on All/Approvals (see the Group-by control); "My time sheets"
  // stays grouped by case, since every card already belongs to the same
  // engineer (you).
  const [groupBy, setGroupBy] = useState<TimeCardGroupBy>("case");

  // Search filters (sent as a POST body, never query params). Project, state,
  // and engineer (see filtersWithEngineer below) are all server-side; work
  // item stays client-side over the returned page — the backend has no
  // case-number filter, only caseId (see byWorkItem below).
  const [filterProject, setFilterProject] = useState<string[]>([]);
  const [filterWorkItem, setFilterWorkItem] = useState<string[]>([]);
  const [filterState, setFilterState] = useState<TimeCardState | "">("");
  const [filterEngineer, setFilterEngineer] = useState<string[]>([]);
  // Date range (YYYY-MM-DD, inclusive) — `from`/`to` are already real
  // server-side filters (see TimeCardSearchFilters), just never had a UI
  // control wired to them until now.
  const [filterFrom, setFilterFrom] = useState("");
  const [filterTo, setFilterTo] = useState("");

  // Search is unscoped by default: with no project filter picked we send no
  // `projectIds`, and the backend returns every time card the caller is
  // entitled to (internal agents get a global list). Picking the project
  // filter narrows it to the chosen project(s). "My time sheets" and
  // "Approvals" are further bounded server-side by the signed-in user /
  // approver, so they stay correct unscoped too.
  const scopeProjectIds = filterProject;

  const baseFilters: TimeCardSearchFilters = {
    ...(scopeProjectIds.length && { projectIds: scopeProjectIds }),
    ...(filterState && { states: [filterState] }),
    ...(filterFrom && { from: filterFrom }),
    ...(filterTo && { to: filterTo }),
  };
  // The Engineer filter only has a control on the All/Approvals tabs (see
  // engineerSlot below) — folded in as a separate `userIds` filter, not into
  // baseFilters itself, so a value picked on those tabs can never leak into
  // "My time sheets" (which is already its own-user-only via useMyTimeCards).
  const filtersWithEngineer: TimeCardSearchFilters = filterEngineer.length
    ? { ...baseFilters, userIds: filterEngineer }
    : baseFilters;

  // Each tab pages independently — was previously fetching its *entire*
  // scope (up to 1,000 cards, sequential page-by-page requests) before
  // showing anything, confirmed live to take 30-60+ seconds and, with all
  // three tabs doing this eagerly at once, enough concurrent load to make
  // some fail outright. Real pagination replaces that: one page at a time,
  // driven by the TablePagination controls below each list. Still gated on
  // its own tab actually being active, for the same concurrent-load reason.
  const minePagination = usePagination();
  const allPagination = usePagination();
  const approvalsPagination = usePagination();

  const myCards = useMyTimeCards(activeTab === "mine", baseFilters, minePagination.pagination);
  const allCards = useAllTimeCards(activeTab === "all", filtersWithEngineer, allPagination.pagination);
  const queue = useApprovalQueue(
    activeTab === "approvals" && role.isApprover,
    filtersWithEngineer,
    approvalsPagination.pagination,
  );
  const decideCard = useDecideCard();

  const anyFilterActive =
    filterProject.length > 0 ||
    filterWorkItem.length > 0 ||
    !!filterState ||
    filterEngineer.length > 0 ||
    !!filterFrom ||
    !!filterTo;

  // A filter change re-scopes the search for every tab, so every tab's page
  // position needs to reset too — otherwise "page 3" of a narrower result
  // set could be past the end, or just show unrelated leftovers.
  const resetAllPages = (): void => {
    minePagination.setPage(0);
    allPagination.setPage(0);
    approvalsPagination.setPage(0);
    clearSelection();
  };
  const handleFilterProjectChange = (v: string[]): void => {
    setFilterProject(v);
    resetAllPages();
  };
  // Unlike the other filters, work item is purely client-side (narrows an
  // already-fetched page — see byWorkItem below), so it doesn't strictly
  // need a page reset. It still needs resetAllPages() for the selection
  // clear bundled into it, though: narrowing to a different set of visible
  // cards on the Approvals tab can silently drop some of the current
  // selection out of view otherwise.
  const handleFilterWorkItemChange = (v: string[]): void => {
    setFilterWorkItem(v);
    resetAllPages();
  };
  const handleFilterStateChange = (v: TimeCardState | ""): void => {
    setFilterState(v);
    resetAllPages();
  };
  const handleFilterEngineerChange = (v: string[]): void => {
    setFilterEngineer(v);
    resetAllPages();
  };
  const handleFilterFromChange = (v: string): void => {
    setFilterFrom(v);
    // min/max on the date inputs only guide the picker UI — typing a date
    // directly can still commit an inverted range, so clamp here too.
    if (filterTo && v > filterTo) setFilterTo(v);
    resetAllPages();
  };
  const handleFilterToChange = (v: string): void => {
    setFilterTo(v);
    if (filterFrom && v < filterFrom) setFilterFrom(v);
    resetAllPages();
  };
  const clearFilters = (): void => {
    setFilterProject([]);
    setFilterWorkItem([]);
    setFilterState("");
    setFilterEngineer([]);
    setFilterFrom("");
    setFilterTo("");
    resetAllPages();
  };

  const handleCardAction = (card: CsmTimeCard, action: TimecardAction): void => {
    if (action === "approve" || action === "reject") setReview({ card, action });
    else if (action === "edit") setEditingCard(card);
    else if (action === "delete") setPendingDelete(card);
  };

  const toggleSelectCard = (card: CsmTimeCard): void => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(card.id)) next.delete(card.id);
      else next.add(card.id);
      return next;
    });
  };
  const toggleSelectAllCards = (selectableCards: CsmTimeCard[]): void => {
    const allAlreadySelected =
      selectableCards.length > 0 && selectableCards.every((c) => selectedIds.has(c.id));
    setSelectedIds(allAlreadySelected ? new Set() : new Set(selectableCards.map((c) => c.id)));
  };

  // "All" shows everyone's cards, own included, and is always read-only
  // (isApprover/isAdmin false regardless of the viewer's actual role) — that
  // stays exclusive to the Approvals tab. Only isOwner varies per card here.
  const allRoleFor = (card: CsmTimeCard): TimecardRoleCtx => ({
    isOwner: card.userId === me.id,
    isApprover: false,
    isAdmin: false,
  });
  const approvalsRole = approvalsRoleFor(role.isAdmin);

  /** Client-side work-item filter (case number is in the selected set),
   * applied over an already-fetched page of cards. Stable per filterWorkItem
   * so the memoized *FilteredCards below only recompute when it (or the
   * underlying data) actually changes. */
  const byWorkItem = useCallback(
    (cards: CsmTimeCard[] | undefined): CsmTimeCard[] => {
      if (!cards) return [];
      if (filterWorkItem.length === 0) return cards;
      return cards.filter((c) => filterWorkItem.includes(c.caseNumber));
    },
    [filterWorkItem],
  );

  // Work-item / engineer option lists are scoped per tab — each tab has its
  // own loaded page of cards, and there's no search endpoint for either, so
  // the picker can only ever offer what's actually on the current page.
  const mineWorkItemOptions = useMemo(
    () => workItemOptionsFrom(myCards.data?.cards),
    [myCards.data],
  );
  const allWorkItemOptions = useMemo(
    () => workItemOptionsFrom(allCards.data?.cards),
    [allCards.data],
  );
  const approvalsWorkItemOptions = useMemo(
    () => workItemOptionsFrom(queue.data?.cards),
    [queue.data],
  );
  // Persistent projectId -> projectName / userId -> userName caches for the
  // Project and Engineer filters' chip labels, accumulated across every tab's
  // loaded cards over the page's lifetime and never shrunk. Unlike
  // workItemOptions above (deliberately scoped to one tab's current page),
  // these can't be a per-tab derivation: each tab's FilterBar/
  // AsyncProjectMultiSelect/AsyncUserIdMultiSelect instance unmounts on tab
  // switch (conditional rendering below), which would otherwise drop a
  // selected project/engineer's name the moment the newly active tab's own
  // cards don't happen to include it — or are empty — leaving the chip
  // showing a raw id until the dropdown is reopened and re-searched. (The
  // Engineer filter's own search always finds the name eventually, since it
  // now queries the real users directory rather than only the current page —
  // this cache just avoids a raw-UUID flash in the meantime.)
  //
  // Reconciled during render (React's "adjusting state when data changes"
  // pattern: https://react.dev/reference/react/useState#storing-information-from-previous-renders)
  // rather than in an effect, so fresh names are available in the same
  // render the data arrived in instead of one render later.
  const [projectNameCache, setProjectNameCache] = useState<Map<string, string>>(
    () => new Map(),
  );
  const [engineerNameCache, setEngineerNameCache] = useState<Map<string, string>>(
    () => new Map(),
  );
  const [lastSeenCards, setLastSeenCards] = useState<{
    mine?: typeof myCards.data;
    all?: typeof allCards.data;
    queue?: typeof queue.data;
  }>(() => ({}));
  if (
    lastSeenCards.mine !== myCards.data ||
    lastSeenCards.all !== allCards.data ||
    lastSeenCards.queue !== queue.data
  ) {
    setLastSeenCards({ mine: myCards.data, all: allCards.data, queue: queue.data });
    const learnedProjects = [
      ...projectNamesIn(myCards.data?.cards),
      ...projectNamesIn(allCards.data?.cards),
      ...projectNamesIn(queue.data?.cards),
    ];
    let nextProjects: Map<string, string> | undefined;
    for (const [id, name] of learnedProjects) {
      if (projectNameCache.get(id) !== name) {
        nextProjects ??= new Map(projectNameCache);
        nextProjects.set(id, name);
      }
    }
    if (nextProjects) setProjectNameCache(nextProjects);

    const learnedEngineers = [
      ...engineerNamesIn(myCards.data?.cards),
      ...engineerNamesIn(allCards.data?.cards),
      ...engineerNamesIn(queue.data?.cards),
    ];
    let nextEngineers: Map<string, string> | undefined;
    for (const [id, name] of learnedEngineers) {
      if (engineerNameCache.get(id) !== name) {
        nextEngineers ??= new Map(engineerNameCache);
        nextEngineers.set(id, name);
      }
    }
    if (nextEngineers) setEngineerNameCache(nextEngineers);
  }

  // `AsyncUserIdMultiSelect` resolves a name the moment a directory search
  // result is picked — earlier than `engineerNamesIn` above could ever know
  // it (that only learns from cards already loaded on screen). Without
  // this, an engineer picked purely from the directory (no card of theirs
  // on the current page) would render as a raw UUID on the next tab, since
  // that tab mounts a fresh `AsyncUserIdMultiSelect` instance with no
  // memory of the pick.
  const handleEngineerNamesResolved = (entries: [string, string][]): void => {
    let next: Map<string, string> | undefined;
    for (const [id, name] of entries) {
      if (engineerNameCache.get(id) !== name) {
        next ??= new Map(engineerNameCache);
        next.set(id, name);
      }
    }
    if (next) setEngineerNameCache(next);
  };

  // Filtered cards per tab, computed once and shared between the FilterBar's
  // export action and the table rendering below — rather than recomputing
  // (and risking drift) in two places. Engineer is already applied
  // server-side (via filtersWithEngineer) by the time allCards/queue resolve
  // — only work item still needs a client-side pass here. Memoized so a
  // render triggered by unrelated state (e.g. the groupBy toggle) doesn't
  // reallocate these arrays on every tab.
  const mineFilteredCards = useMemo(
    () => byWorkItem(myCards.data?.cards),
    [myCards.data, byWorkItem],
  );
  const allFilteredCards = useMemo(
    () => byWorkItem(allCards.data?.cards),
    [allCards.data, byWorkItem],
  );
  const approvalsFilteredCards = useMemo(
    () => byWorkItem(queue.data?.cards),
    [queue.data, byWorkItem],
  );
  // The actually-actionable selection: `selectedIds` alone can outlive its
  // own basis (a queue refetch — e.g. the Refresh button, or another
  // approver deciding a card first — can drop a card that was selected a
  // moment ago), so this re-derives from whatever `approvalsFilteredCards`
  // holds *right now* rather than trusting the raw id set. Used everywhere
  // a "how many/which cards am I about to approve" answer is needed (the
  // toolbar's count and the confirm dialog) so they can never disagree with
  // each other, even though nothing here mutates `selectedIds` itself to
  // prune the stale ids out — the next real toggle/clear naturally drops
  // them.
  const selectedApprovalCards = useMemo(
    () =>
      approvalsFilteredCards.filter(
        (c) =>
          selectedIds.has(c.id) &&
          cardActions(c.state, { isOwner: false, isApprover: true, isAdmin: role.isAdmin }).includes(
            "approve",
          ),
      ),
    [approvalsFilteredCards, selectedIds, role.isAdmin],
  );
  // The ids actually reflected in selectedApprovalCards -- passed to
  // TimeCardsTable instead of the raw selectedIds state so its row-disabling
  // "is a selection active" check (and its row/header checkbox state) can
  // never go stale: a queue refetch dropping every selected card (someone
  // else deciding it first, a filter/refresh) would otherwise leave
  // selectedIds non-empty with nothing left to act on, silently disabling
  // every row's own Approve/Reject with no visible selection (and no Clear
  // button, gated on selectedApprovalCards.length) to unstick it.
  const selectedApprovalCardIds = useMemo(
    () => new Set(selectedApprovalCards.map((c) => c.id)),
    [selectedApprovalCards],
  );
  // Prunes selectedIds itself (not just how it's displayed above) down to
  // the still-actionable subset whenever the approvals queue refetches with
  // different content -- otherwise an id dropped by a refetch (another
  // approver deciding it first, a stale sync) lingers in state forever,
  // ready to silently reappear as "selected" if a card with that id is ever
  // eligible again. Explicit clears (tab/page/filter change, a successful
  // decide/bulk-approve) already call clearSelection() directly and are
  // unaffected by this. Same render-time reconciliation pattern as
  // projectNameCache/lastSeenCards above.
  const [lastPrunedQueueData, setLastPrunedQueueData] = useState(queue.data);
  if (lastPrunedQueueData !== queue.data) {
    setLastPrunedQueueData(queue.data);
    if (selectedApprovalCardIds.size !== selectedIds.size) {
      setSelectedIds(selectedApprovalCardIds);
    }
  }

  return (
    <Box
      sx={{ p: { xs: 2, md: 3 }, display: "flex", flexDirection: "column", gap: 2 }}
    >
      <Box>
        <Typography variant="h5">Time cards</Typography>
        <Typography variant="body2" color="text.secondary">
          Review your logged time and (for approvers) submissions. Log time
          from a case&apos;s <strong>Time tracking</strong> tab.
        </Typography>
      </Box>

      <Tabs
        value={activeTab}
        onChange={(_, v) => {
          setTab(v as TabId);
          clearSelection();
        }}
        sx={{ borderBottom: 1, borderColor: "divider" }}
      >
        <Tab value="mine" label="My time sheets" />
        <Tab value="all" label="All" />
        {role.isApprover && <Tab value="approvals" label="Approvals" />}
      </Tabs>

      {/* My time sheets */}
      {activeTab === "mine" && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <FilterBar
            projectNameSeed={projectNameCache}
            filterProject={filterProject}
            setFilterProject={handleFilterProjectChange}
            filterWorkItem={filterWorkItem}
            setFilterWorkItem={handleFilterWorkItemChange}
            workItemOptions={mineWorkItemOptions}
            filterState={filterState}
            setFilterState={handleFilterStateChange}
            filterFrom={filterFrom}
            setFilterFrom={handleFilterFromChange}
            filterTo={filterTo}
            setFilterTo={handleFilterToChange}
            onClear={clearFilters}
          />

          <Box sx={{ display: "flex", justifyContent: "flex-end", alignItems: "center", gap: 1 }}>
            <RefreshButton
              onRefresh={() => void myCards.refetch()}
              isFetching={myCards.isFetching}
              updatedAt={myCards.dataUpdatedAt}
              label="Refresh my time sheets"
            />
            {!myCards.isError && (
              <ExportCsvButton
                cards={mineFilteredCards}
                filename={`time-cards-my-sheets-${todayStamp}.csv`}
              />
            )}
          </Box>

          {myCards.isError ? (
            <Typography color="error">Could not load your time cards.</Typography>
          ) : (
            <>
              <TimeCardsTable
                cards={mineFilteredCards}
                isLoading={myCards.isLoading}
                groupBy="case"
                roleFor={mineRole}
                onCardAction={handleCardAction}
                emptyText={
                  anyFilterActive
                    ? "No time cards match the current filters."
                    : "No time logged yet. Open a case and use its Time tracking tab to log time."
                }
              />
              {/* Pages over raw cards, not display groups — a case's cards can
               legitimately span a page boundary and look incomplete until
               you've paged further. Accepted tradeoff for real pagination
               instead of a 30-60s upfront full-scope fetch. */}
              <TablePagination
                component="div"
                count={myCards.data?.total ?? 0}
                page={minePagination.pagination.page}
                onPageChange={minePagination.onPageChange}
                rowsPerPage={minePagination.pagination.rowsPerPage}
                onRowsPerPageChange={minePagination.onRowsPerPageChange}
                rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
                showFirstButton
                showLastButton
              />
            </>
          )}
        </Box>
      )}

      {/* All — everyone's cards, own included. Read only: role is always
       passed as non-approver/non-admin here regardless of the viewer's
       actual role, so no Approve/Reject actions ever show — that stays
       exclusive to the Approvals tab. */}
      {activeTab === "all" && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <FilterBar
            projectNameSeed={projectNameCache}
            filterProject={filterProject}
            setFilterProject={handleFilterProjectChange}
            filterWorkItem={filterWorkItem}
            setFilterWorkItem={handleFilterWorkItemChange}
            workItemOptions={allWorkItemOptions}
            filterState={filterState}
            setFilterState={handleFilterStateChange}
            filterFrom={filterFrom}
            setFilterFrom={handleFilterFromChange}
            filterTo={filterTo}
            setFilterTo={handleFilterToChange}
            onClear={clearFilters}
            engineerSlot={
              <AsyncUserIdMultiSelect
                id="timecards-filter-engineer-all"
                label="Engineer"
                values={filterEngineer}
                onChange={handleFilterEngineerChange}
                nameSeed={engineerNameCache}
                onNamesResolved={handleEngineerNamesResolved}
                roleIds={INTERNAL_USER_ROLES}
                active
              />
            }
            engineerActive={filterEngineer.length > 0}
          />

          <GroupByToggle value={groupBy} onChange={setGroupBy} />

          <Box sx={{ display: "flex", justifyContent: "flex-end", alignItems: "center", gap: 1 }}>
            <RefreshButton
              onRefresh={() => void allCards.refetch()}
              isFetching={allCards.isFetching}
              updatedAt={allCards.dataUpdatedAt}
              label="Refresh time cards"
            />
            {!allCards.isError && (
              <ExportCsvButton
                cards={allFilteredCards}
                filename={`time-cards-all-${todayStamp}.csv`}
              />
            )}
          </Box>

          {allCards.isError ? (
            <Typography color="error">Could not load time cards.</Typography>
          ) : (
            <>
              <TimeCardsTable
                cards={allFilteredCards}
                isLoading={allCards.isLoading}
                groupBy={groupBy}
                showEngineerColumn
                roleFor={allRoleFor}
                onCardAction={handleCardAction}
                emptyText={anyFilterActive ? "No time cards match the current filters." : "No time logged yet."}
              />
              <TablePagination
                component="div"
                count={allCards.data?.total ?? 0}
                page={allPagination.pagination.page}
                onPageChange={allPagination.onPageChange}
                rowsPerPage={allPagination.pagination.rowsPerPage}
                onRowsPerPageChange={allPagination.onRowsPerPageChange}
                rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
                showFirstButton
                showLastButton
              />
            </>
          )}
        </Box>
      )}

      {/* Approvals */}
      {activeTab === "approvals" && role.isApprover && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <FilterBar
            projectNameSeed={projectNameCache}
            filterProject={filterProject}
            setFilterProject={handleFilterProjectChange}
            filterWorkItem={filterWorkItem}
            setFilterWorkItem={handleFilterWorkItemChange}
            workItemOptions={approvalsWorkItemOptions}
            filterState={filterState}
            setFilterState={handleFilterStateChange}
            filterFrom={filterFrom}
            setFilterFrom={handleFilterFromChange}
            filterTo={filterTo}
            setFilterTo={handleFilterToChange}
            onClear={clearFilters}
            hideStateFilter
            engineerSlot={
              <AsyncUserIdMultiSelect
                id="timecards-filter-engineer-approvals"
                label="Engineer"
                values={filterEngineer}
                onChange={handleFilterEngineerChange}
                nameSeed={engineerNameCache}
                onNamesResolved={handleEngineerNamesResolved}
                roleIds={INTERNAL_USER_ROLES}
                active
              />
            }
            engineerActive={filterEngineer.length > 0}
          />

          <GroupByToggle value={groupBy} onChange={setGroupBy} />

          <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 1 }}>
            {selectedApprovalCards.length > 0 ? (
              <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
                <Typography variant="body2">{selectedApprovalCards.length} selected</Typography>
                <Button size="small" color="inherit" onClick={clearSelection}>
                  Clear
                </Button>
                <Button
                  size="small"
                  color="primary"
                  variant="outlined"
                  onClick={() => setBulkConfirmOpen(true)}
                >
                  Approve
                </Button>
              </Box>
            ) : (
              <Box />
            )}
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <RefreshButton
                onRefresh={() => void queue.refetch()}
                isFetching={queue.isFetching}
                updatedAt={queue.dataUpdatedAt}
                label="Refresh approval queue"
              />
              {!queue.isError && (
                <ExportCsvButton
                  cards={approvalsFilteredCards}
                  filename={`time-cards-approvals-${todayStamp}.csv`}
                />
              )}
            </Box>
          </Box>

          {queue.isError ? (
            <Typography color="error">Could not load the approval queue.</Typography>
          ) : (
            <>
              <TimeCardsTable
                cards={approvalsFilteredCards}
                isLoading={queue.isLoading}
                groupBy={groupBy}
                showEngineerColumn
                showActionsColumn
                roleFor={approvalsRole}
                onCardAction={handleCardAction}
                selectable
                selectedIds={selectedApprovalCardIds}
                onToggleSelect={toggleSelectCard}
                onToggleSelectAll={toggleSelectAllCards}
                emptyText={anyFilterActive ? "No time cards match the current filters." : "Nothing awaiting approval."}
              />
              <TablePagination
                component="div"
                count={queue.data?.total ?? 0}
                page={approvalsPagination.pagination.page}
                onPageChange={(e, p) => {
                  clearSelection();
                  approvalsPagination.onPageChange(e, p);
                }}
                rowsPerPage={approvalsPagination.pagination.rowsPerPage}
                onRowsPerPageChange={(e) => {
                  clearSelection();
                  approvalsPagination.onRowsPerPageChange(e as ChangeEvent<HTMLInputElement>);
                }}
                rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
                showFirstButton
                showLastButton
              />
            </>
          )}
        </Box>
      )}

      {bulkConfirmOpen && (
        <BulkApproveDialog
          cards={selectedApprovalCards}
          isSubmitting={bulkApprove.isPending}
          onClose={() => setBulkConfirmOpen(false)}
          onConfirm={() => {
            // Captured here, before clearSelection() below can drop it — the
            // only place a failed card's own label (case number, work date)
            // is still available to resolve `f.cardId` back to something a
            // human can actually tell apart from another failure in the same
            // batch (see the onError banner below).
            const cardsToApprove = selectedApprovalCards;
            const ids = cardsToApprove.map((c) => c.id);
            bulkApprove.mutate(ids, {
              onSuccess: (result) => {
                setBulkConfirmOpen(false);
                clearSelection();
                if (result.failed.length === 0) {
                  showSuccess(
                    `${result.succeededIds.length} time card${result.succeededIds.length === 1 ? "" : "s"} approved.`,
                  );
                } else {
                  const cardById = new Map(cardsToApprove.map((c) => [c.id, c]));
                  const failureDetails = result.failed
                    .map((f) => {
                      const card = cardById.get(f.cardId);
                      const label = card ? `${card.caseNumber} (${card.workDate.slice(0, 10)})` : f.cardId;
                      return `${label}: ${f.message}`;
                    })
                    .join("; ");
                  showError(
                    `${result.succeededIds.length} approved, ${result.failed.length} failed: ${failureDetails}`,
                  );
                }
              },
            });
          }}
        />
      )}

      {editingCard && (
        <LogTimeCardDialog
          caseId={editingCard.caseId}
          caseNumber={editingCard.caseNumber}
          projectId={editingCard.projectId}
          projectName={editingCard.projectName}
          editingCard={editingCard}
          isSubmitting={updateTimeCard.isPending}
          onClose={() => setEditingCard(null)}
          onSubmit={(input) => {
            if (!("cardId" in input)) return; // always the edit shape here
            updateTimeCard.mutate(input, {
              onSuccess: () => {
                setEditingCard(null);
                showSuccess("Time card updated.");
              },
              onError: (err) => {
                const msg =
                  err instanceof BackendApiError && err.status < 500 && err.message
                    ? err.message
                    : "Could not save your changes.";
                showError(msg, err);
              },
            });
          }}
        />
      )}

      <Dialog
        open={!!pendingDelete}
        onClose={() => {
          if (!deleteTimeCard.isPending) setPendingDelete(null);
        }}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>Delete time card?</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            Permanently delete this {pendingDelete?.totalMinutes} min entry on{" "}
            <strong>{pendingDelete?.caseNumber}</strong>? This can&apos;t be undone.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button
            color="inherit"
            onClick={() => setPendingDelete(null)}
            disabled={deleteTimeCard.isPending}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            color="error"
            disabled={deleteTimeCard.isPending}
            onClick={() => {
              if (!pendingDelete) return;
              const target = pendingDelete;
              deleteTimeCard.mutate(target.id, {
                onSuccess: () => {
                  setPendingDelete(null);
                  showSuccess("Time card deleted.");
                },
                onError: (err) => {
                  setPendingDelete(null);
                  const msg =
                    err instanceof BackendApiError && err.status < 500 && err.message
                      ? err.message
                      : "Could not delete this time card.";
                  showError(msg, err);
                },
              });
            }}
          >
            {deleteTimeCard.isPending ? "Deleting…" : "Delete"}
          </Button>
        </DialogActions>
      </Dialog>

      {review && (
        <TimeCardReviewDialog
          card={review.card}
          action={review.action}
          isDeciding={decideCard.isPending}
          onClose={() => setReview(null)}
          onDecide={(decision) =>
            decideCard.mutate(decision, {
              onSuccess: () => {
                setReview(null);
                showSuccess(
                  decision.state === "approved"
                    ? "Time card approved."
                    : "Time card rejected.",
                );
              },
              onError: (err) => {
                // The backend 403s when the signed-in user isn't authorized
                // to decide this specific card (confirmed live: approving
                // your own just-created card succeeds, approving another
                // engineer's real card 403s) — surface its own message
                // rather than failing silently.
                const msg =
                  err instanceof BackendApiError && err.status < 500 && err.message
                    ? err.message
                    : "Could not submit your decision. Please try again.";
                showError(msg, err);
              },
            })
          }
        />
      )}
    </Box>
  );
}

/** States reachable via the portal's API today — "pending"/"recalled"/
 * "processed" exist in the backend's enum but nothing here can produce them. */
const FILTER_STATES: TimeCardState[] = ["submitted", "approved", "rejected"];

/**
 * Shared filter bar for the My time sheets, All, and Approvals tabs.
 * Matches the Paper + collapsible-Grid shape used by `CasesFilterBar` and
 * `ChangeRequestsFilterBar` (toggle button with an active count, a "Clear
 * filters" button, a responsive Grid of fields, an "N filters active"
 * caption) instead of a bespoke always-open single-row layout, so time cards
 * doesn't look and behave differently from every other list page's filters.
 */
function FilterBar({
  projectNameSeed,
  filterProject,
  setFilterProject,
  filterWorkItem,
  setFilterWorkItem,
  workItemOptions,
  filterState,
  setFilterState,
  filterFrom,
  setFilterFrom,
  filterTo,
  setFilterTo,
  onClear,
  engineerSlot,
  engineerActive,
  hideStateFilter,
}: {
  /** `projectId -> projectName` lookup for already-selected chips — the
   * page-level, cross-tab accumulating cache (see `projectNameCache` in
   * `CsmTimeCardsPage`), not a per-tab derivation, so a selected project's
   * name survives this component remounting on tab switch. Async search
   * resolves anything the cache hasn't learned yet. */
  projectNameSeed: Map<string, string>;
  filterProject: string[];
  setFilterProject: (v: string[]) => void;
  filterWorkItem: string[];
  setFilterWorkItem: (v: string[]) => void;
  /** Case numbers to offer in the work-item picker — scoped to whatever the
   * calling tab currently has loaded (see `workItemOptionsFrom`). */
  workItemOptions: string[];
  filterState: TimeCardState | "";
  setFilterState: (v: TimeCardState | "") => void;
  /** Inclusive date range (YYYY-MM-DD), matched against a card's work date. */
  filterFrom: string;
  setFilterFrom: (v: string) => void;
  filterTo: string;
  setFilterTo: (v: string) => void;
  onClear: () => void;
  engineerSlot?: JSX.Element;
  /** Whether the Engineer filter (only on the All/Approvals tabs) is active —
   * counted alongside the other fields for the toggle button's badge. */
  engineerActive?: boolean;
  /** Approvals always forces `states: ["submitted"]` server-side (see
   * `useApprovalQueue`), so the State control can't actually narrow anything
   * there — hide it instead of showing a filter that silently does nothing. */
  hideStateFilter?: boolean;
}): JSX.Element {
  const [isFiltersOpen, setIsFiltersOpen] = useState(true);

  // filterState counts here even when this tab hides its own State control
  // (hideStateFilter) — it's shared across tabs, so a value set elsewhere
  // must still be clearable from this one, not just invisible and stuck.
  const activeCount =
    (filterProject.length > 0 ? 1 : 0) +
    (filterWorkItem.length > 0 ? 1 : 0) +
    (engineerActive ? 1 : 0) +
    (filterState ? 1 : 0) +
    (filterFrom || filterTo ? 1 : 0);
  const hasActive = activeCount > 0;

  return (
    <Paper sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        <Button
          variant="outlined"
          size="small"
          color="primary"
          onClick={() => setIsFiltersOpen((v) => !v)}
          startIcon={<ListFilter size={16} />}
          endIcon={isFiltersOpen ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
        >
          {hasActive ? `Filters (${activeCount})` : "Filters"}
        </Button>
        {hasActive && (
          <Button
            variant="text"
            size="small"
            color="primary"
            onClick={onClear}
            startIcon={<X size={16} />}
          >
            Clear filters
          </Button>
        )}
      </Box>

      {isFiltersOpen && (
        <>
          <Divider />
          {/* Row 1: Project / Work item / Engineer / State — each flexes
              equally so the row always fills the container's full width,
              regardless of how many of these fields this tab shows. */}
          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ flex: "1 1 0", minWidth: 160 }}>
              <AsyncProjectMultiSelect
                id="timecards-filter-project"
                label="Project"
                values={filterProject}
                onChange={setFilterProject}
                nameSeed={projectNameSeed}
              />
            </Box>
            <Box sx={{ flex: "1 1 0", minWidth: 160 }}>
              <SearchableMultiSelect
                id="timecards-filter-work-item"
                label="Work item"
                placeholder="Search work items…"
                values={filterWorkItem}
                options={workItemOptions}
                onChange={setFilterWorkItem}
              />
            </Box>
            {engineerSlot && <Box sx={{ flex: "1 1 0", minWidth: 160 }}>{engineerSlot}</Box>}
            {!hideStateFilter && (
              <Box sx={{ flex: "1 1 0", minWidth: 160 }}>
                <TextField
                  select
                  fullWidth
                  size="small"
                  label="State"
                  value={filterState}
                  onChange={(e) => setFilterState(e.target.value as TimeCardState | "")}
                  slotProps={{
                    // oxygen-ui's own theme shifts an unshrunk label up by
                    // `top: -7px` for any Select-backed field (see
                    // `MultiSelectField.tsx`'s doc comment) -- tie `shrink`
                    // to whether a state is actually picked, rather than
                    // MUI's focus-driven default.
                    inputLabel: {
                      shrink: filterState !== "",
                      sx: { top: "0px !important" },
                    },
                    select: { notched: filterState !== "" },
                  }}
                >
                  <MenuItem value="">All states</MenuItem>
                  {FILTER_STATES.map((s) => (
                    <MenuItem key={s} value={s}>
                      {TIME_CARD_STATE_META[s].label}
                    </MenuItem>
                  ))}
                </TextField>
              </Box>
            )}
          </Box>

          {/* Row 2: the work-date range, its own full-width row — each
              picker flexes to half the container instead of sitting
              compact-width with empty space trailing after it. */}
          <Box>
            <Typography
              variant="caption"
              color="text.secondary"
              sx={{ mb: 0.75, display: "block" }}
            >
              Work date
            </Typography>
            <LocalizationProvider dateAdapter={AdapterDateFns}>
              <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
                <Box sx={{ flex: "1 1 0", minWidth: 160 }}>
                  <DatePicker
                    label="From"
                    value={parseDateOnly(filterFrom)}
                    maxDate={parseDateOnly(filterTo) ?? undefined}
                    onChange={(date) =>
                      setFilterFrom(
                        date instanceof Date && !Number.isNaN(date.getTime())
                          ? formatDateOnly(date)
                          : "",
                      )
                    }
                    slotProps={{
                      textField: { size: "small", fullWidth: true },
                      field: { clearable: true },
                    }}
                  />
                </Box>
                <Box sx={{ flex: "1 1 0", minWidth: 160 }}>
                  <DatePicker
                    label="To"
                    value={parseDateOnly(filterTo)}
                    minDate={parseDateOnly(filterFrom) ?? undefined}
                    onChange={(date) =>
                      setFilterTo(
                        date instanceof Date && !Number.isNaN(date.getTime())
                          ? formatDateOnly(date)
                          : "",
                      )
                    }
                    slotProps={{
                      textField: { size: "small", fullWidth: true },
                      field: { clearable: true },
                    }}
                  />
                </Box>
              </Box>
            </LocalizationProvider>
          </Box>
          {activeCount > 0 && (
            <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
              <Typography variant="caption" color="text.secondary">
                {activeCount} {activeCount === 1 ? "filter" : "filters"} active
              </Typography>
            </Box>
          )}
        </>
      )}
    </Paper>
  );
}

/** Downloads whatever `cards` the caller currently has loaded — a "current
 * page" export, not a full report (see the pagination notes on
 * `searchTimeCards` in `useTimeSheets.ts` for why a full-scope export isn't
 * reliable yet). Disabled when there's nothing to export. */
function ExportCsvButton({
  cards,
  filename,
}: {
  cards: CsmTimeCard[];
  filename: string;
}): JSX.Element {
  return (
    <Button
      size="small"
      variant="text"
      startIcon={<Download size={14} />}
      disabled={cards.length === 0}
      onClick={() => exportTimeCardsCsv(cards, filename)}
    >
      Export CSV
    </Button>
  );
}

/**
 * Switches the table between clustering rows by case or by engineer (see
 * `groupTimeCards`) — only shown on All/Approvals, where more than one
 * engineer's cards can appear together. "My time sheets" stays grouped by
 * case only, since every card there already belongs to the signed-in user.
 */
function GroupByToggle({
  value,
  onChange,
}: {
  value: TimeCardGroupBy;
  onChange: (v: TimeCardGroupBy) => void;
}): JSX.Element {
  const isEngineer = value === "engineer";
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
      <Typography variant="body2" color="text.secondary">
        Group by
      </Typography>
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
        <Typography
          variant="body2"
          color={isEngineer ? "text.secondary" : "text.primary"}
          sx={{ fontWeight: isEngineer ? 400 : 600 }}
        >
          Case
        </Typography>
        <Switch
          size="small"
          checked={isEngineer}
          onChange={(e) => onChange(e.target.checked ? "engineer" : "case")}
          inputProps={{ "aria-label": "Group by Case or Engineer" }}
        />
        <Typography
          variant="body2"
          color={isEngineer ? "text.primary" : "text.secondary"}
          sx={{ fontWeight: isEngineer ? 600 : 400 }}
        >
          Engineer
        </Typography>
      </Box>
    </Box>
  );
}
