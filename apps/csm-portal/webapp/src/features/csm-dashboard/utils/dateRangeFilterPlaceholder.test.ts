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
import {
  DATE_RANGE_FROM_PLACEHOLDER,
  DATE_RANGE_TO_PLACEHOLDER,
  hasDateRangeFilterPlaceholder,
  resolveDateRangeFilterPlaceholder,
} from "@features/csm-dashboard/utils/dateRangeFilterPlaceholder";

describe("resolveDateRangeFilterPlaceholder", () => {
  it("substitutes both placeholders with the selected range", () => {
    const result = resolveDateRangeFilterPlaceholder(
      { dateFrom: DATE_RANGE_FROM_PLACEHOLDER, dateTo: DATE_RANGE_TO_PLACEHOLDER, accountIds: ["a"] },
      "2026-07-01",
      "2026-08-01",
    );
    expect(result).toEqual({ dateFrom: "2026-07-01", dateTo: "2026-08-01", accountIds: ["a"] });
  });

  it("drops dateFrom entirely when no range is selected, rather than sending the literal placeholder", () => {
    const result = resolveDateRangeFilterPlaceholder(
      { dateFrom: DATE_RANGE_FROM_PLACEHOLDER },
      undefined,
      undefined,
    );
    expect(result).toEqual({});
    expect("dateFrom" in result).toBe(false);
  });

  it("drops dateTo entirely when no range is selected", () => {
    const result = resolveDateRangeFilterPlaceholder(
      { dateTo: DATE_RANGE_TO_PLACEHOLDER },
      undefined,
      undefined,
    );
    expect(result).toEqual({});
  });

  it("resolves dateFrom and drops dateTo independently", () => {
    const result = resolveDateRangeFilterPlaceholder(
      { dateFrom: DATE_RANGE_FROM_PLACEHOLDER, dateTo: DATE_RANGE_TO_PLACEHOLDER },
      "2026-07-01",
      undefined,
    );
    expect(result).toEqual({ dateFrom: "2026-07-01" });
  });

  it("leaves a literal dateFrom/dateTo untouched (not the placeholder)", () => {
    const result = resolveDateRangeFilterPlaceholder(
      { dateFrom: "2025-01-01", dateTo: "2025-12-31" },
      "2026-07-01",
      "2026-08-01",
    );
    expect(result).toEqual({ dateFrom: "2025-01-01", dateTo: "2025-12-31" });
  });

  it("passes through filters with neither key unchanged", () => {
    const filters = { accountIds: ["a"] };
    expect(resolveDateRangeFilterPlaceholder(filters, "2026-07-01", "2026-08-01")).toEqual(filters);
  });
});

describe("hasDateRangeFilterPlaceholder", () => {
  it("detects the from placeholder", () => {
    expect(hasDateRangeFilterPlaceholder({ dateFrom: DATE_RANGE_FROM_PLACEHOLDER })).toBe(true);
  });

  it("detects the to placeholder", () => {
    expect(hasDateRangeFilterPlaceholder({ dateTo: DATE_RANGE_TO_PLACEHOLDER })).toBe(true);
  });

  it("returns false for filters carrying neither placeholder", () => {
    expect(hasDateRangeFilterPlaceholder({ dateFrom: "2025-01-01", accountIds: ["a"] })).toBe(false);
    expect(hasDateRangeFilterPlaceholder({})).toBe(false);
  });
});
