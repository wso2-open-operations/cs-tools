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
import PendingUpdatesPage from "@features/updates/pages/PendingUpdatesPage";

const mockNavigate = vi.fn();
const mockUseSearchParams = vi.fn();

vi.mock("react-router", () => ({
  useNavigate: () => mockNavigate,
  useParams: () => ({ projectId: "p1" }),
  useSearchParams: () => mockUseSearchParams(),
}));

vi.mock("@features/updates/api/usePostUpdateLevelsSearch", () => ({
  usePostUpdateLevelsSearch: vi.fn(),
}));

vi.mock("@features/updates/components/pending-updates/PendingUpdatesList", () => ({
  PendingUpdatesList: () => <div>pending updates list</div>,
}));

vi.mock(
  "@features/updates/components/pending-updates/PendingUpdatesListSkeleton",
  () => ({
    default: () => <div>pending updates skeleton</div>,
  }),
);

describe("PendingUpdatesPage", () => {
  it("renders missing parameter message when required params are absent", async () => {
    const { usePostUpdateLevelsSearch } = await import(
      "@features/updates/api/usePostUpdateLevelsSearch"
    );
    vi.mocked(usePostUpdateLevelsSearch).mockReturnValue({
      data: null,
      isLoading: false,
      isError: false,
    } as never);
    mockUseSearchParams.mockReturnValue([new URLSearchParams(), vi.fn()]);

    render(<PendingUpdatesPage />);
    expect(screen.getByText(/Missing product parameters/i)).toBeInTheDocument();
  });

  it("renders pending list when params are valid and loading is false", async () => {
    const { usePostUpdateLevelsSearch } = await import(
      "@features/updates/api/usePostUpdateLevelsSearch"
    );
    vi.mocked(usePostUpdateLevelsSearch).mockReturnValue({
      data: { 1: { updateType: "regular", updateDescriptionLevels: [] } },
      isLoading: false,
      isError: false,
    } as never);
    mockUseSearchParams.mockReturnValue([
      new URLSearchParams({
        productName: "wso2am",
        productBaseVersion: "4.4.0",
        startingUpdateLevel: "0",
        endingUpdateLevel: "1",
        mode: "pending",
      }),
      vi.fn(),
    ]);

    render(<PendingUpdatesPage />);
    expect(screen.getByText("pending updates list")).toBeInTheDocument();
    expect(screen.getByText("Pending Updates")).toBeInTheDocument();
  });
});

