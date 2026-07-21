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

import type { BeTaskState } from "@api/backend/types";

type ChipColor = "default" | "info" | "warning" | "success" | "error";

// Only 2 real live values are seen in this org's data (OPEN/CLOSED); OTHER is
// a genuine fallback for undocumented raw values observed live and is styled
// as a neutral/unknown state rather than treated as unreachable.
const STATE_LABEL: Record<BeTaskState, string> = {
  OPEN: "Open",
  CLOSED: "Closed",
  OTHER: "Other",
};

const STATE_COLOR: Record<BeTaskState, ChipColor> = {
  OPEN: "info",
  CLOSED: "success",
  OTHER: "default",
};

/** All task states, for a filter control if one is ever added. */
export const TASK_STATES = Object.keys(STATE_LABEL) as BeTaskState[];

export function taskStateLabel(state?: BeTaskState | string | null): string {
  if (!state) return "—";
  return STATE_LABEL[state as BeTaskState] ?? "Other";
}

export function taskStateColor(state?: BeTaskState | string | null): ChipColor {
  if (!state) return "default";
  return STATE_COLOR[state as BeTaskState] ?? "default";
}
