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

import type { BeIncidentImpact, BeIncidentPriority, BeIncidentUrgency } from "@api/backend/types";

export interface IncidentPriorityResult {
  priority: BeIncidentPriority;
  /** ITIL priority code (P1 highest .. P5 lowest), shown alongside the label. */
  code: "P1" | "P2" | "P3" | "P4" | "P5";
  label: string;
  /** Solid badge colours, not MUI's semantic `color` prop — the spec calls
   * for exact Red/Orange/Yellow/Blue/Gray, and MUI's theme-driven
   * error/warning/info/default don't reliably map to that (no "yellow" at
   * all, and warning/error can render close together in some themes). */
  bg: string;
  fg: string;
}

const PRIORITY_MATRIX: Record<BeIncidentImpact, Record<BeIncidentUrgency, IncidentPriorityResult>> = {
  HIGH: {
    HIGH: { priority: "CRITICAL", code: "P1", label: "Critical", bg: "#D32F2F", fg: "#FFFFFF" },
    MEDIUM: { priority: "HIGH", code: "P2", label: "High", bg: "#F57C00", fg: "#FFFFFF" },
    LOW: { priority: "MODERATE", code: "P3", label: "Moderate", bg: "#FBC02D", fg: "#000000" },
  },
  MEDIUM: {
    HIGH: { priority: "HIGH", code: "P2", label: "High", bg: "#F57C00", fg: "#FFFFFF" },
    MEDIUM: { priority: "MODERATE", code: "P3", label: "Moderate", bg: "#FBC02D", fg: "#000000" },
    LOW: { priority: "LOW", code: "P4", label: "Low", bg: "#1976D2", fg: "#FFFFFF" },
  },
  LOW: {
    HIGH: { priority: "MODERATE", code: "P3", label: "Moderate", bg: "#FBC02D", fg: "#000000" },
    MEDIUM: { priority: "LOW", code: "P4", label: "Low", bg: "#1976D2", fg: "#FFFFFF" },
    LOW: { priority: "PLANNING", code: "P5", label: "Planning", bg: "#9E9E9E", fg: "#FFFFFF" },
  },
};

/**
 * The standard ITIL impact × urgency → priority matrix (ServiceNow's own
 * out-of-the-box default), used to show a live priority preview on the
 * create form. This is purely a client-side preview — `CreateIncidentPayload`
 * has no `priority` field; the real value is computed server-side by
 * ServiceNow from the `impact`/`urgency` actually submitted, and only
 * appears once the incident is read back (see `BeIncident.priority`).
 */
export function computeIncidentPriority(
  impact: BeIncidentImpact | "",
  urgency: BeIncidentUrgency | "",
): IncidentPriorityResult | null {
  if (!impact || !urgency) return null;
  return PRIORITY_MATRIX[impact][urgency];
}
