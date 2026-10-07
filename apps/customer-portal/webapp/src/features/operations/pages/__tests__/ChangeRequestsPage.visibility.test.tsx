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

// Which change requests a customer sees is decided by the server, per customer, so the
// page hides no state of its own: a change request waiting in Authorize (after the
// customer proposed a new time) is on the list, and the state filter offers it.

import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ChangeRequestsPage from "@features/operations/pages/ChangeRequestsPage";

const mocks = vi.hoisted(() => ({
  searchRequests: [] as unknown[],
  locationState: { value: null as Record<string, unknown> | null },
  // What GET /projects/{id}/filters sends: New and Assess are left out by the
  // server, Authorize is in.
  changeRequestStates: [
    { id: "-3", label: "Authorize" },
    { id: "5", label: "Customer Approval" },
    { id: "-2", label: "Scheduled" },
    { id: "-1", label: "Implement" },
    { id: "0", label: "Review" },
    { id: "1", label: "Customer Review" },
    { id: "2", label: "Rollback" },
    { id: "3", label: "Closed" },
    { id: "4", label: "Canceled" },
  ],
}));

vi.mock("react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router")>();
  return {
    ...actual,
    useParams: () => ({ projectId: "proj-1" }),
    useLocation: () => ({
      pathname: "/projects/proj-1/operations/change-requests",
      state: mocks.locationState.value,
    }),
  };
});

vi.mock("@hooks/useModifierAwareNavigate", () => ({
  useModifierAwareNavigate: () => vi.fn(),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => ({ data: { name: "Demo" }, isLoading: false }),
}));

vi.mock("@api/useGetProjectFilters", () => ({
  default: () => ({
    data: { changeRequestStates: mocks.changeRequestStates, changeRequestImpacts: [] },
    isLoading: false,
  }),
}));

vi.mock("@hooks/useSessionState", () => ({
  useSessionState: (key: string, initial: unknown) => {
    if (key.includes("search")) return ["", vi.fn()];
    if (key.includes("rowsPerPage")) return [10, vi.fn()];
    if (key.includes("page")) return [1, vi.fn()];
    if (key.includes("sortField")) return ["updatedOn", vi.fn()];
    if (key.includes("sortOrder")) return ["desc", vi.fn()];
    if (key.includes("filters")) return [{}, vi.fn()];
    return [initial, vi.fn()];
  },
}));

const authorizeItem = {
  id: "cr-authorize",
  number: "CHG0001234",
  title: "Rotate the gateway certificates",
  project: { id: "proj-1", label: "Demo" },
  case: null,
  deployment: null,
  deployedProduct: null,
  product: null,
  assignedEngineer: null,
  assignedTeam: null,
  startDate: "2031-03-01 09:00:00",
  endDate: "2031-03-01 11:00:00",
  duration: null,
  hasServiceOutage: false,
  impact: { id: "2", label: "2 - Medium" },
  state: { id: "-3", label: "Authorize" },
  type: { label: "Normal" },
  createdOn: "2031-01-01 00:00:00",
  updatedOn: "2031-01-02 00:00:00",
};

vi.mock("@features/operations/api/useGetChangeRequests", () => ({
  default: (_projectId: string, searchRequest: unknown) => {
    mocks.searchRequests.push(searchRequest);
    return {
      data: { changeRequests: [authorizeItem], totalRecords: 1 },
      isLoading: false,
      isError: false,
    };
  },
  useGetChangeRequestsInfinite: () => ({
    data: { pages: [{ changeRequests: [], totalRecords: 0 }] },
    isLoading: false,
    fetchNextPage: vi.fn(),
    hasNextPage: false,
  }),
}));

function renderPage() {
  return render(
    <MemoryRouter>
      <ChangeRequestsPage />
    </MemoryRouter>,
  );
}

const lastStateKeys = (): number[] => {
  const last = mocks.searchRequests[mocks.searchRequests.length - 1] as {
    filters: { stateKeys: number[] };
  };
  return last.filters.stateKeys;
};

describe("ChangeRequestsPage: no state is hidden by the page", () => {
  beforeEach(() => {
    mocks.searchRequests.length = 0;
    mocks.locationState.value = null;
  });

  it("lists a change request in Authorize, with its state", () => {
    renderPage();
    expect(screen.getByText("Rotate the gateway certificates")).toBeInTheDocument();
    expect(screen.getByText("CHG0001234")).toBeInTheDocument();
    expect(screen.getByText("Authorize")).toBeInTheDocument();
  });

  it("asks for every state the filters offer, Authorize among them (and never invents New or Assess)", () => {
    renderPage();
    expect(lastStateKeys()).toEqual([-3, 5, -2, -1, 0, 1, 2, 3, 4]);
    expect(lastStateKeys()).not.toContain(-5);
    expect(lastStateKeys()).not.toContain(-4);
  });

  it("counts Authorize as outstanding, not as an action for the customer", () => {
    mocks.locationState.value = { outstandingOnly: true };
    renderPage();
    expect(lastStateKeys()).toEqual([-3, 5, -2, -1, 0, 1]);

    mocks.searchRequests.length = 0;
    mocks.locationState.value = { actionRequired: true };
    renderPage();
    expect(lastStateKeys()).toEqual([5, 1]);
    expect(lastStateKeys()).not.toContain(-3);
  });
});
