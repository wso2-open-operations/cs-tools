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

import { colors } from "@wso2/oxygen-ui";
import type { CaseSeverity, CaseState } from "@src/types";

// The active states the composition donuts track — mirrors the webapp's MATRIX_STATES
// (apps/csm-portal/webapp/src/features/csm-dashboard/api/useCaseCountsMatrix.ts): closed is
// excluded (the dashboard tracks active work) and so is reopened (not a normal lifecycle state).
export const COMPOSITION_STATES: CaseState[] = [
  "open",
  "work_in_progress",
  "waiting_on_wso2",
  "awaiting_info",
  "solution_proposed",
];

// Non-closed states for "Assigned to me" — the same set the webapp's MyAssignedCases widget uses
// (apps/csm-portal/webapp/src/features/csm-dashboard/components/MyAssignedCases.tsx), which
// includes reopened unlike the composition set above.
export const ASSIGNED_TO_ME_STATES: CaseState[] = [...COMPOSITION_STATES, "reopened"];

// Short severity code only (no "(Catastrophic)" etc.) — the composition donut's legend is too
// narrow, sitting in a half-width card, for support/config.tsx's full SEVERITY_LABELS.
export const SEVERITY_SHORT_LABELS: Record<CaseSeverity, string> = {
  catastrophic: "S0",
  critical: "S1",
  high: "S2",
  medium: "S3",
  low: "S4",
};

// Distinct hue per slice — mirrors the webapp's SEVERITY_SLICE_COLOR/STATE_SLICE_COLOR
// (CaseCompositionCharts.tsx). The support/config.tsx chip color configs reuse the same role
// color for several keys (e.g. every "info" state), which would merge pie slices together.
export const SEVERITY_SLICE_COLOR: Record<CaseSeverity, string> = {
  catastrophic: colors.red[600],
  critical: colors.deepOrange[500],
  high: colors.amber[700],
  medium: colors.blue[500],
  low: colors.green[500],
};

export const STATE_SLICE_COLOR: Record<CaseState, string> = {
  open: colors.blue[500],
  work_in_progress: colors.indigo[400],
  waiting_on_wso2: colors.amber[700],
  awaiting_info: colors.cyan[500],
  solution_proposed: colors.teal[500],
  closed: colors.grey[500],
  reopened: colors.purple[500],
};
