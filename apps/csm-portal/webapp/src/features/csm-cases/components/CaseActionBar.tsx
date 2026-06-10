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
import { STATE_LABEL } from "@features/csm-dashboard/utils/abtDashboard";

type ActionConfirm = {
  title: string;
  body: string;
  confirmLabel: string;
  confirmColor: "primary" | "error" | "warning";
};

/**
 * Presentation for a transition *into* a given state. The button LABEL is never
 * stored here — it always comes from `STATE_LABEL[targetState]`, so the bar
 * honours the backend transition graph verbatim and never invents UI-specific
 * verbs. This only carries the icon/colour, the lifecycle action used for the
 * post-transition toast, and an optional confirm gate.
 */
type TargetConfig = {
  action: CaseLifecycleAction;
  color: "primary" | "success" | "warning" | "error";
  icon: JSX.Element;
  confirm?: ActionConfirm;
};

/** A concrete button: a target state plus its presentation. */
type PrimaryButton = TargetConfig & {
  targetState: CaseState;
  label: string;
  tooltip?: string;
};

const CLOSE_CONFIRM: ActionConfirm = {
  title: "Close this case?",
  // No "cannot be reopened" claim — reopen is a lead-only override, so that
  // assertion would be false for leads.
  body: "The customer receives a closure notification and the case moves to “Closed”.",
  confirmLabel: "Close case",
  confirmColor: "warning",
};

/**
 * Per-target-state presentation. One entry per state a case can move INTO.
 * Customer-notifying / hard-to-undo transitions carry a confirm gate so a stray
 * click in a busy queue can't silently close a case or email the customer.
 */
const TARGET_CONFIG: Record<CaseState, TargetConfig> = {
  open: { action: "reopen", color: "primary", icon: <RotateCcw size={16} /> },
  work_in_progress: {
    action: "resume_work",
    color: "primary",
    icon: <Play size={16} />,
  },
  waiting_on_wso2: {
    action: "wait_on_wso2",
    color: "primary",
    icon: <Clock size={16} />,
  },
  awaiting_info: {
    action: "request_info",
    color: "primary",
    icon: <Inbox size={16} />,
  },
  solution_proposed: {
    action: "propose_solution",
    color: "success",
    icon: <Send size={16} />,
    confirm: {
      title: "Propose solution to the customer?",
      body: "The customer is notified that a solution has been proposed and the case moves to “Solution proposed”.",
      confirmLabel: "Propose solution",
      confirmColor: "primary",
    },
  },
  reopen: { action: "reopen", color: "primary", icon: <RotateCcw size={16} /> },
  closed: {
    action: "close",
    color: "warning",
    icon: <CheckCircle size={16} />,
    confirm: CLOSE_CONFIRM,
  },
};

/** Build the button for a transition into `target`, labelled by the BE state. */
function buttonFor(target: CaseState): PrimaryButton {
  return { targetState: target, label: STATE_LABEL[target], ...TARGET_CONFIG[target] };
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
 * caller grants the `canReopenClosed` capability. Labelled by the BE state it
 * lands in (`reopen` → "Reopened") like every other button.
 */
const REOPEN_BUTTON: PrimaryButton = {
  targetState: "reopen",
  label: STATE_LABEL.reopen,
  action: "reopen",
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
 * reachable state, labelled with that state's name, so the bar always reflects
 * exactly the transitions the backend permits. Secondary actions live in an
 * overflow menu and are state-independent.
 */
export default function CaseActionBar({
  caseDetail,
  onAction,
  canReopenClosed = false,
}: CaseActionBarProps): JSX.Element {
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const [pendingConfirm, setPendingConfirm] = useState<PrimaryButton | null>(
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
    .map(buttonFor);
  const primary = [
    ...lifecycle,
    // Lead-only reopen is an override outside the normal `nextStates` flow, so
    // it is gated by capability rather than by the backend transition list.
    ...(from === "closed" && canReopenClosed ? [REOPEN_BUTTON] : []),
  ];
  const secondary = buildSecondaryItems();

  const runPrimary = (p: PrimaryButton): void => {
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
        // `targetState` is unique per button (each moves to a distinct state),
        // so it is the correct React key — the lifecycle action is not (e.g.
        // `resume_work` can map from more than one source state).
        return p.tooltip ? (
          <Tooltip key={p.targetState} title={p.tooltip}>
            {button}
          </Tooltip>
        ) : (
          <Box
            key={p.targetState}
            component="span"
            sx={{ display: "inline-flex" }}
          >
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
