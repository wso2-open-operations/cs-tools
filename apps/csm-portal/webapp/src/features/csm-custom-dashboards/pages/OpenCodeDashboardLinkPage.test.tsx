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
import type { JSX } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

let canView = true;
const openCode = vi.fn();
const openCustom = vi.fn();

vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));
vi.mock("@features/csm-custom-dashboards/utils/advancedDashboardAccess", () => ({
  useCanViewAdvancedDashboards: () => canView,
}));
vi.mock("@features/csm-custom-dashboards/api/useCodeDashboardShares", () => ({
  sharedCodeDashboardPath: (id: string) => `/dashboard/shared/code/${encodeURIComponent(id)}`,
  useOpenCodeDashboardLink: () => ({ mutateAsync: openCode }),
}));
vi.mock("@features/csm-custom-dashboards/api/useCodeDashboards", () => ({
  savedCodeDashboardPath: (id: string) => `/dashboard/my/code/${encodeURIComponent(id)}`,
}));
// The custom dashboards' open call: an advanced link must never use it.
vi.mock("@features/csm-custom-dashboards/api/useDashboardLink", () => ({
  useOpenDashboardLink: () => ({ mutateAsync: openCustom }),
}));

import { BackendApiError } from "@api/backend/client";
import OpenCodeDashboardLinkPage from "@features/csm-custom-dashboards/pages/OpenCodeDashboardLinkPage";

const LINK = "5e636a12-0000-4000-8000-0000000000cc";
const ID = "5e636a12-0000-4000-8000-0000000000c1";

function Where(): JSX.Element {
  return <div data-testid="where">{useLocation().pathname}</div>;
}

function renderAt(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/dashboard/shared/code-link/:linkId" element={<OpenCodeDashboardLinkPage />} />
        <Route path="*" element={<Where />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("OpenCodeDashboardLinkPage", () => {
  beforeEach(() => {
    canView = true;
    openCode.mockReset();
    openCustom.mockReset();
  });

  it("opens the link through the advanced dashboards' call and goes to it under Shared Dashboards", async () => {
    openCode.mockResolvedValue({ dashboardId: ID, own: false });
    renderAt(`/dashboard/shared/code-link/${LINK}`);
    expect(await screen.findByTestId("where")).toHaveTextContent(`/dashboard/shared/code/${ID}`);
    expect(openCode).toHaveBeenCalledWith(LINK);
    expect(openCustom).not.toHaveBeenCalled();
  });

  it("goes to the saved advanced page when it is the caller's own", async () => {
    openCode.mockResolvedValue({ dashboardId: ID, own: true });
    renderAt(`/dashboard/shared/code-link/${LINK}`);
    expect(await screen.findByTestId("where")).toHaveTextContent(`/dashboard/my/code/${ID}`);
  });

  it("says why a link that is off or unknown opened nothing", async () => {
    openCode.mockRejectedValue(new BackendApiError(404, "This link is off or no longer exists."));
    renderAt(`/dashboard/shared/code-link/${LINK}`);
    expect(await screen.findByText("This link is off or no longer exists.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Go to Shared Dashboards" })).toBeInTheDocument();
  });

  it("is the 404 page, and opens nothing, for a user who cannot view advanced dashboards", () => {
    canView = false;
    renderAt(`/dashboard/shared/code-link/${LINK}`);
    expect(screen.getByTestId("where")).toHaveTextContent("/404");
    expect(openCode).not.toHaveBeenCalled();
  });
});
