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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import UsageMetricsTimeRangeSelector from "@features/usage-metrics/components/UsageMetricsTimeRangeSelector";
import { UsageTimeRange } from "@features/project-details/types/usage";
import { UsageMetricsInnerTabId } from "@features/usage-metrics/types/usageMetrics";

describe("UsageMetricsTimeRangeSelector", () => {
  it("calls range and clear handlers when preset is selected", () => {
    const onTimeRangeChange = vi.fn();
    const onClearCustomApplied = vi.fn();

    render(
      <UsageMetricsTimeRangeSelector
        innerTab={UsageMetricsInnerTabId.OVERVIEW}
        timeRange={UsageTimeRange.ONE_MONTH}
        onTimeRangeChange={onTimeRangeChange}
        onClearCustomApplied={onClearCustomApplied}
        customStart=""
        customEnd=""
        onCustomStartChange={vi.fn()}
        onCustomEndChange={vi.fn()}
        onApplyCustom={vi.fn()}
        onCancelCustom={vi.fn()}
        appliedCustomStart=""
        appliedCustomEnd=""
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "3M" }));

    expect(onTimeRangeChange).toHaveBeenCalledWith(UsageTimeRange.THREE_MONTHS);
    expect(onClearCustomApplied).toHaveBeenCalledTimes(1);
  });
});
