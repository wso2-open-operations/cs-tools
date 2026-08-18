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
import "@testing-library/jest-dom/vitest";
import CaseSlaStrip from "@features/csm-cases/components/CaseSlaStrip";
import type { CaseSla } from "@features/csm-cases/types/csmCases";

function sla(overrides: Partial<CaseSla>): CaseSla {
  return {
    id: "sla-1",
    definition: "S1 - Response",
    target: "1 Business Hour",
    stage: "in_progress",
    stageLabel: "In progress",
    hasBreached: false,
    businessTimeLeftLabel: "30 minutes",
    businessElapsedLabel: "30 minutes",
    businessElapsedPercent: 50,
    startTime: "2026-07-01T10:00:00Z",
    stopTime: null,
    ...overrides,
  };
}

describe("CaseSlaStrip", () => {
  it("renders nothing when there are no SLAs", () => {
    const { container } = render(<CaseSlaStrip slas={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when slas is undefined", () => {
    const { container } = render(<CaseSlaStrip slas={undefined} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when every SLA is completed or cancelled", () => {
    const { container } = render(
      <CaseSlaStrip
        slas={[
          sla({ id: "a", stage: "completed", stageLabel: "Completed" }),
          sla({ id: "b", stage: "cancelled", stageLabel: "Cancelled" }),
        ]}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("shows an in-progress SLA with its definition, stage, time left, and percent", () => {
    render(<CaseSlaStrip slas={[sla({})]} />);
    expect(screen.getByText("S1 - Response")).toBeInTheDocument();
    expect(screen.getByText("In progress")).toBeInTheDocument();
    expect(screen.getByText("30 minutes left")).toBeInTheDocument();
    expect(screen.getByText("50%")).toBeInTheDocument();
  });

  it("filters out completed/cancelled SLAs but keeps in-progress and breached ones", () => {
    render(
      <CaseSlaStrip
        slas={[
          sla({ id: "a", definition: "Query - Resolution", stage: "completed" }),
          sla({
            id: "b",
            definition: "S1 - Response",
            stage: "breached",
            stageLabel: "Breached",
            hasBreached: true,
            businessElapsedPercent: 109,
            businessTimeLeftLabel: "0 seconds",
          }),
        ]}
      />,
    );
    expect(screen.queryByText("Query - Resolution")).not.toBeInTheDocument();
    expect(screen.getByText("S1 - Response")).toBeInTheDocument();
    expect(screen.getByText("Breached")).toBeInTheDocument();
    expect(screen.getByText("109%")).toBeInTheDocument();
  });
});
