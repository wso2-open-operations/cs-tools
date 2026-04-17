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

import type {
  CaseListItem,
  CaseSearchResponse,
} from "@features/support/types/cases";
import type { ChangeEvent } from "react";

// Select option.
export type SelectOption = {
  value: string;
  label: string;
};

export type TableFilter = {
  id: string;
  label: string;
  type: "select";
  options?: string[] | SelectOption[];
  placeholder?: string;
  onLoadMore?: () => void;
  hasMore?: boolean;
  isFetchingMore?: boolean;
};

// Case filters props.
export type CasesFiltersProps = {
  filters: Record<string, string | number | undefined>;
  filterFields: TableFilter[];
  onFilterChange: (field: string, value: string | number) => void;
};

// Case list props.
export type CasesListProps = {
  isLoading: boolean;
  isError?: boolean;
  data: CaseSearchResponse | undefined;
  page: number;
  rowsPerPage: number;
  onPageChange: (event: unknown, newPage: number) => void;
  onRowsPerPageChange: (event: ChangeEvent<HTMLInputElement>) => void;
  onCaseClick?: (caseItem: CaseListItem) => void;
  showPagination?: boolean;
  hasListRefinement?: boolean;
};

// Case table header props.
export type CasesTableHeaderProps = {
  activeFiltersCount: number;
  isFiltersOpen: boolean;
  onFilterToggle: () => void;
  hasAgent?: boolean;
};

// Case table skeleton props.
export type CasesTableSkeletonProps = {
  rowsPerPage: number;
};

// Case table props.
export type CasesTableProps = {
  projectId: string;
  excludeS0?: boolean;
  restrictSeverityToLow?: boolean;
  hasAgent?: boolean;
  includeDeploymentFilter?: boolean;
};

// Case table filter values.
export type CasesTableFilterValues = {
  [key: string]: string | number | undefined;
  statusId?: string | number;
  severityId?: string | number;
  issueTypes?: string | number;
  deploymentId?: string | number;
};

// Route params used by cases table header when `projectId` is read from the URL.
export type CasesTableRouteParams = {
  projectId?: string;
};

// Case table severity color path.
export enum CaseTableSeverityColorPath {
  S0 = "error.main",
  S1 = "warning.main",
  S2 = "info.main",
  S3 = "secondary.main",
  S4 = "success.main",
  Default = "text.secondary",
}

// Oxygen theme color paths grouped by case status semantics.
export enum CaseTableStatusColorPath {
  OpenOrReopened = "info.main",
  AwaitingInfo = "primary.main",
  WorkInProgress = "warning.main",
  SuccessTerminal = "success.main",
  Default = "text.secondary",
}

// Case table metadata key.
export enum CaseTableMetadataKey {
  Severities = "severities",
  CaseStates = "caseStates",
  IssueTypes = "issueTypes",
  DeploymentId = "deploymentId",
}