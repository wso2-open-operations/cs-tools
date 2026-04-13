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


// Model type for a label reference with an ID.
export type IdLabelRef = {
  id: string;
  label: string;
  count?: number;
  number?: string | null;
  name?: string | null;
  abbreviation?: string | null;
};

// Model type for metadata items (status, severity, type, etc.).
export type MetadataItem = {
  id: string;
  label: string;
};

// Model type for common audit metadata fields.
export type AuditMetadata = {
  createdOn?: string | null;
  updatedOn?: string | null;
  createdBy?: string | null;
  updatedBy?: string | null;
};

// Enum for trend direction.
export enum TrendDirection {
  UP = "up",
  DOWN = "down",
}

// Enum for trend color.
export enum TrendColor {
  SUCCESS = "success",
  ERROR = "error",
  INFO = "info",
  WARNING = "warning",
}

// Model type for trend data.
export type TrendData = {
  value: string;
  direction: TrendDirection;
  color: TrendColor;
};

// Request type for pagination metadata in search requests.
export type PaginationRequest = {
  offset?: number;
  limit?: number;
};

// Response type for pagination metadata echoed back on list responses.
export type PaginationResponse = {
  offset: number;
  limit: number;
  totalRecords: number;
};

// Enum for sort order used in list/search request payloads.
export enum SortOrder {
  ASC = "asc",
  DESC = "desc",
}

// Request type for sort-by clause used in list/search request payloads.
export type SortBy = {
  field: string;
  order: SortOrder;
};

// Request type for base search request wrapper.
export type SearchRequestBase = {
  pagination?: PaginationRequest;
  sortBy?: SortBy;
};

// Model type for shared environment context for conversations and case classification APIs.
export type SharedEnvContext = {
  envProducts: Record<string, string[]>;
  region: string;
  tier: string;
};

