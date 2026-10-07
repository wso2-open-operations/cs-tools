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

const TEAMS = [
  { key: "choreo-special-ops", label: "Choreo Special Ops" },
  { key: "choreo-runtime-team", label: "Choreo Runtime Team" },
  { key: "choreo-apim-team", label: "Choreo APIM Team" },
];

function renderForm(props: Partial<Parameters<typeof HandoffToSpecialistDialog>[0]> = {}) {
  const onSubmit = vi.fn();
  render(
    <HandoffToSpecialistDialog
      teamOptions={TEAMS}
      isSubmitting={false}
      result={null}
      onClose={() => {}}
      onSubmit={onSubmit}
      {...props}
    />,
  );
  return onSubmit;
}

function pickReason(name: RegExp) {
  openSelect(/^reason$/i);
  fireEvent.click(screen.getByRole("option", { name }));
}

describe("HandoffToSpecialistDialog — form", () => {
  it("requires a team when the product has several, and sends the choice", () => {
    const onSubmit = renderForm();
    const escalate = screen.getByRole("button", { name: /^escalate$/i });

    pickReason(/runbook doesn't solve the incident/i);
    expect(escalate).toBeDisabled();

    const list = openSelect(/escalation team/i);
    expect(within(list).getAllByRole("option").map((o) => o.textContent)).toEqual([
      "-- Select --",
      "Choreo Special Ops",
      "Choreo Runtime Team",
      "Choreo APIM Team",
    ]);
    fireEvent.click(screen.getByRole("option", { name: /choreo apim team/i }));
    fireEvent.click(escalate);

    expect(onSubmit).toHaveBeenCalledWith({
      reasonCode: "runbook-not-working",
      escalationTeam: "choreo-apim-team",
    });
  });

  it("names the only team and asks nothing when the product has one", () => {
    const onSubmit = renderForm({ teamOptions: [{ key: "asgardeo-special-ops", label: "Asgardeo Special Ops" }] });

    expect(screen.queryByRole("combobox", { name: /escalation team/i })).not.toBeInTheDocument();
    expect(screen.getByText(/escalates to/i)).toHaveTextContent("Escalates to Asgardeo Special Ops.");

    pickReason(/runbook is not available/i);
    fireEvent.click(screen.getByRole("button", { name: /^escalate$/i }));

    expect(onSubmit).toHaveBeenCalledWith({ reasonCode: "no-runbook", escalationTeam: undefined });
  });

  it("cannot escalate while the teams load or when the service has none", () => {
    renderForm({ teamOptions: [], isLoadingTeams: true });
    expect(screen.getByText(/loading specialist teams/i)).toBeInTheDocument();
    pickReason(/runbook is not available/i);
    expect(screen.getByRole("button", { name: /^escalate$/i })).toBeDisabled();
  });

  it("says when no specialist team is configured", () => {
    renderForm({ teamOptions: [] });
    expect(screen.getByText(/no specialist team is configured/i)).toBeInTheDocument();
    pickReason(/runbook is not available/i);
    expect(screen.getByRole("button", { name: /^escalate$/i })).toBeDisabled();
  });

  it("disables Escalate until a reason is chosen", () => {
    renderForm({ teamOptions: [{ key: "asgardeo-special-ops", label: "Asgardeo Special Ops" }] });
    expect(screen.getByRole("button", { name: /^escalate$/i })).toBeDisabled();
  });

  it("shows a clean success result without a warning when there's no githubIssueError", () => {
    render(
      <HandoffToSpecialistDialog
        teamOptions={TEAMS}
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
        teamOptions={TEAMS}
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
