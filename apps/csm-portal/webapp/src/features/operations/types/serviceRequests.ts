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

import type { CaseListItem } from "@features/support/types/cases";
import type { MetadataItem, PaginationResponse } from "@/types/common";

/** Route segment for operations vs support navigation (`/projects/:id/operations/...` vs `.../support/...`). */
export enum OperationsNavSegment {
  Operations = "operations",
  Support = "support",
}

/** Case list sort field for the service requests page (`useGetProjectCases`). */
export enum ServiceRequestCaseSortField {
  CreatedOn = "createdOn",
  UpdatedOn = "updatedOn",
  Severity = "severity",
  State = "state",
}

export type ServiceRequestsListProps = {
  serviceRequests: CaseListItem[];
  isLoading: boolean;
  hasListRefinement?: boolean;
  onServiceRequestClick?: (sr: CaseListItem) => void;
};

// --- Catalogs & create-service-request payload (service request creation flow) ---

/** Item type for a catalog item within a catalog. */
export type CatalogItem = MetadataItem;

/** Model type for a catalog with its items. */
export type Catalog = {
  id: string;
  name: string;
  catalogItems: MetadataItem[];
};

/** Response type for catalogs search results. */
export type CatalogSearchResponse = PaginationResponse & {
  catalogs: Catalog[];
};

/** Item type for a variable definition for a catalog item. */
export type CatalogItemVariable = {
  id: string;
  questionText: string;
  order: number;
  type: string;
};

/** Response type for catalog item variables. */
export type CatalogItemVariablesResponse = {
  variables: CatalogItemVariable[];
};

/** Item type for a service request variable. */
export type ServiceRequestVariable = {
  id: string;
  value: string;
};

/** Item type for a service request attachment. */
export type ServiceRequestAttachment = {
  name: string;
  file: string;
};

/** Request type for creating a service request. */
export type CreateServiceRequestPayload = {
  type: "service_request";
  projectId: string;
  deploymentId: string;
  deployedProductId: string;
  catalogId: string;
  catalogItemId: string;
  variables: ServiceRequestVariable[];
  attachments?: ServiceRequestAttachment[];
};
