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

/**
 * Global mock delay for API hooks to simulate network latency.
 * This can be overridden in tests to improve execution time.
 */
export const API_MOCK_DELAY = 800;

/**
 * Constants for API-related query keys.
 */
export const ApiQueryKeys = {
  /**
   * Key for project search api call.
   */
  PROJECTS: "projects",
  /**
   * Key for project support statistics api call.
   */
  SUPPORT_STATS: "support-stats",
  /**
   * Key for case creation metadata api call.
   */
  CASE_CREATION_METADATA: "case-creation-metadata",
  /**
   * Key for project cases statistics api call.
   */
  CASES_STATS: "cases-stats",
  /**
   * Key for dashboard mock statistics api call.
   */
  DASHBOARD_STATS: "dashboard-stats",
} as const;
