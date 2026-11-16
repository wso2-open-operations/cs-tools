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

import { afterEach, describe, expect, it, vi } from "vitest";
import {
  buildCountFeedbackRequest,
  buildSearchFeedbackRequest,
  toCountFeedbackResult,
  toSearchFeedbackResult,
} from "@features/csm-custom-dashboards/api/codeDashboardFeedback";

const byRating = {
  buckets: [
    { bucketStart: "Very Dissatisfied", avgRating: 1, count: 3 },
    { bucketStart: "Dissatisfied", avgRating: 2, count: 4 },
    { bucketStart: "Neutral", avgRating: 3, count: 3 },
    { bucketStart: "Satisfied", avgRating: 4, count: 7 },
    { bucketStart: "Very Satisfied", avgRating: 5, count: 7 },
  ],
  totalRecords: 24,
};

describe("buildCountFeedbackRequest", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("asks for rating buckets when nothing is grouped", () => {
    expect(buildCountFeedbackRequest(undefined)).toEqual({ body: { filters: {}, bucket: "rating" }, grouped: false });
  });

  it("passes a grouping and the submission dates through", () => {
    expect(buildCountFeedbackRequest({ groupBy: "month", startDate: "2026-01-01", endDate: "2026-10-31" })).toEqual({
      body: { filters: { dateFrom: "2026-01-01", dateTo: "2026-10-31" }, bucket: "month" },
      grouped: true,
    });
  });

  it("resolves date tokens to the dates the endpoint takes", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 10, 12, 12));
    expect(buildCountFeedbackRequest({ startDate: "__startOfQuarter:-1__", endDate: "__endOfQuarter:-1__" }).body.filters).toEqual({
      dateFrom: "2026-07-01",
      dateTo: "2026-09-30",
    });
  });

  it.each([
    [{ groupBy: "account" }, /groupBy must be one of rating, month/],
    [{ rating: 1 }, /unknown argument "rating"/],
    [{ dateFrom: "2026-01-01" }, /unknown argument "dateFrom"/],
    [{ startDate: "last quarter" }, /startDate must be a date/],
    [{ endDate: 20261031 }, /endDate must be a date/],
  ])("refuses %j", (args, message) => {
    expect(() => buildCountFeedbackRequest(args)).toThrow(message);
  });
});

describe("toCountFeedbackResult", () => {
  it("gives the total and the average weighted by count", () => {
    expect(toCountFeedbackResult(byRating, false)).toEqual({ total: 24, average: 3.46 });
  });

  it("gives each group's count and average", () => {
    expect(
      toCountFeedbackResult(
        {
          buckets: [
            { bucketStart: "2026-09-01", avgRating: 3.5, count: 4 },
            { bucketStart: "2026-10-01", avgRating: 3.428571, count: 7 },
          ],
          totalRecords: 11,
        },
        true,
      ),
    ).toEqual({
      total: 11,
      average: 3.45,
      groups: [
        { label: "2026-09-01", count: 4, average: 3.5 },
        { label: "2026-10-01", count: 7, average: 3.43 },
      ],
    });
  });

  it("has no average when there are no responses", () => {
    expect(toCountFeedbackResult({ buckets: null, totalRecords: 0 }, true)).toEqual({ total: 0, average: null, groups: [] });
  });
});

describe("buildSearchFeedbackRequest", () => {
  it("reads the first 10 responses by default", () => {
    expect(buildSearchFeedbackRequest(undefined)).toEqual({ filters: {}, page: 1, pageSize: 10 });
  });

  it("passes a rating, the dates and a limit through", () => {
    expect(buildSearchFeedbackRequest({ rating: 1, startDate: "2026-10-01", limit: 5 })).toEqual({
      filters: { dateFrom: "2026-10-01", rating: 1 },
      page: 1,
      pageSize: 5,
    });
  });

  it.each([
    [{ rating: "1" }, /rating must be a whole number from 1 to 5/],
    [{ rating: 0 }, /rating must be a whole number from 1 to 5/],
    [{ rating: 4.5 }, /rating must be a whole number from 1 to 5/],
    [{ limit: 51 }, /limit must be an integer from 1 to 50/],
    [{ groupBy: "rating" }, /unknown argument "groupBy"/],
    [{ sortBy: { field: "submittedAt" } }, /unknown argument "sortBy"/],
  ])("refuses %j", (args, message) => {
    expect(() => buildSearchFeedbackRequest(args)).toThrow(message);
  });
});

describe("toSearchFeedbackResult", () => {
  it("keeps the case, rating, comment and date, never the submitter", () => {
    const response = {
      instanceId: "f1",
      caseNumber: "CASE-0001",
      rating: 5,
      ratingLabel: "Very Satisfied",
      comment: "Quick and clear answer.",
      submittedAt: "2026-10-25T04:43:53Z",
      submitterName: "A Person",
      submitterEmail: "a.person@example.com",
    };
    expect(toSearchFeedbackResult({ totalRecords: 24, results: [response] })).toEqual({
      total: 24,
      items: [
        {
          id: "f1",
          caseNumber: "CASE-0001",
          rating: 5,
          ratingLabel: "Very Satisfied",
          comment: "Quick and clear answer.",
          submittedAt: "2026-10-25T04:43:53Z",
        },
      ],
    });
  });
});
