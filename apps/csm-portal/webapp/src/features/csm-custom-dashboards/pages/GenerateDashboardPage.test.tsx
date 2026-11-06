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
import { MemoryRouter, Route, Routes, useParams } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { BeDashboard } from "@api/backend/types";

const generateMock = vi.fn();
const saveMock = vi.fn();
const showSuccess = vi.fn();
const showError = vi.fn();
let savePending = false;

vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));
vi.mock("@features/csm-custom-dashboards/api/useGenerateDashboard", () => ({
  useGenerateDashboard: () => generateMock,
}));
vi.mock("@features/csm-custom-dashboards/api/useCustomDashboards", () => ({
  usePutCustomDashboard: () => ({ mutateAsync: saveMock, isPending: savePending }),
}));
vi.mock("@context/success-banner/SuccessBannerContext", () => ({ useSuccessBanner: () => ({ showSuccess }) }));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({ useErrorBanner: () => ({ showError }) }));
// The preview's widgets fetch their own data; the grid is not under test here.
vi.mock("@features/csm-dashboard/components/DashboardWidgetGrid", () => ({
  default: ({ widgets }: { widgets: unknown[] }) => <div>{widgets.length} widgets</div>,
}));

import { BackendApiError } from "@api/backend/client";
import GenerateDashboardPage from "@features/csm-custom-dashboards/pages/GenerateDashboardPage";

function dashboard(name: string): BeDashboard {
  return { id: "5e636a12-0000-4000-8000-000000000001", displayName: name, isDefault: false, isTeamBased: false, widgets: [] };
}

function OpenedDashboard() {
  const { dashboardId } = useParams();
  return <div>Opened {dashboardId}</div>;
}

function renderPage(initialDashboard?: BeDashboard) {
  render(
    <MemoryRouter initialEntries={["/dashboard/my/new"]}>
      <Routes>
        <Route path="/dashboard/my/new" element={<GenerateDashboardPage initialDashboard={initialDashboard} />} />
        <Route path="/dashboard/my" element={<div>Opened</div>} />
        <Route path="/dashboard/my/:dashboardId" element={<OpenedDashboard />} />
      </Routes>
    </MemoryRouter>,
  );
}

// Sends a chat message and waits for its reply to be shown.
async function sendMessage(text: string, reply: string) {
  fireEvent.change(screen.getByRole("textbox", { name: "Message" }), { target: { value: text } });
  fireEvent.click(screen.getByRole("button", { name: "Send" }));
  await screen.findByText(reply);
}

describe("GenerateDashboardPage: Save", () => {
  beforeEach(() => {
    generateMock.mockReset();
    saveMock.mockReset();
    showSuccess.mockReset();
    showError.mockReset();
    savePending = false;
  });

  it("is disabled until there is a dashboard to save", () => {
    renderPage();
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it("saves the draft, says so, and opens the saved dashboard", async () => {
    const first = dashboard("My Cases");
    generateMock.mockResolvedValueOnce({ dashboard: first, reply: "Here it is." });
    saveMock.mockResolvedValue({ dashboard: first, createdOn: "a", updatedOn: "b" });
    renderPage();

    await sendMessage("My cases", "Here it is.");
    fireEvent.click(await screen.findByRole("button", { name: "Save" }));

    expect(saveMock).toHaveBeenCalledWith(first);
    expect(await screen.findByText(`Opened ${first.id}`)).toBeInTheDocument();
    expect(showSuccess).toHaveBeenCalledWith('"My Cases" was saved to My Dashboards.');
  });

  it("explains a name that is already taken, and keeps Save available", async () => {
    generateMock.mockResolvedValueOnce({ dashboard: dashboard("My Cases"), reply: "Here it is." });
    const err = new BackendApiError(409, "you already have a dashboard named: My Cases");
    saveMock.mockRejectedValue(err);
    renderPage();

    await sendMessage("My cases", "Here it is.");
    fireEvent.click(await screen.findByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(showError).toHaveBeenCalledWith(
        'You already have a dashboard named "My Cases". Ask for a different name in the chat, then save again.',
        err,
      ),
    );
    expect(showSuccess).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
  });

  it("shows Saving… while the save is in flight", () => {
    savePending = true;
    renderPage();
    expect(screen.getByRole("button", { name: "Saving…" })).toBeDisabled();
  });
});

describe("GenerateDashboardPage: editing a saved dashboard", () => {
  beforeEach(() => {
    generateMock.mockReset();
    saveMock.mockReset();
    showSuccess.mockReset();
    showError.mockReset();
    savePending = false;
  });

  it("starts from the saved dashboard, with nothing to save yet", () => {
    renderPage(dashboard("My Cases"));
    expect(screen.getByRole("heading", { name: "Edit dashboard" })).toBeInTheDocument();
    expect(screen.getByText("Preview")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    // No example prompts for an existing dashboard.
    expect(screen.queryByText("My open cases by severity")).not.toBeInTheDocument();
  });

  it("sends the saved dashboard with the first message, and saves the change in place", async () => {
    const saved = dashboard("My Cases");
    const changed = { ...dashboard("My Open Cases"), id: saved.id };
    generateMock.mockResolvedValueOnce({ dashboard: changed, reply: "Changed." });
    saveMock.mockResolvedValue({ dashboard: changed, createdOn: "a", updatedOn: "b" });
    renderPage(saved);

    await sendMessage("Only open ones", "Changed.");
    expect(generateMock).toHaveBeenCalledWith("Only open ones", saved);

    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText(`Opened ${saved.id}`)).toBeInTheDocument();
    expect(saveMock).toHaveBeenCalledWith(changed);
    expect(showSuccess).toHaveBeenCalledWith('"My Open Cases" was updated.');
  });

  it("goes back to the dashboard without asking when nothing changed", () => {
    const saved = dashboard("My Cases");
    renderPage(saved);
    fireEvent.click(screen.getByRole("button", { name: "Back to My Dashboards" }));
    expect(screen.getByText(`Opened ${saved.id}`)).toBeInTheDocument();
  });

  it("asks before dropping a change, saying the saved dashboard stays", async () => {
    generateMock.mockResolvedValueOnce({ dashboard: dashboard("My Open Cases"), reply: "Changed." });
    renderPage(dashboard("My Cases"));
    await sendMessage("Only open ones", "Changed.");

    fireEvent.click(screen.getByRole("button", { name: "Discard" }));
    expect(screen.getByText("Discard your changes?")).toBeInTheDocument();
    expect(screen.getByText("Your changes and the chat will be lost. The saved dashboard stays as it was.")).toBeInTheDocument();
  });
});

describe("GenerateDashboardPage: the chat", () => {
  beforeEach(() => {
    generateMock.mockReset();
    saveMock.mockReset();
    showSuccess.mockReset();
    showError.mockReset();
    savePending = false;
  });

  it("shows a placeholder until the first reply, then the preview", async () => {
    generateMock.mockResolvedValueOnce({ dashboard: dashboard("My Cases"), reply: "Here it is." });
    renderPage();
    expect(screen.getByText("Your dashboard will appear here")).toBeInTheDocument();

    await sendMessage("My cases", "Here it is.");

    expect(generateMock).toHaveBeenCalledWith("My cases", null);
    expect(screen.getByText("Preview")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "My Cases" })).toBeInTheDocument();
    expect(screen.queryByText("Your dashboard will appear here")).not.toBeInTheDocument();
  });

  it("says so when the first message can't be turned into a dashboard", async () => {
    generateMock.mockRejectedValueOnce(new Error("refused"));
    renderPage();

    await sendMessage("Hello", "The dashboard cannot be generated from the input.");

    expect(screen.getByText("Your dashboard will appear here")).toBeInTheDocument();
  });

  it.each([
    [429, "You've sent a lot of messages in a short time. Try again in 42 seconds."],
    [413, "The dashboard being edited is too large to change in the chat. Start a new one."],
  ])("shows the backend's message for a %d", async (status, message) => {
    generateMock.mockRejectedValueOnce(new BackendApiError(status, message));
    renderPage();

    await sendMessage("My cases", message);

    expect(screen.queryByText("The dashboard cannot be generated from the input.")).not.toBeInTheDocument();
  });

  it("keeps the preview when a follow-up message fails", async () => {
    generateMock
      .mockResolvedValueOnce({ dashboard: dashboard("My Cases"), reply: "Here it is." })
      .mockRejectedValueOnce(new Error("timeout"));
    renderPage();
    await sendMessage("My cases", "Here it is.");

    await sendMessage("Make it purple", "This dashboard cannot be modified with your input.");

    expect(screen.getByRole("heading", { name: "My Cases" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
  });

  it("closes and reopens the chat with the conversation intact", async () => {
    generateMock.mockResolvedValueOnce({ dashboard: dashboard("My Cases"), reply: "Here it is." });
    renderPage();
    await sendMessage("My cases", "Here it is.");

    fireEvent.click(screen.getByRole("button", { name: "Close chat" }));
    expect(screen.queryByRole("textbox", { name: "Message" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Open chat" }));
    expect(screen.getByText("Here it is.")).toBeInTheDocument();
    expect(screen.getByText("My cases")).toBeInTheDocument();
  });

  it("leaves straight away when there is nothing to lose", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Back to My Dashboards" }));
    expect(screen.queryByText("Discard this dashboard?")).not.toBeInTheDocument();
    expect(screen.getByText("Opened")).toBeInTheDocument();
  });

  it("asks before discarding a new dashboard, and leaves on Discard", async () => {
    generateMock.mockResolvedValueOnce({ dashboard: dashboard("My Cases"), reply: "Here it is." });
    renderPage();
    await sendMessage("My cases", "Here it is.");

    fireEvent.click(screen.getByRole("button", { name: "Back to My Dashboards" }));
    expect(screen.getByText("Discard this dashboard?")).toBeInTheDocument();
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Discard" }));

    expect(await screen.findByText("Opened")).toBeInTheDocument();
  });
});
