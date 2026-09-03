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
  CaseState,
  CaseWorkState,
  Severity,
  SeverityOrUnset,
  SlaClockType,
} from "@features/csm-dashboard/types/abtDashboard";
import type {
  BeCaseAutoclosureStep,
  BeCaseCause,
  BeCaseIssueType,
  BeCaseResolutionCode,
  BeCaseType,
} from "@api/backend/types";
import type { UserReference } from "@/types/userReference";

export interface CsmCaseRow {
  /**
   * UUID primary key. Identifies the case in API paths and links only — never
   * shown to humans. Use {@link caseNumber} / {@link wso2CaseId} for display.
   */
  id: string;
  /**
   * ServiceNow-style case number (e.g. "CS-1007"). The number engineers and
   * customers quote day to day. Optional: absent until the BE assigns one, and
   * never substituted with the UUID {@link id}.
   */
  caseNumber?: string;
  /**
   * Project/subscription-scoped WSO2 case reference (e.g. "ACMESUB-123").
   * Distinct from {@link caseNumber}; shown alongside it as `wso2CaseId/caseNumber`.
   * Mirrors the customer portal's `internalId` / BE `wso2Id`. Optional: a case
   * may have no WSO2 reference, and it is never substituted with the UUID
   * {@link id}.
   */
  wso2CaseId?: string;
  subject: string;
  customer: string;
  accountId: string;
  projectId: string;
  projectName: string;
  /** Affected WSO2 product (e.g. "WSO2 Identity Server"). Used for list filtering. */
  product: string;
  /**
   * `"unset"` when the source has no severity value at all (empty/missing),
   * or the value doesn't match anything `severityFromBe` recognizes — a
   * distinct fact from "the severity really is S3/Medium", never collapsed
   * into a real severity. See `severityFromBe` in `api/backend/mappers.ts`.
   */
  severity: SeverityOrUnset;
  state: CaseState;
  /**
   * Case type (BE `typeKey` / search `caseType`). Optional: a legacy row may
   * omit it, in which case the type filter treats it as unmatched.
   */
  caseType?: BeCaseType;
  /** How the issue was categorized at creation (outage/degradation/question/
   * etc. — see {@link BeCaseIssueType}). Optional: a legacy row, or one not
   * created through the "Case" flow (service requests etc. don't collect
   * this), may have none. */
  issueType?: BeCaseIssueType;
  /**
   * Engagement type (e.g. "Migration"), only meaningful when `caseType` is
   * `"engagement"`. Carried through unmodified from the backend's raw display
   * label (not normalized/lowercased) — see `detailFromBeCase` in
   * `useGetCsmCaseDetail.ts`. Consumers that need to test for "is this a
   * migration engagement" must compare case-insensitively, matching the
   * backend's own `strings.EqualFold` check in `RequestCaseUpdate`.
   */
  engagementType?: string;
  /**
   * Work sub-state of an in-progress case (`ongoing` / `paused`); `null` when
   * the case is not `work_in_progress`. Drives the comment gate and the paused
   * indicator. See {@link commentGateReason}.
   */
  workState?: CaseWorkState | null;
  /** CRE / engineer working the case. "Unassigned" for cases with no one picked up yet. */
  assignee: string;
  assigneeIsMe: boolean;
  /** Who reported/opened the case — distinct from {@link assignee}. Optional:
   * absent for any `CsmCaseRow` source that doesn't resolve a creator (e.g.
   * `CsmCaseDetail`'s own {@link CsmCaseDetail.createdBy}, resolved from a
   * different, richer data source, already declares this same field
   * optionally). `mapCaseSearchViewToRow` always sets it, falling back to
   * "Unknown" rather than leaving it unset. */
  createdBy?: string;
  slaClockType: SlaClockType;
  // Minutes until breach (negative = already breached).
  minutesToBreach: number;
  /**
   * Whether SLA timing is actually known for this row. The backend has no SLA
   * data yet, so LIVE rows set this false and the list renders a neutral "—"
   * instead of a misleading orange "0m left". Absent/true → render the clock.
   */
  hasSla?: boolean;
  createdAt: string;
  /** Falls back to {@link createdAt} when the backend hasn't returned
   * `updatedOn` for this row; rendered unprefixed either way. */
  updatedAt: string;
  /**
   * The case's current escalation level: one of the raw escalation-level ids
   * `"0"` through `"5"` (EL0 "not escalated" through EL5 "CEO"), carried
   * through unmapped from `BeCaseView.escalationLevel` /
   * `BeCaseSearchView.escalationLevel` — see
   * `features/csm-cases/utils/escalationLevel.ts` for the display label/color
   * ramp. Null/absent when the backing case carries no escalation level (e.g.
   * non-ServiceNow-backed cases) or is not escalated (`"0"`).
   */
  escalationLevel?: string | null;
}

/**
 * One escalation-level change recorded against a case, as returned by
 * `GET /cases/{id}/escalations` (newest first). Mirrors the wire shape
 * (`BeCaseEscalation`) closely — this is a read-mostly history list, not
 * something the rest of the app maps into a different shape.
 */
export interface CaseEscalationRecord {
  id: string;
  currentLevel: string;
  previousLevel: string;
  createdBy: string;
  createdOn: string;
  reason?: string | null;
}

export interface CsmCasesListResponse {
  cases: CsmCaseRow[];
  /** Total rows matching the query across all pages (BE `total`). */
  total: number;
  /** Page size used for this response (BE `limit`). */
  limit: number;
  /** Zero-based row offset of this page (BE `offset`). */
  offset: number;
  /** Whether more rows exist beyond this page (BE `hasMore`). */
  hasMore: boolean;
}

export type CsmCommentAuthorRole =
  | "customer"
  | "wso2_engineer"
  | "system"
  | "chatbot";

export interface CsmCaseComment {
  id: string;
  caseId: string;
  authorName: string;
  /** Author's email, when the backend returns one — used to link the author
   * name to their profile page. */
  authorEmail?: string;
  /** Canonical reference to the author, when the backend returns one. `id`
   * is populated for a comment author — prefer this over {@link authorEmail}
   * for linking; the email is still needed for bot detection and as the
   * resolution fallback when `id` is null. */
  authorUser?: UserReference;
  authorRole: CsmCommentAuthorRole;
  bodyHtml: string;
  createdAt: string;
  /**
   * Internal work note (not visible to the customer). `false` = public comment
   * that the customer sees in their portal. Mirrors SN's
   * `sn_customerservice_case.work_notes` vs `comments` distinction.
   */
  internal?: boolean;
  /** True for a client-synthesized entry (e.g. the case description echoed
   * into the feed when the backing data source never actually created a
   * comment for it). Suppresses the author-role chip in the rendered bubble —
   * the real creator's role isn't known on the frontend, so nothing should be
   * claimed about it. */
  synthetic?: boolean;
}

export interface CaseAttachment {
  id: string;
  filename: string;
  /** File size in bytes. */
  size: number;
  contentType: string;
  uploadedBy: string;
  /** Uploader's email, when the backend returns one — used to link the
   * uploader name to their profile page. */
  uploadedByEmail?: string;
  /** Canonical reference to the uploader, when the backend returns one. `id`
   * is populated for an attachment uploader. */
  uploadedByUser?: UserReference;
  uploadedAt: string;
}

/**
 * Account tier as shown on a case. Free-form: the CaseView's `account.type` is
 * the PG `basic|enterprise` enum for native cases but a raw ServiceNow support
 * tier (e.g. "Enterprise") for SN-sourced ones, so render it defensively.
 */
export type CustomerTier = string;

/**
 * Stage of a case SLA record, as returned by the case SLA list endpoint. An
 * open enum: the known values below still drive autocomplete, but a stage
 * outside this set (e.g. one added later on the backend) is kept as-is
 * rather than forced into the closed set.
 */
export type SlaStage =
  | "in_progress"
  | "paused"
  | "completed"
  | "cancelled"
  | "breached"
  | (string & {});

/**
 * A single SLA record attached to a case. All time fields
 * are pre-formatted server-side (`*Label`) — the frontend renders them as-is
 * rather than recomputing.
 */
export interface CaseSla {
  id: string;
  /** SLA definition name (e.g. "S1 - Response"). */
  definition: string;
  /** Target duration as a display string (e.g. "4 Business Hours"); absent for open-ended SLAs. */
  target: string | null;
  stage: SlaStage;
  /** Human-readable stage label from the backend (e.g. "In progress"). */
  stageLabel: string;
  hasBreached: boolean;
  businessTimeLeftLabel: string;
  businessElapsedLabel: string;
  /** Percentage (0-100+) of the target consumed in business time. */
  businessElapsedPercent: number;
  /** ISO-8601 UTC; null when the SLA clock hasn't started. */
  startTime: string | null;
  /** ISO-8601 UTC; null while the SLA is still running. */
  stopTime: string | null;
}

export interface CaseSlaList {
  caseId: string;
  count: number;
  slas: CaseSla[];
}

/**
 * Raw SLA record shape returned by the task-SLA search endpoint. One record
 * per SLA definition attached to a task (a case is a task). Kept close to the
 * wire shape; {@link CaseSla} is the row model the SLA table actually renders,
 * built from this by {@link useGetCsmCaseSlas}.
 */
export interface TaskSlaView {
  id: string;
  slaDefinition: {
    id?: string | null;
    name?: string | null;
    type?: string | null;
    target?: string | null;
  } | null;
  stage: string | null;
  task: {
    id?: string | null;
    number?: string | null;
  } | null;
  businessTimeLeft: string | null;
  businessElapsedTime: string | null;
  businessElapsedPercentage: number | null;
  startTime: string | null;
  endTime: string | null;
}

export interface TaskSlaSearchPayload {
  filters?: {
    taskIds?: string[];
  };
  pagination?: {
    limit?: number;
    offset?: number;
  };
}

export interface TaskSlaSearchResponse {
  slas: TaskSlaView[];
  total: number;
  limit: number;
  offset: number;
}

export interface CaseWatcher {
  id: string;
  /** Display name, falling back to the SN username when no display name is set. */
  name: string;
  email?: string;
  isMe?: boolean;
  /** Canonical reference to this watcher, when the backend returns one. `id`
   * is always null here — watchers resolve their profile link through the
   * cached email lookup like any other actor without a resolved id. */
  user?: UserReference;
}

export interface CaseLinkedItem {
  id: string;
  kind: "case" | "incident" | "escalation" | "kb" | "cr" | "sr";
  reference: string;
  title: string;
  state: string;
  /** Optional relative URL for in-app navigation. */
  href?: string;
}

/**
 * One answered catalog-item question on a service request. `value` is `""`
 * when the question was asked and left blank — the UI renders that as an em
 * dash so it stays distinguishable from a question that was never asked.
 */
export interface CaseRequestVariable {
  name: string;
  value: string;
}

export interface CaseTag {
  id: string;
  label: string;
  color?: "default" | "primary" | "warning" | "info" | "success" | "error";
}

export interface CaseTimeLogEntry {
  id: string;
  engineer: string;
  hours: number;
  note: string;
  date: string;
}

export type CaseAuditKind =
  | "state_change"
  | "assignee_change"
  | "severity_change"
  | "linked"
  | "escalated"
  | "watcher_added"
  | "comment_added"
  | "attachment_added"
  | "sla_breached"
  | "created"
  | "field_change";

/** One field changed within a single audited save-transaction. */
export interface CaseAuditFieldChange {
  field: string;
  fieldLabel: string;
  /** Absent/empty when the field was previously unset (a "set" change). */
  previousValue?: string;
  /** Absent/empty when the field was cleared. */
  newValue?: string;
}

/** One Case Feedback survey submission for a case, shown in its activity feed. */
export interface CaseFeedbackEntry {
  id: string;
  rating: number;
  ratingLabel: string;
  /** `null`/absent for feedback submitted without a comment. */
  comment?: string | null;
  submittedAt: string;
  /** `null`/absent if the submitting user record could not be resolved. */
  submitterName?: string | null;
  /** `null`/absent, same as submitterName. */
  submitterEmail?: string | null;
}

export interface CaseAuditEntry {
  id: string;
  kind: CaseAuditKind;
  actor: string;
  /** Canonical reference to the actor, when the backend supplies one. `id` is
   * typically null here (the activity feed doesn't resolve one) and `email`
   * is sometimes a non-email username (e.g. an automation account) rather
   * than a real address — `UserRefLink`'s plausibility check already refuses
   * to look those up, so this is safe to pass through as-is. */
  actorUser?: UserReference;
  /** Free-text summary; used when `changes` is absent (older/synthetic entries). */
  description?: string;
  createdAt: string;
  /** Populated for `kind === "field_change"`: one save-transaction may touch
   * several fields at once (e.g. state + assignee in the same update). */
  changes?: CaseAuditFieldChange[];
}

export interface CaseCustomerContext {
  accountName: string;
  tier: CustomerTier;
  region: string;
  primaryContact: string;
  primaryContactEmail: string;
  accountManager: string;
  technicalOwner?: string;
  /** Number of currently open cases against this customer (incl. this one). */
  openCases: number;
  /** The account's assigned CRE (customer reliability engineering) team, when set (ServiceNow only). */
  creTeam?: { id: string; name: string };
  /** The account's assigned SRE (site reliability engineering) team, when set (ServiceNow only). */
  sreTeam?: { id: string; name: string };
}

/**
 * Deployment classification from the fixed deployment-type list. Mirrors the
 * backend `DeploymentTypeKey` enum (`@features/support/types/case`); kept local
 * so the case product context stays self-contained.
 */
export type DeploymentCategory =
  | "primary_production"
  | "staging"
  | "qa"
  | "stress"
  | "uat"
  | "development";

export interface CaseProductContext {
  product: string;
  version: string;
  updateLevel?: string;
  /** Customer-provided deployment name (set by the customer in the project). */
  deployment: string;
  /** Deployment UUID, when the case is linked to one — drives the detail link. */
  deploymentId?: string;
  /** Deployed-product UUID, when the case is linked to one. */
  deployedProductId?: string;
  /** Fixed-list classification of the deployment (e.g. Primary Production). */
  deploymentCategory?: DeploymentCategory;
  environment: "dev" | "qa" | "staging" | "prod";
  region?: string;
}

export type CaseLifecycleAction =
  | "start_work"
  | "assign_to_me"
  | "propose_solution"
  | "request_info"
  | "wait_on_wso2"
  | "resume_work"
  | "close"
  | "close_no_response"
  // Closed-case replacement for reopening: the backend surfaces this via a
  // `reopened` entry in `nextStates` (a real reopen is never valid — see
  // that field's doc), but it must NOT be PATCHed like a real transition —
  // it opens the new-case form pre-filled with relatedCaseId instead.
  | "create_related_case"
  // Generic transition into a state the frontend has no curated action for
  // (e.g. a state added on the backend). Drives the post-transition toast only;
  // the PATCH target always comes from the backend `nextStates` value.
  | "transition";

/**
 * Router (`navigate(..., { state })`) payload carried from a closed case's
 * "Create related case" action to `/cases/new`, so the new-case form can
 * prefill from the case it's related to without a query-string round trip
 * or a full page load. All fields but the ids are just starting values —
 * the form leaves every one of them editable. See CsmCaseDetailPage.tsx's
 * `create_related_case` handler and CsmCaseCreatePage.tsx's read of
 * `useLocation().state`.
 */
export interface CreateRelatedCaseNavState {
  projectId: string;
  relatedCaseId: string;
  relatedCaseNumber?: string;
  deploymentId?: string;
  deployedProductId?: string;
  severity?: Severity;
  issueType?: BeCaseIssueType;
  subject?: string;
}

/**
 * Router (`navigate(..., { state })`) payload carried from a case's "Create
 * service request" action — either the Related tab's Linked service requests
 * card, or the "More" menu's "Create service request…" item — to
 * `/operations/service-requests/new`, so the create-service-request form can
 * prefill from the originating case and file the new SR as linked to it in
 * one step — no separate create-then-link round trip. `projectId` seeds the
 * form's Project field locked read-only (mirrors
 * {@link CreateRelatedCaseNavState} / CsmCaseCreatePage.tsx); `deploymentId` /
 * `deployedProductId` are just starting values and stay fully editable. See
 * CsmCaseDetailPage.tsx's two "Create service request" entry points and
 * CreateServiceRequestPage.tsx's read of `useLocation().state`.
 */
export interface CreateServiceRequestFromCaseNavState {
  projectId: string;
  relatedCaseId: string;
  relatedCaseNumber?: string;
  deploymentId?: string;
  deployedProductId?: string;
}

/**
 * Router (`navigate(..., { state })`) payload carried from a service
 * request's "Create change request…" action (case detail action bar) to
 * `/operations/change-requests/new`, so the create-change-request form's
 * "Originating service request" picker can pre-select the service request the
 * action was opened from and scope its search to the same project — see
 * CsmCaseDetailPage.tsx's `create_change_request` handler and
 * CreateChangeRequestPage.tsx's read of `useLocation().state`. Distinct from
 * ChangeRequestsTab.tsx's own "Create change request" button, which navigates
 * to the same route with no state at all — that entry point has no service
 * request to prefill or scope by, so its picker searches the whole system
 * exactly as it always has. `projectId` is optional here (unlike the sibling
 * nav states above) because a legacy case row can omit it; the picker simply
 * skips scoping when it's absent.
 */
export interface CreateChangeRequestFromCaseNavState {
  caseId: string;
  caseNumber?: string;
  caseSubject?: string;
  projectId?: string;
}

/**
 * Router (`navigate(..., { state })`) payload carried from a case's "Create
 * incident from case…" action to `/operations/incidents/new`, so the
 * create-incident form can prefill from the originating case without a
 * query-string round trip or a full page load. `caseId` seeds the incident's
 * `parentId` (ServiceNow's generic task-parent reference — the same field
 * used for the case-to-case/case-to-incident hierarchical link, not the
 * incident-specific `parentIncidentId`); every field is just a starting
 * value the form leaves editable. See CsmCaseDetailPage.tsx's
 * `create_incident` handler and CreateIncidentPage.tsx's read of
 * `useLocation().state`.
 */
export interface CreateIncidentFromCaseNavState {
  caseId: string;
  caseNumber?: string;
  subject?: string;
  /** Plain text — the case's rich-text description with tags stripped. */
  description?: string;
}

/**
 * Full case detail used by the case detail page. Extends the lightweight
 * row type used in lists with all the side-widget data plus a curated set
 * of state-driven primary actions.
 */
export interface CsmCaseDetail extends CsmCaseRow {
  description: string;
  assignmentGroup: string;
  /**
   * The engineer who acknowledged the case — a first-write-wins claim that
   * someone has seen it and picked it up, distinct from being assigned to it.
   * Absent when nobody has acknowledged yet, which is what makes the
   * acknowledge action available. Cleared upstream when the case's type or
   * severity changes or it is reopened, so it can become absent again.
   */
  acknowledgedBy?: { name: string; email?: string };
  /**
   * When the case's workaround was marked provided (ISO date-time), or absent
   * until marked (and cleared again on recall). Pauses the case's Workaround
   * SLA clock while set.
   */
  workaroundProvidedOn?: string;
  /** The engineer who marked the workaround as provided. Absent until marked. */
  workaroundProvidedBy?: { name: string; email?: string };
  /** Category of issue reported, when set (e.g. "total_outage", "question"). */
  issueType?: BeCaseIssueType;
  /**
   * Id of the chat conversation this case was spawned from, when any. Drives
   * loading the Novera chat transcript as the earliest entries in the activity
   * feed (mirrors the customer portal). Absent when the case has no linked
   * conversation (e.g. non-ServiceNow source, or a case opened without chat).
   */
  conversationId?: string;
  /**
   * States this case may transition into next, per the backend. For a closed
   * case, a `reopened` entry is not a real reopen (the data source has none)
   * — it signals "Create related case" is available within its 60-day window.
   */
  nextStates?: CaseState[];
  /** The case this one was created as related to, when any. */
  relatedCase?: { id: string; caseNumber?: string };
  /**
   * The case, incident, change request, or problem this case is linked to as
   * its parent (the hierarchical major-case/child-case relationship). Absent
   * when not linked. `type` may still be undefined/null for older data the
   * backend can't resolve a type for — treat that as "case" (the only type
   * this link supported before cross-table parents existed).
   */
  parentCase?: {
    id: string;
    caseNumber?: string;
    type?: "case" | "incident" | "change_request" | "problem" | null;
  };
  /**
   * Service-request cases whose parent points to this case. Populated on
   * every case detail response, not just high-severity cases.
   */
  linkedServiceRequests?: { id: string; number: string; name: string }[];
  /**
   * Change requests raised from this case. Only service-request cases carry
   * these; absent/empty otherwise. One-to-many: promoting the same change
   * through multiple environments produces one change request per
   * environment, all pointing back at the same service request.
   */
  linkedChangeRequests?: {
    id: string;
    number: string;
    /** Subject, or `null` when the record has none — never `""`. */
    name: string | null;
  }[];
  /**
   * The catalog and catalog item a service request was raised against.
   * Absent for every other case type.
   */
  catalog?: { id: string; name: string };
  catalogItem?: { id: string; name: string };
  /**
   * The answers the requester gave to the catalog item's questions, in the
   * backing data source's display order. Absent when the request carried no
   * answers; an individual `value` of `""` means the question was asked and
   * left blank, which is deliberately distinct from the question never being
   * asked at all.
   */
  requestVariables?: CaseRequestVariable[];
  /**
   * Where the case sits in the backing data source's staged auto-closure
   * sequence (ServiceNow only). Read-only; `undefined`/`"DEFAULT"` means no
   * hold is in effect.
   */
  autoclosureStep?: BeCaseAutoclosureStep;
  /** When the auto-closure sequence next advances (ServiceNow only). Read-only. */
  autoclosureStateTime?: string;
  /**
   * Internal-only best-case fix estimate, as a date-only "YYYY-MM-DD"
   * string; `null`/absent when not set. Never shared with the customer.
   * Distinct from the backend-computed SLA clocks shown in {@link CaseSla} —
   * these are settable commitments, not derived clocks.
   */
  bestCaseFixEta?: string | null;
  /**
   * Internal-only most-likely fix estimate, as a date-only "YYYY-MM-DD"
   * string; `null`/absent when not set.
   */
  mostLikelyFixEta?: string | null;
  /**
   * Internal-only worst-case fix estimate, as a date-only "YYYY-MM-DD"
   * string; `null`/absent when not set.
   */
  worstCaseFixEta?: string | null;
  /** Display name of the person who opened the case. */
  createdBy?: string;
  /** Email of the creator — used to tell a WSO2 engineer from a customer. */
  createdByEmail?: string;
  /** Canonical reference to the case creator. `id` is always null here — the
   * data source doesn't resolve the reporter to a user record on this view,
   * so the profile link (if any) comes from resolving {@link createdByEmail}. */
  createdByUser?: UserReference;
  /**
   * Raw assigned-engineer name, with no "Unassigned" display fallback baked
   * in (unlike `assignee`) — for callers that need to tell "actually
   * unassigned" apart from a display label, without string-comparing
   * against that fallback.
   */
  assigneeName?: string;
  /** Email of the assigned engineer, when the data source returns one — used
   * to link the assignee name to their profile page. */
  assigneeEmail?: string;
  /** Canonical reference to the assigned engineer, when the backend returns
   * one. `id` is populated for the case assignee. */
  assigneeUser?: UserReference;
  customerContext: CaseCustomerContext;
  productContext: CaseProductContext;
  watchers: CaseWatcher[];
  linkedItems: CaseLinkedItem[];
  tags: CaseTag[];
  timeLogs: CaseTimeLogEntry[];
  /** Lifecycle/audit entries — distinct from threaded comments. */
  audit: CaseAuditEntry[];
  attachments: CaseAttachment[];
  /** Whether the current user is watching this case (controls Watch toggle). */
  isWatching: boolean;
  /**
   * Post Resolution Activity from a prior close/propose-solution, when set —
   * used to prefill {@link ResolutionDialog} instead of reopening it blank.
   */
  resolution?: {
    resolutionCode?: BeCaseResolutionCode;
    cause?: BeCaseCause;
    notes?: string;
  };
}
