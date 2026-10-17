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

// Response shapes for the Timelogs report, ported from the source app's
// data/utils/types.ts (the reports-relevant subset only — kept local to this
// domain rather than a shared cross-domain file, per the port's scoping).
/* eslint-disable @typescript-eslint/no-explicit-any -- mirrors the source's own loosely-typed index signature */

export interface DataStruct {
  [key: string]: any;
}

export interface TimeLogBreakdownDetails extends DataStruct {
  projectName: string;
  projectKey: string;
  projectType: string;
  remainingQueryHours: string;
  totalQueryHours: string;
  cases: TimeLogBreakdownCase[];
}

export interface TimeLogBreakdownCase extends DataStruct {
  caseSysId: string;
  caseNumber: string;
  caseId: string;
  caseType: string;
  shortDescription: string;
  priority: string;
  state: string;
  totalHours: string;
  consumedQueryHours: string;
  timeCards: TimeCardDetails[];
}

export interface TimeCardDetails extends DataStruct {
  total: string;
  createdOn: string;
  createdBy: string;
  isBillable: string;
  state: string;
}
