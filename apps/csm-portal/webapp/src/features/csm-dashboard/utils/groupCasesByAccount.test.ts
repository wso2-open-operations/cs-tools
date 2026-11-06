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
import { groupCasesByAccount, NO_ACCOUNT_LABEL } from "@features/csm-dashboard/utils/groupCasesByAccount";

const c = (id: string, account?: { id?: string; name?: string }): Record<string, unknown> => ({
  id,
  ...(account ? { account } : {}),
});

describe("groupCasesByAccount", () => {
  it("returns no groups for no rows", () => {
    expect(groupCasesByAccount([])).toEqual([]);
  });

  it("groups by account id and sorts by case count descending", () => {
    const groups = groupCasesByAccount([
      c("1", { id: "a", name: "Acme" }),
      c("2", { id: "b", name: "Beta" }),
      c("3", { id: "b", name: "Beta" }),
      c("4", { id: "b", name: "Beta" }),
      c("5", { id: "a", name: "Acme" }),
      c("6", { id: "z", name: "Zed" }),
    ]);
    expect(groups.map((g) => [g.accountName, g.items.length])).toEqual([
      ["Beta", 3],
      ["Acme", 2],
      ["Zed", 1],
    ]);
    expect(groups[0].items.map((i) => i.id)).toEqual(["2", "3", "4"]);
  });

  it("breaks count ties by account name", () => {
    const groups = groupCasesByAccount([c("1", { id: "z", name: "Zed" }), c("2", { id: "a", name: "Acme" })]);
    expect(groups.map((g) => g.accountName)).toEqual(["Acme", "Zed"]);
  });

  it("keeps cases with no account in one labelled group so counts still add up", () => {
    const groups = groupCasesByAccount([c("1"), c("2", { id: "", name: "" }), c("3", { id: "a", name: "Acme" })]);
    expect(groups.map((g) => [g.accountName, g.items.length])).toEqual([
      [NO_ACCOUNT_LABEL, 2],
      ["Acme", 1],
    ]);
  });

  it("falls back to the name when an account has no id", () => {
    const groups = groupCasesByAccount([c("1", { name: "Acme" }), c("2", { name: "Acme" })]);
    expect(groups).toHaveLength(1);
    expect(groups[0].items).toHaveLength(2);
  });

  it("labels an account that has an id but no name by its id, as separate rows", () => {
    const groups = groupCasesByAccount([
      c("1", { id: "acc-2" }),
      c("2", { id: "acc-1" }),
      c("3", { id: "acc-1", name: "  " }),
      c("4"),
    ]);
    expect(groups.map((g) => [g.accountId, g.accountName, g.items.length])).toEqual([
      ["acc-1", "acc-1", 2],
      ["acc-2", "acc-2", 1],
      ["", NO_ACCOUNT_LABEL, 1],
    ]);
  });
});
