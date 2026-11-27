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

import type {
  BeSreAlertIncidentFilter,
  BeSreAlertSearchPayload,
  BeSreAlertTier,
} from "@api/backend/types";

/** The list's own path: the Operations tab whose nav key is `alerts`. */
export const SRE_ALERTS_LIST_PATH = "/operations/alerts";

export const SRE_ALERT_TIERS: BeSreAlertTier[] = ["priority", "standard"];
export const SRE_ALERT_INCIDENT_FILTERS: BeSreAlertIncidentFilter[] = ["all", "linked", "unlinked"];

export const SRE_ALERT_TIER_LABELS: Record<BeSreAlertTier, string> = {
  priority: "Priority",
  standard: "Standard",
};

export const SRE_ALERT_INCIDENT_FILTER_LABELS: Record<BeSreAlertIncidentFilter, string> = {
  all: "All alerts",
  linked: "Linked to an incident",
  unlinked: "Not linked to an incident",
};

/** Filter state of the Alerts tab. Dates are `YYYY-MM-DD` UTC calendar days ("" = unbounded). */
export interface SreAlertFilters {
  search: string;
  environments: string[];
  services: string[];
  severities: string[];
  sources: string[];
  tiers: BeSreAlertTier[];
  incident: BeSreAlertIncidentFilter;
  receivedFrom: string;
  receivedTo: string;
}

export const DEFAULT_SRE_ALERT_FILTERS: SreAlertFilters = {
  search: "",
  environments: [],
  services: [],
  severities: [],
  sources: [],
  tiers: [],
  incident: "all",
  receivedFrom: "",
  receivedTo: "",
};

type ChipColor = "default" | "error" | "warning" | "success";

/** Chip colour of a severity: the source's own word, so anything unknown stays neutral. */
export function sreAlertSeverityColor(severity: string | null | undefined): ChipColor {
  switch ((severity ?? "").trim().toLowerCase()) {
    case "critical":
    case "major":
      return "error";
    case "warning":
    case "minor":
      return "warning";
    case "ok":
    case "clear":
    case "resolved":
      return "success";
    default:
      return "default";
  }
}

/** A severity as shown: the source's word with its first letter capitalised. */
export function sreAlertSeverityLabel(severity: string | null | undefined): string {
  const s = (severity ?? "").trim();
  return s ? s.charAt(0).toUpperCase() + s.slice(1) : "";
}

/** A tier as shown; an unknown value is shown as it came. */
export function sreAlertTierLabel(tier: string | null | undefined): string {
  if (!tier) return "";
  return SRE_ALERT_TIER_LABELS[tier as BeSreAlertTier] ?? tier;
}

/** How many filters are set, for the Filters button (search is not counted, as on the other tabs). */
export function countActiveSreAlertFilters(f: SreAlertFilters): number {
  let n = 0;
  if (f.environments.length) n++;
  if (f.services.length) n++;
  if (f.severities.length) n++;
  if (f.sources.length) n++;
  if (f.tiers.length) n++;
  if (f.incident !== "all") n++;
  if (f.receivedFrom || f.receivedTo) n++;
  return n;
}

/** The day after a `YYYY-MM-DD` date, as `YYYY-MM-DD`. */
function nextDay(dateOnly: string): string {
  const [y, m, d] = dateOnly.split("-").map(Number);
  return new Date(Date.UTC(y, m - 1, d + 1)).toISOString().slice(0, 10);
}

/**
 * The search's `filters`. The received range is whole UTC days: from the start
 * of the first day (inclusive) to the start of the day after the last
 * (exclusive, as the backend reads `receivedTo`).
 */
export function buildSreAlertSearchFilters(
  f: SreAlertFilters,
  searchQuery: string,
): NonNullable<BeSreAlertSearchPayload["filters"]> {
  return {
    ...(searchQuery && { searchQuery }),
    ...(f.environments.length && { environments: f.environments }),
    ...(f.services.length && { services: f.services }),
    ...(f.severities.length && { severities: f.severities }),
    ...(f.sources.length && { sources: f.sources }),
    ...(f.tiers.length && { tiers: f.tiers }),
    ...(f.incident !== "all" && { incident: f.incident }),
    ...(f.receivedFrom && { receivedFrom: `${f.receivedFrom}T00:00:00Z` }),
    ...(f.receivedTo && { receivedTo: `${nextDay(f.receivedTo)}T00:00:00Z` }),
  };
}

// URL params owned by the Alerts tab's filters. Prefixed (`alt...`) so they
// can't collide with another Operations tab's params in the same URL.
export const SRE_ALERT_FILTER_PARAM_KEYS = [
  "altQ",
  "altEnvironments",
  "altServices",
  "altSeverities",
  "altSources",
  "altTiers",
  "altIncident",
  "altReceivedFrom",
  "altReceivedTo",
] as const;

const DATE_ONLY_RE = /^(\d{4})-(\d{2})-(\d{2})$/;

/** A real `YYYY-MM-DD` calendar date, else "". */
function parseDateOnly(raw: string | null): string {
  const match = raw ? DATE_ONLY_RE.exec(raw) : null;
  if (!match) return "";
  const [, year, month, day] = match;
  const date = new Date(Date.UTC(Number(year), Number(month) - 1, Number(day)));
  const isRealDate =
    date.getUTCFullYear() === Number(year) &&
    date.getUTCMonth() === Number(month) - 1 &&
    date.getUTCDate() === Number(day);
  return isRealDate ? raw! : "";
}

/**
 * A multi-value param, one URL entry per value (`?altServices=a&altServices=b`):
 * facet values are free text from the alert sources and may contain a comma.
 * Blank and repeated entries are dropped.
 */
function parseValues(params: URLSearchParams, key: string): string[] {
  const values = params
    .getAll(key)
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
  return [...new Set(values)];
}

/**
 * Read the Alerts tab's filters from the URL. An unknown tier or incident
 * value, a malformed date, or an end before the start is dropped, so a stale
 * or hand-edited link falls back to unfiltered rather than to a 400.
 */
export function readSreAlertFiltersFromUrl(params: URLSearchParams): SreAlertFilters {
  const receivedFrom = parseDateOnly(params.get("altReceivedFrom"));
  let receivedTo = parseDateOnly(params.get("altReceivedTo"));
  if (receivedFrom && receivedTo && receivedFrom > receivedTo) receivedTo = "";
  const incident = params.get("altIncident");
  return {
    search: params.get("altQ") ?? "",
    environments: parseValues(params, "altEnvironments"),
    services: parseValues(params, "altServices"),
    severities: parseValues(params, "altSeverities"),
    sources: parseValues(params, "altSources"),
    tiers: parseValues(params, "altTiers").filter((t): t is BeSreAlertTier =>
      (SRE_ALERT_TIERS as string[]).includes(t),
    ),
    incident: (SRE_ALERT_INCIDENT_FILTERS as Array<string | null>).includes(incident)
      ? (incident as BeSreAlertIncidentFilter)
      : "all",
    receivedFrom,
    receivedTo,
  };
}

/** The URL params for these filters; defaults are left out so the URL stays clean. */
export function writeSreAlertFiltersToUrl(f: SreAlertFilters): URLSearchParams {
  const out = new URLSearchParams();
  if (f.search) out.set("altQ", f.search);
  f.environments.forEach((v) => out.append("altEnvironments", v));
  f.services.forEach((v) => out.append("altServices", v));
  f.severities.forEach((v) => out.append("altSeverities", v));
  f.sources.forEach((v) => out.append("altSources", v));
  f.tiers.forEach((v) => out.append("altTiers", v));
  if (f.incident !== "all") out.set("altIncident", f.incident);
  if (f.receivedFrom) out.set("altReceivedFrom", f.receivedFrom);
  if (f.receivedTo) out.set("altReceivedTo", f.receivedTo);
  return out;
}

/**
 * The linked incident's state in the incident helpers' spelling: entity-service
 * may write CANCELED where the incident pages say CANCELLED.
 */
export function sreAlertIncidentState(state: string | null | undefined): string {
  const s = (state ?? "").trim().toUpperCase();
  return s === "CANCELED" ? "CANCELLED" : s;
}

/** An impact or urgency enum (HIGH, MEDIUM, …) as shown: "High"; "" when unset. */
export function sreAlertIncidentLevelLabel(value: string | null | undefined): string {
  const s = (value ?? "").trim();
  if (!s) return "";
  return s.charAt(0).toUpperCase() + s.slice(1).toLowerCase().replace(/_/g, " ");
}
