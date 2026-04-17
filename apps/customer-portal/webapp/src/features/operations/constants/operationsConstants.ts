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

import {
  Activity,
  Bell,
  Calendar as CalendarIcon,
  CircleCheck,
  FileText,
} from "@wso2/oxygen-ui-icons-react";
import type { CaseMetadataResponse } from "@features/support/types/cases";
import type { ChangeRequestFilterValues } from "@features/operations/types/changeRequests";
import type { SupportStatConfig } from "@features/support/constants/supportConstants";
import { ChangeRequestsViewMode } from "@features/operations/types/changeRequests";
import { ServiceRequestCaseSortField } from "@features/operations/types/serviceRequests";

/**
 * Marketing bullet points for the Change Request card on Support / Operations.
 */
export const CHANGE_REQUEST_BULLET_ITEMS = [
  "Formal approval process",
  "Scheduled implementation",
  "Customer review and approval",
  "Rollback capabilities",
  "Calendar visualization",
  "Complete audit trail",
  "Impact and risk assessment",
  "Post implementation verification",
] as const;

/**
 * Change Request Status types.
 */
export const ChangeRequestStatus = {
  SCHEDULED: "Scheduled",
  IN_PROGRESS: "In Progress",
  COMPLETED: "Completed",
  CANCELLED: "Cancelled",
  PENDING_APPROVAL: "Pending Approval",
} as const;

export type ChangeRequestStatus =
  (typeof ChangeRequestStatus)[keyof typeof ChangeRequestStatus];

/**
 * Change Request Impact types.
 */
export const ChangeRequestImpact = {
  HIGH: "High",
  MEDIUM: "Medium",
  LOW: "Low",
} as const;

export type ChangeRequestImpact =
  (typeof ChangeRequestImpact)[keyof typeof ChangeRequestImpact];

/**
 * Change Request State labels.
 */
export const ChangeRequestStates = {
  NEW: "New",
  ASSESS: "Assess",
  AUTHORIZE: "Authorize",
  CUSTOMER_APPROVAL: "Customer Approval",
  SCHEDULED: "Scheduled",
  IMPLEMENT: "Implement",
  REVIEW: "Review",
  CUSTOMER_REVIEW: "Customer Review",
  ROLLBACK: "Rollback",
  CLOSED: "Closed",
  CANCELED: "Canceled",
} as const;

export type ChangeRequestState =
  (typeof ChangeRequestStates)[keyof typeof ChangeRequestStates];

/**
 * Canonical workflow order (used by progress UI and PDF).
 */
export const CHANGE_REQUEST_STATE_ORDER: ChangeRequestState[] = [
  ChangeRequestStates.NEW,
  ChangeRequestStates.ASSESS,
  ChangeRequestStates.AUTHORIZE,
  ChangeRequestStates.CUSTOMER_APPROVAL,
  ChangeRequestStates.SCHEDULED,
  ChangeRequestStates.IMPLEMENT,
  ChangeRequestStates.REVIEW,
  ChangeRequestStates.CUSTOMER_REVIEW,
  ChangeRequestStates.ROLLBACK,
  ChangeRequestStates.CLOSED,
  ChangeRequestStates.CANCELED,
];

/**
 * Maps stable API state ids to canonical change request state labels.
 */
export const CHANGE_REQUEST_STATE_API_ID_TO_LABEL: Record<
  string,
  ChangeRequestState
> = {
  "-5": ChangeRequestStates.NEW,
  "-4": ChangeRequestStates.ASSESS,
  "-3": ChangeRequestStates.AUTHORIZE,
  "5": ChangeRequestStates.CUSTOMER_APPROVAL,
  "-2": ChangeRequestStates.SCHEDULED,
  "-1": ChangeRequestStates.IMPLEMENT,
  "0": ChangeRequestStates.REVIEW,
  "1": ChangeRequestStates.CUSTOMER_REVIEW,
  "2": ChangeRequestStates.ROLLBACK,
  "3": ChangeRequestStates.CLOSED,
  "4": ChangeRequestStates.CANCELED,
};

/**
 * Change Request Impact labels.
 */
export const ChangeRequestImpactLabels = {
  HIGH: "1 - High",
  MEDIUM: "2 - Medium",
  LOW: "3 - Low",
} as const;

/**
 * Valid keys for change requests statistics.
 */
export type ChangeRequestStatKey =
  | "totalRequests"
  | "awaitingYourAction"
  | "ongoing"
  | "completed";

/**
 * Configuration for the change requests statistics cards.
 */
export const CHANGE_REQUEST_STAT_CONFIGS: SupportStatConfig<ChangeRequestStatKey>[] =
  [
    {
      icon: FileText,
      iconColor: "info",
      key: "totalRequests",
      label: "Total Requests",
    },
    {
      icon: Bell,
      iconColor: "warning",
      key: "awaitingYourAction",
      label: "Awaiting Your Action",
    },
    {
      icon: Activity,
      iconColor: "primary",
      key: "ongoing",
      label: "Ongoing",
    },
    {
      icon: CircleCheck,
      iconColor: "success",
      key: "completed",
      label: "Completed",
    },
  ];

/**
 * Change request filter definitions.
 */
export const CHANGE_REQUEST_FILTER_DEFINITIONS: Array<{
  filterKey: keyof ChangeRequestFilterValues;
  id: string;
  metadataKey: keyof CaseMetadataResponse;
}> = [
  {
    filterKey: "stateId",
    id: "state",
    metadataKey: "changeRequestStates",
  },
  {
    filterKey: "impactId",
    id: "impact",
    metadataKey: "changeRequestImpacts",
  },
];

export const DAYS_OF_WEEK = [
  "Sun",
  "Mon",
  "Tue",
  "Wed",
  "Thu",
  "Fri",
  "Sat",
] as const;

export const CHANGE_REQUEST_CALENDAR_WEEKDAY_LABELS = DAYS_OF_WEEK;

export const OPERATIONS_LIST_PAGE_SIZE = 10;

/** Shared list header back control. */
export const OPERATIONS_LIST_BACK_LABEL = "Back";

// --- Change requests page ----------------------------------------------------

export const CHANGE_REQUESTS_PAGE_TITLE = "All Change Requests";

export const CHANGE_REQUESTS_PAGE_DESCRIPTION =
  "Track and manage deployment changes and updates";

export const CHANGE_REQUESTS_SEARCH_PLACEHOLDER =
  "Search change requests by number, title, or description...";

export const CHANGE_REQUESTS_ENTITY_LABEL = "change requests";

export const CHANGE_REQUESTS_VIEW_TAB_LIST_LABEL = "List View";

export const CHANGE_REQUESTS_VIEW_TAB_CALENDAR_LABEL = "Calendar View";

export const CHANGE_REQUESTS_EXPORT_EXPORTING_LABEL = "Exporting...";

export const CHANGE_REQUESTS_EXPORT_SCHEDULE_LABEL = "Export Schedule";

/** Tab config for `TabBar` (ids match {@link ChangeRequestsViewMode}). */
export const CHANGE_REQUESTS_VIEW_TABS_CONFIG = [
  {
    id: ChangeRequestsViewMode.List,
    label: CHANGE_REQUESTS_VIEW_TAB_LIST_LABEL,
    icon: FileText,
  },
  {
    id: ChangeRequestsViewMode.Calendar,
    label: CHANGE_REQUESTS_VIEW_TAB_CALENDAR_LABEL,
    icon: CalendarIcon,
  },
];

// --- Change requests list ----------------------------------------------------

export const CHANGE_REQUESTS_LIST_ERROR_MESSAGE =
  "Failed to load change requests. Please try again.";

export const CHANGE_REQUESTS_LIST_EMPTY_REFINED_MESSAGE =
  "No change requests found. Try adjusting your filters or search query.";

export const CHANGE_REQUESTS_LIST_EMPTY_DEFAULT_MESSAGE =
  "No change requests yet.";

export const CHANGE_REQUESTS_LIST_PLACEHOLDER = "Not Available";

export const CHANGE_REQUESTS_LIST_SR_PREFIX = "SR:";

export const CHANGE_REQUESTS_LIST_CREATED_PREFIX = "Created:";

export const CHANGE_REQUESTS_LIST_SERVICE_OUTAGE_LABEL = "Service Outage";

/** Empty-state / error illustration width (px). */
export const OPERATIONS_LIST_EMPTY_ILLUSTRATION_WIDTH_PX = 200;

/** Empty-state icon bottom margin (px). */
export const OPERATIONS_LIST_EMPTY_ICON_MARGIN_BOTTOM_PX = 16;

/** Empty-state vertical padding (MUI spacing). */
export const OPERATIONS_LIST_EMPTY_CONTAINER_PY = 6;

// --- Service requests list ---------------------------------------------------

export const SERVICE_REQUESTS_LIST_EMPTY_REFINED_MESSAGE =
  "No service requests found. Try adjusting your filters or search query.";

export const SERVICE_REQUESTS_LIST_EMPTY_DEFAULT_MESSAGE =
  "No service requests yet.";

// --- Service requests page ---------------------------------------------------

export const SERVICE_REQUESTS_PAGE_TITLE_ALL = "All Service Requests";

export const SERVICE_REQUESTS_PAGE_TITLE_MINE = "My Service Requests";

export const SERVICE_REQUESTS_PAGE_DESCRIPTION_ALL =
  "Manage deployments, operations, infrastructure change, and service configurations";

export const SERVICE_REQUESTS_PAGE_DESCRIPTION_MINE =
  "Manage and track your service requests";

export const SERVICE_REQUESTS_SEARCH_PLACEHOLDER =
  "Search service requests by ID, title, or description...";

export const SERVICE_REQUESTS_ENTITY_LABEL = "service requests";

/** `ListStatGrid` singular entity label for SR stats. */
export const SERVICE_REQUESTS_STAT_ENTITY_NAME = "service request";

export const SERVICE_REQUESTS_NEW_BUTTON_LABEL = "New Service Request";

export const SERVICE_REQUESTS_ERROR_ENTITY_NAME = "service requests";

export type ServiceRequestSortFieldOption = {
  value: ServiceRequestCaseSortField;
  label: string;
};

export const SERVICE_REQUESTS_SORT_FIELD_OPTIONS: ServiceRequestSortFieldOption[] =
  [
    {
      value: ServiceRequestCaseSortField.CreatedOn,
      label: "Created on",
    },
    {
      value: ServiceRequestCaseSortField.UpdatedOn,
      label: "Updated on",
    },
    {
      value: ServiceRequestCaseSortField.Severity,
      label: "Severity",
    },
    {
      value: ServiceRequestCaseSortField.State,
      label: "State",
    },
  ];

// --- Operations hub page -----------------------------------------------------

export const OPERATIONS_HUB_PROJECT_ERROR_MESSAGE =
  "Unable to load project details. Please try again later.";

export const OPERATIONS_HUB_STAT_ENTITY_NAME = "operations";

export const OPERATIONS_HUB_CARD_TITLE_SR = "Service Requests";

export const OPERATIONS_HUB_CARD_TITLE_CR = "Change Requests";

export const OPERATIONS_HUB_HEADER_ACTION_CREATE_SR =
  "Create Service Request";

export const OPERATIONS_HUB_FOOTER_VIEW_MINE = "View my requests";

export const OPERATIONS_HUB_FOOTER_VIEW_ALL_SR = "View all requests";

export const OPERATIONS_HUB_FOOTER_VIEW_ALL_CR = "View all change requests";


/** Legend row order in the calendar view (matches `ChangeRequestStates`). */
export const CHANGE_REQUEST_CALENDAR_LEGEND_STATES: ChangeRequestState[] = [
  ChangeRequestStates.NEW,
  ChangeRequestStates.ASSESS,
  ChangeRequestStates.AUTHORIZE,
  ChangeRequestStates.CUSTOMER_APPROVAL,
  ChangeRequestStates.SCHEDULED,
  ChangeRequestStates.IMPLEMENT,
  ChangeRequestStates.REVIEW,
  ChangeRequestStates.CUSTOMER_REVIEW,
  ChangeRequestStates.ROLLBACK,
  ChangeRequestStates.CLOSED,
  ChangeRequestStates.CANCELED,
];
