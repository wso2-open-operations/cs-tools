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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
// Mutable so a single test can simulate an unresolved user profile — every
// other test wants the default (an already-signed-in, resolved user).
let mockCurrentUserId: string | undefined = "u1";
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: mockCurrentUserId === undefined ? undefined : { id: mockCurrentUserId },
    isLoading: mockCurrentUserId === undefined,
    isError: false,
  }),
}));

import WallboardStatTile from "@features/csm-dashboard/components/WallboardStatTile";
import { CURRENT_USER_PLACEHOLDER } from "@features/csm-dashboard/utils/currentUserFilterPlaceholder";
import { __resetWidgetFetchConcurrencyForTests } from "@features/csm-dashboard/utils/widgetFetchConcurrency";

function renderWithClient(ui: ReactNode) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("WallboardStatTile", () => {
  beforeEach(() => {
    postMock.mockReset();
    mockCurrentUserId = "u1";
    __resetWidgetFetchConcurrencyForTests();
  });

  it("renders a skeleton while its count is in flight", () => {
    postMock.mockReturnValue(new Promise(() => {}));
    const { container } = renderWithClient(
      <WallboardStatTile widgetId="open" displayName="Open" resourceType="incident" filters={{}} section="cre" />,
    );
    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBe(1);
  });

  it("renders the resolved count and label once its search call succeeds", async () => {
    postMock.mockResolvedValue({ total: 5, incidents: [], limit: 1, offset: 0, hasMore: false });
    renderWithClient(
      <WallboardStatTile widgetId="open" displayName="Open" resourceType="incident" filters={{}} section="cre" />,
    );
    expect(await screen.findByText("5")).toBeInTheDocument();
    expect(screen.getByText("Open")).toBeInTheDocument();
  });

  it("marks the tile as alerted when the displayName has emphasis in this section and the count is > 0", async () => {
    postMock.mockResolvedValue({ total: 3, incidents: [], limit: 1, offset: 0, hasMore: false });
    const { container } = renderWithClient(
      <WallboardStatTile
        widgetId="sla"
        displayName="SLA Violations"
        resourceType="incident"
        filters={{}}
        section="cre"
      />,
    );
    await screen.findByText("3");
    expect(container.querySelector('[data-alert="true"]')).not.toBeNull();
  });

  it("does NOT mark the tile as alerted when the count is 0, even for an emphasized metric", async () => {
    postMock.mockResolvedValue({ total: 0, incidents: [], limit: 1, offset: 0, hasMore: false });
    const { container } = renderWithClient(
      <WallboardStatTile
        widgetId="sla"
        displayName="SLA Violations"
        resourceType="incident"
        filters={{}}
        section="cre"
      />,
    );
    await screen.findByText("0");
    expect(container.querySelector('[data-alert="true"]')).toBeNull();
  });

  it("does NOT mark the tile as alerted for a metric with no emphasis in this section, regardless of count", async () => {
    postMock.mockResolvedValue({ total: 42, incidents: [], limit: 1, offset: 0, hasMore: false });
    const { container } = renderWithClient(
      <WallboardStatTile
        widgetId="in-progress"
        displayName="In-Progress"
        resourceType="incident"
        filters={{}}
        section="cre"
      />,
    );
    await screen.findByText("42");
    expect(container.querySelector('[data-alert="true"]')).toBeNull();
  });

  // The exact regression this component exists to get right: the same
  // displayName resolves to emphasis (or not) purely based on `section`.
  it("resolves emphasis per (section, displayName), not displayName alone", async () => {
    postMock.mockResolvedValue({ total: 1, incidents: [], limit: 1, offset: 0, hasMore: false });
    // "New CR" is only emphasized under "sre" — under "cre" it must render plain.
    const { container } = renderWithClient(
      <WallboardStatTile widgetId="cr" displayName="New CR" resourceType="change_request" filters={{}} section="cre" />,
    );
    await screen.findByText("1");
    expect(container.querySelector('[data-alert="true"]')).toBeNull();
  });

  // Regression test (CodeRabbit): a disabled query (see `enabled:
  // !awaitingCurrentUser` on useWidgetData) reports `isLoading` false —
  // TanStack Query v5 reserves that for "actively fetching," not
  // "disabled, never fetched" — so the link-wrapping guard has to check
  // `awaitingCurrentUser` too, not just `isLoading`, or the still-loading
  // skeleton could get wrapped in a clickable link built from filters
  // that still carry the unresolved __current_user__ placeholder.
  it("does not wrap the tile in a link while awaiting the current user's own id", () => {
    mockCurrentUserId = undefined;
    postMock.mockReturnValue(new Promise(() => {}));
    const { container } = renderWithClient(
      <WallboardStatTile
        widgetId="mine"
        displayName="My Open"
        resourceType="incident"
        filters={{ assignedUserId: CURRENT_USER_PLACEHOLDER }}
        section="cre"
      />,
    );
    expect(container.querySelector("a")).toBeNull();
  });
});
