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
  ALL_CALL_REQUEST_STATES,
  CALL_REQUEST_AGENT_ACTIONS,
  caseAcceptsCallRequests,
  callRequestCaseStateBlockReason,
  callRequestStateColor,
  callRequestStateLabel,
  resolveCallRequestStateKey,
} from "./callRequestState";

describe("resolveCallRequestStateKey", () => {
  it("maps the integer choice keys the backing data source returns", () => {
    // The data source passes its native state through untranslated, so state.id
    // is an integer, not our string key.
    expect(resolveCallRequestStateKey({ id: 1 })).toBe("pending_on_customer");
    expect(resolveCallRequestStateKey({ id: 2 })).toBe("pending_on_wso2");
    expect(resolveCallRequestStateKey({ id: 3 })).toBe("scheduled");
    expect(resolveCallRequestStateKey({ id: 7 })).toBe("notes_pending");
    expect(resolveCallRequestStateKey({ id: 8 })).toBe("concluded");
    // string form of the integer key resolves too
    expect(resolveCallRequestStateKey({ id: "5" })).toBe("wso2_rejected");
  });

  it("passes through when the id is already our string enum key", () => {
    expect(resolveCallRequestStateKey({ id: "scheduled" })).toBe("scheduled");
    expect(resolveCallRequestStateKey({ id: "canceled" })).toBe("canceled");
  });

  it("returns null for unknown ids and undefined state", () => {
    expect(resolveCallRequestStateKey({ id: 99 })).toBeNull();
    expect(resolveCallRequestStateKey({ id: "bogus" })).toBeNull();
    expect(resolveCallRequestStateKey(undefined)).toBeNull();
  });

  it("does not resolve a key from the display label (label is display-only)", () => {
    // The FE label table is worded independently of the data source's labels, so
    // label is not a reliable key source; only state.id resolves a key. An
    // unmappable id is not rescued by a label.
    expect(resolveCallRequestStateKey({ id: 99, label: "Scheduled" })).toBeNull();
    // The label is still honoured for display, though:
    expect(callRequestStateLabel({ id: 99, label: "Scheduled" })).toBe("Scheduled");
  });

  it("regression: agent-action lookup is always an array, never undefined", () => {
    // The crash was CALL_REQUEST_AGENT_ACTIONS[String(state.id)] === undefined
    // then reading .length. Resolving first and defaulting to [] prevents it.
    for (const id of [1, 2, 3, 4, 5, 6, 7, 8, 99]) {
      const key = resolveCallRequestStateKey({ id });
      const actions = (key && CALL_REQUEST_AGENT_ACTIONS[key]) ?? [];
      expect(Array.isArray(actions)).toBe(true);
    }
  });

  it("scheduled state offers reschedule + complete + cancel; pending_on_wso2 offers schedule + reject", () => {
    expect(CALL_REQUEST_AGENT_ACTIONS[resolveCallRequestStateKey({ id: 3 })!]).toEqual([
      "reschedule",
      "complete",
      "cancel",
    ]);
    expect(CALL_REQUEST_AGENT_ACTIONS[resolveCallRequestStateKey({ id: 2 })!]).toEqual([
      "schedule",
      "reject",
    ]);
  });

  it("\"Mark as completed\" is offered exactly where the backend accepts a notes-less conclude", () => {
    // entity-service only applies a conclude with no notes to a scheduled or
    // notes-pending call and answers 409 for any other state, so offering it
    // anywhere else would be a button that always fails.
    const withComplete = ALL_CALL_REQUEST_STATES.filter((k) =>
      CALL_REQUEST_AGENT_ACTIONS[k].includes("complete"),
    );
    expect(withComplete.sort()).toEqual(["notes_pending", "scheduled"]);
  });

  it("notes_pending keeps 'send call notes' and adds 'complete'", () => {
    expect(CALL_REQUEST_AGENT_ACTIONS.notes_pending).toEqual(["sendNotes", "complete"]);
  });
});

describe("callRequestStateLabel / callRequestStateColor", () => {
  it("prefers the backend label", () => {
    expect(callRequestStateLabel({ id: 3, label: "Scheduled" })).toBe("Scheduled");
  });

  it("falls back to our label/color via the numeric id when label is absent", () => {
    expect(callRequestStateLabel({ id: 3 })).toBe("Scheduled");
    expect(callRequestStateColor({ id: 3 })).toBe("primary");
    expect(callRequestStateColor({ id: 5 })).toBe("error");
  });

  it("degrades gracefully for unknown states", () => {
    expect(callRequestStateLabel(undefined)).toBe("Unknown");
    expect(callRequestStateColor({ id: 99 })).toBe("default");
    expect(callRequestStateLabel({ id: 99 })).toBe("99");
  });
});

describe("caseAcceptsCallRequests / callRequestCaseStateBlockReason", () => {
  it("allows the 5 states the data source permits a call request in", () => {
    for (const state of [
      "work_in_progress",
      "awaiting_info",
      "waiting_on_wso2",
      "solution_proposed",
      "reopened",
    ] as const) {
      expect(caseAcceptsCallRequests(state)).toBe(true);
      expect(callRequestCaseStateBlockReason(state)).toBeNull();
    }
  });

  it("blocks other case states with a reason listing the allowed states", () => {
    for (const state of ["open", "closed"] as const) {
      expect(caseAcceptsCallRequests(state)).toBe(false);
      const reason = callRequestCaseStateBlockReason(state);
      expect(reason).toContain("Work in progress");
      expect(reason).toContain("Awaiting info");
      expect(reason).toContain("Waiting on WSO2");
      expect(reason).toContain("Solution proposed");
      expect(reason).toContain("Reopened");
    }
  });

  it("is permissive when the state is unknown (undefined) -- the backend still enforces the rule", () => {
    expect(caseAcceptsCallRequests(undefined)).toBe(true);
    expect(callRequestCaseStateBlockReason(undefined)).toBeNull();
  });
});
