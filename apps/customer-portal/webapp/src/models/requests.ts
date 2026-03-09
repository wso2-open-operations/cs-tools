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

// Pagination metadata for search requests.
export interface PaginationRequest {
  offset?: number;
  limit?: number;
}

// Request body for searching projects.
export interface SearchProjectsRequest {
  filters?: {
    searchQuery?: string;
  };
  pagination?: PaginationRequest;
}

// Request body for searching cases.
export interface CaseSearchRequest {
  filters?: {
    issueId?: number;
    deploymentId?: string;
    severityId?: number;
    statusId?: number;
    statusIds?: number[];
    searchQuery?: string;
    caseTypes?: string[];
    createdByMe?: boolean;
  };
  pagination: PaginationRequest;
  sortBy?: {
    field: string;
    order: "asc" | "desc";
  };
}

// Request body for POST /projects/:projectId/conversations/search.
export interface ConversationSearchRequest {
  filters?: {
    searchQuery?: string;
    stateKeys?: number[];
    createdByMe?: boolean;
  };
  pagination: PaginationRequest;
  sortBy?: {
    field: string;
    order: "asc" | "desc";
  };
}

// Request body for searching change requests (POST /projects/:projectId/change-requests/search).
export interface ChangeRequestSearchRequest {
  filters?: {
    impactKey?: number;
    searchQuery?: string;
    stateKeys?: number[];
  };
  pagination: PaginationRequest;
}

/** Shared env context for conversations and case classification APIs. */
export interface SharedEnvContext {
  envProducts: Record<string, string[]>;
  region: string;
  tier: string;
}

// Request body for PATCH /users/me (partial update, only changed fields).
export interface PatchUserMeRequest {
  phoneNumber?: string;
  timeZone?: string;
  firstName?: string;
  lastName?: string;
}

// Request body for case classification.
export interface CaseClassificationRequest extends SharedEnvContext {
  chatHistory: string;
}

// Request body for POST /projects/:projectId/conversations (Novera chat).
export interface ConversationRequest extends SharedEnvContext {
  message: string;
}

// Request body for PATCH /cases/:caseId (update case state).
export interface PatchCaseRequest {
  stateKey: number;
}

// Request body for creating a support case (POST /cases).
export interface CreateCaseRequest {
  attachments?: Array<{ file: string; name: string }>;
  type?: string;
  deploymentId: string;
  description: string;
  issueTypeKey?: number;
  deployedProductId: string;
  projectId: string;
  severityKey?: number;
  title: string;
  parentCaseId?: string;
  conversationId?: string;
}

// Request body for creating a service request (POST /cases with type: "service_request").
export interface CreateServiceRequestPayload {
  type: "service_request";
  projectId: string;
  deploymentId: string;
  deployedProductId: string;
  catalogId: string;
  catalogItemId: string;
  variables: { id: string; value: string }[];
  attachments?: Array<{ name: string; file: string }>;
}

// Request body for product vulnerabilities search.
export interface ProductVulnerabilitiesSearchRequest {
  filters?: {
    searchQuery?: string;
    severityId?: number;
    statusId?: number;
  };
  pagination?: PaginationRequest;
  sortBy?: {
    field: string;
    order: "asc" | "desc";
  };
}

// Request body for posting a case attachment.
export interface PostCaseAttachmentRequest {
  name: string;
  type: string;
  content: string;
  referenceType?: string;
}

// Request body for posting a deployment attachment (POST /deployments/:deploymentId/attachments).
export interface PostDeploymentAttachmentRequest {
  name: string;
  type: string;
  content: string;
}

// Request body for PATCH /deployments/:deploymentId/products/:productId.
export interface PatchDeploymentProductRequest {
  cores?: number;
  tps?: number;
  description?: string;
  active?: boolean;
}

// Request body for POST /deployments/:deploymentId/products.
export interface PostDeploymentProductRequest {
  productId: string;
  versionId: string;
  projectId: string;
  cores?: number;
  tps?: number;
  description?: string;
}

// Request body for POST /products/:productId/versions/search.
export interface ProductVersionsSearchRequest {
  pagination?: { limit?: number; offset?: number };
}

// Request body for creating a deployment.
export interface CreateDeploymentRequest {
  deploymentTypeKey: number;
  description: string;
  name: string;
}

// Request body for PATCH /projects/:projectId/deployments/:deploymentId.
export interface PatchDeploymentRequest {
  active?: boolean;
  description?: string;
  name?: string;
  typeKey?: number;
}

// Request body for creating a call request.
export interface CreateCallRequest {
  durationInMinutes: number;
  reason: string;
  utcTimes: string[];
}

// Request body for updating a call request (PATCH /cases/:caseId/call-requests/:id).
export interface PatchCallRequest {
  reason: string;
  stateKey: number;
  utcTimes?: string[];
}

// Request body for updating current user profile (PATCH /users/me).
export interface PatchUserMeRequest {
  phoneNumber?: string;
  timeZone?: string;
}

// Request body for creating a project contact (POST /projects/:projectId/contacts).
export interface CreateProjectContactRequest {
  contactEmail: string;
  contactFirstName: string;
  contactLastName: string;
  isCsIntegrationUser: boolean;
  isSecurityContact: boolean;
}

// Request body for POST /updates/levels/search.
export interface UpdateLevelsSearchRequest {
  startingUpdateLevel: number;
  endingUpdateLevel: number;
  productName: string;
  productVersion: string;
}

// Request body for project time cards search (POST /projects/:projectId/time-cards/search).
export interface TimeCardSearchRequest {
  filters?: {
    startDate?: string;
    endDate?: string;
    states?: string[];
  };
  pagination?: {
    limit?: number;
    offset?: number;
  };
}

// Request body for validating a project contact (POST /projects/:projectId/contacts/validate).
export interface ValidateContactRequest {
  contactEmail: string;
}

// Request body for PATCH /change-requests/:id (update planned start).
export interface PatchChangeRequestRequest {
  plannedStartOn: string;
}
