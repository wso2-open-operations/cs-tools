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

import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import CaseDetailsActionRow from "@case-details/CaseDetailsActionRow";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

vi.mock("@api/useGetProjectFilters", () => ({
  default: () => ({
    data: {
      caseStates: [
        { id: "1", label: "Open" },
        { id: "3", label: "Closed" },
        { id: "1003", label: "Waiting On WSO2" },
      ],
    },
  }),
}));

vi.mock("@features/support/api/usePatchCase", () => ({
  usePatchCase: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

function renderActionRow(
  props: Partial<Parameters<typeof CaseDetailsActionRow>[0]> = {},
) {
  return render(
    <ThemeProvider theme={createTheme()}>
      <CaseDetailsActionRow
        assignedEngineer="Jane Doe"
        engineerInitials="JD"
        statusLabel="Open"
        projectId="proj-1"
        caseId="case-1"
        {...props}
      />
    </ThemeProvider>,
  );
}

describe("CaseDetailsActionRow", () => {
  it("should render Close action for open status when case can be patched", () => {
    renderActionRow({ statusLabel: "Open" });
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
  });

  it("should render solution actions for solution proposed status", () => {
    renderActionRow({ statusLabel: "Solution Proposed" });
    expect(
      screen.getByRole("button", { name: "Accept Solution" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Reject Solution" }),
    ).toBeInTheDocument();
  });

  it("should render Open Related Case for recently closed cases", () => {
    renderActionRow({
      statusLabel: "Closed",
      closedOn: "2026-05-15 10:00:00",
      onOpenRelatedCase: vi.fn(),
    });
    expect(
      screen.getByRole("button", { name: "Open Related Case" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Close" })).not.toBeInTheDocument();
  });

  it("should hide Open Related Case when closed more than 2 months ago", () => {
    renderActionRow({
      statusLabel: "Closed",
      closedOn: "2020-01-01 10:00:00",
      onOpenRelatedCase: vi.fn(),
    });
    expect(
      screen.queryByRole("button", { name: "Open Related Case" }),
    ).not.toBeInTheDocument();
  });

  it("should call onOpenRelatedCase when Open Related Case is clicked", () => {
    const onOpenRelatedCase = vi.fn();
    renderActionRow({
      statusLabel: "Closed",
      closedOn: "2026-05-15 10:00:00",
      onOpenRelatedCase,
    });
    fireEvent.click(screen.getByRole("button", { name: "Open Related Case" }));
    expect(onOpenRelatedCase).toHaveBeenCalledTimes(1);
  });
});
