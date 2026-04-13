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
  IdLabelRef,
  MetadataItem,
  PaginationResponse,
  SearchRequestBase,
} from "@/types/common";

// Item type for a product vulnerability.
export type ProductVulnerability = {
  id: string;
  cveId: string;
  vulnerabilityId: string;
  severity: MetadataItem;
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
};

// Request type for searching product vulnerabilities.
export type ProductVulnerabilitiesSearchRequest = SearchRequestBase & {
  filters?: ProductVulnerabilitiesSearchFilters;
};

