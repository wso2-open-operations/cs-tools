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

import { parseBackendTimestamp } from "@utils/dateTime";
import { sideOfState, type SlaTimelineEvent } from "./slaImpactTimeline";

export type OwnershipSide = "wso2" | "customer" | "closed";
export interface OwnershipPeriod {
  side: OwnershipSide;
  start: number;
  end: number;
}
export const timestampMs = (at: string | undefined | null): number | undefined =>
  at ? parseBackendTimestamp(at)?.getTime() : undefined;

/** Ownership comes only from states, never from an SLA's pause status or its ETA. */
export function ownershipTimeline(events: SlaTimelineEvent[], openedAt: string | undefined, currentState: string | undefined, now: number, closedAt?: string) {
  const timed = events.filter((e) => {
    const ms = timestampMs(e.at);
    return ms !== undefined && ms <= now;
  }).sort((a, b) => timestampMs(a.at)! - timestampMs(b.at)! || a.id.localeCompare(b.id));
  const states = timed.filter((e) => ["state", "closed", "reopened"].includes(e.kind));
  const lastState = states.at(-1);
  const explicitClose = timestampMs(closedAt);
  const end = lastState?.kind === "closed"
    ? timestampMs(lastState.at)!
    : currentState?.toLowerCase() === "closed" && explicitClose !== undefined
      ? Math.min(explicitClose, now) : now;
  const opened = timestampMs(openedAt);
  const start = Math.min(opened ?? timestampMs(timed[0]?.at) ?? end, end);
  let side: OwnershipSide = states[0]
    ? states[0].kind === "reopened" ? "closed" : states[0].previousSide ?? "wso2"
    : sideOfState(currentState) ?? (currentState?.toLowerCase() === "closed" && explicitClose === undefined ? "closed" : "wso2");
  const periods: OwnershipPeriod[] = [];
  let cursor = start;
  const append = (to: number) => {
    if (to <= cursor) return;
    const previous = periods.at(-1);
    if (previous?.side === side) previous.end = to;
    else periods.push({ side, start: cursor, end: to });
    cursor = to;
  };
  for (const state of states) {
    const at = timestampMs(state.at)!;
    if (at > end) break;
    append(Math.max(start, at));
    side = state.kind === "closed" ? "closed" : state.side ?? "wso2";
  }
  append(end);
  return { start, end, periods, events: timed.filter((e) => timestampMs(e.at)! >= start && timestampMs(e.at)! <= end) };
}

export function elapsedLabel(ms: number): string {
  const minutes = Math.floor(Math.max(0, ms) / 60_000);
  if (minutes === 0) return ms > 0 ? "<1m" : "0m";
  const days = Math.floor(minutes / 1440);
  const hours = Math.floor(minutes % 1440 / 60);
  const remainder = minutes % 60;
  return [days && `${days}d`, hours && `${hours}h`, remainder && `${remainder}m`].filter(Boolean).join(" ");
}

/** Cluster without shifting an event's timestamp. The cluster represents the enclosed range. */
export function clusterTimelineEvents(events: SlaTimelineEvent[], start: number, end: number, width: number, minimumGap = 44): SlaTimelineEvent[][] {
  const groups: SlaTimelineEvent[][] = [];
  const span = Math.max(1, end - start);
  for (const event of events) {
    const group = groups.at(-1);
    // Merge connected crowded events so adjacent cluster labels cannot overlap.
    if (group && (timestampMs(event.at)! - timestampMs(group.at(-1)!.at)!) / span * width < minimumGap) group.push(event);
    else groups.push([event]);
  }
  return groups;
}
