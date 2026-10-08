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

import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi, type Mock } from "vitest";
import "@testing-library/jest-dom/vitest";
import ChangeRequestActionBar from "@features/csm-operations/components/ChangeRequestActionBar";
import type { PendingCustomerReview } from "@features/csm-operations/utils/changeRequests";
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
  {
    isPending = false,
    onAction = vi.fn<(target: string) => void>(),
    pendingCustomerReview,
  }: {
    isPending?: boolean;
    onAction?: Mock<(target: string) => void>;
    pendingCustomerReview?: PendingCustomerReview | null;
  } = {},
): { onAction: Mock<(target: string) => void>; container: HTMLElement } {
  const { container } = render(
    <ChangeRequestActionBar
      cr={{ ...BASE_CR, ...overrides }}
      isPending={isPending}
      pendingCustomerReview={pendingCustomerReview}
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
    renderBar({ state: "new", legalNextStates: ["assess", "implement", "canceled"] });
    expect(screen.getByRole("button", { name: /request approval/i })).toBeInTheDocument();
    openMenu();
    expect(screen.getByRole("menuitem", { name: /start implementation/i })).toBeInTheDocument();
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
    ["assess", /^request approval$/i],
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
  it("renders neither rollback (outside the review states) nor customer approval, as a button or a menu item", () => {
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

  it("never offers rollback from any state but review and customer_review, even if the backend listed it", () => {
    for (const state of [
      "new", "assess", "authorize", "customer_approval", "scheduled", "implement",
      "closed", "canceled", "rollback",
    ]) {
      cleanup();
      const { container } = renderBar({ state, legalNextStates: ["rollback"] });
      expect(container, state).toBeEmptyDOMElement();
    }
  });

  /**
   * Authorize must only ever be reached as the automatic side effect of an
   * approver approving in the Approvers section (`ChangeRequestApprovals.tsx`)
   * — never via a direct click here, even from Assess, where it would
   * otherwise be the obvious next forward move. Same exclusion mechanism as
   * rollback/customer_approval above, for a different reason (a human
   * decision made elsewhere in the UI, not automation).
   */
  it("never offers authorize from Assess, as a button or a menu item", () => {
    renderBar({
      state: "assess",
      legalNextStates: ["authorize", "canceled"],
    });
    expect(screen.queryByRole("button", { name: /^authorize$/i })).not.toBeInTheDocument();
    openMenu();
    expect(screen.queryByRole("menuitem", { name: /^authorize$/i })).not.toBeInTheDocument();
  });

  it("excludes authorize even when it would otherwise render through the generic fallback", () => {
    // `authorize` has no curated action label, so without the exclusion it
    // would still be renderable via `DEFAULT_TARGET_CONFIG`.
    renderBar({ state: "assess", legalNextStates: ["authorize", "awaiting_vendor"] });
    openMenu();
    expect(screen.getByRole("menuitem", { name: /^awaiting vendor$/i })).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: /^authorize$/i })).not.toBeInTheDocument();
  });

  it("renders no bar at all from Assess when authorize is the only legal target", () => {
    const { container } = renderBar({
      state: "assess",
      legalNextStates: ["authorize"],
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

/**
 * "Request Approval" (New -> Assess) requires an assigned team: its members
 * are who the Peer Approval stage is provisioned for.
 */
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
      "Request Approval: Set an assigned team before requesting approval",
    );
  });

  it("leaves the transition enabled once the prerequisite is met", () => {
    renderBar({ state: "new", legalNextStates: ["assess"] });
    expect(screen.getByRole("button", { name: /request approval/i })).toBeEnabled();
  });

  describe("Request Approval needs a Customer Project when a customer box is ticked", () => {
    const REASON = "Select a Customer Project before requesting approval";
    const ACME = { id: "proj-a", name: "Acme Project" };

    it.each([
      ["Customer Approval", { customerApprovalRequired: true }],
      ["Customer Review", { customerReviewRequired: true }],
      ["both", { customerApprovalRequired: true, customerReviewRequired: true }],
    ])("is disabled, with the reason, when %s is ticked and there is no project", (_name, flags) => {
      const { onAction } = renderBar({ state: "new", legalNextStates: ["assess"], ...flags });
      const button = screen.getByRole("button", { name: /request approval/i });
      expect(button).toBeDisabled();
      expect(button.closest('[tabindex="0"]')).toHaveAttribute("aria-label", `Request Approval: ${REASON}`);
      fireEvent.click(button);
      expect(onAction).not.toHaveBeenCalled();
    });

    it("is enabled once a Customer Project is set", () => {
      renderBar({ state: "new", legalNextStates: ["assess"], customerApprovalRequired: true, project: ACME });
      expect(screen.getByRole("button", { name: /request approval/i })).toBeEnabled();
    });

    it("is enabled with no project when no customer part is required (a Standard change too)", () => {
      renderBar({ state: "new", type: "standard", legalNextStates: ["assess"], customerApprovalRequired: false });
      expect(screen.getByRole("button", { name: /request approval/i })).toBeEnabled();
    });

    it("names the missing team first, when both are missing", () => {
      renderBar({ state: "new", legalNextStates: ["assess"], assignedTeam: null, customerApprovalRequired: true });
      expect(screen.getByRole("button", { name: /request approval/i }).closest('[tabindex="0"]')).toHaveAttribute(
        "aria-label",
        "Request Approval: Set an assigned team before requesting approval",
      );
    });

    it("leaves Cancel change usable behind the menu", () => {
      const { onAction } = renderBar({ state: "new", legalNextStates: ["assess", "canceled"], customerReviewRequired: true });
      expect(screen.getByRole("button", { name: /request approval/i })).toBeDisabled();
      openMenu();
      fireEvent.click(screen.getByRole("menuitem", { name: /cancel change/i }));
      expect(onAction).toHaveBeenCalledWith("canceled");
    });
  });

  describe("Request Approval needs somebody to ask when a customer box is ticked (a project with at least one registered contact)", () => {
    const REASON = "Register a contact for the Customer Project before requesting approval";
    const PROJECT = { id: "proj-a", name: "Example Corp Platform" };
    const CONTACT = { id: "k1", name: "Mia Member", email: "mia.member@example.com" };

    it.each([
      ["Customer Approval", { customerApprovalRequired: true }],
      ["Customer Review", { customerReviewRequired: true }],
      ["both", { customerApprovalRequired: true, customerReviewRequired: true }],
    ])("is disabled, with the reason, when %s is ticked and the project has no registered contact", (_name, flags) => {
      const { onAction } = renderBar({ state: "new", legalNextStates: ["assess"], project: PROJECT, customerContacts: [], ...flags });
      const button = screen.getByRole("button", { name: /request approval/i });
      expect(button).toBeDisabled();
      expect(button.closest('[tabindex="0"]')).toHaveAttribute("aria-label", `Request Approval: ${REASON}`);
      fireEvent.click(button);
      expect(onAction).not.toHaveBeenCalled();
    });

    it("is enabled while the project has a registered contact (a requester-only project is the backend's refusal, shown when it answers)", () => {
      const { onAction } = renderBar({ state: "new", legalNextStates: ["assess"], project: PROJECT, customerContacts: [CONTACT], customerApprovalRequired: true });
      const button = screen.getByRole("button", { name: /request approval/i });
      expect(button).toBeEnabled();
      fireEvent.click(button);
      expect(onAction).toHaveBeenCalledWith("assess");
    });

    it("is enabled when no customer part is required, however few contacts the project has (a Standard change too)", () => {
      renderBar({ state: "new", type: "standard", legalNextStates: ["assess"], project: PROJECT, customerContacts: [] });
      expect(screen.getByRole("button", { name: /request approval/i })).toBeEnabled();
    });

    it("claims nothing while the contacts are not in the payload", () => {
      renderBar({ state: "new", legalNextStates: ["assess"], project: PROJECT, customerApprovalRequired: true });
      expect(screen.getByRole("button", { name: /request approval/i })).toBeEnabled();
    });

    it("names the missing project, or the missing team, before the missing contact", () => {
      renderBar({ state: "new", legalNextStates: ["assess"], customerContacts: [], customerApprovalRequired: true });
      expect(screen.getByRole("button", { name: /request approval/i }).closest('[tabindex="0"]')).toHaveAttribute(
        "aria-label",
        "Request Approval: Select a Customer Project before requesting approval",
      );
      cleanup();
      renderBar({ state: "new", legalNextStates: ["assess"], assignedTeam: null, project: PROJECT, customerContacts: [], customerApprovalRequired: true });
      expect(screen.getByRole("button", { name: /request approval/i }).closest('[tabindex="0"]')).toHaveAttribute(
        "aria-label",
        "Request Approval: Set an assigned team before requesting approval",
      );
    });

    it("leaves Cancel change usable behind the menu", () => {
      const { onAction } = renderBar({ state: "new", legalNextStates: ["assess", "canceled"], project: PROJECT, customerContacts: [], customerReviewRequired: true });
      expect(screen.getByRole("button", { name: /request approval/i })).toBeDisabled();
      openMenu();
      fireEvent.click(screen.getByRole("menuitem", { name: /cancel change/i }));
      expect(onAction).toHaveBeenCalledWith("canceled");
    });
  });

  // The backend ignores an Emergency change's two customer boxes (one CAB approval, nobody asked), so a box still ticked on
  // the stored row (an older change, or one written by the sync) needs neither a project nor a registered contact. The
  // button must not claim otherwise; a Normal change with the same row keeps its reason.
  describe("Request Approval on an Emergency change with a customer box still ticked", () => {
    const PROJECT_REASON = "Select a Customer Project before requesting approval";
    const CONTACT_REASON = "Register a contact for the Customer Project before requesting approval";
    const PROJECT = { id: "proj-a", name: "Example Corp Platform" };

    it.each([
      ["Customer Approval", { customerApprovalRequired: true }],
      ["Customer Review", { customerReviewRequired: true }],
      ["both", { customerApprovalRequired: true, customerReviewRequired: true }],
    ])("is enabled, with no reason, when %s is ticked and there is no Customer Project", (_name, flags) => {
      const { onAction } = renderBar({ state: "new", type: "emergency", legalNextStates: ["assess"], ...flags });
      const button = screen.getByRole("button", { name: /request approval/i });
      expect(button).toBeEnabled();
      expect(screen.queryByLabelText(/before requesting approval/i)).not.toBeInTheDocument();
      fireEvent.click(button);
      expect(onAction).toHaveBeenCalledWith("assess");
    });

    it.each([
      ["Customer Approval", { customerApprovalRequired: true }],
      ["Customer Review", { customerReviewRequired: true }],
      ["both", { customerApprovalRequired: true, customerReviewRequired: true }],
    ])("is enabled, with no reason, when %s is ticked and the project has no registered contact", (_name, flags) => {
      const { onAction } = renderBar({ state: "new", type: "emergency", legalNextStates: ["assess"], project: PROJECT, customerContacts: [], ...flags });
      const button = screen.getByRole("button", { name: /request approval/i });
      expect(button).toBeEnabled();
      expect(screen.queryByLabelText(/before requesting approval/i)).not.toBeInTheDocument();
      fireEvent.click(button);
      expect(onAction).toHaveBeenCalledWith("assess");
    });

    it("reads the type in any case, as the rest of the page does", () => {
      renderBar({ state: "new", type: "Emergency", legalNextStates: ["assess"], customerApprovalRequired: true });
      expect(screen.getByRole("button", { name: /request approval/i })).toBeEnabled();
    });

    it("still needs the assigned team, which the CAB stage is provisioned from", () => {
      renderBar({ state: "new", type: "emergency", legalNextStates: ["assess"], assignedTeam: null, customerApprovalRequired: true });
      const button = screen.getByRole("button", { name: /request approval/i });
      expect(button).toBeDisabled();
      expect(button.closest('[tabindex="0"]')).toHaveAttribute("aria-label", "Request Approval: Set an assigned team before requesting approval");
    });

    it("leaves a Normal change with the same stored row blocked, with the project reason and then the contact reason", () => {
      const { onAction } = renderBar({ state: "new", type: "normal", legalNextStates: ["assess"], customerApprovalRequired: true });
      const noProject = screen.getByRole("button", { name: /request approval/i });
      expect(noProject).toBeDisabled();
      expect(noProject.closest('[tabindex="0"]')).toHaveAttribute("aria-label", `Request Approval: ${PROJECT_REASON}`);
      fireEvent.click(noProject);
      cleanup();

      renderBar({ state: "new", type: "normal", legalNextStates: ["assess"], project: PROJECT, customerContacts: [], customerApprovalRequired: true }, { onAction });
      const noContact = screen.getByRole("button", { name: /request approval/i });
      expect(noContact).toBeDisabled();
      expect(noContact.closest('[tabindex="0"]')).toHaveAttribute("aria-label", `Request Approval: ${CONTACT_REASON}`);
      fireEvent.click(noContact);
      expect(onAction).not.toHaveBeenCalled();
    });
  });

  it("blocks only the target with the unmet prerequisite, leaving the others clickable", () => {
    // `assess` is blocked *and* is first in FORWARD_ORDER, so it stays the
    // promoted (disabled) primary while `canceled` stays usable behind the
    // menu — a blocked target must not take the rest of the bar down with it.
    const { onAction } = renderBar({
      state: "new",
      legalNextStates: ["canceled", "assess"],
      assignedTeam: null,
    });
    expect(screen.getByRole("button", { name: /request approval/i })).toBeDisabled();
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: /cancel change/i }));
    expect(onAction).toHaveBeenCalledWith("canceled");
  });
});

/**
 * CAB approval moves a CR to Scheduled automatically, and a Standard
 * change goes straight there from Request Approval -- there is no manual
 * "Schedule" button. The backend no longer lists `scheduled` in
 * `legalNextStates`; the bar also filters it defensively.
 */
describe("ChangeRequestActionBar — Request Approval flow, no manual Schedule", () => {
  it("shows 'Request Approval' and never 'Move to Assess' for a new CR", () => {
    renderBar({ state: "new", legalNextStates: ["assess", "canceled"] });
    expect(screen.getByRole("button", { name: "Request Approval" })).toBeInTheDocument();
    expect(screen.queryByText(/move to assess/i)).not.toBeInTheDocument();
  });

  it("never offers Schedule, as a button or menu item, even if the backend lists scheduled", () => {
    renderBar({ state: "authorize", legalNextStates: ["scheduled", "canceled"] });
    expect(screen.queryByRole("button", { name: /schedule/i })).not.toBeInTheDocument();
    openMenu();
    expect(screen.queryByRole("menuitem", { name: /schedule/i })).not.toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
  });

  it("renders no bar at all when scheduled is the only legal target", () => {
    const { container } = renderBar({ state: "authorize", legalNextStates: ["scheduled"] });
    expect(container).toBeEmptyDOMElement();
  });

  it("offers no Schedule for an Assess-stage CR (approval pending), only Cancel", () => {
    renderBar({ state: "assess", legalNextStates: ["authorize", "scheduled", "canceled"] });
    openMenu();
    expect(screen.queryByRole("menuitem", { name: /schedule|authorize/i })).not.toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
  });

  it("from Scheduled, the forward move is Start implementation (the CR got there automatically)", () => {
    renderBar({ state: "scheduled", legalNextStates: ["implement", "canceled"] });
    expect(screen.getByRole("button", { name: /start implementation/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /schedule/i })).not.toBeInTheDocument();
  });
});

/**
 * Customer Approval / Customer Review gates. Staff never record a customer's
 * approval or review: `scheduled` out of `customer_approval` and `closed` out of
 * `customer_review` are the customer's own answers, given in the Customer
 * Portal, so the bar never offers them -- not as a button, not as a menu entry,
 * not even a disabled one -- whatever `legalNextStates` says. `legalNextStates`
 * stays the single source of truth for which of Close / Send for customer
 * review the Review state offers.
 */
describe("ChangeRequestActionBar — customer approval and customer review gates", () => {
  const buttonLabels = (): Array<string | null> => screen.getAllByRole("button").map((b) => b.textContent);
  const menuLabels = (): Array<string | null> => screen.getAllByRole("menuitem").map((i) => i.textContent);

  it("from customer_approval offers Re-schedule and, in the menu, Cancel change: no way to record the customer's approval", () => {
    const { onAction } = renderBar({
      state: "customer_approval",
      customerApprovalRequired: true,
      legalNextStates: ["authorize", "canceled"],
    });
    // Re-schedule is the outlined button; "Change state" is the contained one (there is no primary move).
    expect(buttonLabels()).toEqual(["Re-schedule", "Change state"]);
    expect(screen.getByRole("button", { name: "Re-schedule" }).className).toContain("MuiButton-outlined");
    expect(screen.getByRole("button", { name: "Change state" }).className).toContain("MuiButton-contained");
    openMenu();
    expect(menuLabels()).toEqual(["Cancel change"]);
    // No bypass or Schedule wording anywhere, and the retired "Record customer approval" is long gone.
    expect(screen.queryByText(/bypass|record customer|^schedule/i)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("menuitem", { name: /cancel change/i }));
    expect(onAction).toHaveBeenCalledWith("canceled");
  });

  it("never offers `scheduled` out of customer_approval, even when the backend lists it (an older one did)", () => {
    const { onAction } = renderBar({
      state: "customer_approval",
      customerApprovalRequired: true,
      legalNextStates: ["scheduled", "authorize", "canceled"],
    });
    expect(buttonLabels()).toEqual(["Re-schedule", "Change state"]);
    openMenu();
    expect(menuLabels()).toEqual(["Cancel change"]);
    expect(screen.queryByText(/^schedul|bypass/i)).not.toBeInTheDocument();
    expect(onAction).not.toHaveBeenCalled();
  });

  it("offers only Cancel change at customer_approval when Re-schedule is not legal, and nothing when `scheduled` is the only target", () => {
    renderBar({ state: "customer_approval", legalNextStates: ["scheduled", "canceled"] });
    expect(buttonLabels()).toEqual(["Change state"]);
    openMenu();
    expect(menuLabels()).toEqual(["Cancel change"]);
    cleanup();
    const { container } = renderBar({ state: "customer_approval", legalNextStates: ["scheduled"] });
    expect(container).toBeEmptyDOMElement();
  });

  it("never offers the customer_approval state itself as an action, even when listed", () => {
    renderBar({
      state: "authorize",
      legalNextStates: ["customer_approval", "canceled"],
    });
    expect(screen.queryByRole("button", { name: /customer approval/i })).not.toBeInTheDocument();
    openMenu();
    expect(screen.queryByRole("menuitem", { name: /customer approval/i })).not.toBeInTheDocument();
  });

  it("offers `scheduled` from no state at all, even if the backend listed it", () => {
    for (const state of [
      "new", "assess", "authorize", "customer_approval", "scheduled", "implement", "review", "customer_review",
      "closed", "canceled", "rollback",
    ]) {
      cleanup();
      const { container } = renderBar({ state, legalNextStates: ["scheduled"] });
      expect(container, state).toBeEmptyDOMElement();
    }
  });

  it("offers no button or menu item labelled Schedule/Scheduled or Bypass in any state", () => {
    for (const state of ["new", "assess", "authorize", "customer_approval", "scheduled", "implement", "review", "customer_review"]) {
      renderBar({
        state,
        legalNextStates: ["assess", "scheduled", "implement", "review", "customer_review", "closed", "canceled"],
      });
      expect(screen.queryByRole("button", { name: /schedul|bypass/i }), state).not.toBeInTheDocument();
      const trigger = screen.queryByRole("button", { name: /change state/i });
      if (trigger) {
        fireEvent.click(trigger);
        expect(screen.queryByRole("menuitem", { name: /schedul|bypass/i }), state).not.toBeInTheDocument();
      }
      cleanup();
    }
  });

  it("Review with customer review NOT required offers Close (the primary move) and Cancel, and no customer review", () => {
    const { onAction } = renderBar({
      state: "review",
      customerReviewRequired: false,
      legalNextStates: ["closed", "canceled"],
    });
    const close = screen.getByRole("button", { name: "Close" });
    expect(close.className).toContain("MuiButton-contained");
    expect(screen.queryByText(/send for customer review/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/bypass/i)).not.toBeInTheDocument();
    openMenu();
    expect(screen.queryByRole("menuitem", { name: /customer review/i })).not.toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
    fireEvent.keyDown(screen.getByRole("menu"), { key: "Escape" });
    fireEvent.click(close);
    expect(onAction).toHaveBeenCalledWith("closed");
  });

  it("Review with customer review required offers Send for customer review and Cancel, and no Close", () => {
    renderBar({
      state: "review",
      customerReviewRequired: true,
      legalNextStates: ["customer_review", "canceled"],
    });
    expect(screen.getByRole("button", { name: "Send for customer review" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Close" })).not.toBeInTheDocument();
    openMenu();
    expect(screen.queryByRole("menuitem", { name: /^close$/i })).not.toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
  });

  it("from customer_review offers Roll back and Cancel change in the menu only: no Close, no way to record the customer's review", () => {
    const { onAction } = renderBar({
      state: "customer_review",
      customerReviewRequired: true,
      legalNextStates: ["rollback", "canceled"],
    });
    expect(buttonLabels()).toEqual(["Change state"]);
    expect(screen.getByRole("button", { name: "Change state" }).className).toContain("MuiButton-contained");
    expect(screen.queryByText(/send for customer review|bypass/i)).not.toBeInTheDocument();
    openMenu();
    expect(menuLabels()).toEqual(["Roll back", "Cancel change"]);
    fireEvent.click(screen.getByRole("menuitem", { name: "Roll back" }));
    expect(onAction).toHaveBeenCalledWith("rollback");
  });

  it("never offers `closed` out of customer_review, even when the backend lists it (an older one did)", () => {
    const { onAction } = renderBar({
      state: "customer_review",
      customerReviewRequired: true,
      legalNextStates: ["closed", "rollback", "canceled"],
    });
    expect(screen.queryByRole("button", { name: /^close$/i })).not.toBeInTheDocument();
    expect(buttonLabels()).toEqual(["Change state"]);
    openMenu();
    expect(screen.queryByRole("menuitem", { name: /close|bypass/i })).not.toBeInTheDocument();
    expect(menuLabels()).toEqual(["Roll back", "Cancel change"]);
    expect(onAction).not.toHaveBeenCalled();
  });

  it("renders no bar at all from customer_review when `closed` is the only legal target", () => {
    const { container } = renderBar({ state: "customer_review", legalNextStates: ["closed"] });
    expect(container).toBeEmptyDOMElement();
  });

  it("keeps `closed` out of every state but customer_review (Review's plain Close is not the customer's answer)", () => {
    for (const state of ["review", "implement", "scheduled"]) {
      cleanup();
      renderBar({ state, legalNextStates: ["closed"] });
      expect(screen.getByRole("button", { name: "Close" }), state).toBeInTheDocument();
    }
  });
});

/**
 * With a live Customer Review stage the backend offers only `canceled`; with
 * nobody to ask it offers Roll back too. Roll back is enabled exactly when
 * `legalNextStates` offers it, and shown disabled -- with who the customer's
 * review is waiting on -- when the caller says one is pending (a failed review
 * is the customer's to give, in the Customer Portal). Nothing here ever makes a
 * customer's approval or review something staff can record.
 */
describe("ChangeRequestActionBar — customer gates with and without a live customer stage", () => {
  const PENDING_REVIEW: PendingCustomerReview = { contactNames: ["Mira Santos"] };
  const ROLLBACK_PENDING_REASON =
    "Customer review is pending from Mira Santos. A failed review is theirs to give in the Customer Portal, so the change can't be rolled back from here.";

  it("customer_approval with a live customer request is the same as without: Re-schedule, and Cancel change in the menu", () => {
    // Nobody can be asked about Roll back at Customer Approval, so even a pending review passed in adds nothing.
    for (const pending of [null, PENDING_REVIEW]) {
      cleanup();
      renderBar(
        { state: "customer_approval", customerApprovalRequired: true, legalNextStates: ["authorize", "canceled"] },
        { pendingCustomerReview: pending },
      );
      expect(screen.getAllByRole("button").map((b) => b.textContent)).toEqual(["Re-schedule", "Change state"]);
      openMenu();
      expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual(["Cancel change"]);
    }
  });

  it("customer_review with legalNextStates=[canceled] and nothing known to be pending offers only Cancel", () => {
    renderBar({ state: "customer_review", customerReviewRequired: true, legalNextStates: ["canceled"] });
    expect(screen.queryByRole("button", { name: /^close$/i })).not.toBeInTheDocument();
    openMenu();
    expect(screen.getAllByRole("menuitem")).toHaveLength(1);
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
  });

  it("customer_review with a pending review shows Roll back DISABLED, saying who is asked and where they answer, next to Cancel", () => {
    const { onAction } = renderBar(
      { state: "customer_review", customerReviewRequired: true, legalNextStates: ["canceled"] },
      { pendingCustomerReview: PENDING_REVIEW },
    );
    expect(screen.getAllByRole("button").map((b) => b.textContent)).toEqual(["Change state"]);
    openMenu();
    expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual([
      `Roll back${ROLLBACK_PENDING_REASON}`,
      "Cancel change",
    ]);
    const rollBack = screen.getByRole("menuitem", { name: /^Roll back: / });
    expect(rollBack).toHaveAttribute("aria-disabled", "true");
    expect(rollBack).toHaveAttribute("aria-label", `Roll back: ${ROLLBACK_PENDING_REASON}`);
    fireEvent.click(rollBack);
    expect(onAction).not.toHaveBeenCalled();
    // Cancel is the one way out an engineer still has; nothing else is added: no Close, no bypass.
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).not.toHaveAttribute("aria-disabled", "true");
    expect(screen.queryByText(/bypass|^close/i)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("menuitem", { name: /cancel change/i }));
    expect(onAction).toHaveBeenCalledWith("canceled");
  });

  it("keeps the disabled Roll back reachable by keyboard so its reason can be read", () => {
    renderBar(
      { state: "customer_review", legalNextStates: ["canceled"] },
      { pendingCustomerReview: PENDING_REVIEW },
    );
    openMenu();
    const item = screen.getByRole("menuitem", { name: /^Roll back: / });
    act(() => item.focus());
    expect(item).toHaveFocus();
    expect(item).not.toHaveAttribute("disabled");
  });

  it("keeps Roll back disabled at Customer Review even if an older backend still lists it, and enables it with nothing pending", () => {
    const { onAction } = renderBar(
      { state: "customer_review", legalNextStates: ["closed", "rollback", "canceled"] },
      { pendingCustomerReview: PENDING_REVIEW },
    );
    openMenu();
    // `closed` is never offered, listed or not: Roll back and Cancel change only.
    expect(screen.getAllByRole("menuitem")).toHaveLength(2);
    const blocked = screen.getByRole("menuitem", { name: /^Roll back: / });
    expect(blocked).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(blocked);
    expect(onAction).not.toHaveBeenCalled();
    cleanup();
    const second = renderBar(
      { state: "customer_review", legalNextStates: ["rollback", "canceled"] },
      { pendingCustomerReview: null },
    );
    openMenu();
    const free = screen.getByRole("menuitem", { name: "Roll back" });
    expect(free).not.toHaveAttribute("aria-disabled", "true");
    fireEvent.click(free);
    expect(second.onAction).toHaveBeenCalledWith("rollback");
  });

  it("never blocks Roll back out of Review, whatever customer review is passed as pending", () => {
    renderBar(
      { state: "review", customerReviewRequired: true, legalNextStates: ["customer_review", "rollback", "canceled"] },
      { pendingCustomerReview: PENDING_REVIEW },
    );
    openMenu();
    expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual(["Roll back", "Cancel change"]);
    expect(screen.getByRole("menuitem", { name: "Roll back" })).not.toHaveAttribute("aria-disabled", "true");
  });

  it("uses a generic sentence when the pending review names nobody", () => {
    renderBar(
      { state: "customer_review", legalNextStates: ["canceled"] },
      { pendingCustomerReview: { contactNames: [] } },
    );
    openMenu();
    expect(screen.getByRole("menuitem", { name: /^Roll back: / })).toHaveTextContent(
      "Customer review is pending. A failed review is the customer's to give in the Customer Portal, so the change can't be rolled back from here.",
    );
  });

  it("adds the disabled Roll back only alongside targets the backend offered", () => {
    // No legal targets (a record the caller may not transition): nothing to render.
    const { container } = renderBar(
      { state: "customer_review", legalNextStates: [] },
      { pendingCustomerReview: PENDING_REVIEW },
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("never turns a pending customer review into a block on an ordinary Close out of Review", () => {
    const { onAction } = renderBar(
      { state: "review", customerReviewRequired: false, legalNextStates: ["closed", "rollback", "canceled"] },
      { pendingCustomerReview: PENDING_REVIEW },
    );
    const close = screen.getByRole("button", { name: "Close" });
    expect(close).toBeEnabled();
    openMenu();
    expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual(["Roll back", "Cancel change"]);
    fireEvent.keyDown(screen.getByRole("menu"), { key: "Escape" });
    fireEvent.click(close);
    expect(onAction).toHaveBeenCalledWith("closed");
  });

  it("adds no Roll back to states that are not Customer Review, whatever is passed as pending", () => {
    for (const state of ["new", "assess", "authorize", "customer_approval", "scheduled", "implement"]) {
      cleanup();
      renderBar({ state, legalNextStates: ["canceled"] }, { pendingCustomerReview: PENDING_REVIEW });
      openMenu();
      expect(screen.getAllByRole("menuitem").map((i) => i.textContent), state).toEqual(["Cancel change"]);
    }
  });

  it("disables the whole Change state menu while a transition is in flight, Roll back and Cancel change included", () => {
    renderBar({ state: "customer_review", legalNextStates: ["rollback", "canceled"] }, { isPending: true });
    expect(screen.getByRole("button", { name: /change state/i })).toBeDisabled();
  });
});

/**
 * How the gates look now that nobody answers for the customer: nothing in
 * either gate is styled as a bypass (no warning colour, no skip icon), and
 * nothing in any gate, for any `legalNextStates`, is a button named for one.
 */
describe("ChangeRequestActionBar — no bypass anywhere in either customer gate", () => {
  it("renders no 'bypass' button, menu entry or skip icon in either gate, whatever else is legal", () => {
    for (const [state, legal] of [
      ["customer_approval", ["scheduled", "authorize", "canceled"]],
      ["customer_approval", ["scheduled", "canceled"]],
      ["customer_approval", ["authorize", "canceled"]],
      ["customer_review", ["closed", "rollback", "canceled"]],
      ["customer_review", ["closed", "canceled"]],
      ["customer_review", ["rollback", "canceled"]],
    ] as const) {
      cleanup();
      const { container } = renderBar({ state, legalNextStates: [...legal] });
      for (const b of screen.getAllByRole("button")) {
        expect(b.textContent, `${state} ${legal.join(",")}`).not.toMatch(/bypass/i);
      }
      openMenu();
      for (const item of screen.getAllByRole("menuitem")) {
        expect(item.textContent, `${state} ${legal.join(",")}`).not.toMatch(/bypass/i);
      }
      expect(container.querySelector(".lucide-skip-forward"), `${state} ${legal.join(",")}`).toBeNull();
      expect(document.querySelector(".lucide-skip-forward"), `${state} ${legal.join(",")}`).toBeNull();
    }
  });

  it("orders the menu forward moves, then Roll back, then Cancel change (the destructive ones last, in the error colour)", () => {
    renderBar({ state: "review", customerReviewRequired: false, legalNextStates: ["canceled", "rollback", "closed"] });
    openMenu();
    const items = screen.getAllByRole("menuitem");
    expect(items.map((i) => i.textContent)).toEqual(["Roll back", "Cancel change"]);
    for (const item of items) expect(item.querySelector("span")).toHaveStyle({ color: "rgb(211, 47, 47)" });
  });
});

/**
 * The whole bar, state by state: which button is primary, which is the outlined
 * secondary, and what sits behind "Change state" (in order). Each row is the
 * legalNextStates the backend sends for that state with nothing blocking it.
 */
describe("ChangeRequestActionBar — what each state shows", () => {
  const TABLE: Array<{
    name: string;
    cr: Partial<BeChangeRequestDetail>;
    primary: string | null;
    secondary: string[];
    menu: string[];
  }> = [
    { name: "new", cr: { state: "new", legalNextStates: ["assess", "canceled"] }, primary: "Request Approval", secondary: [], menu: ["Cancel change"] },
    { name: "assess", cr: { state: "assess", legalNextStates: ["authorize", "canceled"] }, primary: null, secondary: [], menu: ["Cancel change"] },
    { name: "authorize", cr: { state: "authorize", legalNextStates: ["canceled"] }, primary: null, secondary: [], menu: ["Cancel change"] },
    {
      name: "customer_approval",
      cr: { state: "customer_approval", legalNextStates: ["authorize", "canceled"] },
      primary: null,
      secondary: ["Re-schedule"],
      menu: ["Cancel change"],
    },
    {
      name: "customer_approval (an older backend that still lists scheduled)",
      cr: { state: "customer_approval", legalNextStates: ["scheduled", "authorize", "canceled"] },
      primary: null,
      secondary: ["Re-schedule"],
      menu: ["Cancel change"],
    },
    { name: "scheduled", cr: { state: "scheduled", legalNextStates: ["implement", "canceled"] }, primary: "Start implementation", secondary: [], menu: ["Cancel change"] },
    { name: "implement", cr: { state: "implement", legalNextStates: ["review", "canceled"] }, primary: "Mark implemented", secondary: [], menu: ["Cancel change"] },
    {
      name: "review (customer review required)",
      cr: { state: "review", customerReviewRequired: true, legalNextStates: ["customer_review", "rollback", "canceled"] },
      primary: "Send for customer review",
      secondary: [],
      menu: ["Roll back", "Cancel change"],
    },
    {
      name: "review (no customer review)",
      cr: { state: "review", customerReviewRequired: false, legalNextStates: ["closed", "rollback", "canceled"] },
      primary: "Close",
      secondary: [],
      menu: ["Roll back", "Cancel change"],
    },
    {
      name: "customer_review (nobody being asked)",
      cr: { state: "customer_review", legalNextStates: ["rollback", "canceled"] },
      primary: null,
      secondary: [],
      menu: ["Roll back", "Cancel change"],
    },
    {
      name: "customer_review (an older backend that still lists closed)",
      cr: { state: "customer_review", legalNextStates: ["closed", "rollback", "canceled"] },
      primary: null,
      secondary: [],
      menu: ["Roll back", "Cancel change"],
    },
  ];

  it.each(TABLE)("$name", ({ cr, primary, secondary, menu }) => {
    renderBar(cr);
    const contained = screen.queryAllByRole("button").filter((b) => b.className.includes("MuiButton-contained"));
    const outlined = screen
      .getAllByRole("button")
      .filter((b) => b.className.includes("MuiButton-outlined") && b.textContent !== "Change state");
    // Exactly one contained button: the primary move, or "Change state" when there is none.
    expect(contained.map((b) => b.textContent)).toEqual([primary ?? "Change state"]);
    expect(outlined.map((b) => b.textContent)).toEqual(secondary);
    openMenu();
    expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual(menu);
  });

  it.each(["rollback", "closed", "canceled"])("%s is terminal and renders nothing", (state) => {
    const { container } = renderBar({ state, legalNextStates: [] });
    expect(container).toBeEmptyDOMElement();
  });
});

/** "Roll back": the failed-review off-ramp, offered from the two review states only. */
describe("ChangeRequestActionBar — Roll back", () => {
  it.each([
    ["review", ["closed", "rollback", "canceled"], /^close$/i, ["Roll back", "Cancel change"]],
    ["review", ["customer_review", "rollback", "canceled"], /send for customer review/i, ["Roll back", "Cancel change"]],
    // Out of Customer Review there is no forward move for staff (the customer's review is theirs to give): no primary at all.
    ["customer_review", ["rollback", "canceled"], null, ["Roll back", "Cancel change"]],
  ])("from %s (%j) offers Roll back as a destructive menu item next to the forward move", (state, legal, forward, expected) => {
    const { onAction } = renderBar({ state, legalNextStates: legal });
    // At most one primary button: the forward move, never Roll back.
    if (forward) expect(screen.getByRole("button", { name: forward })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /roll back/i })).not.toBeInTheDocument();
    openMenu();
    const items = screen.getAllByRole("menuitem").map((i) => i.textContent);
    expect(items).toEqual(expected);
    const item = screen.getByRole("menuitem", { name: /roll back/i });
    // Error colour, same as Cancel change.
    expect(item.querySelector("span")).toHaveStyle({ color: "rgb(211, 47, 47)" });
    fireEvent.click(item);
    expect(onAction).toHaveBeenCalledWith("rollback");
  });

  it("is not offered while only Cancel is legal (a live customer stage), nor from a terminal state", () => {
    renderBar({ state: "customer_review", legalNextStates: ["canceled"] });
    openMenu();
    expect(screen.getByRole("menuitem", { name: /cancel change/i })).toBeInTheDocument();
    expect(screen.queryByText(/roll back/i)).not.toBeInTheDocument();
    cleanup();
    const { container } = renderBar({ state: "rollback", legalNextStates: [] });
    expect(container).toBeEmptyDOMElement();
  });
});

/**
 * "Re-schedule": `authorize` from Customer Approval -- the planned time changed,
 * so the customer is asked to approve the new time (the change stays in Customer
 * Approval and never goes back through CAB). A secondary (outlined) button next to
 * the primary move; offered from `customer_approval` only. With the customer's
 * proposed time waiting for WSO2 it is "Propose a different time" instead.
 */
describe("ChangeRequestActionBar — Re-schedule", () => {
  it("is an outlined button beside 'Change state', which holds Cancel change", () => {
    const { onAction } = renderBar({
      state: "customer_approval",
      legalNextStates: ["authorize", "canceled"],
    });
    const contained = screen.getAllByRole("button").filter((b) => b.className.includes("MuiButton-contained"));
    expect(contained).toHaveLength(1);
    expect(contained[0]).toHaveTextContent("Change state");
    const reschedule = screen.getByRole("button", { name: "Re-schedule" });
    expect(reschedule.className).toContain("MuiButton-outlined");
    openMenu();
    expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual(["Cancel change"]);
    fireEvent.keyDown(screen.getByRole("menu"), { key: "Escape" });
    fireEvent.click(reschedule);
    expect(onAction).toHaveBeenCalledWith("authorize");
  });

  it("stays on offer while a customer group's approval is pending (Cancel is the only other action)", () => {
    renderBar({ state: "customer_approval", legalNextStates: ["authorize", "canceled"] });
    expect(screen.getByRole("button", { name: "Re-schedule" })).toBeInTheDocument();
    expect(screen.queryByText(/bypass/i)).not.toBeInTheDocument();
    openMenu();
    expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual(["Cancel change"]);
  });

  it("is disabled while a transition is in flight", () => {
    renderBar({ state: "customer_approval", legalNextStates: ["authorize", "canceled"] }, { isPending: true });
    expect(screen.getByRole("button", { name: "Re-schedule" })).toBeDisabled();
  });

  describe("with the customer's proposed time waiting for WSO2", () => {
    const PENDING = { startOn: "2030-03-08T09:00:00Z", endOn: "2030-03-08T11:00:00Z", answer: "pending" };

    it("reads 'Propose a different time' instead, outlined, and sends the same authorize; Accept is not in the bar (it is the banner's)", () => {
      const { onAction } = renderBar({ state: "customer_approval", legalNextStates: ["authorize", "canceled"], customerProposal: PENDING });
      expect(screen.queryByRole("button", { name: "Re-schedule" })).not.toBeInTheDocument();
      const counter = screen.getByRole("button", { name: "Propose a different time" });
      expect(counter.className).toContain("MuiButton-outlined");
      expect(screen.queryByRole("button", { name: /accept/i })).not.toBeInTheDocument();
      openMenu();
      expect(screen.getAllByRole("menuitem").map((i) => i.textContent)).toEqual(["Cancel change"]);
      fireEvent.keyDown(screen.getByRole("menu"), { key: "Escape" });
      fireEvent.click(counter);
      expect(onAction).toHaveBeenCalledWith("authorize");
    });

    it("reads Re-schedule again for every proposal that is not waiting for an answer, and for a state that is not Customer Approval", () => {
      for (const answer of ["agreed", "disagreed", "unanswered"]) {
        cleanup();
        renderBar({ state: "customer_approval", legalNextStates: ["authorize", "canceled"], customerProposal: { ...PENDING, answer } });
        expect(screen.getByRole("button", { name: "Re-schedule" }), answer).toBeInTheDocument();
      }
      cleanup();
      // A pending verdict on a change that is not in Customer Approval is not one the bar acts on.
      renderBar({ state: "authorize", legalNextStates: ["authorize", "canceled"], customerProposal: PENDING });
      expect(screen.queryByRole("button", { name: /re-schedule|propose a different time/i })).not.toBeInTheDocument();
    });

    it("is disabled while a transition is in flight", () => {
      renderBar({ state: "customer_approval", legalNextStates: ["authorize", "canceled"], customerProposal: PENDING }, { isPending: true });
      expect(screen.getByRole("button", { name: "Propose a different time" })).toBeDisabled();
    });
  });

  it("is never offered from any other state, even if the backend listed authorize", () => {
    for (const state of [
      "new", "assess", "authorize", "scheduled", "implement", "review", "customer_review",
      "closed", "canceled", "rollback",
    ]) {
      cleanup();
      renderBar({ state, legalNextStates: ["authorize"] });
      expect(screen.queryByRole("button", { name: /re-schedule/i }), state).not.toBeInTheDocument();
      expect(screen.queryByRole("button", { name: /authorize/i }), state).not.toBeInTheDocument();
    }
  });
});

describe("ChangeRequestActionBar — the Change state button", () => {
  it("says whether its menu is open, and which element the menu is", () => {
    renderBar({ state: "customer_approval", legalNextStates: ["authorize", "canceled"] });
    const button = screen.getByRole("button", { name: /change state/i });
    expect(button).toHaveAttribute("aria-haspopup", "menu");
    expect(button).toHaveAttribute("aria-expanded", "false");
    expect(button).not.toHaveAttribute("aria-controls");
    fireEvent.click(button);
    expect(button).toHaveAttribute("aria-expanded", "true");
    const menuId = button.getAttribute("aria-controls");
    expect(menuId).toBeTruthy();
    expect(document.getElementById(menuId!)).toBe(screen.getByRole("menu"));
    fireEvent.keyDown(screen.getByRole("menu"), { key: "Escape" });
  });
});
