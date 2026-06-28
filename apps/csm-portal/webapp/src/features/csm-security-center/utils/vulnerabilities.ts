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

import type { BeVulnerabilityPriority } from "@api/backend/types";

type ChipColor = "default" | "info" | "warning" | "success" | "error";

/**
 * Human-readable label for a `VulnerabilityPriority` enum value (used in the
 * filter select). The `priority` field on `ProductVulnerabilityView` is already
 * a label string from the upstream (e.g. "High"), so use it directly for display.
 */
const PRIORITY_LABEL: Record<BeVulnerabilityPriority, string> = {
  info: "Info",
  low: "Low",
  medium: "Medium",
  high: "High",
  critical: "Critical",
  unknown: "Unknown",
};

/**
 * MUI/Oxygen chip colour keyed on the *lowercase* priority string. Works with
 * both the enum values (filter select) and the label strings from
 * `ProductVulnerabilityView.priority` (e.g. "High" → lower-cased to "high").
 */
const PRIORITY_COLOR: Record<string, ChipColor> = {
  critical: "error",
  high: "error",
  medium: "warning",
  low: "info",
  info: "default",
  unknown: "default",
};

/** All vulnerability priorities in display order (highest to lowest). */
export const VULNERABILITY_PRIORITIES = [
  "critical",
  "high",
  "medium",
  "low",
  "info",
  "unknown",
] as BeVulnerabilityPriority[];

export function vulnerabilityPriorityLabel(priority: BeVulnerabilityPriority): string {
  return PRIORITY_LABEL[priority] ?? priority;
}

/**
 * Returns the MUI chip colour for a priority value. Accepts both enum values
 * ("high") and label strings from the view ("High") by normalising to lowercase.
 */
export function vulnerabilityPriorityColor(priority?: string | null): ChipColor {
  if (!priority) return "default";
  return PRIORITY_COLOR[priority.toLowerCase()] ?? "default";
}
