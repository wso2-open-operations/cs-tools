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

type StateAction = {
  action: CaseLifecycleAction;
  /**
   * The state this transition moves the case into. It comes straight from the
   * backend's `nextStates`, and the same value is what the PATCH writes — so the
   * button the engineer sees and the transition that is persisted can never
   * drift apart. (The earlier bug was a hand-maintained action→state map that
   * disagreed with the backend graph and silently dropped valid buttons.)
   */
  targetState: CaseState;
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

const CLOSE_CONFIRM: ActionConfirm = {
  title: "Close this case?",
  body: "The customer receives a closure notification and the case moves to “Closed”. Once closed, the case cannot be reopened.",
  confirmLabel: "Close case",
  confirmColor: "warning",
};

/**
 * Default button for a transition *into* `target`, used whenever the
 * (source → target) edge needs no source-specific wording. The label describes
 * the destination; `targetState` is what gets PATCHed.
 */
function defaultActionFor(target: CaseState): StateAction | null {
  switch (target) {
    case "work_in_progress":
      // Reached from a paused/parked sub-state — the engineer is un-pausing.
      return {
        action: "resume_work",
        targetState: target,
        label: "Resume work",
        color: "primary",
        icon: <Play size={16} />,
      };
    case "waiting_on_wso2":
      return {
        action: "wait_on_wso2",
        targetState: target,
        label: "Wait on WSO2",
        color: "primary",
        icon: <Clock size={16} />,
      };
    case "awaiting_info":
      return {
        action: "request_info",
        targetState: target,
        label: "Request info",
        color: "primary",
        icon: <Inbox size={16} />,
      };
    case "solution_proposed":
      return {
        action: "propose_solution",
        targetState: target,
        label: "Propose solution",
        color: "success",
        icon: <Send size={16} />,
        confirm: {
          title: "Propose solution to the customer?",
          body: "The customer is notified that a solution has been proposed and the case moves to “Solution proposed”.",
          confirmLabel: "Propose solution",
          confirmColor: "primary",
        },
      };
    case "closed":
      return {
        action: "close",
        targetState: target,
        label: "Close",
        color: "success",
        icon: <CheckCircle size={16} />,
        confirm: CLOSE_CONFIRM,
      };
    case "reopen":
      return {
        action: "reopen",
        targetState: target,
        label: "Reopen",
        color: "primary",
        icon: <RotateCcw size={16} />,
      };
    // `open` is an initial state, not a transition target the bar offers.
    case "open":
    default:
      return null;
  }
}

/**
 * Source-specific overrides where the label depends on *where the case is
 * coming from*, not just where it is going. The clearest case is the move into
 * Waiting on WSO2: from Work in progress it is a deliberate "Wait on WSO2"
 * pause, but from a paused/parked state (Awaiting info, Solution proposed,
 * Reopened) the same target means the engineer is *resuming* the case. Anything
 * not listed here falls back to `defaultActionFor`.
 */
const EDGE_OVERRIDES: Partial<
  Record<CaseState, Partial<Record<CaseState, StateAction>>>
> = {
  open: {
    work_in_progress: {
      action: "start_work",
      targetState: "work_in_progress",
      label: "Start work",
      color: "primary",
      icon: <Play size={16} />,
      tooltip: "Assigns this case to you and moves it to Work in progress.",
    },
  },
  awaiting_info: {
    waiting_on_wso2: {
      action: "resume_work",
      targetState: "waiting_on_wso2",
      label: "Resume work",
      color: "primary",
      icon: <Play size={16} />,
    },
  },
  solution_proposed: {
    waiting_on_wso2: {
      action: "resume_work",
      targetState: "waiting_on_wso2",
      label: "Resume work",
      color: "primary",
      icon: <RotateCcw size={16} />,
    },
  },
  reopen: {
    waiting_on_wso2: {
      action: "resume_work",
      targetState: "waiting_on_wso2",
      label: "Resume work",
      color: "primary",
      icon: <Play size={16} />,
    },
  },
};

function actionFor(from: CaseState, to: CaseState): StateAction | null {
  return EDGE_OVERRIDES[from]?.[to] ?? defaultActionFor(to);
}

/**
 * Fallback target lists, mirroring the backend `nextStates` graph in
 * `state.go`. Used only when the case carries no `nextStates` (e.g. mock data,
 * or a backend that hasn't populated the field) so the bar still shows the
 * expected actions. When the case *does* carry `nextStates`, that drives the
 * buttons directly and this map is ignored.
 */
const FALLBACK_TARGETS: Record<CaseState, CaseState[]> = {
  open: ["work_in_progress"],
  work_in_progress: [
    "solution_proposed",
    "awaiting_info",
    "waiting_on_wso2",
    "closed",
  ],
  solution_proposed: ["waiting_on_wso2", "closed"],
  awaiting_info: ["waiting_on_wso2"],
  waiting_on_wso2: ["work_in_progress"],
  reopen: ["waiting_on_wso2"],
  closed: [],
};

/**
 * Display order for the lifecycle buttons. The forward/safe action sits first
 * (it gets the contained emphasis); Close is always last so a destructive
 * action is never the emphasised default. States not listed sort just before
 * Close.
 */
const DISPLAY_ORDER: CaseState[] = [
  "solution_proposed",
  "work_in_progress",
  "awaiting_info",
  "waiting_on_wso2",
  "reopen",
  "open",
  "closed",
];
const CLOSED_RANK = DISPLAY_ORDER.indexOf("closed");
function orderRank(s: CaseState): number {
  const i = DISPLAY_ORDER.indexOf(s);
  return i === -1 ? CLOSED_RANK - 0.5 : i;
}

/**
 * Lead-only override to reopen a closed case. Not part of the normal
 * `nextStates` flow — the backend reports a closed case as terminal
 * (`nextStates: []`) — so it is appended for the `closed` state only when the
 * caller grants the `canReopenClosed` capability.
 */
const REOPEN_ACTION: StateAction = {
  action: "reopen",
  targetState: "reopen",
  label: "Reopen",
  color: "primary",
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
    targetState?: CaseState,
  ) => void | Promise<unknown>;
  /**
   * Grants the lead-only "Reopen" action on a closed case. Defaults to false,
   * so a closed case is terminal unless the caller is a lead.
   */
  canReopenClosed?: boolean;
}

/**
 * Lifecycle + overflow action bar for the case detail page. Primary buttons are
 * driven directly by the case's backend-supplied `nextStates`: one button per
 * reachable state, so the bar always reflects exactly the transitions the
 * backend permits. Secondary actions live in an overflow menu and are
 * state-independent.
 */
export default function CaseActionBar({
  caseDetail,
  onAction,
  canReopenClosed = false,
}: CaseActionBarProps): JSX.Element {
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const [pendingConfirm, setPendingConfirm] = useState<StateAction | null>(
    null,
  );

  const from = caseDetail.state;
  // Render a button for every state the backend says the case can move to. When
  // the field is absent (undefined — mock data / not yet populated) fall back to
  // the known graph so the bar isn't empty. An explicit empty list (a terminal
  // case, e.g. Closed) correctly yields no lifecycle buttons.
  const targets = caseDetail.nextStates ?? FALLBACK_TARGETS[from] ?? [];
  const lifecycle = [...new Set(targets)]
    .sort((a, b) => orderRank(a) - orderRank(b))
    .map((to) => actionFor(from, to))
    .filter((a): a is StateAction => a !== null);
  const primary = [
    ...lifecycle,
    // Lead-only reopen is an override outside the normal `nextStates` flow, so
    // it is gated by capability rather than by the backend transition list.
    ...(from === "closed" && canReopenClosed ? [REOPEN_ACTION] : []),
  ];
  const secondary = buildSecondaryItems();

  const runPrimary = (p: StateAction): void => {
    if (p.confirm) {
      setPendingConfirm(p);
      return;
    }
    void onAction(p.action, p.targetState);
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
              if (p) void onAction(p.action, p.targetState);
            }}
          >
            {pendingConfirm?.confirm?.confirmLabel ?? "Confirm"}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
