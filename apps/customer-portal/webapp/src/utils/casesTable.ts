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
 * Returns the Oxygen ui color path for a given severity label.
 * @param label - The severity label (e.g., S0, S1, S2, S3, S4).
 * @returns The Oxygen ui color path (e.g., "error.main").
 */
export const getSeverityColor = (label?: string): string => {
  const normalized = label?.toUpperCase() || "";
  if (normalized === "S0") return "error.main";
  if (normalized === "S1") return "warning.main";
  if (normalized === "S2") return "text.disabled";
  if (normalized === "S3") return "info.main";
  if (normalized === "S4") return "success.main";
  return "text.secondary";
};

/**
 * Get status color based on label.
 * @param label - Status label
 * @returns Color string
 */
export const getStatusColor = (label?: string): string => {
  const normalized = label?.toLowerCase() || "";
  if (normalized.includes("open")) return "info.main";
  if (normalized.includes("awaiting")) return "primary.main";
  if (normalized.includes("progress")) return "warning.main";
  if (normalized.includes("resolved") || normalized.includes("closed"))
    return "success.main";
  return "text.secondary";
};
