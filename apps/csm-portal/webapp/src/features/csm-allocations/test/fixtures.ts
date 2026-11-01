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

import type { Allocation, AllocationUpdate } from "../types";

const Z = "00000000-0000-0000-0000-";

export const ALLOCATION: Allocation = {
  id: `${Z}0000000000a1`,
  allocationId: "ALLOC0001001",
  state: "CONFIRMED",
  startDate: "2026-09-21",
  endDate: "2026-09-25",
  startTime: "09:00:00",
  endTime: "17:00:00",
  timezone: "UTC",
  utilization: 50,
  engagement: {
    id: `${Z}0000000000e1`,
    displayId: "ENG0001001",
    name: "Example Migration",
    type: "MIGRATION",
    modeOfDelivery: "REMOTE",
    state: "IN_PROGRESS",
    businessUnit: "Integration",
    isPaid: true,
    plannedStartDate: "2026-09-01",
    plannedEndDate: "2026-12-01",
  },
  account: { id: `${Z}0000000000c1`, name: "Example Corp" },
  engineer: { id: `${Z}0000000000b1`, name: "Jane Doe", email: "jane.doe@example.com" },
  latestUpdateDate: "2026-09-22T10:00:00Z",
};

export const SECOND_ALLOCATION: Allocation = {
  ...ALLOCATION,
  id: `${Z}0000000000a2`,
  allocationId: "ALLOC0001002",
  state: "TENTATIVE",
  utilization: 25.5,
  latestUpdateDate: undefined,
  engagement: { ...ALLOCATION.engagement, id: `${Z}0000000000e2`, displayId: "ENG0001002", name: "Example Rollout" },
  account: { id: `${Z}0000000000c2`, name: "Sample Ltd" },
};

export const UPDATE: AllocationUpdate = {
  id: `${Z}0000000000d1`,
  subject: "Week 1 status",
  content: "Line one\nLine two",
  state: "PUBLISHED",
  visibility: "INTERNAL",
  cycleStartDate: "2026-09-21",
  publishedDate: "2026-09-22T10:00:00Z",
  createdOn: "2026-09-22T10:00:00Z",
  author: { id: `${Z}0000000000b1`, name: "Jane Doe" },
};

export const CANCELLED_UPDATE: AllocationUpdate = {
  ...UPDATE,
  id: `${Z}0000000000d2`,
  subject: "Withdrawn status",
  state: "CANCELLED",
};
