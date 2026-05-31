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

import type { ChangeEvent } from "react";
import type {
  IdLabelRef,
  MetadataItem,
  PaginationResponse,
  SearchRequestBase,
} from "@/types/common";
/** Security Center main tabs (URL `tab` query). */
export enum SecurityTabId {
  COMPONENTS = "components",
  VULNERABILITIES = "vulnerabilities",
}

/** Report type discriminator for security cases. */
export enum CaseReportType {
  SECURITY = "security",
}

/** Keys for `ListStatGrid` stats on the Security page. */
export enum SecurityStatKey {
  totalVulnerabilities = "totalVulnerabilities",
  activeSecurityReports = "activeSecurityReports",
  resolvedSecurityReports = "resolvedSecurityReports",
}

/** My Reports vs All Reports on Security Report Analysis. */
export enum SecurityReportViewMode {
  MY = "my",
  ALL = "all",
}

/** Sort field for security report cases list. */
export enum SecurityReportCaseSortField {
  createdOn = "createdOn",
  updatedOn = "updatedOn",
  severity = "severity",
  state = "state",
}

/** Normalized severity token for chip colour helpers (API labels lowercased). */
export enum VulnerabilitySeverityToken {
  CRITICAL = "critical",
  HIGH = "high",
  MEDIUM = "medium",
  LOW = "low",
}

/** Normalized status token for chip colour helpers. */
export enum VulnerabilityStatusToken {
  IN_PROGRESS = "in progress",
  OPEN = "open",
  RESOLVED = "resolved",
}

/** Severity dropdown option for product vulnerabilities filters. */
export type VulnerabilitySelectOption = {
  value: string | number;
  label: string;
};

// Item type for a product vulnerability.
export type ProductVulnerability = {
  id: string;
  cveId: string;
  vulnerabilityId: string;
  severity: MetadataItem;
  productName?: string;
  productVersion?: string;
  componentName: string;
  version: string;
  type: string;
  useCase: string | null;
  justification: string | null;
  resolution: string | null;
  componentType?: string;
  updateLevel?: string;
  status?: { id: string | number; label: string } | null;
};

export type ProductVulnerabilitiesTableProps = {
  onTotalRecordsChange?: (total: number) => void;
  onError?: (isError: boolean) => void;
  onVulnerabilityClick?: (vulnerability: { id: string }) => void;
};

export type ProductVulnerabilitiesFiltersProps = {
  filters: Record<string, string | number>;
  severityOptions?: VulnerabilitySelectOption[];
  productOptions?: VulnerabilitySelectOption[];
  productVersionOptions?: VulnerabilitySelectOption[];
  onFilterChange: (field: string, value: string | number) => void;
  onClearFilters: () => void;
};

export type ProductVulnerabilitiesListData = {
  vulnerabilities: ProductVulnerability[];
  totalRecords: number;
};

export type ProductVulnerabilitiesListProps = {
  isLoading: boolean;
  isError?: boolean;
  data: ProductVulnerabilitiesListData | undefined;
  page: number;
  rowsPerPage: number;
  onPageChange: (event: unknown, newPage: number) => void;
  onRowsPerPageChange: (event: ChangeEvent<HTMLInputElement>) => void;
  onVulnerabilityClick?: (vulnerability: ProductVulnerability) => void;
};

export type ProductVulnerabilitiesTableHeaderProps = {
  searchValue: string;
  onSearchChange: (value: string) => void;
  onFilterToggle: () => void;
  isFiltersOpen: boolean;
  activeFiltersCount: number;
};

export type VulnerabilityDetailsContentProps = {
  data: ProductVulnerability | undefined;
  isLoading: boolean;
  isError: boolean;
  onBack: () => void;
};

// Response type for product vulnerabilities metadata.
export type VulnerabilitiesMetaResponse = {
  severities: IdLabelRef[];
};

// Response type for product vulnerabilities search results.
export type ProductVulnerabilitiesSearchResponse = PaginationResponse & {
  productVulnerabilities: ProductVulnerability[];
};

// Filter type for product vulnerabilities search filters.
export type ProductVulnerabilitiesSearchFilters = {
  searchQuery?: string;
  severityId?: number;
  statusId?: number;
  productName?: string;
  productVersion?: string;
};

// Request type for searching product vulnerabilities.
export type ProductVulnerabilitiesSearchRequest = SearchRequestBase & {
  filters?: ProductVulnerabilitiesSearchFilters;
};
