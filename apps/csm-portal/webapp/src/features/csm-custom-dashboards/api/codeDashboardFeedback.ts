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

import { resolveRelativeDatePlaceholderToDate } from "@utils/resolveRelativeDatePlaceholder";

/**
 * A code dashboard's customer satisfaction calls (`portal.countFeedback`,
 * `portal.searchFeedback`) on `POST /cases/feedback/aggregate` and
 * `/search`. Both take the submission date range as plain dates only and
 * answer a bad body with a 400 that gives no reason, so date tokens are
 * resolved and every arg checked here. The aggregate takes no rating
 * filter: a count of one rating is its group in a `rating` grouping.
 */

/** The groupings `portal.countFeedback` offers: the aggregate's buckets. */
const FEEDBACK_GROUP_BY = [
  "rating",
  "month",
  "week",
  "day",
  "reasons_very_dissatisfied",
  "reasons_dissatisfied",
  "reasons_neutral",
  "reasons_satisfied",
  "reasons_very_satisfied",
] as const;
type FeedbackGroupBy = (typeof FEEDBACK_GROUP_BY)[number];

/** `portal.searchFeedback`' default page and its cap. */
const SEARCH_FEEDBACK_DEFAULT_LIMIT = 10;
const SEARCH_FEEDBACK_MAX_LIMIT = 50;

const PLAIN_DATE = /^\d{4}-\d{2}-\d{2}$/;

/** The submission date filters both endpoints take. */
interface FeedbackFilters {
  rating?: number;
  dateFrom?: string;
  dateTo?: string;
}

/**
 * Checks the arg names and the `startDate` / `endDate` pair (the submission
 * date, from / to inclusive) and returns them as the endpoints' `dateFrom` /
 * `dateTo`, tokens resolved to the viewer's local date.
 */
function buildFeedbackDates(fn: string, args: Record<string, unknown>, extra: readonly string[]): FeedbackFilters {
  for (const key of Object.keys(args)) {
    if (!["startDate", "endDate", ...extra].includes(key)) throw new Error(`${fn}: unknown argument "${key}"`);
  }
  const filters: FeedbackFilters = {};
  for (const [arg, key] of [
    ["startDate", "dateFrom"],
    ["endDate", "dateTo"],
  ] as const) {
    const value = args[arg];
    if (value === undefined) continue;
    const resolved =
      typeof value !== "string" || PLAIN_DATE.test(value) ? value : resolveRelativeDatePlaceholderToDate(value, new Date());
    if (typeof resolved !== "string" || !PLAIN_DATE.test(resolved)) {
      throw new Error(`${fn}: ${arg} must be a date (YYYY-MM-DD) or a date token`);
    }
    filters[key] = resolved;
  }
  return filters;
}

/** Body of POST /cases/feedback/aggregate. */
export interface FeedbackAggregateRequest {
  filters: FeedbackFilters;
  bucket: FeedbackGroupBy;
}

/** Response of POST /cases/feedback/aggregate. */
export interface FeedbackAggregateResponse {
  buckets?: { bucketStart: string; avgRating?: number | null; count: number }[] | null;
  totalRecords?: number;
}

/** What `portal.countFeedback` returns: responses and the average rating, per group when asked. */
export interface CountFeedbackResult {
  total: number;
  average: number | null;
  groups?: { label: string; count: number; average: number | null }[];
}

/**
 * Turns `portal.countFeedback(args)` into a `POST /cases/feedback/aggregate`
 * body, or throws (the code sees the message). Args: `startDate`, `endDate`
 * and `groupBy` (a bucket). Without a `groupBy` the endpoint is asked for
 * `rating` buckets, which also give the overall average.
 */
export function buildCountFeedbackRequest(args: unknown): { body: FeedbackAggregateRequest; grouped: boolean } {
  const all = (args ?? {}) as Record<string, unknown>;
  const filters = buildFeedbackDates("countFeedback", all, ["groupBy"]);
  if (all.groupBy !== undefined && !FEEDBACK_GROUP_BY.includes(all.groupBy as FeedbackGroupBy)) {
    throw new Error(`countFeedback: groupBy must be one of ${FEEDBACK_GROUP_BY.join(", ")}`);
  }
  return {
    body: { filters, bucket: (all.groupBy as FeedbackGroupBy | undefined) ?? "rating" },
    grouped: all.groupBy !== undefined,
  };
}

/** A rating, rounded to two decimals; null when there is none. */
function rounded(rating: number | null | undefined): number | null {
  return typeof rating === "number" ? Math.round(rating * 100) / 100 : null;
}

/**
 * Reshapes a `POST /cases/feedback/aggregate` response into what
 * `portal.countFeedback` returns. The overall average is the buckets'
 * averages weighted by their counts: exact for `rating` and the date
 * buckets, which split the responses; for a `reasons_*` grouping a response
 * can tick several reasons, so there it is the reasons' own average.
 */
export function toCountFeedbackResult(res: FeedbackAggregateResponse, grouped: boolean): CountFeedbackResult {
  const buckets = res.buckets ?? [];
  const counted = buckets.filter((b) => typeof b.avgRating === "number" && b.count > 0);
  const weight = counted.reduce((n, b) => n + b.count, 0);
  const average = weight > 0 ? rounded(counted.reduce((sum, b) => sum + (b.avgRating as number) * b.count, 0) / weight) : null;
  const totals = { total: res.totalRecords ?? 0, average };
  if (!grouped) return totals;
  return { ...totals, groups: buckets.map((b) => ({ label: b.bucketStart, count: b.count, average: rounded(b.avgRating) })) };
}

/** Body of POST /cases/feedback/search, as `portal.searchFeedback` sends it: always the first page. */
export interface FeedbackSearchRequest {
  filters: FeedbackFilters;
  page: 1;
  pageSize: number;
}

/**
 * Turns `portal.searchFeedback(args)` into a `POST /cases/feedback/search`
 * body, or throws. Args: `rating` (a whole number 1 to 5), `startDate`,
 * `endDate` and `limit` (1 to 50, default 10). Newest first; no sort.
 */
export function buildSearchFeedbackRequest(args: unknown): FeedbackSearchRequest {
  const all = (args ?? {}) as Record<string, unknown>;
  const filters = buildFeedbackDates("searchFeedback", all, ["rating", "limit"]);
  if (all.rating !== undefined) {
    if (!(Number.isInteger(all.rating) && (all.rating as number) >= 1 && (all.rating as number) <= 5)) {
      throw new Error("searchFeedback: rating must be a whole number from 1 to 5");
    }
    filters.rating = all.rating as number;
  }
  const { limit } = all;
  if (limit !== undefined && !(Number.isInteger(limit) && (limit as number) >= 1 && (limit as number) <= SEARCH_FEEDBACK_MAX_LIMIT)) {
    throw new Error(`searchFeedback: limit must be an integer from 1 to ${SEARCH_FEEDBACK_MAX_LIMIT}`);
  }
  return { filters, page: 1, pageSize: (limit as number | undefined) ?? SEARCH_FEEDBACK_DEFAULT_LIMIT };
}

/** A feedback response as `POST /cases/feedback/search` returns it (the fields read here). */
interface FeedbackSearchView {
  instanceId: string;
  caseNumber?: string | null;
  rating?: number | null;
  ratingLabel?: string | null;
  comment?: string | null;
  submittedAt?: string | null;
}

/** One response as `portal.searchFeedback` returns it; the submitter is left out. */
export interface CodeDashboardFeedback {
  id: string;
  caseNumber: string | null;
  rating: number | null;
  ratingLabel: string | null;
  comment: string | null;
  submittedAt: string | null;
}

/** Reshapes a `POST /cases/feedback/search` response into what `portal.searchFeedback` returns. */
export function toSearchFeedbackResult(res: { results?: FeedbackSearchView[] | null; totalRecords?: number }): {
  total: number;
  items: CodeDashboardFeedback[];
} {
  const items = (res.results ?? []).map(
    (f): CodeDashboardFeedback => ({
      id: f.instanceId,
      caseNumber: f.caseNumber ?? null,
      rating: f.rating ?? null,
      ratingLabel: f.ratingLabel ?? null,
      comment: f.comment ?? null,
      submittedAt: f.submittedAt ?? null,
    }),
  );
  return { total: res.totalRecords ?? items.length, items };
}
