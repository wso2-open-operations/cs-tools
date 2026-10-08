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
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter, Route, Routes } from "react-router";
import type { BeIncidentTaskDetail } from "@api/backend/types";

const navigateMock = vi.fn();
const useGetIncidentTaskMock = vi.fn();
const patchMutateMock = vi.fn();

vi.mock("@hooks/useNavTransition", () => ({
  useNavTransition: () => navigateMock,
}));
vi.mock("@features/csm-operations/api/useGetIncidentTask", () => ({
  useGetIncidentTask: (id: string | undefined) => useGetIncidentTaskMock(id),
}));
vi.mock("@features/csm-operations/api/usePatchIncidentTask", () => ({
  usePatchIncidentTask: () => ({
    mutate: patchMutateMock,
    reset: vi.fn(),
    isPending: false,
    isError: false,
    error: null,
  }),
}));

// Imported after the mocks above so the module picks them up.
import IncidentTaskDetailPage from "@features/csm-operations/pages/IncidentTaskDetailPage";

const INCIDENT_ID = "11111111-1111-4111-8111-111111111111";
const TASK_ID = "22222222-2222-4222-8222-222222222222";

const TASK: BeIncidentTaskDetail = {
  id: TASK_ID,
  number: "CS-PORTAL-000021",
  subject: "[Incident Report] Create the incident report for INC0099782",
  state: "OPEN",
  stateLabel: "Open",
  incident: { id: INCIDENT_ID, number: "INC0099782" },
  assignmentGroup: { id: "g1", name: "Choreo SRE Team" },
  assignedTo: { id: "u1", name: "Sasmitha Ekanayaka" },
  description: "Write the report.\nAttach the timeline.",
  priority: "MODERATE",
  openedOn: "2026-10-05T08:00:00Z",
  closedOn: null,
};

function renderPage(state?: { from: string }): void {
  render(
    <MemoryRouter initialEntries={[{ pathname: `/operations/incident-tasks/${TASK_ID}`, state }]}>
      <Routes>
        <Route path="/operations/incident-tasks/:id" element={<IncidentTaskDetailPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("IncidentTaskDetailPage", () => {
  beforeEach(() => {
    navigateMock.mockReset();
    useGetIncidentTaskMock.mockReset();
    patchMutateMock.mockReset();
  });

  // Any state from any state, as in ServiceNow; the three closed ones sit
  // behind "Close" and its dialog.
  function chooseState(label: string): void {
    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: label }));
  }

  it("applies a not-closed state straight away", () => {
    useGetIncidentTaskMock.mockReturnValue({ data: TASK, isLoading: false, isError: false });
    renderPage();
    chooseState("Work in Progress");
    expect(patchMutateMock).toHaveBeenCalledWith({ id: TASK_ID, patch: { state: "WORK_IN_PROGRESS" } });
  });

  it("offers the open states and one Close", () => {
    useGetIncidentTaskMock.mockReturnValue({ data: TASK, isLoading: false, isError: false });
    renderPage();
    fireEvent.mouseDown(screen.getByRole("combobox"));
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual([
      "Pending",
      "Open",
      "Work in Progress",
      "Close",
    ]);
  });

  it("closes through the dialog, which asks for the outcome", () => {
    useGetIncidentTaskMock.mockReturnValue({ data: TASK, isLoading: false, isError: false });
    renderPage();
    chooseState("Close");
    expect(patchMutateMock).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Closed Complete")).toBeChecked();
    fireEvent.click(screen.getByLabelText("Closed Skipped"));
    fireEvent.change(screen.getByLabelText("Close notes"), { target: { value: "  covered by INC0099783  " } });
    fireEvent.click(screen.getByRole("button", { name: "Close task" }));
    expect(patchMutateMock).toHaveBeenCalledWith(
      { id: TASK_ID, patch: { state: "CLOSED_SKIPPED", closeNotes: "covered by INC0099783" } },
      expect.anything(),
    );
  });

  it("does nothing when the current state is chosen again", () => {
    useGetIncidentTaskMock.mockReturnValue({ data: TASK, isLoading: false, isError: false });
    renderPage();
    chooseState("Open");
    expect(patchMutateMock).not.toHaveBeenCalled();
  });

  it("reopens a closed task from the menu, and shows its close notes", () => {
    useGetIncidentTaskMock.mockReturnValue({
      data: { ...TASK, state: "CLOSED_COMPLETE", stateLabel: "Closed Complete", closeNotes: "Report filed." },
      isLoading: false,
      isError: false,
    });
    renderPage();
    expect(screen.getByText("Report filed.")).toBeInTheDocument();
    chooseState("Open");
    expect(patchMutateMock).toHaveBeenCalledWith({ id: TASK_ID, patch: { state: "OPEN" } });
  });

  it("shows a closed task's outcome, and Close starts from it", () => {
    useGetIncidentTaskMock.mockReturnValue({
      data: { ...TASK, state: "CLOSED_INCOMPLETE", stateLabel: "Closed Incomplete" },
      isLoading: false,
      isError: false,
    });
    renderPage();
    expect(screen.getByRole("combobox")).toHaveTextContent("Closed Incomplete");
    chooseState("Close");
    expect(screen.getByRole("radio", { name: "Closed Incomplete" })).toBeChecked();
  });

  it("shows the task and links to its incident's Related tab", () => {
    useGetIncidentTaskMock.mockReturnValue({ data: TASK, isLoading: false, isError: false });
    renderPage();

    expect(useGetIncidentTaskMock).toHaveBeenCalledWith(TASK_ID);
    expect(screen.getByText("CS-PORTAL-000021")).toBeInTheDocument();
    expect(screen.getByText(TASK.subject as string)).toBeInTheDocument();
    // The state chip (kept for print) and the State dropdown.
    expect(screen.getAllByText("Open")).toHaveLength(2);
    expect(screen.getByText("Moderate")).toBeInTheDocument();
    expect(screen.getByText("Choreo SRE Team")).toBeInTheDocument();
    expect(screen.getByText("Sasmitha Ekanayaka")).toBeInTheDocument();
    expect(screen.getByText(/Attach the timeline/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "INC0099782" })).toHaveAttribute(
      "href",
      `/operations/incidents/${INCIDENT_ID}?tab=related`,
    );
  });

  it("goes back to the page the row link came from", () => {
    useGetIncidentTaskMock.mockReturnValue({ data: TASK, isLoading: false, isError: false });
    renderPage({ from: "/somewhere/else" });
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(navigateMock).toHaveBeenCalledWith("/somewhere/else");
  });

  it("falls back to the parent incident's Related tab for a direct link", () => {
    useGetIncidentTaskMock.mockReturnValue({ data: TASK, isLoading: false, isError: false });
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(navigateMock).toHaveBeenCalledWith(`/operations/incidents/${INCIDENT_ID}?tab=related`);
  });

  it("renders not-found for an unknown id", () => {
    useGetIncidentTaskMock.mockReturnValue({ data: null, isLoading: false, isError: false });
    renderPage();
    expect(screen.getByText("Incident task not found")).toBeInTheDocument();
  });

  it("reports a failed load", () => {
    useGetIncidentTaskMock.mockReturnValue({ data: undefined, isLoading: false, isError: true });
    renderPage();
    expect(screen.getByText(`Could not load incident task ${TASK_ID}.`)).toBeInTheDocument();
  });
});
