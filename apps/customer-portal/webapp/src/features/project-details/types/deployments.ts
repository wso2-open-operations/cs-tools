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
  AuditMetadata,
  IdLabelRef,
  PaginationResponse,
  SearchRequestBase,
} from "@/types/common";
import type { ProductUpdate } from "@features/project-details/types/products";

// Item type for a product deployed in an environment.
export type DeploymentProduct = {
  id: string;
  name: string;
  version: string;
  supportStatus: string;
  description: string;
  cores: number;
  tps: number;
  releasedDate: string;
  endOfLifeDate: string;
  updateLevel: string;
};

// Item type for a document attached to a deployment.
export type DeploymentDocument = {
  id: string;
  name: string;
  description?: string | null;
  category?: string;
  sizeBytes?: number;
  size?: number;
  uploadedAt?: string;
  createdOn?: string;
  uploadedBy?: string;
  createdBy?: string;
  content?: string | null;
  downloadUrl?: string;
  previewUrl?: string | null;
};

// Response type for GET /deployments/:deploymentId/attachments.
export type DeploymentAttachmentsResponse = PaginationResponse & {
  attachments: DeploymentDocument[];
};

// Response type for POST /deployments/:deploymentId/attachments.
export type PostDeploymentAttachmentResponse = AuditMetadata & {
  id: string;
  downloadUrl: string;
  size: number;
};

// Enum for deployment status.
export enum DeploymentStatus {
  HEALTHY = "Healthy",
  WARNING = "Warning",
}

// Model type for a single deployment environment.
export type Deployment = {
  id: string;
  name: string;
  status: DeploymentStatus;
  url: string;
  version: string;
  description: string;
  products: DeploymentProduct[];
  documents: DeploymentDocument[];
  deployedAt: string;
  uptimePercent: number;
};

// Response type for project deployments list.
export type ProjectDeploymentsListResponse = PaginationResponse & {
  deployments: ProjectDeploymentItem[];
};

// Item type for a single item from GET /projects/:projectId/deployments.
export type ProjectDeploymentItem = AuditMetadata & {
  id: string;
  number?: string | null;
  name: string;
  description: string | null;
  url: string | null;
  project: IdLabelRef;
  type: IdLabelRef;
  deployedProductCount?: number;
  instanceCount?: number;
};

// Response type for GET /deployments/:deploymentId/products.
export type DeployedProductsResponse = PaginationResponse & {
  deployedProducts: DeploymentProductItem[];
};

// Model type for union type for the deployment products endpoint response.
export type DeployedProductsResponsePayload =
  | DeploymentProductItem[]
  | DeployedProductsResponse;

// Item type for a deployment product instance.
export type DeploymentProductInstance = AuditMetadata & {
  id: string;
  instance: string;
  coreUsageCount?: number | null;
  updates?: number | null;
  jdkVersion?: string | null;
  customCreatedOn?: string | null;
  customUpdatedOn?: string | null;
};

// Item type for a single item from GET /deployments/:deploymentId/products.
export type DeploymentProductItem = AuditMetadata & {
  id: string;
  description: string | null;
  product: IdLabelRef;
  deployment: IdLabelRef;
  version?: IdLabelRef | string | null;
  cores?: number | null;
  tps?: number | null;
  updates?: ProductUpdate[] | null;
  instanceCount?: number;
  instances?: DeploymentProductInstance[] | null;
};

// Response type for creating a deployment.
export type CreateDeploymentResponse = AuditMetadata & {
  id: string;
};

// Item type for subscription data within license response.
export type SubscriptionData = {
  deploymentId: string;
  deploymentName: string;
  subscriptionKey: string;
  clientId: string;
  clientSecret: string;
  secrets: string;
};

// Response type for license details.
export type DeploymentLicense = {
  subscriptionData: SubscriptionData;
  signature: string;
};

// Request type for posting a deployment attachment.
export type PostDeploymentAttachmentRequest = {
  name: string;
  type: string;
  content: string;
  description?: string;
};

// Item type for deployment product update levels.
export type DeploymentProductUpdate = {
  date: string;
  details?: string;
  updateLevel: number;
};

// Request type for patching a deployment product.
export type PatchDeploymentProductRequest = {
  cores?: number;
  tps?: number;
  description?: string;
  active?: boolean;
  updates?: DeploymentProductUpdate[];
};

// Request type for posting a deployment product.
export type PostDeploymentProductRequest = {
  productId: string;
  versionId: string;
  projectId: string;
  cores?: number;
  tps?: number;
  description?: string;
};

// Request type for creating a deployment.
export type CreateDeploymentRequest = {
  deploymentTypeKey: number;
  description: string;
  name: string;
};

// Request type for patching a deployment.
export type PatchDeploymentRequest = {
  active?: boolean;
  description?: string;
  name?: string;
  typeKey?: number;
};

// Filter type for consumption statistics.
export type ConsumptionFilter = {
  include?: boolean;
  startDate?: string;
  endDate?: string;
};

// Filter type for searching deployments.
export type DeploymentSearchFilters = {
  consumption?: ConsumptionFilter;
};

// Request type for searching deployments.
export type DeploymentSearchRequest = SearchRequestBase & {
  filters?: DeploymentSearchFilters;
};

// Filter type for searching deployed products.
export type DeployedProductSearchFilters = {
  consumption?: ConsumptionFilter;
};

// Request type for searching deployed products.
export type DeployedProductSearchRequest = SearchRequestBase & {
  filters?: DeployedProductSearchFilters;
};

/** One deployment product row selected for quick service request creation. */
export type SelectedDeploymentProduct = {
  deploymentId: string;
  productItemId: string;
};
