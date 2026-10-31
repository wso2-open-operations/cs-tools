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

import { describe, expect, it } from "vitest";
import { dropFromStack, nextStackOrder, placeInStack, type StackRow } from "@features/csm-admin/banners/bannerOrder";

const stack: StackRow[] = [
  { id: "a", sortOrder: 0, unpublished: false },
  { id: "b", sortOrder: 1, unpublished: false },
  { id: "c", sortOrder: 2, unpublished: false },
];

function orders(rows: StackRow[]): string[] {
  return rows.map((row) => `${row.id}:${row.sortOrder}`);
}

describe("placeInStack", () => {
  it("gives the first banner 0 and the next one the following number", () => {
    expect(nextStackOrder([])).toBe(0);
    const first = placeInStack([], "a", 0, false);
    expect(orders(first)).toEqual(["a:0"]);
    expect(nextStackOrder(first)).toBe(1);
    expect(orders(placeInStack(first, "b", 1, false))).toEqual(["a:0", "b:1"]);
  });

  it("inserts at a used number and rewrites the list with no gaps", () => {
    expect(orders(placeInStack(stack, "d", 0, false))).toEqual(["d:0", "a:1", "b:2", "c:3"]);
  });

  it("moves the next banner into an unpublished banner's place", () => {
    expect(orders(placeInStack(stack, "b", 1, true))).toEqual(["a:0", "c:1", "b:2"]);
  });

  it("deletes a banner and closes the gap", () => {
    expect(orders(dropFromStack(stack, "b"))).toEqual(["a:0", "c:1"]);
    const parked = placeInStack(stack, "b", 1, true);
    expect(orders(dropFromStack(parked, "b"))).toEqual(["a:0", "c:1"]);
  });
});
