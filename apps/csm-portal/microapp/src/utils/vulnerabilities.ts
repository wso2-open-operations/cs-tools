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

import type { ChipProps } from "@wso2/oxygen-ui";
import type { VulnerabilityPriority } from "@src/types";

/** All vulnerability priorities in display order (highest to lowest) — mirrors the
 * webapp's VULNERABILITY_PRIORITIES (csm-security-center/utils/vulnerabilities.ts). */
export const ALL_VULNERABILITY_PRIORITIES: VulnerabilityPriority[] = [
  "critical",
  "high",
  "medium",
  "low",
  "info",
  "unknown",
];

export const VULNERABILITY_PRIORITY_LABEL: Record<VulnerabilityPriority, string> = {
  critical: "Critical",
  high: "High",
  medium: "Medium",
  low: "Low",
  info: "Info",
  unknown: "Unknown",
};

const PRIORITY_COLOR: Record<string, NonNullable<ChipProps["color"]>> = {
  critical: "error",
  high: "error",
  medium: "warning",
  low: "info",
  info: "default",
  unknown: "default",
};

/** Chip colour for a priority value. Accepts both the enum values ("high") and the label
 * strings the search/detail endpoints actually return ("High") by normalizing to lowercase —
 * mirrors the webapp's vulnerabilityPriorityColor. */
export function vulnerabilityPriorityColor(priority?: string | null): NonNullable<ChipProps["color"]> {
  if (!priority) return "default";
  return PRIORITY_COLOR[priority.toLowerCase()] ?? "default";
}
