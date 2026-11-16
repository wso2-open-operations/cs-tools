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

import { describe, expect, it } from "vitest";
import {
  actionResultTone,
  closureEventLabel,
  parseClosureProcessState,
} from "@features/csm-projects/utils/closureProcessState";

describe("closureEventLabel", () => {
  it.each([
    ["30_days_notice", null, "30-day notice sent"],
    ["90_days_notice", null, "90-day notice sent"],
    ["90_DAYS_NOTICE", null, "90-day notice sent"],
    ["suspend_notice", null, "Suspension notice sent"],
    ["suspend", "2026-03-05", "Suspended on Mar 5, 2026"],
    ["suspend", null, "Suspended"],
    ["suspend", "not-a-date", "Suspended on not-a-date"],
    ["open", null, "Open (no notice yet)"],
    ["  open ", null, "Open (no notice yet)"],
    ["something_new", null, "something_new"],
    ["", null, "No event recorded"],
    [null, null, "No event recorded"],
    [undefined, null, "No event recorded"],
  ])("%j (suspended_on %j) -> %s", (eventType, suspendedOn, expected) => {
    expect(closureEventLabel(eventType as string | null | undefined, suspendedOn)).toBe(expected);
  });
});

describe("actionResultTone", () => {
  it.each([
    ["SUCCESSFUL", "success"],
    ["successful", "success"],
    ["FAILED", "error"],
    ["PENDING", "warning"],
    ["IGNORED", "default"],
    ["WHATEVER", "default"],
  ])("%s -> %s", (result, tone) => {
    expect(actionResultTone(result)).toBe(tone);
  });
});

describe("parseClosureProcessState", () => {
  it.each([[null], [undefined], [""], ["   "], ["null"], [{}], ["{}"]])(
    "reads %j as empty",
    (input) => {
      expect(parseClosureProcessState(input)).toEqual({ kind: "empty" });
    },
  );

  it("parses the three known tracks with their actions", () => {
    const result = parseClosureProcessState({
      based_on_subscription_end_date: {
        event_type: "30_days_notice",
        actionSendEmailNotification: "SUCCESSFUL",
        actionServicePortalAnnouncement: "IGNORED",
      },
      based_on_due_invoices: {
        event_type: "suspend",
        actionSuspendProject: "SUCCESSFUL",
        suspended_on: "2026-02-01",
      },
      based_on_compliance: { event_type: "open" },
    });
    expect(result.kind).toBe("tracks");
    if (result.kind !== "tracks") return;
    expect(result.leftover).toBeNull();
    expect(result.tracks.map((t) => [t.title, t.label])).toEqual([
      ["Subscription end date", "30-day notice sent"],
      ["Due invoices", "Suspended on Feb 1, 2026"],
      ["Compliance", "Open (no notice yet)"],
    ]);
    expect(result.tracks[0]?.actions).toEqual([
      { key: "actionSendEmailNotification", title: "Email notification", result: "Successful", tone: "success" },
      { key: "actionServicePortalAnnouncement", title: "Portal announcement", result: "Ignored", tone: "default" },
    ]);
    expect(result.tracks[2]?.actions).toEqual([]);
  });

  it("accepts a JSON string instead of an object", () => {
    const result = parseClosureProcessState(
      JSON.stringify({ based_on_compliance: { event_type: "suspend_notice" } }),
    );
    expect(result).toMatchObject({
      kind: "tracks",
      tracks: [{ key: "based_on_compliance", label: "Suspension notice sent" }],
    });
  });

  it("keeps an unknown track key as a track with a humanised title", () => {
    const result = parseClosureProcessState({ based_on_new_thing: { event_type: "open" } });
    expect(result).toMatchObject({
      kind: "tracks",
      tracks: [{ title: "New thing", label: "Open (no notice yet)" }],
    });
  });

  it("surfaces unknown keys and malformed sections as leftover, never drops them", () => {
    const result = parseClosureProcessState({
      based_on_compliance: { event_type: "open" },
      based_on_due_invoices: "oops",
      extra: 5,
      empty_section: {},
    });
    expect(result.kind).toBe("tracks");
    if (result.kind !== "tracks") return;
    expect(result.tracks).toHaveLength(1);
    expect(JSON.parse(result.leftover ?? "null")).toEqual({
      based_on_due_invoices: "oops",
      extra: 5,
      empty_section: {},
    });
  });

  it("tolerates a track with missing fields and non-string values", () => {
    const result = parseClosureProcessState({
      based_on_compliance: { event_type: 7, actionSuspendProject: { nested: true } },
    });
    // Numeric event type is read as text; the object-valued action is ignored.
    expect(result).toMatchObject({
      kind: "tracks",
      tracks: [{ eventType: "7", label: "7", actions: [] }],
    });
  });

  it("falls back to unrecognised, verbatim, for shapes it cannot read", () => {
    expect(parseClosureProcessState("not json")).toEqual({ kind: "unrecognised", raw: "not json" });
    expect(parseClosureProcessState([1, 2])).toEqual({
      kind: "unrecognised",
      raw: JSON.stringify([1, 2], null, 2),
    });
    expect(parseClosureProcessState(42)).toEqual({ kind: "unrecognised", raw: "42" });
    expect(parseClosureProcessState({ foo: { bar: 1 } })).toEqual({
      kind: "unrecognised",
      raw: JSON.stringify({ foo: { bar: 1 } }, null, 2),
    });
    expect(parseClosureProcessState('"quoted"')).toEqual({ kind: "unrecognised", raw: '"quoted"' });
  });
});
