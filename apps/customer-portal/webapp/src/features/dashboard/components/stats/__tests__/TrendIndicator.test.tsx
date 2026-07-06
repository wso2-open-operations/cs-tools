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

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { TrendIndicator } from "@features/dashboard/components/stats/TrendIndicator";
import { TrendColor, TrendDirection } from "@features/dashboard/types/stats";

describe("TrendIndicator", () => {
  it("renders trend value when provided", () => {
    render(
      <TrendIndicator
        trend={{
          value: "-5%",
          direction: TrendDirection.DOWN,
          color: TrendColor.ERROR,
        }}
      />,
    );
    expect(screen.getByText("-5%")).toBeInTheDocument();
    expect(screen.getByText("vs Previous 30 Days")).toBeInTheDocument();
  });

  it("renders skeleton while loading", () => {
    const { container } = render(<TrendIndicator isLoading />);
    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBeGreaterThan(0);
  });

  it("returns null when no trend, loading, or error", () => {
    const { container } = render(<TrendIndicator />);
    expect(container).toBeEmptyDOMElement();
  });
});
