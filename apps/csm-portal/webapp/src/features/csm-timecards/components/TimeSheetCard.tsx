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
import { Box, Button, Card, Chip, Divider, Typography } from "@wso2/oxygen-ui";
import { Check, X } from "@wso2/oxygen-ui-icons-react";
import SemanticChip from "@components/SemanticChip";
import RelativeTime from "@components/RelativeTime";
import { TIME_SHEET_STATE_META } from "@features/csm-timecards/constants/timeCardConstants";
import TimeCardStatusChip from "@features/csm-timecards/components/TimeCardStatusChip";
import {
  cardActions,
  type TimecardAction,
  type TimecardRoleCtx,
} from "@features/csm-timecards/utils/timeSheetState";
import { weekLabel } from "@features/csm-timecards/utils/timeSheetWeek";
import type { CsmTimeCard, CsmTimeSheet } from "@features/csm-timecards/types/timeCards";

interface TimeSheetCardProps {
  sheet: CsmTimeSheet;
  role: TimecardRoleCtx;
  /** Show the engineer's name (approvals view); omit on the user's own sheets. */
  showEngineer?: boolean;
  onCardAction: (card: CsmTimeCard, action: TimecardAction) => void;
}

const CARD_BUTTONS: Record<
  TimecardAction,
  {
    label: string;
    color: "success" | "error";
    variant: "contained" | "outlined";
    icon: JSX.Element;
  }
> = {
  approve: {
    label: "Approve",
    color: "success",
    variant: "contained",
    icon: <Check size={14} />,
  },
  reject: {
    label: "Reject",
    color: "error",
    variant: "outlined",
    icon: <X size={14} />,
  },
};

/**
 * A weekly time sheet: header (week, total, status) and each card with its
 * status and role-appropriate actions. Used in both "My time sheets" (owner,
 * no actions) and "Approvals" (approver/admin: approve/reject a submitted
 * card). There is no sheet-level bulk action — the backend has no such
 * endpoint, only per-card `PATCH /time-cards/{id}`.
 */
export default function TimeSheetCard({
  sheet,
  role,
  showEngineer = false,
  onCardAction,
}: TimeSheetCardProps): JSX.Element {
  const meta = TIME_SHEET_STATE_META[sheet.state];

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

      <Divider sx={{ mb: 1 }} />

      <Box sx={{ display: "flex", flexDirection: "column", gap: 0.75 }}>
        {sheet.cards.map((c) => {
          const actions = cardActions(c.state, role);
          return (
            <Box
              key={c.id}
              data-testid={`timecard-row-${c.id}`}
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
                    {c.caseNumber} · {c.totalHours.toFixed(2)}h
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
                  <RelativeTime iso={c.createdOn} />
                  {c.approvedByName && ` · Decided by ${c.approvedByName}`}
                </Typography>
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
    </Card>
  );
}
