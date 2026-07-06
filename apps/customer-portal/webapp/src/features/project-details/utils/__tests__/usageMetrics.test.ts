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
import type { InstanceUsageEntry } from "@features/project-details/types/usage";
import {
  buildUsageTrendFromUsages,
  computeUsageHeadlineDelta,
  formatIsoDateForUsageChart,
  formatUsageMetricCount,
  sumUsageEntryTransactions,
} from "@features/project-details/utils/usageMetrics";

describe("formatIsoDateForUsageChart", () => {
  it("formats date with year on large screens", () => {
    expect(formatIsoDateForUsageChart("2024-06-15", false)).toContain("Jun");
    expect(formatIsoDateForUsageChart("2024-06-15", false)).toContain("2024");
  });

  it("omits year on small screens", () => {
    expect(formatIsoDateForUsageChart("2024-06-15", true)).toBe("Jun 15");
  });
});

describe("formatUsageMetricCount", () => {
  it("formats thousands and millions", () => {
    expect(formatUsageMetricCount(1500)).toBe("1.5K");
    expect(formatUsageMetricCount(2_500_000)).toBe("2.5M");
    expect(formatUsageMetricCount(42)).toBe("42");
  });
});

describe("sumUsageEntryTransactions", () => {
  it("sums TRANSACTION_COUNT across period summaries", () => {
    const entry: InstanceUsageEntry = {
      periodSummaries: [
        { period: "2024-01", counts: { TRANSACTION_COUNT: 10 } },
        { period: "2024-02", counts: { TRANSACTION_COUNT: 5 } },
      ],
    } as unknown as InstanceUsageEntry;
    expect(sumUsageEntryTransactions(entry)).toBe(15);
  });
});

describe("buildUsageTrendFromUsages", () => {
  it("aggregates counts by period", () => {
    const usages: InstanceUsageEntry[] = [
      {
        periodSummaries: [
          { period: "2024-01", counts: { API_CALLS: 3 } },
          { period: "2024-02", counts: { API_CALLS: 2 } },
        ],
      },
      {
        periodSummaries: [{ period: "2024-01", counts: { API_CALLS: 1 } }],
      },
    ] as unknown as InstanceUsageEntry[];

    const trend = buildUsageTrendFromUsages(usages, "API_CALLS", (p) => p);
    expect(trend).toEqual([
      { name: "2024-01", value: 4 },
      { name: "2024-02", value: 2 },
    ]);
  });
});

describe("computeUsageHeadlineDelta", () => {
  it("returns em dash when trend is empty", () => {
    expect(computeUsageHeadlineDelta([])).toEqual({ headline: "—", delta: "—" });
  });

  it("computes delta between last two points", () => {
    const result = computeUsageHeadlineDelta([
      { name: "a", value: 100 },
      { name: "b", value: 150 },
    ]);
    expect(result.headline).toBe("150");
    expect(result.delta).toBe("+50.0%");
  });
});
