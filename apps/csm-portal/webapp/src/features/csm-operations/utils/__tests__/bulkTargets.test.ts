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
import { bulkChangeRequestTargets, isChangeRequestTerminal } from "@features/csm-operations/utils/changeRequests";
import { bulkIncidentTargetStates, isIncidentTerminal } from "@features/csm-operations/utils/incidents";

// Which records a bulk edit may touch, and which moves it may offer for a selection.

describe("isChangeRequestTerminal", () => {
  it.each(["closed", "canceled", "rollback"])("%s is final", (state) => {
    expect(isChangeRequestTerminal(state)).toBe(true);
  });
  it.each(["new", "assess", "authorize", "customer_approval", "scheduled", "implement", "review", "customer_review"])(
    "%s is not",
    (state) => {
      expect(isChangeRequestTerminal(state)).toBe(false);
    },
  );
  it("does not call an unknown or missing state final", () => {
    expect(isChangeRequestTerminal(null)).toBe(false);
    expect(isChangeRequestTerminal(undefined)).toBe(false);
  });
});

describe("bulkChangeRequestTargets", () => {
  it("offers the moves every selected change request can make, in the first one's order", () => {
    expect(
      bulkChangeRequestTargets([
        { state: "review", legalNextStates: ["closed", "rollback", "canceled"] },
        { state: "customer_review", legalNextStates: ["rollback", "canceled"] },
      ]),
    ).toEqual(["rollback", "canceled"]);
  });

  it("offers Close, Cancel change and Roll back when legal for all of them", () => {
    expect(
      bulkChangeRequestTargets([
        { state: "review", legalNextStates: ["closed", "rollback", "canceled"] },
        { state: "review", legalNextStates: ["closed", "rollback", "canceled"] },
      ]),
    ).toEqual(["closed", "rollback", "canceled"]);
  });

  it("never offers Re-schedule, nor a move the action bar never offers", () => {
    expect(
      bulkChangeRequestTargets([
        { state: "customer_approval", legalNextStates: ["authorize", "scheduled", "canceled"] },
      ]),
    ).toEqual(["canceled"]);
    // Out of Customer Review a Close is the customer's own answer.
    expect(
      bulkChangeRequestTargets([{ state: "customer_review", legalNextStates: ["closed", "rollback", "canceled"] }]),
    ).toEqual(["rollback", "canceled"]);
  });

  it("offers nothing when they share no move, when one's moves are unknown, or for no selection", () => {
    expect(
      bulkChangeRequestTargets([
        { state: "new", legalNextStates: ["assess"] },
        { state: "scheduled", legalNextStates: ["implement"] },
      ]),
    ).toEqual([]);
    expect(bulkChangeRequestTargets([{ state: "new", legalNextStates: ["assess"] }, { state: "new" }])).toEqual([]);
    expect(bulkChangeRequestTargets([])).toEqual([]);
  });
});

describe("isIncidentTerminal", () => {
  it("is true for Closed and Cancelled only", () => {
    expect(isIncidentTerminal("CLOSED")).toBe(true);
    expect(isIncidentTerminal("CANCELLED")).toBe(true);
    for (const state of ["NEW", "IN_PROGRESS", "ON_HOLD", "RESOLVED", null, undefined]) {
      expect(isIncidentTerminal(state)).toBe(false);
    }
  });
});

describe("bulkIncidentTargetStates", () => {
  it("offers the states every selected incident can move to", () => {
    expect(bulkIncidentTargetStates(["NEW", "ON_HOLD"])).toEqual(["IN_PROGRESS"]);
    expect(bulkIncidentTargetStates(["IN_PROGRESS", "ON_HOLD"])).toEqual(["CANCELLED"]);
    expect(bulkIncidentTargetStates(["IN_PROGRESS"])).toEqual(["ON_HOLD", "RESOLVED", "CANCELLED"]);
  });

  it("offers Closed only when every one of them is Resolved", () => {
    expect(bulkIncidentTargetStates(["RESOLVED", "RESOLVED"])).toEqual(["IN_PROGRESS", "CLOSED"]);
    expect(bulkIncidentTargetStates(["RESOLVED", "IN_PROGRESS"])).toEqual([]);
  });

  it("offers nothing for a closed incident, an unknown state, or no selection", () => {
    expect(bulkIncidentTargetStates(["NEW", "CLOSED"])).toEqual([]);
    expect(bulkIncidentTargetStates(["NEW", "SOMETHING_ELSE"])).toEqual([]);
    expect(bulkIncidentTargetStates(["NEW", null])).toEqual([]);
    expect(bulkIncidentTargetStates([])).toEqual([]);
  });
});
