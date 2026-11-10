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

import { useEffect, useState } from "react";
import { utcMsToZonedInputValue, zonedInputToUtcIso } from "@utils/dateTime";
import type {
  AlertMuteRuleCondition,
  AlertMuteRuleField,
  AlertMuteRuleInput,
  AlertMuteRule,
  AlertMuteRuleState,
  AlertMuteRuleStateFilter,
} from "@features/csm-operations/types/alertMuteRules";

/** The longest a rule may run: the same cap as a maintenance window. */
export const MAX_ALERT_MUTE_RULE_HOURS = 72;

const HOUR_MS = 3_600_000;

/** The fields a condition can match, in the order alert-core lists them. */
export const ALERT_MUTE_RULE_FIELDS: ReadonlyArray<{ value: AlertMuteRuleField; label: string }> = [
  { value: "service", label: "service" },
  { value: "environment", label: "environment" },
  { value: "source", label: "source" },
  { value: "metric_name", label: "metric_name" },
  { value: "severity", label: "severity" },
  { value: "category", label: "category" },
  { value: "unique_identifier", label: "unique_identifier" },
];

/** The Environment list. Values match alerts' spellings too (production = prod = PROD). */
export const ALERT_MUTE_RULE_ENVIRONMENTS: ReadonlyArray<{ value: string; label: string }> = [
  { value: "production", label: "Production" },
  { value: "staging", label: "Staging" },
  { value: "development", label: "Development" },
  { value: "qa", label: "QA" },
  { value: "uat", label: "UAT" },
  { value: "stress", label: "Stress" },
];

/** Every rule must name these, so one rule can't mute everything. */
export const REQUIRED_ALERT_MUTE_RULE_FIELDS: readonly AlertMuteRuleField[] = ["service", "environment"];

/** Suggestions per field, as alerts spell them. Any other value can be typed. */
export const ALERT_MUTE_RULE_VALUE_SUGGESTIONS: Partial<Record<AlertMuteRuleField, readonly string[]>> = {
  source: [
    "AWS",
    "Azure",
    "Datadog",
    "Elasticsearch",
    "GCP",
    "Icinga",
    "OpenObserve",
    "OpenSearch",
    "Prometheus",
    "Site24x7",
  ],
  severity: ["Critical", "Major", "Minor", "Warning"],
};

export const MAX_ALERT_MUTE_RULE_CONDITION_VALUE = 255;

export const ALERT_MUTE_RULE_STATE_FILTERS: ReadonlyArray<{ value: AlertMuteRuleStateFilter; label: string }> = [
  { value: "all", label: "All" },
  { value: "active", label: "Active" },
  { value: "scheduled", label: "Scheduled" },
  { value: "expired", label: "Expired" },
];

/** A rule's state at `now`: ended early, not started, running, or past its end. */
export function alertMuteRuleState(rule: Pick<AlertMuteRule, "startOn" | "endOn" | "endedOn">, now: number): AlertMuteRuleState {
  if (rule.endedOn) return "ended";
  if (now < Date.parse(rule.startOn)) return "scheduled";
  if (now < Date.parse(rule.endOn)) return "active";
  return "expired";
}

export function alertMuteRuleStateLabel(state: AlertMuteRuleState): string {
  switch (state) {
    case "scheduled":
      return "Scheduled";
    case "active":
      return "Active";
    case "expired":
      return "Expired";
    case "ended":
      return "Ended early";
  }
}

export function alertMuteRuleStateColor(state: AlertMuteRuleState): "success" | "info" | "default" {
  if (state === "active") return "success";
  if (state === "scheduled") return "info";
  return "default";
}

/** Whether a rule in `state` is shown under the list's `filter`. */
export function matchesStateFilter(state: AlertMuteRuleState, filter: AlertMuteRuleStateFilter): boolean {
  if (filter === "all") return true;
  if (filter === "expired") return state === "expired" || state === "ended";
  return state === filter;
}

/** "2 h", "1 h 30 min", "45 min". */
export function formatDuration(ms: number): string {
  const totalMin = Math.max(0, Math.round(ms / 60_000));
  const h = Math.floor(totalMin / 60);
  const m = totalMin % 60;
  if (h === 0) return `${m} min`;
  return m === 0 ? `${h} h` : `${h} h ${m} min`;
}

/** "service = payments, environment = prod, source = Site24x7". */
export function conditionsSummary(conditions: readonly AlertMuteRuleCondition[]): string {
  return conditions.map((c) => `${c.field} = ${c.value}`).join(", ");
}

/** A condition's value, or "" when the rule has none for that field. */
export function conditionValue(conditions: readonly AlertMuteRuleCondition[], field: AlertMuteRuleField): string {
  return conditions.find((c) => c.field === field)?.value ?? "";
}

/** The same limits as CloudWatch's mute rules. */
export const MAX_ALERT_MUTE_RULE_NAME = 255;
export const MAX_ALERT_MUTE_RULE_DESCRIPTION = 1024;

/** Errors keyed by field; empty when the input can be saved. */
export type AlertMuteRuleErrors = Partial<
  Record<"name" | "description" | "conditions" | "timeZone" | "startOn" | "endOn", string>
> & {
  /** Per condition row, by index. */
  conditionRows?: Record<number, string>;
};

/**
 * The rules a rule must follow (entity-service checks the same): a name of
 * 1-255 characters, a description of at most 1024, a time zone, conditions
 * with a value each, no field twice, and always service and environment, and an end after
 * its start, in the future, at most {@link MAX_ALERT_MUTE_RULE_HOURS} after it.
 * `isNew` also refuses a start in the past (beyond a minute's grace).
 */
export function validateAlertMuteRule(input: AlertMuteRuleInput, now: number, isNew: boolean): AlertMuteRuleErrors {
  const errors: AlertMuteRuleErrors = {};
  const name = input.name.trim();
  if (!name) errors.name = "Enter a name.";
  else if (name.length > MAX_ALERT_MUTE_RULE_NAME) errors.name = `Use at most ${MAX_ALERT_MUTE_RULE_NAME} characters.`;
  if (input.description.length > MAX_ALERT_MUTE_RULE_DESCRIPTION)
    errors.description = `Use at most ${MAX_ALERT_MUTE_RULE_DESCRIPTION} characters.`;
  const rowErrors: Record<number, string> = {};
  const seen = new Set<AlertMuteRuleField>();
  input.conditions.forEach((c, i) => {
    const value = c.value.trim();
    if (!value)
      rowErrors[i] =
        c.field === "service" ? "Pick a service." : c.field === "environment" ? "Pick an environment." : "Enter a value.";
    else if (value.length > MAX_ALERT_MUTE_RULE_CONDITION_VALUE)
      rowErrors[i] = `Use at most ${MAX_ALERT_MUTE_RULE_CONDITION_VALUE} characters.`;
    else if (seen.has(c.field)) rowErrors[i] = `${c.field} is already a condition.`;
    seen.add(c.field);
  });
  if (Object.keys(rowErrors).length > 0) errors.conditionRows = rowErrors;
  const missing = REQUIRED_ALERT_MUTE_RULE_FIELDS.filter((f) => !input.conditions.some((c) => c.field === f));
  if (missing.length > 0) errors.conditions = `Add a condition for ${missing.join(" and ")}.`;
  if (!input.timeZone) errors.timeZone = "Pick a time zone.";

  const start = Date.parse(input.startOn);
  const end = Date.parse(input.endOn);
  if (Number.isNaN(start)) errors.startOn = "Enter a start date and time.";
  else if (isNew && start < now - 60_000) errors.startOn = "The start can't be in the past.";
  if (Number.isNaN(end)) errors.endOn = "Enter an end date and time.";
  else if (!Number.isNaN(start)) {
    if (end <= start) errors.endOn = "The end must be after the start.";
    else if (end - start > MAX_ALERT_MUTE_RULE_HOURS * HOUR_MS)
      errors.endOn = `A rule can run for at most ${MAX_ALERT_MUTE_RULE_HOURS} hours.`;
    else if (end <= now) errors.endOn = "The end must be in the future.";
  }
  return errors;
}

const pad = (n: number): string => String(n).padStart(2, "0");

/**
 * The instant a date picker's day and a time picker's hh:mm mean in
 * `timeZone`, as a UTC ISO string ("" until both are set). Only the pickers'
 * wall-clock parts are read, so the browser's own zone doesn't matter.
 */
export function pickerInstant(date: Date | null, time: Date | null, timeZone: string): string {
  if (!date || !time || Number.isNaN(date.getTime()) || Number.isNaN(time.getTime())) return "";
  const local = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(time.getHours())}:${pad(time.getMinutes())}`;
  return zonedInputToUtcIso(local, timeZone) ?? "";
}

/** The inverse of {@link pickerInstant}: picker values showing `iso`'s wall clock in `timeZone`. */
export function pickerValuesInZone(iso: string, timeZone: string): { date: Date; time: Date } {
  const [d, t] = utcMsToZonedInputValue(Date.parse(iso), timeZone).split("T");
  const [y, mo, day] = d.split("-").map(Number);
  const [h, mi] = t.split(":").map(Number);
  return { date: new Date(y, mo - 1, day), time: new Date(y, mo - 1, day, h, mi) };
}

/** "(UTC+05:30) Asia/Colombo", as CloudWatch lists zones. */
export function timeZoneLabel(timeZone: string, at: number = Date.now()): string {
  try {
    const part = new Intl.DateTimeFormat("en-US", { timeZone, timeZoneName: "longOffset" })
      .formatToParts(new Date(at))
      .find((p) => p.type === "timeZoneName")?.value;
    const offset = !part || part === "GMT" ? "+00:00" : part.replace(/^GMT/, "");
    return `(UTC${offset}) ${timeZone}`;
  } catch {
    return timeZone;
  }
}

/**
 * How far a running or scheduled rule can be extended: up to
 * {@link MAX_ALERT_MUTE_RULE_HOURS} after its start. Returns the latest end, in ms.
 */
export function latestAllowedEnd(rule: Pick<AlertMuteRule, "startOn">): number {
  return Date.parse(rule.startOn) + MAX_ALERT_MUTE_RULE_HOURS * HOUR_MS;
}


export type AlertMuteRuleAction = "end" | "extend" | "edit" | "delete";

/**
 * What can be done to a rule in each state. A scheduled rule can still be
 * edited, extended or deleted; a running one only extended or ended (its match
 * stays as it was while alerts are being held); a finished one is kept as a
 * record and has no actions.
 */
export function alertMuteRuleActions(state: AlertMuteRuleState): AlertMuteRuleAction[] {
  switch (state) {
    case "scheduled":
      return ["edit", "extend", "delete"];
    case "active":
      return ["extend", "end"];
    default:
      return [];
  }
}

/** The current time, re-read every `intervalMs`, so a rule's state moves on by itself. */
export function useNow(intervalMs = 30_000): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), intervalMs);
    return () => window.clearInterval(timer);
  }, [intervalMs]);
  return now;
}
