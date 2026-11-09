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

/**
 * Matches the shape of every relative-date placeholder the entity-service's
 * `resolveRelativeDate` (see `case_filters.go`) recognizes: `__<name>__` (no
 * argument, e.g. `__today__`) or `__<name>:<integer>__` (e.g. `__daysAgo:30__`,
 * `__startOfMonth:-1__`). Deliberately stricter than the backend's own
 * pattern (which accepts any non-underscore argument and defers the
 * integer check to a second step, so it can raise a targeted "non-integer
 * offset" error): here, anything that doesn't cleanly match is simply left
 * unresolved and forwarded to the backend as-is, which still rejects a
 * malformed placeholder like `__daysAgo:abc__` with that same validation
 * error — the fallback behavior is unchanged, only well-formed placeholders
 * are handled client-side.
 */
const RELATIVE_DATE_PLACEHOLDER_PATTERN = /^__([a-zA-Z]+)(?::(-?\d+))?__$/;

/** Local (browser) midnight for the given y/m/d. */
function localMidnight(year: number, month: number, date: number): Date {
  return new Date(year, month, date, 0, 0, 0, 0);
}

/** `now`'s own local midnight — the reference "today" every placeholder
 * below resolves relative to. */
function today(now: Date): Date {
  return localMidnight(now.getFullYear(), now.getMonth(), now.getDate());
}

function addDays(day: Date, n: number): Date {
  const copy = new Date(day);
  copy.setDate(copy.getDate() + n);
  return copy;
}

/** 1st of the month `n` months from `now`'s own month (local), e.g. n=0 this
 * month, n=-1 last month — mirrors `case_filters.go`'s
 * `startOfRelativeMonth`, computed in local time instead of UTC. */
function startOfRelativeMonth(now: Date, n: number): Date {
  const base = localMidnight(now.getFullYear(), now.getMonth(), 1);
  base.setMonth(base.getMonth() + n);
  return base;
}

/** 1st of the quarter `n` quarters from `now`'s own quarter (local) —
 * mirrors `case_filters.go`'s `startOfRelativeQuarter`. */
function startOfRelativeQuarter(now: Date, n: number): Date {
  const quarterStartMonth = Math.floor(now.getMonth() / 3) * 3;
  const base = localMidnight(now.getFullYear(), quarterStartMonth, 1);
  base.setMonth(base.getMonth() + n * 3);
  return base;
}

/** The Monday of the week `n` weeks from `now`'s own week (local), e.g. n=0
 * this week, n=-1 last week. Weeks run Monday to Sunday (ISO) — mirrors
 * `case_filters.go`'s `startOfRelativeWeek`. */
function startOfRelativeWeek(now: Date, n: number): Date {
  const refToday = today(now);
  // getDay(): 0 = Sunday ... 6 = Saturday, so Sunday is 6 days after Monday.
  const sinceMonday = (refToday.getDay() + 6) % 7;
  return addDays(refToday, -sinceMonday + n * 7);
}

/** 1 January of the year `n` years from `now`'s own year (local) — mirrors
 * `case_filters.go`'s `startOfRelativeYear`. */
function startOfRelativeYear(now: Date, n: number): Date {
  return localMidnight(now.getFullYear() + n, 0, 1);
}

/**
 * Converts a resolved calendar day (local midnight) into the concrete RFC3339
 * instant this filter entry's own `op` means: for `lte`, the day is inclusive
 * of its own whole 24h span (matching `parseCaseFilterDate`'s
 * date-only-plus-`lte` bump), so the bound is one millisecond before the next
 * day's local midnight; every other op (`gte`, chiefly) means the bound
 * starts at the day's own local midnight.
 */
function dayToBoundInstant(day: Date, op: string): string | undefined {
  if (op === "lte") {
    const nextDayMidnight = addDays(day, 1);
    const bound = new Date(nextDayMidnight.getTime() - 1);
    // A well-formed placeholder with an extreme offset (e.g.
    // `__daysAgo:99999999999__`) computes a Date outside JS's representable
    // range; `toISOString()` on it throws `RangeError: Invalid time value`
    // instead of returning a string. Fall through to `undefined` -- same as
    // any other unresolvable value -- rather than letting that throw
    // propagate out of `resolveRelativeDateFilters`/`useGetCsmCases` and
    // crash the caller.
    return Number.isFinite(bound.getTime()) ? bound.toISOString() : undefined;
  }
  return Number.isFinite(day.getTime()) ? day.toISOString() : undefined;
}

/**
 * Resolves a single filter value against `now` if — and only if — it matches
 * one of the relative-date placeholders `case_filters.go`'s
 * `resolveRelativeDate` documents. Returns `undefined` for anything else (a
 * literal date, an unrecognized/malformed placeholder, or a value for an
 * unrelated field like a UUID or enum) so the caller can leave it untouched.
 *
 * `op` decides whether the resolved calendar day is read as its own start
 * (any op other than `lte`) or end (`lte`) — see {@link dayToBoundInstant}.
 *
 * Shared between the dashboard widgets' own filter resolution
 * (`resolveRelativeDateFilters`, `csm-dashboard`) and the Cases list's
 * `createdFrom`/`createdTo` filters (`useGetCsmCases`, `csm-cases`) — both
 * features resolve the same placeholder grammar client-side, so the resolver
 * itself lives outside either feature's own `utils/`.
 */
export function resolveRelativeDatePlaceholder(
  value: string,
  op: string,
  now: Date,
): string | undefined {
  const day = relativeDay(value, now);
  return day === undefined ? undefined : dayToBoundInstant(day, op);
}

/**
 * Resolves a relative-date placeholder to a plain local calendar date,
 * `YYYY-MM-DD`, for a flat date filter whose bounds are whole days (a time
 * card's `startDate`/`endDate`, both inclusive on the work date). Same
 * grammar and the same "today" as {@link resolveRelativeDatePlaceholder};
 * `undefined` for anything that is not a placeholder it resolves.
 */
export function resolveRelativeDatePlaceholderToDate(value: string, now: Date): string | undefined {
  const day = relativeDay(value, now);
  if (day === undefined || !Number.isFinite(day.getTime())) return undefined;
  const pad = (n: number): string => String(n).padStart(2, "0");
  return `${day.getFullYear()}-${pad(day.getMonth() + 1)}-${pad(day.getDate())}`;
}

/**
 * Resolves a relative-date placeholder to a plain UTC calendar date,
 * `YYYY-MM-DD`: the same as {@link resolveRelativeDatePlaceholderToDate}, but
 * "today" is `now`'s UTC date, not the viewer's local one. For a value read
 * as a UTC day: a line widget's window, which `POST /cases/volume-trend`
 * counts by UTC day, so the window is the same for every viewer and "today"
 * is never a day the service has not reached yet.
 */
export function resolveRelativeDatePlaceholderToUtcDate(value: string, now: Date): string | undefined {
  // The UTC date's own numbers as a local midnight: every step after this is
  // whole-day calendar arithmetic on those numbers.
  return resolveRelativeDatePlaceholderToDate(
    value,
    localMidnight(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()),
  );
}

/** "this month", "last month", "next month", "the month 3 months ago". */
function periodPhrase(unit: "week" | "month" | "quarter" | "year", n: number): string {
  if (n === 0) return `this ${unit}`;
  if (n === -1) return `last ${unit}`;
  if (n === 1) return `next ${unit}`;
  return n < 0 ? `the ${unit} ${-n} ${unit}s ago` : `the ${unit} ${n} ${unit}s ahead`;
}

/**
 * A relative-date placeholder in words, for showing a filter to a person:
 * "Today", "30 days ago", "Start of this week", "Start of last month", "End of
 * this quarter", "End of last year". The
 * same wording as the Cases list's `RELATIVE_DATE_PRESETS`, so a widget's
 * placeholder reads the same wherever it is shown. `undefined` for anything
 * {@link resolveRelativeDatePlaceholder} would not resolve.
 */
export function describeRelativeDatePlaceholder(value: string): string | undefined {
  // Validity is the resolver's: anything it rejects is not described either.
  if (relativeDay(value, new Date()) === undefined) return undefined;
  const [, name, argStr] = RELATIVE_DATE_PLACEHOLDER_PATTERN.exec(value) ?? [];
  const n = Number(argStr);
  switch (name) {
    case "today":
      return "Today";
    case "daysAgo":
      return n === 0 ? "Today" : `${n} ${n === 1 ? "day" : "days"} ago`;
    case "startOfMonth":
      return `Start of ${periodPhrase("month", n)}`;
    case "endOfMonth":
      return `End of ${periodPhrase("month", n)}`;
    case "startOfQuarter":
      return `Start of ${periodPhrase("quarter", n)}`;
    case "endOfQuarter":
      return `End of ${periodPhrase("quarter", n)}`;
    case "startOfWeek":
      return `Start of ${periodPhrase("week", n)}`;
    case "endOfWeek":
      return `End of ${periodPhrase("week", n)}`;
    case "startOfYear":
      return `Start of ${periodPhrase("year", n)}`;
    case "endOfYear":
      return `End of ${periodPhrase("year", n)}`;
    default:
      return undefined;
  }
}

/** The largest offset (`:N`, either sign) a placeholder may carry, as in
 * entity-service's `resolveRelativeDate` and the CSM backend's
 * `relativeDate`. Past it the backends' date arithmetic would overflow;
 * every offset beyond it lands outside the years 0000-9999, which
 * entity-service rejects, so the bound leaves nothing unresolved that used to
 * give a usable date. */
const MAX_RELATIVE_DATE_OFFSET = 4_000_000;

/** The local calendar day a placeholder names, or `undefined` when `value` is not one. */
function relativeDay(value: string, now: Date): Date | undefined {
  const match = RELATIVE_DATE_PLACEHOLDER_PATTERN.exec(value);
  if (!match) return undefined;
  const [, name, argStr] = match;
  if (argStr !== undefined && Math.abs(Number(argStr)) > MAX_RELATIVE_DATE_OFFSET) return undefined;
  const refToday = today(now);

  switch (name) {
    case "today":
      // Takes no argument — a shape match with one (`__today:5__`) is not
      // ours to resolve; leave it for the backend's own rejection.
      return argStr === undefined ? refToday : undefined;

    case "daysAgo": {
      if (argStr === undefined) return undefined;
      const n = Number(argStr);
      // Same non-negative constraint as the backend's own daysAgo.
      if (n < 0) return undefined;
      return addDays(refToday, -n);
    }

    case "startOfMonth": {
      if (argStr === undefined) return undefined;
      return startOfRelativeMonth(now, Number(argStr));
    }

    case "endOfMonth": {
      if (argStr === undefined) return undefined;
      return addDays(startOfRelativeMonth(now, Number(argStr) + 1), -1);
    }

    case "startOfQuarter": {
      if (argStr === undefined) return undefined;
      return startOfRelativeQuarter(now, Number(argStr));
    }

    case "endOfQuarter": {
      if (argStr === undefined) return undefined;
      return addDays(startOfRelativeQuarter(now, Number(argStr) + 1), -1);
    }

    case "startOfWeek": {
      if (argStr === undefined) return undefined;
      return startOfRelativeWeek(now, Number(argStr));
    }

    case "endOfWeek": {
      if (argStr === undefined) return undefined;
      // The Sunday: six days after that week's Monday.
      return addDays(startOfRelativeWeek(now, Number(argStr)), 6);
    }

    case "startOfYear": {
      if (argStr === undefined) return undefined;
      return startOfRelativeYear(now, Number(argStr));
    }

    case "endOfYear": {
      if (argStr === undefined) return undefined;
      return addDays(startOfRelativeYear(now, Number(argStr) + 1), -1);
    }

    default:
      // Shape matches (__name__ / __name:N__) but the name isn't one of the
      // relative-date placeholders — not ours to resolve.
      return undefined;
  }
}
