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
  BeChangeRequestCategory,
  BeChangeRequestCustomerProposal,
  BeChangeRequestDetail,
  BeChangeRequestImpact,
  BeChangeRequestSearchPayload,
  BeChangeRequestState,
  BeChangeRequestType,
} from "@api/backend/types";
import { formatBackendTimestampForDisplay, parseBackendTimestamp } from "@utils/dateTime";
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
 * True for `rollback`/`canceled`: the two exits off the forward path (they are
 * reachable from several points in it, see `DESTRUCTIVE_TRANSITIONS` below).
 * The lifecycle stepper plots them in the customer portal's place for them,
 * after the forward stages (`CHANGE_REQUEST_LIFECYCLE_ORDER` in
 * `changeRequestStages.ts`), and styles them as exceptions.
 */
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

/**
 * Display labels for the approval stages of the change-request flow:
 * Normal changes go Peer Approval -> CAB Approval; Emergency changes have a
 * single CAB Approval (there is no separate Emergency CAB: the previous system has none
 * either); Standard changes have none. The backend decides which stages exist --
 * this only maps a stage name it returned to its label, so both the legacy
 * names of the previous system ("Assess", "Authorize") and the explicit ones ("Peer
 * Approval", "CAB Approval") read the same; the post-implementation "Review"
 * stage keeps its own name. Matching is case/space/punctuation-insensitive.
 *
 * "ECAB Approval" / "Emergency CAB" are HISTORY: an Emergency change raised
 * before ECAB was retired still carries such a stage (possibly still waiting on
 * approvers), and it keeps reading "ECAB Approval" so the record is shown as it
 * is. Nothing creates one any more.
 */
const KNOWN_APPROVAL_STAGE_LABELS: Record<string, string> = {
  assess: "Peer Approval",
  peer: "Peer Approval",
  peerapproval: "Peer Approval",
  authorize: "CAB Approval",
  cab: "CAB Approval",
  cabapproval: "CAB Approval",
  review: "Review",
  // Historic only (see above): a stage an earlier version provisioned for an Emergency change.
  ecab: "ECAB Approval",
  ecabapproval: "ECAB Approval",
  emergencycab: "ECAB Approval",
  emergencycabapproval: "ECAB Approval",
  // Stages the backend provisions for the CR's customer group (its members are
  // the approvers) on entering `customer_approval` / `customer_review`.
  customerapproval: "Customer Approval",
  customerreview: "Customer Review",
};

function knownApprovalStageLabel(stage?: string | null): string | null {
  if (!stage) return null;
  return KNOWN_APPROVAL_STAGE_LABELS[stage.toLowerCase().replace(/[^a-z]/g, "")] ?? null;
}

/** Label for an approval stage name, e.g. `Authorize` -> `CAB Approval`.
 * Unrecognised stages render as the backend sent them. */
export function approvalStageLabel(stage?: string | null): string {
  return knownApprovalStageLabel(stage) ?? (stage?.trim() || "Approval");
}

/**
 * The three change types a new change request can be created as, in the order
 * the ServiceNow "What type of change is required?" screen lists them. `value`
 * is the backend's `ChangeRequestType` enum value (entity-service
 * `domain.ChangeRequestType*`: "normal" / "standard" / "emergency"); the
 * create form requires exactly one of these. The type drives the approval
 * flow server-side: Normal = Peer -> CAB, Standard = none, Emergency = a single
 * CAB stage (and no customer approval or review: see
 * {@link EMERGENCY_CUSTOMER_STEPS_HELPER}).
 */
export const CHANGE_REQUEST_CREATE_TYPE_OPTIONS: ReadonlyArray<{
  value: Extract<BeChangeRequestType, "normal" | "standard" | "emergency">;
  label: string;
  description: string;
}> = [
  {
    value: "normal",
    label: "Normal",
    description:
      "Normal Changes are a general purpose change type that requires one or more approvals.",
  },
  {
    value: "standard",
    label: "Standard",
    description:
      "Preapproved, repeatable changes that follow an established template. These changes do not require approval.",
  },
  {
    value: "emergency",
    label: "Emergency",
    description:
      "Emergency Changes are a change type that must be implemented as soon as possible.",
  },
];

/**
 * The ServiceNow change-request Category choice list, in the legacy form's
 * order. `value` is the backend's `category` enum value. The form defaults to
 * {@link DEFAULT_CHANGE_REQUEST_CATEGORY} (ServiceNow's own default).
 */
export const CHANGE_REQUEST_CATEGORY_OPTIONS: ReadonlyArray<{
  value: BeChangeRequestCategory;
  label: string;
}> = [
  { value: "hardware", label: "Hardware" },
  { value: "software", label: "Software" },
  { value: "service", label: "Service" },
  { value: "system_software", label: "System Software" },
  { value: "applications_software", label: "Applications Software" },
  { value: "network", label: "Network" },
  { value: "telecom", label: "Telecom" },
  { value: "documentation", label: "Documentation" },
  { value: "other", label: "Other" },
  { value: "regular_release_cloud", label: "Regular Release - Cloud" },
  { value: "hotfix_release_cloud", label: "Hotfix Release - Cloud" },
  { value: "devops", label: "DevOps" },
  { value: "cloud_computing", label: "Cloud Computing" },
];

export const DEFAULT_CHANGE_REQUEST_CATEGORY: BeChangeRequestCategory = "other";

/** True when `value` is a category the backend accepts. */
export function isChangeRequestCategory(value: string | null | undefined): value is BeChangeRequestCategory {
  return CHANGE_REQUEST_CATEGORY_OPTIONS.some((o) => o.value === value);
}

/** Maximum length of the Additional comments / Work notes fields (ServiceNow journal limit). */
export const CHANGE_REQUEST_JOURNAL_MAX = 4000;

/**
 * Reads a category off a detail response: the enum value itself (Postgres data
 * source) or an entity ref whose `id` is the enum value.
 */
export function changeRequestCategoryValue(
  category: BeChangeRequestDetail["category"],
): BeChangeRequestCategory | "" {
  const id = typeof category === "string" ? category : category?.id;
  return isChangeRequestCategory(id) ? id : "";
}

/** Display label for a detail response's category ("—" when unset). */
export function changeRequestCategoryLabel(category: BeChangeRequestDetail["category"]): string {
  if (!category) return "—";
  const id = typeof category === "string" ? category : category.id;
  const known = CHANGE_REQUEST_CATEGORY_OPTIONS.find((o) => o.value === id);
  if (known) return known.label;
  if (typeof category === "string") return category || "—";
  return category.label ?? category.name ?? (category.id || "—");
}

/** True when `value` is one of the three types a change request can be created as. */
export function isCreatableChangeRequestType(value: string | null | undefined): boolean {
  return CHANGE_REQUEST_CREATE_TYPE_OPTIONS.some((o) => o.value === value);
}

/** True when `type` is Emergency (the backend's enum value "emergency"; case and padding ignored). */
export function isEmergencyChangeRequestType(type: string | null | undefined): boolean {
  return type?.trim().toLowerCase() === "emergency";
}

/**
 * The one line shown where an Emergency change's two customer boxes (Customer
 * Approval, Customer Review) sit disabled and unticked. An Emergency change acts
 * without customer consent: it is authorized by the CAB alone and never reaches
 * a customer state, so there is nothing for the customer to be asked.
 */
export const EMERGENCY_CUSTOMER_STEPS_HELPER = "Emergency changes proceed without customer approval or review.";

/** What the detail page reads where a customer step does not exist for the change. */
export const CUSTOMER_STEP_NOT_APPLICABLE = "Not applicable";

/**
 * Whether a customer step -- Customer Approval or Customer Review: the requirement
 * and what the customer confirmed -- is not applicable to this change: it is an
 * Emergency change, which acts without customer consent (the flow ignores its two
 * boxes), and nothing on the record shows it went through that gate. An Emergency
 * change raised before the rule can still sit in a customer state, carry the
 * customer's outcome or hold a stage row for the gate: then the step is shown as
 * it is, never hidden.
 *
 * `approvals` is `GET /change-requests/{id}/approvals`; while it is not loaded the
 * stage rows count for nothing, and the state and the customer's outcome speak.
 */
export function isCustomerStepNotApplicable(
  cr: Pick<BeChangeRequestDetail, "type" | "state" | "hasCustomerApproved" | "hasCustomerReviewed">,
  step: "approval" | "review",
  approvals?: readonly Pick<BeChangeRequestApproval, "stage">[],
): boolean {
  if (!isEmergencyChangeRequestType(cr.type)) return false;
  const stage = step === "approval" ? "Customer Approval" : "Customer Review";
  const gateState = step === "approval" ? "customer_approval" : "customer_review";
  const outcome = step === "approval" ? cr.hasCustomerApproved : cr.hasCustomerReviewed;
  return !(cr.state === gateState || outcome === true || approvals?.some((a) => approvalStageLabel(a.stage) === stage));
}

/**
 * Whether the signed-in user is the creator/requester of this change request.
 * The backend refuses approvals from the creator (Peer and CAB alike);
 * this lets the UI say so up front rather than offering a control that will
 * 403. Defensive on purpose: the detail only carries `requestedBy` (an entity
 * ref) and `createdBy` (a display string whose shape -- id, email or name --
 * the backend may change), so it compares each against the user's id and
 * email and never throws on a missing field. Returns false when nothing
 * matches or the user hasn't loaded, i.e. it never hides controls on a guess.
 */
export function isChangeRequestCreator(
  cr: { requestedBy?: { id?: string | null } | null; createdBy?: string | null },
  user: { id?: string | null; email?: string | null } | undefined,
): boolean {
  if (!user) return false;
  const id = user.id?.trim().toLowerCase();
  const email = user.email?.trim().toLowerCase();
  const candidates = [cr.requestedBy?.id, cr.createdBy]
    .map((c) => c?.trim().toLowerCase())
    .filter((c): c is string => !!c);
  return candidates.some((c) => (!!id && c === id) || (!!email && c === email));
}

// ---------------------------------------------------------------------------
// A time the customer proposed, and WSO2's answer to it
//
// The previous system's own mechanism, which our schema already carries: the customer's
// proposal is written to `customer_updated_on` (a planned START) and the change
// STAYS in Customer Approval, with the planned window untouched; WSO2 answers it
// with "Agree" (Accept proposed time: the change goes straight to Scheduled, no
// CAB, no new customer request) or "Disagree" (Propose a different time: the
// customer is asked again, no CAB; or Decline, keeping the current time). The
// backend derives the conversation (`customerProposal`) with an allowlist rule;
// the page only ever reads `answer === "pending"` from it, never infers it.
// ---------------------------------------------------------------------------

/** What the header says while a proposed time waits for WSO2: the change is waiting for WSO2, not for the customer. */
export const CUSTOMER_PROPOSAL_WAITING_REASON = "Waiting for WSO2 to respond to the customer's proposed time";

/**
 * The time stored on a change in Customer Approval that has not been answered, or `null`. Only the
 * backend's own verdict counts (`answer: "pending"`: the allowlist that keeps a
 * migrated change with a live CAB stage, a closed or a scheduled one out), and only
 * in Customer Approval, where the conversation takes place.
 *
 * It is a customer's PROPOSAL only when somebody is recorded as having proposed it
 * ({@link customerProposalProposer}); a stored time nobody is recorded as having proposed (written by
 * someone at WSO2, or left over from an earlier cycle) is returned too, so the page can say it is there,
 * but there is no proposal to answer: Accept is refused, and "Propose a different time" is a plain
 * Re-schedule that asks the customer to approve the time WSO2 names.
 */
export function pendingCustomerProposal(
  cr: Pick<BeChangeRequestDetail, "state" | "customerProposal">,
): BeChangeRequestCustomerProposal | null {
  return cr.state === "customer_approval" && cr.customerProposal?.answer === "pending" ? cr.customerProposal : null;
}

/** Who proposed the time and when, as far as the backend can still tell. */
export interface CustomerProposalProposer {
  name?: string;
  email?: string;
  /** When it was proposed (the change request's last write), as the backend sent it. */
  on?: string;
}

/**
 * The proposer of a stored time, or `null` when nobody is recorded as having proposed it. The backend
 * names one only when a registered contact of the project is on record as having written the time
 * (`proposerRecorded` says so); a date a user of the previous system wrote (WSO2 users do too), one
 * left over from an earlier cycle, or a sync rewrite is nobody to name, and the page must not guess:
 * it says nobody is recorded, and no answer is about it ({@link acceptProposedTimeBlockedReason}).
 */
export function customerProposalProposer(
  proposal: Pick<BeChangeRequestCustomerProposal, "proposerRecorded" | "proposedByName" | "proposedByEmail" | "proposedOn">,
): CustomerProposalProposer | null {
  // The backend's own verdict first: with `proposerRecorded: false` nobody is named, whatever else came with it.
  if (proposal.proposerRecorded === false) return null;
  const name = proposal.proposedByName?.trim() || undefined;
  const email = proposal.proposedByEmail?.trim() || undefined;
  if (!name && !email) return null;
  return { name, email, on: proposal.proposedOn?.trim() || undefined };
}

/** "Mia Member (mia@example.com)", the name alone, or the email alone. */
export function customerProposalProposerLabel(proposer: CustomerProposalProposer): string {
  if (proposer.name && proposer.email) return `${proposer.name} (${proposer.email})`;
  return proposer.name ?? proposer.email ?? "";
}

/**
 * The words the page uses for a stored time that waits for WSO2. They say "the customer" only when the
 * proposer is on record ({@link customerProposalProposer}): the previous system lets WSO2 users write the
 * date too, and one left over from an earlier round reads the same, so with nobody on record the page
 * must not claim the customer proposed it, and nothing in it is "waiting for your answer": there is no
 * proposal to answer.
 */
export interface CustomerProposalWording {
  /** The banner's title (its region name). */
  bannerTitle: string;
  /** The label of the stored window in the banner and the Accept dialog. */
  windowLabel: string;
  /** The Re-schedule dialog's lead for the stored window: "<lead>." with the window formatted by the caller. */
  counterLead: (window: string) => string;
  /** The counter dialog's note when the window typed in is the very one the customer proposed (only said of a recorded proposal). */
  isTheProposedTimeNote: string;
}

const CUSTOMER_PROPOSED_WORDING: CustomerProposalWording = {
  bannerTitle: "The customer proposed a new time",
  windowLabel: "Proposed by the customer",
  counterLead: (window) => `The customer proposed ${window}.`,
  isTheProposedTimeNote: "That is the time the customer proposed. Close this and use Accept proposed time instead.",
};

const STORED_TIME_WORDING: CustomerProposalWording = {
  bannerTitle: "A time is stored on this change request",
  windowLabel: "Stored time",
  counterLead: (window) => `${storedTimeSentence(window)} There is no proposal to decline.`,
  isTheProposedTimeNote: "",
};

/** "A time is stored (<window>) but nobody is recorded as having proposed it." */
export function storedTimeSentence(window: string): string {
  return `A time is stored (${window}) but nobody is recorded as having proposed it.`;
}

/** The page's words for a stored time: the customer-attributed ones only when `proposer` is on record. */
export function customerProposalWording(proposer: CustomerProposalProposer | null): CustomerProposalWording {
  return proposer ? CUSTOMER_PROPOSED_WORDING : STORED_TIME_WORDING;
}

/**
 * Why Accept is unavailable when nobody is recorded as having proposed the stored time: the words of the
 * backend's own refusal (`change_request_proposer_not_recorded`, and the `acceptBlockedReason` it sends with
 * `canAccept: false`), for a backend that sends none. No staff action stands in for the customer's own
 * answer, so a time no customer is recorded as having proposed is never accepted for them.
 */
export const PROPOSER_NOT_RECORDED_ACCEPT_REASON =
  'Nobody is recorded as having proposed this time (it may have been written by someone at WSO2 or left over from an earlier cycle), so it cannot be accepted: use "Propose a different time" to ask the customer to approve a time.';

/** What the banner adds for a stored time nobody is recorded as having proposed: what it may be, and what is still true. */
export const STORED_TIME_ADVICE =
  "It may have been written by someone at WSO2, be left over from an earlier cycle, or be a customer's time that this change request was edited after (only the last edit is on record), " +
  "so there is no proposal to accept. " +
  'The customer is still being asked to approve the planned time; use "Propose a different time" to ask them to approve another time, or this one.';

/**
 * The stable `errorCode`s of the refusals that mean "what this dialog showed is no longer what is
 * stored, or the answer cannot be given now": the one machine-readable thing the page may branch
 * on (the message is for people and can change). Each comes with a 409 from entity-service, through
 * the BFF, on the two answers to a time: Accept proposed time and the Re-schedule / counter PATCH.
 * A code this list does not name (a newer backend) is handled as no code at all.
 */
export const ChangeRequestErrorCode = {
  /** The planned window is no longer the one the page showed. */
  SCHEDULE_CHANGED: "change_request_schedule_changed",
  /** The change is not in Customer Approval, or nobody has been asked for the customer's approval. */
  NOT_PROPOSABLE: "change_request_not_proposable",
  /** Another approval, not the customer's, is being asked at the same time. */
  PROPOSAL_NOT_NOW: "change_request_proposal_not_now",
  /** There is no planned window to keep the length of. */
  NO_PLANNED_WINDOW: "change_request_no_planned_window",
  /** The change is on hold. */
  ON_HOLD: "change_request_on_hold",
  /** Nobody is recorded as having proposed the stored time, so it cannot be accepted. */
  PROPOSER_NOT_RECORDED: "change_request_proposer_not_recorded",
} as const;

const STALE_ANSWER_CODES: ReadonlySet<string> = new Set(Object.values(ChangeRequestErrorCode));

/** The `errorCode` the backend named on a failed request, if it named one (a `BackendApiError`'s payload). */
export function backendErrorCode(err: unknown): string | undefined {
  if (typeof err !== "object" || err === null) return undefined;
  const code = (err as { payload?: { errorCode?: unknown } }).payload?.errorCode;
  return typeof code === "string" && code ? code : undefined;
}

/**
 * Whether a failed answer to a time (Accept proposed time, or the Re-schedule / counter PATCH) was
 * refused because the change request is no longer what the dialog was opened on: a 409 that names one
 * of the {@link ChangeRequestErrorCode}s. Sending the same request again would be refused the same way,
 * so the page closes the dialog, says why and reads the change request again. Any other failure (a 400
 * about the window, a refusal with no code or an unknown one, a server error) is the dialog's own to
 * show: it stays open with its message.
 */
export function isStaleAnswerError(err: unknown): boolean {
  if (typeof err !== "object" || err === null) return false;
  if ((err as { status?: unknown }).status !== 409) return false;
  const code = backendErrorCode(err);
  return code !== undefined && STALE_ANSWER_CODES.has(code);
}

/** What an answer dialog was opened on, and what the page holds for the same change request. */
export interface AnswerSnapshot {
  cr: Pick<BeChangeRequestDetail, "state" | "plannedStartOn" | "plannedEndOn">;
  proposal: Pick<
    BeChangeRequestCustomerProposal,
    "startOn" | "proposerRecorded" | "proposedByName" | "proposedByEmail" | "proposedOn"
  > | null;
}

/**
 * Whether the page now holds something other than what an answer dialog was opened on: another state,
 * another planned window, a proposal that is gone, a different one, or one whose proposer is now (not)
 * on record. The dialog keeps what its reader was shown (what they answer is what they read), so after
 * a refused attempt this is what tells it to stop offering the same request again, whether or not the
 * backend named the refusal with a code.
 */
export function answerSnapshotMoved(shown: AnswerSnapshot, now: AnswerSnapshot): boolean {
  const instant = (value: string | null | undefined): number | null => parseBackendTimestamp(value)?.getTime() ?? null;
  if ((shown.cr.state ?? null) !== (now.cr.state ?? null)) return true;
  if (instant(shown.cr.plannedStartOn) !== instant(now.cr.plannedStartOn)) return true;
  if (instant(shown.cr.plannedEndOn) !== instant(now.cr.plannedEndOn)) return true;
  if (!shown.proposal || !now.proposal) return !!shown.proposal !== !!now.proposal;
  if (instant(shown.proposal.startOn) !== instant(now.proposal.startOn)) return true;
  return !customerProposalProposer(shown.proposal) !== !customerProposalProposer(now.proposal);
}

/** A window as two instants (epoch ms); `endMs` is `null` when the end is not known. */
export interface WindowMs {
  startMs: number;
  endMs: number | null;
}

/** The planned window of a change, or `null` when it has none to move (no start or end, or an end that is not after the start). */
export function plannedWindowMs(cr: Pick<BeChangeRequestDetail, "plannedStartOn" | "plannedEndOn">): { startMs: number; endMs: number } | null {
  const start = parseBackendTimestamp(cr.plannedStartOn);
  const end = parseBackendTimestamp(cr.plannedEndOn);
  if (!start || !end || end.getTime() <= start.getTime()) return null;
  return { startMs: start.getTime(), endMs: end.getTime() };
}

/**
 * The window the customer proposed: their start, and the end the backend derived (start +
 * the planned length: a customer proposes a START and the planned length is kept). When the
 * backend sent no end, it is derived here the same way; `null` when there is no planned
 * length to keep either, or the proposal's start cannot be read.
 */
export function proposedWindowMs(
  cr: Pick<BeChangeRequestDetail, "plannedStartOn" | "plannedEndOn">,
  proposal: Pick<BeChangeRequestCustomerProposal, "startOn" | "endOn">,
): WindowMs | null {
  const start = parseBackendTimestamp(proposal.startOn);
  if (!start) return null;
  const end = parseBackendTimestamp(proposal.endOn);
  if (end) return { startMs: start.getTime(), endMs: end.getTime() };
  const planned = plannedWindowMs(cr);
  return { startMs: start.getTime(), endMs: planned ? start.getTime() + (planned.endMs - planned.startMs) : null };
}

/** An instant as the CSM pages show a change request's times (the viewer's time zone, "Mar 1, 2030, 9:00 AM"). */
export function formatCrDateTime(value: string | number | null | undefined): string {
  const raw = typeof value === "number" ? (Number.isFinite(value) ? new Date(value).toISOString() : null) : value;
  return formatBackendTimestampForDisplay(raw, { dateStyle: "medium", timeStyle: "short" }) ?? "—";
}

/** A window as "<start> to <end>", or the one bound there is, or "—". */
export function formatCrWindow(start: string | number | null | undefined, end: string | number | null | undefined): string {
  const from = formatCrDateTime(start);
  const to = formatCrDateTime(end);
  if (from === "—") return to;
  return to === "—" ? from : `${from} to ${to}`;
}

/** A window's length as words ("2 hours", "1 hour 30 minutes", "1 day"), or "" when it has none. */
export function formatWindowLength(ms: number): string {
  const totalMinutes = Math.round(ms / 60_000);
  if (!Number.isFinite(totalMinutes) || totalMinutes <= 0) return "";
  const days = Math.floor(totalMinutes / 1440);
  const hours = Math.floor((totalMinutes % 1440) / 60);
  const minutes = totalMinutes % 60;
  const unit = (n: number, word: string): string => `${n} ${word}${n === 1 ? "" : "s"}`;
  return [days && unit(days, "day"), hours && unit(hours, "hour"), minutes && unit(minutes, "minute")]
    .filter((part): part is string => !!part)
    .join(" ");
}

/**
 * Why "Accept proposed time" is unavailable, or `null` when it is on offer. The backend is the
 * authority and refuses each of these in words (409 / 400); the page says so up front where it
 * can know. First, nobody being recorded as having proposed the time: no staff action stands in for
 * the customer's answer, so such a time is never accepted (the backend's own words when it sends
 * them, else {@link PROPOSER_NOT_RECORDED_ACCEPT_REASON}). Then the change being on hold (a state change
 * is refused), the proposed time having already passed (it was valid when made; accepting it
 * would schedule the past), or there being no planned window whose length the proposal could keep. The
 * page's own reading comes first (the proposed time can pass while the page is open, and its words are
 * short); the backend's verdict (`canAccept: false`, with its own `acceptBlockedReason`) holds the
 * button back for anything the page cannot tell.
 */
export function acceptProposedTimeBlockedReason(
  cr: Pick<BeChangeRequestDetail, "onHold" | "plannedStartOn" | "plannedEndOn">,
  proposal: Pick<
    BeChangeRequestCustomerProposal,
    "startOn" | "canAccept" | "acceptBlockedReason" | "proposerRecorded" | "proposedByName" | "proposedByEmail" | "proposedOn"
  >,
  nowMs: number = Date.now(),
): string | null {
  const said = proposal.acceptBlockedReason?.trim();
  const sentence = (text: string): string => `${text.charAt(0).toUpperCase()}${text.slice(1)}${/[.!?]$/.test(text) ? "" : "."}`;
  if (!customerProposalProposer(proposal)) return said ? sentence(said) : PROPOSER_NOT_RECORDED_ACCEPT_REASON;
  if (cr.onHold === true) return "This change request is on hold. Take it off hold first.";
  const start = parseBackendTimestamp(proposal.startOn);
  if (start && start.getTime() <= nowMs) return "The proposed time has passed. Propose a different time.";
  if (!plannedWindowMs(cr)) return "This change request has no planned window whose length the proposed time could keep. Propose a different time.";
  if (proposal.canAccept === false) {
    return said ? sentence(said) : "The backend would refuse this right now. Propose a different time.";
  }
  return null;
}

/** The states a change request is in once it has moved on from Customer Approval (Scheduled, then every state after it). */
const STATES_PAST_CUSTOMER_APPROVAL: readonly string[] = ["scheduled", "implement", "review", "customer_review", "rollback", "closed", "canceled"];

/**
 * What the Overview's "Customer approved" cell reads. The customer's own approval (`hasCustomerApproved`,
 * stamped when they answer in the Customer Portal) reads Yes. A change that went to Scheduled because WSO2
 * ACCEPTED the time the customer proposed was never stamped (no staff action records the customer's
 * approval: the proposal is the customer's own consent), so a plain "No" there would be misleading: it
 * reads "Proposed time accepted". Display only; nothing reads this to decide anything.
 *
 * It reads that only once the change has moved on from Customer Approval (Scheduled or later): the answer
 * (`agreed` / the raw `agree`) stays on the row after it, nothing clears it when the customers are asked
 * again, so a change that is (back) in Customer Approval with an Agree standing was NOT scheduled by it and
 * is waiting for the customer's own answer: "Proposed time accepted" there would say there is nothing left
 * to answer. The same gate is in the CSM microapp's model (`toChangeRequestDetail`).
 */
export function customerApprovedDisplay(
  cr: Pick<BeChangeRequestDetail, "state" | "hasCustomerApproved" | "customerProposal" | "confirmCustomerUpdatedDate">,
): "Yes" | "No" | "Proposed time accepted" {
  if (cr.hasCustomerApproved) return "Yes";
  const agreed = cr.customerProposal?.answer === "agreed" || cr.confirmCustomerUpdatedDate?.trim().toLowerCase() === "agree";
  return agreed && !!cr.state && STATES_PAST_CUSTOMER_APPROVAL.includes(cr.state) ? "Proposed time accepted" : "No";
}

/** Stage-level statuses that mean the stage is actively waiting on someone. */
const WAITING_APPROVAL_STATUSES = new Set(["PENDING", "REQUESTED"]);

/** Approver-level statuses that mean the approver is no longer being asked. */
const NO_LONGER_ASKED_APPROVER_STATUSES = new Set(["CANCELLED", "CANCELED", "NOT_REQUIRED"]);

/**
 * Plain-language reason a change request isn't moving on its own right now,
 * derived from its approval stages (`GET /change-requests/{id}/approvals`) —
 * the same data `ChangeRequestApprovals` renders. Names the first stage still
 * waiting on someone (in stage order, not necessarily severity order): e.g.
 * "Awaiting Authorize approval" or, when the stage carries a named approver
 * group, "Awaiting Devops Approval". Returns `null` when nothing is currently
 * blocking on approval — no waiting stage, or the approvals haven't loaded
 * yet — so callers should treat `null` as "no reason to show", not an error.
 *
 * `pendingProposal` (see {@link pendingCustomerProposal}) is a customer's proposed time that nobody
 * at WSO2 has answered: at Customer Approval the change is then waiting for WSO2, not for
 * the customer ({@link CUSTOMER_PROPOSAL_WAITING_REASON}). A stored time that nobody is recorded as
 * having proposed is not that: there is no proposal to answer, the customer is still being asked to
 * approve the planned time, and the header says so ("Awaiting Customer Approval").
 */
export function changeRequestBlockingReason(
  approvals: BeChangeRequestApproval[] | undefined,
  state?: string | null,
  pendingProposal?: BeChangeRequestCustomerProposal | null,
): string | null {
  // The customer gates are named from the state: the CR is waiting on the
  // customer's own answer (given in the Customer Portal) whether or not the
  // backend provisioned a "Customer Approval" / "Customer Review" stage for
  // the customer group. Same wording the stage label gives, never doubled.
  // The one exception is a proposed time nobody at WSO2 has answered: the change
  // stays in Customer Approval, but what it is waiting for is WSO2.
  if (state === "customer_approval") {
    if (!pendingProposal || !customerProposalProposer(pendingProposal)) return "Awaiting Customer Approval";
    return CUSTOMER_PROPOSAL_WAITING_REASON;
  }
  if (state === "customer_review") return "Awaiting Customer Review";
  // A stage whose every approver was cancelled or marked not required (a
  // superseded customer stage after a Re-schedule, a group change) has nobody
  // left to answer, so it is not what the change is waiting on -- even though
  // the backend reports such a stage as PENDING (nothing was approved or
  // rejected on it).
  const waiting = approvals?.find(
    (a) =>
      WAITING_APPROVAL_STATUSES.has(a.status.trim().toUpperCase()) &&
      (a.approvers.length === 0 ||
        a.approvers.some((p) => !NO_LONGER_ASKED_APPROVER_STATUSES.has(p.status.trim().toUpperCase()))),
  );
  if (!waiting) return null;
  // A recognised stage (Peer / CAB) is named by its stage label, which
  // already ends in "Approval" -- so this reads "Awaiting CAB Approval" and
  // never "Awaiting CAB approval approval".
  const stageLabel = knownApprovalStageLabel(waiting.stage);
  if (stageLabel) return `Awaiting ${stageLabel}`;
  const who = waiting.approverName?.trim() || waiting.stage;
  // Approver-group names sometimes already say "Approval" ("Devops
  // Approval"); avoid a doubled "approval approval" in that case.
  return /approval/i.test(who) ? `Awaiting ${who}` : `Awaiting ${who} approval`;
}

/**
 * Why nobody is being asked, for a project with no registered contacts at all:
 * what the Customer Project holds, and that it cannot be changed to route the
 * step (it is fixed once approval is requested).
 */
export const NO_CUSTOMER_CONTACTS_HELPER =
  "No registered customer contacts are assigned to this change request's project, so no customer approvers were assigned. " +
  "The Customer Project is fixed once approval is requested, so it cannot be changed to route the step.";

/**
 * Why nobody is being asked when the project does have registered contacts but
 * none of them has a request waiting. The web cannot tell which of the causes
 * applies, so it names them rather than one: the request goes to the project's
 * registered contacts leaving out whoever raised the change and anyone no longer
 * active (so a project whose only contact is the requester, or whose contacts
 * were all deactivated, asks nobody), and a change migrated from the previous system
 * sitting at the step can have had no request at all.
 */
export const NOBODY_ASKED_HELPER =
  "Nobody is being asked to answer at this step. The request goes to the Customer Project's registered contacts, " +
  "leaving out whoever raised the change and anyone no longer active, and none of them has one waiting; " +
  "a change migrated from the previous system may also have no request at all.";

/**
 * What staff are left with, per customer gate, when nobody is being asked.
 * Staff never record a customer's approval or review, so the only exits are the
 * ones staff always have there. Out of Customer Approval that is Cancel change:
 * Re-schedule asks the project's registered contacts again at once, and is refused
 * while nobody can be asked, so it ends no wait. Out of Customer Review it is
 * Roll back or Cancel change.
 */
export const NOBODY_ASKED_WAY_OUT: Readonly<Record<"customer_approval" | "customer_review", string>> = {
  customer_approval:
    "Staff never record a customer's approval, so there is nobody to answer here: Cancel change is the only way out. " +
    "Re-schedule would ask the same group again, and it is refused while nobody can be asked.",
  customer_review:
    "Staff never record a customer's review, so there is nobody to answer here: Roll back or Cancel change are the only ways out.",
};

/**
 * The same for Customer Approval when the project DOES have registered contacts
 * and none has a request waiting. The web cannot tell whether anybody can be
 * asked this time (the requester alone and deactivated contacts leave nobody; a
 * legacy change that reached the gate with no request, on a project with
 * eligible contacts, does get its question put by a Re-schedule), so it says what
 * Re-schedule does and when Cancel change is the only way out, rather than
 * claiming either.
 */
export const NOBODY_ASKED_WAY_OUT_RESCHEDULE_MAY_HELP =
  "Staff never record a customer's approval, so there is nobody to answer here. " +
  "Re-schedule asks the project's registered contacts again at once, which helps only if someone can be asked this time " +
  "(it is refused when nobody can be); if nobody can, Cancel change is the only way out.";

/**
 * Whether any approver of any stage is still being asked: an approver row in
 * the `REQUESTED` state, the same test the backend and `pendingCustomerReview`
 * use (never the stage's own status, which stays `PENDING` after every approver
 * was cancelled). `null` while the approvals are not known (not loaded, or being
 * reloaded).
 *
 * Any stage counts, not only one labelled as a customer stage: a stage the
 * sync brought over is labelled by its position, so the customer's request on a
 * migrated change can carry any label, and a note that says nobody is asked
 * must never be wrong about a change somebody is asked about. At a customer
 * gate every internal stage is settled (leaving a state cancels what was left
 * waiting in it), so a row still waiting there is the customer's.
 */
export function anyApproverBeingAsked(approvals: readonly BeChangeRequestApproval[] | null | undefined): boolean | null {
  if (!approvals) return null;
  return approvals.some((stage) => stage.approvers.some((a) => a.status.trim().toUpperCase() === "REQUESTED"));
}

/**
 * The note shown in the Approval tab when a change sits at a customer gate
 * (Customer Approval / Customer Review) and nobody is being asked to answer, so
 * nobody can: staff never record a customer's approval or review, and the
 * customer's own answer has no one to come from. `null` in every other case.
 *
 * Nobody is asked when:
 *  - the project has no registered contacts (`customerContacts` empty): the
 *    project-specific reason, shown even before the approvals load, since the
 *    contacts alone prove it unless an old request is still waiting;
 *  - the project has contacts, but the approvals (loaded) show no approver
 *    waiting: only the requester is registered, the contacts are deactivated,
 *    or the change reached the gate with no request at all (a legacy change
 *    with no stage).
 * It is `null`, never a guess, while the approvals are unknown (`undefined`),
 * when somebody IS waiting (an old request still stands: only the approvers
 * already asked may answer it), and when `customerContacts` is `undefined`
 * (absent from the payload: another data source, nothing is claimed).
 */
export function noCustomerAskedHelper(
  state: string | null | undefined,
  customerContacts: readonly unknown[] | null | undefined,
  approvals?: readonly BeChangeRequestApproval[] | null,
): string | null {
  if (state !== "customer_approval" && state !== "customer_review") return null;
  if (customerContacts === undefined) return null;
  const asked = anyApproverBeingAsked(approvals);
  if (asked === true) return null;
  if (!customerContacts || customerContacts.length === 0) return `${NO_CUSTOMER_CONTACTS_HELPER} ${NOBODY_ASKED_WAY_OUT[state]}`;
  if (asked !== false) return null;
  return `${NOBODY_ASKED_HELPER} ${state === "customer_approval" ? NOBODY_ASKED_WAY_OUT_RESCHEDULE_MAY_HELP : NOBODY_ASKED_WAY_OUT.customer_review}`;
}

// ---------------------------------------------------------------------------
// What can still be edited about the customer's part of a change request
//
// The rule is a pure function of (state, the stored value, whether the change
// request has a Customer Project), so this mirrors the backend EXACTLY, up front,
// instead of waiting for its 400 (the backend stays the authority and still
// answers one; its own table test and the table in `__tests__/changeRequests.test.ts`
// here carry the same rows, under the same ids):
//
//   - CREATION PHASE is the state New (before Request Approval). The Customer
//     Project (which fixes the Customer Group, derived read-only from that
//     project's registered contacts) and both tick boxes are fully editable in it.
//   - The moment the change request leaves New the Customer Project is FROZEN in
//     every later state, for everyone. A correction is Cancel + Clone.
//   - After New a tick box is ADD-ONLY: an unticked one may be ticked until the
//     gate it controls is passed (Customer Approval: while the change request is
//     New / Assess / Authorize, i.e. before it can reach Customer Approval;
//     Customer Review: until it reaches Customer Review), and needs a Customer
//     Project to already be set (it can no longer be set then); a ticked one can
//     never be unticked.
//   - Deployments and deployment products keep their own rule: editable until
//     Implement, and always within the frozen project.
//
// A change request cannot return to New, so none of this needs a record of what
// it reached. A ticked box stays ticked, so nothing can untick the customer's
// part to skip it: a Re-schedule asks the same contacts again.
// ---------------------------------------------------------------------------

/** True while a change request is being created: state New, or none recorded yet. */
export function isChangeRequestCreationPhase(state?: string | null): boolean {
  return !state || state === "new";
}

/** Why the Customer Project cannot be changed once approval was requested. */
export const CUSTOMER_PROJECT_FROZEN_REASON =
  "Fixed when approval was requested. Cancel and clone to change it.";

/** Why a ticked customer requirement is read-only after New. */
export const CUSTOMER_REQUIREMENT_ADD_ONLY_REASON =
  "Once approval has been requested a customer requirement can be added but never removed.";

/** Why an unticked customer requirement cannot be added after New: no project, and none can be set. */
export const CUSTOMER_REQUIREMENT_NEEDS_PROJECT_REASON =
  "Needs a Customer Project, which can no longer be set. Cancel and clone.";

/** The helper under a customer requirement that may still be added, but not removed. */
export const CUSTOMER_REQUIREMENT_ONCE_SAVED_HELPER = "Once saved this can't be removed.";

/** Why the Customer Project is read-only in `state`, or `null` while it is editable (state New). */
export function customerProjectLockedReason(state?: string | null): string | null {
  return isChangeRequestCreationPhase(state) ? null : CUSTOMER_PROJECT_FROZEN_REASON;
}

/**
 * States from which the "Customer Approval" checkbox can no longer be turned ON:
 * the gate it controls (between internal approval and scheduling) is either
 * being worked (`customer_approval`) or already behind the CR (`scheduled` and
 * everything after it, including the off-ramps). Mirrors the backend's
 * `approvalRequirementEditable`.
 */
const CUSTOMER_APPROVAL_LOCKED_STATES: readonly string[] = [
  "customer_approval",
  "scheduled",
  "implement",
  "review",
  "customer_review",
  "closed",
  "rollback",
  "canceled",
];

/** States from which the "Customer Review" checkbox can no longer be turned ON (`reviewRequirementEditable`). */
const CUSTOMER_REVIEW_LOCKED_STATES: readonly string[] = [
  "customer_review",
  "closed",
  "rollback",
  "canceled",
];

/** What a customer requirement's rule depends on besides the state. */
export interface CustomerRequirementContext {
  /** The tick box as stored (`customerApprovalRequired` / `customerReviewRequired`). */
  stored: boolean;
  /** Whether the change request has a Customer Project. */
  hasProject: boolean;
}

function customerRequirementLockedReason(
  gateLockedStates: readonly string[],
  gateWord: "approval" | "review",
  state: string | null | undefined,
  { stored, hasProject }: CustomerRequirementContext,
): string | null {
  if (isChangeRequestCreationPhase(state)) return null;
  // Add-only: a requirement that was added stays, in every state after New.
  if (stored) return CUSTOMER_REQUIREMENT_ADD_ONLY_REASON;
  // Adding one is possible only until its gate is passed...
  if (state && gateLockedStates.includes(state)) {
    return `Locked: the change request has already reached the customer ${gateWord} step or later.`;
  }
  // ...and only with a Customer Project to ask (it can no longer be set).
  if (!hasProject) return CUSTOMER_REQUIREMENT_NEEDS_PROJECT_REASON;
  return null;
}

/**
 * Why the Customer Approval checkbox cannot be changed in `state`, or `null` when
 * it can (in New either way; after New only to tick an unticked one).
 */
export function customerApprovalLockedReason(
  state: string | null | undefined,
  context: CustomerRequirementContext,
): string | null {
  return customerRequirementLockedReason(CUSTOMER_APPROVAL_LOCKED_STATES, "approval", state, context);
}

/** Why the Customer Review checkbox cannot be changed in `state`, or `null` when it can. */
export function customerReviewLockedReason(
  state: string | null | undefined,
  context: CustomerRequirementContext,
): string | null {
  return customerRequirementLockedReason(CUSTOMER_REVIEW_LOCKED_STATES, "review", state, context);
}

/**
 * The warning shown under a customer requirement that can still be ticked after
 * New (it can be added but never removed), or `null` where nothing is final yet:
 * in New, or when the box is ticked already (it then has its own reason).
 */
export function customerRequirementOnceSavedHelper(
  state: string | null | undefined,
  stored: boolean,
): string | null {
  return isChangeRequestCreationPhase(state) || stored ? null : CUSTOMER_REQUIREMENT_ONCE_SAVED_HELPER;
}

/** Why Request Approval is blocked for want of a Customer Project, or `null`. */
export const REQUEST_APPROVAL_NEEDS_PROJECT_REASON = "Select a Customer Project before requesting approval";

/**
 * Request Approval (New -> Assess) is refused by the backend when the customer's
 * approval and/or review is required but the change request has no Customer
 * Project: it would reach a customer stage with nobody to ask, and the project can
 * no longer be set once it has left New. Offered up front as the reason the action
 * is disabled.
 *
 * Never for an Emergency change: the backend ignores its two customer boxes (it is
 * authorized by the CAB alone and never reaches a customer state), so a stored
 * tick on one neither needs a project nor blocks the request.
 */
export function requestApprovalNeedsProjectReason(
  cr: Pick<BeChangeRequestDetail, "state" | "type" | "project" | "customerApprovalRequired" | "customerReviewRequired">,
): string | null {
  // Only the move out of New: a resent {state: "assess"} on a change request that
  // is already past it is the backend's idempotent no-op, not a request.
  if (!isChangeRequestCreationPhase(cr.state)) return null;
  if (isEmergencyChangeRequestType(cr.type)) return null;
  const needsCustomer = cr.customerApprovalRequired === true || cr.customerReviewRequired === true;
  return needsCustomer && !cr.project?.id ? REQUEST_APPROVAL_NEEDS_PROJECT_REASON : null;
}

/** Why Request Approval is blocked for want of anybody to ask on the Customer Project, or `null`. */
export const REQUEST_APPROVAL_NEEDS_CONTACT_REASON = "Register a contact for the Customer Project before requesting approval";

/**
 * Request Approval (New -> Assess) is refused by the backend when the customer's
 * approval and/or review is required and nobody on the Customer Project can be
 * asked: the change would reach Customer Approval / Customer Review with nobody to
 * answer, and with no way for staff to answer for the customer it could only be
 * cancelled (or rolled back from Review). The backend asks the project's
 * registered contacts leaving out whoever raised the change and anyone no longer
 * active, and it answers that refusal in words.
 *
 * The page knows part of that: when the change's own `customerContacts` is an
 * EMPTY list (the project has no registered contact at all, or none still active)
 * the refusal is certain, so the action is offered disabled with the reason, like
 * the missing-project one. It does not guess the rest (a project whose only
 * contact is the requester, or a contact with no sign-in yet): there the request
 * goes out and the backend's own message shows in the page's error banner, as for
 * any other refusal. It never claims anything while `customerContacts` is
 * `undefined` (not in the payload: another data source). With no project the
 * missing-project reason (`requestApprovalNeedsProjectReason`) applies instead.
 *
 * Never for an Emergency change, whose stored customer boxes the backend ignores
 * (see `requestApprovalNeedsProjectReason`): nobody is asked, so nobody is missing.
 */
export function requestApprovalNeedsContactReason(
  cr: Pick<
    BeChangeRequestDetail,
    "state" | "type" | "project" | "customerApprovalRequired" | "customerReviewRequired" | "customerContacts"
  >,
): string | null {
  if (!isChangeRequestCreationPhase(cr.state)) return null;
  if (isEmergencyChangeRequestType(cr.type)) return null;
  const needsCustomer = cr.customerApprovalRequired === true || cr.customerReviewRequired === true;
  if (!needsCustomer || !cr.project?.id) return null;
  return cr.customerContacts !== undefined && cr.customerContacts !== null && cr.customerContacts.length === 0
    ? REQUEST_APPROVAL_NEEDS_CONTACT_REASON
    : null;
}

/**
 * States from which the deployments / deployment products can no longer be
 * changed (the backend refuses with a 400 from `implement` onward). The Customer
 * Project is frozen far earlier (see `customerProjectLockedReason`).
 */
const SCOPE_LOCKED_STATES: readonly string[] = [
  "implement",
  "review",
  "customer_review",
  "closed",
  "rollback",
  "canceled",
];

/** Why the deployments (and their products) are locked in `state`, or `null` when editable. */
export function changeRequestScopeLockedReason(state?: string | null): string | null {
  return state && SCOPE_LOCKED_STATES.includes(state)
    ? "Locked: the deployments can't be changed once implementation has started."
    : null;
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
 * action being taken ("Request Approval", "Mark implemented"), not as the destination,
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
  // New -> Assess is the "Request Approval" action: it sends the CR into its
  // approval flow (Peer -> CAB for Normal, a single CAB stage for Emergency,
  // straight to Scheduled for Standard -- all the backend's call).
  assess: "Request Approval",
  // There is deliberately no entry for `scheduled`: a CR is moved to
  // Scheduled automatically when its approval is granted (the customer's own
  // answer, for a change that needs one), never by a manual "Schedule" action.
  implement: "Start implementation",
  review: "Mark implemented",
  customer_review: "Send for customer review",
  // Plain "Close" is the move out of Review when no customer review is
  // required. Out of `customer_review` there is none: that Close is the
  // customer's own answer.
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
export function changeRequestTransitionLabel(
  target: string,
  fromState?: string | null,
  customerProposalPending = false,
): string {
  // `authorize` is only ever an action from `customer_approval`, and it does not
  // move the state (the wire name of the Time Change loop): with a customer's
  // proposal waiting it is WSO2's counter ("Propose a different time", or a decline
  // that keeps the current time), otherwise a plain Re-schedule that asks the customer
  // again.
  if (target === "authorize" && fromState === "customer_approval") {
    return customerProposalPending ? "Propose a different time" : "Re-schedule";
  }
  return TRANSITION_LABEL[target] ?? sentenceCase(target);
}

/** True when this transition is destructive: menu-only, error-coloured. */
export function isDestructiveChangeRequestTransition(target: string): boolean {
  return DESTRUCTIVE_TRANSITIONS.includes(target);
}

/**
 * True when moving to `target` must not happen without a stated reason: the
 * destructive off-ramps (Roll back, Cancel change). The reason is recorded as
 * an ordinary internal work note on the change request *before* the state is
 * patched -- the PATCH contract has no reason or comment field of its own. See
 * `ChangeRequestTransitionReasonDialog`.
 */
export function changeRequestTransitionRequiresReason(target: string): boolean {
  return isDestructiveChangeRequestTransition(target);
}

/**
 * The customer's review of a change sitting in Customer Review, while it is
 * still waiting for an answer: the "Customer Review" stage the change's
 * customer gate provisioned, with at least one customer contact still being
 * asked. `contactNames` are those contacts' names (non-empty ones,
 * de-duplicated, in stage order).
 */
export interface PendingCustomerReview {
  contactNames: string[];
  /**
   * How many contacts are being asked: every approver still `REQUESTED`, the
   * nameless ones and the ones that share a name with another included, so it
   * can exceed `contactNames.length` (the backend sends an empty name for a
   * user without one). Absent reads as `contactNames.length`.
   */
  askedCount?: number;
}

/**
 * The customer review currently pending for a change in Customer Review,
 * derived from its approval stages (`GET /change-requests/{id}/approvals`), or
 * `null` when `state` is not Customer Review, the approvals have not loaded, or
 * nobody is being asked (no registered contacts, or the request was
 * superseded). "Being asked" is the approver-level `REQUESTED` status -- the
 * same test the backend's refusal uses -- never the stage's own status, which
 * stays `PENDING` after every approver was cancelled.
 *
 * Only the review is read: it is what the action bar needs, to hold Roll back
 * (a failed review is the customer's rejection, which they give in the
 * Customer Portal). There is nothing to read for Customer Approval, where no
 * staff action waits on the customer's answer.
 */
export function pendingCustomerReview(
  approvals: BeChangeRequestApproval[] | null | undefined,
  state?: string | null,
): PendingCustomerReview | null {
  if (state !== "customer_review" || !approvals) return null;
  const names: string[] = [];
  let askedCount = 0;
  for (const stage of approvals) {
    if (knownApprovalStageLabel(stage.stage) !== "Customer Review") continue;
    for (const approver of stage.approvers) {
      if (approver.status.trim().toUpperCase() !== "REQUESTED") continue;
      askedCount += 1;
      const name = approver.name?.trim();
      if (name && !names.includes(name)) names.push(name);
    }
  }
  return askedCount > 0 ? { contactNames: names, askedCount } : null;
}

/** Most contact names spelled out in the pending-review reason before "and N more". */
const MAX_PENDING_CONTACT_NAMES = 3;

/**
 * The targets the backend leaves out of `legalNextStates` while the customer's
 * review is live in `state`: `rollback`, because a failed review is the
 * customer's rejection, which they give in the Customer Portal. Cancel stays.
 * Empty for any other state, Customer Approval included: Re-schedule and Cancel
 * change are both always on offer there.
 */
export function customerGateWithheldTargets(state?: string | null): string[] {
  return state === "customer_review" ? ["rollback"] : [];
}

/**
 * Why Roll back is unavailable while `pending` is waiting for the customer's
 * review, or `null` when nothing is pending. The backend refuses a manual
 * `rollback` out of Customer Review while the customer group's review is live;
 * the customer gives a failed review in the Customer Portal.
 */
export function rollbackPendingReviewReason(pending: PendingCustomerReview | null | undefined): string | null {
  if (!pending) return null;
  const shown = pending.contactNames.slice(0, MAX_PENDING_CONTACT_NAMES);
  if (shown.length === 0) {
    return "Customer review is pending. A failed review is the customer's to give in the Customer Portal, so the change can't be rolled back from here.";
  }
  // Everyone asked counts, named or not: "A and 2 more" when two more people
  // (nameless, or sharing a name already shown) are being asked too.
  const more = Math.max(pending.askedCount ?? 0, pending.contactNames.length) - shown.length;
  const who = more > 0 ? `${shown.join(", ")} and ${more} more` : shown.join(", ");
  return `Customer review is pending from ${who}. A failed review is theirs to give in the Customer Portal, so the change can't be rolled back from here.`;
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
  /** Selected project ids — sent as the named `projectIds` payload field
   * (see `buildChangeRequestSearchFilters`), matching the entity-service's
   * own field name for this search. */
  projectIds: string[];
}

export const DEFAULT_CR_FILTERS: ChangeRequestFilters = {
  search: "",
  states: [],
  impacts: [],
  closedStartDate: "",
  closedEndDate: "",
  sreTeamIds: [],
  projectIds: [],
};

/** Count non-search active filters (used for the badge on the Filters button). */
export function countActiveCRFilters(filters: ChangeRequestFilters): number {
  return (
    (filters.states.length > 0 ? 1 : 0) +
    (filters.impacts.length > 0 ? 1 : 0) +
    (filters.closedStartDate ? 1 : 0) +
    (filters.closedEndDate ? 1 : 0) +
    (filters.sreTeamIds.length > 0 ? 1 : 0) +
    (filters.projectIds.length > 0 ? 1 : 0)
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
    ...(filters.projectIds.length > 0 && { projectIds: filters.projectIds }),
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
//   - `riskImpactAnalysis` is write-only for the same reason.
//     (`implementationPlan` was write-only too, but the read side now
//     returns it — see `BeChangeRequestDetail.implementationPlan` — so it's
//     no longer part of this gap. It isn't wired into the clone fields
//     below yet, though: that's a separate feature decision, not a gap.)
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
  /** The source's "Customer Approval" / "Customer Review" checkbox settings.
   * These are configuration of the flow (which steps the change goes through),
   * not an approval outcome, so a clone carries them; the customer's actual
   * confirmation (`hasCustomerApproved`/`hasCustomerReviewed`) is never copied. */
  customerApprovalRequired?: boolean;
  customerReviewRequired?: boolean;
  /** The source's Customer Project and Category, with the display label the
   * form shows until fresh lookups resolve it. The source's Deployments /
   * Deployment products are deliberately NOT carried: they name the
   * deployment the change targets, and a clone exists to promote the change
   * to a different one. The Customer Group is never carried either: it is
   * derived from the project's registered contacts (read-only). */
  projectId?: string;
  projectLabel?: string;
  category?: BeChangeRequestCategory;
}

/** Rich-text field carried into the clone form only when it has real content. */
function cloneableHtml(html?: string | null): string | undefined {
  if (!html || isBlankHtml(html)) return undefined;
  return sanitizeRichTextHtml(html);
}

/**
 * Builds the router-state payload for a change request's "Clone" action.
 * Deliberately omits: deployments / deployment products,
 * state, approval fields
 * (`hasCustomerApproved`/`hasCustomerReviewed`/`approvedBy`/`approvedOn`; the
 * `customerApprovalRequired`/`customerReviewRequired` settings ARE carried),
 * planned start/end, and every auto-numbered/timestamp/created-by field —
 * per this feature's requirement that promoting a change to a new
 * deployment must never silently carry an approval or a stale schedule
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
    customerApprovalRequired: cr.customerApprovalRequired ?? undefined,
    customerReviewRequired: cr.customerReviewRequired ?? undefined,
    projectId: cr.project?.id || undefined,
    projectLabel: cr.project?.name || undefined,
    category: changeRequestCategoryValue(cr.category) || undefined,
  };
}

/**
 * User-facing disclosure shown on the create form when it was opened via
 * Clone, so the field gap documented above is visible rather than silently
 * dropped. Kept as a single shared string so the detail page (if it ever
 * wants a preview) and the create page stay in sync.
 */
export const CLONE_SOURCE_GAP_MESSAGE =
  "Copied the subject, description, justification, test plan, type, impact, assigned engineer, customer project, category, and customer approval/review settings. " +
  "Priority, implementation plan, risk/impact analysis, backout plan, assignment group, " +
  "linked case, and affected product aren't available to copy and need to be re-entered. " +
  "Deployments, deployment products, schedule, and approval fields are intentionally left blank for you to set for the new environment. The Customer Group follows the customer project.";

// ---------------------------------------------------------------------------
// "Originating service request" picker — unified parent-record search
//
// The create form's picker searches both service requests (by CS number) and
// incidents (by INC number) — see `useSearchParentRecordsForSelect`. But the
// live `PATCH /change-requests/{id}` write (`ChangeRequestUtils.
// patchChangeRequestFields` in the shared ServiceNow scoped app) only ever
// resolves `caseId` against the `sn_customerservice_case` table — passing an
// incident's sys_id through that same call 404s. Until the backend adds a
// path for a change request to link directly to an incident, an incident
// result can be *found* by this picker (so the UI is ready the moment that
// ships) but must never be *submitted* — see `CreateChangeRequestPage`'s
// `isIncidentParentSelected` gate.
// ---------------------------------------------------------------------------

export type ParentRecordKind = "service_request" | "incident";

/** A single option in the unified service-request/incident picker. */
export interface ParentRecordOption {
  kind: ParentRecordKind;
  id: string;
  number?: string | null;
  subject?: string | null;
}

const PARENT_RECORD_VALUE_PREFIX: Record<ParentRecordKind, string> = {
  service_request: "sr:",
  incident: "inc:",
};

/**
 * Encodes a `ParentRecordOption`'s kind and id into the single string id
 * `AsyncEntitySelect` (and this page's `caseId` state) works with — the kind
 * has to travel with the id since a plain incident id and a plain case id are
 * both opaque UUIDs the form otherwise can't tell apart.
 */
export function encodeParentRecordValue(kind: ParentRecordKind, id: string): string {
  return `${PARENT_RECORD_VALUE_PREFIX[kind]}${id}`;
}

/** Reverses {@link encodeParentRecordValue}; `undefined` for an empty/unrecognized value. */
export function decodeParentRecordValue(
  value: string,
): { kind: ParentRecordKind; id: string } | undefined {
  if (value.startsWith(PARENT_RECORD_VALUE_PREFIX.service_request)) {
    return {
      kind: "service_request",
      id: value.slice(PARENT_RECORD_VALUE_PREFIX.service_request.length),
    };
  }
  if (value.startsWith(PARENT_RECORD_VALUE_PREFIX.incident)) {
    return { kind: "incident", id: value.slice(PARENT_RECORD_VALUE_PREFIX.incident.length) };
  }
  return undefined;
}

/**
 * Display label for a parent-record option, as "CS0001234 — subject" (service
 * request) or "INC0001234 — subject" (incident) — number and subject are both
 * optional on the underlying search views, so it degrades to whichever exists
 * and finally to the id.
 */
export function parentRecordLabel(o: {
  id: string;
  number?: string | null;
  subject?: string | null;
}): string {
  return [o.number, o.subject].filter(Boolean).join(" — ") || o.id;
}

/**
 * Router state carried from an incident's own "Create change request…" action
 * (`CsmIncidentDetailPage`) to `/operations/change-requests/new`, mirroring
 * `CreateChangeRequestFromCaseNavState` — pre-selects that incident as the
 * intended parent so the picker starts populated rather than blank. Unlike
 * the service-request entry point, submitting with this pre-fill in place is
 * gated (see this file's header comment) until the backend accepts an
 * incident-linked change request.
 */
export interface CreateChangeRequestFromIncidentNavState {
  incidentId: string;
  incidentNumber?: string;
  incidentSubject?: string;
}

// ---------------------------------------------------------------------------
// Create-form in-progress draft persistence
//
// CreateChangeRequestPage is a normal route (`/change-requests/new`), not a
// tab kept alive by a persistent tab router — navigating to another
// operations tab unmounts it, and navigating back remounts it fresh, which
// would otherwise re-seed every field from the *original* clone/service-
// request/incident source again and silently discard anything the user had
// typed. sessionStorage (not a backend draft) closes that gap: the form
// writes its own state back on every change and restores from it on mount,
// scoped per browser tab (sessionStorage, not localStorage) so two tabs
// editing different change requests never collide.

/** Every field CreateChangeRequestPage keeps as local state, persisted as a
 * single JSON draft so restoring it is a straight round-trip into useState's
 * initializers. */
export interface ChangeRequestDraft {
  subject: string;
  type: string;
  impact: string;
  priority: string;
  plannedStartDate: string;
  plannedEndDate: string;
  description: string;
  justification: string;
  implementationPlan: string;
  riskImpactAnalysis: string;
  backoutPlan: string;
  testPlan: string;
  isPlanningVisibleToCustomers: boolean;
  /** Optional: a draft saved before these checkboxes existed lacks them. */
  customerApprovalRequired?: boolean;
  customerReviewRequired?: boolean;
  groupId: string;
  assignedEngineerId: string;
  requestedById: string;
  parentValue: string;
  /** Optional: a draft saved before these fields existed lacks them. The
   * `*Labels` maps hold the display names for the picked ids so a restored
   * draft shows names, not UUIDs, before the lookups resolve. */
  projectId?: string;
  projectLabel?: string;
  deploymentIds?: string[];
  deploymentLabels?: Record<string, string>;
  deploymentProductIds?: string[];
  deploymentProductLabels?: Record<string, string>;
  category?: string;
  comment?: string;
  workNote?: string;
}

/** Which of the create form's three entry points (or none — opened fresh) a
 * draft belongs to. Mirrors the mutually-exclusive nav-state shapes the page
 * itself narrows on. */
export type ChangeRequestDraftContext =
  | { kind: "clone"; sourceNumber?: string }
  | { kind: "case"; caseId: string }
  | { kind: "incident"; incidentId: string }
  | { kind: "new" };

const DRAFT_STORAGE_PREFIX = "csm.createChangeRequest.draft.";

/**
 * The sessionStorage key an in-progress draft is saved under, scoped to the
 * specific entry context the form was opened with. This is deliberate, not
 * incidental: a single shared key would mean navigating to this same route
 * for a *different* clone source (or a from-scratch change request, or a
 * different originating service request/incident) would silently load a
 * stale, mismatched draft left over from an earlier, unrelated in-progress
 * edit — arguably worse than today's bug, since the wrong content would look
 * plausible rather than obviously reset.
 */
export function changeRequestDraftKey(context: ChangeRequestDraftContext): string {
  switch (context.kind) {
    case "clone":
      // Falls back to a fixed suffix on the (unexpected) case where a cloned
      // record carries no number at all, rather than collapsing into the
      // same key as the from-scratch path.
      return `${DRAFT_STORAGE_PREFIX}clone:${context.sourceNumber ?? "unknown"}`;
    case "case":
      return `${DRAFT_STORAGE_PREFIX}case:${context.caseId}`;
    case "incident":
      return `${DRAFT_STORAGE_PREFIX}incident:${context.incidentId}`;
    case "new":
      return `${DRAFT_STORAGE_PREFIX}new`;
  }
}

/** Reads back a previously saved draft for `key`, or `null` when there is
 * none — including when sessionStorage is unavailable or the stored value
 * doesn't parse, so a corrupt/foreign entry degrades to "no draft" rather
 * than throwing during render. */
export function loadChangeRequestDraft(key: string): ChangeRequestDraft | null {
  try {
    const raw = sessionStorage.getItem(key);
    return raw ? (JSON.parse(raw) as ChangeRequestDraft) : null;
  } catch {
    return null;
  }
}

/** Persists the form's current field values under `key`. Best-effort:
 * sessionStorage can throw (quota, private-mode restrictions) and losing
 * draft persistence is a degraded experience, not a reason to break the
 * form, so a failure here is swallowed. */
export function saveChangeRequestDraft(key: string, draft: ChangeRequestDraft): void {
  try {
    sessionStorage.setItem(key, JSON.stringify(draft));
  } catch {
    // See doc comment above.
  }
}

/** Removes the draft at `key` — called once the change request this draft
 * was building has actually been created, or the user explicitly cancels, so
 * a later visit to the same entry context starts clean instead of restoring
 * stale content. */
export function clearChangeRequestDraft(key: string): void {
  try {
    sessionStorage.removeItem(key);
  } catch {
    // See saveChangeRequestDraft's doc comment.
  }
}
