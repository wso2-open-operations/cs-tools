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
  const match = RELATIVE_DATE_PLACEHOLDER_PATTERN.exec(value);
  if (!match) return undefined;
  const [, name, argStr] = match;
  const refToday = today(now);

  switch (name) {
    case "today":
      // Takes no argument — a shape match with one (`__today:5__`) is not
      // ours to resolve; leave it for the backend's own rejection.
      return argStr === undefined ? dayToBoundInstant(refToday, op) : undefined;

    case "daysAgo": {
      if (argStr === undefined) return undefined;
      const n = Number(argStr);
      // Same non-negative constraint as the backend's own daysAgo.
      if (n < 0) return undefined;
      return dayToBoundInstant(addDays(refToday, -n), op);
    }

    case "startOfMonth": {
      if (argStr === undefined) return undefined;
      return dayToBoundInstant(startOfRelativeMonth(now, Number(argStr)), op);
    }

    case "endOfMonth": {
      if (argStr === undefined) return undefined;
      const lastDay = addDays(startOfRelativeMonth(now, Number(argStr) + 1), -1);
      return dayToBoundInstant(lastDay, op);
    }

    case "startOfQuarter": {
      if (argStr === undefined) return undefined;
      return dayToBoundInstant(startOfRelativeQuarter(now, Number(argStr)), op);
    }

    case "endOfQuarter": {
      if (argStr === undefined) return undefined;
      const lastDay = addDays(startOfRelativeQuarter(now, Number(argStr) + 1), -1);
      return dayToBoundInstant(lastDay, op);
    }

    default:
      // Shape matches (__name__ / __name:N__) but the name isn't one of the
      // relative-date placeholders — not ours to resolve.
      return undefined;
  }
}
