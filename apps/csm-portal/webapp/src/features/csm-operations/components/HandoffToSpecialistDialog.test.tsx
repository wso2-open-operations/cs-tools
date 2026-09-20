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

import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import HandoffToSpecialistDialog from "@features/csm-operations/components/HandoffToSpecialistDialog";
import type { BeIncidentHandoffResult } from "@api/backend/types";

function openSelect(name: RegExp | string): HTMLElement {
  fireEvent.mouseDown(screen.getByRole("combobox", { name }));
  return screen.getByRole("listbox");
}

function baseResult(overrides: Partial<BeIncidentHandoffResult> = {}): BeIncidentHandoffResult {
  return {
    assignmentGroup: { id: "grp-1", name: "Choreo Special Ops" },
    previousAssignmentGroup: null,
    reasonCode: "no-runbook",
    reasonDescription: "Runbook is not available",
    escalationTeam: null,
    task: { id: "task-1", number: "TASK0082502", subject: "[Runbook Task] …" },
    githubIssue: null,
    githubIssueError: null,
    incident: {
      id: "inc-1",
      number: "INC0001",
      openedOn: null,
      subject: "x",
      priority: null,
      state: "IN_PROGRESS",
      category: null,
    },
    ...overrides,
  };
}

describe("HandoffToSpecialistDialog — form", () => {
  it("submits the mandatory reason code only when the team select is hidden", () => {
    const onSubmit = vi.fn();
    render(
      <HandoffToSpecialistDialog
        showTeamSelect={false}
        isSubmitting={false}
        result={null}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );

    expect(screen.queryByLabelText(/team \(applies to choreo only\)/i)).not.toBeInTheDocument();

    fireEvent.mouseDown(screen.getByRole("combobox", { name: /reason/i }));
    fireEvent.click(within(screen.getByRole("listbox")).getByText(/runbook doesn't solve the incident/i));
    fireEvent.click(screen.getByRole("button", { name: /^escalate$/i }));

    expect(onSubmit).toHaveBeenCalledWith({
      reasonCode: "runbook-not-working",
      escalationTeam: undefined,
    });
  });

  it("shows the team select only when showTeamSelect is true, and includes the choice when submitted", () => {
    const onSubmit = vi.fn();
    render(
      <HandoffToSpecialistDialog
        showTeamSelect
        isSubmitting={false}
        result={null}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );

    openSelect(/^reason$/i);
    fireEvent.click(screen.getByRole("option", { name: /runbook is not available/i }));

    openSelect(/team \(applies to choreo only\)/i);
    fireEvent.click(screen.getByRole("option", { name: /choreo apim team/i }));

    fireEvent.click(screen.getByRole("button", { name: /^escalate$/i }));

    expect(onSubmit).toHaveBeenCalledWith({
      reasonCode: "no-runbook",
      escalationTeam: "choreo-apim-team",
    });
  });

  it("disables Escalate until a reason is chosen", () => {
    render(
      <HandoffToSpecialistDialog
        showTeamSelect={false}
        isSubmitting={false}
        result={null}
        onClose={() => {}}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: /^escalate$/i })).toBeDisabled();
  });

  it("shows a clean success result without a warning when there's no githubIssueError", () => {
    render(
      <HandoffToSpecialistDialog
        showTeamSelect={false}
        isSubmitting={false}
        result={baseResult()}
        onClose={() => {}}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getByText(/handed off to/i)).toBeInTheDocument();
    expect(screen.getByText(/choreo special ops/i)).toBeInTheDocument();
    expect(screen.queryByText(/could not be created/i)).not.toBeInTheDocument();
  });

  it("surfaces a githubIssueError distinctly on an otherwise-successful handoff", () => {
    render(
      <HandoffToSpecialistDialog
        showTeamSelect={false}
        isSubmitting={false}
        result={baseResult({ githubIssueError: "GitHub issue creation failed (401)" })}
        onClose={() => {}}
        onSubmit={vi.fn()}
      />,
    );

    // The handoff itself is still reported as a success…
    expect(screen.getByText(/handed off to/i)).toBeInTheDocument();
    // …but the GitHub failure is called out, not silently swallowed.
    expect(screen.getByText(/could not be created/i)).toBeInTheDocument();
    expect(screen.getByText(/401/)).toBeInTheDocument();
  });
});
