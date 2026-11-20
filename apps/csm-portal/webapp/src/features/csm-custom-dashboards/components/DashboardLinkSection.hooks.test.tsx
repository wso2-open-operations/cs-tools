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
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

// The custom dashboards' link hooks: given other hooks, the section must not
// call these at all.
const customCalls: string[] = [];
vi.mock("@features/csm-custom-dashboards/api/useDashboardLink", () => ({
  useGetCustomDashboardLink: () => {
    customCalls.push("get");
    return { data: { enabled: false, openedBy: 0 } };
  },
  usePutCustomDashboardLink: () => {
    customCalls.push("put");
    return { mutateAsync: vi.fn(), isPending: false };
  },
  useDeleteCustomDashboardLink: () => {
    customCalls.push("delete");
    return { mutateAsync: vi.fn(), isPending: false };
  },
}));

import DashboardLinkSection, {
  type DashboardLinkHooks,
} from "@features/csm-custom-dashboards/components/DashboardLinkSection";

const DASHBOARD = "5e636a12-0000-4000-8000-000000000001";
const LINK_ID = "5e636a12-0000-4000-8000-0000000000cc";

let linkState: { data?: unknown };
const gotIds: (string | undefined)[] = [];
const enableMock = vi.fn();
const disableMock = vi.fn();

const hooks = {
  useGetLink: (id: string | undefined) => {
    gotIds.push(id);
    return linkState;
  },
  usePutLink: () => ({ mutateAsync: enableMock, isPending: false }),
  useDeleteLink: () => ({ mutateAsync: disableMock, isPending: false }),
  linkUrl: (linkId: string) => `https://portal.example/advanced/${linkId}`,
} as unknown as DashboardLinkHooks;

function toggle(): HTMLElement {
  return screen.getByRole("checkbox", { name: "Anyone with CSM Portal Viewer access and the link can view" });
}

describe("DashboardLinkSection with other hooks", () => {
  beforeEach(() => {
    linkState = { data: { enabled: false, openedBy: 0 } };
    gotIds.length = 0;
    customCalls.length = 0;
    enableMock.mockReset().mockResolvedValue({ enabled: true, linkId: LINK_ID, openedBy: 0 });
    disableMock.mockReset().mockResolvedValue(null);
  });

  it("reads the link through them and shows their URL", () => {
    linkState = { data: { enabled: true, linkId: LINK_ID, openedBy: 2 } };
    render(<DashboardLinkSection dashboardId={DASHBOARD} hooks={hooks} />);
    expect(gotIds).toContain(DASHBOARD);
    expect(screen.getByLabelText("Dashboard link")).toHaveValue(`https://portal.example/advanced/${LINK_ID}`);
    expect(screen.getByText(/Opened by 2 people\./)).toBeInTheDocument();
    expect(customCalls).toEqual([]);
  });

  it("turns the link on through them", async () => {
    render(<DashboardLinkSection dashboardId={DASHBOARD} hooks={hooks} />);
    fireEvent.click(toggle());
    await waitFor(() => expect(enableMock).toHaveBeenCalledWith(DASHBOARD));
    expect(disableMock).not.toHaveBeenCalled();
    expect(customCalls).toEqual([]);
  });

  it("turns the link off through them", async () => {
    linkState = { data: { enabled: true, linkId: LINK_ID, openedBy: 0 } };
    render(<DashboardLinkSection dashboardId={DASHBOARD} hooks={hooks} />);
    fireEvent.click(toggle());
    await waitFor(() => expect(disableMock).toHaveBeenCalledWith(DASHBOARD));
    expect(enableMock).not.toHaveBeenCalled();
    expect(customCalls).toEqual([]);
  });
});
