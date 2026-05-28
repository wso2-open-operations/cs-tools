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
import DescribeIssuePage from "@features/support/pages/DescribeIssuePage";

const mockNavigate = vi.fn();
const mockUseLocation = vi.fn();
const mockUseParams = vi.fn();

vi.mock("react-router", () => ({
  useNavigate: () => mockNavigate,
  useLocation: () => mockUseLocation(),
  useParams: () => mockUseParams(),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

vi.mock("@api/usePostProjectDeploymentsSearch", () => ({
  usePostProjectDeploymentsSearchAll: () => ({ data: [] }),
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => ({ data: { account: { id: "account-1" }, type: { label: "Enterprise" } } }),
}));

vi.mock("@features/support/hooks/useAllDeploymentProducts", () => ({
  useAllDeploymentProducts: () => ({ productsByDeploymentId: {}, isLoading: false }),
}));

describe("DescribeIssuePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseParams.mockReturnValue({ projectId: "project-1" });
    mockUseLocation.mockReturnValue({ key: "abc" });
  });

  it("should disable submit when issue message is empty", () => {
    render(<DescribeIssuePage />);
    expect(screen.getByRole("button", { name: "Submit & Get Help" })).toBeDisabled();
  });

  it("should navigate to chat with composed state", () => {
    render(<DescribeIssuePage />);

    fireEvent.change(screen.getByPlaceholderText(/Example:/), {
      target: { value: "API gateway returns 504 errors" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Submit & Get Help" }));

    expect(mockNavigate).toHaveBeenCalledWith("/projects/project-1/support/chat", {
      state: expect.objectContaining({
        initialUserMessage: "API gateway returns 504 errors",
        accountId: "account-1",
      }),
    });
  });
});

