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

import { formatDateOnlyForDisplay, parseBackendTimestamp } from "@utils/dateTime";
import { severityChangeLabel } from "@features/csm-cases/utils/caseActivityFeed";
import { stateLabel } from "@features/csm-dashboard/utils/abtDashboard";
import type { CaseAuditEntry } from "@features/csm-cases/types/csmCases";

/** Whose move it is once the case is in a state. `null` for a closed case. */
export type SlaTimelineSide = "customer" | "wso2";

export type SlaTimelineEventKind =
  | "state"
  | "closed"
  | "reopened"
  | "eta"
  | "workaround"
  | "workaround_withdrawn"
  | "severity"
  | "update_released";

/** One from -> to row of an event. `from` is `null` when the value was previously unset. */
export interface SlaTimelineChange {
  label: string;
  from: string | null;
  to: string | null;
}

export interface SlaTimelineEvent {
  id: string;
  /** Raw backend timestamp of the audit row. */
  at: string;
  kind: SlaTimelineEventKind;
  title: string;
  actor: string;
  /** Side the case is on after this event; set for state events only. */
  side: SlaTimelineSide | null;
  /** Side the case was on before it, when that is known; state events only. */
  previousSide: SlaTimelineSide | null;
  changes: SlaTimelineChange[];
}

// States in which the case waits on the customer. Mirrors the "whose move is
// it" buckets documented beside `STATE_COLOR` (grey = waiting on the customer);
// every other open state is WSO2's move. Whether the SLA clock actually pauses
// is decided per SLA definition by the backend, so it is not derived here.
const CUSTOMER_STATES = new Set(["awaiting_info", "solution_proposed"]);

function stateKey(state: string): string {
  return state.trim().toLowerCase().replace(/\s+/g, "_");
}

/** Side a state puts the case on; `null` for closed (nobody's move). */
export function sideOfState(state: string | null | undefined): SlaTimelineSide | null {
  if (!state || !state.trim()) return null;
  const key = stateKey(state);
  if (key === "closed") return null;
  return CUSTOMER_STATES.has(key) ? "customer" : "wso2";
}

const ETA_FIELDS = new Set(["best_case_eta", "most_likely_eta", "worst_case_eta"]);

function present(value: string | undefined): string | null {
  const v = value?.trim();
  return v ? v : null;
}

function etaDisplay(value: string | null): string | null {
  return value === null ? null : (formatDateOnlyForDisplay(value) ?? value);
}

function stateTitle(from: string | null, to: string): string {
  const fromKey = from ? stateKey(from) : "";
  const toKey = stateKey(to);
  if (toKey === "closed") return "Case closed";
  if (fromKey === "closed") return "Case reopened";
  const before = sideOfState(from);
  const after = sideOfState(to);
  if (before !== after && after === "customer") return "Waiting on customer";
  if (before !== after && after === "wso2" && before === "customer") return "Back with WSO2";
  return "State changed";
}

function timeOf(at: string): number {
  return parseBackendTimestamp(at)?.getTime() ?? 0;
}

/**
 * Pick the SLA-relevant events out of a case's audited field changes, oldest
 * first. Maps `state`, `severity`, the three fix-ETA fields,
 * `workaround_provided_on` and `fix_issued_on`; everything else (assignee,
 * work state, the derived "ETA shared" stamp, ...) is dropped on purpose. The
 * three ETA dates saved together by one actor in the same second collapse into
 * one event.
 */
export function buildSlaImpactTimeline(entries: CaseAuditEntry[]): SlaTimelineEvent[] {
  const sorted = [...entries].sort(
    (a, b) => timeOf(a.createdAt) - timeOf(b.createdAt) || a.id.localeCompare(b.id),
  );
  const events: SlaTimelineEvent[] = [];
  const etaByGroup = new Map<string, SlaTimelineEvent>();

  for (const entry of sorted) {
    for (const c of entry.changes ?? []) {
      const base = { id: `${entry.id}:${c.field}`, at: entry.createdAt, actor: entry.actor };
      const from = present(c.previousValue);
      const to = present(c.newValue);

      if (c.field === "state") {
        if (!to || (from && stateKey(from) === stateKey(to))) continue;
        const title = stateTitle(from, to);
        events.push({
          ...base,
          kind: title === "Case closed" ? "closed" : title === "Case reopened" ? "reopened" : "state",
          title,
          side: sideOfState(to),
          previousSide: sideOfState(from),
          changes: [{ label: "State", from: from ? stateLabel(from) : null, to: stateLabel(to) }],
        });
      } else if (c.field === "severity") {
        if (!to || from === to) continue;
        events.push({
          ...base,
          kind: "severity",
          title: "Severity changed",
          side: null,
          previousSide: null,
          changes: [
            { label: "Severity", from: from ? severityChangeLabel(from) : null, to: severityChangeLabel(to) },
          ],
        });
      } else if (ETA_FIELDS.has(c.field)) {
        const change = { label: c.fieldLabel, from: etaDisplay(from), to: etaDisplay(to) };
        const group = `${entry.actor}|${entry.createdAt.slice(0, 19)}`;
        const existing = etaByGroup.get(group);
        if (existing) {
          existing.changes.push(change);
        } else {
          const event: SlaTimelineEvent = {
            ...base,
            kind: "eta",
            title: from === null ? "Fix ETA set" : "Fix ETA changed",
            side: null,
            previousSide: null,
            changes: [change],
          };
          etaByGroup.set(group, event);
          events.push(event);
        }
      } else if (c.field === "workaround_provided_on") {
        events.push({
          ...base,
          kind: to ? "workaround" : "workaround_withdrawn",
          title: to ? "Workaround provided" : "Workaround withdrawn",
          side: null,
          previousSide: null,
          changes: [],
        });
      } else if (c.field === "fix_issued_on" && to) {
        events.push({
          ...base,
          kind: "update_released",
          title: "Update released",
          side: null,
          previousSide: null,
          changes: [],
        });
      }
    }
  }
  return events;
}
