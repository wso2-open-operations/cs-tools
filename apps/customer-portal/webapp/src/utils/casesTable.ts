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
 * Get priority color based on label.
 * @param label - Priority label
 * @returns Color string
 */
export const getPriorityColor = (
  label?: string,
): "error" | "warning" | "info" | "success" | "default" => {
  const normalized = label?.toLowerCase() || "";
  if (normalized.includes("critical") || normalized.includes("s1"))
    return "error";
  if (normalized.includes("high") || normalized.includes("s2"))
    return "warning";
  if (normalized.includes("medium") || normalized.includes("s3")) return "info";
  if (normalized.includes("low") || normalized.includes("s4")) return "success";
  return "default";
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
