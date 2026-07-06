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
 * Pure mapping helpers between the task-SLA search wire shape
 * ({@link TaskSlaView}) and the {@link CaseSla} row model the SLA table
 * renders. Kept free of any API-client import so this stays unit-testable
 * without the backend config the client requires.
 */

import type {
  CaseSla,
  SlaStage,
  TaskSlaView,
} from "@features/csm-cases/types/csmCases";

/** The known, closed set of SLA stages the backend documents today. */
type KnownSlaStage = "in_progress" | "paused" | "completed" | "cancelled" | "breached";

const KNOWN_SLA_STAGES: readonly KnownSlaStage[] = [
  "in_progress",
  "paused",
  "completed",
  "cancelled",
  "breached",
];

const SLA_STAGE_LABEL: Record<KnownSlaStage, string> = {
  in_progress: "In progress",
  paused: "Paused",
  completed: "Completed",
  cancelled: "Cancelled",
  breached: "Breached",
};

/** Lowercases/trims/spaces-to-underscores a raw stage value and folds the
 * backend's "achieved" wording onto our "completed" vocabulary. */
export function normalizeStage(raw: string | null): { stage: SlaStage; label: string } {
  const trimmed = (raw ?? "").trim();
  if (!trimmed) return { stage: "in_progress", label: "In progress" };

  let normalized = trimmed.toLowerCase().replace(/\s+/g, "_");
  if (normalized === "achieved") normalized = "completed";

  if ((KNOWN_SLA_STAGES as readonly string[]).includes(normalized)) {
    const stage = normalized as KnownSlaStage;
    return { stage, label: SLA_STAGE_LABEL[stage] };
  }

  // Unknown stage from the backend: keep it visible rather than silently
  // recoding it, but still give the table a stable SlaStage to key off of.
  return { stage: normalized, label: trimmed };
}

/** Maps one wire-shape SLA record onto the row model {@link CaseSlaTable} renders. */
export function toCaseSla(view: TaskSlaView): CaseSla {
  const { stage, label } = normalizeStage(view.stage);
  const businessElapsedPercent = view.businessElapsedPercentage ?? 0;

  return {
    id: view.id,
    definition: view.slaDefinition?.name ?? "",
    target: view.slaDefinition?.target ?? null,
    stage,
    stageLabel: label,
    // The backend doesn't expose an authoritative "breached" flag yet, so
    // this derives a UI proxy from the elapsed percentage crossing 100%.
    hasBreached: businessElapsedPercent >= 100,
    businessTimeLeftLabel: view.businessTimeLeft ?? "",
    businessElapsedLabel: view.businessElapsedTime ?? "",
    businessElapsedPercent,
    startTime: view.startTime,
    stopTime: view.endTime,
  };
}
