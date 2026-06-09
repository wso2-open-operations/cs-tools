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

import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Menu,
  MenuItem,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import {
  AlertTriangle,
  CheckCircle,
  ChevronDown,
  Clock,
  Copy,
  GitBranch,
  Inbox,
  Link as LinkIcon,
  ListChecks,
  PauseCircle,
  Phone,
  Play,
  RotateCcw,
  Send,
  ShieldAlert,
  TriangleAlert,
  User,
  Users,
} from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import type {
  CaseLifecycleAction,
  CsmCaseDetail,
} from "@features/csm-cases/types/csmCases";
import type { CaseState } from "@features/csm-dashboard/types/abtDashboard";

type ActionConfirm = {
  title: string;
  body: string;
  confirmLabel: string;
  confirmColor: "primary" | "error" | "warning";
};

type PrimaryAction = {
  action: CaseLifecycleAction;
  label: string;
  color: "primary" | "success" | "warning" | "error";
  icon: JSX.Element;
  /** Hover hint clarifying the consequence of the transition. */
  tooltip?: string;
  /**
   * When set, the action is gated behind a confirmation dialog. Used for
   * transitions that notify the customer or are otherwise hard to undo, so a
   * stray click in a busy queue can't silently close a case or email the
   * customer.
   */
  confirm?: ActionConfirm;
};

const PRIMARY_BY_STATE: Record<CaseState, PrimaryAction[]> = {
  // Self-assignment moves the case to WIP (ISSU-002), so "Start work" and a
  // separate "Assign to me" were the same transition — collapsed to one.
  open: [
    {
      action: "start_work",
      label: "Start work",
      color: "primary",
      icon: <Play size={16} />,
      tooltip: "Assigns this case to you and moves it to Work in progress.",
    },
  ],
  work_in_progress: [
    {
      action: "propose_solution",
      label: "Propose solution",
      color: "success",
      icon: <Send size={16} />,
      confirm: {
        title: "Propose solution to the customer?",
        body: "The customer is notified that a solution has been proposed and the case moves to “Solution proposed”.",
        confirmLabel: "Propose solution",
        confirmColor: "primary",
      },
    },
    {
      action: "request_info",
      label: "Request info",
      color: "warning",
      icon: <Inbox size={16} />,
    },
    {
      action: "wait_on_wso2",
      label: "Wait on WSO2",
      color: "warning",
      icon: <Clock size={16} />,
    },
  ],
  // Resume work first (the safe, recoverable default); "Mark resolved" is the
  // irreversible, customer-notifying action so it is demoted to outlined AND
  // gated behind a confirmation.
  solution_proposed: [
    {
      action: "resume_work",
      label: "Resume work",
      color: "primary",
      icon: <RotateCcw size={16} />,
    },
    {
      action: "close",
      label: "Close",
      color: "success",
      icon: <CheckCircle size={16} />,
      confirm: {
        title: "Close this case?",
        body: "The customer receives a closure notification and the case moves to “Closed”. Once closed, the case cannot be reopened.",
        confirmLabel: "Close case",
        confirmColor: "warning",
      },
    },
  ],
  awaiting_info: [
    {
      action: "resume_work",
      label: "Resume work",
      color: "primary",
      icon: <Play size={16} />,
    },
    {
      action: "close_no_response",
      label: "Close (no response)",
      color: "warning",
      icon: <CheckCircle size={16} />,
      confirm: {
        title: "Close without a customer response?",
        body: "The case is closed because the customer did not respond. They still receive a closure notification. Once closed, the case cannot be reopened.",
        confirmLabel: "Close case",
        confirmColor: "warning",
      },
    },
  ],
  waiting_on_wso2: [
    {
      action: "resume_work",
      label: "Resume work",
      color: "primary",
      icon: <Play size={16} />,
    },
  ],
  reopen: [
    {
      action: "start_work",
      label: "Start work",
      color: "primary",
      icon: <Play size={16} />,
    },
  ],
  // Closed is terminal for engineers and customers. Reopening is a lead-only
  // override (see REOPEN_ACTION + the `canReopenClosed` prop) and is appended
  // separately, so the default closed case offers no reopen path.
  closed: [],
};

/**
 * Lead-only override to reopen a closed case. Not part of PRIMARY_BY_STATE —
 * it is appended for the `closed` state only when the caller grants the
 * `canReopenClosed` capability, so normal engineers never see it.
 */
const REOPEN_ACTION: PrimaryAction = {
  action: "reopen",
  label: "Reopen",
  color: "warning",
  icon: <RotateCcw size={16} />,
  tooltip: "Lead-only: reopen a closed case for an exceptional follow-up.",
  confirm: {
    title: "Reopen this closed case?",
    body: "The case returns to an active state and the customer is notified.",
    confirmLabel: "Reopen case",
    confirmColor: "warning",
  },
};

interface SecondaryItem {
  key: string;
  label: string;
  icon: JSX.Element;
  /** If true, render with a divider below in the menu. */
  divider?: boolean;
}

/**
 * The "More" overflow lists state-independent actions on a case. Items here
 * map to documented use cases — see `UseCases.md`:
 *   - Reassign / group               → ISSU-002 (self-assign generalised)
 *   - Escalate / Severity change     → ISSU-006, ISSU-007
 *   - Hold auto-closure              → ISSU-027
 *   - Create incident / link incident → ISSU-021
 *   - Raise Git issue                → ISSU-020
 *   - Create task                    → ISSU-025
 *   - Request a call                 → ISSU-008
 *   - Log time                       → ISSU-017
 *   - Copy case link                 → ISSU-010 (per-comment + per-case permalinks)
 *
 * Intentionally NOT here:
 *   - Watch / unwatch  → managed via the Watchers widget in Details (ISSU-018)
 *   - Open in ServiceNow → this platform replaces ServiceNow; no back-link
 */
function buildSecondaryItems(): SecondaryItem[] {
  return [
    { key: "reassign_engineer", label: "Reassign engineer…", icon: <User size={16} /> },
    { key: "reassign_group", label: "Reassign to group…", icon: <Users size={16} />, divider: true },
    { key: "escalate", label: "Escalate to lead…", icon: <TriangleAlert size={16} /> },
    { key: "change_severity", label: "Request severity change…", icon: <ShieldAlert size={16} /> },
    { key: "hold_auto_close", label: "Hold auto-closure…", icon: <PauseCircle size={16} />, divider: true },
    { key: "create_incident", label: "Create incident from case…", icon: <AlertTriangle size={16} /> },
    { key: "link_incident", label: "Link to incident…", icon: <LinkIcon size={16} /> },
    { key: "raise_git_issue", label: "Raise internal Git issue…", icon: <GitBranch size={16} /> },
    { key: "create_task", label: "Create task…", icon: <ListChecks size={16} />, divider: true },
    { key: "request_call", label: "Request a call…", icon: <Phone size={16} /> },
    { key: "log_time", label: "Log time…", icon: <Clock size={16} />, divider: true },
    { key: "copy_link", label: "Copy case link", icon: <Copy size={16} /> },
  ];
}

interface CaseActionBarProps {
  caseDetail: CsmCaseDetail;
  onAction: (
    action: CaseLifecycleAction | { secondary: string },
  ) => void | Promise<unknown>;
  /**
   * Grants the lead-only "Reopen" action on a closed case. Defaults to false,
   * so a closed case is terminal unless the caller is a lead.
   */
  canReopenClosed?: boolean;
}

/**
 * Lifecycle + overflow action bar for the case detail page. Primary buttons
 * are state-driven (only valid transitions show). Secondary actions live in
 * an overflow menu and are state-independent.
 */
export default function CaseActionBar({
  caseDetail,
  onAction,
  canReopenClosed = false,
}: CaseActionBarProps): JSX.Element {
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const [pendingConfirm, setPendingConfirm] = useState<PrimaryAction | null>(
    null,
  );
  const primary = [
    ...(PRIMARY_BY_STATE[caseDetail.state] ?? []),
    ...(caseDetail.state === "closed" && canReopenClosed
      ? [REOPEN_ACTION]
      : []),
  ];
  const secondary = buildSecondaryItems();

  const runPrimary = (p: PrimaryAction): void => {
    if (p.confirm) {
      setPendingConfirm(p);
      return;
    }
    void onAction(p.action);
  };

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        gap: 1,
        flexWrap: "wrap",
        justifyContent: { xs: "flex-start", md: "flex-end" },
      }}
    >
      {primary.map((p, idx) => {
        const button = (
          <Button
            size="small"
            variant={idx === 0 ? "contained" : "outlined"}
            color={p.color}
            startIcon={p.icon}
            onClick={() => runPrimary(p)}
          >
            {p.label}
          </Button>
        );
        return p.tooltip ? (
          <Tooltip key={p.action} title={p.tooltip}>
            {button}
          </Tooltip>
        ) : (
          <Box key={p.action} component="span" sx={{ display: "inline-flex" }}>
            {button}
          </Box>
        );
      })}

      <Button
        size="small"
        variant="outlined"
        color="primary"
        endIcon={<ChevronDown size={16} />}
        onClick={(e) => setMenuAnchor(e.currentTarget)}
      >
        More
      </Button>
      <Menu
        anchorEl={menuAnchor}
        open={!!menuAnchor}
        onClose={() => setMenuAnchor(null)}
      >
        {secondary.map((item) => [
          <MenuItem
            key={item.key}
            onClick={() => {
              setMenuAnchor(null);
              void onAction({ secondary: item.key });
            }}
            sx={{ gap: 1.25, minHeight: 36 }}
          >
            {item.icon}
            {item.label}
          </MenuItem>,
          item.divider ? (
            <Box
              key={`${item.key}-divider`}
              sx={{ borderTop: 1, borderColor: "divider", my: 0.25 }}
            />
          ) : null,
        ])}
      </Menu>

      <Dialog
        open={!!pendingConfirm}
        onClose={() => setPendingConfirm(null)}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>{pendingConfirm?.confirm?.title}</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            {pendingConfirm?.confirm?.body}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPendingConfirm(null)}>Cancel</Button>
          <Button
            variant="contained"
            color={pendingConfirm?.confirm?.confirmColor ?? "primary"}
            startIcon={pendingConfirm?.icon}
            onClick={() => {
              const p = pendingConfirm;
              setPendingConfirm(null);
              if (p) void onAction(p.action);
            }}
          >
            {pendingConfirm?.confirm?.confirmLabel ?? "Confirm"}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
