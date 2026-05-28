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
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it, vi } from "vitest";
import ChangeRequestDetailsPage from "@features/operations/pages/ChangeRequestDetailsPage";

vi.mock("@features/operations/api/useGetChangeRequestDetails", () => ({
  default: () => ({
    data: {
      id: "cr-1",
      number: "CHG001",
      title: "Deploy patch",
      state: { id: "-2", label: "Scheduled" },
      type: { id: "normal", label: "Normal" },
      project: { id: "p1", label: "Proj", number: null },
      case: null,
      deployment: null,
      deployedProduct: null,
      product: null,
      assignedEngineer: null,
      assignedTeam: null,
      startDate: null,
      endDate: null,
      createdOn: "2026-01-01",
      updatedOn: "2026-01-02",
    },
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));

vi.mock("@features/operations/api/usePatchChangeRequest", () => ({
  usePatchChangeRequest: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

describe("ChangeRequestDetailsPage", () => {
  it("renders change request number when loaded", () => {
    render(
      <MemoryRouter initialEntries={["/projects/p1/operations/change-requests/cr-1"]}>
        <Routes>
          <Route
            path="/projects/:projectId/operations/change-requests/:changeRequestId"
            element={<ChangeRequestDetailsPage />}
          />
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByText("CHG001")).toBeInTheDocument();
  });
});
