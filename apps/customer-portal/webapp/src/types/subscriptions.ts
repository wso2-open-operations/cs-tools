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

// Enum for project types handled by subscription visibility rules.
export enum ProjectType {
  MANAGED_CLOUD_SUBSCRIPTION = "Managed Cloud Subscription",
  CLOUD_SUPPORT = "Cloud Support",
  CLOUD_EVALUATION_SUPPORT = "Cloud Evaluation Support",
  EVALUATION_SUBSCRIPTION = "Evaluation Subscription",
  SUBSCRIPTION = "Subscription",
}

// Model type for feature flags derived from project type for UI visibility and stats.
export type ProjectPermissions = {
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
};
