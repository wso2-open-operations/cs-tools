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
  CALL_REQUEST_AGENT_ACTIONS,
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

  it("scheduled state offers reschedule + cancel; pending_on_wso2 offers schedule + reject", () => {
    expect(CALL_REQUEST_AGENT_ACTIONS[resolveCallRequestStateKey({ id: 3 })!]).toEqual([
      "reschedule",
      "cancel",
    ]);
    expect(CALL_REQUEST_AGENT_ACTIONS[resolveCallRequestStateKey({ id: 2 })!]).toEqual([
      "schedule",
      "reject",
    ]);
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
