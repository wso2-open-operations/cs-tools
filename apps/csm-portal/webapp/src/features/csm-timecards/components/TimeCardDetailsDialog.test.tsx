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
import "@testing-library/jest-dom/vitest";
import TimeCardDetailsDialog from "@features/csm-timecards/components/TimeCardDetailsDialog";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

function card(overrides: Partial<CsmTimeCard> = {}): CsmTimeCard {
  return {
    id: "card-1",
    caseId: "case-1",
    caseNumber: "CS0000001",
    projectId: "proj-1",
    projectName: "Acme",
    workDate: "2026-07-13",
    userId: "user-1",
    userName: "Jane Doe",
    state: "submitted",
    billable: true,
    totalMinutes: 30,
    ...overrides,
  };
}

describe("TimeCardDetailsDialog", () => {
  it("shows the card's own fields", () => {
    render(<TimeCardDetailsDialog card={card()} onClose={vi.fn()} />);

    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByText("Acme")).toBeInTheDocument();
    expect(screen.getByText("Total time")).toBeInTheDocument();
    expect(screen.getAllByText("30 min").length).toBeGreaterThan(0);
  });

  it("shows the engineer's work-log comment, sanitized, when the card has one", () => {
    render(
      <TimeCardDetailsDialog
        card={card({ workLogComment: "<p>Investigated the reported latency issue.</p>" })}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("Engineer's comment")).toBeInTheDocument();
    expect(screen.getByText("Investigated the reported latency issue.")).toBeInTheDocument();
  });

  it("strips unsafe markup from the work-log comment before rendering", () => {
    render(
      <TimeCardDetailsDialog
        card={card({ workLogComment: '<p>hi<script>window.x=1</script></p><img src=x onerror="window.y=1">' })}
        onClose={vi.fn()}
      />,
    );

    expect(document.querySelector("script")).not.toBeInTheDocument();
    const img = document.querySelector("img");
    expect(img?.getAttribute("onerror")).toBeNull();
  });

  it("renders no comment section when the card has none", () => {
    render(<TimeCardDetailsDialog card={card()} onClose={vi.fn()} />);
    expect(screen.queryByText("Engineer's comment")).not.toBeInTheDocument();
  });

  it("shows the per-activity breakdown when present, in the fixed display order", () => {
    render(
      <TimeCardDetailsDialog
        card={card({
          breakdown: {
            analysisDebugging: 15,
            reproduce: 5,
            settingUp: 0,
            providingSolution: 10,
            answering: 0,
          },
        })}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("Breakdown")).toBeInTheDocument();
    expect(screen.getByText("Analysis and debugging")).toBeInTheDocument();
    expect(screen.getByText("Reproduce")).toBeInTheDocument();
    expect(screen.getByText("Setting up")).toBeInTheDocument();
    expect(screen.getByText("Providing solution")).toBeInTheDocument();
    expect(screen.getByText("Answering")).toBeInTheDocument();

    expect(screen.getByText("15 min")).toBeInTheDocument();
    expect(screen.getByText("5 min")).toBeInTheDocument();
    expect(screen.getByText("10 min")).toBeInTheDocument();
    expect(screen.getAllByText("0 min").length).toBe(2);
  });

  it("renders no breakdown section when the card has none", () => {
    render(<TimeCardDetailsDialog card={card()} onClose={vi.fn()} />);
    expect(screen.queryByText("Breakdown")).not.toBeInTheDocument();
  });

  it("shows issue complexity when present", () => {
    render(<TimeCardDetailsDialog card={card({ issueComplexity: "High" })} onClose={vi.fn()} />);
    expect(screen.getByText("Issue complexity")).toBeInTheDocument();
    expect(screen.getByText("High")).toBeInTheDocument();
  });

  it("lists the eligible approvers on a submitted card", () => {
    render(
      <TimeCardDetailsDialog
        card={card({
          approvers: [
            { id: "lead-1", name: "Lead One" },
            { id: "lead-2", name: "Lead Two" },
          ],
        })}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("Approvers")).toBeInTheDocument();
    expect(screen.getByText("Lead One, Lead Two")).toBeInTheDocument();
  });

  it("shows the decision summary on a decided card", () => {
    render(
      <TimeCardDetailsDialog
        card={card({ state: "approved", approvedByName: "Lead Person" })}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("Approved by: Lead Person")).toBeInTheDocument();
  });

  it("calls onClose when Close is clicked", () => {
    const onClose = vi.fn();
    render(<TimeCardDetailsDialog card={card()} onClose={onClose} />);

    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalled();
  });
});
