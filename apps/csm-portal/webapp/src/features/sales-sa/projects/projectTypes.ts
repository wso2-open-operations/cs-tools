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

// Loosely typed to match the Go backend's servicenow.ProjectDetails, which
// carries a few fields (accountName, accountNumber, totalQueryHours,
// projectType) beyond what's declared below.
export interface ProjectDetails {
  /** entity-service's internal UUID -- same value as sysId. */
  id?: string;
  number: string;
  sysId: string;
  name: string;
  key: string;
  startDate: string;
  endDate: string;
  remainingQueryHours: string;
  closureState: string;
  accountNumber?: string;
  accountName?: string;
  /** entity-service's internal UUID for the linked account -- used for navigation, not display. */
  accountId?: string;
  totalQueryHours?: string;
  projectType?: string;
  [key: string]: unknown;
}

export interface Contact {
  contactName: string;
  email: string;
  state: string;
  [key: string]: unknown;
}

export interface CaseDetails {
  /** entity-service's internal UUID -- same value as sysId. */
  id?: string;
  sysId?: string;
  number: string;
  caseId: string;
  caseType: string;
  shortDescription: string;
  priority: string;
  state: string;
  [key: string]: unknown;
}
