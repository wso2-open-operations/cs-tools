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

export interface StackRow {
  id: string;
  sortOrder: number;
  unpublished: boolean;
}

/**
 * Puts id into the stack and rewrites every place as 0, 1, 2, … with no gaps.
 * An unpublished banner sits after the stack. A banner that is not unpublished
 * is inserted at order, and the banners at that place and below move down.
 */
export function placeInStack(rows: StackRow[], id: string, order: number, unpublished: boolean): StackRow[] {
  const sorted = [...rows].sort((a, b) => a.sortOrder - b.sortOrder || a.id.localeCompare(b.id));
  const active: StackRow[] = [];
  const parked: StackRow[] = [];
  for (const row of sorted) {
    if (row.id === id) continue;
    if (row.unpublished) parked.push(row);
    else active.push(row);
  }
  const target: StackRow = { id, sortOrder: 0, unpublished };
  if (unpublished) {
    parked.push(target);
  } else {
    const index = Math.min(Math.max(order, 0), active.length);
    active.splice(index, 0, target);
  }
  return [...active, ...parked].map((row, index) => ({ ...row, sortOrder: index }));
}

/** Drops a banner and rewrites the places that remain as 0, 1, 2, … with no gaps. */
export function dropFromStack(rows: StackRow[], id: string): StackRow[] {
  const sorted = [...rows].sort((a, b) => a.sortOrder - b.sortOrder || a.id.localeCompare(b.id));
  const active: StackRow[] = [];
  const parked: StackRow[] = [];
  for (const row of sorted) {
    if (row.id === id) continue;
    if (row.unpublished) parked.push(row);
    else active.push(row);
  }
  return [...active, ...parked].map((row, index) => ({ ...row, sortOrder: index }));
}

/** The place filled in for a new banner: the end of the stack, which is 0 when the stack is empty. */
export function nextStackOrder(rows: StackRow[]): number {
  return rows.filter((row) => !row.unpublished).length;
}
