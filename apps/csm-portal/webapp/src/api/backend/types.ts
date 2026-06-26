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
  closedAt?: string;
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
  nextStates?: BeCaseState[];
  createdBy?: BeUserRef;
  /** The CS engineer the case is assigned to; null when unassigned. */
  assignedEngineer?: BeAssignedEngineerRef | null;
  account?: BeCaseAccountRef;
  project?: BeEntityRef;
  /** Nullable: ServiceNow-sourced cases may have no deployment / product. */
  deployment?: BeEntityRef | null;
  deployedProduct?: BeDeployedProductRef | null;
  /** SR catalog refs (managed-cloud); null for non-catalog cases. */
  catalog?: BeEntityRef | null;
  catalogItem?: BeEntityRef | null;
  /** Assigned team and linked chat conversation; null when not set. */
  assignedTeam?: BeEntityRef | null;
  conversation?: BeEntityRef | null;
  createdOn?: string;
  updatedOn?: string;
  closedAt?: string | null;
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
  | { state: BeCaseState; severity?: never; workState?: never; assigneeEmail?: never; watchList?: never }
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
  // NOTE: there is no assigned-engineer filter here yet. The cases-list
  // assignee control is disabled until the entity/BFF add one (e.g.
  // `assignedTo`/`assignedToMe`, mirroring `createdBy`/`createdByMe`).
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

// ---------------------------------------------------------------------------
// Attachments
// ---------------------------------------------------------------------------

/** A case attachment as returned by `POST /cases/{id}/attachments/search`. */
export interface BeAttachment {
  id: string;
  caseId: string;
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

export interface BeAttachmentSearchPayload {
  pagination?: BePagination;
}

export interface BeAttachmentSearchResponse extends BeSearchResponseBase {
  attachments: BeAttachment[];
}

/**
 * Upload payload for `POST /cases/{id}/attachments`. `file` is a base64 data
 * URI (e.g. `data:image/png;base64,...`); the BE caps the decoded size at 10 MB.
 */
export interface BeAttachmentCreatePayload {
  name: string;
  type: string;
  file: string;
  description?: string | null;
}

/** Thin ack returned by `POST /cases/{id}/attachments`. */
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
  createdAt?: string;
  updatedAt?: string;
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
  cores?: number | null;
  tps?: number | null;
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
