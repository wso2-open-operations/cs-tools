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

import { Box, Button, Menu, MenuItem, Tooltip, Typography } from "@wso2/oxygen-ui";
import {
  ArrowRight,
  Ban,
  CalendarClock,
  CheckCircle,
  ChevronDown,
  Play,
  Send,
  Undo2,
  UserCheck,
} from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import {
  changeRequestTransitionLabel,
  isDestructiveChangeRequestTransition,
} from "@features/csm-operations/utils/changeRequests";
import type { BeChangeRequestDetail } from "@api/backend/types";

/**
 * Icon and colour for a transition *into* a given state. The button LABEL is
 * never stored here — it always comes from `changeRequestTransitionLabel`,
 * same "no invented verbs" convention as `IncidentActionBar`'s `TargetConfig`.
 */
type TargetConfig = {
  color: "primary" | "success" | "warning" | "error";
  icon: JSX.Element;
};

const TARGET_CONFIG: Record<string, TargetConfig> = {
  assess: { color: "primary", icon: <Send size={16} /> },
  scheduled: { color: "primary", icon: <CalendarClock size={16} /> },
  implement: { color: "primary", icon: <Play size={16} /> },
  review: { color: "primary", icon: <CheckCircle size={16} /> },
  customer_review: { color: "primary", icon: <UserCheck size={16} /> },
  closed: { color: "primary", icon: <CheckCircle size={16} /> },
  rollback: { color: "error", icon: <Undo2 size={16} /> },
  canceled: { color: "error", icon: <Ban size={16} /> },
};

/**
 * Presentation for a transition this bar has no curated config for — a state
 * the backend added, or one of the lifecycle states with no agreed action
 * verb yet. Same fallback convention as `IncidentActionBar`'s
 * `DEFAULT_TARGET_CONFIG`, so a new backend state stays renderable and
 * clickable with no frontend change.
 */
const DEFAULT_TARGET_CONFIG: TargetConfig = {
  color: "primary",
  icon: <ArrowRight size={16} />,
};

/**
 * Forward progression through the lifecycle. Used for one thing only: picking
 * which single target gets the primary button when several are legal at once.
 * This array owns ordering, never legality.
 *
 * Membership doubles as primary-button eligibility, so it deliberately
 * excludes the destructive off-ramps and every uncurated state: without a
 * curated action label there is no evidence a target is *the* expected
 * forward move, so it goes in the overflow menu instead.
 */
const FORWARD_ORDER: readonly string[] = [
  "assess",
  "scheduled",
  "implement",
  "review",
  "customer_review",
  "closed",
];

/** Menu ordering: forward moves first, destructive off-ramps last. */
const MENU_ORDER: readonly string[] = [...FORWARD_ORDER, "rollback", "canceled"];

/**
 * States this bar never offers, no matter what `legalNextStates` contains.
 *
 * Do not delete this filter because "the list doesn't include them anyway".
 * Neither state is human-enterable in the backing system: of its 38 UI
 * actions on the change-request table, none sets either one. Both are reached
 * only by automation — rollback is written by the workflow that handles a
 * rejected review, customer approval by the approval process itself. Setting
 * either by hand from here would leave a record sitting in an approval state
 * with no approver record behind it, which is an audit hole rather than a
 * shortcut. The exclusion is deliberately unconditional so a future backend
 * change that starts returning them cannot silently reopen it.
 */
const NEVER_OFFERED_TARGETS: readonly string[] = ["rollback", "customer_approval"];

/** Sort key for a target: curated order first, uncurated states after. */
function menuRank(target: string): number {
  const index = MENU_ORDER.indexOf(target);
  return index === -1 ? MENU_ORDER.length : index;
}

/**
 * Prerequisites the state machine itself doesn't express. A target can be
 * legal per `legalNextStates` and still be blocked by a missing field the
 * backing system checks on write — offering it anyway just round-trips into a
 * rejection, so it renders disabled with the reason instead.
 *
 * Deliberately a per-target map rather than a special case for `assess`: the
 * same situation (legal transition, unmet prerequisite) can apply to any
 * target.
 */
const TARGET_BLOCKED_REASON: Record<
  string,
  (cr: BeChangeRequestDetail) => string | null
> = {
  assess: (cr) =>
    cr.assignedTeam ? null : "Set an assigned team before requesting approval",
};

interface ChangeRequestActionBarProps {
  cr: BeChangeRequestDetail;
  /** True while a state-changing request for this CR is in flight. */
  isPending: boolean;
  /**
   * Fired with the target state the engineer picked. The caller decides how
   * to apply it — a direct patch for most targets, or (for the destructive
   * ones flagged by `changeRequestTransitionRequiresReason`) opening a dialog
   * to collect the reason first. Same split of responsibility as
   * `IncidentActionBar` + `CsmIncidentDetailPage`.
   */
  onAction: (target: string) => void;
}

/**
 * Lifecycle action bar for the change-request detail page. Every button comes
 * straight from the record's own `legalNextStates` — there is no client-side
 * state machine here and no hardcoded state list, so a transition the backend
 * starts offering appears with no frontend change. Renders nothing when
 * `legalNextStates` is empty or absent (a terminal state, or a record the
 * caller may not transition).
 *
 * Exactly one target — the first forward move present, by `FORWARD_ORDER` —
 * gets a primary button; everything else sits behind a "Change state"
 * overflow menu. The header this sits in already carries Back, Clone and
 * Edit, so a row of eight buttons would bury the one action the engineer
 * actually wants.
 */
export default function ChangeRequestActionBar({
  cr,
  isPending,
  onAction,
}: ChangeRequestActionBarProps): JSX.Element | null {
  const [stateMenuAnchor, setStateMenuAnchor] = useState<HTMLElement | null>(null);

  // Single choke point for what is renderable: the exclusion below therefore
  // covers the primary button, the overflow menu, and states rendered through
  // `DEFAULT_TARGET_CONFIG` alike.
  const targets = Array.from(
    new Set(
      (cr.legalNextStates ?? []).filter(
        (s) => !!s && s !== cr.state && !NEVER_OFFERED_TARGETS.includes(s),
      ),
    ),
  ).sort((a, b) => menuRank(a) - menuRank(b));
  if (targets.length === 0) return null;

  const primaryTarget = targets.find((t) => FORWARD_ORDER.includes(t));
  const menuTargets = targets.filter((t) => t !== primaryTarget);

  const dispatch = (target: string): void => {
    setStateMenuAnchor(null);
    onAction(target);
  };

  const configFor = (target: string): TargetConfig =>
    TARGET_CONFIG[target] ?? DEFAULT_TARGET_CONFIG;

  const blockedReason = (target: string): string | null =>
    TARGET_BLOCKED_REASON[target]?.(cr) ?? null;

  const renderPrimary = (target: string): JSX.Element => {
    const { color, icon } = configFor(target);
    const label = changeRequestTransitionLabel(target);
    const reason = blockedReason(target);
    if (reason) {
      return (
        <Tooltip title={reason}>
          {/* A disabled button is not focusable, so the tooltip alone would be
              unreachable by keyboard — this focusable, labelled wrapper is
              what exposes the reason to assistive tech. */}
          <Box
            component="span"
            tabIndex={0}
            aria-label={`${label}: ${reason}`}
            sx={{ flexShrink: 0 }}
          >
            <Button
              size="small"
              variant="contained"
              color={color}
              startIcon={icon}
              disabled
              sx={{ flexShrink: 0 }}
            >
              {label}
            </Button>
          </Box>
        </Tooltip>
      );
    }
    return (
      <Button
        size="small"
        variant="contained"
        color={color}
        startIcon={icon}
        loading={isPending}
        onClick={() => dispatch(target)}
        sx={{ flexShrink: 0 }}
      >
        {label}
      </Button>
    );
  };

  return (
    <Box sx={{ display: "flex", gap: 1, flexShrink: 0 }}>
      {primaryTarget && renderPrimary(primaryTarget)}
      {menuTargets.length > 0 && (
        <>
          <Button
            size="small"
            variant={primaryTarget ? "outlined" : "contained"}
            color="primary"
            endIcon={<ChevronDown size={16} />}
            disabled={isPending}
            aria-haspopup="menu"
            onClick={(e) => setStateMenuAnchor(e.currentTarget)}
          >
            Change state
          </Button>
          <Menu
            anchorEl={stateMenuAnchor}
            open={!!stateMenuAnchor}
            onClose={() => setStateMenuAnchor(null)}
            // `disabledItemsFocusable` keeps a blocked entry reachable by
            // keyboard so its reason can actually be read, instead of the item
            // being skipped over silently.
            MenuListProps={{
              "aria-label": "Change state",
              disabledItemsFocusable: true,
            }}
          >
            {menuTargets.map((target) => {
              const { color, icon } = configFor(target);
              const label = changeRequestTransitionLabel(target);
              const reason = blockedReason(target);
              const destructive = isDestructiveChangeRequestTransition(target);
              return (
                <MenuItem
                  key={target}
                  disabled={isPending || !!reason}
                  aria-label={reason ? `${label}: ${reason}` : label}
                  onClick={() => {
                    if (reason) return;
                    dispatch(target);
                  }}
                  sx={{ gap: 1.25, minHeight: 36, alignItems: "flex-start", py: 1 }}
                >
                  <Box sx={{ color: `${color}.main`, display: "flex", mt: 0.25 }}>
                    {icon}
                  </Box>
                  <Box sx={{ display: "flex", flexDirection: "column", minWidth: 0 }}>
                    <Box
                      component="span"
                      sx={{ color: destructive ? "error.main" : "inherit" }}
                    >
                      {label}
                    </Box>
                    {reason && (
                      <Typography variant="caption" color="text.secondary">
                        {reason}
                      </Typography>
                    )}
                  </Box>
                </MenuItem>
              );
            })}
          </Menu>
        </>
      )}
    </Box>
  );
}
