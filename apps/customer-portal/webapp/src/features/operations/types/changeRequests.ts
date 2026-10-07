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

import type { CaseMetadataResponse } from "@features/support/types/cases";
import type {
  AuditMetadata,
  IdLabelRef,
  PaginationResponse,
  SearchRequestBase,
} from "@/types/common";

// Item type for a change request.
export type ChangeRequestItem = AuditMetadata & {
  id: string;
  internalId?: string | null;
  number: string;
  title: string;
  description?: string | null;
  project: IdLabelRef | null;
  case: IdLabelRef | null;
  deployment: IdLabelRef | null;
  deployedProduct: IdLabelRef | null;
  product: IdLabelRef | null;
  assignedEngineer: IdLabelRef | null;
  assignedTeam: IdLabelRef | null;
  startDate: string;
  endDate: string;
  duration: string | null;
  hasServiceOutage: boolean;
  impact: IdLabelRef | null;
  state: IdLabelRef | null;
  type: IdLabelRef | null;
};

// Response type for detailed change request information.
export type ChangeRequestDetails = ChangeRequestItem & {
  description: string | null;
  createdBy: string;
  justification: string | null;
  impactDescription: string | null;
  serviceOutage: string | null;
  communicationPlan: string | null;
  rollbackPlan: string | null;
  testPlan: string | null;
  hasCustomerApproved: boolean;
  hasCustomerReviewed: boolean;
  /**
   * Viewer-specific: true only when the signed-in customer has a pending answer
   * on this change request right now; false when they do not (already answered,
   * not asked, not a contact). Omitted when the data source cannot say (the
   * legacy ServiceNow source); the page then falls back to `hasCustomerApproved`
   * at Customer Approval and always offers the review answer at Customer Review.
   */
  customerCanAnswer?: boolean;
  /**
   * True while WSO2 has the change request on hold (the reason is not shared).
   * A held change refuses a proposed time, not an answer, so the page turns
   * Propose New Time off, and says why. Omitted when the data source cannot say,
   * which is not the same as "not held".
   */
  isOnHold?: boolean;
  approvedBy: IdLabelRef | null;
  approvedOn: string | null;
};

// Response type for change request search results.
export type ChangeRequestSearchResponse = PaginationResponse & {
  changeRequests: ChangeRequestItem[];
};

// Response type for change request statistics.
export type ChangeRequestStats = {
  totalRequests: number;
  awaitingYourAction: number;
  ongoing: number;
  completed: number;
};

// Item type for change request state count.
export type ChangeRequestStateCount = IdLabelRef & { count: number };

// Response type for change request statistics breakdown.
export type ChangeRequestStatsResponse = {
  totalCount: number;
  activeCount?: number;
  outstandingCount?: number;
  actionRequiredCount?: number;
  stateCount: ChangeRequestStateCount[];
  resolvedCount: {
    total: number;
    currentMonth: number;
    pastThirtyDays: number;
  };
};

// Response type for patching a change request.
export type PatchChangeRequestResponse = AuditMetadata & {
  id: string;
};

// Model type for change request filters state.
export type ChangeRequestFilterValues = {
  stateIds?: string[];
  impactIds?: string[];
};

/** Change request list sort field for search API `sortBy.field`. */
export enum ChangeRequestSortField {
  UpdatedOn = "updatedOn",
  CreatedOn = "createdOn",
}

// Filter type for searching change requests.
export type ChangeRequestSearchFilters = {
  impactKeys?: number[];
  searchQuery?: string;
  stateKeys?: number[];
  closedStartDate?: string;
  closedEndDate?: string;
};

// Request type for searching change requests.
export type ChangeRequestSearchRequest = SearchRequestBase & {
  filters?: ChangeRequestSearchFilters;
};

// Request type for patching a change request. The customer-portal backend takes
// either the customer's answer (isCustomerApproved / isCustomerReviewed) or a
// proposed window (plannedStartOn / plannedEndOn, "YYYY-MM-DD HH:MM:SS" in UTC),
// never both in one request. An answer also names the planned window the
// customer was looking at (expectedPlannedStartOn / expectedPlannedEndOn, as the
// details read them): it is then recorded only while that is still the window.
export type PatchChangeRequestRequest = {
  plannedStartOn?: string;
  plannedEndOn?: string;
  isCustomerApproved?: boolean;
  isCustomerReviewed?: boolean;
  expectedPlannedStartOn?: string;
  expectedPlannedEndOn?: string;
};

// Enum for change request decision mode.
export enum ChangeRequestDecisionMode {
  CUSTOMER_APPROVAL = "customerApproval",
  CUSTOMER_REVIEW = "customerReview",
  NONE = "none",
}

/**
 * Whether the customer can propose a new implementation time right now, as the
 * page decides it (it is offered at Customer Approval only, and switched off
 * while WSO2 has the change on hold). Wording that points a customer at Propose
 * New Time must follow it, so that it never points at an action that is off.
 */
export type ProposeNewTimeAvailability =
  /** The Propose New Time button is on. */
  | "available"
  /** It is offered but switched off because WSO2 has the change on hold. */
  | "on_hold"
  /** It is not offered at all (Customer Review). */
  | "unavailable";

// Item type for a change request workflow stage.
export type ChangeRequestWorkflowStage = {
  name: string;
  description: string;
  completed: boolean;
  current: boolean;
  disabled: boolean;
};

// --- Change requests UI (list, calendar, filters) ---------------------------

/** List vs calendar on the change requests page. */
export enum ChangeRequestsViewMode {
  List = "list",
  Calendar = "calendar",
}

/**
 * `ListFiltersPanel` / `CHANGE_REQUEST_FILTER_DEFINITIONS` entry `id` values
 * (used when resolving metadata into select options).
 */
export enum ChangeRequestFilterDefinitionId {
  State = "state",
  Impact = "impact",
}

export type ChangeRequestsListProps = {
  changeRequests: ChangeRequestItem[];
  isLoading: boolean;
  isError?: boolean;
  hasListRefinement?: boolean;
  onChangeRequestClick?: (item: ChangeRequestItem) => void;
};

export type ChangeRequestsCalendarViewProps = {
  changeRequests: ChangeRequestItem[];
  isLoading: boolean;
  isError?: boolean;
  onChangeRequestClick?: (item: ChangeRequestItem) => void;
  legendStates?: { label: string }[];
};

export type ChangeRequestFilterOption = {
  label: string;
  value: string;
};

export type ChangeRequestsSearchBarProps = {
  searchTerm: string;
  onSearchChange: (value: string) => void;
  isFiltersOpen: boolean;
  onFiltersToggle: () => void;
  filters: ChangeRequestFilterValues;
  filterMetadata: CaseMetadataResponse | undefined;
  onFilterChange: (field: string, value: string | string[]) => void;
  onClearFilters: () => void;
};

export type ChangeRequestsStatCardsProps = {
  isLoading: boolean;
  isError?: boolean;
  stats: ChangeRequestStats | undefined;
};

export type ProposeNewImplementationTimeModalProps = {
  open: boolean;
  onClose: () => void;
  /** Called once a proposal has been accepted, just before the dialog closes. */
  onProposed?: () => void;
  /**
   * Called when the backend refused the proposal for good (the change request no
   * longer waits on this customer), just before the dialog closes: the form and
   * the button that opened it are gone, so whoever owns the page moves focus
   * somewhere stable. Not called for a refusal that keeps the dialog open.
   */
  onRefused?: () => void;
  changeRequest: ChangeRequestDetails | null;
};

export type ScheduledMaintenanceWindowCardProps = {
  changeRequest: ChangeRequestDetails | null;
};
