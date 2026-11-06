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

import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import FeaturePreviewDialog from "@components/header/FeaturePreviewDialog";
import { resetFeatureStatesForTests } from "@config/featureFlags";

// "admin" + "viewer", not "admin" alone: isSplAudience (which gates Customer
// Health) is keyed on actually holding the viewer role, not on full
// access -- an admin-only role set would already show those two as
// capability-hidden candidates regardless of any config override, making the
// "exactly these ids" assertions below noisy. Holding viewer alongside admin
// isolates the rows under test to only the ones this suite's own
// `setOverrides` calls create.
let mockRoles: string[] | undefined = ["admin", "viewer"];
let mockUserId: string | undefined = "user-1";
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: { roles: mockRoles, id: mockUserId },
    isLoading: false,
    isError: false,
  }),
}));

function setOverrides(value: unknown): void {
  window.config = {
    ...window.config,
    CSM_PORTAL_FEATURE_OVERRIDES: value,
  } as Window["config"];
  resetFeatureStatesForTests();
}

describe("FeaturePreviewDialog", () => {
  beforeEach(() => {
    localStorage.clear();
    mockRoles = ["admin", "viewer"];
    mockUserId = "user-1";
    setOverrides(undefined);
  });

  it("renders a row for every hidden/WIP feature, with its state badge", () => {
    setOverrides({ updates: "hidden", engagements: "wip" });
    render(<FeaturePreviewDialog open onClose={vi.fn()} />);

    expect(screen.getByText("Updates")).toBeInTheDocument();
    expect(screen.getByText("Hidden")).toBeInTheDocument();
    expect(screen.getByText("Engagements")).toBeInTheDocument();
    expect(screen.getByText("WIP")).toBeInTheDocument();
  });

  it("shows the empty state when nothing is hidden or WIP", () => {
    render(<FeaturePreviewDialog open onClose={vi.fn()} />);
    expect(
      screen.getByText("Nothing to preview right now — every feature is already visible to you."),
    ).toBeInTheDocument();
  });

  it("toggling a row's switch calls setOverride (persists to this admin's own storage)", () => {
    setOverrides({ updates: "hidden" });
    render(<FeaturePreviewDialog open onClose={vi.fn()} />);

    const toggle = screen.getByRole("switch", { name: "Preview Updates" });
    expect(toggle).not.toBeChecked();
    fireEvent.click(toggle);
    expect(toggle).toBeChecked();
    expect(localStorage.getItem("csm.featurePreview.v1.user-1")).toContain("updates");
  });

  it("reflects an already-stored override as checked on render", () => {
    setOverrides({ updates: "hidden" });
    localStorage.setItem(
      "csm.featurePreview.v1.user-1",
      JSON.stringify({ updates: true }),
    );
    render(<FeaturePreviewDialog open onClose={vi.fn()} />);
    expect(screen.getByRole("switch", { name: "Preview Updates" })).toBeChecked();
  });

  it("hides the state badge once this admin's own toggle is on, and brings it back when turned off", () => {
    setOverrides({ updates: "hidden" });
    render(<FeaturePreviewDialog open onClose={vi.fn()} />);

    expect(screen.getByText("Hidden")).toBeInTheDocument();

    const toggle = screen.getByRole("switch", { name: "Preview Updates" });
    fireEvent.click(toggle);
    expect(toggle).toBeChecked();
    expect(screen.queryByText("Hidden")).not.toBeInTheDocument();

    fireEvent.click(toggle);
    expect(toggle).not.toBeChecked();
    expect(screen.getByText("Hidden")).toBeInTheDocument();
  });

  it("renders no badge at all for an already-stored, previewed-on override", () => {
    setOverrides({ updates: "hidden" });
    localStorage.setItem(
      "csm.featurePreview.v1.user-1",
      JSON.stringify({ updates: true }),
    );
    render(<FeaturePreviewDialog open onClose={vi.fn()} />);
    expect(screen.getByRole("switch", { name: "Preview Updates" })).toBeChecked();
    expect(screen.queryByText("Hidden")).not.toBeInTheDocument();
  });

  it("does not offer a tab that is only hidden because its section is — only what the config names directly", () => {
    setOverrides({ operations: "hidden" });
    render(<FeaturePreviewDialog open onClose={vi.fn()} />);
    expect(screen.getByText("Operations")).toBeInTheDocument();
    expect(screen.queryByText("Incidents")).not.toBeInTheDocument();
    expect(screen.queryByText("Problem management")).not.toBeInTheDocument();
  });
});
