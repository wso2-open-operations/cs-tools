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
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import AnnouncementDetailsPage from "@features/announcements/pages/AnnouncementDetailsPage";

vi.mock("react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router")>();
  return {
    ...actual,
    useParams: () => ({ projectId: "proj-1", caseId: "case-1" }),
    useNavigate: () => vi.fn(),
  };
});

vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: () => ({ showLoader: vi.fn(), hideLoader: vi.fn() }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

vi.mock("@features/support/api/useGetCaseDetails", () => ({
  default: () => ({
    data: {
      title: "Release notes",
      description: "<p>Details</p>",
      status: { id: "1", label: "Open" },
    },
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@features/announcements/components/AnnouncementDetailsPanel", () => ({
  default: ({ data }: { data?: { title: string } }) => (
    <div>announcement:{data?.title}</div>
  ),
}));

describe("AnnouncementDetailsPage", () => {
  it("renders announcement details panel with case data", () => {
    render(
      <MemoryRouter>
        <AnnouncementDetailsPage />
      </MemoryRouter>,
    );
    expect(screen.getByText("announcement:Release notes")).toBeInTheDocument();
  });
});
