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
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import CaseDetailsActivityPanel from "@case-details-activity/CaseDetailsActivityPanel";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
import LoggerProvider from "@context/logger/LoggerProvider";

const mockCaseComments = [
  { id: "c1", content: "Thanks for the detailed recommendations. I'll review with our team.", type: "comments", createdOn: "2026-02-12T11:15:42", createdBy: "user@test.com", isEscalated: false },
  { id: "c2", content: "Show more content here.", type: "comments", createdOn: "2026-02-12T10:30:15", createdBy: "support@wso2.com", isEscalated: false },
];

const mockUserDetails = { id: "u1", email: "user@test.com", lastName: "User", firstName: "Test", timeZone: "UTC" };

vi.mock("@features/support/api/useGetCaseComments", () => ({
  __esModule: true,
  default: vi.fn(() => ({
    data: {
      comments: mockCaseComments,
      totalRecords: mockCaseComments.length,
      offset: 0,
      limit: 50,
    },
    isLoading: false,
    isError: false,
  })),
}));

vi.mock("@features/settings/api/useGetUserDetails", () => ({
  __esModule: true,
  default: vi.fn(() => ({
    data: mockUserDetails,
    isLoading: false,
    isError: false,
  })),
}));

vi.mock("@features/support/api/usePostComment", () => ({
  usePostComment: vi.fn(() => ({
    mutate: vi.fn(),
    isPending: false,
  })),
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: vi.fn(() => ({
    isSignedIn: true,
    isLoading: false,
  })),
}));

function renderPanel(props: {
  projectId?: string;
  caseId?: string;
  caseCreatedOn?: string | null;
} = {}) {
  return render(
    <ThemeProvider theme={createTheme()}>
      <LoggerProvider>
        <ErrorBannerProvider>
          <CaseDetailsActivityPanel
            projectId={props.projectId ?? "project-001"}
            caseId={props.caseId ?? "case-001"}
            caseCreatedOn={props.caseCreatedOn}
          />
        </ErrorBannerProvider>
      </LoggerProvider>
    </ThemeProvider>,
  );
}

describe("CaseDetailsActivityPanel", () => {
  it("should render comments from API", () => {
    renderPanel();
    expect(
      screen.getByText(/Thanks for the detailed recommendations/),
    ).toBeInTheDocument();
    expect(screen.getByText(/Show more/)).toBeInTheDocument();
  });

  it("should show Support Engineer chip for non-current-user comments", () => {
    renderPanel();
    expect(screen.getAllByText(/Support Engineer/).length).toBeGreaterThan(0);
  });

  it("should show case created date when caseCreatedOn is provided", () => {
    renderPanel({ caseCreatedOn: "2026-01-31 10:45:12" });
    expect(screen.getByText(/Case created on/)).toBeInTheDocument();
  });

  it("should not show case created section when caseCreatedOn is null", () => {
    renderPanel({ caseCreatedOn: null });
    expect(screen.queryByText(/Case created on/)).not.toBeInTheDocument();
  });

  it("should not show You label for current user comments", () => {
    renderPanel();
    expect(screen.queryByText("You")).not.toBeInTheDocument();
  });

  it("should render input field and send button", () => {
    renderPanel();
    expect(screen.getByRole("button", { name: /send comment/i })).toBeInTheDocument();
    expect(screen.getByRole("textbox")).toBeInTheDocument();
  });
});
