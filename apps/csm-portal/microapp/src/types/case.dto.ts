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

export type CaseType = "case" | "service_request" | "security_report_analysis" | "engagement" | "announcement";

export type EngagementType = "migration" | "consultancy" | "new_feature_improvement" | "follow_up" | "onboarding";

export type CaseSeverity = "catastrophic" | "critical" | "high" | "medium" | "low";

export type CaseState =
  | "open"
  | "work_in_progress"
  | "waiting_on_wso2"
  | "awaiting_info"
  | "reopened"
  | "solution_proposed"
  | "closed";

export type CaseWorkState = "ongoing" | "paused" | null;

export type CaseIssueType =
  | "error"
  | "partial_outage"
  | "performance_degradation"
  | "question"
  | "security_or_compliance"
  | "total_outage";

export interface EntityRefDto {
  id: string;
  name: string;
}

// The canonical {id, email, name} UserReference shape (openapi.yaml), used for CaseViewDto's
// createdBy — confirmed live against a real response. Previously modeled as {id, displayName,
// userId, email}, none of which (besides id/email) exist on the real object — CaseDetailPage's
// `displayName` read always came up empty and silently fell back to rendering the email instead
// of the name. openapi.yaml marks both UserReference and CaseView.createdBy nullable, so callers
// must not assume a CaseViewDto always has one.
export interface UserRefDto {
  id: string | null;
  email: string;
  name: string;
}

export interface UserIdEmailRefDto {
  id: string;
  email: string;
}

// CaseView/CaseSearchView.assignedEngineer both reference the canonical UserReference schema
// (openapi.yaml) — {id: string|null, email: string, name: string} — not the separate,
// differently-nullable AssignedEngineerRef schema used elsewhere (e.g. acknowledgedBy). Field
// names already matched reality (no render crash), but id/email's nullability were backwards:
// id can genuinely be null, email is always populated when assignedEngineer itself is non-null.
export interface AssignedEngineerRefDto {
  id: string | null;
  name: string;
  email: string;
}

export interface CaseNumberRefDto {
  id: string;
  number: string;
}

export interface AccountRefDto {
  id: string;
  name: string;
  type: string;
}

export interface DeployedProductRefDto {
  id: string;
  displayName: string;
}

export interface CaseSearchFiltersDto {
  types?: CaseType[];
  searchQuery?: string;
  projectIds?: string[];
  deploymentIds?: string[];
  states?: CaseState[];
  severities?: CaseSeverity[];
  issueTypes?: CaseIssueType[];
  assignedUserIds?: string[];
  createdByMe?: boolean;
  workStates?: NonNullable<CaseWorkState>[];
  /** Filter by engagement type; only applies when `types` includes `"engagement"`. */
  engagementTypes?: EngagementType[];
  /** Product family names (e.g. "API Manager"); matches all versions of each. */
  productNames?: string[];
  /** UUID of the case whose children (their own `parentId` pointing here) to find — the
   * hierarchical major-case/child-case relationship. Mirrors the webapp's useSearchChildCases. */
  parentId?: string;
}

export interface CaseSearchPayloadDto {
  filters?: CaseSearchFiltersDto;
  sortBy?: {
    field?: "createdOn" | "updatedOn" | "severity" | "state";
    order?: "asc" | "desc";
  };
  pagination?: {
    offset?: number;
    limit?: number;
  };
}

export interface CaseSearchViewDto {
  id: string;
  number: string;
  // openapi.yaml's CaseSearchView documents this field as `wso2Id`, but the live /cases/search
  // response sends it as `internalId` (confirmed live: {"id":"20d3964f-...","internalId":"CPPSUB-175",
  // "number":"CS0441016",...}) — same spec-vs-reality drift already documented elsewhere in this
  // file. `wso2Id` is kept as the app-facing name in case.model.ts's CaseSummary/CaseDetail (it's
  // the concept name used throughout the UI); only this wire-level DTO field is renamed to match
  // what actually arrives.
  internalId: string;
  subject: string;
  description: string;
  // Only meaningful for the "case" type — null for service_request/security_report_analysis/etc.
  // Loosely typed when present: the backend sends either the canonical word ("high") or a legacy
  // ServiceNow priority code ("P2", "High (P2)") depending on data source — see
  // normalizeSeverity() in case.model.ts.
  severity: string | null;
  issueType: CaseIssueType | null;
  // Loosely typed: e.g. "Work In Progress" (labeled, space-separated) rather than the snake_case
  // enum — see normalizeState() in case.model.ts.
  state: string;
  workState: CaseWorkState;
  type: string;
  createdOn?: string;
  updatedOn?: string;
  closedOn: string | null;
  // This comment used to claim the case-search view returns the creator's email as a plain
  // string, unlike the by-id detail view. That was never true: openapi.yaml documents
  // CaseSearchView.createdBy as the same nullable UserReference ({id, email, name}) as the
  // detail view, and live data confirms it (e.g. {"id":null,"email":"jane.doe@example.com","name":""}).
  // Rendering this directly as a string (AnnouncementCard.tsx, which shows CaseSummary.createdBy)
  // crashed with "Objects are not valid as a React child" and nothing caught it, so the whole
  // Announcements page went blank. Same bug class as CaseCommentDto/AttachmentViewDto/CaseViewDto's
  // createdBy fields, just the one instance not caught in that earlier pass.
  createdBy: string | UserRefDto | null;
  project: EntityRefDto;
  deployment: EntityRefDto | null;
  deployedProduct: EntityRefDto | null;
  product: EntityRefDto | null;
  assignedEngineer: AssignedEngineerRefDto | null;
  parentCase: CaseNumberRefDto | null;
  relatedCase: CaseNumberRefDto | null;
  account: AccountRefDto | null;
}

export interface CaseSearchResponseDto {
  cases: CaseSearchViewDto[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

// A linked case/service-request/change-request reference on the case-detail response — only
// id/number/name are ever carried (mirrors the webapp's BeLinkedServiceRequestRef/
// BeLinkedChangeRequestRef); `name` is the target's subject, nullable on records without one.
export interface CaseLinkRefDto {
  id: string;
  number: string;
  name: string | null;
}

export interface CaseViewDto {
  id: string;
  number: string;
  // See CaseSearchViewDto's internalId comment — same spec-vs-reality field-name drift, confirmed
  // by this same symptom (case detail header not showing a second id) on the by-id detail view.
  internalId: string;
  subject: string;
  description: string;
  severity: string | null;
  issueType: CaseIssueType | null;
  state: string;
  workState: CaseWorkState;
  type: string | null;
  engagementType: string | null;
  createdOn?: string;
  updatedOn?: string;
  closedOn: string | null;
  createdBy: UserRefDto | null;
  project: EntityRefDto;
  deployment: EntityRefDto | null;
  deployedProduct: DeployedProductRefDto | null;
  product: EntityRefDto | null;
  catalog: EntityRefDto | null;
  catalogItem: EntityRefDto | null;
  assignedTeam: EntityRefDto | null;
  conversation: EntityRefDto | null;
  assignedEngineer: AssignedEngineerRefDto | null;
  parentCase: CaseNumberRefDto | null;
  relatedCase: CaseNumberRefDto | null;
  account: AccountRefDto | null;
  nextStates: CaseState[];
  linkedServiceRequests?: CaseLinkRefDto[] | null;
  linkedChangeRequests?: CaseLinkRefDto[] | null;
}

export type CaseCommentType = "work_note" | "comment" | "activity";

export interface CaseCommentAuthorDto {
  id: string | null;
  email: string;
  name: string;
}

export interface CaseCommentDto {
  id: string;
  caseId: string;
  type: CaseCommentType;
  content: string;
  createdBy: string | CaseCommentAuthorDto;
  createdOn: string;
}

export interface CaseCommentSearchResponseDto {
  comments: CaseCommentDto[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

export interface CaseCommentCreatePayloadDto {
  type: CaseCommentType;
  content: string;
}

// openapi.yaml declares POST /cases/{id}/comments' 201 response as the full CaseComment shape,
// but the live response is actually a thin ack — {message, comment: {id, createdOn, createdBy}},
// missing type/content/caseId entirely (confirmed live: {"message":"Comment created
// successfully","comment":{"id":"...","createdOn":"...","createdBy":"jane.doe@example.com"}}).
// Matches the webapp's own documented workaround for this same gap (usePostCsmCaseComment.ts) —
// don't try to build a full Comment from this response; refetch the list instead.
export interface CaseCommentCreateResponseDto {
  message?: string;
  comment: {
    id: string;
    createdOn: string;
    createdBy: string;
  };
}

// Backend's UpdateCaseRequest: exactly one of state/severity/workState/assigneeEmail/parentId/
// relatedCaseId must be provided per PATCH call (they're mutually exclusive `oneOf` branches in
// openapi.yaml) — resolutionCode/cause/closeNotes are the exception, allowed alongside `state`
// only.
export interface CasePatchPayloadDto {
  state?: CaseState;
  severity?: CaseSeverity;
  workState?: NonNullable<CaseWorkState>;
  /** An email assigns the case; an explicit `null` clears the assignee. */
  assigneeEmail?: string | null;
  resolutionCode?: CaseResolutionCode;
  cause?: CaseCause;
  closeNotes?: string;
  /** Links this case to another as its parent (the hierarchical major-case/child-case
   * relationship) — this case can't close while it has open children linked this way. */
  parentId?: string;
  /** Cross-links this case to another as a related case — looser than `parentId`, not subject to
   * the child-case close restriction. */
  relatedCaseId?: string;
}

export interface UpdateCaseResponseDto {
  message: string;
  case: { id: string; updatedOn: string };
}

// Mirrors backend's CaseResolutionCode/CaseCause enums (openapi.yaml) — only allowed alongside
// `state: closed` or `state: solution_proposed`.
export type CaseResolutionCode =
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

export type CaseCause =
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

// Mirrors the webapp's BeCaseCreatePayload (the "case" type variant).
export interface CaseCreatePayloadDto {
  type: "case";
  projectId: string;
  deploymentId: string;
  deployedProductId: string;
  subject: string;
  description: string;
  severity: CaseSeverity;
  issueType: CaseIssueType;
}

// The "security_report_analysis" type variant — see NewSecurityReportPage.tsx. Unlike the "case"
// type, attachments are embedded directly in the create payload (raw base64, no `data:` prefix)
// rather than uploaded separately via POST /attachments after the case exists; the backend
// requires at least one entry.
export interface SecurityReportCreatePayloadDto {
  type: "security_report_analysis";
  projectId: string;
  deploymentId: string;
  deployedProductId: string;
  subject: string;
  description: string;
  attachments: { name: string; file: string }[];
}

/** A single answered catalog-item variable in a service-request create. */
export interface CaseVariableDto {
  /** Variable (question) id, as returned by the catalog-item variables endpoint. */
  id: string;
  /** The engineer's answer for this variable. */
  value: string;
}

// The "service_request" type variant — see NewServiceRequestPage.tsx. Mirrors the webapp's
// BeServiceRequestCreatePayload: catalog/catalogItem come from the deployed-product-scoped
// catalog cascade, variables from that catalog item's form. Like the "case" type, attachments
// upload separately via POST /attachments after the case exists (the create endpoint only
// embeds attachments for "security_report_analysis").
export interface ServiceRequestCreatePayloadDto {
  type: "service_request";
  projectId: string;
  deploymentId: string;
  deployedProductId: string;
  catalogId: string;
  catalogItemId: string;
  variables: CaseVariableDto[];
  /** UUID of the case this service request is filed from — links the new request back to it in
   * the same create call, mirroring the webapp's CreateServiceRequestFromCaseNavState flow. */
  relatedCaseId?: string;
}

export interface CreatedCaseDto {
  id: string;
}

export interface CaseCreateResponseDto {
  message?: string;
  case: CreatedCaseDto;
}
