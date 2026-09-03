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
 * The "optional" middle columns `CasesList` can render between Subject and
 * State — everything a caller can add, remove, or reorder via
 * `ColumnCustomizerButton`. Case ID, Subject, State, and Updated are never
 * optional: they carry the row's identity, its own sort control, or (Case ID)
 * the row's real anchor link, so removing them would break navigation/sort
 * rather than just decluttering.
 *
 * Fields on `CsmCaseRow` deliberately left off this list, and why:
 * - `id`, `accountId`, `projectId` — raw UUIDs, never human-facing.
 * - `wso2CaseId` / `caseNumber` — already rendered together in the fixed
 *   Case ID column; a separate column would just repeat it.
 * - `projectName` — already rendered under Subject on every row (see
 *   `CasesList`), regardless of which optional columns are on; adding it here
 *   too would show the same value twice.
 * - `state` / `updatedAt` — fixed columns (State, Updated).
 * - `workState` — only meaningful for `work_in_progress` cases, and already
 *   rendered stacked under the State chip for exactly those rows; a standalone
 *   column would be blank for everything else.
 * - `assigneeIsMe`, `hasSla` — booleans that drive other UI (assignee-filter
 *   defaults, the SLA clock's "unknown" fallback), not stand-alone facts worth
 *   a column of their own.
 * - `slaClockType`, `minutesToBreach` — no existing list-row formatter/chip to
 *   reuse, and `hasSla`'s own doc comment says the backend doesn't have real
 *   SLA data for every row yet (LIVE rows deliberately report `hasSla: false`
 *   rather than a misleading countdown) — a column built on that today would
 *   show "unknown" for rows that do have a real SLA and could reasonably be
 *   revisited once SLA data is reliably populated across all sources.
 */
export type CaseOptionalColumnId =
  | "product"
  | "type"
  | "issueType"
  | "severity"
  | "assignee"
  | "createdBy"
  | "customer"
  | "createdAt"
  | "escalationLevel";

export const CASE_OPTIONAL_COLUMNS: Record<
  CaseOptionalColumnId,
  { label: string; track: string }
> = {
  product: { label: "Product", track: "minmax(140px, 1fr)" },
  type: { label: "Type", track: "auto" },
  issueType: { label: "Issue type", track: "minmax(140px, 1fr)" },
  severity: { label: "Severity", track: "auto" },
  assignee: { label: "Assignee", track: "minmax(140px, 1fr)" },
  createdBy: { label: "Reporter", track: "minmax(140px, 1fr)" },
  customer: { label: "Customer", track: "minmax(140px, 1fr)" },
  createdAt: { label: "Created", track: "minmax(100px, 0.7fr)" },
  // Blank (not a "not escalated" chip) for cases with no escalation level —
  // see `renderOptionalCell`'s "escalationLevel" case in CasesList.tsx.
  escalationLevel: { label: "Escalation", track: "auto" },
};
