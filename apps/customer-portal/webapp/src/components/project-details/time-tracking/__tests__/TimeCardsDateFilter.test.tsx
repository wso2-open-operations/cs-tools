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
import { describe, it, expect } from "vitest";
import TimeCardsDateFilter from "@time-tracking/TimeCardsDateFilter";

describe("TimeCardsDateFilter", () => {
  it("should render filter by date range label and date inputs", () => {
    render(
      <TimeCardsDateFilter
        startDate="2025-01-01"
        endDate="2025-12-31"
        onStartDateChange={() => {}}
        onEndDateChange={() => {}}
      />,
    );

    expect(screen.getByText("Filter by Date Range:")).toBeInTheDocument();
    expect(screen.getByLabelText("From:")).toBeInTheDocument();
    expect(screen.getByLabelText("To:")).toBeInTheDocument();
  });
});
