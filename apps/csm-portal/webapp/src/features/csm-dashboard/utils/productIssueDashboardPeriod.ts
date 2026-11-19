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

import { formatDateOnlyForDisplay } from "@utils/dateTime";
import type {
  ProductIssueDashboardBucket,
  ProductIssueTrendBucket,
} from "@features/csm-dashboard/types/productIssueDashboard";
import type { CaseVolumeTrendChartPoint } from "@features/csm-dashboard/api/useCaseVolumeTrendData";

/** The dashboard's calendar (entity-service computes every figure in it). */
const DASHBOARD_TIME_ZONE = "Asia/Kolkata";

export type ProductIssuePeriodId = "4w" | "12w" | "26w" | "6m" | "12m";

export interface ProductIssuePeriodOption {
  id: ProductIssuePeriodId;
  label: string;
  bucket: ProductIssueDashboardBucket;
  /** How many buckets (the current, still running one included). */
  count: number;
}

export const PRODUCT_ISSUE_PERIODS: readonly ProductIssuePeriodOption[] = [
  { id: "4w", label: "Last 4 weeks", bucket: "week", count: 4 },
  { id: "12w", label: "Last 12 weeks", bucket: "week", count: 12 },
  { id: "26w", label: "Last 26 weeks", bucket: "week", count: 26 },
  { id: "6m", label: "Last 6 months", bucket: "month", count: 6 },
  { id: "12m", label: "Last 12 months", bucket: "month", count: 12 },
];

export const DEFAULT_PRODUCT_ISSUE_PERIOD: ProductIssuePeriodId = "12w";

const DAY_MS = 24 * 60 * 60 * 1000;

function pad(n: number): string {
  return String(n).padStart(2, "0");
}

function formatUtcDate(d: Date): string {
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}`;
}

/** Today's calendar date in Asia/Kolkata, as a UTC-midnight Date. */
function todayInDashboardZone(now: Date): Date {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: DASHBOARD_TIME_ZONE,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(now);
  const [y, m, d] = parts.split("-").map(Number);
  return new Date(Date.UTC(y, m - 1, d));
}

/**
 * The request window of a period option: the last `count` buckets, the
 * running one included, ending tomorrow (`to` is exclusive, so today is in).
 * Weeks start on Monday and months on the 1st, in Asia/Kolkata.
 */
export function productIssuePeriodRange(
  id: ProductIssuePeriodId,
  now: Date = new Date(),
): { from: string; to: string; bucket: ProductIssueDashboardBucket } {
  const option = PRODUCT_ISSUE_PERIODS.find((p) => p.id === id) ?? PRODUCT_ISSUE_PERIODS[1];
  const today = todayInDashboardZone(now);
  const to = new Date(today.getTime() + DAY_MS);
  if (option.bucket === "week") {
    const sinceMonday = (today.getUTCDay() + 6) % 7;
    const monday = new Date(today.getTime() - sinceMonday * DAY_MS);
    const from = new Date(monday.getTime() - (option.count - 1) * 7 * DAY_MS);
    return { from: formatUtcDate(from), to: formatUtcDate(to), bucket: "week" };
  }
  const from = new Date(Date.UTC(today.getUTCFullYear(), today.getUTCMonth() - (option.count - 1), 1));
  return { from: formatUtcDate(from), to: formatUtcDate(to), bucket: "month" };
}

/** The date part of an ISO timestamp with an offset ("2026-11-09T00:00:00+05:30"):
 * the calendar day in the dashboard's own zone. */
export function productIssueDatePart(iso: string): string {
  return iso.slice(0, 10);
}

/**
 * The trend buckets as the points `DashboardLineChart` draws (as bars):
 * `date` is the bucket's first day and `endDate` its last (`end` is exclusive),
 * both in Asia/Kolkata.
 */
export function productIssueTrendPoints(trend: ProductIssueTrendBucket[]): CaseVolumeTrendChartPoint[] {
  return trend.map((b) => {
    const start = productIssueDatePart(b.start);
    const endExclusive = productIssueDatePart(b.end);
    const parsed = new Date(`${endExclusive}T00:00:00Z`);
    const last = Number.isNaN(parsed.getTime())
      ? endExclusive
      : formatUtcDate(new Date(parsed.getTime() - DAY_MS));
    return {
      date: start,
      ...(last > start && { endDate: last }),
      opened: b.opened,
      closed: b.resolved,
    };
  });
}

/** "Nov 9, 2026 to Nov 18, 2026": the period's calendar days in Asia/Kolkata
 * (the end is exclusive), not shifted into the viewer's own zone. */
export function productIssuePeriodText(periodStart: string, periodEnd: string): string {
  const [last] = productIssueTrendPoints([{ start: periodStart, end: periodEnd, opened: 0, resolved: 0 }]).map(
    (p) => p.endDate ?? p.date,
  );
  const first = formatDateOnlyForDisplay(productIssueDatePart(periodStart)) ?? periodStart;
  return `${first} to ${formatDateOnlyForDisplay(last) ?? periodEnd}`;
}
