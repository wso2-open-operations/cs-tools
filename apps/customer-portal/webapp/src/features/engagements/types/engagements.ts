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

import type { ProjectCasesStats } from "@features/support/types/cases";
import type {
  AllCasesFilterValues,
  CaseListItem,
  CaseMetadataResponse,
} from "@features/support/types/cases";
import { SortOrder } from "@/types/common";
import type { ChangeEvent } from "react";

/** Sort field for engagements list API (`sortBy.field`). */
export enum EngagementsSortField {
  CreatedOn = "createdOn",
  UpdatedOn = "updatedOn",
  Severity = "severity",
  State = "state",
}

/** Keys mapped from {@link ProjectCasesStats} for engagement stat cards. */
export type EngagementsStatKey = "total" | "active" | "completed" | "onHold";

export type EngagementsStatCardsProps = {
  stats?: ProjectCasesStats;
  isLoading: boolean;
  isError: boolean;
};

export type EngagementsSortOption = {
  value: EngagementsSortField;
  label: string;
};

export type EngagementsListSectionProps = {
  searchTerm: string;
  onSearchChange: (value: string) => void;
  isFiltersOpen: boolean;
  onFiltersToggle: () => void;
  filters: AllCasesFilterValues;
  filterMetadata: CaseMetadataResponse | undefined;
  onFilterChange: (field: string, value: string) => void;
  onClearFilters: () => void;
  excludeS0: boolean;
  restrictSeverityToLow: boolean;
  isProjectContextLoading: boolean;
  sortField: EngagementsSortField;
  onSortFieldChange: (value: string) => void;
  sortOrder: SortOrder;
  onSortOrderChange: (value: SortOrder) => void;
  paginatedCases: CaseListItem[];
  isCasesAreaLoading: boolean;
  isCasesError: boolean;
  listHasRefinement: boolean;
  totalItems: number;
  onCaseClick: ((caseItem: CaseListItem) => void) | undefined;
  totalPages: number;
  page: number;
  onPageChange: (event: ChangeEvent<unknown>, value: number) => void;
};
