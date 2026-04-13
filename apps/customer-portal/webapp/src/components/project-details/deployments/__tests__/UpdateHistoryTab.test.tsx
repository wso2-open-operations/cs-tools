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

import { render, screen, within } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import UpdateHistoryTab from "@components/project-details/deployments/UpdateHistoryTab";
import type { ProductUpdate } from "@/types/products";

vi.mock("@api/useGetRecommendedUpdateLevels", () => ({
  useGetRecommendedUpdateLevels: () => ({
    data: [],
    isLoading: false,
  }),
}));

vi.mock("@api/usePostUpdateLevelsSearch", () => ({
  usePostUpdateLevelsSearch: () => ({
    data: null,
    isLoading: false,
    isFetching: false,
  }),
}));

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

describe("UpdateHistoryTab", () => {
  const mockUpdates: ProductUpdate[] = [
    {
      updateLevel: 18,
      date: "2026-02-17",
      details: "Performance improvements and bug fixes",
    },
    {
      updateLevel: 15,
      date: "2026-02-03",
      details: "Security patch for CVE-2026-0001",
    },
  ];

  const defaultProps = {
    updates: mockUpdates,
    productName: "wso2am",
    productVersion: "2.1.0",
    isLoading: false,
    onSaveUpdates: vi.fn(),
  };

  it("renders update history timeline", () => {
    render(<UpdateHistoryTab {...defaultProps} />);

    const allU18 = screen.getAllByText(/U18/i);
    expect(allU18.length).toBeGreaterThanOrEqual(1);

    expect(screen.getByText(/U15/i)).toBeInTheDocument();
    expect(
      screen.getByText(/Performance improvements and bug fixes/i),
    ).toBeInTheDocument();
  });

  it("displays current update level", () => {
    render(<UpdateHistoryTab {...defaultProps} />);

    const banner = screen.getByText(/Current Update Level:/i).closest("div");
    expect(banner).toBeInTheDocument();
    if (banner) {
      expect(within(banner).getByText(/U18/i)).toBeInTheDocument();
    }
  });

  it("shows add update form", () => {
    render(<UpdateHistoryTab {...defaultProps} />);

    expect(screen.getByLabelText(/Update Level \*/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/Applied On \*/i)).toBeInTheDocument();
    expect(
      screen.getByLabelText(/Description \(Optional\)/i),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Add Update/i }),
    ).toBeInTheDocument();
  });

  it("shows loading skeleton when isLoading is true", () => {
    const { container } = render(
      <UpdateHistoryTab {...defaultProps} isLoading={true} />,
    );

    const skeletons = container.querySelectorAll(".MuiSkeleton-root");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("displays 'No update history available' when updates array is empty", () => {
    render(<UpdateHistoryTab {...defaultProps} updates={[]} />);

    expect(
      screen.getByText(/No update history available/i),
    ).toBeInTheDocument();
  });
});
