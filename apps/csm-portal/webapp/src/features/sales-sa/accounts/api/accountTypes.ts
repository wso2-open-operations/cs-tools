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

// Ported from apps/support-portal-lite/webapp's own accountTypes.ts.
// DataStruct's index signature is preserved because ListAccountDetail reads
// a handful of ServiceNow-integration fields (integrationCSTeam*) that
// aren't declared below either — the real backend (internal/servicenow.
// AccountDetails) returns more fields than this UI-facing type declares.
export interface DataStruct {
  [key: string]: unknown;
}

export interface AccountDetails extends DataStruct {
  /** entity-service's internal UUID -- the id every navigation link now uses. */
  id: string;
  number: string;
  name: string;
  region: string;
  country: string;
  city: string;
  arr: string;
  accountManager: string;
  technicalOwner: string;
  customerSuccessManager: string;
  rating: string;
  driveLocation: string;
  integrationCSTeamName?: string;
  integrationCSTeamSysId?: string;
  /** Internal to useAccountsApi.ts's client-side "active" filter -- not rendered. */
  _deactivationDate?: string | null;
}

export interface ProjectDetails extends DataStruct {
  number: string;
  sysId: string;
  name: string;
  key: string;
  startDate: string;
  endDate: string;
  remainingQueryHours: string;
  closureState: string;
}

export interface EscalationDetails extends DataStruct {
  id: string;
  severity: string;
  state: string;
  escalatedOn: string;
}

export interface ABTTeamMembersDetails extends DataStruct {
  name: string;
  email: string;
  role: string;
  employeeThumbnail?: string;
}

export type ToggleSwitchState = "active-accounts" | "all-accounts";
