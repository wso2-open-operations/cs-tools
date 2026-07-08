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

/**
 * Types mirroring `apps/csm-portal/backend/openapi.yaml`. The CSM portal
 * backend proxies these calls to the entity service. Keep this file in sync
 * with the OpenAPI spec: every endpoint hook below imports its request /
 * response from here. Types are prefixed `Be` to distinguish them from
 * existing UI-shape types (`Case`, `Project`, etc.) without namespace imports.
 */

// ---------------------------------------------------------------------------
// Shared
// ---------------------------------------------------------------------------

export interface BePagination {
  /** Zero-based offset. */
  offset?: number;
  /** Page size; capped per endpoint (typically 100). */
  limit?: number;
}

export interface BeErrorPayload {
  message?: string;
}

export interface BeSearchResponseBase {
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

// ---------------------------------------------------------------------------
// Cases
// ---------------------------------------------------------------------------

export type BeCaseSeverity =
  | "catastrophic"
  | "critical"
  | "high"
  | "medium"
  | "low";

export type BeCaseIssueType =
  | "error"
  | "partial_outage"
  | "performance_degradation"
  | "question"
  | "security_or_compliance"
  | "total_outage";

/** Case type (entity `type`). Only `case` is creatable from the portal. */
export type BeCaseType =
  | "case"
  | "service_request"
  | "security_report_analysis"
  | "announcement"
  | "engagement";

export type BeCaseState =
  | "open"
  | "work_in_progress"
  | "waiting_on_wso2"
  | "awaiting_info"
  | "reopened"
  | "solution_proposed"
  | "closed";

/**
 * Work sub-state of a `work_in_progress` case (entity `CaseWorkState`). `null`
 * when the case is not in progress. The backend rejects comment creation unless
 * the case is `work_in_progress` AND `ongoing`.
 */
export type BeCaseWorkState = "ongoing" | "paused";

/**
 * Resolution code for a closed or solution-proposed case (the Post
 * Resolution Activity — see UseCases.md ISSU-026). Only accepted by
 * `PATCH /cases/{id}` alongside `state: "closed"` or `"solution_proposed"`.
 */
export type BeCaseResolutionCode =
  | "SOLVED_FIXED_BY_SUPPORT_GUIDANCE_PROVIDED"
  | "SOLVED_FIXED_BY_CLOSING_RELATED_INCIDENT"
  | "SOLVED_FIXED_BY_CLOSING_RELATED_RD_TICKET"
  | "SOLVED_WORKAROUND_PROVIDED"
  | "SOLVED_BY_CUSTOMER"
  | "CONSIDERED_FOR_ROADMAP"
  | "INCONCLUSIVE_OUT_OF_SCOPE"
  | "INCONCLUSIVE_CANNOT_REPRODUCE"
  | "INCONCLUSIVE_NO_WORKAROUND"
  | "DUPLICATE_ISSUE"
  | "VOIDED_CANCELED"
  | "ON_HOLD"
  | "CONSIDERED_FOR_ROADMAP_ALT"
  | "SOLVED_FIXED_THE_ISSUE"
  | "SOLVED_WORKAROUND_PROVIDED_ALT"
  | "SOLVED_BY_CONTRIBUTOR"
  | "SOLVED_BY_NOVERA"
  | "ABRUPTLY_CLOSED_DUE_TO_NON_RESPONSIVENESS";

/** Root-cause category for a closed or solution-proposed case. Same gating as {@link BeCaseResolutionCode}. */
export type BeCaseCause =
  | "USER_MISUNDERSTANDING_CONCEPTS"
  | "USER_MISUNDERSTANDING_DOCUMENTATION"
  | "USER_NOT_FOLLOWING_DOCUMENTATION"
  | "USER_MISTAKE"
  | "SOLUTION_PROBLEMATIC_SOLUTION_ARCHITECTURE"
  | "SOLUTION_PROBLEMATIC_CODE"
  | "APPLICATION_BUG"
  | "APPLICATION_MISLEADING_UX_UI"
  | "APPLICATION_LIMITATION"
  | "APPLICATION_MISSING_FEATURE"
  | "APPLICATION_DOCUMENTATION_GAP"
  | "APPLICATION_DOCUMENTATION_ERROR"
  | "INFRASTRUCTURE_CUSTOMERS_SIDE"
  | "INFRASTRUCTURE_SAAS_SIDE_NOT_ENOUGH"
  | "INFRASTRUCTURE_SAAS_SIDE_OTHER"
  | "UNKNOWN";

export type BeCaseSortField = "createdOn" | "updatedOn" | "severity" | "state";

export interface BeCase {
  id: string;
  number?: string;
  wso2Id?: string;
  createdBy?: string;
  projectId?: string;
  deploymentId?: string;
  deployedProductId?: string;
  subject?: string;
  description?: string;
  severity?: BeCaseSeverity;
  issueType?: BeCaseIssueType;
  type?: BeCaseType;
  state?: BeCaseState;
  createdAt?: string;
  updatedAt?: string;
  closedOn?: string;
}

/** A referenced user, as embedded in case views (not just an id string). */
export interface BeUserRef {
  id: string;
  name?: string;
  userId?: string;
  email?: string;
}

/** A referenced entity carrying its display name (project, deployment, ...). */
export interface BeEntityRef {
  id: string;
  name: string;
}

/** A referenced case carrying only its display number, e.g. the related case. */
export interface BeCaseNumberRef {
  id: string;
  number?: string;
}

/**
 * The assigned CS engineer embedded in case views. Carries `email` so the FE
 * can tell whether the case is assigned to the signed-in user (the only stable
 * identity the FE has from the JWT). `email` may be `null` depending on the data
 * source / endpoint — the GET populates it where available.
 */
export interface BeAssignedEngineerRef {
  id: string;
  name?: string;
  email?: string | null;
}

/** A referenced deployed product (its display name is the product + version). */
export interface BeDeployedProductRef {
  id: string;
  displayName: string;
}

/**
 * Account summary embedded in the CaseView. `type` is the account/support tier
 * (e.g. "Enterprise"); it is a free-form string, not the PG `basic|enterprise`
 * enum, because ServiceNow-sourced cases pass their support tier through as-is.
 */
export interface BeCaseAccountRef {
  id: string;
  name?: string;
  type?: string;
}

/**
 * `GET /cases/{id}` response — the rich CaseView. Unlike {@link BeCase} (the
 * flat create/legacy shape), it embeds the related entities as objects, so the
 * UI gets account / project / deployment / deployed-product / reporter names
 * without any extra lookups.
 */
export interface BeCaseView {
  id: string;
  number?: string;
  /** Project-scoped WSO2 case reference (customer portal calls this the same). */
  internalId?: string;
  subject?: string;
  description?: string;
  /** Free-form display string from the backend, e.g. "Critical (P1)", "Low (P4)". */
  severity?: string;
  issueType?: BeCaseIssueType;
  type?: BeCaseType | null;
  /** Engagement type; only meaningful for `engagement` cases. Null otherwise. */
  engagementType?: string | null;
  state?: BeCaseState;
  /** Work sub-state; only meaningful while `state` is `work_in_progress`. */
  workState?: BeCaseWorkState | null;
  /**
   * States this case may transition into next. For a closed case, `reopened`
   * appearing here is not a real reopen (the data source has no such
   * transition) — it signals that a new case may still be created as related
   * to this one, within its 60-day window.
   */
  nextStates?: BeCaseState[];
  /** The case this one was created as related to, when any. */
  relatedCase?: BeCaseNumberRef | null;
  createdBy?: BeUserRef;
  /** The CS engineer the case is assigned to; null when unassigned. */
  assignedEngineer?: BeAssignedEngineerRef | null;
  account?: BeCaseAccountRef;
  project?: BeEntityRef;
  /** Nullable: ServiceNow-sourced cases may have no deployment / product. */
  deployment?: BeEntityRef | null;
  deployedProduct?: BeDeployedProductRef | null;
  /**
   * The product itself (distinct from {@link deployedProduct}, the
   * deployment-scoped instance). Populated even when no specific deployed
   * product is linked, so it's the reliable fallback for the product name.
   */
  product?: BeEntityRef | null;
  /** SR catalog refs (managed-cloud); null for non-catalog cases. */
  catalog?: BeEntityRef | null;
  catalogItem?: BeEntityRef | null;
  /** Assigned team and linked chat conversation; null when not set. */
  assignedTeam?: BeEntityRef | null;
  conversation?: BeEntityRef | null;
  createdOn?: string;
  updatedOn?: string;
  closedOn?: string | null;
}

export interface BeCaseCreatePayload {
  /** Standard support case. */
  type: "case";
  projectId: string;
  deploymentId: string;
  deployedProductId: string;
  subject: string;
  description: string;
  severity: BeCaseSeverity;
  issueType: BeCaseIssueType;
  /**
   * UUID of the closed case this one is related to. The data source only
   * accepts this for a case closed within the last 60 days — otherwise it
   * rejects the create with a "related case too old" error.
   */
  relatedCaseId?: string;
  /** Optional supporting files (raw base64), like the customer portal. */
  attachments?: BeCaseAttachmentPayload[];
}

/** A single answered catalog-item variable in a service-request create. */
export interface BeCaseVariable {
  /** Variable (question) id, as returned by the catalog-item variables endpoint. */
  id: string;
  /** The engineer's answer for this variable. */
  value: string;
}

/**
 * Catalog-based service request (`type: "service_request"`). ServiceNow-only:
 * the catalog/catalog-item come from `POST /catalogs/search` (filtered by the
 * deployed product) and the variables from the catalog-item variables endpoint.
 */
export interface BeServiceRequestCreatePayload {
  type: "service_request";
  projectId: string;
  deploymentId: string;
  deployedProductId: string;
  catalogId: string;
  catalogItemId: string;
  variables: BeCaseVariable[];
}

/**
 * An inline attachment in a security-report create. Note `file` is **raw**
 * base64 (the file's bytes), NOT a `data:` URI — the SRA create path passes it
 * straight through. (The separate post-case attachment endpoint, by contrast,
 * requires a data URI.)
 */
export interface BeCaseAttachmentPayload {
  /** File name including extension. */
  name: string;
  /** Base64-encoded file content (raw base64, no `data:` prefix). */
  file: string;
}

/**
 * Security report analysis (`type: "security_report_analysis"`). ServiceNow-only;
 * requires a subject, description, and at least one attachment.
 */
export interface BeSecurityReportCreatePayload {
  type: "security_report_analysis";
  projectId: string;
  deploymentId: string;
  deployedProductId: string;
  subject: string;
  description: string;
  attachments: BeCaseAttachmentPayload[];
}

/**
 * Any body accepted by `POST /cases`: a standard support case, a catalog
 * service request, or a security report analysis.
 */
export type BeCaseCreateBody =
  | BeCaseCreatePayload
  | BeServiceRequestCreatePayload
  | BeSecurityReportCreatePayload;

/** The case summary embedded in the `POST /cases` success envelope. */
export interface BeCreatedCase {
  id: string;
  internalId?: string;
  number?: string;
  createdBy?: string;
  createdOn?: string;
  state?: string;
}

/** `POST /cases` response: a message plus the created case. */
export interface BeCaseCreateResponse {
  message?: string;
  case: BeCreatedCase;
}

// ---------------------------------------------------------------------------
// Service catalogs (ServiceNow only) — drive service-request creation
// ---------------------------------------------------------------------------

/** A catalog item (request form) within a catalog. */
export interface BeCatalogItemRef {
  id: string;
  name?: string;
}

/** A service catalog and the catalog items it offers. */
export interface BeCatalogRef {
  id: string;
  name?: string;
  catalogItems?: BeCatalogItemRef[];
}

/** Request body for `POST /catalogs/search`. */
export interface BeSearchCatalogsPayload {
  /** Deployed product to scope catalogs to (required). */
  deployedProductId: string;
  pagination?: { offset?: number; limit?: number };
}

/** `POST /catalogs/search` response. */
export interface BeSearchCatalogsResponse {
  catalogs?: BeCatalogRef[];
  total?: number;
  limit?: number;
  offset?: number;
}

/**
 * A catalog-item variable (form field). The contract carries the question
 * text, display order, and a free-form `type` hint, but no choice/option list
 * or mandatory flag — so the portal renders every variable as a text field.
 */
export interface BeCatalogItemVariable {
  id: string;
  questionText?: string;
  order?: number;
  type?: string;
}

/** `GET /catalogs/{catalogId}/items/{catalogItemId}/variables` response. */
export interface BeGetCatalogItemVariablesResponse {
  variables?: BeCatalogItemVariable[];
}

/**
 * Request body for `PATCH /cases/{id}` (mirrors the entity `UpdateCaseRequest`).
 * **Exactly one** of `state` / `severity` / `workState` /
 * `assigneeEmail` / `watchList` is sent per call — the backend rejects zero or
 * more than one. Encoded as a discriminated union (each variant `?: never`s the
 * others) so the exactly-one-field contract is enforced at compile time, not
 * just in docs. `assigneeEmail` and `watchList` are supported **only** for the
 * ServiceNow data source. `workState` is only accepted while the case is
 * `work_in_progress`.
 */
export type BeCaseUpdatePayload =
  | {
      state: BeCaseState;
      severity?: never;
      workState?: never;
      assigneeEmail?: never;
      watchList?: never;
      /** Post Resolution Activity — only meaningful (and only accepted by the backend) alongside `state: "closed"` or `"solution_proposed"`. */
      resolutionCode?: BeCaseResolutionCode;
      cause?: BeCaseCause;
      closeNotes?: string;
    }
  | { state?: never; severity: BeCaseSeverity; workState?: never; assigneeEmail?: never; watchList?: never }
  /** Work sub-state toggle (`ongoing` / `paused`) for an in-progress case. */
  | { state?: never; severity?: never; workState: BeCaseWorkState; assigneeEmail?: never; watchList?: never }
  /** Email of the engineer to assign (ServiceNow only). */
  | { state?: never; severity?: never; workState?: never; assigneeEmail: string; watchList?: never }
  /** Full replacement watch list as emails (ServiceNow only). */
  | { state?: never; severity?: never; workState?: never; assigneeEmail?: never; watchList: string[] };

/** A user in the case watch list, as echoed by `PATCH /cases/{id}`. */
export interface BeWatchListUser {
  id: string;
  userName: string;
  name?: string;
  email?: string;
}

/** The mutated case fields echoed by `PATCH /cases/{id}`. */
export interface BeUpdatedCase {
  id: string;
  updatedOn?: string;
  updatedBy?: string;
  state?: BeCaseState;
  severity?: BeCaseSeverity;
  workState?: BeCaseWorkState | null;
  watchList?: BeWatchListUser[];
  assignedTo?: BeEntityRef | null;
}

/** `PATCH /cases/{id}` response: a message plus the mutated case fields. */
export interface BeUpdateCaseResponse {
  message?: string;
  case: BeUpdatedCase;
}

export type BeEngagementType =
  | "migration"
  | "consultancy"
  | "new_feature_improvement"
  | "follow_up"
  | "onboarding";

/** Request body for `POST /cases/search` (the flat, cross-project search). */
/** Filter block for `POST /cases/search`; all fields are optional. */
export interface BeCaseSearchFilters {
  /** Searches across subject, number, and wso2Id (case-insensitive). */
  searchQuery?: string;
  /** Optional project filter; omit for a cross-project search. */
  projectIds?: string[];
  deploymentIds?: string[];
  types?: BeCaseType[];
  states?: BeCaseState[];
  severities?: BeCaseSeverity[];
  issueTypes?: BeCaseIssueType[];
  /** Filter by engagement type; only applies when `types` includes `"engagement"`. */
  engagementTypes?: BeEngagementType[];
  /** Filter to cases created by these emails. */
  createdBy?: string[];
  /** When true, the caller's email (from the JWT) is appended to `createdBy`. */
  createdByMe?: boolean;
  /**
   * Work sub-state filter (ServiceNow: `ongoing` → 1, `paused` → 2). Only
   * meaningful when `states` includes `work_in_progress`.
   */
  workStates?: BeCaseWorkState[];
  /**
   * Filter by assigned-engineer user UUIDs. The cases-list assignee picker is
   * email/`@me`-based; `useGetCsmCases` resolves the selection to UUIDs (named
   * engineers via `/users/search`, `@me` via the app-wide current-user context
   * backed by `/users/me`). `@me` needs the caller's `id`, which `/users/me`
   * omits only when the entity service is unavailable — in that case an
   * `@me`-only selection resolves to nothing and the filter is omitted.
   */
  assignedUserIds?: string[];
  /**
   * Filter by product family name (e.g. `"API Manager"`, `"Asgardeo"`). Matches
   * every version of each named product (ServiceNow matches `product.name`).
   * SN data source only.
   */
  productNames?: string[];
}

export interface BeCaseSearchPayload {
  /** All filter fields are nested here; `sortBy`/`pagination` stay top-level. */
  filters?: BeCaseSearchFilters;
  pagination?: BePagination;
  sortBy?: {
    field?: BeCaseSortField;
    order?: "asc" | "desc";
  };
}

/**
 * Item shape returned by `POST /cases/search`. Like {@link BeCaseView} it
 * embeds the related entities, so the list gets project / deployment /
 * deployed-product names without extra lookups. (The account/customer is not
 * embedded.)
 */
export interface BeCaseSearchView {
  id: string;
  number?: string;
  /** Project-scoped WSO2 case reference (customer portal calls this the same). */
  internalId?: string;
  /**
   * Case subject. The search response now returns `subject`, matching the
   * `GET /cases/{id}` CaseView (the earlier `title` naming inconsistency is gone).
   */
  subject?: string;
  description?: string;
  /** Free-form display string from the backend, e.g. "Low (P4)", "Critical (P1)". */
  severity?: string;
  issueType?: BeCaseIssueType;
  /** Case type (e.g. `case`, `service_request`). */
  type?: BeCaseType;
  state?: BeCaseState;
  /** Work sub-state; only meaningful while `state` is `work_in_progress`. */
  workState?: BeCaseWorkState | null;
  /** The CS engineer the case is assigned to; null when unassigned. */
  assignedEngineer?: BeAssignedEngineerRef | null;
  createdOn?: string;
  /** Often absent on the search view (unlike the GET view); tolerate it missing. */
  updatedOn?: string;
  /** Created-by is a bare email string here (not a UserRef like the GET view). */
  createdBy?: string;
  project?: BeEntityRef;
  /** Nullable: ServiceNow-sourced cases may have no deployment / product. */
  deployment?: BeEntityRef | null;
  /** Embedded as `{ id, name }` (name already includes the version), not the
   * `displayName`-shaped ref the GET view uses. */
  deployedProduct?: BeEntityRef | null;
  /** The product itself (distinct from `deployedProduct`); used to populate the
   * list Product column when no deployed product is set (e.g. cloud cases). */
  product?: BeEntityRef | null;
}

export interface BeCaseSearchResponse extends BeSearchResponseBase {
  cases: BeCaseSearchView[];
}

// ---------------------------------------------------------------------------
// Case comments
// ---------------------------------------------------------------------------

export type BeCaseCommentType = "work_note" | "comment" | "activity";

/** Author block embedded in a comment; the BE hydrates it from the user store. */
export interface BeCaseCommentAuthor {
  id: string;
  firstName?: string;
  lastName?: string;
  fullName?: string;
}

export interface BeCaseComment {
  id: string;
  caseId: string;
  type: BeCaseCommentType;
  /** Rich-text HTML body (sanitised at render time). */
  content: string;
  createdBy: BeCaseCommentAuthor;
  createdOn: string;
}

/** Comment types a client may create. `activity` is system-generated only. */
export type BeCreatableCommentType = Exclude<BeCaseCommentType, "activity">;

export interface BeCaseCommentCreatePayload {
  type: BeCreatableCommentType;
  /** Rich-text HTML body. */
  content: string;
}

export interface BeCaseCommentSearchPayload {
  pagination?: BePagination;
}

export interface BeCaseCommentSearchResponse extends BeSearchResponseBase {
  comments: BeCaseComment[];
}

/**
 * Comment shape returned by the generic `POST /comments/search`, which — since
 * the entity-service refactor — backs BOTH `/cases/{id}/comments/search` and
 * `/conversations/{id}/messages`. It reuses `referenceId` (the case or
 * conversation id) rather than `caseId`; the backend normalizes `type` to the
 * `BeCaseCommentType` enum (unknown SN types default to `comment`).
 *
 * `createdBy` is typed `BeCaseCommentAuthor | string`: search/messages embed the
 * nested author object, while the comment-create ack echoes only a bare string.
 * The mapper handles both.
 */
export interface BeComment {
  id: string;
  /** Parent reference id — the case id or conversation id per the endpoint. */
  referenceId?: string;
  /** Rich-text HTML (case comment) or Markdown (Novera chat) body. */
  content: string;
  /** Normalized comment type; `string` (not the enum) to tolerate new values. */
  type: string;
  createdOn: string;
  createdBy: BeCaseCommentAuthor | string | null;
}

export interface BeCommentSearchResponse extends BeSearchResponseBase {
  /** Optional: the backend may omit the array on an empty result. */
  comments?: BeComment[];
}

// ---------------------------------------------------------------------------
// Case activities (unified comment / attachment / field-change stream)
// ---------------------------------------------------------------------------

/** One field changed within a single audited save-transaction. */
export interface BeFieldChange {
  /** Wire field name (e.g. `state`, `priority`, `assignedEngineer`). */
  field: string;
  /** Human-readable label for the field (e.g. "State", "Severity"). */
  fieldLabel: string;
  /** Absent/empty when the field was previously unset. */
  previousValue?: string;
  /** Absent/empty when the field was cleared. */
  newValue?: string;
}

export type BeCaseActivityType = "comment" | "attachment" | "field_change";

/**
 * One entry from `POST /cases/{id}/activities/search`. Shared fields are
 * present on every entry regardless of `type`; `changes` is populated only
 * for `type === "field_change"`. This endpoint intentionally excludes work
 * notes — the comments/work-notes feed continues to read from
 * `/cases/{id}/comments/search` (see {@link BeComment}).
 */
export interface BeCaseActivityEntry {
  id: string;
  type: BeCaseActivityType;
  content?: string;
  createdOn: string;
  createdBy?: string;
  createdByFirstName?: string;
  createdByLastName?: string;
  createdByFullName?: string;
  /** Only present on `type === "field_change"` entries. */
  changes?: BeFieldChange[];
}

export interface BeCaseActivitiesSearchPayload {
  pagination?: BePagination;
  /** Whether the response should include `field_change` entries. */
  includeFieldChanges?: boolean;
}

export interface BeCaseActivitiesSearchResponse extends BeSearchResponseBase {
  activity?: BeCaseActivityEntry[];
}

// ---------------------------------------------------------------------------
// Attachments
// ---------------------------------------------------------------------------

/**
 * Entity an attachment is linked to. Attachments are no longer case-scoped:
 * the same endpoints serve change requests, deployments, and conversations.
 */
export type BeReferenceType =
  | "case"
  | "conversation"
  | "change_request"
  | "deployment";

/** A attachment as returned by `POST /attachments/search`. */
export interface BeAttachment {
  id: string;
  referenceId: string;
  referenceType: BeReferenceType;
  name: string;
  /** MIME type (e.g. image/png, application/pdf). */
  type: string;
  sizeBytes: number;
  description?: string | null;
  createdBy: string;
  createdOn: string;
  downloadUrl?: string | null;
  previewUrl?: string | null;
}

/**
 * Search payload for `POST /attachments/search`. `referenceId` + `referenceType`
 * scope the search to one entity (both required by the BE).
 */
export interface BeAttachmentSearchPayload {
  referenceId: string;
  referenceType: BeReferenceType;
  pagination?: BePagination;
}

export interface BeAttachmentSearchResponse extends BeSearchResponseBase {
  attachments: BeAttachment[];
}

/**
 * Upload payload for `POST /attachments`. `referenceId` + `referenceType` link
 * the file to its owning entity; `file` is a base64 data URI (e.g.
 * `data:image/png;base64,...`); the BE caps the decoded size at 10 MB.
 */
export interface BeAttachmentCreatePayload {
  referenceId: string;
  referenceType: BeReferenceType;
  name: string;
  type: string;
  file: string;
  description?: string | null;
}

/** Thin ack returned by `POST /attachments`. */
export interface BeAttachmentDetail {
  id: string;
  sizeBytes: number;
  createdOn: string;
  createdBy: string;
  downloadUrl: string;
}

export interface BeAttachmentCreateResponse {
  message?: string;
  attachment?: BeAttachmentDetail;
}

/** Ack returned by `DELETE /attachments/{id}`. */
export interface BeDeleteAttachmentResponse {
  message?: string;
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

export type BeUserType = "internal" | "customer";

export interface BeUser {
  id?: string;
  userName?: string;
  firstName?: string;
  lastName?: string;
  email?: string;
  phone?: string;
  timezone?: string;
  userType?: BeUserType;
  createdAt?: string;
  updatedAt?: string;
}

export interface BeUserSearchPayload {
  pagination?: BePagination;
  searchQuery?: string;
}

export interface BeUserSearchResponse extends BeSearchResponseBase {
  users: BeUser[];
}

// ---------------------------------------------------------------------------
// Accounts
// ---------------------------------------------------------------------------

export type BeAccountTier = "basic" | "enterprise";

export interface BeAccount {
  id: string;
  sfId?: string;
  name?: string;
  tier?: BeAccountTier;
  region?: string;
  activationDate?: string;
  deactivationDate?: string;
  ownerId?: string;
  technicalOwnerId?: string;
  agentEnabled?: boolean;
  kbReferencesEnabled?: boolean;
  createdAt?: string;
  updatedAt?: string;
}

export interface BeAccountSearchPayload {
  pagination?: BePagination;
  searchQuery?: string;
}

export interface BeAccountSearchResponse extends BeSearchResponseBase {
  accounts: BeAccount[];
}

// ---------------------------------------------------------------------------
// Projects
// ---------------------------------------------------------------------------

export type BeSubscriptionType =
  | "development_support"
  | "managed_cloud_subscription"
  | "evaluation_subscription"
  | "subscription"
  | "cloud_evaluation_support"
  | "internal"
  | "platformer_subscription"
  | "cloud_support"
  | "professional_services";

export interface BeProject {
  id: string;
  accountId?: string;
  sfId?: string;
  name?: string;
  projectKey?: string;
  subscriptionType?: BeSubscriptionType;
  startDate?: string;
  endDate?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface BeProjectSearchPayload {
  pagination?: BePagination;
  searchQuery?: string;
}

export interface BeProjectSearchResponse extends BeSearchResponseBase {
  projects: BeProject[];
}

// ---------------------------------------------------------------------------
// Deployments
// ---------------------------------------------------------------------------

export type BeDeploymentType =
  | "primary_production"
  | "staging"
  | "qa"
  | "stress"
  | "uat"
  | "development";

export interface BeDeployment {
  id: string;
  projectId?: string;
  name?: string;
  type?: BeDeploymentType;
  description?: string;
  createdBy?: string;
  createdOn?: string;
  updatedOn?: string;
}

/**
 * Detail-field update for `PATCH /deployments/{id}`. At least one field must be
 * present (BE rejects an empty body). `description: null` clears the value.
 * `active` is forbidden on this variant — use `BeDeploymentDeactivatePayload`.
 *
 * Per PR #957: the BE now accepts the string `type` enum directly, so
 * `typeKey` (the old integer) is gone.
 */
export type BeDeploymentDetailUpdatePayload = {
  name?: string;
  type?: BeDeploymentType;
  description?: string | null;
  active?: never;
} & ({ name: string } | { type: BeDeploymentType } | { description: string | null });

/** Deactivation for `PATCH /deployments/{id}`: `active` must be `false`, alone. */
export interface BeDeploymentDeactivatePayload {
  active: false;
  name?: never;
  type?: never;
  description?: never;
}

export type BeDeploymentUpdatePayload =
  | BeDeploymentDetailUpdatePayload
  | BeDeploymentDeactivatePayload;

export interface BeDeploymentUpdateResponse {
  message: string;
  deployment: {
    id: string;
    updatedOn: string;
    updatedBy: string;
  };
}

/**
 * Request body for `POST /deployments`. All four fields are required per the
 * BE contract (PR #957: `type` is the string enum, `typeKey` integer is gone).
 */
export interface BeDeploymentCreatePayload {
  projectId: string;
  name: string;
  type: BeDeploymentType;
  description: string;
}

export interface BeDeploymentCreateResponse {
  message: string;
  deployment: {
    id: string;
    projectId: string;
    name: string;
    type: BeDeploymentType;
    description: string;
    createdOn: string;
    createdBy: string;
  };
}

export interface BeDeploymentSearchPayload {
  pagination?: BePagination;
  searchQuery?: string;
  projectIds?: string[];
  deploymentTypes?: BeDeploymentType[];
}

export interface BeDeploymentSearchResponse extends BeSearchResponseBase {
  deployments: BeDeployment[];
}

// ---------------------------------------------------------------------------
// Products + product versions
// ---------------------------------------------------------------------------

export type BeProductClass = "software" | "service";
export type BeProductSupportStatus =
  | "available"
  | "extended"
  | "deprecated"
  | "discontinued";

export interface BeProduct {
  id: string;
  name?: string;
  class?: BeProductClass;
  createdAt?: string;
  updatedAt?: string;
}

export interface BeProductSearchPayload {
  pagination?: BePagination;
  searchQuery?: string;
}

export interface BeProductSearchResponse extends BeSearchResponseBase {
  products: BeProduct[];
}

export interface BeProductVersion {
  id: string;
  productId?: string;
  version?: string;
  currentSupportStatus?: BeProductSupportStatus;
  releaseDate?: string;
  supportEolDate?: string;
  earliestPossibleSupportEolDate?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface BeProductVersionSearchPayload {
  pagination?: BePagination;
  searchQuery?: string;
}

export interface BeProductVersionSearchResponse extends BeSearchResponseBase {
  productVersions: BeProductVersion[];
}

// ---------------------------------------------------------------------------
// Deployed products (deployment ↔ product link)
// ---------------------------------------------------------------------------

/** Product version as embedded in a deployed-product record. */
export interface BeDeployedProductVersion {
  id: string;
  name: string;
  releasedDate?: string | null;
  supportEoLDate?: string | null;
}

export interface BeDeployedProduct {
  id: string;
  deployment?: BeEntityRef;
  product?: BeEntityRef;
  version?: BeDeployedProductVersion | null;
  // SN-only sizing fields; the entity service returns them as strings (and
  // always null for the Postgres data source).
  cores?: string | null;
  tps?: string | null;
  category?: string | null;
  createdOn?: string;
  updatedOn?: string;
}

export interface BeDeployedProductSearchPayload {
  pagination?: BePagination;
}

export interface BeDeployedProductSearchResponse extends BeSearchResponseBase {
  deployedProducts: BeDeployedProduct[];
}

/**
 * Request body for `POST /deployments/{id}/products`. All three ID fields are
 * required per the OpenAPI spec. Sizing fields are optional.
 */
export interface BeDeployedProductCreatePayload {
  projectId: string;
  productId: string;
  versionId: string;
  cores?: number;
  tps?: number;
  description?: string;
}

export interface BeDeployedProductCreateResponse {
  message?: string;
  deployedProduct?: {
    id: string;
    createdOn: string;
    createdBy: string;
  };
}

/**
 * Detail-field update for `PATCH /deployments/{deploymentId}/products/{productId}`.
 * At least one field required. All fields nullable (clears the value).
 * `active` is forbidden on this variant -- use `BeDeployedProductDeactivatePayload`.
 */
export interface BeDeployedProductDetailUpdatePayload {
  cores?: number | null;
  tps?: number | null;
  description?: string | null;
  active?: never;
}

/** Deactivation variant: `active` must be `false`, alone. */
export interface BeDeployedProductDeactivatePayload {
  active: false;
  cores?: never;
  tps?: never;
  description?: never;
}

export type BeDeployedProductUpdatePayload =
  | BeDeployedProductDetailUpdatePayload
  | BeDeployedProductDeactivatePayload;

export interface BeDeployedProductUpdateResponse {
  message?: string;
  deployedProduct?: {
    id: string;
    updatedOn: string;
    updatedBy: string;
  };
}

// ---------------------------------------------------------------------------
// Call requests
// ---------------------------------------------------------------------------

/**
 * State of a call request. `id` may be an integer or string (per the OpenAPI
 * spec -- treat it as opaque and use `label` for display). `label` is the
 * human-readable state name.
 */
export interface BeCallRequestState {
  id: number | string;
  label?: string;
}

/**
 * The 8 state keys that `SearchCallRequestsPayload.filters.states` and
 * `UpdateCallRequestPayload.state` accept as plain string enums.
 */
export type BeCallRequestStateKey =
  | "pending_on_customer"
  | "pending_on_wso2"
  | "scheduled"
  | "customer_rejected"
  | "wso2_rejected"
  | "canceled"
  | "notes_pending"
  | "concluded";

/** A call request list/detail item returned by `POST /cases/{id}/call-requests/search`. */
export interface BeCallRequestView {
  id: string;
  number?: string;
  case?: {
    id: string;
    name?: string;
    number?: string;
  };
  reason?: string;
  preferredTimes?: string[];
  durationMin?: number;
  scheduleTime?: string;
  meetingLink?: string;
  createdOn?: string;
  updatedOn?: string;
  state?: BeCallRequestState;
  cancellationReason?: string;
  /** Agent (or team) assigned to run the call, once scheduled. */
  assignee?: string;
  /** Call notes recorded after the call concludes. */
  notes?: string;
  /** Follow-up plan recorded alongside the call notes. */
  plan?: string;
  /** Attendee list recorded alongside the call notes. */
  attendees?: string;
  /** Action items recorded alongside the call notes. */
  actionItems?: string;
  /** Actual call duration in minutes, recorded alongside the call notes. */
  actualDurationMin?: number;
}

/** `POST /cases/{id}/call-requests` request body. */
export interface BeCreateCallRequestPayload {
  reason: string;
  /** One or more preferred UTC datetimes (ISO strings). */
  utcTimes: string[];
  /** Duration of the call in minutes (min 1). */
  durationInMinutes: number;
}

/** `POST /cases/{id}/call-requests` response. */
export interface BeCreateCallRequestResponse {
  message?: string;
  callRequest?: {
    id: string;
    createdOn?: string;
    createdBy?: string;
    state?: BeCallRequestState;
  };
}

// ---------------------------------------------------------------------------
// GitHub issue creation from a case — `POST /cases/{id}/github-issues`
// (ServiceNow data source only; the BFF forwards the body opaquely to the
// entity service, which validates and routes it to the SN scoped app).
// ---------------------------------------------------------------------------

/**
 * Why the issue is being filed. Selects the label set applied on the GitHub
 * issue and collapses the three legacy "open git issue" flows into one:
 * `default` (Open Git Issue), `migration` (Open Migration Git Issue),
 * `rd_ticket` (Open R&D ticket).
 */
export type BeCaseGithubIssueReason = "default" | "migration" | "rd_ticket";

/** Explicit owner/repo, overriding the product-based routing lookup on the SN side. */
export interface BeCaseGithubIssueRepoOverride {
  owner: string;
  repo: string;
}

/** `POST /cases/{id}/github-issues` request body. */
export interface BeCreateCaseGithubIssuePayload {
  reason: BeCaseGithubIssueReason;
  title: string;
  description: string;
  /** Explicit target repo; when omitted, the SN side routes by the case's product unit. */
  repoOverride?: BeCaseGithubIssueRepoOverride;
  /** Product update level, appended to the issue body. */
  updateLevel?: string;
  /** Link to a related public-facing GitHub issue, appended to the issue body. */
  publicIssueUrl?: string;
  /** When true: adds a `regression` label and tags the case as a regression. */
  regression?: boolean;
  /** When true: appends "Hotfix Required : Yes" to the issue body. */
  hotFixRequired?: boolean;
  /** Issue-type label to apply on GitHub (e.g. "Type/Patch", "Type/Incident"). */
  issueTypeLabel?: string;
  /** Priority label, applied only when `issueTypeLabel` is "Type/Incident". */
  priorityLevel?: string;
}

/** `POST /cases/{id}/github-issues` response. */
export interface BeCreateCaseGithubIssueResponse {
  message?: string;
  issue?: {
    /** URL of the created GitHub issue. */
    url: string;
    /** GitHub issue number. */
    number: number;
    /** Repo the issue was created in. */
    repo: string;
  };
}

/** `POST /cases/{id}/call-requests/search` request body. */
export interface BeSearchCallRequestsPayload {
  filters?: {
    states?: BeCallRequestStateKey[];
  };
  pagination?: {
    offset?: number;
    /** Default 20, max 100. */
    limit?: number;
  };
}

/** `POST /cases/{id}/call-requests/search` response. */
export interface BeSearchCallRequestsResponse {
  callRequests: BeCallRequestView[];
  total: number;
  offset: number;
  limit: number;
}

/**
 * `PATCH /cases/{caseId}/call-requests/{callRequestId}` request body.
 * `state` selects the transition; the optional fields below are interpreted
 * according to the target `state`:
 * - `scheduled` (agent schedule/reschedule): `meetingDate` + `durationInMinutes`
 *   required, `assignee` optional.
 * - `wso2_rejected` (agent reject) / `canceled`: `cancellationReason` optional
 *   (used as the reject/cancel reason).
 * - `concluded` (agent send notes): `notes` required, `plan`/`attendees`/
 *   `actionItems`/`actualDurationMin` optional.
 * - `pending_on_wso2` (reschedule request back to the customer): `utcTimes` +
 *   `durationInMinutes`.
 */
export interface BeUpdateCallRequestPayload {
  state: BeCallRequestStateKey;
  /** Used as the reject/cancellation reason for `wso2_rejected`/`canceled`. */
  cancellationReason?: string;
  /** Updated preferred UTC datetimes (used when `state` is `pending_on_wso2`). */
  utcTimes?: string[];
  /** Updated duration in minutes (min 1); used for `pending_on_wso2`/`scheduled`. */
  durationInMinutes?: number;
  /** UTC datetime (ISO string) the call is scheduled for; required for `scheduled`. */
  meetingDate?: string;
  /** Agent (or team) assigned to run the call; used for `scheduled`. */
  assignee?: string;
  /** Call notes; required for `concluded`. */
  notes?: string;
  /** Follow-up plan recorded alongside the call notes; used for `concluded`. */
  plan?: string;
  /** Attendee list recorded alongside the call notes; used for `concluded`. */
  attendees?: string;
  /** Action items recorded alongside the call notes; used for `concluded`. */
  actionItems?: string;
  /** Actual call duration in minutes; used for `concluded`. */
  actualDurationMin?: number;
}

/** `PATCH /cases/{caseId}/call-requests/{callRequestId}` response. */
export interface BeUpdateCallRequestResponse {
  message?: string;
  callRequest?: {
    id: string;
    updatedOn?: string;
    updatedBy?: string;
  };
}

// ---------------------------------------------------------------------------
// Change requests (managed-cloud; ServiceNow data source only)
// ---------------------------------------------------------------------------

export type BeChangeRequestState =
  | "customer_approval"
  | "scheduled"
  | "implement"
  | "review"
  | "customer_review"
  | "rollback"
  | "closed"
  | "canceled";

export type BeChangeRequestImpact = "high" | "medium" | "low";

/** List-item / shared shape for a change request (`POST /change-requests/search`). */
export interface BeChangeRequestSearchView {
  id: string;
  number?: string;
  subject?: string | null;
  description?: string | null;
  project?: BeEntityRef;
  case?: BeEntityRef | null;
  deployment?: BeEntityRef | null;
  deployedProduct?: BeEntityRef | null;
  product?: BeEntityRef | null;
  assignedEngineer?: BeEntityRef | null;
  assignedTeam?: BeEntityRef | null;
  plannedStartOn?: string | null;
  plannedEndOn?: string | null;
  duration?: string | null;
  impact?: string | null;
  state?: string | null;
  type?: string | null;
  createdOn?: string;
  updatedOn?: string;
}

/** `GET /change-requests/{id}` — the search view plus the heavy plan fields. */
export interface BeChangeRequestDetail extends BeChangeRequestSearchView {
  createdBy?: string;
  justification?: string | null;
  impactDescription?: string | null;
  serviceOutage?: string | null;
  communicationPlan?: string | null;
  rollbackPlan?: string | null;
  testPlan?: string | null;
  hasCustomerApproved?: boolean;
  hasCustomerReviewed?: boolean;
  approvedBy?: BeEntityRef | null;
  approvedOn?: string | null;
}

/**
 * `PATCH /change-requests/{id}` body (ServiceNow data source only). At least
 * one field is required by the BE (`minProperties: 1`). `plannedStartOn` is a
 * `YYYY-MM-DD HH:MM:SS` string.
 */
export interface BePatchChangeRequestPayload {
  plannedStartOn?: string;
  isCustomerApproved?: boolean;
  isCustomerReviewed?: boolean;
}

/** `PATCH /change-requests/{id}` response — the touched identifiers. */
export interface BePatchChangeRequestResponse {
  id: string;
  updatedOn?: string;
  updatedBy?: string;
}

export interface BeChangeRequestSearchPayload {
  filters?: {
    projectIds?: string[];
    searchQuery?: string;
    states?: BeChangeRequestState[];
    impacts?: BeChangeRequestImpact[];
    closedStartDate?: string;
    closedEndDate?: string;
  };
  sortBy?: {
    field?: "createdOn" | "updatedOn";
    order?: "asc" | "desc";
  };
  pagination?: BePagination;
}

/** Note: the CR search response carries no `hasMore` (unlike the other searches). */
export interface BeChangeRequestSearchResponse {
  changeRequests: BeChangeRequestSearchView[];
  total: number;
  limit: number;
  offset: number;
}

// ---------------------------------------------------------------------------
// Product vulnerabilities (managed-cloud; ServiceNow data source only)
// ---------------------------------------------------------------------------

/** Priority enum for product vulnerabilities. */
export type BeVulnerabilityPriority =
  | "info"
  | "low"
  | "medium"
  | "high"
  | "critical"
  | "unknown";

/**
 * Single product vulnerability as returned by both the search list and the
 * `GET /products/vulnerabilities/{id}` detail endpoint.
 */
export interface BeProductVulnerabilityView {
  id: string;
  cveId?: string;
  vulnerabilityId?: string;
  /** Priority label from the upstream (e.g. "High", "Critical"). */
  priority?: string;
  productName?: string | null;
  productVersion?: string | null;
  componentName?: string;
  version?: string;
  type?: string;
  componentType?: string | null;
  updateLevel?: string | null;
  useCase?: string | null;
  justification?: string | null;
  resolution?: string | null;
}

export interface BeSearchProductVulnerabilitiesFilters {
  searchQuery?: string;
  priority?: BeVulnerabilityPriority;
  productName?: string;
  productVersion?: string;
}

export interface BeSearchProductVulnerabilitiesPayload {
  filters?: BeSearchProductVulnerabilitiesFilters;
  pagination?: BePagination;
}

/** Note: the vulnerabilities search response carries no `hasMore`. */
export interface BeSearchProductVulnerabilitiesResponse {
  productVulnerabilities: BeProductVulnerabilityView[];
  total: number;
  limit: number;
  offset: number;
}

export type BeTimeCardState =
  | "pending"
  | "submitted"
  | "approved"
  | "rejected"
  | "processed"
  | "recalled";

export interface BeTimeCardRef {
  id: string;
  name: string;
}

export interface BeTimeCardCaseRef {
  id: string;
  name: string;
  number: string;
}

/**
 * A time card as returned by search and the mutation endpoints. The backend
 * never echoes back category / issue complexity / work-log comment / hour
 * breakdown / lead comment, even though those are accepted on write — see
 * {@link BeCreateTimeCardPayload}.
 */
export interface BeTimeCardView {
  id: string;
  totalTime: number;
  createdOn: string;
  hasBillable: boolean;
  state: BeTimeCardState;
  user?: BeTimeCardRef;
  approvedBy?: BeTimeCardRef;
  project?: BeTimeCardRef;
  case?: BeTimeCardCaseRef;
}

/**
 * `entity-service`'s `openapi.yaml` also documents a `caseId` filter here
 * (and it's genuinely implemented end-to-end, forwarded through to
 * ServiceNow) — deliberately omitted: confirmed live to be non-functional,
 * always returning `total: 0` even for a case with cards provably matching
 * that exact id. See the note on `useCaseTimeCards` in `useTimeCards.ts`
 * before re-adding it.
 */
export interface BeSearchTimeCardsFilters {
  projectIds?: string[];
  /** Only time cards submitted by this user. */
  userId?: string;
  /** Only time cards this user is eligible to approve (SN `approver_list`). */
  approverId?: string;
  /** Only time cards actually approved by this user (SN `approved_by`). */
  approvedById?: string;
  /** ISO 8601 date (YYYY-MM-DD). */
  startDate?: string;
  /** ISO 8601 date (YYYY-MM-DD). */
  endDate?: string;
  states?: BeTimeCardState[];
}

export interface BeSearchTimeCardsPayload {
  filters?: BeSearchTimeCardsFilters;
  pagination?: BePagination;
}

export interface BeSearchTimeCardsResponse {
  timeCards: BeTimeCardView[];
  total: number;
  limit: number;
  offset: number;
}

export interface BeCreateTimeCardPayload {
  caseId: string;
  projectId: string;
  /** ISO 8601 date (YYYY-MM-DD). */
  date: string;
  /** Eligible approvers (approver_list). Must be non-empty. */
  approverIds: string[];
  isBillable?: boolean;
  issueComplexity?: string;
  workLogComment?: string;
  timeAnalyzing?: number;
  timeSettingUp?: number;
  timeReproducingDebugging?: number;
  timeProvidingSolution?: number;
  timePatching?: number;
}

/**
 * Either editable fields (no `state`), or a state transition (`state`:
 * "approved", or "rejected" with a `leadComment`) — mutually exclusive, per
 * the backend contract. The portal only ever sends the state-transition form
 * (see ISSU-009 Edit-card gap: editable fields can't be safely round-tripped
 * since reads never return them).
 */
export interface BeUpdateTimeCardPayload {
  state?: Extract<BeTimeCardState, "approved" | "rejected">;
  leadComment?: string;
}

export interface BeTimeCardMutationResponse {
  message?: string;
  timeCard: BeTimeCardView;
}
