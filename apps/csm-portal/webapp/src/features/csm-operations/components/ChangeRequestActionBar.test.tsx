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
import ChangeRequestActionBar from "@features/csm-operations/components/ChangeRequestActionBar";
import type { BeChangeRequestDetail } from "@api/backend/types";

const BASE_CR: BeChangeRequestDetail = {
  id: "chg-1",
  number: "CHG0009988",
  subject: "Upgrade the gateway cluster",
  createdOn: "2026-01-01T00:00:00Z",
  state: "new",
  type: "normal",
  assignedTeam: { id: "team-1", name: "Platform" },
};

function renderBar(
  overrides: Partial<BeChangeRequestDetail>,
  { isPending = false, onAction = vi.fn() } = {},
): { onAction: ReturnType<typeof vi.fn>; container: HTMLElement } {
  const { container } = render(
    <ChangeRequestActionBar
      cr={{ ...BASE_CR, ...overrides }}
      isPending={isPending}
      onAction={onAction}
    />,
  );
  return { onAction, container };
}

/** Open the overflow menu, which must exist for this to succeed. */
function openMenu(): void {
  fireEvent.click(screen.getByRole("button", { name: /change state/i }));
}

describe("ChangeRequestActionBar — driven only by legalNextStates", () => {
  it("renders nothing when legalNextStates is absent", () => {
    const { container } = renderBar({ legalNextStates: undefined });
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when legalNextStates is empty", () => {
    const { container } = renderBar({ legalNextStates: [] });
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when the only entry is the CR's own current state", () => {
    const { container } = renderBar({ state: "assess", legalNextStates: ["assess"] });
    expect(container).toBeEmptyDOMElement();
  });

  it("offers only the states present in legalNextStates, not the whole lifecycle", () => {
    renderBar({ state: "scheduled", legalNextStates: ["implement", "canceled"] });
    // "Start implementation" is the forward move -> primary button.
    expect(
      screen.getByRole("button", { name: /start implementation/i }),
    ).toBeInTheDocument();
    openMenu();
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
    // Never offered: legal elsewhere in the lifecycle, but not in this array.
    expect(screen.queryByRole("menuitem", { name: /^close$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: /roll back/i })).not.toBeInTheDocument();
  });

  it("renders a state it has no curated config for, via the generic fallback", () => {
    renderBar({ state: "review", legalNextStates: ["closed", "awaiting_vendor"] });
    openMenu();
    // Sentence-cased from the raw value — no frontend change was needed for it.
    expect(
      screen.getByRole("menuitem", { name: /^awaiting vendor$/i }),
    ).toBeInTheDocument();
  });

  it("dispatches an uncurated state verbatim, not a normalised guess at it", () => {
    const { onAction } = renderBar({
      state: "review",
      legalNextStates: ["closed", "awaiting_vendor"],
    });
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: /^awaiting vendor$/i }));
    expect(onAction).toHaveBeenCalledWith("awaiting_vendor");
  });
});

describe("ChangeRequestActionBar — exactly one primary button", () => {
  it("promotes only the first forward move, even with six legal targets", () => {
    renderBar({
      state: "new",
      legalNextStates: [
        "closed",
        "customer_review",
        "review",
        "implement",
        "scheduled",
        "assess",
        "rollback",
        "canceled",
      ],
    });
    const contained = screen
      .getAllByRole("button")
      .filter((b) => b.className.includes("MuiButton-contained"));
    expect(contained).toHaveLength(1);
    expect(contained[0]).toHaveTextContent(/request approval/i);
  });

  it("puts every non-promoted target behind the Change state menu", () => {
    renderBar({ state: "new", legalNextStates: ["assess", "scheduled", "canceled"] });
    expect(screen.getByRole("button", { name: /request approval/i })).toBeInTheDocument();
    openMenu();
    expect(screen.getByRole("menuitem", { name: /^schedule$/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
  });

  it("renders no overflow menu at all when the single legal target is the primary one", () => {
    renderBar({ state: "scheduled", legalNextStates: ["implement"] });
    expect(screen.getByRole("button", { name: /start implementation/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /change state/i })).not.toBeInTheDocument();
  });

  it("never promotes a destructive target: with only cancel legal, there is no primary button", () => {
    renderBar({ state: "implement", legalNextStates: ["canceled"] });
    expect(screen.queryByRole("button", { name: /cancel change/i })).not.toBeInTheDocument();
    openMenu();
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
  });

  it("never promotes an uncurated target, even when it is the only legal one", () => {
    renderBar({ state: "review", legalNextStates: ["awaiting_vendor"] });
    expect(
      screen.queryByRole("button", { name: /^awaiting vendor$/i }),
    ).not.toBeInTheDocument();
    openMenu();
    expect(screen.getByRole("menuitem", { name: /^awaiting vendor$/i })).toBeInTheDocument();
  });
});

describe("ChangeRequestActionBar — labels are the action, not the destination", () => {
  it.each([
    ["assess", /request approval/i],
    ["scheduled", /^schedule$/i],
    ["implement", /start implementation/i],
    ["review", /mark implemented/i],
    ["customer_review", /send for customer review/i],
    ["closed", /^close$/i],
  ])("labels the %s transition as the action taken", (target, label) => {
    renderBar({ state: "new", legalNextStates: [target] });
    expect(screen.getByRole("button", { name: label })).toBeInTheDocument();
  });
});

describe("ChangeRequestActionBar — dispatch", () => {
  it("calls onAction with the target when the primary button is clicked", () => {
    const { onAction } = renderBar({ state: "new", legalNextStates: ["assess"] });
    fireEvent.click(screen.getByRole("button", { name: /request approval/i }));
    expect(onAction).toHaveBeenCalledWith("assess");
  });

  it("calls onAction with the target when a menu item is clicked, and closes the menu", () => {
    const { onAction } = renderBar({
      state: "implement",
      legalNextStates: ["review", "canceled"],
    });
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: /cancel change/i }));
    expect(onAction).toHaveBeenCalledWith("canceled");
  });
});

/**
 * Neither state is human-enterable in the backing system, so the bar must not
 * offer them however they arrive in `legalNextStates`. See
 * `NEVER_OFFERED_TARGETS` for why the filter exists — these tests are what
 * stops it being removed as dead code.
 */
describe("ChangeRequestActionBar — states the bar never offers", () => {
  it("renders neither rollback nor customer approval, as a button or a menu item", () => {
    renderBar({
      state: "implement",
      legalNextStates: ["review", "rollback", "customer_approval", "canceled"],
    });
    expect(screen.queryByRole("button", { name: /roll back/i })).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /customer approval/i }),
    ).not.toBeInTheDocument();
    openMenu();
    expect(screen.queryByRole("menuitem", { name: /roll back/i })).not.toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: /customer approval/i }),
    ).not.toBeInTheDocument();
  });

  it("still renders the other legal targets normally alongside them", () => {
    renderBar({
      state: "implement",
      legalNextStates: ["review", "rollback", "customer_approval", "canceled"],
    });
    expect(screen.getByRole("button", { name: /mark implemented/i })).toBeInTheDocument();
    openMenu();
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
  });

  it("excludes them even when they would otherwise render through the generic fallback", () => {
    // `customer_approval` has no curated action label, so without the
    // exclusion it would still be renderable via `DEFAULT_TARGET_CONFIG`.
    renderBar({ state: "assess", legalNextStates: ["customer_approval", "awaiting_vendor"] });
    openMenu();
    expect(screen.getByRole("menuitem", { name: /^awaiting vendor$/i })).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: /customer approval/i }),
    ).not.toBeInTheDocument();
  });

  it("renders no bar at all when every legal target is excluded", () => {
    const { container } = renderBar({
      state: "implement",
      legalNextStates: ["rollback", "customer_approval"],
    });
    expect(container).toBeEmptyDOMElement();
  });
});

describe("ChangeRequestActionBar — pending state", () => {
  it("disables the primary button while a transition is in flight", () => {
    renderBar({ state: "new", legalNextStates: ["assess", "canceled"] }, { isPending: true });
    expect(screen.getByRole("button", { name: /request approval/i })).toBeDisabled();
  });

  it("disables the Change state menu trigger while a transition is in flight", () => {
    renderBar({ state: "new", legalNextStates: ["assess", "canceled"] }, { isPending: true });
    expect(screen.getByRole("button", { name: /change state/i })).toBeDisabled();
  });
});

describe("ChangeRequestActionBar — per-target blocked reasons", () => {
  it("disables the assess transition when the CR has no assigned team", () => {
    const { onAction } = renderBar({
      state: "new",
      legalNextStates: ["assess"],
      assignedTeam: null,
    });
    const button = screen.getByRole("button", { name: /request approval/i });
    expect(button).toBeDisabled();
    fireEvent.click(button);
    expect(onAction).not.toHaveBeenCalled();
  });

  it("exposes the blocked reason to keyboard users via a focusable, labelled wrapper", () => {
    renderBar({ state: "new", legalNextStates: ["assess"], assignedTeam: null });
    const focusTarget = screen
      .getByRole("button", { name: /request approval/i })
      .closest('[tabindex="0"]');
    expect(focusTarget).not.toBeNull();
    expect(focusTarget).toHaveAttribute(
      "aria-label",
      "Request approval: Set an assigned team before requesting approval",
    );
  });

  it("leaves the transition enabled once the prerequisite is met", () => {
    renderBar({ state: "new", legalNextStates: ["assess"] });
    expect(screen.getByRole("button", { name: /request approval/i })).toBeEnabled();
  });

  it("blocks only the target with the unmet prerequisite, leaving the others clickable", () => {
    // `assess` is blocked *and* is first in FORWARD_ORDER, so it stays the
    // promoted (disabled) primary while `scheduled` stays usable behind the
    // menu — a blocked target must not take the rest of the bar down with it.
    const { onAction } = renderBar({
      state: "new",
      legalNextStates: ["scheduled", "assess"],
      assignedTeam: null,
    });
    expect(screen.getByRole("button", { name: /request approval/i })).toBeDisabled();
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: /^schedule$/i }));
    expect(onAction).toHaveBeenCalledWith("scheduled");
  });
});
