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

import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

// The custom dashboards' hooks: given other hooks, the dialog must not call
// any of these.
const customCalls: string[] = [];
function custom(name: string, result: unknown) {
  return () => {
    customCalls.push(name);
    return result;
  };
}
vi.mock("@features/csm-custom-dashboards/api/useDashboardShares", () => ({
  useGetCustomDashboardShares: custom("getShares", { data: [] }),
  usePutCustomDashboardShare: custom("putShare", { mutateAsync: vi.fn(), isPending: false }),
  useDeleteCustomDashboardShare: custom("deleteShare", { mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@features/csm-custom-dashboards/api/useDashboardTeamShares", () => ({
  useGetCustomDashboardTeamShares: custom("getTeamShares", { data: [] }),
  usePutCustomDashboardTeamShare: custom("putTeamShare", { mutateAsync: vi.fn(), isPending: false }),
  useDeleteCustomDashboardTeamShare: custom("deleteTeamShare", { mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@features/csm-custom-dashboards/api/useDashboardLink", () => ({
  useGetCustomDashboardLink: custom("getLink", { data: { enabled: false, openedBy: 0 } }),
}));
vi.mock("@features/csm-users/api/useSearchUsers", () => ({
  useSearchUsers: () => ({
    data: { users: [{ id: "alice", userName: "alice", name: "Alice Perera", email: "alice.perera@example.com", userType: "internal" }] },
  }),
}));
vi.mock("@features/csm-dashboard/api/useTeams", () => ({
  useTeams: () => ({ data: [{ id: "rigel", name: "Rigel", family: "cre-abt", creGroupId: "g-rigel" }] }),
}));
// Which hooks the link part was given: its own tests cover what it does with them.
let linkSectionHooks: unknown = "not rendered";
vi.mock("@features/csm-custom-dashboards/components/DashboardLinkSection", () => ({
  default: ({ dashboardId, hooks }: { dashboardId: string | undefined; hooks?: unknown }) => {
    linkSectionHooks = hooks;
    return <div data-testid="link-section">{dashboardId ?? "closed"}</div>;
  },
}));

import ShareCustomDashboardDialog, {
  type DashboardShareHooks,
} from "@features/csm-custom-dashboards/components/ShareCustomDashboardDialog";

const DASHBOARD = { id: "5e636a12-0000-4000-8000-000000000001", name: "Advanced Board" };

const got: { kind: string; id: string | undefined }[] = [];
const shareMock = vi.fn();
const unshareMock = vi.fn();
const shareTeamMock = vi.fn();
const unshareTeamMock = vi.fn();
const linkHooks = {
  useGetLink: (id: string | undefined) => {
    got.push({ kind: "link", id });
    return { data: { enabled: true, linkId: "l", openedBy: 0 } };
  },
  usePutLink: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteLink: () => ({ mutateAsync: vi.fn(), isPending: false }),
  linkUrl: (linkId: string) => linkId,
};
const hooks = {
  useGetShares: (id: string | undefined) => {
    got.push({ kind: "shares", id });
    return { data: [{ user: { id: "ben", email: "ben@example.com", name: "Ben Bee" }, sharedOn: "a" }] };
  },
  usePutShare: () => ({ mutateAsync: shareMock, isPending: false }),
  useDeleteShare: () => ({ mutateAsync: unshareMock, isPending: false, variables: undefined }),
  useGetTeamShares: (id: string | undefined) => {
    got.push({ kind: "teamShares", id });
    return { data: [{ team: { key: "apollo", name: "Apollo", family: "sre-abt" }, sharedOn: "a" }] };
  },
  usePutTeamShare: () => ({ mutateAsync: shareTeamMock, isPending: false }),
  useDeleteTeamShare: () => ({ mutateAsync: unshareTeamMock, isPending: false, variables: undefined }),
  linkHooks,
} as unknown as DashboardShareHooks;

function renderDialog(given: DashboardShareHooks = hooks) {
  render(<ShareCustomDashboardDialog open dashboard={DASHBOARD} onClose={vi.fn()} hooks={given} />);
}

describe("ShareCustomDashboardDialog with other hooks", () => {
  beforeEach(() => {
    got.length = 0;
    customCalls.length = 0;
    linkSectionHooks = "not rendered";
    shareMock.mockReset().mockResolvedValue([]);
    unshareMock.mockReset().mockResolvedValue(null);
    shareTeamMock.mockReset().mockResolvedValue([]);
    unshareTeamMock.mockReset().mockResolvedValue(null);
  });

  it("lists who it is shared with, and whether the link is on, from them alone", () => {
    renderDialog();
    expect(got).toEqual(
      expect.arrayContaining([
        { kind: "shares", id: DASHBOARD.id },
        { kind: "teamShares", id: DASHBOARD.id },
        { kind: "link", id: DASHBOARD.id },
      ]),
    );
    const shared = within(screen.getByLabelText("Shared with"));
    expect(shared.getByText("Ben Bee")).toBeInTheDocument();
    expect(shared.getByText("Apollo")).toBeInTheDocument();
    expect(shared.getByText("Anyone with CSM Portal Viewer access")).toBeInTheDocument();
    expect(customCalls).toEqual([]);
  });

  it("shares and unshares people through them", async () => {
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Share with Alice Perera" }));
    await waitFor(() => expect(shareMock).toHaveBeenCalledWith({ dashboardId: DASHBOARD.id, userId: "alice" }));
    fireEvent.click(screen.getByRole("button", { name: "Stop sharing with Ben Bee" }));
    await waitFor(() => expect(unshareMock).toHaveBeenCalledWith({ dashboardId: DASHBOARD.id, userId: "ben" }));
  });

  it("shares and unshares teams through them", async () => {
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Teams" }));
    fireEvent.click(screen.getByRole("button", { name: "Share with team Rigel" }));
    await waitFor(() => expect(shareTeamMock).toHaveBeenCalledWith({ dashboardId: DASHBOARD.id, teamKey: "rigel" }));
    fireEvent.click(screen.getByRole("button", { name: "Stop sharing with team Apollo" }));
    await waitFor(() => expect(unshareTeamMock).toHaveBeenCalledWith({ dashboardId: DASHBOARD.id, teamKey: "apollo" }));
  });

  it("hands their link hooks to the link part", () => {
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Anyone" }));
    expect(screen.getByTestId("link-section")).toHaveTextContent(DASHBOARD.id);
    expect(linkSectionHooks).toBe(linkHooks);
  });

  it("leaves the link to the custom dashboards' hooks when they give none", () => {
    renderDialog({ ...hooks, linkHooks: undefined });
    fireEvent.click(screen.getByRole("button", { name: "Anyone" }));
    expect(linkSectionHooks).toBeUndefined();
    expect(customCalls).toEqual(expect.arrayContaining(["getLink"]));
  });
});
