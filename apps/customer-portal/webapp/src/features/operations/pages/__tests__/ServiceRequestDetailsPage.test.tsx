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
import ServiceRequestDetailsPage from "@features/operations/pages/ServiceRequestDetailsPage";

vi.mock("@features/support/api/useGetCaseDetails", () => ({
  default: () => ({
    data: {
      id: "case-1",
      number: "SR001",
      title: "Certificate renewal",
    },
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@case-details-details/CaseDetailsContent", () => ({
  default: ({ data }: { data?: { number: string } }) => (
    <div>case:{data?.number}</div>
  ),
}));

vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: () => ({ showLoader: vi.fn(), hideLoader: vi.fn() }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

describe("ServiceRequestDetailsPage", () => {
  it("renders service request details content", () => {
    render(
      <MemoryRouter initialEntries={["/projects/p1/operations/service-requests/case-1"]}>
        <Routes>
          <Route
            path="/projects/:projectId/operations/service-requests/:serviceRequestId"
            element={<ServiceRequestDetailsPage />}
          />
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByText("case:SR001")).toBeInTheDocument();
  });
});
