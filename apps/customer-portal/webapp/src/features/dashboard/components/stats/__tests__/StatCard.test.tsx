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
import { Activity } from "@wso2/oxygen-ui-icons-react";
import { describe, expect, it } from "vitest";
import { StatCard } from "@features/dashboard/components/stats/StatCard";
import { TrendColor, TrendDirection } from "@features/dashboard/types/stats";

describe("StatCard", () => {
  it("renders label and value", () => {
    render(
      <StatCard
        label="Open Cases"
        value="12"
        icon={<Activity size={20} />}
        iconColor="primary"
        tooltipText="Open cases tooltip"
        isLoading={false}
        isError={false}
      />,
    );
    expect(screen.getByText("Open Cases")).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
  });

  it("renders trend indicator when trend is provided", () => {
    render(
      <StatCard
        label="Resolved"
        value="4"
        icon={<Activity size={20} />}
        iconColor="success"
        trend={{
          value: "+10%",
          direction: TrendDirection.UP,
          color: TrendColor.SUCCESS,
        }}
        tooltipText="Resolved cases tooltip"
        isLoading={false}
        isError={false}
      />,
    );
    expect(screen.getByText("+10%")).toBeInTheDocument();
  });
});
