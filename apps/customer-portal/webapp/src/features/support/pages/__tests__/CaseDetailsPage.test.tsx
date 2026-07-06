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
import { beforeEach, describe, expect, it, vi } from "vitest";
import CaseDetailsPage from "@features/support/pages/CaseDetailsPage";

const mockNavigate = vi.fn();
const mockShowLoader = vi.fn();
const mockHideLoader = vi.fn();
const mockShowError = vi.fn();

const mockUseLocation = vi.fn();
const mockUseParams = vi.fn();
const mockUseGetCaseDetails = vi.fn();

vi.mock("react-router", () => ({
  useNavigate: () => mockNavigate,
  useLocation: () => mockUseLocation(),
  useParams: () => mockUseParams(),
}));

vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: () => ({ showLoader: mockShowLoader, hideLoader: mockHideLoader }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: mockShowError }),
}));

vi.mock("@features/support/api/useGetCaseDetails", () => ({
  default: () => mockUseGetCaseDetails(),
}));

vi.mock("@case-details-details/CaseDetailsContent", () => ({
  default: ({
    isLoading,
    onBack,
  }: {
    isLoading: boolean;
    onBack: () => void;
  }) => (
    <div>
      <span data-testid="loading">{String(isLoading)}</span>
      <button onClick={onBack}>Back</button>
    </div>
  ),
}));

describe("CaseDetailsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseParams.mockReturnValue({ projectId: "project-1", caseId: "case-1" });
    mockUseLocation.mockReturnValue({
      pathname: "/projects/project-1/support/cases/case-1",
      search: "",
      state: { returnTo: "/projects/project-1/support/cases" },
    });
    mockUseGetCaseDetails.mockReturnValue({
      data: { id: "case-1", title: "Case 1" },
      isLoading: false,
      isError: false,
      error: undefined,
    });
  });

  it("should surface loading orchestration to page content", () => {
    mockUseGetCaseDetails.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: undefined,
    });

    render(<CaseDetailsPage />);

    expect(screen.getByTestId("loading")).toHaveTextContent("true");
    expect(mockShowLoader).toHaveBeenCalled();
  });

  it("should navigate back to returnTo from location state", () => {
    render(<CaseDetailsPage />);

    fireEvent.click(screen.getByText("Back"));

    expect(mockNavigate).toHaveBeenCalledWith("/projects/project-1/support/cases", {
      state: { fromBack: true },
    });
  });
});

