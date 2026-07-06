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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import UpdateLevelDetailsPage from "@features/updates/pages/UpdateLevelDetailsPage";

const mockNavigate = vi.fn();

vi.mock("react-router", () => ({
  useNavigate: () => mockNavigate,
  useParams: () => ({ projectId: "p1", levelKey: "7" }),
  useSearchParams: () => [
    new URLSearchParams({
      productName: "wso2am",
      productBaseVersion: "4.4.0",
      startingUpdateLevel: "0",
      endingUpdateLevel: "10",
    }),
    vi.fn(),
  ],
}));

vi.mock("@utils/useDarkMode", () => ({
  useDarkMode: () => false,
}));

vi.mock("@features/updates/api/usePostUpdateLevelsSearch", () => ({
  usePostUpdateLevelsSearch: () => ({
    isLoading: false,
    isError: false,
    data: {
      7: {
        updateType: "regular",
        updateDescriptionLevels: [
          {
            updateNumber: 701,
            updateType: "regular",
            description: "Fix issue",
            instructions: "N/A",
            bugFixes: "[]",
            filesModified: "[]",
            filesAdded: "[]",
            filesRemoved: "[]",
            securityAdvisories: [],
            timestamp: 1710000000000,
          },
        ],
      },
    },
  }),
}));

describe("UpdateLevelDetailsPage", () => {
  it("renders level details and filter actions", () => {
    render(<UpdateLevelDetailsPage />);
    expect(screen.getByText(/Update Level 7 Details/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "All" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Security" })).toBeInTheDocument();
  });

  it("navigates back on back button click", () => {
    render(<UpdateLevelDetailsPage />);
    fireEvent.click(screen.getByRole("button", { name: /Back/i }));
    expect(mockNavigate).toHaveBeenCalled();
  });
});

