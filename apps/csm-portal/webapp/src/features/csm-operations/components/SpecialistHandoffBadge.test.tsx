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
import SpecialistHandoffBadge from "@features/csm-operations/components/SpecialistHandoffBadge";
import type { BeSpecialistHandoffSummary } from "@api/backend/types";

function summary(overrides: Partial<BeSpecialistHandoffSummary> = {}): BeSpecialistHandoffSummary {
  return {
    reasonCode: "no-runbook",
    reasonDescription: "Runbook is not available",
    escalationTeam: null,
    handedOffAt: "2026-10-19 06:00:00",
    handedOffBy: "Jane Doe",
    assignmentGroup: { id: "grp-1", name: "U2" },
    task: { number: "TASK1000010", subject: "[Runbook Task] No entry available for INC0100712" },
    githubIssueUrl: null,
    ...overrides,
  };
}

describe("SpecialistHandoffBadge", () => {
  it("flags a missing GitHub issue when the product files one", () => {
    render(<SpecialistHandoffBadge handoff={summary({ githubIssueExpected: true })} />);
    expect(screen.getByText("No GitHub issue")).toBeInTheDocument();
  });

  it("says nothing about GitHub for a product without a repository", () => {
    render(<SpecialistHandoffBadge handoff={summary({ githubIssueExpected: false })} />);
    expect(screen.queryByText("No GitHub issue")).not.toBeInTheDocument();
    expect(screen.getByText(/TASK1000010/)).toBeInTheDocument();
  });

  it("says nothing about GitHub when it is not known whether one is filed", () => {
    render(<SpecialistHandoffBadge handoff={summary()} />);
    expect(screen.queryByText("No GitHub issue")).not.toBeInTheDocument();
  });

  it("links the issue when there is one", () => {
    render(
      <SpecialistHandoffBadge
        handoff={summary({ githubIssueExpected: true, githubIssueUrl: "https://github.com/wso2-enterprise/choreo/issues/1" })}
      />,
    );
    expect(screen.getByRole("link", { name: /GitHub issue/ })).toHaveAttribute(
      "href",
      "https://github.com/wso2-enterprise/choreo/issues/1",
    );
    expect(screen.queryByText("No GitHub issue")).not.toBeInTheDocument();
  });
});
