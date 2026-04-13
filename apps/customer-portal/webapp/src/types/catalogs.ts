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

import type { PaginationResponse } from "./common";

// Item type for a catalog item within a catalog.
export type CatalogItem = {
  id: string;
  label: string;
}

// Model type for a catalog with its items.
export type Catalog = {
  id: string;
  name: string;
  catalogItems: CatalogItem[];
}

// Response type for catalogs search results.
export type CatalogSearchResponse = PaginationResponse & {
  catalogs: Catalog[];
};

// Item type for a variable definition for a catalog item.
export type CatalogItemVariable = {
  id: string;
  questionText: string;
  order: number;
  type: string;
}

// Response type for catalog item variables.
export type CatalogItemVariablesResponse = {
  variables: CatalogItemVariable[];
}

// Item type for a service request variable.
export type ServiceRequestVariable = {
  id: string;
  value: string;
};

// Item type for a service request attachment.
export type ServiceRequestAttachment = {
  name: string;
  file: string;
};

// Request type for creating a service request.
export type CreateServiceRequestPayload = {
  type: "service_request";
  projectId: string;
  deploymentId: string;
  deployedProductId: string;
  catalogId: string;
  catalogItemId: string;
  variables: ServiceRequestVariable[];
  attachments?: ServiceRequestAttachment[];
}
