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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactElement } from "react";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
// GenericColumnList's `case` branch renders CasePreviewDrawer -> CasePreviewContent,
// which fetches comments via useGetCsmCaseComments -- needs both the backend
// client and apiConfig mocked (see apps/csm-portal/webapp/CLAUDE.md).
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

import GenericColumnList from "@features/csm-dashboard/components/GenericColumnList";

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

function renderWithProviders(ui: ReactElement, destinationPath: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/dashboard"]}>
        <Routes>
          <Route path="/dashboard" element={ui} />
          <Route path={destinationPath} element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("GenericColumnList — quick-preview icon", () => {
  it("case: renders the preview icon per row, opens CasePreviewDrawer without navigating, row click still navigates", async () => {
    postMock.mockResolvedValue({ comments: [] });

    renderWithProviders(
      <GenericColumnList
        items={[
          {
            id: "case-1",
            number: "CS0001",
            internalId: "WSO2-1",
            subject: "Patch needed",
            state: "new",
          },
        ]}
        isLoading={false}
        resourceType="case"
        columns={[{ path: "number", label: "Number" }]}
      />,
      "/cases/:id",
    );

    fireEvent.click(screen.getByRole("button", { name: /quick preview/i }));
    expect(screen.getByText("Patch needed")).toBeInTheDocument();
    expect(screen.queryByTestId("location-probe")).not.toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("Close preview"));
    fireEvent.click(screen.getByText("CS0001"));
    await waitFor(() =>
      expect(screen.getByTestId("location-probe")).toHaveTextContent("/cases/case-1"),
    );
  });

  it("incident: dispatches to IncidentPreviewDrawer for a columns-configured incident widget", () => {
    renderWithProviders(
      <GenericColumnList
        items={[{ id: "inc-1", number: "INC0000001", subject: "Down", state: "new" }]}
        isLoading={false}
        resourceType="incident"
        columns={[{ path: "number", label: "Number" }]}
      />,
      "/operations/incidents/:id",
    );

    fireEvent.click(screen.getByRole("button", { name: /quick preview/i }));
    expect(screen.getByText("Down")).toBeInTheDocument();
    expect(screen.queryByTestId("location-probe")).not.toBeInTheDocument();
  });
});
