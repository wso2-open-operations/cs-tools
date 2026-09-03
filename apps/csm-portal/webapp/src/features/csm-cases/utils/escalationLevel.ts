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

import type { SemanticRole } from "@components/SemanticChip";

// Single source of truth for how a raw escalation-level id ("0"-"5") is
// labelled and coloured across the CSM portal — the chip (`EscalationLevelChip`)
// and the case-detail escalation card both read this rather than keeping their
// own copies. Lives in its own module (not a component file) so it can be
// imported from a plain data mapper without tripping the react-refresh
// "components-only export" rule.
//
// Deliberately simpler than the customer portal's own escalation constants
// (`ESCALATION_NEXT_LEVEL` et al. in supportConstants.ts): CSM has no
// lead-gating or "who may de-escalate" rules today (see BFF/entity-service —
// no role gate), so this only carries the label/colour ramp, not the
// customer-facing next-level stepper's full rule set.
export const ESCALATION_MAX_LEVEL = "5";

/** Human label for each raw escalation-level id, e.g. "EL2 — Technology Unit Head". */
export const ESCALATION_LEVEL_LABEL: Record<string, string> = {
  "0": "Not escalated",
  "1": "EL1 — Team Lead",
  "2": "EL2 — Technology Unit Head",
  "3": "EL3 — CRE Head",
  "4": "EL4 — CCO / CRO",
  "5": "EL5 — CEO",
};

/** Short label for compact contexts (list column, chip), e.g. "EL2". */
export const ESCALATION_LEVEL_SHORT_LABEL: Record<string, string> = {
  "0": "Not escalated",
  "1": "EL1",
  "2": "EL2",
  "3": "EL3",
  "4": "EL4",
  "5": "EL5",
};

// Higher level = more attention-grabbing colour. EL0 renders as a quiet
// "default" role (never actually shown — see EscalationLevelChip/the list
// column's blank-unless-escalated rule) so an unrecognised id also falls back
// to something contrast-safe rather than an alarming red.
const ESCALATION_LEVEL_COLOR: Record<string, SemanticRole> = {
  "0": "default",
  "1": "info",
  "2": "warning",
  "3": "warning",
  "4": "error",
  "5": "error",
};

/** Semantic colour role for a raw escalation-level id; unrecognised ids fall
 * back to "default" rather than guessing. */
export function escalationLevelColor(level: string): SemanticRole {
  return ESCALATION_LEVEL_COLOR[level] ?? "default";
}

/** Human label for a raw escalation-level id; unrecognised ids render as-is
 * (prefixed "EL") rather than silently hiding the value. */
export function escalationLevelLabel(level: string, short = false): string {
  const table = short ? ESCALATION_LEVEL_SHORT_LABEL : ESCALATION_LEVEL_LABEL;
  return table[level] ?? `EL${level}`;
}

/** Whether a level string represents "not escalated" — the case has never
 * been escalated, or the backing data source doesn't track it. */
export function isEscalationLevelUnset(
  level: string | null | undefined,
): boolean {
  return !level || level === "0";
}

export function canEscalateFurther(level: string | null | undefined): boolean {
  return (level ?? "0") !== ESCALATION_MAX_LEVEL;
}

export function canDeescalate(level: string | null | undefined): boolean {
  return !isEscalationLevelUnset(level);
}
