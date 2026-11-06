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

import type { StoredFeatureState } from "@config/featureFlags";
import type { FeatureMap } from "@features/csm-admin/feature-visibility/featureTree";

/** One `PUT /features`. */
export interface FeatureWrite {
  navNodeId: string;
  state: StoredFeatureState;
  /** Set on the last write of a first save only. */
  markSaved?: boolean;
}

/** The page ids whose edit differs from what is saved. */
export function changedFeatureIds(saved: FeatureMap, edits: FeatureMap): string[] {
  return Object.keys(edits).filter((id) => edits[id] !== saved[id]);
}

interface PlanInput {
  /** What the page showed as saved: the saved rows, or before the first save the config lines under them. */
  saved: FeatureMap;
  /** The admin's unsaved choices, one entry per page they touched. */
  edits: FeatureMap;
  /** The rows the server really has. */
  rows: Record<string, string>;
  /** True once an admin has finished a first save. */
  savedOnce: boolean;
}

/**
 * The writes one Save sends: one per changed page, with an `enabled` choice
 * written like any other (a row is kept, never dropped, so a page that was
 * saved once keeps its own tag).
 *
 * The first save also writes every line the config file holds that has no row
 * yet, ahead of the edits, so turning the table into the source loses nothing.
 * The last write of a first save carries `markSaved`, so a save that stops part
 * way leaves the table reading as unsaved and `config.js` still applies.
 */
export function planFeatureSave({ saved, edits, rows, savedOnce }: PlanInput): FeatureWrite[] {
  const changed = changedFeatureIds(saved, edits);
  const changedSet = new Set(changed);
  const seed = savedOnce
    ? []
    : Object.keys(saved).filter((id) => !(id in rows) && !changedSet.has(id));
  const writes: FeatureWrite[] = [
    ...seed.map((id) => ({ navNodeId: id, state: saved[id]! })),
    ...changed.map((id) => ({ navNodeId: id, state: edits[id]! })),
  ];
  if (!savedOnce && writes.length > 0) {
    writes[writes.length - 1] = { ...writes[writes.length - 1]!, markSaved: true };
  }
  return writes;
}

export interface FeatureSaveResult {
  succeeded: string[];
  failed: string[];
}

/** How many writes run at once, so a long first save does not trip the gateway's rate limit. */
const WRITE_BATCH = 5;

/**
 * Sends the writes. Every write but the one carrying `markSaved` goes first,
 * a few at a time. The mark is sent only when all of those went through: if one
 * failed, the table is left reading as unsaved and nothing is half-switched.
 */
export async function executeFeatureSave(
  writes: readonly FeatureWrite[],
  put: (write: FeatureWrite) => Promise<unknown>,
): Promise<FeatureSaveResult> {
  const first = writes.filter((write) => !write.markSaved);
  const last = writes.filter((write) => write.markSaved);
  const succeeded: string[] = [];
  const failed: string[] = [];

  const send = async (batch: readonly FeatureWrite[]): Promise<void> => {
    const settled = await Promise.allSettled(batch.map((write) => put(write)));
    settled.forEach((outcome, index) => {
      const id = batch[index]!.navNodeId;
      if (outcome.status === "fulfilled") succeeded.push(id);
      else failed.push(id);
    });
  };

  for (let i = 0; i < first.length; i += WRITE_BATCH) {
    await send(first.slice(i, i + WRITE_BATCH));
  }
  if (failed.length === 0) await send(last);
  else failed.push(...last.map((write) => write.navNodeId));

  return { succeeded, failed };
}
