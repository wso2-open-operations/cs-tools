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

export interface Case {
  id: string;
  internalId: string;
  number: string;
  createdOn: Date;
  createdBy: string;
  updatedOn: Date;
  title: string;
  description: string;
  slaResponseTime: string;
  assigned?: string;
  statusId: string;
  severityId?: string;
  issueTypeId?: string;
  product?: string;
  reporter?: string;
  account?: string;
  parentCaseId?: string;
  conversationId?: string;
  deployment?: string;
}

export type CaseSummary = Omit<
  Case,
  "updatedOn" | "productId" | "reporter" | "account" | "parentCaseId" | "conversationId" | "slaResponseTime"
> & { engagementType?: string };

export interface Comment {
  id: string;
  content: string;
  createdOn: Date;
  createdBy: string;
  attachments: Attachment[];
}

export interface Attachment {
  id: string;
  fileName: string;
  downloadUrl: string;
  createdOn: Date;
  createdBy: string;
}
