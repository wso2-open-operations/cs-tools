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
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { useState, type ReactElement, type ReactNode } from "react";

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
// The column-customizer wiring reads both of these -- see
// WidgetInlineDrilldownPanel.test.tsx for the same pattern.
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "11111111-aaaa-bbbb-cccc-000000000001" } }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "agent@example.test" }),
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

describe("GenericColumnList — column customizer wiring", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  const TWO_COLUMN_PROPS = {
    items: [{ id: "inc-1", number: "INC0000001", subject: "Down", state: "new" }],
    isLoading: false,
    resourceType: "incident" as const,
    columns: [
      { path: "number", label: "Number" },
      { path: "subject", label: "Subject" },
    ],
    widgetId: "widget-abc",
  };

  it("renders both configured columns by default, with no customizer button when onColumnCustomizerChange is omitted", () => {
    renderWithProviders(<GenericColumnList {...TWO_COLUMN_PROPS} />, "/operations/incidents/:id");

    expect(screen.getByText("Number")).toBeInTheDocument();
    expect(screen.getByText("Subject")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /customise columns/i })).not.toBeInTheDocument();
  });

  it("hands the customizer button up via onColumnCustomizerChange, and hiding a column removes it from the table", () => {
    function Harness() {
      const [customizer, setCustomizer] = useState<ReactNode>(null);
      return (
        <>
          {customizer}
          <GenericColumnList {...TWO_COLUMN_PROPS} onColumnCustomizerChange={setCustomizer} />
        </>
      );
    }
    renderWithProviders(<Harness />, "/operations/incidents/:id");

    const customizerButton = screen.getByRole("button", { name: /customise columns/i });
    expect(customizerButton).toBeInTheDocument();
    expect(screen.getAllByText("Subject")).toHaveLength(1); // the table header only, popover not open yet

    fireEvent.click(customizerButton);
    // Two "Subject"s now: the table header + the popover's own row label.
    expect(screen.getAllByText("Subject")).toHaveLength(2);
    fireEvent.click(screen.getAllByText("Subject")[1]);

    // Only the popover's own (still-open) row label remains -- the table's
    // own header cell for "Subject" is gone.
    expect(screen.getAllByText("Subject")).toHaveLength(1);
    // "Number" still appears twice: the table header (unaffected) + the
    // popover's own row label.
    expect(screen.getAllByText("Number")).toHaveLength(2);
  });

  it("persists a hidden column across remount, keyed per widgetId", () => {
    function Harness() {
      const [customizer, setCustomizer] = useState<ReactNode>(null);
      return (
        <>
          {customizer}
          <GenericColumnList {...TWO_COLUMN_PROPS} onColumnCustomizerChange={setCustomizer} />
        </>
      );
    }
    const { unmount } = renderWithProviders(<Harness />, "/operations/incidents/:id");
    fireEvent.click(screen.getByRole("button", { name: /customise columns/i }));
    fireEvent.click(screen.getAllByText("Subject")[1]);
    unmount();

    renderWithProviders(<Harness />, "/operations/incidents/:id");
    expect(screen.getByText("Number")).toBeInTheDocument();
    expect(screen.queryByText("Subject")).not.toBeInTheDocument();

    // A different widgetId (same resourceType) never shares this saved layout.
    const key = Object.keys(window.localStorage).find((k) => k.includes("dashboard-generic-list:widget-abc"));
    expect(key).toBeDefined();
    expect(key).not.toContain("dashboard-generic-list:incident:");
  });
});
