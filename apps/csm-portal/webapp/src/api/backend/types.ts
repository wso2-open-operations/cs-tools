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

export type BeCasePriority =
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

export type BeCaseState =
  | "open"
  | "work_in_progress"
  | "waiting_on_wso2"
  | "awaiting_info"
  | "reopened"
  | "solution_proposed"
  | "closed";

export type BeCaseSortField = "created_at" | "updated_at" | "closed_at";

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
  priority?: BeCasePriority;
  issueType?: BeCaseIssueType;
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
  priority?: BeCasePriority;
  issueType?: BeCaseIssueType;
  state?: BeCaseState;
  nextStates?: BeCaseState[];
  createdBy?: BeUserRef;
  /** The CS engineer the case is assigned to; null when unassigned. */
  assignedEngineer?: BeEntityRef | null;
  account?: BeCaseAccountRef;
  project?: BeEntityRef;
  deployment?: BeEntityRef;
  deployedProduct?: BeDeployedProductRef;
  createdOn?: string;
  updatedOn?: string;
  closedAt?: string | null;
}

export interface BeCaseCreatePayload {
  projectId: string;
  deploymentId: string;
  deployedProductId: string;
  subject: string;
  description: string;
  priority: BeCasePriority;
  issueType: BeCaseIssueType;
}

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

/**
 * Request body for `PATCH /cases/{id}`. At least one of `state` / `priority`
 * must be set (mirrors the entity `UpdateCaseRequest`).
 */
export interface BeCaseUpdatePayload {
  state?: BeCaseState;
  priority?: BeCasePriority;
}

/** Request body for `POST /cases/search` (the flat, cross-project search). */
export interface BeCaseSearchPayload {
  pagination?: BePagination;
  searchQuery?: string;
  /** Optional project filter; omit for a cross-project search. */
  projectIds?: string[];
  deploymentIds?: string[];
  deployedProductIds?: string[];
  stateKeys?: BeCaseState[];
  priorityKeys?: BeCasePriority[];
  issueTypeKeys?: BeCaseIssueType[];
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
  subject?: string;
  description?: string;
  priority?: BeCasePriority;
  issueType?: BeCaseIssueType;
  state?: BeCaseState;
  createdOn?: string;
  updatedOn?: string;
  closedAt?: string | null;
  createdBy?: BeUserRef;
  project?: BeEntityRef;
  deployment?: BeEntityRef;
  deployedProduct?: BeDeployedProductRef;
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
  deploymentTypeKeys?: BeDeploymentType[];
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
