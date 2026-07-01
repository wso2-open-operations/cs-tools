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

import { useMemo, useState, type JSX } from "react";
import {
  Alert,
  AlertTitle,
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  InputAdornment,
  MenuItem,
  Tab,
  Tabs,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ListFilter, Search, X } from "@wso2/oxygen-ui-icons-react";
import {
  useApprovalQueue,
  useApproveSheet,
  useDecideCard,
  useDeleteCard,
  useDelegation,
  useMyTimeSheets,
  useProcessCard,
  useRecallCard,
  useRecallSheet,
  useRejectSheet,
  useSetDelegation,
  useSubmitSheet,
  useUpdateCard,
} from "@features/csm-timecards/api/useTimeSheets";
import { useProjectOptions } from "@features/csm-cases/api/useProjectOptions";
import type { BeProject } from "@api/backend/types";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import { timeCardNotices } from "@features/csm-timecards/utils/timeCardNotifications";
import { weekStartOf } from "@features/csm-timecards/utils/timeSheetWeek";
import {
  breakdownSummary,
  TIME_CARD_STATE_META,
} from "@features/csm-timecards/constants/timeCardConstants";
import { useTimecardRole } from "@features/csm-timecards/hooks/useTimecardRole";
import TimeSheetCard from "@features/csm-timecards/components/TimeSheetCard";
import TimeCardReviewDialog from "@features/csm-timecards/components/TimeCardReviewDialog";
import DelegateApprovalsDialog from "@features/csm-timecards/components/DelegateApprovalsDialog";
import LogTimeCardDialog from "@features/csm-timecards/components/LogTimeCardDialog";
import {
  type SheetAction,
  type TimecardAction,
} from "@features/csm-timecards/utils/timeSheetState";
import type {
  CsmTimeCard,
  CsmTimeSheet,
  TimeCardSearchFilters,
  TimeCardState,
} from "@features/csm-timecards/types/timeCards";

type TabId = "mine" | "approvals";

/**
 * Time cards workspace. Three tabs: **My time sheets** (weekly grouping, submit,
 * edit/resubmit), **Approvals** (approver/admin: approve/reject/recall, with
 * delegation), and **Reports** (KPIs + exceptions). Logging a new card opens the
 * log dialog; logging from a case lands the card in that week's sheet.
 */
export default function CsmTimeCardsPage(): JSX.Element {
  const role = useTimecardRole();
  const [tab, setTab] = useState<TabId>("mine");
  const activeTab: TabId = tab !== "mine" && !role.isApprover ? "mine" : tab;

  const [editingCard, setEditingCard] = useState<CsmTimeCard | null>(null);
  const [reviewCard, setReviewCard] = useState<CsmTimeCard | null>(null);
  const [delegateOpen, setDelegateOpen] = useState(false);
  const [deletingCard, setDeletingCard] = useState<CsmTimeCard | null>(null);

  // Search filters (sent as a POST body, never query params). Project, work
  // item and state scope both "My time sheets" and "Approvals"; Approvals adds
  // an engineer filter.
  const [filterProject, setFilterProject] = useState("");
  const [filterWorkItem, setFilterWorkItem] = useState("");
  const [filterState, setFilterState] = useState<TimeCardState | "">("");
  const [filterEngineer, setFilterEngineer] = useState("");

  const baseFilters: TimeCardSearchFilters = {
    ...(filterProject && { projectIds: [filterProject] }),
    ...(filterWorkItem.trim() && { workItemId: filterWorkItem.trim() }),
    ...(filterState && { states: [filterState] }),
  };

  const mySheets = useMyTimeSheets(baseFilters);
  const queue = useApprovalQueue(role.isApprover, baseFilters);
  const delegation = useDelegation();
  const projects = useProjectOptions();

  const update = useUpdateCard();
  const submitSheet = useSubmitSheet();
  const approveSheet = useApproveSheet();
  const rejectSheet = useRejectSheet();
  const recallSheet = useRecallSheet();
  const decideCard = useDecideCard();
  const recallCard = useRecallCard();
  const processCard = useProcessCard();
  const deleteCard = useDeleteCard();
  const setDelegation = useSetDelegation();

  const activeDelegation = delegation.data ?? null;
  const approverDisabled = !!activeDelegation;

  const anyFilterActive =
    !!filterProject || !!filterWorkItem.trim() || !!filterState || !!filterEngineer.trim();
  const clearFilters = (): void => {
    setFilterProject("");
    setFilterWorkItem("");
    setFilterState("");
    setFilterEngineer("");
  };

  const handleSheetAction = (sheet: CsmTimeSheet, action: SheetAction): void => {
    const target = { userId: sheet.userId, weekStart: sheet.weekStart };
    if (action === "submit") submitSheet.mutate(target);
    else if (action === "approve") approveSheet.mutate(target);
    else if (action === "reject") rejectSheet.mutate(target);
    else if (action === "recall") recallSheet.mutate(target);
  };

  const handleCardAction = (card: CsmTimeCard, action: TimecardAction): void => {
    if (action === "edit") setEditingCard(card);
    else if (action === "delete") setDeletingCard(card);
    else if (action === "submit" || action === "resubmit")
      submitSheet.mutate({ userId: card.userId, weekStart: weekStartOf(card.date) });
    else if (action === "approve" || action === "reject") setReviewCard(card);
    else if (action === "recall") recallCard.mutate(card.id);
    else if (action === "process") processCard.mutate(card.id);
  };

  const [dismissedNoticeIds, setDismissedNoticeIds] = useState<Set<string>>(new Set());
  const allNotices = timeCardNotices(mySheets.data ?? []);
  const notices = allNotices.filter((n) => !dismissedNoticeIds.has(n.cardId));

  /** Flat list of the signed-in user's cards — used for duplicate-entry warnings. */
  const myCards = (mySheets.data ?? [])
    .flatMap((s) => s.cards)
    .sort((a, b) => b.date.localeCompare(a.date));

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
        {role.isApprover && <Tab value="approvals" label="Approvals" />}
      </Tabs>

      {/* My time sheets */}
      {activeTab === "mine" &&
        (mySheets.isLoading ? (
          <Centered>
            <CircularProgress />
          </Centered>
        ) : mySheets.isError ? (
          <Typography color="error">Could not load your time sheets.</Typography>
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            {notices.length > 0 && (
              <Alert
                severity={notices[0].severity}
                onClose={() =>
                  setDismissedNoticeIds(
                    new Set([...dismissedNoticeIds, ...notices.map((n) => n.cardId)]),
                  )
                }
              >
                <AlertTitle>
                  {notices.length === 1
                    ? "A time card needs your attention"
                    : `${notices.length} time cards need your attention`}
                </AlertTitle>
                <Box component="ul" sx={{ m: 0, pl: 2.5 }}>
                  {notices.slice(0, 4).map((n) => (
                    <li key={n.cardId}>{n.message}</li>
                  ))}
                </Box>
              </Alert>
            )}

            <FilterBar
              projects={projects.data ?? []}
              filterProject={filterProject}
              setFilterProject={setFilterProject}
              filterWorkItem={filterWorkItem}
              setFilterWorkItem={setFilterWorkItem}
              filterState={filterState}
              setFilterState={setFilterState}
              onClear={clearFilters}
              engineerSlot={
                <EngineerFilter value={filterEngineer} onChange={setFilterEngineer} />
              }
            />

            {(() => {
              const q = filterEngineer.trim().toLowerCase();
              const filtered = (mySheets.data ?? []).filter(
                (s) => !q || s.userName.toLowerCase().includes(q),
              );
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
                  onSheetAction={handleSheetAction}
                  onCardAction={handleCardAction}
                />
              ));
            })()}
          </Box>
        ))}

      {/* Approvals */}
      {activeTab === "approvals" && role.isApprover && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 1,
            }}
          >
            <Typography variant="subtitle2">Submitted for your approval</Typography>
            <Button variant="outlined" size="small" onClick={() => setDelegateOpen(true)}>
              {activeDelegation ? "Manage delegation" : "Delegate approvals"}
            </Button>
          </Box>

          <FilterBar
            projects={projects.data ?? []}
            filterProject={filterProject}
            setFilterProject={setFilterProject}
            filterWorkItem={filterWorkItem}
            setFilterWorkItem={setFilterWorkItem}
            filterState={filterState}
            setFilterState={setFilterState}
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
          />

          {activeDelegation && (
            <Alert severity="info">
              Approvals delegated to <strong>{activeDelegation.delegateName}</strong>{" "}
              ({activeDelegation.from} → {activeDelegation.to}). Your accept/reject
              is paused for this period.
            </Alert>
          )}

          {queue.isLoading ? (
            <Centered>
              <CircularProgress />
            </Centered>
          ) : queue.isError ? (
            <Typography color="error">Could not load the approval queue.</Typography>
          ) : (queue.data ?? []).length === 0 ? (
            <Empty text="Nothing awaiting approval." />
          ) : (
            (() => {
              const q = filterEngineer.trim().toLowerCase();
              const filtered = (queue.data ?? []).filter(
                (s) => !q || s.userName.toLowerCase().includes(q),
              );
              if (filtered.length === 0) {
                return <Empty text={`No engineers match “${filterEngineer}”.`} />;
              }
              return filtered.map((s) => (
                <TimeSheetCard
                  key={s.id}
                  sheet={s}
                  role={{ isOwner: false, isApprover: true, isAdmin: role.isAdmin }}
                  showEngineer
                  approverDisabled={approverDisabled}
                  onSheetAction={handleSheetAction}
                  onCardAction={handleCardAction}
                />
              ));
            })()
          )}
        </Box>
      )}

      {/* Dialogs. Logging new cards happens from a case; here we only edit an
          existing (rejected/recalled) card, review, delegate, or delete. */}
      {editingCard && (
        <LogTimeCardDialog
          editing={editingCard}
          existingCards={myCards}
          isSubmitting={update.isPending}
          onClose={() => setEditingCard(null)}
          onSubmit={(input) =>
            update.mutate(
              {
                cardId: editingCard.id,
                patch: {
                  date: input.date,
                  category: input.category,
                  billable: input.billable,
                  breakdown: input.breakdown,
                  workLogComment: input.workLogComment,
                  issueComplexity: input.issueComplexity,
                },
              },
              { onSuccess: () => setEditingCard(null) },
            )
          }
        />
      )}

      {reviewCard && (
        <TimeCardReviewDialog
          card={reviewCard}
          isDeciding={decideCard.isPending}
          onClose={() => setReviewCard(null)}
          onDecide={(decision) =>
            decideCard.mutate(decision, { onSuccess: () => setReviewCard(null) })
          }
        />
      )}

      {delegateOpen && (
        <DelegateApprovalsDialog
          current={activeDelegation}
          isSaving={setDelegation.isPending}
          onClose={() => setDelegateOpen(false)}
          onSave={(input) =>
            setDelegation.mutate(input, { onSuccess: () => setDelegateOpen(false) })
          }
        />
      )}

      {deletingCard && (
        <Dialog open onClose={() => setDeletingCard(null)} maxWidth="xs" fullWidth>
          <DialogTitle>Delete time card?</DialogTitle>
          <DialogContent>
            <DialogContentText sx={{ mb: 1.5 }}>
              This permanently removes the entry below. This can&apos;t be undone.
            </DialogContentText>
            <Box
              sx={{
                bgcolor: "action.hover",
                borderRadius: 1,
                px: 2,
                py: 1.5,
                display: "flex",
                flexDirection: "column",
                gap: 0.5,
              }}
            >
              <Typography variant="body2">
                <strong>{deletingCard.caseNumber}</strong> &middot; {deletingCard.date}
              </Typography>
              <Typography variant="body2">
                {deletingCard.totalHours.toFixed(2)}h &middot; {deletingCard.category}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {breakdownSummary(deletingCard.breakdown)}
              </Typography>
            </Box>
          </DialogContent>
          <DialogActions>
            <Button
              color="inherit"
              onClick={() => setDeletingCard(null)}
              disabled={deleteCard.isPending}
            >
              Cancel
            </Button>
            <Button
              color="error"
              variant="contained"
              disabled={deleteCard.isPending}
              onClick={() =>
                deleteCard.mutate(deletingCard.id, {
                  onSuccess: () => setDeletingCard(null),
                })
              }
            >
              Delete
            </Button>
          </DialogActions>
        </Dialog>
      )}
    </Box>
  );
}

const FILTER_STATES: TimeCardState[] = [
  "pending",
  "submitted",
  "approved",
  "rejected",
  "recalled",
  "processed",
];

/** Searchable engineer picker used in both filter bars. */
function EngineerFilter({
  value,
  onChange,
}: {
  value: string;
  onChange: (name: string) => void;
}): JSX.Element {
  const [input, setInput] = useState("");
  const search = useDebouncedValue(input.trim(), 300);
  const { data } = useSearchUsers({
    ...(search.length > 0 && { searchQuery: search }),
    pagination: { limit: 6, offset: 0 },
  });

  const candidates = useMemo(
    () =>
      input.trim().length === 0
        ? []
        : (data?.users ?? [])
            .filter((u) => u.userType === "internal")
            .map((u) => ({
              id: u.id,
              name: u.name.trim() || u.userName,
              email: u.email,
            })),
    [data, input],
  );

  if (value) {
    return (
      <Chip
        label={value}
        size="small"
        onDelete={() => onChange("")}
        deleteIcon={<X size={14} />}
      />
    );
  }

  return (
    <Box sx={{ position: "relative" }}>
      <TextField
        size="small"
        label="Engineer"
        placeholder="Search…"
        value={input}
        onChange={(e) => setInput(e.target.value)}
        sx={{ width: 180 }}
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
      {input.trim().length > 0 && (
        <Box
          sx={{
            position: "absolute",
            top: "100%",
            left: 0,
            zIndex: 10,
            mt: 0.5,
            width: 240,
            border: 1,
            borderColor: "divider",
            borderRadius: 1,
            bgcolor: "background.paper",
            boxShadow: 2,
            maxHeight: 200,
            overflowY: "auto",
          }}
        >
          {candidates.length === 0 ? (
            <Typography
              variant="caption"
              color="text.secondary"
              sx={{ display: "block", p: 1 }}
            >
              No matching engineers.
            </Typography>
          ) : (
            candidates.map((u) => (
              <Button
                key={u.id}
                variant="text"
                color="inherit"
                onClick={() => {
                  onChange(u.name);
                  setInput("");
                }}
                sx={{
                  display: "flex",
                  justifyContent: "flex-start",
                  textTransform: "none",
                  width: "100%",
                  px: 1.5,
                  py: 0.75,
                  gap: 1,
                }}
              >
                <Box sx={{ minWidth: 0, textAlign: "left" }}>
                  <Typography variant="body2" noWrap>
                    {u.name}
                  </Typography>
                  {u.email && (
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      noWrap
                      sx={{ display: "block" }}
                    >
                      {u.email}
                    </Typography>
                  )}
                </Box>
              </Button>
            ))
          )}
        </Box>
      )}
    </Box>
  );
}

/** Shared filter bar for the My time sheets and Approvals tabs. */
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
}): JSX.Element {
  const activeChips: { key: string; label: string; onDelete: () => void }[] = [];
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
  if (filterState) {
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
