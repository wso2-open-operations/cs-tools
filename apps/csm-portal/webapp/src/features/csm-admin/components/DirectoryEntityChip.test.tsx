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
import { MemoryRouter } from "react-router";
import type { ReactElement } from "react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const navigateMock = vi.fn();
vi.mock("@hooks/useNavTransition", () => ({
  useNavTransition: () => navigateMock,
}));

import DirectoryEntityChip from "@features/csm-admin/components/DirectoryEntityChip";

// `from` — the chip's own current location — is what lets DirectoryMemberPage's
// Back button return here (a case, an account, a user profile) instead of
// always dropping the caller on the plain directory list; reported live as a
// bug ("Back to Teams" from a case always went to the Teams directory) before
// this existed.
function renderAt(initialEntry: string, ui: ReactElement) {
  return render(<MemoryRouter initialEntries={[initialEntry]}>{ui}</MemoryRouter>);
}

describe("DirectoryEntityChip", () => {
  it("navigates to the entity's directory page with the name and current location as router state", () => {
    renderAt(
      "/cases/case-1",
      <DirectoryEntityChip id="agent" name="Agent" routeBase="/admin/roles" />,
    );
    fireEvent.click(screen.getByText("Agent"));
    expect(navigateMock).toHaveBeenCalledWith("/admin/roles/agent", {
      state: { name: "Agent", from: "/cases/case-1" },
    });
  });

  it("stops the click from bubbling to a parent handler (e.g. a clickable table row)", () => {
    const parentOnClick = vi.fn();
    renderAt(
      "/customers/accounts/acct-1",
      <div onClick={parentOnClick}>
        <DirectoryEntityChip id="alpha" name="Alpha Team" routeBase="/admin/teams" />
      </div>,
    );
    fireEvent.click(screen.getByText("Alpha Team"));
    expect(navigateMock).toHaveBeenCalledWith("/admin/teams/alpha", {
      state: { name: "Alpha Team", from: "/customers/accounts/acct-1" },
    });
    expect(parentOnClick).not.toHaveBeenCalled();
  });

  it("encodes the id in the destination path", () => {
    renderAt(
      "/admin/groups",
      <DirectoryEntityChip id="a/b c" name="Weird Id" routeBase="/admin/groups" />,
    );
    fireEvent.click(screen.getByText("Weird Id"));
    expect(navigateMock).toHaveBeenCalledWith("/admin/groups/a%2Fb%20c", {
      state: { name: "Weird Id", from: "/admin/groups" },
    });
  });

  it("carries the current search/hash along with the pathname", () => {
    renderAt(
      "/people/user-1?tab=activity#roles",
      <DirectoryEntityChip id="agent" name="Agent" routeBase="/admin/roles" />,
    );
    fireEvent.click(screen.getByText("Agent"));
    expect(navigateMock).toHaveBeenCalledWith("/admin/roles/agent", {
      state: { name: "Agent", from: "/people/user-1?tab=activity#roles" },
    });
  });
});
