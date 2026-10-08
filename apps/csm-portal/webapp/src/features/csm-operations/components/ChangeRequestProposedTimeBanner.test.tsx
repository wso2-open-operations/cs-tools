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

import type { ComponentProps } from "react";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import ChangeRequestProposedTimeBanner from "@features/csm-operations/components/ChangeRequestProposedTimeBanner";
import { PROPOSER_NOT_RECORDED_ACCEPT_REASON } from "@features/csm-operations/utils/changeRequests";
import { clearUserPreferredTimeZone, setUserPreferredTimeZone } from "@utils/dateTime";
import type { BeChangeRequestCustomerProposal, BeChangeRequestDetail } from "@api/backend/types";

// Synthetic: shapes only.
const CR: BeChangeRequestDetail = {
  id: "chg-1",
  number: "CHG0001234",
  subject: "Upgrade the gateway cluster",
  createdOn: "2026-01-01T00:00:00Z",
  state: "customer_approval",
  type: "normal",
  plannedStartOn: "2030-03-01 09:00:00",
  plannedEndOn: "2030-03-01 11:00:00",
};

const KNOWN: BeChangeRequestCustomerProposal = {
  startOn: "2030-03-08T09:00:00Z",
  endOn: "2030-03-08T11:00:00Z",
  answer: "pending",
  proposedByName: "Mia Member",
  proposedByEmail: "mia.member@example.com",
  proposedOn: "2030-02-01T10:00:00Z",
};

// "Not recorded": the backend names nobody.
const UNKNOWN: BeChangeRequestCustomerProposal = { startOn: KNOWN.startOn, endOn: KNOWN.endOn, answer: "pending" };

// Well before every time above, so the proposal is in the future.
const NOW = Date.UTC(2030, 1, 15);

function renderBanner(
  props: Partial<Omit<ComponentProps<typeof ChangeRequestProposedTimeBanner>, "cr">> & { cr?: Partial<BeChangeRequestDetail> } = {},
): { onAccept: ReturnType<typeof vi.fn>; onProposeDifferent: ReturnType<typeof vi.fn> } {
  const onAccept = vi.fn();
  const onProposeDifferent = vi.fn();
  const { cr, ...rest } = props;
  render(
    <ChangeRequestProposedTimeBanner
      cr={{ ...CR, ...cr }}
      proposal={KNOWN}
      isPending={false}
      nowMs={NOW}
      onAccept={onAccept}
      onProposeDifferent={onProposeDifferent}
      {...rest}
    />,
  );
  return { onAccept, onProposeDifferent };
}

/** The banner for a given proposal (the rest as `renderBanner` has it); returns `unmount` for a loop over proposals. */
function renderBannerWith(proposal: BeChangeRequestCustomerProposal): ReturnType<typeof render> {
  return render(
    <ChangeRequestProposedTimeBanner
      cr={CR}
      proposal={proposal}
      isPending={false}
      nowMs={NOW}
      onAccept={vi.fn()}
      onProposeDifferent={vi.fn()}
    />,
  );
}

const acceptButton = (): HTMLElement => screen.getByRole("button", { name: "Accept proposed time" });
const counterButton = (): HTMLElement => screen.getByRole("button", { name: "Propose a different time" });

describe("ChangeRequestProposedTimeBanner", () => {
  beforeEach(() => setUserPreferredTimeZone("UTC"));
  afterEach(() => vi.restoreAllMocks());
  afterEach(() => clearUserPreferredTimeZone());

  it("is a named region that says the customer proposed a new time (the proposer is recorded)", () => {
    renderBanner();
    const region = screen.getByRole("region", { name: "The customer proposed a new time" });
    expect(region).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "A time is stored on this change request" })).not.toBeInTheDocument();
    expect(within(region).queryByText("Stored time")).not.toBeInTheDocument();
    expect(region).toHaveTextContent(/The planned time stays as it is until you answer\./);
  });

  it("shows the planned window beside the proposed one, with the length the customer's proposal keeps", () => {
    renderBanner();
    const region = screen.getByRole("region");
    expect(within(region).getByText("Planned now")).toBeInTheDocument();
    expect(within(region).getByText("Mar 1, 2030, 9:00 AM to Mar 1, 2030, 11:00 AM")).toBeInTheDocument();
    expect(within(region).getByText("Proposed by the customer")).toBeInTheDocument();
    expect(within(region).getByText("Mar 8, 2030, 9:00 AM to Mar 8, 2030, 11:00 AM")).toBeInTheDocument();
    expect(within(region).getByText("Same length as the planned window (2 hours)")).toBeInTheDocument();
  });

  it("derives the proposed end from the planned length when the backend sent none", () => {
    renderBanner({ proposal: { startOn: "2030-03-08T10:00:00Z", answer: "pending", proposedByName: "Mia Member" } });
    expect(screen.getByText("Mar 8, 2030, 10:00 AM to Mar 8, 2030, 12:00 PM")).toBeInTheDocument();
  });

  it("shows the viewer's own time zone", () => {
    setUserPreferredTimeZone("Asia/Colombo");
    renderBanner();
    // 09:00 UTC is 14:30 in Colombo.
    expect(screen.getByText("Mar 8, 2030, 2:30 PM to Mar 8, 2030, 4:30 PM")).toBeInTheDocument();
  });

  describe("the proposer is known", () => {
    it("names who proposed it and when, with Accept as the one primary action", () => {
      renderBanner();
      expect(screen.getByTestId("cr-proposal-proposer")).toHaveTextContent(
        "Proposed by Mia Member (mia.member@example.com) on Feb 1, 2030, 10:00 AM.",
      );
      expect(screen.queryByText(/proposer is not recorded/i)).not.toBeInTheDocument();
      expect(acceptButton().className).toContain("MuiButton-contained");
      expect(counterButton().className).toContain("MuiButton-outlined");
      expect(screen.getAllByRole("button").filter((b) => b.className.includes("MuiButton-contained"))).toHaveLength(1);
    });

    it("names the proposer without a time when the backend sent none", () => {
      renderBanner({ proposal: { ...KNOWN, proposedOn: null } });
      expect(screen.getByTestId("cr-proposal-proposer")).toHaveTextContent("Proposed by Mia Member (mia.member@example.com).");
    });

    it("the two buttons call the caller; neither sends anything itself", () => {
      const { onAccept, onProposeDifferent } = renderBanner();
      fireEvent.click(acceptButton());
      expect(onAccept).toHaveBeenCalledTimes(1);
      expect(onProposeDifferent).not.toHaveBeenCalled();
      fireEvent.click(counterButton());
      expect(onProposeDifferent).toHaveBeenCalledTimes(1);
    });
  });

  describe("nobody is recorded as having proposed the stored time (a date WSO2 users write too, or one left over from an earlier round)", () => {
    const STORED_SENTENCE = "A time is stored (Mar 8, 2030, 9:00 AM to Mar 8, 2030, 11:00 AM) but nobody is recorded as having proposed it.";
    // What the backend says with it: canAccept false and the words of its refusal.
    const SERVER_SAYS = {
      ...UNKNOWN,
      proposerRecorded: false,
      canAccept: false,
      acceptBlockedReason:
        'nobody is recorded as having proposed this time (it may have been written by someone at WSO2 or left over from an earlier cycle), so it cannot be accepted: use "Propose a different time" to ask the customer to approve a time',
    };

    it("says a time is stored and nobody is recorded as having proposed it, with no claim that anything waits for an answer", () => {
      renderBanner({ proposal: UNKNOWN });
      expect(screen.getByTestId("cr-proposal-proposer")).toHaveTextContent(STORED_SENTENCE);
      const region = screen.getByRole("region", { name: "A time is stored on this change request" });
      expect(region).toHaveTextContent(/no proposal to accept/);
      expect(region).toHaveTextContent(/still being asked to approve the planned time/);
      expect(region).not.toHaveTextContent(/waiting for your answer|The planned time stays as it is until you answer|Proposed by/);
      // The recorded-proposer banner is not there.
      expect(screen.queryByRole("region", { name: "The customer proposed a new time" })).not.toBeInTheDocument();
      // It is information, not a call to answer: an info alert, not the warning a waiting proposal is.
      expect(region.className).toContain("MuiAlert-colorInfo");
    });

    it("Accept is disabled with the reason, focusable, and 'Propose a different time' stays on offer", () => {
      const { onAccept, onProposeDifferent } = renderBanner({ proposal: UNKNOWN });
      expect(acceptButton()).toBeDisabled();
      const reason = screen.getByLabelText(`Accept proposed time: ${PROPOSER_NOT_RECORDED_ACCEPT_REASON}`);
      expect(reason).toHaveAttribute("tabindex", "0");
      fireEvent.click(acceptButton());
      expect(onAccept).not.toHaveBeenCalled();
      expect(counterButton()).toBeEnabled();
      fireEvent.click(counterButton());
      expect(onProposeDifferent).toHaveBeenCalledTimes(1);
      // Nothing is the single primary action: no Accept to recommend.
      expect(screen.getAllByRole("button").filter((b) => b.className.includes("MuiButton-contained"))).toHaveLength(0);
    });

    it("gives the reason in the backend's own words when it sends them", () => {
      renderBanner({ proposal: SERVER_SAYS });
      expect(acceptButton()).toBeDisabled();
      expect(
        screen.getByLabelText(
          'Accept proposed time: Nobody is recorded as having proposed this time (it may have been written by someone at WSO2 or left over from an earlier cycle), so it cannot be accepted: use "Propose a different time" to ask the customer to approve a time.',
        ),
      ).toBeInTheDocument();
    });

    it("does not say the customer proposed it or that the planned time waits for WSO2, whichever way 'not recorded' arrives", () => {
      for (const proposal of [
        UNKNOWN,
        // The backend's own verdict wins over a name that came with it.
        { ...UNKNOWN, proposerRecorded: false, proposedByName: "Mia Member", proposedByEmail: "mia.member@example.com" },
        { ...UNKNOWN, proposerRecorded: true },
        { ...UNKNOWN, proposedByName: "  ", proposedByEmail: "" },
      ]) {
        const { unmount } = renderBannerWith(proposal);
        const region = screen.getByRole("region", { name: "A time is stored on this change request" });
        expect(screen.queryByRole("region", { name: "The customer proposed a new time" })).not.toBeInTheDocument();
        expect(within(region).getByText("Stored time")).toBeInTheDocument();
        expect(within(region).queryByText("Proposed by the customer")).not.toBeInTheDocument();
        expect(region).not.toHaveTextContent(/The customer proposed/);
        expect(region).not.toHaveTextContent(/Proposed by /);
        // The windows are still there, side by side.
        expect(within(region).getByText("Planned now")).toBeInTheDocument();
        expect(within(region).getAllByText("Mar 8, 2030, 9:00 AM to Mar 8, 2030, 11:00 AM").length).toBeGreaterThan(0);
        unmount();
      }
    });

    it("an email alone, or a name alone, is still a proposer on record", () => {
      renderBanner({ proposal: { ...UNKNOWN, proposedByEmail: "mia.member@example.com" } });
      expect(screen.getByTestId("cr-proposal-proposer")).toHaveTextContent("Proposed by mia.member@example.com.");
      expect(acceptButton().className).toContain("MuiButton-contained");
      expect(acceptButton()).toBeEnabled();
    });
  });

  describe("what Accept cannot do, said up front (the backend refuses each in words as well)", () => {
    it("is disabled once the proposed time has passed, with a focusable reason", () => {
      renderBanner({ nowMs: Date.UTC(2030, 2, 9) });
      expect(acceptButton()).toBeDisabled();
      expect(screen.getByLabelText("Accept proposed time: The proposed time has passed. Propose a different time.")).toHaveAttribute("tabindex", "0");
      expect(counterButton()).toBeEnabled();
    });

    it("is disabled while the change is on hold", () => {
      renderBanner({ cr: { onHold: true } });
      expect(acceptButton()).toBeDisabled();
      expect(screen.getByLabelText("Accept proposed time: This change request is on hold. Take it off hold first.")).toBeInTheDocument();
      expect(counterButton()).toBeEnabled();
    });

    it("is disabled when there is no planned window whose length the proposal could keep", () => {
      renderBanner({ cr: { plannedStartOn: null, plannedEndOn: null } });
      expect(acceptButton()).toBeDisabled();
      expect(screen.getByLabelText(/Accept proposed time: This change request has no planned window/)).toBeInTheDocument();
    });

    it("a click on the disabled Accept does nothing", () => {
      const { onAccept } = renderBanner({ cr: { onHold: true } });
      fireEvent.click(acceptButton());
      expect(onAccept).not.toHaveBeenCalled();
    });
  });

  it("disables both answers while a request is in flight", () => {
    renderBanner({ isPending: true });
    expect(acceptButton()).toBeDisabled();
    expect(counterButton()).toBeDisabled();
  });

  it("offers no Decline of its own: declining is keeping the current time in 'Propose a different time'", () => {
    renderBanner();
    expect(screen.queryByRole("button", { name: /decline|reject/i })).not.toBeInTheDocument();
  });
});
