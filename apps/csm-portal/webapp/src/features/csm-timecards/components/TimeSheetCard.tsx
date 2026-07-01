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

import type { JSX } from "react";
import {
  Box,
  Button,
  Card,
  Chip,
  Divider,
  LinearProgress,
  Typography,
} from "@wso2/oxygen-ui";
import { Check, Trash2, X } from "@wso2/oxygen-ui-icons-react";
import SemanticChip from "@components/SemanticChip";
import RelativeTime from "@components/RelativeTime";
import {
  breakdownSummary,
  TIME_SHEET_STATE_META,
} from "@features/csm-timecards/constants/timeCardConstants";
import TimeCardStatusChip from "@features/csm-timecards/components/TimeCardStatusChip";
import {
  cardActions,
  sheetActions,
  type SheetAction,
  type TimecardAction,
  type TimecardRoleCtx,
} from "@features/csm-timecards/utils/timeSheetState";
import { weekLabel } from "@features/csm-timecards/utils/timeSheetWeek";
import type {
  CsmTimeCard,
  CsmTimeSheet,
} from "@features/csm-timecards/types/timeCards";

interface TimeSheetCardProps {
  sheet: CsmTimeSheet;
  role: TimecardRoleCtx;
  /** Show the engineer's name (approvals view); omit on the user's own sheets. */
  showEngineer?: boolean;
  /** Approver actions disabled (e.g. approvals delegated away). */
  approverDisabled?: boolean;
  onSheetAction: (sheet: CsmTimeSheet, action: SheetAction) => void;
  onCardAction: (card: CsmTimeCard, action: TimecardAction) => void;
}

const CARD_BUTTONS: Record<
  TimecardAction,
  {
    label: string;
    color: "success" | "error" | "warning" | "inherit";
    variant: "contained" | "outlined" | "text";
    icon?: JSX.Element;
    approverAction: boolean;
  }
> = {
  edit: { label: "Edit", color: "inherit", variant: "text", approverAction: false },
  submit: { label: "Submit", color: "inherit", variant: "outlined", approverAction: false },
  resubmit: { label: "Resubmit", color: "inherit", variant: "outlined", approverAction: false },
  delete: {
    label: "Delete",
    color: "error",
    variant: "text",
    icon: <Trash2 size={14} />,
    approverAction: false,
  },
  approve: {
    label: "Approve",
    color: "success",
    variant: "contained",
    icon: <Check size={14} />,
    approverAction: true,
  },
  reject: {
    label: "Reject",
    color: "error",
    variant: "outlined",
    icon: <X size={14} />,
    approverAction: true,
  },
  recall: { label: "Recall", color: "warning", variant: "outlined", approverAction: true },
  process: { label: "Process", color: "inherit", variant: "outlined", approverAction: true },
};

const SHEET_BUTTONS: Record<
  SheetAction,
  {
    label: string;
    color: "primary" | "success" | "error" | "warning";
    variant: "contained" | "outlined";
    icon?: JSX.Element;
    approverAction: boolean;
  }
> = {
  approve: {
    label: "Approve remaining",
    color: "success",
    variant: "contained",
    icon: <Check size={14} />,
    approverAction: true,
  },
  reject: {
    label: "Reject remaining",
    color: "error",
    variant: "outlined",
    icon: <X size={14} />,
    approverAction: true,
  },
  recall: { label: "Recall week", color: "warning", variant: "outlined", approverAction: true },
  submit: { label: "Submit week", color: "primary", variant: "contained", approverAction: false },
};

/** A weekly time sheet: header (week, total, status), sheet-level actions, and
 * each card with its status and role/state-appropriate actions. Used in both
 * "My time sheets" (owner) and "Approvals" (approver/admin). */
export default function TimeSheetCard({
  sheet,
  role,
  showEngineer = false,
  approverDisabled = false,
  onSheetAction,
  onCardAction,
}: TimeSheetCardProps): JSX.Element {
  const meta = TIME_SHEET_STATE_META[sheet.state];
  const sActions = sheetActions(sheet, role);

  return (
    <Card variant="outlined" sx={{ p: 2 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
          gap: 1,
          mb: 1,
        }}
      >
        <Box>
          {showEngineer && (
            <Typography variant="subtitle2" sx={{ lineHeight: 1.2 }}>
              {sheet.userName}
            </Typography>
          )}
          <Typography variant="body2" color="text.secondary">
            {weekLabel(sheet.weekStart)}
          </Typography>
        </Box>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <SemanticChip role={meta.role} label={meta.label} />
          <Typography variant="h6" sx={{ lineHeight: 1 }}>
            {sheet.totalHours.toFixed(2)}h
          </Typography>
        </Box>
      </Box>

      <LinearProgress
        variant="determinate"
        value={Math.min(100, (sheet.totalHours / 40) * 100)}
        sx={{ height: 6, borderRadius: 3, mb: 1.5 }}
      />

      <Divider sx={{ mb: 1 }} />

      <Box sx={{ display: "flex", flexDirection: "column", gap: 0.75 }}>
        {sheet.cards.map((c) => {
          const actions = cardActions(c.state, role);
          return (
            <Box
              key={c.id}
              data-testid={`timecard-row-${c.caseNumber}`}
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                gap: 1,
                py: 0.5,
                flexWrap: "wrap",
              }}
            >
              <Box sx={{ minWidth: 0 }}>
                <Box sx={{ display: "flex", alignItems: "center", gap: 0.75, flexWrap: "wrap" }}>
                  <Typography variant="body2" noWrap>
                    {c.caseNumber} · {c.category} · {c.totalHours.toFixed(2)}h
                  </Typography>
                  <Chip
                    label={c.billable ? "Billable" : "Non-billable"}
                    size="small"
                    sx={{
                      height: 18,
                      fontSize: "0.65rem",
                      fontWeight: 600,
                      bgcolor: c.billable ? "success.light" : "action.hover",
                      color: c.billable ? "success.dark" : "text.secondary",
                    }}
                  />
                </Box>
                <Typography variant="caption" color="text.secondary">
                  {breakdownSummary(c.breakdown)} · <RelativeTime iso={c.submittedAt} />
                </Typography>
                {c.leadComment && (
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    sx={{ display: "block" }}
                  >
                    Lead: {c.leadComment}
                  </Typography>
                )}
              </Box>
              <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
                <TimeCardStatusChip state={c.state} />
                {actions.map((a) => {
                  const b = CARD_BUTTONS[a];
                  return (
                    <Button
                      key={a}
                      size="small"
                      color={b.color}
                      variant={b.variant}
                      startIcon={b.icon}
                      disabled={b.approverAction && approverDisabled}
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

      {sActions.length > 0 && (
        <>
          <Divider sx={{ my: 1 }} />
          <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1 }}>
            {sActions.map((a) => {
              const b = SHEET_BUTTONS[a];
              return (
                <Button
                  key={a}
                  size="small"
                  variant={b.variant}
                  color={b.color}
                  startIcon={b.icon}
                  disabled={b.approverAction && approverDisabled}
                  onClick={() => onSheetAction(sheet, a)}
                >
                  {b.label}
                </Button>
              );
            })}
          </Box>
        </>
      )}
    </Card>
  );
}
