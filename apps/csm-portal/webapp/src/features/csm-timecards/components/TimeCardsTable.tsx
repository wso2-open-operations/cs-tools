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

import { Fragment, useState, type JSX } from "react";
import { Box, Checkbox, IconButton, Skeleton, Tooltip, Typography } from "@wso2/oxygen-ui";
import { Check, Eye, Pencil, Trash2, X } from "@wso2/oxygen-ui-icons-react";
import RelativeDate from "@components/RelativeDate";
import TimeCardCasePreviewDrawer from "@features/csm-timecards/components/TimeCardCasePreviewDrawer";
import TimeCardStatusChip from "@features/csm-timecards/components/TimeCardStatusChip";
import { groupTimeCards, type TimeCardGroupBy } from "@features/csm-timecards/utils/timeCardGrouping";
import {
  cardActions,
  type TimecardAction,
  type TimecardRoleCtx,
} from "@features/csm-timecards/utils/timeSheetState";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

interface TimeCardsTableProps {
  cards: CsmTimeCard[];
  isLoading: boolean;
  /** Number of skeleton rows to show while loading. Defaults to 6. */
  skeletonCount?: number;
  emptyText: string;
  /** Which field rows are clustered by — cards for the same case (or the same
   * engineer) sort adjacent to each other; whether that grouping key has its
   * own visible column is a separate decision (see `showCaseColumn` /
   * `showEngineerColumn` below), this prop only controls ordering. */
  groupBy: TimeCardGroupBy;
  /** Show the Case column. On for all three tabs, including "My time
   * sheets" — a personal sheet still spans every case the engineer worked
   * on, so the case number is the most useful column on the row, not the
   * least. Defaults to on; only tests turn it off. */
  showCaseColumn?: boolean;
  /** Show the Engineer column. Off on "My time sheets", where every card
   * already belongs to the signed-in user and an Engineer column would be
   * redundant; on for "All" and "Approvals", which span multiple engineers. */
  showEngineerColumn?: boolean;
  /** Show the Approve/Reject buttons alongside the view-details eye icon in
   * the Actions column. Off outside the Approvals tab, where `roleFor`
   * already makes `cardActions` return none per row anyway — this just
   * avoids reserving the extra button space for it. The Actions column
   * itself (eye icon) is always present. */
  showActionsColumn?: boolean;
  /** Per-card role context — varies per row on "All" and "Approvals" (isOwner
   * depends on who submitted that specific card; an admin's approve-by-
   * exception queue isn't scoped to exclude their own cards server-side, so
   * Approvals needs this too — see useApprovalQueue's own doc comment),
   * constant on "My time sheets". */
  roleFor: (card: CsmTimeCard) => TimecardRoleCtx;
  onCardAction: (card: CsmTimeCard, action: TimecardAction) => void;
  /** Adds a leading checkbox column for multi-select bulk approve — Approvals
   * tab only. A row only gets a checkbox when its own `cardActions` include
   * "approve" (same eligibility check the per-row Approve button already
   * uses); every other row gets an empty placeholder cell instead, so the
   * grid's columns stay aligned. Omitted/false renders no checkbox column at
   * all, byte-for-byte the same as before this prop existed. */
  selectable?: boolean;
  selectedIds?: Set<string>;
  onToggleSelect?: (card: CsmTimeCard) => void;
  /** Called with the current page's full set of selectable cards (not just
   * the ones already selected) — the caller decides select-all vs
   * clear-all based on whether every one of those is already selected. */
  onToggleSelectAll?: (selectableCards: CsmTimeCard[]) => void;
}

// "edit"/"delete" aren't in here — unlike approve/reject, they each render as
// their own icon button (matching the Eye view icon) shown unconditionally
// when a card is eligible, not as an icon button gated by showActionsColumn
// (see below).
const ACTION_BUTTONS: Record<
  Exclude<TimecardAction, "edit" | "delete">,
  { label: string; color: "primary" | "error"; icon: JSX.Element }
> = {
  approve: { label: "Approve", color: "primary", icon: <Check size={16} /> },
  reject: { label: "Reject", color: "error", icon: <X size={16} /> },
};

/**
 * Time cards as a grid-based table (matches `CasesList.tsx`'s visual
 * convention). One row per card, flat — Case and Engineer are regular
 * columns, not a section header; `groupBy` only decides sort order (cards for
 * the same case, or the same engineer, end up adjacent), via `groupTimeCards`.
 */
export default function TimeCardsTable({
  cards,
  isLoading,
  skeletonCount = 6,
  emptyText,
  groupBy,
  showCaseColumn = true,
  showEngineerColumn = false,
  showActionsColumn = false,
  roleFor,
  onCardAction,
  selectable = false,
  selectedIds,
  onToggleSelect,
  onToggleSelectAll,
}: TimeCardsTableProps): JSX.Element {
  // The card currently open in the quick-review drawer — local to this
  // table. Its own Approve/Reject buttons still hand the actual decision off
  // to `onCardAction` (closing this drawer first — see
  // `TimeCardCasePreviewDrawer`'s own doc comment on why), the same as the
  // row-level buttons; no mutation state lives here.
  const [detailCard, setDetailCard] = useState<CsmTimeCard | null>(null);

  const headerCells = [
    "Preview",
    ...(showCaseColumn ? ["Case"] : []),
    ...(showEngineerColumn ? ["Engineer"] : []),
    "Project",
    "Date",
    "Minutes",
    "State",
    "Actions",
  ];
  const grid = [
    ...(selectable ? ["40px"] : []),
    "auto",
    ...(showCaseColumn ? ["minmax(120px, 0.9fr)"] : []),
    ...(showEngineerColumn ? ["minmax(140px, 1fr)"] : []),
    "minmax(160px, 1.4fr)",
    "minmax(90px, 0.6fr)",
    "minmax(90px, 0.6fr)",
    "minmax(140px, 1fr)",
    "auto",
  ].join(" ");

  // groupTimeCards clusters + sorts (newest-active group first, newest card
  // first within it) — flattened back into one ordered list since rows no
  // longer render a group header, just Case/Engineer as columns.
  const orderedCards = groupTimeCards(cards, groupBy).flatMap((g) => g.cards);
  // Same eligibility check the per-row Approve button uses (line ~207 below)
  // — computed once up front so the header's select-all checkbox can reflect
  // "every selectable row on this page", not every row.
  const selectableCards = selectable
    ? orderedCards.filter((c) => cardActions(c.state, roleFor(c)).includes("approve"))
    : [];
  const allSelected =
    selectableCards.length > 0 && selectableCards.every((c) => selectedIds?.has(c.id));
  const someSelected = selectableCards.some((c) => selectedIds?.has(c.id));
  // Clicking a row's own Approve/Reject while the checkbox column is in use
  // mixes two interaction modes at once — direct single-row action alongside
  // bulk selection — which reads as confusing even with just one row
  // checked (per review feedback: "otherwise user gets confused"). Every
  // row's own action buttons disable as soon as ANY row is selected, not
  // just once there are 2+, forcing the bulk toolbar (or clearing the
  // selection) to be the only way forward while a selection is active.
  const selectionActive = (selectedIds?.size ?? 0) > 0;

  return (
    <Fragment>
      <Box
        role="table"
        sx={{
          border: 1,
          borderColor: "divider",
          borderRadius: 1,
          overflow: "hidden",
          display: "grid",
          gridTemplateColumns: grid,
          columnGap: 2,
        }}
      >
        <Box
          role="row"
          sx={{
            gridColumn: "1 / -1",
            display: "grid",
            gridTemplateColumns: "subgrid",
            columnGap: 2,
            alignItems: "center",
            px: 2,
            py: 1.25,
            bgcolor: "action.hover",
            borderBottom: 1,
            borderColor: "divider",
          }}
        >
          {selectable && (
            <Box role="columnheader" sx={{ display: "flex", alignItems: "center" }}>
              <Checkbox
                size="small"
                checked={allSelected}
                indeterminate={someSelected && !allSelected}
                disabled={selectableCards.length === 0}
                onChange={() => onToggleSelectAll?.(selectableCards)}
                inputProps={{ "aria-label": "Select all approvable time cards on this page" }}
                sx={{ p: 0 }}
              />
            </Box>
          )}
          {headerCells.map((label, i) => (
            <Typography
              key={`${label}-${i}`}
              role="columnheader"
              variant="caption"
              color="text.secondary"
              sx={{
                fontWeight: 600,
                justifySelf:
                  label === "State" || label === "Actions" ? "center" : "start",
              }}
            >
              {label}
            </Typography>
          ))}
        </Box>

        {isLoading &&
          Array.from({ length: skeletonCount }).map((_, i) => (
            <Box
              key={i}
              role="presentation"
              sx={{
                gridColumn: "1 / -1",
                px: 2,
                py: 1.25,
                borderBottom: 1,
                borderColor: "divider",
                "&:last-of-type": { borderBottom: 0 },
              }}
            >
              <Skeleton variant="rounded" height={24} />
            </Box>
          ))}

        {!isLoading && orderedCards.length === 0 && (
          <Box role="presentation" sx={{ gridColumn: "1 / -1", px: 2, py: 4, textAlign: "center" }}>
            <Typography variant="body2" color="text.secondary">
              {emptyText}
            </Typography>
          </Box>
        )}

        {!isLoading &&
          orderedCards.map((c, index) => {
            const role = roleFor(c);
            const actions = cardActions(c.state, role);
            const isLast = index === orderedCards.length - 1;
            return (
              <Box
                key={c.id}
                role="row"
                data-testid={`timecard-row-${c.id}`}
                sx={{
                  gridColumn: "1 / -1",
                  display: "grid",
                  gridTemplateColumns: "subgrid",
                  columnGap: 2,
                  alignItems: "center",
                  px: 2,
                  py: 1.25,
                  borderBottom: isLast ? 0 : 1,
                  borderColor: "divider",
                }}
              >
                {selectable && (
                  <Box role="cell" sx={{ display: "flex", alignItems: "center" }}>
                    {actions.includes("approve") && (
                      <Checkbox
                        size="small"
                        checked={selectedIds?.has(c.id) ?? false}
                        onChange={() => onToggleSelect?.(c)}
                        inputProps={{ "aria-label": `Select time card ${c.caseNumber} for bulk approve` }}
                        sx={{ p: 0 }}
                      />
                    )}
                  </Box>
                )}
                {/* Quick preview, at the row's left edge (right after the
                    optional bulk-select checkbox) so it's reachable without
                    hunting across the row; the drawer opens on the right.
                    Clicking the eye for the row already open closes it
                    instead of re-opening the same preview. */}
                <Box role="cell" sx={{ display: "flex", alignItems: "center" }}>
                  <IconButton
                    size="small"
                    aria-label="View details"
                    aria-pressed={detailCard?.id === c.id}
                    data-testid={`timecard-view-${c.id}`}
                    data-quick-preview-eye="true"
                    onClick={() =>
                      setDetailCard((prev) => (prev?.id === c.id ? null : c))
                    }
                  >
                    <Eye size={16} />
                  </IconButton>
                </Box>
                {showCaseColumn && (
                  <Typography role="cell" variant="body2" noWrap title={c.caseNumber}>
                    {c.caseNumber}
                  </Typography>
                )}
                {showEngineerColumn && (
                  <Typography role="cell" variant="body2" noWrap title={c.userName}>
                    {c.userName}
                  </Typography>
                )}
                <Tooltip title={c.projectName}>
                  <Typography role="cell" variant="body2" noWrap>
                    {c.projectName}
                  </Typography>
                </Tooltip>
                <Typography role="cell" variant="body2" noWrap>
                  <RelativeDate value={c.workDate} />
                </Typography>
                <Box role="cell" sx={{ minWidth: 0 }}>
                  <Typography variant="body2" noWrap>
                    {c.totalMinutes} min
                  </Typography>
                  {!c.billable && (
                    <Typography variant="caption" color="text.secondary" noWrap display="block">
                      Non-billable
                    </Typography>
                  )}
                </Box>
                <Box role="cell" sx={{ minWidth: 0, justifySelf: "center" }}>
                  <TimeCardStatusChip state={c.state} />
                </Box>
                <Box
                  role="cell"
                  sx={{ display: "flex", alignItems: "center", gap: 0.75, justifySelf: "end" }}
                >
                  {/* Own actionable buttons regardless of showActionsColumn —
                      unlike approve/reject, editing and deleting are offered
                      to the card's own owner on every tab it can appear on
                      (My time sheets, All), not just the Approvals queue. */}
                  {actions.includes("edit") && (
                    <IconButton
                      size="small"
                      aria-label="Edit time card"
                      data-testid={`timecard-edit-${c.id}`}
                      onClick={() => onCardAction(c, "edit")}
                    >
                      <Pencil size={16} />
                    </IconButton>
                  )}
                  {actions.includes("delete") && (
                    <IconButton
                      size="small"
                      color="error"
                      aria-label="Delete time card"
                      data-testid={`timecard-delete-${c.id}`}
                      onClick={() => onCardAction(c, "delete")}
                    >
                      <Trash2 size={16} />
                    </IconButton>
                  )}
                  {showActionsColumn &&
                    actions
                      .filter(
                        (a): a is Exclude<TimecardAction, "edit" | "delete"> =>
                          a !== "edit" && a !== "delete",
                      )
                      .map((a) => {
                        const b = ACTION_BUTTONS[a];
                        return (
                          <Tooltip
                            key={a}
                            title={
                              selectionActive
                                ? "Clear the selection to act on this card directly, or use the bulk Approve button above"
                                : b.label
                            }
                          >
                            {/* A disabled IconButton doesn't fire the mouse
                                events Tooltip listens for -- the extra span
                                keeps the tooltip working while disabled. */}
                            <span>
                              <IconButton
                                size="small"
                                color={b.color}
                                disabled={selectionActive}
                                aria-label={b.label}
                                onClick={() => onCardAction(c, a)}
                              >
                                {b.icon}
                              </IconButton>
                            </span>
                          </Tooltip>
                        );
                      })}
                </Box>
              </Box>
            );
          })}
      </Box>
      <TimeCardCasePreviewDrawer
        card={detailCard}
        // Same gating as the row-level Approve/Reject buttons above: while a
        // bulk selection is active, approve/reject only happens through the
        // bulk toolbar. Without this filter the drawer would still offer its
        // own Approve/Reject regardless of `selectionActive`, letting a
        // direct decision bypass the same restriction the row buttons
        // enforce.
        actions={
          detailCard
            ? cardActions(detailCard.state, roleFor(detailCard)).filter(
                (a) => !selectionActive || (a !== "approve" && a !== "reject"),
              )
            : []
        }
        onClose={() => setDetailCard(null)}
        onDecide={(action) => {
          if (detailCard) onCardAction(detailCard, action);
          setDetailCard(null);
        }}
      />
    </Fragment>
  );
}
