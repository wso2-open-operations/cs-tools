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
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import CaseDetailsContent from "@case-details-details/CaseDetailsContent";
import { mockCaseDetails } from "@models/mockData";

vi.mock("@api/useGetCaseAttachments", () => ({
  default: vi.fn(() => ({
    data: { totalRecords: 3, attachments: [], limit: 50, offset: 0 },
    isLoading: false,
    isError: false,
  })),
}));

vi.mock("@case-details/CaseDetailsTabPanels", () => ({
  __esModule: true,
  default: () => <div data-testid="tab-panels">Tab panels</div>,
}));

function renderContent(props: {
  data?: typeof mockCaseDetails;
  isLoading?: boolean;
  isError?: boolean;
  caseId?: string;
  onBack?: () => void;
} = {}) {
  return render(
    <ThemeProvider theme={createTheme()}>
      <CaseDetailsContent
        data={props.data ?? mockCaseDetails}
        isLoading={props.isLoading ?? false}
        isError={props.isError ?? false}
        caseId={props.caseId ?? "case-001"}
        onBack={props.onBack ?? vi.fn()}
      />
    </ThemeProvider>,
  );
}

describe("CaseDetailsContent", () => {
  it("should show loading skeleton when isLoading is true", () => {
    renderContent({ data: undefined, isLoading: true });
    expect(screen.getByText("Back to Support Center")).toBeInTheDocument();
    const skeletons = document.querySelectorAll(".MuiSkeleton-root");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("should render header and action row when not in focus mode", () => {
    renderContent();
    expect(screen.getByText("CS0001001")).toBeInTheDocument();
    expect(screen.getByText("Support Engineer")).toBeInTheDocument();
  });

  it("should hide header and action row when focus mode is on", () => {
    renderContent();
    const focusButton = screen.getByRole("button", { name: /focus mode/i });
    fireEvent.click(focusButton);
    expect(screen.queryByText("Support Engineer")).not.toBeInTheDocument();
  });

  it("should show attachment count in Attachments tab label when available", () => {
    renderContent();
    expect(screen.getByText("Attachments (3)")).toBeInTheDocument();
  });

  it("should render tab panels", () => {
    renderContent();
    expect(screen.getByTestId("tab-panels")).toBeInTheDocument();
  });
});
