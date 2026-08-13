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
import "@testing-library/jest-dom/vitest";
import type { JSX } from "react";
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from "react-router";
import WorkItemsTab from "@features/csm-projects/components/WorkItemsTab";

const mockCsmIssuesView = vi.fn();

vi.mock("@features/csm-cases/components/CsmIssuesView", () => ({
  default: (props: Record<string, unknown>) => {
    mockCsmIssuesView(props);
    return <div>IssuesView: {props.entityNoun as string}</div>;
  },
}));

vi.mock("@features/csm-projects/components/ConversationsTab", () => ({
  default: ({ projectId }: { projectId: string }) => <div>Conversations for {projectId}</div>,
}));

// The sub-tab strip is now a real URL path segment (`usePathTabs`), which
// calls `useNavTransition`; bridge it to the real router's `useNavigate` so a
// simulated sub-tab click actually drives the URL — same pattern as
// CsmChangeRequestDetailPage.test.tsx.
const navigateMock = vi.fn();
vi.mock("@hooks/useNavTransition", () => ({
  useNavTransition: () => navigateMock,
}));

function NavigateBridge(): null {
  const navigate = useNavigate();
  navigateMock.mockImplementation(
    (...args: Parameters<typeof navigate>) => navigate(...args),
  );
  return null;
}

function LocationProbe(): JSX.Element {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

function renderWorkItemsTab(
  initialEntry = "/customers/projects/proj-1/work-items",
): ReturnType<typeof render> {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <NavigateBridge />
      <LocationProbe />
      <Routes>
        <Route
          path="/customers/projects/:id/work-items/:tab?"
          element={
            <WorkItemsTab
              projectId="proj-1"
              basePath="/customers/projects/proj-1/work-items"
            />
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe("WorkItemsTab", () => {
  it("defaults to the Cases sub-tab, locked to this project's case-type cases", () => {
    renderWorkItemsTab();

    expect(screen.getByText("IssuesView: cases")).toBeInTheDocument();
    expect(mockCsmIssuesView).toHaveBeenCalledWith(
      expect.objectContaining({
        entityNoun: "cases",
        lockedFilters: { projects: ["proj-1"], caseTypes: ["case"] },
        hideProjectFilter: true,
        hideTypeFilter: true,
      }),
    );
  });

  it("switches to the Service requests sub-tab, routed to the operations detail page", () => {
    renderWorkItemsTab();

    fireEvent.click(screen.getByText("Service requests"));

    expect(screen.getByText("IssuesView: service requests")).toBeInTheDocument();
    expect(mockCsmIssuesView).toHaveBeenCalledWith(
      expect.objectContaining({
        lockedFilters: { projects: ["proj-1"], caseTypes: ["service_request"] },
        detailBasePath: "/operations/service-requests",
      }),
    );
  });

  it("switches to the Security reports sub-tab with severity hidden", () => {
    renderWorkItemsTab();

    fireEvent.click(screen.getByText("Security reports"));

    expect(screen.getByText("IssuesView: security reports")).toBeInTheDocument();
    expect(mockCsmIssuesView).toHaveBeenCalledWith(
      expect.objectContaining({
        lockedFilters: { projects: ["proj-1"], caseTypes: ["security_report_analysis"] },
        hideSeverityColumn: true,
        detailBasePath: "/security-center/security-reports",
      }),
    );
  });

  it("switches to the Engagements sub-tab with the engagement-type filter shown", () => {
    renderWorkItemsTab();

    fireEvent.click(screen.getByText("Engagements"));

    expect(screen.getByText("IssuesView: engagements")).toBeInTheDocument();
    expect(mockCsmIssuesView).toHaveBeenCalledWith(
      expect.objectContaining({
        lockedFilters: { projects: ["proj-1"], caseTypes: ["engagement"] },
        showEngagementTypeFilter: true,
        hideSeverityColumn: true,
        detailBasePath: "/engagements",
      }),
    );
  });

  it("switches to the Conversations sub-tab", () => {
    renderWorkItemsTab();

    fireEvent.click(screen.getByText("Chats"));

    expect(screen.getByText("Conversations for proj-1")).toBeInTheDocument();
  });
});

describe("WorkItemsTab — sub-tab is a real URL path segment", () => {
  it("clicking a sub-tab navigates to that sub-tab's own URL", () => {
    renderWorkItemsTab();

    fireEvent.click(screen.getByText("Conversations"));

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/customers/projects/proj-1/work-items/conversations",
    );
    expect(screen.getByRole("tab", { name: "Conversations" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("loading the page directly at a sub-tab's URL opens on that sub-tab", () => {
    renderWorkItemsTab("/customers/projects/proj-1/work-items/engagements");

    expect(screen.getByRole("tab", { name: "Engagements" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByText("IssuesView: engagements")).toBeInTheDocument();
  });

  it("falls back to Cases for an unknown sub-tab segment, without redirecting the URL", () => {
    renderWorkItemsTab("/customers/projects/proj-1/work-items/bogus-tab");

    expect(screen.getByRole("tab", { name: "Cases" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/customers/projects/proj-1/work-items/bogus-tab",
    );
  });
});
