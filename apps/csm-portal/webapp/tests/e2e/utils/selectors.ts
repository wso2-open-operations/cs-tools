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

//
// Stable anchors for the FE-first time-card store's seeded demo data
// (features/csm-timecards/api/timeCardStore.ts). Card ids are random and
// dates/weeks are relative to "now", so specs must key off these stable case
// numbers / engineer names / states — never ids or absolute dates.
//

/** Cards seeded for the *signed-in* user, shown on the "My time sheets" tab. */
export const MY_SEED = {
  /** Pending card — has Edit / Delete / Submit actions. */
  pending: { caseNumber: "CS0353001", state: "Pending" },
  /** Approved card — no owner actions. */
  approved: { caseNumber: "CS0352900", state: "Approved" },
} as const;

/** Other engineers' sheets seeded in the approver "Approvals" queue. */
export const QUEUE_SEED = {
  sajith: { name: "Sajith Ekanayaka", submitted: "CS0352584" },
  nimal: { name: "Nimal Perera", submitted: "CS0349881" },
} as const;

/** Page-level accessible names used across the time-cards UI. */
export const TIMECARDS = {
  path: "/time-cards",
  heading: "Time cards",
  tabs: { mine: "My time sheets", approvals: "Approvals" },
} as const;
