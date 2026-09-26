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

const MINUTE_MS = 60_000;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;

/**
 * Renders iso relative to now (both read as epoch ms via Date.parse), floored
 * to the coarsest unit under a day: "just now" (<1m), "Xm ago"/"Xm from now"
 * (<1h), "Xh ago"/"Xh from now" (<24h), or "Xd ago"/"Xd from now" beyond
 * that. A future iso (clock skew) renders with "from now" instead of "ago".
 * Returns "—" for null, empty, or unparseable input.
 */
export function formatRelativeTime(iso: string | null | undefined, now: number): string {
  if (!iso) return "—";
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return "—";

  const diffMs = now - t;
  const absMs = Math.abs(diffMs);
  const minutes = Math.floor(absMs / MINUTE_MS);
  if (minutes < 1) return "just now";

  const suffix = diffMs >= 0 ? "ago" : "from now";
  const hours = Math.floor(absMs / HOUR_MS);
  if (hours < 1) return `${minutes}m ${suffix}`;
  const days = Math.floor(absMs / DAY_MS);
  if (days < 1) return `${hours}h ${suffix}`;
  return `${days}d ${suffix}`;
}

/** Formats iso as an exact timestamp in the browser's own time zone, e.g. "Sep 23, 2026, 3:04:05 PM GMT+5:30". Returns null for null, empty, or unparseable input. */
export function formatAbsoluteTimestamp(iso: string | null | undefined): string | null {
  if (!iso) return null;
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return null;
  return new Intl.DateTimeFormat("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    second: "2-digit",
    timeZoneName: "shortOffset",
  }).format(new Date(t));
}
