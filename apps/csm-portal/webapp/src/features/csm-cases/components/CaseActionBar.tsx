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
  CircularProgress,
  Menu,
  MenuItem,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import {
  AlertTriangle,
  ArrowLeftRight,
  ArrowRight,
  CalendarClock,
  CheckCircle,
  ChevronDown,
  Clock,
  FileText,
  Gauge,
  GitBranch,
  GitPullRequest,
  Inbox,
  Link as LinkIcon,
  MessageCircleQuestion,
  PauseCircle,
  Pencil,
  Play,
  Send,
  Undo2,
  User,
  Wrench,
} from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import type {
  CaseLifecycleAction,
  CsmCaseDetail,
} from "@features/csm-cases/types/csmCases";
import { canRequestCaseUpdate } from "@features/csm-cases/utils/caseUpdateRequests";
import type {
  CaseState,
  SeverityOrUnset,
} from "@features/csm-dashboard/types/abtDashboard";
import { stateLabel } from "@features/csm-dashboard/utils/abtDashboard";
import UserRefLink from "@components/UserRefLink";

/**
 * Presentation for a transition *into* a given state. The button LABEL is never
 * stored here — it always comes from `stateLabel(targetState)`, so the bar
 * honours the backend transition graph verbatim and never invents UI-specific
 * verbs. This only carries the icon/colour and the lifecycle action used for
 * the post-transition toast. `closed`/`solution_proposed` don't need a
 * confirm gate here — `onAction` opens the Post Resolution Activity dialog
 * for those, which doubles as the confirmation step.
 */
type TargetConfig = {
  action: CaseLifecycleAction;
  color: "primary" | "success" | "warning" | "error";
  icon: JSX.Element;
};

/** A concrete button: a target state plus its presentation. */
type PrimaryButton = TargetConfig & {
  targetState: CaseState;
  label: string;
  tooltip?: string;
};

/**
 * Per-target-state presentation. One entry per state a case can move INTO.
 * `closed` and `solution_proposed` dispatch immediately like every other
 * target, but `onAction` (in CsmCaseDetailPage) opens the Post Resolution
 * Activity dialog for those two (resolution code, cause, close notes —
 * ISSU-026) instead of PATCHing right away — that dialog is the confirm
 * gate for a customer-notifying transition, so no separate one is needed here.
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
  },
  closed: {
    action: "close",
    color: "warning",
    icon: <CheckCircle size={16} />,
  },
  // Not a real reopen — the data source has no transition out of closed. The
  // backend only puts `reopened` in a closed case's `nextStates` as a signal
  // that a related case may still be created (see that field's doc); `action`
  // routes to the create-related-case flow in `onAction` instead of a PATCH.
  reopened: {
    action: "create_related_case",
    color: "primary",
    icon: <LinkIcon size={16} />,
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
  reopened: "Create related case",
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
 *   - Hold auto-closure              → ISSU-027 (PATCH /cases/{id} { autocloseHoldUntil }, see SetAutocloseHoldDialog.tsx)
 *   - Edit case details              → subject/description/deployment/deployed product (see EditCaseDetailsDialog.tsx)
 *   - Create incident from case      → ISSU-021 (POST /incidents { parentId }, see
 *                                       CreateIncidentPage.tsx's read of the nav state)
 *   - Create service request…        → navigates to the service-request create form
 *                                       pre-linked to this case (POST /cases { type:
 *                                       "service_request", relatedCaseId }, see
 *                                       CreateServiceRequestPage.tsx's read of the nav
 *                                       state). Mirrors the same action already reachable
 *                                       from the Related tab's Linked service requests
 *                                       card — kept in both places, same as "Create change
 *                                       request…" below.
 *   - Link to incident               → ISSU-021 (PATCH /cases/{id} { parentId }, see
 *                                       LinkIncidentDialog.tsx)
 *   - Raise Git issue                → ISSU-020
 *   - Request update…                → nudge the customer for a response using a fixed
 *                                       reminder template or a custom message. Only offered
 *                                       while the case is Awaiting info or Solution proposed
 *                                       — see `POST /cases/{id}/request-update` and
 *                                       RequestUpdateDialog.tsx.
 *   - Create change request          → service-request-only. Navigates to the change-request
 *                                       create form pre-filled with this service request as the
 *                                       "Originating service request" (POST /change-requests, then
 *                                       PATCH { caseId } — see CreateChangeRequestPage.tsx).
 *   - Set fix ETA                    → PATCH /cases/{id} { bestCaseFixEta?, mostLikelyFixEta?,
 *                                       worstCaseFixEta?, addPublicComment?, product?,
 *                                       publicTicket? } combined in one call, see SetFixEtaDialog.tsx
 *   - Log time                       → ISSU-017
 *   - Change severity                → PATCH /cases/{id} { severity }, already fully
 *                                       backend-supported (see ChangeSeverityDialog.tsx)
 *
 * Intentionally NOT here (removed as duplicate entry points once their target
 * tab already exposes the same action directly, so there's one way to do each
 * thing rather than two):
 *   - Manage watchers   → the Watchers tab does inline add/remove already; the
 *                          menu item only ever jumped there with no dialog of
 *                          its own (ISSU-018).
 *   - Link to another case → the Related tab's "Linked service requests" card
 *                          has its own "Link to another case…" button wired to
 *                          the same LinkCaseDialog.
 *   - Link to incident → relocated to the Related tab's new "Linked incident"
 *                          widget, which has its own "Link to incident…"
 *                          button — same pattern as "Link to another case"
 *                          above.
 *   - Create task → hidden with no replacement; the Tasks tab this would
 *                          otherwise belong to is still hidden in
 *                          CsmCaseDetailPage's TAB_DEFS, so task creation is
 *                          unreachable until that's addressed separately
 *                          (deliberate, not an oversight).
 *   - Copy case link → removed per product decision, no replacement.
 *   - Request a call    → the Call requests tab has its own "Create call
 *                          request" button (CallRequestsWidget.tsx); the menu
 *                          item only jumped there and auto-opened that same
 *                          dialog (ISSU-008).
 *   - Open in ServiceNow → this platform replaces ServiceNow; no back-link
 *   - Escalate to lead → withdrawn, no backend flow planned yet
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

  // Mark / recall the case's workaround. Pauses the case's Workaround SLA
  // clock while set; recalling clears it. Not assignee-gated (unlike
  // pause/resume work above) -- any engineer working the case can mark or
  // recall it. Blocked once the case is closed, same as every other write
  // action below.
  {
    const caseClosedForWorkaround = caseDetail.state === "closed";
    items.push({
      key: "toggle_workaround_provided",
      label: caseDetail.workaroundProvidedOn
        ? "Recall workaround"
        : "Provide workaround",
      icon: caseDetail.workaroundProvidedOn ? (
        <Undo2 size={16} />
      ) : (
        <Wrench size={16} />
      ),
      divider: true,
      disabled: caseClosedForWorkaround,
      tooltip: caseClosedForWorkaround
        ? "This case is closed — it's read-only."
        : undefined,
    });
  }

  // Assignee changes are rejected while a case is Work in progress + Ongoing:
  // the backend silently reverts the change and still reports success, so the
  // reassign would appear to work while the assignee stays put. Gate the action
  // here so we never fire that no-op. The work must be paused first (by the
  // current assignee) or the reassignment handled by a lead.
  const reassignBlocked =
    caseDetail.state === "work_in_progress" && caseDetail.workState === "ongoing";

  // A closed case is done — filing a new Git issue against it doesn't fit
  // the same "closed is read-only" rule already applied to comments,
  // attachments, and time tracking (see CsmCaseDetailPage.tsx's isClosed).
  const caseClosed = caseDetail.state === "closed";

  // Git issues may only be raised while the case is in a state SN's own
  // gate allows — CaseGithubIssueUtils.isCaseActionable's ALLOWED_CASE_STATES
  // ('1', '10', '1002', '1003', '1006': Open, Work in progress, [a legacy SN
  // state id with no CSM-side equivalent], Waiting on WSO2, Reopened).
  // Notably this does NOT include Awaiting info — anything outside this
  // list (Awaiting info, Solution proposed, Closed) blocks the action.
  // `caseDetail.state` is the normalized label (see `uiStateFromBe`) — this
  // must match the real `CaseState` values (there is no "waiting_on_client"
  // state).
  const GIT_ISSUE_ALLOWED_STATES: readonly string[] = [
    "open",
    "work_in_progress",
    "waiting_on_wso2",
    "reopened",
  ];
  const gitIssueStateBlocked = !GIT_ISSUE_ALLOWED_STATES.includes(
    caseDetail.state,
  );

  // Opposite shape to the git-issue gate above: "Request update" is only
  // meaningful while the ball is meant to be in the customer's court, so it's
  // enabled ONLY in those two states rather than blocked in a few — mirrors
  // the backend's own state gate in `RequestCaseUpdate` (409 outside these
  // states). Also requires `assigneeIsMe`: the backend separately rejects a
  // non-assignee with 403 (same ownership rule `CreateCaseComment` already
  // enforces for public comments), so gating on state alone would let anyone
  // open the dialog and fill it in only to hit a confusing 403 on submit.
  const requestUpdateStateAllowed = canRequestCaseUpdate(caseDetail);
  const requestUpdateAllowed =
    requestUpdateStateAllowed && caseDetail.assigneeIsMe;

  // Only a service request can be the "Originating service request" a change
  // request links back to (see the create form's picker), so the action is
  // offered only there — mirrors the same `caseType === "service_request"`
  // gate the Related tab already applies to LinkedChangeRequestsWidget.
  if (caseDetail.caseType === "service_request") {
    items.push({
      key: "create_change_request",
      label: "Create change request…",
      icon: <GitPullRequest size={16} />,
      divider: true,
      disabled: caseClosed,
      tooltip: caseClosed ? "This case is closed — it's read-only." : undefined,
    });
  }

  items.push(
    {
      key: "raise_git_issue",
      label: "Raise internal Git issue…",
      icon: <GitBranch size={16} />,
      divider: true,
      disabled: caseClosed || gitIssueStateBlocked,
      tooltip: caseClosed
        ? "This case is closed — it's read-only."
        : gitIssueStateBlocked
          ? "Git issues can only be raised while the case is Open, Work in progress, Waiting on WSO2, or Reopened."
          : undefined,
    },
    {
      key: "request_update",
      label: "Request update…",
      icon: <MessageCircleQuestion size={16} />,
      divider: true,
      disabled: !requestUpdateAllowed,
      tooltip: !requestUpdateStateAllowed
        ? "Requesting an update is only available while the case is Awaiting info or has a proposed solution."
        : !requestUpdateAllowed
          ? "Only the assigned engineer can request an update on this case."
          : undefined,
    },
    {
      key: "reassign_engineer",
      label: "Assign / reassign engineer…",
      icon: <User size={16} />,
      divider: true,
      disabled: caseClosed || reassignBlocked,
      tooltip: caseClosed
        ? "This case is closed — it's read-only."
        : reassignBlocked
          ? "Can't reassign while the case is in progress and ongoing. Pause the work first, or ask the current assignee or a lead to reassign."
          : undefined,
    },
    {
      key: "change_severity",
      label: "Change severity…",
      icon: <Gauge size={16} />,
      disabled: caseClosed,
      tooltip: caseClosed ? "This case is closed — it's read-only." : undefined,
    },
    {
      key: "edit_case_details",
      label: "Edit case details…",
      icon: <Pencil size={16} />,
      disabled: caseClosed,
      tooltip: caseClosed ? "This case is closed — it's read-only." : undefined,
    },
    {
      key: "change_case_type",
      label: "Change case type…",
      icon: <ArrowLeftRight size={16} />,
      divider: true,
      disabled: caseClosed,
      tooltip: caseClosed ? "This case is closed — it's read-only." : undefined,
    },
    {
      key: "hold_auto_close",
      label: "Hold auto-closure…",
      icon: <PauseCircle size={16} />,
      divider: true,
      disabled: caseClosed,
      tooltip: caseClosed ? "This case is closed — it's read-only." : undefined,
    },
    {
      key: "create_incident",
      label: "Create incident from case…",
      icon: <AlertTriangle size={16} />,
      disabled: caseClosed,
      tooltip: caseClosed ? "This case is closed — it's read-only." : undefined,
    },
    {
      key: "create_service_request",
      label: "Create service request…",
      icon: <FileText size={16} />,
      divider: true,
      disabled: caseClosed,
      tooltip: caseClosed ? "This case is closed — it's read-only." : undefined,
    },
    {
      key: "set_fix_eta",
      label: "Set fix ETA…",
      icon: <CalendarClock size={16} />,
      divider: true,
      disabled: caseClosed,
      tooltip: caseClosed ? "This case is closed — it's read-only." : undefined,
    },
    { key: "log_time", label: "Log time…", icon: <Clock size={16} /> },
  );

  return items;
}

interface CaseActionBarProps {
  caseDetail: CsmCaseDetail;
  onAction: (
    action: CaseLifecycleAction | { secondary: string },
    targetState?: CaseState,
  ) => void | Promise<unknown>;
  /**
   * FE-only, advisory reason to disable the "Close" transition — e.g. an open
   * task still on the case. Purely a UX hint: the entity-service already
   * enforces the real close gate server-side, so a rejected close still
   * surfaces via the caller's own error handling even if this prop is unset
   * or stale. Only ever applied to a `targetState === "closed"` button.
   */
  closeBlockedReason?: string;
  /** True while a lifecycle PATCH from this bar (e.g. "Assign to me") is in
   * flight — disables the primary action and swaps its icon for a spinner,
   * so a click has visible feedback even before the resulting toast/state
   * change lands. */
  isPending?: boolean;
  /**
   * Acknowledge the case as the signed-in engineer. When omitted, the
   * acknowledge button is never rendered — so a caller that has no acknowledge
   * mutation wired up cannot show a dead button.
   */
  onAcknowledge?: () => void | Promise<unknown>;
  /** True while the acknowledge PATCH is in flight. */
  isAcknowledging?: boolean;
}

/**
 * Severities whose cases are worth acknowledging. S4 is excluded deliberately:
 * it mirrors which cases the out-of-band acknowledgement notifications are
 * raised for, so the button appears on exactly the cases an engineer could
 * already have acknowledged from a notification, and on no others.
 * `caseDetail.severity` may be `"unset"` (case has no severity value) —
 * typed `SeverityOrUnset` so `.has()` accepts it directly; it is never a
 * member of this set, so an unset-severity case is correctly never
 * acknowledgeable.
 */
const ACKNOWLEDGEABLE_SEVERITIES = new Set<SeverityOrUnset>(["S0", "S1", "S2", "S3"]);

/**
 * Whether the acknowledge action applies to this case: nobody has claimed it
 * yet and it is severe enough to be worth claiming. Acknowledgement is
 * first-write-wins, so once `acknowledgedBy` is set there is nothing left to
 * do and the button disappears rather than turning into a no-op.
 */
// eslint-disable-next-line react-refresh/only-export-components -- exported so CsmCaseDetailPage's startWork can reuse the same acknowledgeability check rather than duplicating it (fast-refresh DX only)
export function canAcknowledge(caseDetail: CsmCaseDetail): boolean {
  return (
    !caseDetail.acknowledgedBy && ACKNOWLEDGEABLE_SEVERITIES.has(caseDetail.severity)
  );
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
  closeBlockedReason,
  isPending = false,
  onAcknowledge,
  isAcknowledging = false,
}: CaseActionBarProps): JSX.Element {
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const [stateMenuAnchor, setStateMenuAnchor] = useState<HTMLElement | null>(null);

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
    if (p.targetState === "closed" && closeBlockedReason) return;
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
      {!!caseDetail.acknowledgedBy && ACKNOWLEDGEABLE_SEVERITIES.has(caseDetail.severity) && (
        // Leads the bar as context, read before the action buttons. Mutually
        // exclusive with the Acknowledge button below (only one of the two
        // ever renders, since one implies the case is already claimed and
        // the other implies it isn't) — they sit in different positions,
        // not "the same slot": this leads the bar, the button trails it.
        <Typography variant="body2" color="text.secondary" noWrap>
          Acknowledged by{" "}
          <UserRefLink
            name={caseDetail.acknowledgedBy.name}
            email={caseDetail.acknowledgedBy.email}
          />
        </Typography>
      )}
      {primary.length === 1 && (
        // A single reachable state needs no menu — show the transition
        // itself as one click rather than "Change state" → pick the only item.
        (() => {
          const p = primary[0];
          const blocked = p.targetState === "closed" && !!closeBlockedReason;
          const button = (
            <Button
              size="small"
              variant="contained"
              color={p.color}
              startIcon={isPending ? <CircularProgress size={14} color="inherit" /> : p.icon}
              disabled={blocked || isPending}
              onClick={() => runPrimary(p)}
            >
              {p.label}
            </Button>
          );
          return blocked ? (
            <Tooltip title={closeBlockedReason}>
              <Box component="span">{button}</Box>
            </Tooltip>
          ) : (
            button
          );
        })()
      )}
      {primary.length > 1 && (
        <>
          <Button
            size="small"
            variant="contained"
            color="primary"
            endIcon={isPending ? <CircularProgress size={14} color="inherit" /> : <ChevronDown size={16} />}
            disabled={isPending}
            onClick={(e) => setStateMenuAnchor(e.currentTarget)}
          >
            Change state
          </Button>
          <Menu
            anchorEl={stateMenuAnchor}
            open={!!stateMenuAnchor}
            onClose={() => setStateMenuAnchor(null)}
          >
            {primary.map((p) => {
              const blocked = p.targetState === "closed" && !!closeBlockedReason;
              const menuItem = (
                <MenuItem
                  key={p.targetState}
                  disabled={blocked}
                  onClick={() => {
                    if (blocked) return;
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
              );
              return blocked ? (
                <Tooltip key={p.targetState} title={closeBlockedReason}>
                  <Box component="span" sx={{ display: "block" }}>
                    {menuItem}
                  </Box>
                </Tooltip>
              ) : (
                menuItem
              );
            })}
          </Menu>
        </>
      )}

      {!!onAcknowledge && canAcknowledge(caseDetail) && (
        // Sits between the state control and "More": claiming a case is a
        // lighter act than moving it through its lifecycle, so it trails the
        // primary transition rather than leading the bar. No leading icon,
        // for the same plain label-only look as "Change state"/"More" — a
        // spinner still appears in its place while the claim is in flight.
        <Button
          size="small"
          variant="outlined"
          color="primary"
          startIcon={isAcknowledging ? <CircularProgress size={14} color="inherit" /> : undefined}
          disabled={isAcknowledging || isPending}
          onClick={() => void onAcknowledge()}
        >
          Acknowledge
        </Button>
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
    </Box>
  );
}
