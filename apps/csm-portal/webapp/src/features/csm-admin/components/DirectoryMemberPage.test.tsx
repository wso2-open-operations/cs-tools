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
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

// DirectoryMembersList owns its own API-fetching (see its own test file) --
// irrelevant here, where the only thing under test is DirectoryMemberPage's
// own name/Back-button logic.
vi.mock("@features/csm-admin/components/DirectoryMembersList", () => ({
  default: () => <div>member list</div>,
}));

import DirectoryMemberPage from "@features/csm-admin/components/DirectoryMemberPage";

function renderAt(initialEntry: string) {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Routes>
        <Route
          path="/admin/teams/:id"
          element={<DirectoryMemberPage filterKey="teamIds" entityNoun="team" listPath="/admin/teams" />}
        />
        <Route path="/admin/teams" element={<div>teams directory</div>} />
        <Route path="/cases/:id" element={<div>case detail page</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

// Regression: reported live as "Back to Teams" from a case always dropping
// the caller on the plain Teams directory instead of back on the case --
// DirectoryEntityChip now carries `from`, and this is the other half of the
// fix: reading it back out, with a plain "Back" label since the destination
// is no longer fixed.
describe("DirectoryMemberPage — Back button", () => {
  it("returns to location.state.from when present, labeled plain 'Back'", () => {
    render(
      <MemoryRouter
        initialEntries={[
          { pathname: "/admin/teams/alpha", state: { name: "Alpha Team", from: "/cases/case-1" } },
        ]}
      >
        <Routes>
          <Route
            path="/admin/teams/:id"
            element={<DirectoryMemberPage filterKey="teamIds" entityNoun="team" listPath="/admin/teams" />}
          />
          <Route path="/cases/:id" element={<div>case detail page</div>} />
        </Routes>
      </MemoryRouter>,
    );

    const back = screen.getByRole("link", { name: "Back" });
    expect(back).toHaveAttribute("href", "/cases/case-1");
  });

  it("falls back to listPath when there is no from state (a direct/shared link)", () => {
    renderAt("/admin/teams/alpha");

    const back = screen.getByRole("link", { name: "Back" });
    expect(back).toHaveAttribute("href", "/admin/teams");
  });

  it("shows the entity's name from state, falling back to the raw id", () => {
    render(
      <MemoryRouter
        initialEntries={[{ pathname: "/admin/teams/alpha", state: { name: "Alpha Team" } }]}
      >
        <Routes>
          <Route
            path="/admin/teams/:id"
            element={<DirectoryMemberPage filterKey="teamIds" entityNoun="team" listPath="/admin/teams" />}
          />
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByText("Alpha Team")).toBeInTheDocument();
  });

  it("falls back to the raw id when no name is in state (a direct/shared link)", () => {
    renderAt("/admin/teams/alpha");
    expect(screen.getByText("alpha")).toBeInTheDocument();
  });
});
