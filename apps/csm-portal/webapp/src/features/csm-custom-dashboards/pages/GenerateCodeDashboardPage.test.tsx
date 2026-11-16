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

const generateMock = vi.fn();
const saveMock = vi.fn();
const showSuccess = vi.fn();

vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    payload?: { message?: string; errorCode?: string };
    constructor(status: number, message: string, payload?: { message?: string; errorCode?: string }) {
      super(message);
      this.status = status;
      this.payload = payload;
    }
  },
}));
vi.mock("@features/csm-custom-dashboards/api/useGenerateCodeDashboard", () => ({
  useGenerateCodeDashboard: () => generateMock,
}));
vi.mock("@features/csm-custom-dashboards/api/useCodeDashboards", () => ({
  savedCodeDashboardPath: (id: string) => `/dashboard/my/code/${encodeURIComponent(id)}`,
  usePutCodeDashboard: () => ({ mutateAsync: saveMock, isPending: false }),
}));
vi.mock("@context/success-banner/SuccessBannerContext", () => ({ useSuccessBanner: () => ({ showSuccess }) }));
// The frame runs the code against the portal; it is tested on its own.
vi.mock("@features/csm-custom-dashboards/components/CodeDashboardFrame", () => ({
  default: ({ source }: { source: string }) => <div>frame running {source.length} characters</div>,
}));

import { BackendApiError } from "@api/backend/client";
import type { SavedCodeDashboard } from "@features/csm-custom-dashboards/api/useCodeDashboards";
import GenerateCodeDashboardPage from "@features/csm-custom-dashboards/pages/GenerateCodeDashboardPage";

const SOURCE = "async function render(portal, root) { root.textContent = 'hi'; }\n";

function renderPage(initialDashboard?: SavedCodeDashboard) {
  render(
    <MemoryRouter initialEntries={["/dashboard/my/new-code"]}>
      <Routes>
        <Route path="/dashboard/my/new-code" element={<GenerateCodeDashboardPage initialDashboard={initialDashboard} />} />
        <Route path="/dashboard/my" element={<div>My Dashboards</div>} />
        <Route path="/dashboard/my/code/:dashboardId" element={<OpenedPath />} />
      </Routes>
    </MemoryRouter>,
  );
}

/** Stands in for the saved dashboard's page: shows the id it was opened with. */
function OpenedPath() {
  const { dashboardId } = useParams();
  return <div>saved dashboard {dashboardId}</div>;
}

async function send(text: string, reply: string) {
  fireEvent.change(screen.getByRole("textbox", { name: "Message" }), { target: { value: text } });
  fireEvent.click(screen.getByRole("button", { name: "Send" }));
  await screen.findByText(reply);
}

describe("GenerateCodeDashboardPage", () => {
  beforeEach(() => {
    generateMock.mockReset();
    saveMock.mockReset();
    showSuccess.mockReset();
  });

  it("runs the generated code in the preview, never shows the code, and sends it back with a follow-up", async () => {
    generateMock.mockResolvedValue({ source: SOURCE, reply: "Here it is." });
    renderPage();
    expect(screen.getByText("The generated dashboard will appear here")).toBeInTheDocument();

    await send("My cases", "Here it is.");
    expect(generateMock).toHaveBeenCalledWith("My cases", null);
    expect(screen.getByText(`frame running ${SOURCE.length} characters`)).toBeInTheDocument();
    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
    expect(screen.queryByText(/root\.textContent/)).not.toBeInTheDocument();

    generateMock.mockResolvedValue({ source: SOURCE, reply: "Changed." });
    await send("Add a title", "Changed.");
    expect(generateMock).toHaveBeenLastCalledWith("Add a title", SOURCE);
  });

  it("says why a message failed and keeps the code it had", async () => {
    generateMock.mockResolvedValueOnce({ source: SOURCE, reply: "Here it is." });
    renderPage();
    await send("My cases", "Here it is.");

    generateMock.mockRejectedValueOnce(
      new BackendApiError(422, "declined", { message: "Revenue isn't part of the case data.", errorCode: "generate_declined" }),
    );
    fireEvent.change(screen.getByRole("textbox", { name: "Message" }), { target: { value: "Revenue by month" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(await screen.findByText(/Revenue isn't part of the case data\./)).toBeInTheDocument();
    expect(screen.getByText(/This dashboard cannot be modified with your input\./)).toBeInTheDocument();
    expect(screen.getByText(`frame running ${SOURCE.length} characters`)).toBeInTheDocument();
  });

  it("asks before leaving with work on the page, and leaves straight away without", async () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Back to My Dashboards" }));
    expect(await screen.findByText("My Dashboards")).toBeInTheDocument();
  });

  it("asks before discarding a generated dashboard", async () => {
    generateMock.mockResolvedValue({ source: SOURCE, reply: "Here it is." });
    renderPage();
    await send("My cases", "Here it is.");
    fireEvent.click(screen.getByRole("button", { name: "Discard" }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.queryByText("My Dashboards")).not.toBeInTheDocument();
  });

  /** Opens Save's dialog and confirms name. */
  async function saveAs(name: string) {
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.change(within(dialog).getByRole("textbox", { name: "Name" }), { target: { value: name } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
  }

  it("saves the code under a name and opens the saved dashboard", async () => {
    generateMock.mockResolvedValue({ source: SOURCE, reply: "Here it is." });
    saveMock.mockResolvedValue({ id: "x", name: "Open Cases", source: SOURCE, createdOn: "a", updatedOn: "b" });
    renderPage();
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();

    await send("My cases", "Here it is.");
    await saveAs("  Open Cases ");
    await waitFor(() => expect(showSuccess).toHaveBeenCalledWith('"Open Cases" was saved to My Dashboards.'));
    const first = saveMock.mock.calls[0][0] as { id: string; name: string; source: string };
    expect(first).toEqual({ id: expect.stringMatching(/^[0-9a-f-]{36}$/), name: "Open Cases", source: SOURCE });
    expect(await screen.findByText(`saved dashboard ${first.id}`)).toBeInTheDocument();
  });

  /** Opens Save's dialog and returns the name it starts with, then cancels. */
  async function suggestedName(): Promise<string> {
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("dialog");
    const value = (within(dialog).getByRole("textbox", { name: "Name" }) as HTMLInputElement).value;
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    return value;
  }

  it("starts Save's name from the model's suggestion, the latest one, kept by a reply without one", async () => {
    renderPage();
    generateMock.mockResolvedValueOnce({ source: SOURCE, reply: "Here it is.", name: "Open Cases" });
    await send("My cases", "Here it is.");
    expect(await suggestedName()).toBe("Open Cases");

    generateMock.mockResolvedValueOnce({ source: SOURCE.replace("hi", "hello"), reply: "Changed.", name: "Open Cases by Severity" });
    await send("By severity", "Changed.");
    expect(await suggestedName()).toBe("Open Cases by Severity");

    generateMock.mockResolvedValueOnce({ source: SOURCE.replace("hi", "hey"), reply: "Changed again." });
    await send("Say hey", "Changed again.");
    expect(await suggestedName()).toBe("Open Cases by Severity");
  });

  it("starts Save's name empty when the model suggested none", async () => {
    generateMock.mockResolvedValue({ source: SOURCE, reply: "Here it is." });
    renderPage();
    await send("My cases", "Here it is.");
    expect(await suggestedName()).toBe("");
  });

  it("keeps the dialog open with the reason when the save is refused", async () => {
    generateMock.mockResolvedValue({ source: SOURCE, reply: "Here it is." });
    saveMock.mockRejectedValue(new BackendApiError(409, "you already have an advanced dashboard named: Open Cases"));
    renderPage();
    await send("My cases", "Here it is.");
    await saveAs("Open Cases");
    expect(await screen.findByText('You already have a dashboard named "Open Cases". Choose a different name.')).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(showSuccess).not.toHaveBeenCalled();
    // Still unsaved: leaving asks first.
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "Discard" }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  describe("editing a saved dashboard", () => {
    const savedDashboard: SavedCodeDashboard = {
      id: "5e636a12-0000-4000-8000-000000000001",
      name: "Open Cases",
      source: SOURCE,
      createdOn: "a",
      updatedOn: "b",
    };

    it("starts from its code, with nothing to save or lose until a message changes it", async () => {
      renderPage(savedDashboard);
      expect(screen.getByText("Edit Dashboard (Advanced)")).toBeInTheDocument();
      expect(screen.getByText(`frame running ${SOURCE.length} characters`)).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
      fireEvent.click(screen.getByRole("button", { name: "Back to My Dashboards" }));
      expect(await screen.findByText(`saved dashboard ${savedDashboard.id}`)).toBeInTheDocument();
    });

    it("sends its code with a message and saves the change under its id and name", async () => {
      const changed = SOURCE.replace("hi", "hello");
      // The model's suggestion does not replace the saved name.
      generateMock.mockResolvedValue({ source: changed, reply: "Changed.", name: "Hello Dashboard" });
      saveMock.mockResolvedValue({ ...savedDashboard, source: changed });
      renderPage(savedDashboard);

      await send("Say hello", "Changed.");
      expect(generateMock).toHaveBeenCalledWith("Say hello", SOURCE);
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
      const dialog = await screen.findByRole("dialog");
      expect(within(dialog).getByRole("textbox", { name: "Name" })).toHaveValue("Open Cases");
      fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));

      await waitFor(() => expect(showSuccess).toHaveBeenCalledWith('"Open Cases" was updated.'));
      expect(saveMock).toHaveBeenCalledWith({ id: savedDashboard.id, name: "Open Cases", source: changed });
      expect(await screen.findByText(`saved dashboard ${savedDashboard.id}`)).toBeInTheDocument();
    });

    it("asks before discarding a change, and returns to the dashboard", async () => {
      generateMock.mockResolvedValue({ source: SOURCE.replace("hi", "hello"), reply: "Changed." });
      renderPage(savedDashboard);
      await send("Say hello", "Changed.");
      fireEvent.click(screen.getByRole("button", { name: "Discard" }));
      const dialog = screen.getByRole("dialog");
      fireEvent.click(within(dialog).getByRole("button", { name: /discard/i }));
      expect(await screen.findByText(`saved dashboard ${savedDashboard.id}`)).toBeInTheDocument();
    });
  });
});
