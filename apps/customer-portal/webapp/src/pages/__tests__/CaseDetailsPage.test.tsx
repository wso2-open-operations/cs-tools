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
import CaseDetailsPage from "@pages/CaseDetailsPage";
import useGetCaseDetails from "@api/useGetCaseDetails";

vi.mock("react-router", () => ({
  useNavigate: () => vi.fn(),
  useParams: () => ({ projectId: "project-1", caseId: "case-001" }),
}));

vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: () => ({
    showLoader: vi.fn(),
    hideLoader: vi.fn(),
  }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({
    showError: vi.fn(),
  }),
}));

vi.mock("@api/useGetCaseDetails", () => ({
  default: vi.fn(() => ({
    data: undefined,
    isLoading: true,
    isFetching: false,
    isError: false,
  })),
}));

describe("CaseDetailsPage", () => {
  it("should render Back to Support Center when loading", () => {
    render(<CaseDetailsPage />);
    expect(screen.getByText("Back to Support Center")).toBeInTheDocument();
  });

  it("should show skeletons when isFetching (refresh) so loader and content stay in sync", () => {
    vi.mocked(useGetCaseDetails).mockReturnValue({
      data: undefined,
      isLoading: false,
      isFetching: true,
      isError: false,
    } as unknown as ReturnType<typeof useGetCaseDetails>);
    const { container } = render(<CaseDetailsPage />);
    const skeletons = container.querySelectorAll(".MuiSkeleton-root");
    expect(skeletons.length).toBeGreaterThan(0);
  });
});
