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
  /** Every record that shares this vulnerability's CVE (one per product / version / component). */
  affectedProducts?: ProductVulnerability[];
  isAffectedProductsLoading?: boolean;
  isAffectedProductsError?: boolean;
  /** Set when only part of the CVE's records could be loaded (safety cap). */
  affectedProductsTruncation?: AffectedProductsTruncation;
  onAffectedProductClick?: (vulnerability: ProductVulnerability) => void;
};

/** Every record of one vulnerability for the same product and product version. */
export type AffectedProductGroup = {
  /** Stable identity of the product + product version (normalised). */
  key: string;
  productName: string;
  productVersion: string;
  /** Distinct update levels of the group's records (normally just one). */
  updateLevels: string[];
  /** One record per affected component, in the order they were supplied. */
  records: ProductVulnerability[];
};

/** How many of a CVE's records were loaded versus how many exist. */
export type AffectedProductsTruncation = {
  loaded: number;
  total: number;
};

export type VulnerabilityAffectedProductsProps = {
  affectedProducts: ProductVulnerability[];
  currentId?: string;
  isLoading?: boolean;
  isError?: boolean;
  truncation?: AffectedProductsTruncation;
  onRowClick?: (vulnerability: ProductVulnerability) => void;
  /**
   * The user's expand/collapse choices by group key. Pass it together with
   * `onToggleGroup` to keep the choices in a parent that outlives this component
   * (the details page unmounts the section while the next record loads); leave
   * both out and the component keeps them itself.
   */
  toggledGroups?: Record<string, boolean>;
  onToggleGroup?: (groupKey: string, expanded: boolean) => void;
};

// Response type for product vulnerabilities metadata.
// One product with the versions it has vulnerability records for (exact stored strings).
export type VulnerabilitiesMetaProduct = {
  name: string;
  versions: string[];
};

export type VulnerabilitiesMetaResponse = {
  severities: IdLabelRef[];
  products: VulnerabilitiesMetaProduct[];
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
  /** Exact CVE id (case-insensitive); only that CVE's records are returned. */
  cveId?: string;
};

// Request type for searching product vulnerabilities.
export type ProductVulnerabilitiesSearchRequest = SearchRequestBase & {
  filters?: ProductVulnerabilitiesSearchFilters;
};
