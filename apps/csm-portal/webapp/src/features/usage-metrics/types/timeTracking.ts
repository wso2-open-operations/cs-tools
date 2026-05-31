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

import { type TimeTrackingBadgeType } from "@features/project-details/constants/projectDetailsConstants";
import type {
  IdLabelRef,
  PaginationResponse,
  SearchRequestBase,
} from "@/types/common";

// Response type for project time tracking statistics.
export type ProjectTimeTrackingStats = {
  totalHours: number;
  billableHours: number;
  nonBillableHours: number;
};

// Item type for a time tracking log badge.
export type TimeTrackingLogBadge = {
  text: string;
  type: TimeTrackingBadgeType;
};

// Item type for a single time tracking log.
export type TimeTrackingLog = {
  id: string;
  badges: TimeTrackingLogBadge[];
  description: string | null;
  user: string | null;
  role: string | null;
  date: string | null;
  hours: number | null;
};

// Response type for project time tracking details.
export type TimeTrackingDetailsResponse = {
  timeLogs: TimeTrackingLog[];
};

// Item type for a time card from projects/:projectId/time-cards/search.
export type TimeCard = {
  id: string;
  totalTime: number;
  createdOn: string;
  hasBillable: boolean;
  state: IdLabelRef | null;
  approvedBy: IdLabelRef | null;
  reportedBy: IdLabelRef | null;
  project: IdLabelRef | null;
  case: IdLabelRef & { number: string };
};

// Response type for project time cards search results.
export type TimeCardSearchResponse = PaginationResponse & {
  timeCards: TimeCard[];
};

// Filter type for project time cards search filters.
export type TimeCardSearchFilters = {
  startDate?: string;
  endDate?: string;
  states?: string[];
};

// Request type for project time cards search.
export type TimeCardSearchRequest = SearchRequestBase & {
  filters?: TimeCardSearchFilters;
};

// Case summary nested inside a CaseTimeCard.
export type CaseTimeCardCaseRef = {
  id: string;
  number: string;
  name: string;
  updatedOn: string;
  project: { id: string; label: string };
};

// Billable / non-billable breakdown in a CaseTimeCard.
export type CaseTimeCardBucket = {
  totalTime: number;
  count: number;
};

// Item type for a case time card from projects/:projectId/cases/time-cards/search.
export type CaseTimeCard = {
  case: CaseTimeCardCaseRef;
  totalTime: number;
  totalCount: number;
  billable: CaseTimeCardBucket;
  nonBillable: CaseTimeCardBucket;
};

// Response type for case time cards search results.
export type CaseTimeCardSearchResponse = PaginationResponse & {
  caseTimeCards: CaseTimeCard[];
};
