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

export interface UserRefDto {
  id: string;
  displayName: string;
  userId: string;
  email: string;
}

export interface UserIdEmailRefDto {
  id: string;
  email: string;
}

export interface AssignedEngineerRefDto {
  id: string;
  name: string;
  email: string | null;
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
  wso2Id: string;
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
  // The case-search view returns the creator's email as a plain string (unlike
  // the by-id detail view, which returns a full user object).
  createdBy: string;
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

export interface CaseViewDto {
  id: string;
  number: string;
  wso2Id: string;
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
  createdBy: UserRefDto;
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
}

export type CaseCommentType = "work_note" | "comment" | "activity";

// openapi.yaml declares `createdBy` as a plain string, but the live entity-service response
// doesn't consistently match that — it comes back as a richer {id, firstName, lastName, fullName}
// object for at least some comments (same createdBy string-vs-object drift already seen on
// CaseSearchViewDto — see reference_csm_announcements memory). Modeled as a union and normalized
// to a display string in `toComment` rather than trusted as a string at the DTO boundary.
export interface CaseCommentAuthorDto {
  id?: string;
  firstName?: string;
  lastName?: string;
  fullName?: string;
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
// successfully","comment":{"id":"...","createdOn":"...","createdBy":"hesara@wso2.com"}}).
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

// Backend's UpdateCaseRequest: exactly one of state/severity/workState/assigneeEmail must be
// provided per PATCH call (they're mutually exclusive `oneOf` branches in openapi.yaml) —
// resolutionCode/cause/closeNotes are the exception, allowed alongside `state` only.
export interface CasePatchPayloadDto {
  state?: CaseState;
  severity?: CaseSeverity;
  workState?: NonNullable<CaseWorkState>;
  assigneeEmail?: string;
  resolutionCode?: CaseResolutionCode;
  cause?: CaseCause;
  closeNotes?: string;
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

// Mirrors the webapp's BeCaseCreatePayload (the "case" type variant only — service_request and
// security_report_analysis creation aren't in the microapp's scope, see NewCasePage.tsx).
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

export interface CreatedCaseDto {
  id: string;
}
