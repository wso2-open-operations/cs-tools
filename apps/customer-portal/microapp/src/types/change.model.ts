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

export interface ChangeRequest {
  id: string;
  internalId?: string;
  number: string;
  title: string;
  description: string;
  requestType?: string;
  impactId?: string;
  statusId?: string;
  endDate?: Date;
  createdOn: Date;
  updatedOn: Date;
  createdBy: string;
  approvedBy?: string;
  approvedOn?: Date;
  duration?: string;
  hasCustomerApproved: boolean;
  /**
   * True when WSO2 accepted a time the customer proposed AND the change request has moved on from Customer
   * Approval because of it (Scheduled or later). `hasCustomerApproved` stays false then (no staff action records a
   * customer's approval: the proposal was the customer's own consent), so readers of `hasCustomerApproved` show
   * "Proposed time accepted" instead of "not approved". False while the change is in Customer Approval, whatever
   * answer is standing: the customers are asked again there and Approve and Reject are live.
   */
  isProposedTimeAccepted: boolean;
  hasCustomerReviewed: boolean;
  assignedTeam?: string;
  serviceOutage?: string;
  rollbackPlan?: string;
  communicationPlan?: string;
  testPlan?: string;
  deployment?: string;
}

export type ChangeRequestSummary = Pick<
  ChangeRequest,
  | "id"
  | "internalId"
  | "number"
  | "title"
  | "description"
  | "requestType"
  | "impactId"
  | "statusId"
  | "assignedTeam"
  | "endDate"
  | "createdOn"
  | "updatedOn"
>;
