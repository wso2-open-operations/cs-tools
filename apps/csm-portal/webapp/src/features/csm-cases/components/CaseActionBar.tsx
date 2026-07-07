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
  ArrowRight,
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
  Send,
  User,
} from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import type {
  CaseLifecycleAction,
  CsmCaseDetail,
} from "@features/csm-cases/types/csmCases";
import type { CaseState } from "@features/csm-dashboard/types/abtDashboard";
import { stateLabel } from "@features/csm-dashboard/utils/abtDashboard";

type ActionConfirm = {
  title: string;
  body: string;
  confirmLabel: string;
  confirmColor: "primary" | "error" | "warning";
};

/**
 * Presentation for a transition *into* a given state. The button LABEL is never
 * stored here — it always comes from `stateLabel(targetState)`, so the bar
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
  body: "The customer receives a closure notification and the case moves to “Closed”.",
  confirmLabel: "Close case",
  confirmColor: "warning",
};

/**
 * Per-target-state presentation. One entry per state a case can move INTO.
 * Customer-notifying / hard-to-undo transitions carry a confirm gate so a stray
 * click in a busy queue can't silently close a case or email the customer.
 */
const TARGET_CONFIG: Partial<Record<CaseState, TargetConfig>> = {
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
  closed: {
    action: "close",
    color: "warning",
    icon: <CheckCircle size={16} />,
    confirm: CLOSE_CONFIRM,
  },
};

/**
 * Presentation for a transition into a state the bar has no curated config for
 * (e.g. a state added on the backend). Keeps the button renderable and safe to
 * click — neutral styling, a generic `transition` action for the toast — so a
 * new backend state needs no frontend change to appear and work. The PATCH
 * target still comes from the state itself, not from this action.
 */
const DEFAULT_TARGET_CONFIG: TargetConfig = {
  action: "transition",
  color: "primary",
  icon: <ArrowRight size={16} />,
};

// Friendlier verbs for the transition buttons than the raw state name reads
// as. `work_in_progress` is special-cased in `buttonFor` since its wording
// (and underlying action) depends on who the case is assigned to.
const TRANSITION_LABEL: Partial<Record<CaseState, string>> = {
  solution_proposed: "Propose solution",
  awaiting_info: "Request information",
  waiting_on_wso2: "Wait on WSO2",
  closed: "Close",
};

/** Build the button for a transition into `target`, labelled by the BE state. */
function buttonFor(target: CaseState, caseDetail: CsmCaseDetail): PrimaryButton {
  const config = TARGET_CONFIG[target] ?? DEFAULT_TARGET_CONFIG;
  if (target === "work_in_progress") {
    // Moving into Work in progress reads differently depending on whether the
    // case is already the current engineer's: unassigned/someone-else's case
    // needs claiming first ("Assign to me"), while an already-own case just
    // needs its work started ("Start progress"). `onAction` uses the `action`
    // value (not just the target state) to decide whether to PATCH the
    // assignee before moving the state.
    return {
      targetState: target,
      ...config,
      label: caseDetail.assigneeIsMe ? "Start progress" : "Assign to me",
      action: caseDetail.assigneeIsMe ? "start_work" : "assign_to_me",
    };
  }
  return {
    targetState: target,
    label: TRANSITION_LABEL[target] ?? stateLabel(target),
    ...config,
  };
}

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
  "open",
  "closed",
];
const CLOSED_RANK = DISPLAY_ORDER.indexOf("closed");
function orderRank(s: CaseState): number {
  const i = DISPLAY_ORDER.indexOf(s);
  return i === -1 ? CLOSED_RANK - 0.5 : i;
}

interface SecondaryItem {
  key: string;
  label: string;
  icon: JSX.Element;
  /** If true, render with a divider below in the menu. */
  divider?: boolean;
  /** If true, the item is greyed out and not clickable. */
  disabled?: boolean;
  /** Optional hover hint — used to explain why a `disabled` item is greyed out. */
  tooltip?: string;
}

/**
 * The "More" overflow lists state-independent actions on a case. Items here
 * map to documented use cases — see `UseCases.md`:
 *   - Reassign engineer              → ISSU-002 (self-assign generalised)
 *   - Hold auto-closure              → ISSU-027 (only while awaiting info / solution proposed)
 *   - Create incident / link incident → ISSU-021
 *   - Raise Git issue                → ISSU-020
 *   - Create task                    → ISSU-025
 *   - Request a call                 → ISSU-008 (opens the Call requests tab's create dialog)
 *   - Log time                       → ISSU-017
 *   - Copy case link                 → ISSU-010 (per-comment + per-case permalinks)
 *
 * Intentionally NOT here:
 *   - Watch / unwatch  → withdrawn along with the Watchers widget (ISSU-018), no backend flow planned yet
 *   - Open in ServiceNow → this platform replaces ServiceNow; no back-link
 *   - Escalate to lead / Request severity change → withdrawn, no backend flow planned yet
 */
function buildSecondaryItems(caseDetail: CsmCaseDetail): SecondaryItem[] {
  const items: SecondaryItem[] = [];

  // Pause / resume the work sub-state. Offered only for an in-progress case
  // assigned to the current user — pausing/resuming is the assignee's own
  // workflow control, not something to do to someone else's active case.
  if (caseDetail.assigneeIsMe && caseDetail.state === "work_in_progress") {
    // Only `ongoing` is pausable; anything else (paused OR a null work-state
    // in-progress case) is resumable — otherwise the only action that can set
    // `ongoing` would be hidden for null work-state cases.
    const ongoing = caseDetail.workState === "ongoing";
    items.push({
      key: "toggle_work_state",
      label: ongoing ? "Pause work" : "Resume work",
      icon: ongoing ? <PauseCircle size={16} /> : <Play size={16} />,
      divider: true,
    });
  }

  // Assignee changes are rejected while a case is Work in progress + Ongoing:
  // the backend silently reverts the change and still reports success, so the
  // reassign would appear to work while the assignee stays put. Gate the action
  // here so we never fire that no-op. The work must be paused first (by the
  // current assignee) or the reassignment handled by a lead.
  const reassignBlocked =
    caseDetail.state === "work_in_progress" && caseDetail.workState === "ongoing";

  // Hold auto-closure only makes sense while the case is sitting in a state
  // that's subject to auto-closure (awaiting the customer's response) —
  // showing it at any other time would offer to hold a closure that isn't
  // pending.
  const canHoldAutoClose =
    caseDetail.state === "awaiting_info" || caseDetail.state === "solution_proposed";

  // Only "Copy case link", "Request a call", and "Log time" are wired up.
  // The rest are disabled until their backend flows land, so the menu
  // advertises the roadmap without exposing dead actions that would no-op or
  // toast a mock message.
  items.push(
    { key: "raise_git_issue", label: "Raise internal Git issue…", icon: <GitBranch size={16} />, divider: true },
    {
      key: "reassign_engineer",
      label: "Assign / reassign engineer…",
      icon: <User size={16} />,
      divider: true,
      disabled: reassignBlocked,
      tooltip: reassignBlocked
        ? "Can't reassign while the case is in progress and ongoing. Pause the work first, or ask the current assignee or a lead to reassign."
        : undefined,
    },
    ...(canHoldAutoClose
      ? [{ key: "hold_auto_close", label: "Hold auto-closure…", icon: <PauseCircle size={16} />, divider: true }]
      : []),
    { key: "create_incident", label: "Create incident from case…", icon: <AlertTriangle size={16} />, disabled: true },
    { key: "link_incident", label: "Link to incident…", icon: <LinkIcon size={16} />, divider: true, disabled: true },
    { key: "create_task", label: "Create task…", icon: <ListChecks size={16} />, divider: true, disabled: true },
    { key: "request_call", label: "Request a call…", icon: <Phone size={16} /> },
    { key: "log_time", label: "Log time…", icon: <Clock size={16} />, divider: true },
    { key: "copy_link", label: "Copy case link", icon: <Copy size={16} /> },
  );

  return items;
}

interface CaseActionBarProps {
  caseDetail: CsmCaseDetail;
  onAction: (
    action: CaseLifecycleAction | { secondary: string },
    targetState?: CaseState,
  ) => void | Promise<unknown>;
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
}: CaseActionBarProps): JSX.Element {
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const [stateMenuAnchor, setStateMenuAnchor] = useState<HTMLElement | null>(null);
  const [pendingConfirm, setPendingConfirm] = useState<PrimaryButton | null>(
    null,
  );

  // Render a button for every state the backend says the case can move to. The
  // backend `nextStates` is the single source of truth: an empty/terminal list
  // (e.g. Closed) yields no lifecycle buttons, and a missing field also yields
  // none rather than guessing from a duplicated client-side graph.
  const targets = caseDetail.nextStates ?? [];
  const lifecycle = [...new Set(targets)]
    .sort((a, b) => orderRank(a) - orderRank(b))
    .map((target) => buttonFor(target, caseDetail));
  const primary = lifecycle;
  const secondary = buildSecondaryItems(caseDetail);

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
      {primary.length === 1 && (
        // A single reachable state needs no menu — show the transition
        // itself as one click rather than "Change state" → pick the only item.
        <Button
          size="small"
          variant="contained"
          color={primary[0].color}
          startIcon={primary[0].icon}
          onClick={() => runPrimary(primary[0])}
        >
          {primary[0].label}
        </Button>
      )}
      {primary.length > 1 && (
        <>
          <Button
            size="small"
            variant="contained"
            color="primary"
            endIcon={<ChevronDown size={16} />}
            onClick={(e) => setStateMenuAnchor(e.currentTarget)}
          >
            Change state
          </Button>
          <Menu
            anchorEl={stateMenuAnchor}
            open={!!stateMenuAnchor}
            onClose={() => setStateMenuAnchor(null)}
          >
            {primary.map((p) => (
              <MenuItem
                key={p.targetState}
                onClick={() => {
                  setStateMenuAnchor(null);
                  runPrimary(p);
                }}
                sx={{ gap: 1.25, minHeight: 36 }}
              >
                <Box sx={{ color: `${p.color}.main`, display: "flex" }}>
                  {p.icon}
                </Box>
                {p.label}
              </MenuItem>
            ))}
          </Menu>
        </>
      )}

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
        {secondary.map((item) => {
          const menuItem = (
            <MenuItem
              key={item.key}
              disabled={item.disabled}
              onClick={() => {
                if (item.disabled) return;
                setMenuAnchor(null);
                void onAction({ secondary: item.key });
              }}
              sx={{ gap: 1.25, minHeight: 36 }}
            >
              {item.icon}
              {item.label}
            </MenuItem>
          );
          return [
            // A disabled MenuItem has `pointer-events: none`, so a Tooltip only
            // fires when it wraps a non-disabled span. Wrap only the tooltip item
            // (which is disabled, so not keyboard-focusable anyway); leave the
            // other items as bare MenuItems so the menu's arrow-key nav works.
            item.tooltip ? (
              <Tooltip key={item.key} title={item.tooltip}>
                <Box component="span" sx={{ display: "block" }}>
                  {menuItem}
                </Box>
              </Tooltip>
            ) : (
              menuItem
            ),
            item.divider ? (
              <Box
                key={`${item.key}-divider`}
                sx={{ borderTop: 1, borderColor: "divider", my: 0.25 }}
              />
            ) : null,
          ];
        })}
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
