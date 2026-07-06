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

import { DashboardCasesViewMode } from "@features/dashboard/types/casesTable";

// Lowercase substring for labels treated like resolved for status coloring
export const CASE_TABLE_RESOLVED_LABEL_SUBSTRING = "resolved";

// Normalized case-state label meaning “closed” for filter and default status logic.
export const CASE_TABLE_CLOSED_STATUS_NORMALIZED = "closed";

// Default status id fallbacks when metadata has no open statuses.
export const CASE_TABLE_OUTSTANDING_STATUS_IDS = [
  1, 10, 18, 1003, 1006,
] as const;

// Cases table header title.
export const CASES_TABLE_HEADER_TITLE = "Outstanding Support Cases";

// Cases table header subtitle.
export const CASES_TABLE_HEADER_SUBTITLE =
  "Track and manage all active support tickets";

// Cases table column header for creator.
export const CASES_TABLE_HEADER_CREATED_BY = "Created by";

// Cases table button filters.
export const CASES_TABLE_BUTTON_FILTERS = "Filters";

// Cases table button create.
export const CASES_TABLE_BUTTON_CREATE = "Create";

// Cases table clear filters label.
export const CASES_TABLE_CLEAR_FILTERS_LABEL = "Clear Filters";

// TabBar options for my vs all outstanding cases (dashboard).
export const DASHBOARD_CASES_VIEW_TABS = [
  { id: DashboardCasesViewMode.MyCases, label: "My Cases" },
  { id: DashboardCasesViewMode.AllCases, label: "All Cases" },
] as const;
