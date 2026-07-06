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

export type GlobalSearchType = "projects" | "cases";

export interface GlobalSearchFilters {
  searchQuery?: string;
  types?: GlobalSearchType[];
}

export interface GlobalSearchPagination {
  offset: number;
  limit: number;
}

export interface GlobalSearchPayload {
  filters?: GlobalSearchFilters;
  projectsPagination?: GlobalSearchPagination;
  casesPagination?: GlobalSearchPagination;
}

export interface ReferenceItem {
  id: string;
  label: string;
}

export interface GlobalSearchProject {
  id: string;
  name: string;
  key: string;
  description?: string | null;
  type: ReferenceItem;
  createdOn: string;
  startDate?: string | null;
  endDate?: string | null;
  hasPdpSubscription: boolean;
  closureState?: string | null;
  account: ReferenceItem;
}

export interface GlobalSearchCase {
  id: string;
  internalId: string;
  number: string;
  title?: string | null;
  description?: string | null;
  createdOn: string;
  createdBy: string;
  updatedOn: string;
  project?: ReferenceItem | null;
  caseType?: ReferenceItem | null;
  state?: ReferenceItem | null;
  severity?: ReferenceItem | null;
  assignedEngineer?: ReferenceItem | null;
  account: ReferenceItem;
}

export interface GlobalSearchResponse {
  query: string;
  projectsTotal: number;
  casesTotal: number;
  projects: GlobalSearchProject[];
  cases: GlobalSearchCase[];
}
