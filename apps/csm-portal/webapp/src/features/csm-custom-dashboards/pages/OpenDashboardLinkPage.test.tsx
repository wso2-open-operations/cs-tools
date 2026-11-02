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

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { type JSX, StrictMode } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const postMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

import { BackendApiError } from "@api/backend/client";
import { DASHBOARD_LINK_PATH } from "@features/csm-custom-dashboards/utils/dashboardLinkUrl";
import OpenDashboardLinkPage from "@features/csm-custom-dashboards/pages/OpenDashboardLinkPage";

const LINK_ID = "5e636a12-0000-4000-8000-0000000000cc";
const DASHBOARD = "5e636a12-0000-4000-8000-000000000001";

function Probe(): JSX.Element {
  const location = useLocation();
  return <div data-testid="landed">{location.pathname}</div>;
}

function renderAt(linkId: string, { strict = false } = {}) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  const tree = (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[`${DASHBOARD_LINK_PATH}/${linkId}`]}>
        <Routes>
          <Route path="/dashboard/shared/link/:linkId" element={<OpenDashboardLinkPage />} />
          <Route path="/dashboard/shared/:dashboardId" element={<Probe />} />
          <Route path="/dashboard/shared" element={<Probe />} />
          <Route path="/dashboard/my/:dashboardId" element={<Probe />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
  render(strict ? <StrictMode>{tree}</StrictMode> : tree);
}

describe("OpenDashboardLinkPage", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("opens the link with its id in the body and lands on the shared dashboard", async () => {
    postMock.mockResolvedValue({ dashboardId: DASHBOARD, own: false });
    renderAt(LINK_ID);
    expect(screen.getByText("Opening the dashboard…")).toBeInTheDocument();
    expect(await screen.findByTestId("landed")).toHaveTextContent(`/dashboard/shared/${DASHBOARD}`);
    expect(postMock).toHaveBeenCalledWith("/dashboards/shared/link", { linkId: LINK_ID });
  });

  it("lands on My Dashboards when the dashboard is the caller's own", async () => {
    postMock.mockResolvedValue({ dashboardId: DASHBOARD, own: true });
    renderAt(LINK_ID);
    expect(await screen.findByTestId("landed")).toHaveTextContent(`/dashboard/my/${DASHBOARD}`);
  });

  it("opens the link once, also under StrictMode", async () => {
    postMock.mockResolvedValue({ dashboardId: DASHBOARD, own: false });
    renderAt(LINK_ID, { strict: true });
    await screen.findByTestId("landed");
    expect(postMock).toHaveBeenCalledTimes(1);
  });

  it("says when the link is off or unknown, in the backend's words, without the id", async () => {
    postMock.mockRejectedValue(
      new BackendApiError(404, "This link doesn't open a dashboard. Its owner may have turned it off."),
    );
    renderAt(LINK_ID);
    expect(await screen.findByText("Couldn't open the link")).toBeInTheDocument();
    expect(screen.getByText("This link doesn't open a dashboard. Its owner may have turned it off.")).toBeInTheDocument();
    expect(document.body.textContent).not.toContain(LINK_ID);
  });

  it("is generic when the backend fails", async () => {
    postMock.mockRejectedValue(new BackendApiError(502, "upstream said something internal"));
    renderAt(LINK_ID);
    expect(await screen.findByText("The link couldn't be opened. Try again in a moment.")).toBeInTheDocument();
    expect(screen.queryByText(/upstream said/)).not.toBeInTheDocument();
  });

  it("offers a way to the Shared Dashboards list", async () => {
    postMock.mockRejectedValue(new BackendApiError(400, "This isn't a valid dashboard link. Check that it was copied in full."));
    renderAt("not-a-link");
    fireEvent.click(await screen.findByRole("button", { name: "Go to Shared Dashboards" }));
    expect(screen.getByTestId("landed")).toHaveTextContent(/^\/dashboard\/shared$/);
  });
});
