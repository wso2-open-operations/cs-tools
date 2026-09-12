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

import { fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
// widgetListConfig.tsx pulls in useTimeSheets.ts (time_card's own mapper),
// which reads `window.config` at module load via `@config/apiConfig` --
// unavailable under vitest (see apps/csm-portal/webapp/CLAUDE.md).
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
// CaseWidgetList's own "Customise columns" wiring needs both of these --
// neither provider is set up by `renderRenderer` below.
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" }, isLoading: false, isError: false }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "agent@example.test" }),
}));

import { WIDGET_LIST_RENDERERS } from "@features/csm-dashboard/config/widgetListConfig";

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

/** Renders one renderer at `/dashboard`, with a matching destination route so
 * a row click's navigation is observable via `LocationProbe`, the same
 * pattern `DashboardWidgetTile.test.tsx`/`DashboardMiniTable.test.tsx` use. */
function renderRenderer(ui: React.ReactElement, destinationPath: string) {
  return render(
    <MemoryRouter initialEntries={["/dashboard"]}>
      <Routes>
        <Route path="/dashboard" element={ui} />
        <Route path={destinationPath} element={<LocationProbe />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("widgetListConfig — quick-preview icon per resourceType", () => {
  it("incident: renders the preview icon, opens the drawer without navigating, row click still navigates", () => {
    const Renderer = WIDGET_LIST_RENDERERS.incident;
    renderRenderer(
      <Renderer
        resourceType="incident"
        items={[
          {
            id: "inc-1",
            number: "INC0000001",
            subject: "Something broke",
            state: "in_progress",
            priority: "HIGH",
          },
        ]}
        isLoading={false}
      />,
      "/operations/incidents/:id",
    );

    fireEvent.click(screen.getByRole("button", { name: /quick preview inc0000001/i }));
    expect(screen.getAllByText("Something broke").length).toBeGreaterThan(0);
    expect(screen.queryByTestId("location-probe")).not.toBeInTheDocument();

    // Closing, then clicking the row itself navigates.
    fireEvent.click(screen.getByLabelText("Close preview"));
    fireEvent.click(screen.getByText("INC0000001"));
    expect(screen.getByTestId("location-probe")).toHaveTextContent("/operations/incidents/inc-1");
  });

  it("change_request: renders the preview icon and opens the drawer without navigating", () => {
    const Renderer = WIDGET_LIST_RENDERERS.change_request;
    renderRenderer(
      <Renderer
        resourceType="change_request"
        items={[
          {
            id: "cr-1",
            number: "CHG0000001",
            subject: "Upgrade the cluster",
            state: "scheduled",
            impact: "high",
          },
        ]}
        isLoading={false}
      />,
      "/operations/change-requests/:id",
    );

    fireEvent.click(screen.getByRole("button", { name: /quick preview chg0000001/i }));
    expect(screen.getAllByText("Upgrade the cluster").length).toBeGreaterThan(0);
    expect(screen.queryByTestId("location-probe")).not.toBeInTheDocument();
  });

  it("problem: renders the preview icon and opens the drawer without navigating", () => {
    const Renderer = WIDGET_LIST_RENDERERS.problem;
    renderRenderer(
      <Renderer
        resourceType="problem"
        items={[{ id: "prb-1", number: "PRB0000001", subject: "Recurring failure", state: "open" }]}
        isLoading={false}
      />,
      "/operations/problems/:id",
    );

    fireEvent.click(screen.getByRole("button", { name: /quick preview prb0000001/i }));
    expect(screen.getAllByText("Recurring failure").length).toBeGreaterThan(0);
    expect(screen.queryByTestId("location-probe")).not.toBeInTheDocument();
  });

  it("account: renders the preview icon, opens the drawer without navigating, row click still navigates", () => {
    const Renderer = WIDGET_LIST_RENDERERS.account;
    renderRenderer(
      <Renderer
        resourceType="account"
        items={[{ id: "acc-1", name: "Acme Corp", tier: "enterprise", region: "us-east" }]}
        isLoading={false}
      />,
      "/customers/accounts/:id",
    );

    fireEvent.click(screen.getByRole("button", { name: /quick preview acme corp/i }));
    expect(screen.getAllByText("Acme Corp").length).toBeGreaterThan(0);
    expect(screen.queryByTestId("location-probe")).not.toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("Close preview"));
    fireEvent.click(screen.getByText("us-east"));
    expect(screen.getByTestId("location-probe")).toHaveTextContent("/customers/accounts/acc-1");
  });

  it("project: renders the preview icon and opens the drawer without navigating", () => {
    const Renderer = WIDGET_LIST_RENDERERS.project;
    renderRenderer(
      <Renderer
        resourceType="project"
        items={[
          {
            id: "proj-1",
            name: "Support Platform",
            key: "SPT",
            subscriptionType: "subscription",
            closureState: "open",
          },
        ]}
        isLoading={false}
      />,
      "/customers/projects/:id",
    );

    fireEvent.click(screen.getByRole("button", { name: /quick preview support platform/i }));
    expect(screen.getAllByText("Support Platform").length).toBeGreaterThan(0);
    expect(screen.queryByTestId("location-probe")).not.toBeInTheDocument();
  });

  it("user: renders the preview icon and opens the drawer without navigating", () => {
    const Renderer = WIDGET_LIST_RENDERERS.user;
    renderRenderer(
      <Renderer
        resourceType="user"
        items={[
          {
            id: "user-1",
            userName: "jane.doe",
            name: "Jane Doe",
            email: "jane.doe@example.com",
            active: true,
          },
        ]}
        isLoading={false}
      />,
      "/people/:id",
    );

    fireEvent.click(screen.getByRole("button", { name: /quick preview jane doe/i }));
    expect(screen.getAllByText("Jane Doe").length).toBeGreaterThan(0);
    expect(screen.queryByTestId("location-probe")).not.toBeInTheDocument();
  });

  it("product_vulnerability: renders the preview icon and opens the drawer without navigating", () => {
    const Renderer = WIDGET_LIST_RENDERERS.product_vulnerability;
    renderRenderer(
      <Renderer
        resourceType="product_vulnerability"
        items={[
          {
            id: "vuln-1",
            cveId: "CVE-2026-0001",
            productName: "API Manager",
            priority: "Critical",
          },
        ]}
        isLoading={false}
      />,
      "/security-center/vulnerabilities/:id",
    );

    fireEvent.click(screen.getByRole("button", { name: /quick preview cve-2026-0001/i }));
    expect(screen.getAllByText("CVE-2026-0001").length).toBeGreaterThan(0);
    expect(screen.queryByTestId("location-probe")).not.toBeInTheDocument();
  });

  it("case: renders an Assignee column, falling back to Unassigned when no engineer is assigned", () => {
    const Renderer = WIDGET_LIST_RENDERERS.case;
    renderRenderer(
      <Renderer
        resourceType="case"
        items={[
          {
            id: "case-1",
            caseNumber: "CS0000001",
            subject: "Cluster degraded",
            assignedEngineer: { id: "u-1", email: "jane.doe@example.com", name: "Jane Doe" },
          },
          {
            id: "case-2",
            caseNumber: "CS0000002",
            subject: "Login failing",
            assignedEngineer: null,
          },
        ]}
        isLoading={false}
      />,
      "/cases/:id",
    );

    expect(screen.getByText("Assignee")).toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByText("Unassigned")).toBeInTheDocument();
  });

  it("case: shows a 'Customise columns' picker on the dashboard tile, offering Severity", () => {
    // Reported live: the "Customise columns" control was missing on every
    // dashboard tile showing a case-family list (e.g. the "My Work" widget),
    // even though the main Cases tab, and this widget's own "View more"
    // preview, both already had it.
    const Renderer = WIDGET_LIST_RENDERERS.case;
    renderRenderer(
      <Renderer
        resourceType="case"
        items={[{ id: "case-1", caseNumber: "CS0000001", subject: "Cluster degraded" }]}
        isLoading={false}
      />,
      "/cases/:id",
    );

    fireEvent.click(screen.getByRole("button", { name: "Customise columns" }));
    const picker = screen.getByRole("list", { name: "Customise columns" });
    expect(within(picker).getByText("Severity")).toBeInTheDocument();
  });

  it("service_request: does not offer a Severity column in the picker, unlike case", () => {
    // Severity is a support-case concept; a widget for any other case-family
    // resourceType sharing this same renderer must not offer a column that
    // would only ever render "—".
    const Renderer = WIDGET_LIST_RENDERERS.service_request;
    renderRenderer(
      <Renderer
        resourceType="service_request"
        items={[{ id: "sr-1", caseNumber: "CS0000002", subject: "Need a new environment" }]}
        isLoading={false}
      />,
      "/operations/service-requests/:id",
    );

    fireEvent.click(screen.getByRole("button", { name: "Customise columns" }));
    expect(screen.queryByText("Severity")).not.toBeInTheDocument();
  });

  it("case_feedback: renders both the internal case id and the case number, falling back to the raw caseId when neither resolved", () => {
    const Renderer = WIDGET_LIST_RENDERERS.case_feedback;
    renderRenderer(
      <Renderer
        resourceType="case_feedback"
        items={[
          {
            instanceId: "fb-1",
            caseId: "case-1",
            caseNumber: "CS0440709",
            caseInternalId: "SCTPSUB-88",
            rating: 4,
            ratingLabel: "Satisfied",
            comment: null,
            submittedAt: "2026-08-03T04:37:50Z",
            submitterName: "Jane Customer",
          },
          {
            instanceId: "fb-2",
            caseId: "case-2",
            caseNumber: null,
            caseInternalId: null,
            rating: 5,
            ratingLabel: "Very Satisfied",
            comment: null,
            submittedAt: "2026-08-17T06:17:56Z",
            submitterName: null,
          },
        ]}
        isLoading={false}
      />,
      "/cases/:id",
    );

    expect(screen.getByText("SCTPSUB-88")).toBeInTheDocument();
    expect(screen.getByText("CS0440709")).toBeInTheDocument();
    expect(screen.getByText("case-2")).toBeInTheDocument();
    expect(screen.getByText("Jane Customer")).toBeInTheDocument();
    expect(screen.getByText("Customer")).toBeInTheDocument();
  });

  it("call_request: renders the preview icon, opens the existing detail modal without navigating the owning case", () => {
    const Renderer = WIDGET_LIST_RENDERERS.call_request;
    renderRenderer(
      <Renderer
        resourceType="call_request"
        items={[
          {
            id: "call-1",
            number: "CALL0000001",
            reason: "Escalation review",
            case: { id: "case-1", number: "CS0001" },
          },
        ]}
        isLoading={false}
      />,
      "/cases/:id",
    );

    fireEvent.click(screen.getByRole("button", { name: /quick preview call0000001/i }));
    expect(screen.getAllByText("Escalation review").length).toBeGreaterThan(0);
    expect(screen.queryByTestId("location-probe")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /close/i }));
    fireEvent.click(screen.getByText("CALL0000001"));
    expect(screen.getByTestId("location-probe")).toHaveTextContent("/cases/case-1");
  });
});
