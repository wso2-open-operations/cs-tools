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
import { Box, Button, IconButton, Skeleton, Tooltip, Typography } from "@wso2/oxygen-ui";
import { Check, Eye, X } from "@wso2/oxygen-ui-icons-react";
import RelativeTime from "@components/RelativeTime";
import TimeCardDetailModal from "@features/csm-timecards/components/TimeCardDetailModal";
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
   * engineer) sort adjacent to each other; both Case and Engineer stay
   * visible as their own columns regardless (see `showCaseEngineerColumns`),
   * this only controls ordering. */
  groupBy: TimeCardGroupBy;
  /** Show the Case and Engineer columns. Off on "My time sheets", where every
   * card already belongs to the signed-in user — an Engineer column would be
   * redundant, and there's no cross-case grouping to distinguish either. */
  showCaseEngineerColumns?: boolean;
  /** Show the Approve/Reject buttons alongside the view-details eye icon in
   * the Actions column. Off outside the Approvals tab, where `roleFor`
   * already makes `cardActions` return none per row anyway — this just
   * avoids reserving the extra button space for it. The Actions column
   * itself (eye icon) is always present. */
  showActionsColumn?: boolean;
  /** Per-card role context — varies per row on "All" (isOwner depends on who
   * submitted that specific card), constant on "My time sheets"/"Approvals". */
  roleFor: (card: CsmTimeCard) => TimecardRoleCtx;
  onCardAction: (card: CsmTimeCard, action: TimecardAction) => void;
}

const ACTION_BUTTONS: Record<
  TimecardAction,
  { label: string; color: "success" | "error"; variant: "contained" | "outlined"; icon: JSX.Element }
> = {
  approve: { label: "Approve", color: "success", variant: "outlined", icon: <Check size={14} /> },
  reject: { label: "Reject", color: "error", variant: "outlined", icon: <X size={14} /> },
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
  showCaseEngineerColumns = false,
  showActionsColumn = false,
  roleFor,
  onCardAction,
}: TimeCardsTableProps): JSX.Element {
  // The card currently open in the read-only detail modal — local to this
  // table (no mutation involved, unlike `review` on the page, which needs to
  // carry mutation state alongside the card).
  const [detailCard, setDetailCard] = useState<CsmTimeCard | null>(null);

  const headerCells = [
    ...(showCaseEngineerColumns ? ["Case", "Engineer"] : []),
    "Project",
    "Date",
    "Minutes",
    "State",
    "Actions",
  ];
  const grid = [
    ...(showCaseEngineerColumns ? ["minmax(120px, 0.9fr)", "minmax(140px, 1fr)"] : []),
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
          {headerCells.map((label, i) => (
            <Typography
              key={`${label}-${i}`}
              role="columnheader"
              variant="caption"
              color="text.secondary"
              sx={{
                fontWeight: 600,
                justifySelf:
                  label === "State" ? "center" : i === headerCells.length - 1 ? "end" : "start",
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
                {showCaseEngineerColumns && (
                  <>
                    <Typography role="cell" variant="body2" noWrap title={c.caseNumber}>
                      {c.caseNumber}
                    </Typography>
                    <Typography role="cell" variant="body2" noWrap title={c.userName}>
                      {c.userName}
                    </Typography>
                  </>
                )}
                <Tooltip title={c.projectName}>
                  <Typography role="cell" variant="body2" noWrap>
                    {c.projectName}
                  </Typography>
                </Tooltip>
                <Typography role="cell" variant="body2" noWrap>
                  <RelativeTime iso={c.workDate} />
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
                  <IconButton
                    size="small"
                    aria-label="View details"
                    data-testid={`timecard-view-${c.id}`}
                    onClick={() => setDetailCard(c)}
                  >
                    <Eye size={16} />
                  </IconButton>
                  {showActionsColumn &&
                    actions.map((a) => {
                      const b = ACTION_BUTTONS[a];
                      return (
                        <Button
                          key={a}
                          size="small"
                          color={b.color}
                          variant={b.variant}
                          startIcon={b.icon}
                          onClick={() => onCardAction(c, a)}
                        >
                          {b.label}
                        </Button>
                      );
                    })}
                </Box>
              </Box>
            );
          })}
      </Box>
      {detailCard && <TimeCardDetailModal card={detailCard} onClose={() => setDetailCard(null)} />}
    </Fragment>
  );
}
