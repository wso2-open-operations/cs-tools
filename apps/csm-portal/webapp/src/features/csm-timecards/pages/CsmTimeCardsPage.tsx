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

import { useState, type ChangeEvent, type JSX } from "react";
import {
  Box,
  Chip,
  CircularProgress,
  InputAdornment,
  MenuItem,
  Tab,
  Tabs,
  TablePagination,
  TextField,
  Typography,
  Button,
} from "@wso2/oxygen-ui";
import { ListFilter, Search, X } from "@wso2/oxygen-ui-icons-react";
import {
  useAllTimeCards,
  useApprovalQueue,
  useCurrentEngineer,
  useDecideCard,
  useMyTimeSheets,
  type TimeCardPagination,
} from "@features/csm-timecards/api/useTimeSheets";
import { useProjectOptions } from "@features/csm-cases/api/useProjectOptions";
import { BackendApiError } from "@api/backend/client";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import type { BeProject } from "@api/backend/types";
import { TIME_CARD_STATE_META } from "@features/csm-timecards/constants/timeCardConstants";
import { useTimecardRole } from "@features/csm-timecards/hooks/useTimecardRole";
import TimeSheetCard from "@features/csm-timecards/components/TimeSheetCard";
import TimeCardReviewDialog from "@features/csm-timecards/components/TimeCardReviewDialog";
import type { TimecardAction } from "@features/csm-timecards/utils/timeSheetState";
import type {
  CsmTimeCard,
  CsmTimeSheet,
  TimeCardSearchFilters,
  TimeCardState,
} from "@features/csm-timecards/types/timeCards";

const DEFAULT_ROWS_PER_PAGE = 20;
// Top option is the backend's max page limit; larger requests are rejected.
const ROWS_PER_PAGE_OPTIONS = [10, 20, BE_MAX_PAGE_LIMIT];

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
  const [tab, setTab] = useState<TabId>("mine");
  const activeTab: TabId = tab === "approvals" && !role.isApprover ? "mine" : tab;

  const [reviewCard, setReviewCard] = useState<CsmTimeCard | null>(null);

  // Search filters (sent as a POST body, never query params). Project and
  // state are server-side; work item and engineer are client-side over the
  // returned page — the backend has no filter for either.
  const [filterProject, setFilterProject] = useState("");
  const [filterWorkItem, setFilterWorkItem] = useState("");
  const [filterState, setFilterState] = useState<TimeCardState | "">("");
  const [filterEngineer, setFilterEngineer] = useState("");

  const projects = useProjectOptions();
  // The backend requires a non-empty `projectIds` to return anything at all
  // (confirmed live — an unscoped search always returns `total: 0`, despite
  // the OpenAPI spec documenting it as optional). Default to every project
  // the user can see (already fetched for the filter dropdown below) so
  // "My time sheets" / "Approvals" work with no filter picked; the explicit
  // project filter narrows that down when set.
  const scopeProjectIds = filterProject
    ? [filterProject]
    : (projects.data ?? []).map((p) => p.id);

  const baseFilters: TimeCardSearchFilters = {
    ...(scopeProjectIds.length && { projectIds: scopeProjectIds }),
    ...(filterState && { states: [filterState] }),
  };

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

  const mySheets = useMyTimeSheets(activeTab === "mine", baseFilters, minePagination.pagination);
  const allCards = useAllTimeCards(activeTab === "all", baseFilters, allPagination.pagination);
  const queue = useApprovalQueue(
    activeTab === "approvals" && role.isApprover,
    baseFilters,
    approvalsPagination.pagination,
  );
  const decideCard = useDecideCard();

  const anyFilterActive =
    !!filterProject || !!filterWorkItem.trim() || !!filterState || !!filterEngineer.trim();

  // A filter change re-scopes the search for every tab, so every tab's page
  // position needs to reset too — otherwise "page 3" of a narrower result
  // set could be past the end, or just show unrelated leftovers.
  const resetAllPages = (): void => {
    minePagination.setPage(0);
    allPagination.setPage(0);
    approvalsPagination.setPage(0);
  };
  const handleFilterProjectChange = (v: string): void => {
    setFilterProject(v);
    resetAllPages();
  };
  const handleFilterStateChange = (v: TimeCardState | ""): void => {
    setFilterState(v);
    resetAllPages();
  };
  const clearFilters = (): void => {
    setFilterProject("");
    setFilterWorkItem("");
    setFilterState("");
    setFilterEngineer("");
    resetAllPages();
  };

  const handleCardAction = (card: CsmTimeCard, action: TimecardAction): void => {
    if (action === "approve" || action === "reject") setReviewCard(card);
  };

  /** Client-side work-item substring filter, applied over already-fetched sheets. */
  const byWorkItem = (sheets: CsmTimeSheet[] | undefined): CsmTimeSheet[] | undefined => {
    const q = filterWorkItem.trim().toLowerCase();
    if (!q || !sheets) return sheets;
    return sheets
      .map((s) => ({
        ...s,
        cards: s.cards.filter((c) => c.caseNumber.toLowerCase().includes(q)),
      }))
      .filter((s) => s.cards.length > 0);
  };

  return (
    <Box
      sx={{ p: { xs: 2, md: 3 }, display: "flex", flexDirection: "column", gap: 2 }}
    >
      <Box>
        <Typography variant="h5">Time cards</Typography>
        <Typography variant="body2" color="text.secondary">
          Review your weekly time sheets and (for approvers) submissions. Log
          time from a case&apos;s <strong>Time tracking</strong> tab.
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
            filterState={filterState}
            setFilterState={handleFilterStateChange}
            onClear={clearFilters}
          />

          {/* mySheets stays disabled until `projects` resolves a project
           scope (see scopeProjectIds above), so its own isLoading never
           turns true during that wait — fold projects.isLoading in too, or
           this would flash the empty state before real data has a chance to
           load. The filter bar itself renders regardless, same as Approvals
           below, so it's never hidden behind this spinner. */}
          {mySheets.isLoading || projects.isLoading ? (
            <Centered>
              <CircularProgress />
            </Centered>
          ) : mySheets.isError ? (
            <Typography color="error">Could not load your time sheets.</Typography>
          ) : (
            <>
              {(() => {
                const filtered = byWorkItem(mySheets.data?.sheets) ?? [];
                if (filtered.length === 0) {
                  return (
                    <Empty
                      text={
                        anyFilterActive
                          ? "No time cards match the current filters."
                          : "No time logged yet. Open a case and use its Time tracking tab to log time."
                      }
                    />
                  );
                }
                return filtered.map((s) => (
                  <TimeSheetCard
                    key={s.id}
                    sheet={s}
                    role={{ isOwner: true, isApprover: false, isAdmin: false }}
                    onCardAction={handleCardAction}
                  />
                ));
              })()}
              {/* Pages over raw records, not weekly sheets — a week's cards
               can legitimately span a page boundary and look incomplete
               until you've paged further; same for "my cards" being a small
               slice of whatever's on each raw page. Accepted tradeoff for
               real pagination instead of a 30-60s upfront full-scope fetch. */}
              <TablePagination
                component="div"
                count={mySheets.data?.total ?? 0}
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
            filterState={filterState}
            setFilterState={handleFilterStateChange}
            onClear={clearFilters}
            engineerSlot={
              <TextField
                size="small"
                label="Engineer"
                placeholder="Name…"
                value={filterEngineer}
                onChange={(e) => setFilterEngineer(e.target.value)}
                sx={{ width: 180 }}
              />
            }
            engineerChip={
              filterEngineer.trim()
                ? {
                    label: `Engineer: ${filterEngineer.trim()}`,
                    onDelete: () => setFilterEngineer(""),
                  }
                : undefined
            }
          />

          {allCards.isLoading || projects.isLoading ? (
            <Centered>
              <CircularProgress />
            </Centered>
          ) : allCards.isError ? (
            <Typography color="error">Could not load time cards.</Typography>
          ) : (
            <>
              {(() => {
                const q = filterEngineer.trim().toLowerCase();
                const byEngineer = (allCards.data?.sheets ?? []).filter(
                  (s) => !q || s.userName.toLowerCase().includes(q),
                );
                const filtered = byWorkItem(byEngineer) ?? [];
                if (filtered.length === 0) {
                  return (
                    <Empty
                      text={
                        q && byEngineer.length === 0
                          ? `No engineers match "${filterEngineer}".`
                          : anyFilterActive
                            ? "No time cards match the current filters."
                            : "No time logged yet."
                      }
                    />
                  );
                }
                return filtered.map((s) => (
                  <TimeSheetCard
                    key={s.id}
                    sheet={s}
                    role={{ isOwner: s.userId === me.id, isApprover: false, isAdmin: false }}
                    showEngineer
                    onCardAction={handleCardAction}
                  />
                ));
              })()}
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
          <Typography variant="subtitle2">Submitted for your approval</Typography>

          <FilterBar
            projects={projects.data ?? []}
            filterProject={filterProject}
            setFilterProject={handleFilterProjectChange}
            filterWorkItem={filterWorkItem}
            setFilterWorkItem={setFilterWorkItem}
            filterState={filterState}
            setFilterState={handleFilterStateChange}
            onClear={clearFilters}
            hideStateFilter
            engineerSlot={
              <TextField
                size="small"
                label="Engineer"
                placeholder="Name…"
                value={filterEngineer}
                onChange={(e) => setFilterEngineer(e.target.value)}
                sx={{ width: 180 }}
              />
            }
            engineerChip={
              filterEngineer.trim()
                ? {
                    label: `Engineer: ${filterEngineer.trim()}`,
                    onDelete: () => setFilterEngineer(""),
                  }
                : undefined
            }
          />

          {queue.isLoading || projects.isLoading ? (
            <Centered>
              <CircularProgress />
            </Centered>
          ) : queue.isError ? (
            <Typography color="error">Could not load the approval queue.</Typography>
          ) : (
            <>
              {(queue.data?.sheets ?? []).length === 0 ? (
                <Empty text="Nothing awaiting approval." />
              ) : (
                (() => {
                  const q = filterEngineer.trim().toLowerCase();
                  const byEngineer = (queue.data?.sheets ?? []).filter(
                    (s) => !q || s.userName.toLowerCase().includes(q),
                  );
                  const filtered = byWorkItem(byEngineer) ?? [];
                  if (filtered.length === 0) {
                    return (
                      <Empty
                        text={
                          // Only blame the engineer filter when it's the one that
                          // excluded everything — if it matched fine and the
                          // work-item filter emptied the result, say that instead.
                          q && byEngineer.length === 0
                            ? `No engineers match "${filterEngineer}".`
                            : "No time cards match the current filters."
                        }
                      />
                    );
                  }
                  return filtered.map((s) => (
                    <TimeSheetCard
                      key={s.id}
                      sheet={s}
                      role={{ isOwner: false, isApprover: true, isAdmin: role.isAdmin }}
                      showEngineer
                      onCardAction={handleCardAction}
                    />
                  ));
                })()
              )}
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

      {reviewCard && (
        <TimeCardReviewDialog
          card={reviewCard}
          isDeciding={decideCard.isPending}
          onClose={() => setReviewCard(null)}
          onDecide={(decision) =>
            decideCard.mutate(decision, {
              onSuccess: () => setReviewCard(null),
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

/** Shared filter bar for the My time sheets, All, and Approvals tabs. */
function FilterBar({
  projects,
  filterProject,
  setFilterProject,
  filterWorkItem,
  setFilterWorkItem,
  filterState,
  setFilterState,
  onClear,
  engineerSlot,
  engineerChip,
  hideStateFilter,
}: {
  projects: BeProject[];
  filterProject: string;
  setFilterProject: (v: string) => void;
  filterWorkItem: string;
  setFilterWorkItem: (v: string) => void;
  filterState: TimeCardState | "";
  setFilterState: (v: TimeCardState | "") => void;
  onClear: () => void;
  engineerSlot?: JSX.Element;
  /** Active-chip for the Engineer filter — only the Approvals tab has one. */
  engineerChip?: { label: string; onDelete: () => void };
  /** Approvals always forces `states: ["submitted"]` server-side (see
   * `useApprovalQueue`), so the State control can't actually narrow anything
   * there — hide it instead of showing a filter that silently does nothing. */
  hideStateFilter?: boolean;
}): JSX.Element {
  const activeChips: { key: string; label: string; onDelete: () => void }[] = [];
  if (engineerChip) activeChips.push({ key: "engineer", ...engineerChip });
  if (filterProject) {
    const name = projects.find((p) => p.id === filterProject)?.name ?? filterProject;
    activeChips.push({
      key: "project",
      label: `Project: ${name}`,
      onDelete: () => setFilterProject(""),
    });
  }
  if (filterWorkItem.trim()) {
    activeChips.push({
      key: "workItem",
      label: `Work item: ${filterWorkItem.trim()}`,
      onDelete: () => setFilterWorkItem(""),
    });
  }
  if (!hideStateFilter && filterState) {
    activeChips.push({
      key: "state",
      label: `State: ${TIME_CARD_STATE_META[filterState].label}`,
      onDelete: () => setFilterState(""),
    });
  }

  return (
    <Box
      sx={{
        border: 1,
        borderColor: "divider",
        borderRadius: 2,
        bgcolor: "background.paper",
        overflow: "hidden",
      }}
    >
      {/* Controls row */}
      <Box
        sx={{
          display: "flex",
          flexWrap: "wrap",
          alignItems: "center",
          gap: 1.5,
          px: 2,
          py: 1.5,
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 0.75, color: "text.secondary", mr: 0.5 }}>
          <ListFilter size={16} />
          <Typography variant="caption" fontWeight={600} color="text.secondary">
            Filters
          </Typography>
        </Box>

        <TextField
          select
          size="small"
          label="Project"
          value={filterProject}
          onChange={(e) => setFilterProject(e.target.value)}
          sx={{ width: 200 }}
        >
          <MenuItem value="">All projects</MenuItem>
          {projects.map((p) => (
            <MenuItem key={p.id} value={p.id}>
              {p.name ?? p.id}
            </MenuItem>
          ))}
        </TextField>

        <TextField
          size="small"
          label="Work item"
          placeholder="e.g. CS0352584"
          value={filterWorkItem}
          onChange={(e) => setFilterWorkItem(e.target.value)}
          sx={{ width: 190 }}
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <Search size={14} />
                </InputAdornment>
              ),
            },
          }}
        />

        {engineerSlot}

        {!hideStateFilter && (
          <TextField
            select
            size="small"
            label="State"
            value={filterState}
            onChange={(e) => setFilterState(e.target.value as TimeCardState | "")}
            sx={{ width: 150 }}
          >
            <MenuItem value="">All states</MenuItem>
            {FILTER_STATES.map((s) => (
              <MenuItem key={s} value={s}>
                {TIME_CARD_STATE_META[s].label}
              </MenuItem>
            ))}
          </TextField>
        )}

        <Box sx={{ flexGrow: 1 }} />
      </Box>

      {/* Active filter chips */}
      {activeChips.length > 0 && (
        <Box
          sx={{
            display: "flex",
            flexWrap: "wrap",
            alignItems: "center",
            gap: 0.75,
            px: 2,
            py: 1,
            borderTop: 1,
            borderColor: "divider",
            bgcolor: "action.hover",
          }}
        >
          <Typography variant="caption" color="text.disabled" sx={{ mr: 0.5 }}>
            Active:
          </Typography>
          {activeChips.map((c) => (
            <Chip
              key={c.key}
              label={c.label}
              size="small"
              onDelete={c.onDelete}
              deleteIcon={<X size={12} />}
              sx={{ height: 22, fontSize: "0.72rem" }}
            />
          ))}
          <Button
            size="small"
            variant="text"
            onClick={onClear}
            sx={{ ml: 0.5, fontSize: "0.72rem", py: 0, minHeight: 0 }}
          >
            Clear all
          </Button>
        </Box>
      )}
    </Box>
  );
}

function Centered({ children }: { children: JSX.Element }): JSX.Element {
  return (
    <Box sx={{ display: "flex", justifyContent: "center", py: 6 }}>{children}</Box>
  );
}

function Empty({ text }: { text: string }): JSX.Element {
  return (
    <Typography variant="body2" color="text.secondary" sx={{ py: 4 }}>
      {text}
    </Typography>
  );
}
