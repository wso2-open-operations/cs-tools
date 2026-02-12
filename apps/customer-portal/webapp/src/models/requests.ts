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
  pagination?: PaginationRequest;
}

// Request body for searching cases.
export interface CaseSearchRequest {
  filters?: {
    issueId?: number;
    deploymentId?: string;
    severityId?: number;
    statusId?: number;
  };
  pagination: PaginationRequest;
  sortBy?: {
    field: string;
    order: "asc" | "desc";
  };
}

// Request body for case classification.
export interface CaseClassificationRequest {
  chatHistory: string;
  environments: string[];
  productDetails: string[];
  region: string;
  tier: string;
}

// Request body for creating a support case (POST /cases).
export interface CreateCaseRequest {
  deploymentId: string;
  description: string;
  issueTypeKey: number;
  productId: string;
  projectId: string;
  severityKey: number;
  title: string;
}

// Request body for posting a case attachment (POST /cases/:caseId/attachments).
export interface PostCaseAttachmentRequest {
  referenceType: "case";
  name: string;
  type: string;
  content: string;
}
