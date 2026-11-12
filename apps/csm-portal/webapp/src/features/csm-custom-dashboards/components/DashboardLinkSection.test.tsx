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

let linkState: { data?: unknown; isLoading?: boolean; isError?: boolean };
const enableMock = vi.fn();
const disableMock = vi.fn();
let pending = false;
const gotIds: (string | undefined)[] = [];

vi.mock("@features/csm-custom-dashboards/api/useDashboardLink", () => ({
  useGetCustomDashboardLink: (id: string | undefined) => {
    gotIds.push(id);
    return linkState;
  },
  usePutCustomDashboardLink: () => ({ mutateAsync: enableMock, isPending: pending }),
  useDeleteCustomDashboardLink: () => ({ mutateAsync: disableMock, isPending: pending }),
}));

import DashboardLinkSection from "@features/csm-custom-dashboards/components/DashboardLinkSection";

const DASHBOARD = "5e636a12-0000-4000-8000-000000000001";
const LINK_ID = "5e636a12-0000-4000-8000-0000000000cc";
const LINK_URL = `${window.location.origin}/dashboard/shared/link/${LINK_ID}`;

function toggle(): HTMLElement {
  return screen.getByRole("checkbox", { name: "Anyone with CSM Portal Viewer access and the link can view" });
}

describe("DashboardLinkSection", () => {
  const writeText = vi.fn();

  beforeEach(() => {
    linkState = { data: { enabled: false, openedBy: 0 } };
    enableMock.mockReset().mockResolvedValue({ enabled: true, linkId: LINK_ID, openedBy: 0 });
    disableMock.mockReset().mockResolvedValue(null);
    writeText.mockReset().mockResolvedValue(undefined);
    pending = false;
    gotIds.length = 0;
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  });

  it("is off by default, with no link shown", () => {
    render(<DashboardLinkSection dashboardId={DASHBOARD} />);
    expect(gotIds).toContain(DASHBOARD);
    expect(toggle()).not.toBeChecked();
    expect(screen.getByText("Off: only the people and teams you share with can view")).toBeInTheDocument();
    expect(screen.queryByLabelText("Dashboard link")).not.toBeInTheDocument();
  });

  it("turns the link on", async () => {
    render(<DashboardLinkSection dashboardId={DASHBOARD} />);
    fireEvent.click(toggle());
    await waitFor(() => expect(enableMock).toHaveBeenCalledWith(DASHBOARD));
    expect(disableMock).not.toHaveBeenCalled();
  });

  it("shows the link, a Copy button and the count while on", () => {
    linkState = { data: { enabled: true, linkId: LINK_ID, openedBy: 3 } };
    render(<DashboardLinkSection dashboardId={DASHBOARD} />);
    expect(toggle()).toBeChecked();
    expect(screen.getByLabelText("Dashboard link")).toHaveValue(LINK_URL);
    expect(screen.getByText(/Opened by 3 people\./)).toBeInTheDocument();
  });

  it("counts one person in the singular, and none as not opened yet", () => {
    linkState = { data: { enabled: true, linkId: LINK_ID, openedBy: 1 } };
    const { unmount } = render(<DashboardLinkSection dashboardId={DASHBOARD} />);
    expect(screen.getByText(/Opened by 1 person\./)).toBeInTheDocument();
    unmount();
    linkState = { data: { enabled: true, linkId: LINK_ID, openedBy: 0 } };
    render(<DashboardLinkSection dashboardId={DASHBOARD} />);
    expect(screen.getByText(/Not opened yet\./)).toBeInTheDocument();
  });

  it("copies the link", async () => {
    linkState = { data: { enabled: true, linkId: LINK_ID, openedBy: 0 } };
    render(<DashboardLinkSection dashboardId={DASHBOARD} />);
    fireEvent.click(screen.getByRole("button", { name: "Copy link" }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(LINK_URL));
  });

  it("says so when the link can't be copied", async () => {
    writeText.mockRejectedValue(new Error("denied"));
    linkState = { data: { enabled: true, linkId: LINK_ID, openedBy: 0 } };
    render(<DashboardLinkSection dashboardId={DASHBOARD} />);
    fireEvent.click(screen.getByRole("button", { name: "Copy link" }));
    expect(await screen.findByText("The link couldn't be copied. Select it and copy it instead.")).toBeInTheDocument();
  });

  it("turns the link off", async () => {
    linkState = { data: { enabled: true, linkId: LINK_ID, openedBy: 2 } };
    render(<DashboardLinkSection dashboardId={DASHBOARD} />);
    fireEvent.click(toggle());
    await waitFor(() => expect(disableMock).toHaveBeenCalledWith(DASHBOARD));
    expect(enableMock).not.toHaveBeenCalled();
  });

  it("shows a failure to turn it on or off", async () => {
    enableMock.mockRejectedValue(new Error("boom"));
    const { unmount } = render(<DashboardLinkSection dashboardId={DASHBOARD} />);
    fireEvent.click(toggle());
    expect(await screen.findByText("The link couldn't be turned on. Try again in a moment.")).toBeInTheDocument();
    unmount();

    disableMock.mockRejectedValue(new Error("boom"));
    linkState = { data: { enabled: true, linkId: LINK_ID, openedBy: 0 } };
    render(<DashboardLinkSection dashboardId={DASHBOARD} />);
    fireEvent.click(toggle());
    expect(await screen.findByText("The link couldn't be turned off. Try again in a moment.")).toBeInTheDocument();
  });

  it("can't be switched while a change is in flight", () => {
    pending = true;
    render(<DashboardLinkSection dashboardId={DASHBOARD} />);
    expect(toggle()).toBeDisabled();
  });

  it("says when the link can't be loaded, with no switch", () => {
    linkState = { isError: true };
    render(<DashboardLinkSection dashboardId={DASHBOARD} />);
    expect(screen.getByText("Could not load this dashboard's link.")).toBeInTheDocument();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  });

  it("reads nothing while the dialog is closed", () => {
    linkState = { isLoading: false };
    render(<DashboardLinkSection dashboardId={undefined} />);
    expect(gotIds).toEqual([undefined]);
  });
});
