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

import type {
  BeChangeRequestApproval,
  BeChangeRequestDetail,
  BeChangeRequestImpact,
  BeChangeRequestSearchPayload,
  BeChangeRequestState,
  BeChangeRequestType,
} from "@api/backend/types";
import { isBlankHtml, sanitizeRichTextHtml } from "@utils/sanitizeHtml";

type ChipColor = "default" | "info" | "warning" | "success" | "error";

const STATE_LABEL: Record<BeChangeRequestState, string> = {
  new: "New",
  assess: "Assess",
  authorize: "Authorize",
  customer_approval: "Customer Approval",
  scheduled: "Scheduled",
  implement: "Implement",
  review: "Review",
  customer_review: "Customer Review",
  rollback: "Rollback",
  closed: "Closed",
  canceled: "Canceled",
};

// State chip colour: approvals/reviews are in-flight (info), implement is active
// (warning), rollback/cancel are problem states (error), closed is terminal-good.
const STATE_COLOR: Record<BeChangeRequestState, ChipColor> = {
  new: "default",
  assess: "info",
  authorize: "info",
  customer_approval: "info",
  scheduled: "info",
  implement: "warning",
  review: "info",
  customer_review: "info",
  rollback: "error",
  closed: "success",
  canceled: "error",
};

const IMPACT_LABEL: Record<BeChangeRequestImpact, string> = {
  high: "High",
  medium: "Medium",
  low: "Low",
};

const IMPACT_COLOR: Record<BeChangeRequestImpact, ChipColor> = {
  high: "error",
  medium: "warning",
  low: "default",
};

/** All CR states, for a filter control. */
export const CHANGE_REQUEST_STATES = Object.keys(STATE_LABEL) as BeChangeRequestState[];

/**
 * The 9 states that make up the CR's linear forward path, in order —
 * `CHANGE_REQUEST_STATES` minus `rollback`/`canceled`. Those two are
 * destructive off-ramps reachable from several points in the path (see
 * `DESTRUCTIVE_TRANSITIONS` below), not sequential steps in it, so a lifecycle
 * step indicator built from this array should render them separately rather
 * than forcing them into the same line.
 */
export const CHANGE_REQUEST_FORWARD_STATES = CHANGE_REQUEST_STATES.filter(
  (s) => s !== "rollback" && s !== "canceled",
);

/** True for `rollback`/`canceled`: an off-ramp from the linear forward path, not a step in it. */
export function isChangeRequestOffRampState(state?: string | null): boolean {
  return state === "rollback" || state === "canceled";
}

/** All CR impact levels, for a filter control. */
export const CHANGE_REQUEST_IMPACTS = Object.keys(IMPACT_LABEL) as BeChangeRequestImpact[];

function humanize(value: string): string {
  return value.replace(/_/g, " ");
}

export function changeRequestStateLabel(state?: string | null): string {
  if (!state) return "—";
  return STATE_LABEL[state as BeChangeRequestState] ?? humanize(state);
}

export function changeRequestStateColor(state?: string | null): ChipColor {
  return STATE_COLOR[state as BeChangeRequestState] ?? "default";
}

/**
 * Human-readable reason a comment cannot be posted on this change request right
 * now, or `null` when it can. Change requests don't share the case work-state
 * model — the only gate is terminal state.
 */
export function changeRequestCommentGateReason(
  state?: string | null,
): string | null {
  if (state === "closed" || state === "canceled") {
    return "Comments are disabled on a closed or canceled change request.";
  }
  return null;
}

export function changeRequestImpactLabel(impact?: string | null): string {
  if (!impact) return "—";
  return IMPACT_LABEL[impact as BeChangeRequestImpact] ?? humanize(impact);
}

export function changeRequestImpactColor(impact?: string | null): ChipColor {
  return IMPACT_COLOR[impact as BeChangeRequestImpact] ?? "default";
}

// Approval-stage / approver status labels and colours. The backend passes
// these through from the data source without validating them (see
// `BeChangeRequestApprover.status`), so both maps are deliberately partial —
// unrecognized values fall back to a humanized version of the raw string
// rather than crashing or rendering nothing.
const APPROVAL_STATUS_LABEL: Record<string, string> = {
  APPROVED: "Approved",
  REJECTED: "Rejected",
  PENDING: "Pending",
  REQUESTED: "Requested",
  NOT_REQUIRED: "Not required",
  CANCELLED: "Cancelled",
  NO_CONSENSUS: "No consensus",
};

const APPROVAL_STATUS_COLOR: Record<string, ChipColor> = {
  APPROVED: "success",
  REJECTED: "error",
  PENDING: "warning",
  REQUESTED: "warning",
  NOT_REQUIRED: "default",
  CANCELLED: "default",
  NO_CONSENSUS: "error",
};

export function approvalStatusLabel(status?: string | null): string {
  if (!status) return "—";
  return APPROVAL_STATUS_LABEL[status.toUpperCase()] ?? humanize(status.toLowerCase());
}

export function approvalStatusColor(status?: string | null): ChipColor {
  if (!status) return "default";
  return APPROVAL_STATUS_COLOR[status.toUpperCase()] ?? "default";
}

/** Stage-level statuses that mean the stage is actively waiting on someone. */
const WAITING_APPROVAL_STATUSES = new Set(["PENDING", "REQUESTED"]);

/**
 * Plain-language reason a change request isn't moving on its own right now,
 * derived from its approval stages (`GET /change-requests/{id}/approvals`) —
 * the same data `ChangeRequestApprovals` renders. Names the first stage still
 * waiting on someone (in stage order, not necessarily severity order): e.g.
 * "Awaiting Authorize approval" or, when the stage carries a named approver
 * group, "Awaiting Devops Approval". Returns `null` when nothing is currently
 * blocking on approval — no waiting stage, or the approvals haven't loaded
 * yet — so callers should treat `null` as "no reason to show", not an error.
 */
export function changeRequestBlockingReason(
  approvals: BeChangeRequestApproval[] | undefined,
): string | null {
  const waiting = approvals?.find((a) => WAITING_APPROVAL_STATUSES.has(a.status.trim().toUpperCase()));
  if (!waiting) return null;
  const who = waiting.approverName?.trim() || waiting.stage;
  // Approver-group names sometimes already say "Approval" ("Devops
  // Approval"); avoid a doubled "approval approval" in that case.
  return /approval/i.test(who) ? `Awaiting ${who}` : `Awaiting ${who} approval`;
}

// ---------------------------------------------------------------------------
// Lifecycle transitions
//
// The backing system owns transition legality — a change request carries its
// own `legalNextStates`, and nothing here re-derives or second-guesses it.
// These maps only supply the *presentation* of a transition the record has
// already declared legal.
// ---------------------------------------------------------------------------

/**
 * Action-phrased label for a transition *into* a given state. Phrased as the
 * action being taken ("Schedule", "Mark implemented"), not as the destination,
 * because the state chip next to the action bar already names the state —
 * same "no invented verbs for the state itself" convention as
 * `IncidentActionBar`/`CaseActionBar`.
 *
 * Deliberately partial: `new`, `authorize` and `customer_approval` have no
 * agreed action verb yet, and a state the backend adds later has none by
 * definition. Both fall back to a sentence-cased version of the raw value via
 * {@link changeRequestTransitionLabel}, so they still render and still work.
 */
const TRANSITION_LABEL: Record<string, string> = {
  assess: "Request approval",
  scheduled: "Schedule",
  implement: "Start implementation",
  review: "Mark implemented",
  customer_review: "Send for customer review",
  closed: "Close",
  rollback: "Roll back",
  canceled: "Cancel change",
};

/**
 * Transitions that are destructive and effectively irreversible. These are
 * never offered as a primary button, always render in the error colour, and
 * require a stated reason before they fire.
 */
const DESTRUCTIVE_TRANSITIONS: readonly string[] = ["rollback", "canceled"];

/** `customer_review` -> `Customer review`. */
function sentenceCase(raw: string): string {
  const words = raw.replace(/_/g, " ").trim();
  if (!words) return raw;
  return words.charAt(0).toUpperCase() + words.slice(1).toLowerCase();
}

/** The action-phrased label for a transition target, curated or generic. */
export function changeRequestTransitionLabel(target: string): string {
  return TRANSITION_LABEL[target] ?? sentenceCase(target);
}

/** True when this transition is destructive: menu-only, error-coloured. */
export function isDestructiveChangeRequestTransition(target: string): boolean {
  return DESTRUCTIVE_TRANSITIONS.includes(target);
}

/**
 * True when moving to `target` must not happen without a stated reason. The
 * reason is recorded as an ordinary comment on the change request *before*
 * the state is patched — the PATCH contract has no reason or comment field of
 * its own. See `ChangeRequestTransitionReasonDialog`.
 */
export function changeRequestTransitionRequiresReason(target: string): boolean {
  return isDestructiveChangeRequestTransition(target);
}

export interface ChangeRequestFilters {
  search: string;
  states: BeChangeRequestState[];
  impacts: BeChangeRequestImpact[];
  /** YYYY-MM-DD local date string, or empty. */
  closedStartDate: string;
  /** YYYY-MM-DD local date string, or empty. */
  closedEndDate: string;
  /** Selected SRE team `sreGroupId`s — sent as a generic
   * `{ field: "assignmentGroupId", op: "in" }` filter entry (see
   * `buildChangeRequestSearchFilters`), not a named payload field. */
  sreTeamIds: string[];
}

export const DEFAULT_CR_FILTERS: ChangeRequestFilters = {
  search: "",
  states: [],
  impacts: [],
  closedStartDate: "",
  closedEndDate: "",
  sreTeamIds: [],
};

/** Count non-search active filters (used for the badge on the Filters button). */
export function countActiveCRFilters(filters: ChangeRequestFilters): number {
  return (
    (filters.states.length > 0 ? 1 : 0) +
    (filters.impacts.length > 0 ? 1 : 0) +
    (filters.closedStartDate ? 1 : 0) +
    (filters.closedEndDate ? 1 : 0) +
    (filters.sreTeamIds.length > 0 ? 1 : 0)
  );
}

/** Convert a YYYY-MM-DD date picker value to an ISO 8601 string at midnight UTC. */
export function crDateOnlyToISOStart(date: string): string {
  return `${date}T00:00:00Z`;
}

/** Convert a YYYY-MM-DD date picker value to an ISO 8601 string at end-of-day UTC. */
export function crDateOnlyToISOEnd(date: string): string {
  return `${date}T23:59:59Z`;
}

/**
 * Build `ChangeRequestSearchPayload.filters` from the UI's
 * {@link ChangeRequestFilters} plus the (separately debounced) search text
 * — mirrors `buildIncidentSearchFilters` in `incidents.ts`.
 */
export function buildChangeRequestSearchFilters(
  filters: ChangeRequestFilters,
  debouncedSearch: string,
): NonNullable<BeChangeRequestSearchPayload["filters"]> {
  return {
    ...(debouncedSearch.length > 0 && { searchQuery: debouncedSearch }),
    ...(filters.states.length > 0 && { states: filters.states }),
    ...(filters.impacts.length > 0 && { impacts: filters.impacts }),
    ...(filters.closedStartDate && {
      closedStartDate: crDateOnlyToISOStart(filters.closedStartDate),
    }),
    ...(filters.closedEndDate && {
      closedEndDate: crDateOnlyToISOEnd(filters.closedEndDate),
    }),
    ...(filters.sreTeamIds.length > 0 && {
      filters: [
        { field: "assignmentGroupId" as const, op: "in" as const, values: filters.sreTeamIds },
      ],
    }),
  };
}

// ---------------------------------------------------------------------------
// Clone ("create similar") support: pre-fills the create form from an
// existing change request via router state, so promoting the same change
// through another environment doesn't mean re-typing every field.
//
// This is a *partial* clone by necessity, not by choice: `GET
// /change-requests/{id}` (BeChangeRequestDetail) and `POST /change-requests`
// (BeCreateChangeRequestPayload) are asymmetric on the backend today —
// several fields the create form can set are never returned by the read,
// and several fields the detail page can show have no create-time
// equivalent at all. Concretely (verified against the entity service's own
// request/response structs, not just the two frontend types):
//   - `priority` is write-only — accepted by create, never present on the
//     read response — so there is no source value to copy from, ever,
//     regardless of how the form is wired.
//   - `implementationPlan` and `riskImpactAnalysis` are write-only for the
//     same reason.
//   - `impactDescription`, `serviceOutage`, `communicationPlan`, and
//     `rollbackPlan` are read-only today — the create payload has no field
//     for any of them.
//   - `project`, `case`, `deployment`, `deployedProduct`, and `product` are
//     read-only refs with no create-time field to set them from at all.
//   - `assignedTeam` is read-only; create's nearest-sounding field
//     (`groupId`, "Assignment group") is a *different* underlying reference
//     with no confirmed equivalence to `assignedTeam` — mapping one into the
//     other would be a guess, not a verified carry-over, so it's left alone.
// (`category` and `risk` aren't in this gap analysis at all: `category` has
// no editable control anywhere in this portal — see
// `BeCreateChangeRequestPayload`'s doc comment — and `risk`/`serviceId`/
// `serviceOfferingId`/`configurationItemId` were removed from the create form
// entirely, since none of them exist on the real ServiceNow CR form.)
// None of the above can be safely carried over without either fabricating
// data or guessing at an unconfirmed field mapping, so this only clones the
// fields that are genuinely the same field on both sides: `subject`,
// `description`, `justification`, `testPlan`, `type`, `impact`, and
// `assignedEngineer`. Everything else resets to the create form's own
// defaults, same as a from-scratch change request — see
// `CLONE_SOURCE_GAP_MESSAGE` for the user-facing disclosure of this gap.
export interface CloneChangeRequestNavState {
  /** For the banner shown on the create form — never sent to the backend. */
  sourceNumber?: string;
  subject?: string;
  description?: string;
  justification?: string;
  testPlan?: string;
  type?: BeChangeRequestType;
  impact?: BeChangeRequestImpact;
  assignedEngineerId?: string;
  /** Display label for `assignedEngineerId` until a fresh search resolves it. */
  assignedEngineerLabel?: string;
}

/** Rich-text field carried into the clone form only when it has real content. */
function cloneableHtml(html?: string | null): string | undefined {
  if (!html || isBlankHtml(html)) return undefined;
  return sanitizeRichTextHtml(html);
}

/**
 * Builds the router-state payload for a change request's "Clone" action.
 * Deliberately omits: environment/deployment, state, approval fields
 * (`hasCustomerApproved`/`hasCustomerReviewed`/`approvedBy`/`approvedOn`),
 * planned start/end, and every auto-numbered/timestamp/created-by field —
 * per this feature's requirement that promoting a change to a new
 * environment must never silently carry an approval or a stale schedule
 * across. Comments and attachments are never part of this payload; they
 * belong to the original record only.
 */
export function buildCloneChangeRequestNavState(
  cr: BeChangeRequestDetail,
): CloneChangeRequestNavState {
  return {
    sourceNumber: cr.number,
    subject: cr.subject ?? undefined,
    description: cloneableHtml(cr.description),
    justification: cloneableHtml(cr.justification),
    testPlan: cloneableHtml(cr.testPlan),
    type: (cr.type as BeChangeRequestType) ?? undefined,
    impact: (cr.impact as BeChangeRequestImpact) ?? undefined,
    assignedEngineerId: cr.assignedEngineer?.id || undefined,
    assignedEngineerLabel: cr.assignedEngineer?.name || undefined,
  };
}

/**
 * User-facing disclosure shown on the create form when it was opened via
 * Clone, so the field gap documented above is visible rather than silently
 * dropped. Kept as a single shared string so the detail page (if it ever
 * wants a preview) and the create page stay in sync.
 */
export const CLONE_SOURCE_GAP_MESSAGE =
  "Copied the subject, description, justification, test plan, type, impact, and assigned engineer. " +
  "Priority, implementation plan, risk/impact analysis, backout plan, assignment group, " +
  "linked project/case, and affected product aren't available to copy and need to be re-entered. " +
  "Deployment, schedule, and approval fields are intentionally left blank for you to set for the new environment.";
