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
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { BeTeam } from "@api/backend/types";

// The dialog's team pickers go through `useSearchTeams`, which hits the
// backend client via react-query — stub it out, same approach
// `EditProblemDialog.test.tsx` uses for its own async reference pickers.
const useSearchTeamsMock = vi.fn();
vi.mock("@features/csm-admin/api/useSearchTeams", () => ({
  useSearchTeams: (...args: unknown[]) => useSearchTeamsMock(...args),
}));

import EditAccountTeamsDialog from "@features/csm-accounts/components/EditAccountTeamsDialog";

const CRE_TEAM: BeTeam = { id: "team-cre-1", name: "CRE Alpha", family: "cre-abt", creGroupId: "grp-cre-1" };
const CRE_TEAM_2: BeTeam = { id: "team-cre-2", name: "CRE Beta", family: "cre-abt", creGroupId: "grp-cre-2" };
const SRE_TEAM: BeTeam = { id: "team-sre-1", name: "SRE Gamma", family: "sre-abt", sreGroupId: "grp-sre-1" };
// A team of a different family — must never surface as a CRE/SRE option even
// though it's in the same catalogue response (mirrors CasesFilterBar's own
// family-scoping precedent for these two fields).
const OTHER_TEAM: BeTeam = { id: "team-other-1", name: "Random Team", family: "cs" };

const ALL_TEAMS = [CRE_TEAM, CRE_TEAM_2, SRE_TEAM, OTHER_TEAM];

function mockTeams(): void {
  useSearchTeamsMock.mockReturnValue({
    data: { teams: ALL_TEAMS, total: ALL_TEAMS.length, limit: 50, offset: 0 },
    isFetching: false,
    isError: false,
  });
}

async function openCreDropdown(): Promise<void> {
  const input = screen.getByLabelText("CRE team");
  fireEvent.focus(input);
  fireEvent.change(input, { target: { value: "CRE" } });
  await screen.findByRole("option", { name: "CRE Alpha" });
}

describe("EditAccountTeamsDialog", () => {
  it("disables Save when nothing has changed", () => {
    mockTeams();
    render(
      <EditAccountTeamsDialog isSaving={false} onClose={vi.fn()} onSave={vi.fn()} />,
    );
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it("scopes the CRE picker to cre-abt teams with a backing group id", async () => {
    mockTeams();
    render(
      <EditAccountTeamsDialog isSaving={false} onClose={vi.fn()} onSave={vi.fn()} />,
    );
    await openCreDropdown();
    expect(screen.getByRole("option", { name: "CRE Alpha" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "CRE Beta" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "SRE Gamma" })).not.toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Random Team" })).not.toBeInTheDocument();
  });

  it("enables Save and submits only the selected CRE team", async () => {
    mockTeams();
    const onSave = vi.fn();
    render(
      <EditAccountTeamsDialog isSaving={false} onClose={vi.fn()} onSave={onSave} />,
    );

    await openCreDropdown();
    fireEvent.click(screen.getByRole("option", { name: "CRE Alpha" }));

    const saveBtn = screen.getByRole("button", { name: "Save" });
    expect(saveBtn).not.toBeDisabled();
    fireEvent.click(saveBtn);
    expect(onSave).toHaveBeenCalledWith({ creTeamId: "team-cre-1" });
  });

  it("never submits a cleared team as a change (the backend can't clear yet)", () => {
    mockTeams();
    const onSave = vi.fn();
    render(
      <EditAccountTeamsDialog
        currentCreTeam={{ id: "team-cre-1", name: "CRE Alpha" }}
        isSaving={false}
        onClose={vi.fn()}
        onSave={onSave}
      />,
    );

    // MUI Autocomplete's clear ("x") button, present once a value is set.
    fireEvent.click(screen.getByLabelText("Clear", { exact: false }));

    // Clearing alone is not a submittable change: it would send null, which
    // the backend treats as "leave unchanged," not "clear" — so Save must
    // stay disabled rather than let the user believe it saved.
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it("drops a cleared field from the payload even when another field changed", async () => {
    mockTeams();
    const onSave = vi.fn();
    render(
      <EditAccountTeamsDialog
        currentCreTeam={{ id: "team-cre-1", name: "CRE Alpha" }}
        isSaving={false}
        onClose={vi.fn()}
        onSave={onSave}
      />,
    );

    fireEvent.click(screen.getByLabelText("Clear", { exact: false }));

    const sreInput = screen.getByLabelText("SRE team");
    fireEvent.focus(sreInput);
    fireEvent.change(sreInput, { target: { value: "SRE" } });
    await screen.findByRole("option", { name: "SRE Gamma" });
    fireEvent.click(screen.getByRole("option", { name: "SRE Gamma" }));

    const saveBtn = screen.getByRole("button", { name: "Save" });
    expect(saveBtn).not.toBeDisabled();
    fireEvent.click(saveBtn);
    // creTeamId must be absent, not null — the cleared CRE field never
    // reaches the payload, even though the SRE change makes Save clickable.
    expect(onSave).toHaveBeenCalledWith({ sreTeamId: "team-sre-1" });
  });

  it("shows the passed-in save error", () => {
    mockTeams();
    render(
      <EditAccountTeamsDialog
        isSaving={false}
        saveError="Could not update the account's teams. Please try again."
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );
    expect(
      screen.getByText("Could not update the account's teams. Please try again."),
    ).toBeInTheDocument();
  });

  it("disables actions while saving", () => {
    mockTeams();
    render(<EditAccountTeamsDialog isSaving onClose={vi.fn()} onSave={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Saving…" })).toBeDisabled();
  });

  it("calls onClose when Cancel is clicked", () => {
    mockTeams();
    const onClose = vi.fn();
    render(<EditAccountTeamsDialog isSaving={false} onClose={onClose} onSave={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onClose).toHaveBeenCalled();
  });
});
