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
import {
  assignIncidentToMePlan,
  bulkIncidentPlan,
  bulkWatchListNeedsCurrent,
  nextWatchList,
} from "@features/csm-operations/utils/bulkIncidentEdit";

describe("nextWatchList", () => {
  it("adds people without duplicates, removes only the named ones, or replaces the list", () => {
    expect(nextWatchList(["a", "b"], "add", ["b", "c"])).toEqual(["a", "b", "c"]);
    expect(nextWatchList(["a", "b", "c"], "remove", ["b", "x"])).toEqual(["a", "c"]);
    expect(nextWatchList(["a"], "replace", ["c", "c", "d"])).toEqual(["c", "d"]);
  });
});

describe("bulkWatchListNeedsCurrent", () => {
  it("reads each incident's list only to add or remove people", () => {
    expect(bulkWatchListNeedsCurrent({ patch: {}, watchList: { mode: "add", userIds: ["a"] } })).toBe(true);
    expect(bulkWatchListNeedsCurrent({ patch: {}, watchList: { mode: "remove", userIds: ["a"] } })).toBe(true);
    expect(bulkWatchListNeedsCurrent({ patch: {}, watchList: { mode: "replace", userIds: ["a"] } })).toBe(false);
    expect(bulkWatchListNeedsCurrent({ patch: { state: "ON_HOLD" } })).toBe(false);
  });
});

describe("bulkIncidentPlan", () => {
  const open = { id: "inc-1", state: "IN_PROGRESS" };

  it("sends the fields that were set", () => {
    expect(bulkIncidentPlan(open, { patch: { subject: "S", problemId: "p-1" } })).toEqual({
      patch: { subject: "S", problemId: "p-1" },
    });
  });

  it("merges people into the incident's own watch list", () => {
    expect(bulkIncidentPlan(open, { patch: {}, watchList: { mode: "add", userIds: ["u2"] } }, ["u1"])).toEqual({
      patch: { watchList: ["u1", "u2"] },
    });
    expect(bulkIncidentPlan(open, { patch: {}, watchList: { mode: "remove", userIds: ["u1"] } }, ["u1", "u3"])).toEqual({
      patch: { watchList: ["u3"] },
    });
  });

  it("does not send a watch list that would not change, and skips the incident when nothing else is set", () => {
    expect(bulkIncidentPlan(open, { patch: { state: "ON_HOLD" }, watchList: { mode: "add", userIds: ["u1"] } }, ["u1"])).toEqual({
      patch: { state: "ON_HOLD" },
    });
    expect(bulkIncidentPlan(open, { patch: {}, watchList: { mode: "add", userIds: ["u1"] } }, ["u1"])).toEqual({
      skip: "Nothing to change: its watch list already is as asked.",
    });
  });

  it("leaves the watch list alone when it could not be read, rather than guess", () => {
    expect(bulkIncidentPlan(open, { patch: { state: "ON_HOLD" }, watchList: { mode: "add", userIds: ["u1"] } })).toEqual({
      skip: "Couldn't read its watch list, so it was not changed. Try again.",
    });
  });

  it("replaces the watch list without reading it", () => {
    expect(bulkIncidentPlan(open, { patch: {}, watchList: { mode: "replace", userIds: ["u9"] } })).toEqual({
      patch: { watchList: ["u9"] },
    });
  });

  it("never makes an incident its own parent, and never edits a closed one", () => {
    expect(bulkIncidentPlan(open, { patch: { parentIncidentId: "inc-1" } })).toEqual({
      skip: "An incident can't be its own parent incident.",
    });
    expect(bulkIncidentPlan(open, { patch: { parentIncidentId: "inc-9" } })).toEqual({
      patch: { parentIncidentId: "inc-9" },
    });
    expect(bulkIncidentPlan({ id: "inc-2", state: "CLOSED" }, { patch: { subject: "S" } })).toEqual({
      skip: "Closed or cancelled: it can't be edited.",
    });
  });
});

describe("assignIncidentToMePlan", () => {
  it("assigns an open incident, and skips one already assigned to the user or closed", () => {
    expect(assignIncidentToMePlan({ state: "NEW" }, "me")).toEqual({ patch: { assignedEngineerId: "me" } });
    expect(assignIncidentToMePlan({ state: "NEW", assignedTo: { id: "me" } }, "me")).toEqual({ skip: "Already assigned to you." });
    expect(assignIncidentToMePlan({ state: "CANCELLED" }, "me")).toEqual({ skip: "Closed or cancelled: it can't be edited." });
  });
});
