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

import { PROJECT_TYPE_LABELS } from "@constants/projectTypeConstants";

/** API project type labels handled by subscription visibility rules. */
export type ProjectType =
  | (typeof PROJECT_TYPE_LABELS)[keyof typeof PROJECT_TYPE_LABELS]
  | string;

/**
 * Feature flags derived from project type for UI visibility and stats.
 */
export interface ProjectPermissions {
  hasOperations: boolean;
  hasDeployments: boolean;
  hasQueryHours: boolean;
  hasTimeLogs: boolean;
  hasSR: boolean;
  hasCR: boolean;
  showOutstandingOpsChart: boolean;
  includeChangeRequestsInDashboardTotals: boolean;
  includeS0InSupportMetrics: boolean;
  showServiceHoursAllocationsCard: boolean;
}
