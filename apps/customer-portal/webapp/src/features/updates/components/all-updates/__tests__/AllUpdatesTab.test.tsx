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
import { describe, it, expect, vi, beforeEach } from "vitest";
import { MemoryRouter } from "react-router";
import AllUpdatesTab from "@features/updates/components/all-updates/AllUpdatesTab";

vi.mock("@features/updates/api/useGetProductUpdateLevels", () => ({
  useGetProductUpdateLevels: () => ({
    data: [
      {
        productName: "WSO2 API Manager",
        productUpdateLevels: [
          {
            productBaseVersion: "4.4.0",
            channel: "full",
            updateLevels: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19],
          },
        ],
      },
    ],
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@features/updates/api/usePostUpdateLevelsSearch", () => ({
  usePostUpdateLevelsSearch: () => ({
    data: null,
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("react-router", async (importOriginal) => {
  const orig = await importOriginal<typeof import("react-router")>();
  return {
    ...orig,
    useNavigate: () => vi.fn(),
    useParams: () => ({ projectId: "test-project-id" }),
  };
});

describe("AllUpdatesTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders filter section with dropdown labels", () => {
    render(
      <MemoryRouter>
        <AllUpdatesTab />
      </MemoryRouter>,
    );
    expect(screen.getByText("Search Update Levels")).toBeDefined();
    expect(screen.getByLabelText(/Product Name/)).toBeDefined();
    expect(screen.getByLabelText(/Product Version/)).toBeDefined();
    expect(screen.getByLabelText(/Starting Update Level/)).toBeDefined();
    expect(screen.getByLabelText(/Ending Update Level/)).toBeDefined();
  });

  it("renders Search, Clear Filters and Download Report buttons", () => {
    render(
      <MemoryRouter>
        <AllUpdatesTab />
      </MemoryRouter>,
    );
    expect(screen.getByRole("button", { name: "Search" })).toBeDefined();
    expect(screen.getByRole("button", { name: "Clear Filters" })).toBeDefined();
    // The report is downloaded as a PDF and is only available once a search has run.
    const downloadReport = screen.getByRole("button", { name: "Download Report" });
    expect(downloadReport).toBeDefined();
    expect((downloadReport as HTMLButtonElement).disabled).toBe(true);
  });

  it("shows placeholder text when no search has been run", () => {
    render(
      <MemoryRouter>
        <AllUpdatesTab />
      </MemoryRouter>,
    );
    expect(
      screen.getByText(/Select product, version, and update level range/),
    ).toBeDefined();
  });
});
