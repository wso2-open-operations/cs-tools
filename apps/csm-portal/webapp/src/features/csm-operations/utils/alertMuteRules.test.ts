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
import type { AlertMuteRuleInput } from "@features/csm-operations/types/alertMuteRules";
import {
  alertMuteRuleActions,
  alertMuteRuleState,
  formatDuration,
  latestAllowedEnd,
  matchesStateFilter,
  conditionsSummary,
  pickerInstant,
  pickerValuesInZone,
  timeZoneLabel,
  validateAlertMuteRule,
} from "@features/csm-operations/utils/alertMuteRules";

const NOW = Date.parse("2026-11-02T10:00:00Z");
const H = 3_600_000;
const iso = (ms: number): string => new Date(ms).toISOString();

function input(overrides: Partial<AlertMuteRuleInput> = {}): AlertMuteRuleInput {
  return {
    name: "Payments DB upgrade",
    description: "",
    timeZone: "Asia/Colombo",
    startOn: iso(NOW),
    endOn: iso(NOW + 2 * H),
    conditions: [
      { field: "service", value: "payments" },
      { field: "environment", value: "prod" },
    ],
    ...overrides,
  };
}

describe("alertMuteRuleState", () => {
  const rule = { startOn: iso(NOW), endOn: iso(NOW + 2 * H), endedOn: null };

  it("is scheduled before the start, active until the end, expired after", () => {
    expect(alertMuteRuleState(rule, NOW - 1)).toBe("scheduled");
    expect(alertMuteRuleState(rule, NOW)).toBe("active");
    expect(alertMuteRuleState(rule, NOW + 2 * H - 1)).toBe("active");
    expect(alertMuteRuleState(rule, NOW + 2 * H)).toBe("expired");
  });

  it("is ended once an engineer ended it, whatever the time", () => {
    expect(alertMuteRuleState({ ...rule, endedOn: iso(NOW + H) }, NOW + H)).toBe("ended");
  });
});

describe("matchesStateFilter", () => {
  it("files rules ended early under Expired", () => {
    expect(matchesStateFilter("ended", "expired")).toBe(true);
    expect(matchesStateFilter("expired", "expired")).toBe(true);
    expect(matchesStateFilter("active", "expired")).toBe(false);
    expect(matchesStateFilter("scheduled", "all")).toBe(true);
    expect(matchesStateFilter("scheduled", "active")).toBe(false);
  });
});

describe("validateAlertMuteRule", () => {
  it("accepts a complete rule", () => {
    expect(validateAlertMuteRule(input(), NOW, true)).toEqual({});
  });

  it("requires a name and time zone, but not a description", () => {
    const errors = validateAlertMuteRule(input({ name: " ", timeZone: "" }), NOW, true);
    expect(Object.keys(errors).sort()).toEqual(["name", "timeZone"]);
  });

  it("always needs service and environment conditions", () => {
    expect(validateAlertMuteRule(input({ conditions: [{ field: "service", value: "payments" }] }), NOW, true).conditions).toBe(
      "Add a condition for environment.",
    );
    expect(validateAlertMuteRule(input({ conditions: [] }), NOW, true).conditions).toBe(
      "Add a condition for service and environment.",
    );
  });

  it("needs a value per condition and each field once", () => {
    const errors = validateAlertMuteRule(
      input({
        conditions: [
          { field: "service", value: "payments" },
          { field: "environment", value: " " },
          { field: "service", value: "other" },
        ],
      }),
      NOW,
      true,
    );
    expect(errors.conditionRows).toEqual({ 1: "Pick an environment.", 2: "service is already a condition." });
  });

  it("limits the name to 255 characters and the description to 1024", () => {
    expect(validateAlertMuteRule(input({ name: "x".repeat(255) }), NOW, true).name).toBeUndefined();
    expect(validateAlertMuteRule(input({ name: "x".repeat(256) }), NOW, true).name).toMatch(/255/);
    expect(validateAlertMuteRule(input({ description: "x".repeat(1025) }), NOW, true).description).toMatch(/1024/);
  });

  it("caps a rule at 72 hours", () => {
    expect(validateAlertMuteRule(input({ endOn: iso(NOW + 72 * H) }), NOW, true).endOn).toBeUndefined();
    expect(validateAlertMuteRule(input({ endOn: iso(NOW + 72 * H + 60_000) }), NOW, true).endOn).toMatch(/at most 72 hours/);
  });

  it("refuses an end at or before the start", () => {
    expect(validateAlertMuteRule(input({ endOn: iso(NOW) }), NOW, true).endOn).toMatch(/after the start/);
  });

  it("refuses a past start for a new rule, but not when editing", () => {
    const past = input({ startOn: iso(NOW - H), endOn: iso(NOW + H) });
    expect(validateAlertMuteRule(past, NOW, true).startOn).toMatch(/past/);
    expect(validateAlertMuteRule(past, NOW, false).startOn).toBeUndefined();
  });

  it("allows a minute's grace for 'start now'", () => {
    expect(validateAlertMuteRule(input({ startOn: iso(NOW - 30_000) }), NOW, true).startOn).toBeUndefined();
  });
});

describe("alertMuteRuleActions", () => {
  it("edits only rules that haven't started, ends only running ones, and leaves finished ones alone", () => {
    expect(alertMuteRuleActions("scheduled")).toEqual(["edit", "extend", "delete"]);
    expect(alertMuteRuleActions("active")).toEqual(["extend", "end"]);
    expect(alertMuteRuleActions("expired")).toEqual([]);
    expect(alertMuteRuleActions("ended")).toEqual([]);
  });
});

describe("formatting", () => {
  it("formats durations", () => {
    expect(formatDuration(45 * 60_000)).toBe("45 min");
    expect(formatDuration(2 * H)).toBe("2 h");
    expect(formatDuration(90 * 60_000)).toBe("1 h 30 min");
  });

  it("summarises a rule's conditions", () => {
    expect(conditionsSummary(input().conditions)).toBe("service = payments, environment = prod");
  });

  it("labels a time zone with its UTC offset", () => {
    expect(timeZoneLabel("Asia/Colombo", NOW)).toBe("(UTC+05:30) Asia/Colombo");
    expect(timeZoneLabel("UTC", NOW)).toBe("(UTC+00:00) UTC");
  });

  it("allows extending to 72 hours after the start", () => {
    expect(latestAllowedEnd({ startOn: iso(NOW) })).toBe(NOW + 72 * H);
  });
});

describe("picker conversion", () => {
  it("reads the pickers' wall clock in the chosen zone, whatever the browser's zone", () => {
    const date = new Date(2026, 10, 3);
    const time = new Date(2026, 10, 3, 22, 30);
    expect(pickerInstant(date, time, "Asia/Colombo")).toBe("2026-11-03T17:00:00.000Z");
    expect(pickerInstant(date, time, "UTC")).toBe("2026-11-03T22:30:00.000Z");
    expect(pickerInstant(date, null, "UTC")).toBe("");
  });

  it("round-trips through pickerValuesInZone", () => {
    const { date, time } = pickerValuesInZone("2026-11-03T17:00:00.000Z", "Asia/Colombo");
    expect(pickerInstant(date, time, "Asia/Colombo")).toBe("2026-11-03T17:00:00.000Z");
  });
});
