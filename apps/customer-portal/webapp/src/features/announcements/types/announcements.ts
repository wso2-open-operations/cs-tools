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

import type { CaseDetails, CaseListItem, CaseMetadataResponse } from "@features/support/types/cases";
import type { AnnouncementFilterValues } from "@features/support/constants/supportConstants";
import { ANNOUNCEMENT_FILTER_DEFINITIONS } from "@features/support/constants/supportConstants";

export type { AnnouncementFilterValues };

/** API `sortBy.field` for announcements list. */
export enum AnnouncementSortField {
  CreatedOn = "createdOn",
  State = "state",
}

/**
 * Keys from {@link CaseMetadataResponse} used when building announcement filter dropdowns.
 */
export enum AnnouncementFilterMetadataKey {
  CaseStates = "caseStates",
  Severities = "severities",
}

/** Single row in `ListResultsBar` sort dropdown for announcements. */
export type AnnouncementSortOption = {
  value: AnnouncementSortField;
  label: string;
  kind?: "chronological" | "ordinal";
};

/** Label/value pair for announcement filter dropdowns (`ListFiltersPanel`, `AnnouncementsFilters`). */
export type AnnouncementFilterOption = {
  label: string;
  value: string;
};

/** Shape of entries in {@link ANNOUNCEMENT_FILTER_DEFINITIONS}. */
export type AnnouncementFilterDefinition =
  (typeof ANNOUNCEMENT_FILTER_DEFINITIONS)[number];

export type AnnouncementListProps = {
  cases: CaseListItem[];
  isLoading: boolean;
  hasListRefinement?: boolean;
  onCaseClick?: (caseItem: CaseListItem) => void;
};

export type AnnouncementsFiltersProps = {
  filters: AnnouncementFilterValues;
  filterMetadata: CaseMetadataResponse | undefined;
  onFilterChange: (field: string, value: string) => void;
  disabled?: boolean;
};

export type AnnouncementsSearchBarProps = {
  searchTerm: string;
  onSearchChange: (value: string) => void;
  isFiltersOpen: boolean;
  onFiltersToggle: () => void;
  filters: AnnouncementFilterValues;
  filterMetadata: CaseMetadataResponse | undefined;
  onFilterChange: (field: string, value: string) => void;
  onClearFilters: () => void;
  filtersDisabled?: boolean;
};

export type AnnouncementDetailsPanelProps = {
  data: CaseDetails | undefined;
  isLoading: boolean;
  isError: boolean;
  onBack: () => void;
  projectId?: string;
  caseId?: string;
};
