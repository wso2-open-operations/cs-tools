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

import { useMemo, useState, type ChangeEvent, type JSX } from "react";
import {
  AdapterDateFns,
  Box,
  DatePickers,
  Divider,
  MenuItem,
  Paper,
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
  useCurrentEngineer,
  useDecideCard,
  useMyTimeCards,
  type TimeCardPagination,
} from "@features/csm-timecards/api/useTimeSheets";
import { useProjectOptions } from "@features/csm-cases/api/useProjectOptions";
import { BackendApiError } from "@api/backend/client";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import type { BeProject } from "@api/backend/types";
import { TIME_CARD_STATE_META } from "@features/csm-timecards/constants/timeCardConstants";
import { useTimecardRole } from "@features/csm-timecards/hooks/useTimecardRole";
import TimeCardsTable from "@features/csm-timecards/components/TimeCardsTable";
import TimeCardReviewDialog from "@features/csm-timecards/components/TimeCardReviewDialog";
import SearchableMultiSelect from "@components/SearchableMultiSelect";
import { exportTimeCardsCsv } from "@features/csm-timecards/utils/timeCardCsvExport";
import type { TimecardAction, TimecardRoleCtx } from "@features/csm-timecards/utils/timeSheetState";
import type { TimeCardGroupBy } from "@features/csm-timecards/utils/timeCardGrouping";
import type {
  CsmTimeCard,
  TimeCardSearchFilters,
  TimeCardState,
} from "@features/csm-timecards/types/timeCards";

/** Builds a `userId -> userName` lookup plus the option list a
 * `SearchableMultiSelect` engineer filter needs, scoped to whatever cards are
 * currently loaded for one tab (there's no engineer-search endpoint for time
 * cards, so — like the work-item filter — this only ever offers engineers
 * actually present on the current page, not the full directory). */
function engineerOptionsFrom(cards: CsmTimeCard[] | undefined): {
  ids: string[];
  nameById: Map<string, string>;
} {
  const nameById = new Map<string, string>();
  (cards ?? []).forEach((c) => nameById.set(c.userId, c.userName));
  return { ids: [...nameById.keys()], nameById };
}

/** Distinct case numbers present in whatever cards are currently loaded for
 * one tab — the option list for the work-item filter. Same "current page
 * only" caveat as {@link engineerOptionsFrom}. */
function workItemOptionsFrom(cards: CsmTimeCard[] | undefined): string[] {
  return Array.from(new Set((cards ?? []).map((c) => c.caseNumber)));
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
 * **Approvals** (approver/admin: approve/reject a submitted card). Logging
 * time happens from a case's Time tracking tab, not here. There's no
 * sheet-level bulk action, delegation, or reports — the backend has no
 * endpoints for those (see the module-level notes in `types/timeCards.ts`).
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

  const projects = useProjectOptions();
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
  };
  const handleFilterProjectChange = (v: string[]): void => {
    setFilterProject(v);
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
   * applied over an already-fetched page of cards. */
  const byWorkItem = (cards: CsmTimeCard[] | undefined): CsmTimeCard[] => {
    if (!cards) return [];
    if (filterWorkItem.length === 0) return cards;
    return cards.filter((c) => filterWorkItem.includes(c.caseNumber));
  };

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
  const allEngineerOptions = useMemo(
    () => engineerOptionsFrom(allCards.data?.cards),
    [allCards.data],
  );
  const approvalsEngineerOptions = useMemo(
    () => engineerOptionsFrom(queue.data?.cards),
    [queue.data],
  );

  // Filtered cards per tab, computed once and shared between the FilterBar's
  // export action and the table rendering below — rather than recomputing
  // (and risking drift) in two places. Engineer is already applied
  // server-side (via filtersWithEngineer) by the time allCards/queue resolve
  // — only work item still needs a client-side pass here.
  const mineFilteredCards = byWorkItem(myCards.data?.cards);
  const allFilteredCards = byWorkItem(allCards.data?.cards);
  const approvalsFilteredCards = byWorkItem(queue.data?.cards);

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
        onChange={(_, v) => setTab(v as TabId)}
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
            projects={projects.data ?? []}
            filterProject={filterProject}
            setFilterProject={handleFilterProjectChange}
            filterWorkItem={filterWorkItem}
            setFilterWorkItem={setFilterWorkItem}
            workItemOptions={mineWorkItemOptions}
            filterState={filterState}
            setFilterState={handleFilterStateChange}
            filterFrom={filterFrom}
            setFilterFrom={handleFilterFromChange}
            filterTo={filterTo}
            setFilterTo={handleFilterToChange}
            onClear={clearFilters}
          />

          {/* myCards stays disabled until `projects` resolves a project
           scope (see scopeProjectIds above), so its own isLoading never
           turns true during that wait — fold projects.isLoading in too, or
           this would flash the empty state before real data has a chance to
           load. The filter bar itself renders regardless, same as Approvals
           below, so it's never hidden behind this spinner. */}
          {myCards.isError ? (
            <Typography color="error">Could not load your time cards.</Typography>
          ) : (
            <>
              <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
                <ExportCsvButton
                  cards={mineFilteredCards}
                  filename={`time-cards-my-sheets-${todayStamp}.csv`}
                />
              </Box>
              <TimeCardsTable
                cards={mineFilteredCards}
                isLoading={myCards.isLoading || projects.isLoading}
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
            projects={projects.data ?? []}
            filterProject={filterProject}
            setFilterProject={handleFilterProjectChange}
            filterWorkItem={filterWorkItem}
            setFilterWorkItem={setFilterWorkItem}
            workItemOptions={allWorkItemOptions}
            filterState={filterState}
            setFilterState={handleFilterStateChange}
            filterFrom={filterFrom}
            setFilterFrom={handleFilterFromChange}
            filterTo={filterTo}
            setFilterTo={handleFilterToChange}
            onClear={clearFilters}
            engineerSlot={
              <SearchableMultiSelect
                id="timecards-filter-engineer-all"
                label="Engineer"
                placeholder="Search engineers…"
                values={filterEngineer}
                options={allEngineerOptions.ids}
                formatOption={(id) => allEngineerOptions.nameById.get(id) ?? id}
                onChange={handleFilterEngineerChange}
              />
            }
            engineerActive={filterEngineer.length > 0}
          />

          <GroupByToggle value={groupBy} onChange={setGroupBy} />

          {allCards.isError ? (
            <Typography color="error">Could not load time cards.</Typography>
          ) : (
            <>
              <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
                <ExportCsvButton
                  cards={allFilteredCards}
                  filename={`time-cards-all-${todayStamp}.csv`}
                />
              </Box>
              <TimeCardsTable
                cards={allFilteredCards}
                isLoading={allCards.isLoading || projects.isLoading}
                groupBy={groupBy}
                showCaseEngineerColumns
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
            projects={projects.data ?? []}
            filterProject={filterProject}
            setFilterProject={handleFilterProjectChange}
            filterWorkItem={filterWorkItem}
            setFilterWorkItem={setFilterWorkItem}
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
              <SearchableMultiSelect
                id="timecards-filter-engineer-approvals"
                label="Engineer"
                placeholder="Search engineers…"
                values={filterEngineer}
                options={approvalsEngineerOptions.ids}
                formatOption={(id) => approvalsEngineerOptions.nameById.get(id) ?? id}
                onChange={handleFilterEngineerChange}
              />
            }
            engineerActive={filterEngineer.length > 0}
          />

          <GroupByToggle value={groupBy} onChange={setGroupBy} />

          {queue.isError ? (
            <Typography color="error">Could not load the approval queue.</Typography>
          ) : (
            <>
              <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
                <ExportCsvButton
                  cards={approvalsFilteredCards}
                  filename={`time-cards-approvals-${todayStamp}.csv`}
                />
              </Box>
              <TimeCardsTable
                cards={approvalsFilteredCards}
                isLoading={queue.isLoading || projects.isLoading}
                groupBy={groupBy}
                showCaseEngineerColumns
                showActionsColumn
                roleFor={approvalsRole}
                onCardAction={handleCardAction}
                emptyText={anyFilterActive ? "No time cards match the current filters." : "Nothing awaiting approval."}
              />
              <TablePagination
                component="div"
                count={queue.data?.total ?? 0}
                page={approvalsPagination.pagination.page}
                onPageChange={approvalsPagination.onPageChange}
                rowsPerPage={approvalsPagination.pagination.rowsPerPage}
                onRowsPerPageChange={approvalsPagination.onRowsPerPageChange}
                rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
                showFirstButton
                showLastButton
              />
            </>
          )}
        </Box>
      )}

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
  projects,
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
  projects: BeProject[];
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
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);

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
              <SearchableMultiSelect
                id="timecards-filter-project"
                label="Project"
                placeholder="Search projects…"
                values={filterProject}
                options={projects.map((p) => p.id)}
                formatOption={(id) => projects.find((p) => p.id === id)?.name ?? id}
                onChange={setFilterProject}
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
  const options: { value: TimeCardGroupBy; label: string }[] = [
    { value: "case", label: "Case" },
    { value: "engineer", label: "Engineer" },
  ];
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
      <Typography variant="body2" color="text.secondary">
        Group by
      </Typography>
      <Box sx={{ display: "flex", gap: 0.5 }}>
        {options.map((o) => (
          <Button
            key={o.value}
            size="small"
            variant={value === o.value ? "contained" : "outlined"}
            onClick={() => onChange(o.value)}
          >
            {o.label}
          </Button>
        ))}
      </Box>
    </Box>
  );
}
