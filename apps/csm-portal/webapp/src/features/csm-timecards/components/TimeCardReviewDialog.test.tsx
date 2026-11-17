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
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import TimeCardReviewDialog from "@features/csm-timecards/components/TimeCardReviewDialog";
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

describe("TimeCardReviewDialog", () => {
  it("shows the engineer's work-log comment, sanitized, when the card has one", () => {
    render(
      <TimeCardReviewDialog
        card={card({ workLogComment: "<p>Investigated the reported latency issue.</p>" })}
        isDeciding={false}
        onClose={vi.fn()}
        onDecide={vi.fn()}
      />,
    );

    expect(screen.getByText("Engineer's comment")).toBeInTheDocument();
    expect(screen.getByText("Investigated the reported latency issue.")).toBeInTheDocument();
  });

  it("strips unsafe markup from the work-log comment before rendering", () => {
    render(
      <TimeCardReviewDialog
        card={card({ workLogComment: '<p>hi<script>window.x=1</script></p><img src=x onerror="window.y=1">' })}
        isDeciding={false}
        onClose={vi.fn()}
        onDecide={vi.fn()}
      />,
    );

    expect(document.querySelector("script")).not.toBeInTheDocument();
    const img = document.querySelector("img");
    expect(img?.getAttribute("onerror")).toBeNull();
  });

  it("renders no comment section when the card has none", () => {
    render(
      <TimeCardReviewDialog
        card={card()}
        isDeciding={false}
        onClose={vi.fn()}
        onDecide={vi.fn()}
      />,
    );

    expect(screen.queryByText("Engineer's comment")).not.toBeInTheDocument();
  });
});

describe("TimeCardReviewDialog — required lead comment", () => {
  const renderWith = (action?: "approve" | "reject") =>
    render(
      <TimeCardReviewDialog
        card={card()}
        action={action}
        isDeciding={false}
        onClose={vi.fn()}
        onDecide={vi.fn()}
      />,
    );

  it("is required when the dialog was opened from Reject", () => {
    renderWith("reject");
    expect(screen.getByRole("textbox", { name: /Lead's comment/ })).toBeRequired();
  });

  it("is marked in review mode too, since Reject is offered there", () => {
    renderWith();
    expect(screen.getByRole("textbox", { name: /Lead's comment/ })).toBeRequired();
  });

  it("is not required when the dialog was opened from Accept", () => {
    renderWith("approve");
    expect(screen.getByRole("textbox", { name: /Lead's comment/ })).not.toBeRequired();
  });
});
