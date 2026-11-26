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
} from "@features/csm-operations/utils/bulkIncidentEdit";

describe("bulkIncidentPlan", () => {
  const open = { id: "inc-1", state: "IN_PROGRESS" };

  it("sends the fields that were set", () => {
    expect(bulkIncidentPlan(open, { patch: { subject: "S", problemId: "p-1" } })).toEqual({
      patch: { subject: "S", problemId: "p-1" },
    });
  });

  it("adds and removes people as a delta, never a whole list that could drop a concurrent watcher", () => {
    expect(bulkIncidentPlan(open, { patch: {}, watchList: { mode: "add", userIds: ["u2", "u2"] } })).toEqual({
      patch: { addWatchers: ["u2"] },
    });
    expect(bulkIncidentPlan(open, { patch: {}, watchList: { mode: "remove", userIds: ["u1"] } })).toEqual({
      patch: { removeWatchers: ["u1"] },
    });
  });

  it("sends a watch list delta together with the other fields", () => {
    expect(bulkIncidentPlan(open, { patch: { state: "ON_HOLD" }, watchList: { mode: "add", userIds: ["u1"] } })).toEqual({
      patch: { state: "ON_HOLD", addWatchers: ["u1"] },
    });
  });

  it("skips an incident when there is nothing to send", () => {
    expect(bulkIncidentPlan(open, { patch: {} })).toEqual({ skip: "Nothing to change." });
  });

  it("replaces the watch list with the whole list", () => {
    expect(bulkIncidentPlan(open, { patch: {}, watchList: { mode: "replace", userIds: ["u9", "u9"] } })).toEqual({
      patch: { watchList: ["u9"] },
    });
  });

  it("never makes an incident its own parent, and never edits a closed one", () => {
    expect(bulkIncidentPlan(open, { patch: { parentId: "inc-1" } })).toEqual({
      skip: "An incident can't be its own parent incident.",
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
