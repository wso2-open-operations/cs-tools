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
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";
import ChangeRequestLifecycleStepper from "@features/csm-operations/components/ChangeRequestLifecycleStepper";

function list(): HTMLElement {
  return screen.getByRole("list", { name: /change request lifecycle/i });
}

/** The stage's list item, found by its visible label. */
function stage(label: string): HTMLElement {
  return within(list()).getByText(label).closest('[role="listitem"]') as HTMLElement;
}

/** Every stage's accessible text, in order: "New, done", "Assess, current", ... */
function readout(): string[] {
  return within(list())
    .getAllByRole("listitem")
    .map((item) => item.textContent ?? "");
}

describe("ChangeRequestLifecycleStepper", () => {
  it("plots the customer portal's eleven stages, in its order, as list items", () => {
    render(<ChangeRequestLifecycleStepper state="new" />);
    expect(screen.getAllByRole("listitem")).toHaveLength(11);
    expect(readout().map((t) => t.split(",")[0])).toEqual([
      "New",
      "Assess",
      "Authorize",
      "Customer Approval",
      "Scheduled",
      "Implement",
      "Review",
      "Customer Review",
      "Rollback",
      "Closed",
      "Canceled",
    ]);
  });

  it("marks the CR's current state with aria-current='step'", () => {
    render(<ChangeRequestLifecycleStepper state="implement" />);
    const current = screen.getByText("Implement").closest('[aria-current="step"]');
    expect(current).not.toBeNull();
    expect(document.querySelectorAll('[aria-current="step"]')).toHaveLength(1);
  });

  it("marks every state before the current one as complete, and none after", () => {
    render(<ChangeRequestLifecycleStepper state="implement" />);

    // Prior states (New, Assess, Authorize, Customer Approval, Scheduled) each
    // render a check icon (svg) inside their step marker.
    ["New", "Assess", "Authorize", "Customer Approval", "Scheduled"].forEach((label) => {
      expect(stage(label).querySelector("svg")).not.toBeNull();
    });

    // The current step itself carries aria-current, states after it don't
    // and have no check icon.
    const review = stage("Review");
    expect(review).not.toHaveAttribute("aria-current");
    expect(review.querySelector("svg")).toBeNull();
  });

  it("says each stage's status in words, not only by colour", () => {
    render(<ChangeRequestLifecycleStepper state="implement" />);
    expect(readout()).toEqual([
      "New, done",
      "Assess, done",
      "Authorize, done",
      "Customer Approval, done",
      "Scheduled, done",
      "Implement, current",
      "Review, upcoming",
      "Customer Review, upcoming",
      "Rollback, not taken",
      "Closed, upcoming",
      "Canceled, not taken",
    ]);
  });

  describe("Rollback and Canceled", () => {
    it("are plotted on a change that goes to plan, as not taken", () => {
      render(<ChangeRequestLifecycleStepper state="implement" />);
      expect(stage("Rollback")).toHaveTextContent("Rollback, not taken");
      expect(stage("Canceled")).toHaveTextContent("Canceled, not taken");
      expect(stage("Rollback")).not.toHaveAttribute("aria-current");
      expect(stage("Canceled")).not.toHaveAttribute("aria-current");
      // They keep their icon so they are still recognisable when faint.
      expect(stage("Rollback").querySelector("svg")).not.toBeNull();
      expect(stage("Canceled").querySelector("svg")).not.toBeNull();
    });

    it("is never shown as done, even once the change is closed", () => {
      render(<ChangeRequestLifecycleStepper state="closed" />);
      expect(stage("Closed")).toHaveAttribute("aria-current", "step");
      expect(stage("Rollback")).toHaveTextContent("Rollback, not taken");
      expect(stage("Canceled")).toHaveTextContent("Canceled, not taken");
      expect(stage("Customer Review")).toHaveTextContent("Customer Review, done");
    });

    it("makes Canceled the current stage of a canceled change, with no note about a diversion", () => {
      render(<ChangeRequestLifecycleStepper state="canceled" />);
      const current = document.querySelector('[aria-current="step"]');
      expect(current).toHaveTextContent("Canceled, current");
      expect(document.querySelectorAll('[aria-current="step"]')).toHaveLength(1);
      expect(screen.queryByText(/diverted from the standard path/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/current state:/i)).not.toBeInTheDocument();
    });

    it("makes Rollback the current stage of a rolled-back change", () => {
      render(<ChangeRequestLifecycleStepper state="rollback" approvals={[]} hasCustomerContacts />);
      expect(document.querySelector('[aria-current="step"]')).toHaveTextContent("Rollback, current");
      expect(readout()).toEqual([
        "New, done",
        "Assess, done",
        "Authorize, done",
        "Customer Approval, done",
        "Scheduled, done",
        "Implement, done",
        "Review, done",
        "Customer Review, not taken",
        "Rollback, current",
        "Closed, not taken",
        "Canceled, not taken",
      ]);
    });

    it("shows Customer Review done on a rolled-back change when its stage proves the review happened", () => {
      render(
        <ChangeRequestLifecycleStepper
          state="rollback"
          approvals={[{ stage: "Customer Review", status: "APPROVED" }]}
        />,
      );
      expect(stage("Customer Review")).toHaveTextContent("Customer Review, done");
    });

    it("says history not recorded for Customer Review of a rolled-back change when there is no stage and no contacts to have asked", () => {
      render(<ChangeRequestLifecycleStepper state="rollback" approvals={[]} hasCustomerContacts={false} />);
      expect(stage("Customer Review")).toHaveTextContent("Customer Review, history not recorded");
      expect(stage("Review")).toHaveTextContent("Review, done");
    });

    it("shows Customer Review rejected on a change the customer's review rolled back, with a cross", () => {
      render(
        <ChangeRequestLifecycleStepper
          state="rollback"
          approvals={[{ stage: "Customer Review", status: "REJECTED" }]}
        />,
      );
      expect(stage("Customer Review")).toHaveTextContent("Customer Review, rejected by the customer");
      expect(stage("Customer Review").querySelector("svg")).toHaveClass("lucide-x");
      expect(stage("Customer Review")).not.toHaveAttribute("aria-current");
      expect(document.querySelector('[aria-current="step"]')).toHaveTextContent("Rollback, current");
    });

    it("shows a change the customer rejected at Customer Approval as canceled there, the later stages never reached", () => {
      render(
        <ChangeRequestLifecycleStepper
          state="canceled"
          approvals={[
            { stage: "Peer Approval", status: "APPROVED" },
            { stage: "CAB Approval", status: "APPROVED" },
            { stage: "Customer Approval", status: "REJECTED" },
          ]}
        />,
      );
      expect(readout()).toEqual([
        "New, done",
        "Assess, done",
        "Authorize, done",
        "Customer Approval, rejected by the customer",
        "Scheduled, not taken",
        "Implement, not taken",
        "Review, not taken",
        "Customer Review, not taken",
        "Rollback, not taken",
        "Closed, not taken",
        "Canceled, current",
      ]);
      expect(screen.queryByText(/history not recorded/i)).not.toBeInTheDocument();
    });

    it("counts a recorded customer approval on a canceled change as proof Customer Approval was passed", () => {
      render(
        <ChangeRequestLifecycleStepper
          state="canceled"
          approvals={[
            { stage: "Peer Approval", status: "APPROVED" },
            { stage: "CAB Approval", status: "APPROVED" },
          ]}
          customerApproved
        />,
      );
      expect(stage("Customer Approval")).toHaveTextContent("Customer Approval, done");
      expect(stage("Scheduled")).toHaveTextContent("Scheduled, history not recorded");
    });

    it("marks nothing done on a canceled change the approvals cannot vouch for", () => {
      render(<ChangeRequestLifecycleStepper state="canceled" approvals={[]} />);
      expect(readout()).toEqual([
        "New, history not recorded",
        "Assess, history not recorded",
        "Authorize, history not recorded",
        "Customer Approval, history not recorded",
        "Scheduled, history not recorded",
        "Implement, history not recorded",
        "Review, history not recorded",
        "Customer Review, history not recorded",
        "Rollback, not taken",
        "Closed, not taken",
        "Canceled, current",
      ]);
    });

    it("marks what the approvals prove done on a canceled change", () => {
      render(
        <ChangeRequestLifecycleStepper
          state="canceled"
          approvals={[
            { stage: "Peer Approval", status: "APPROVED" },
            { stage: "CAB Approval", status: "PENDING" },
          ]}
        />,
      );
      expect(stage("New")).toHaveTextContent("New, done");
      expect(stage("Assess")).toHaveTextContent("Assess, done");
      expect(stage("Authorize")).toHaveTextContent("Authorize, history not recorded");
      expect(stage("Canceled")).toHaveTextContent("Canceled, current");
    });
  });

  it("shows no note for a normal forward state", () => {
    render(<ChangeRequestLifecycleStepper state="scheduled" />);
    expect(screen.queryByText(/diverted from the standard path/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/current state:/i)).not.toBeInTheDocument();
  });

  it("shows a current-state note for a state that is not one of the eleven", () => {
    render(<ChangeRequestLifecycleStepper state="some_future_state" />);
    expect(screen.getByText(/current state:/i)).toBeInTheDocument();
    expect(screen.getByText("some future state")).toBeInTheDocument();
    // No marker claims to be current, and none render complete.
    expect(document.querySelector('[aria-current="step"]')).toBeNull();
    expect(readout().every((t) => /upcoming|not taken/.test(t))).toBe(true);
  });

  it("shows no current-state note when the CR has no state yet", () => {
    render(<ChangeRequestLifecycleStepper state={null} />);
    expect(screen.queryByText(/current state:/i)).not.toBeInTheDocument();
    expect(document.querySelector('[aria-current="step"]')).toBeNull();
  });

  it("highlights Customer Approval as the current step when the CR is in customer_approval", () => {
    render(
      <ChangeRequestLifecycleStepper
        state="customer_approval"
        customerApprovalRequired
        customerReviewRequired={false}
      />,
    );
    const current = document.querySelector('[aria-current="step"]');
    expect(current).not.toBeNull();
    expect(current).toHaveTextContent("Customer Approval, current");
  });

  it("leaves the optional customer steps off the line when their flags are false", () => {
    render(
      <ChangeRequestLifecycleStepper
        state="scheduled"
        customerApprovalRequired={false}
        customerReviewRequired={false}
      />,
    );
    // Seven stages of the path (New ... Closed minus the two) plus Rollback and Canceled.
    expect(screen.getAllByRole("listitem")).toHaveLength(9);
    expect(screen.queryByText("Customer Approval")).not.toBeInTheDocument();
    expect(screen.queryByText("Customer Review")).not.toBeInTheDocument();
    expect(screen.getByText("Rollback")).toBeInTheDocument();
  });

  it("keeps each optional customer step only when its own flag is on", () => {
    render(
      <ChangeRequestLifecycleStepper
        state="implement"
        customerApprovalRequired
        customerReviewRequired={false}
      />,
    );
    expect(screen.getAllByRole("listitem")).toHaveLength(10);
    expect(screen.getByText("Customer Approval")).toBeInTheDocument();
    expect(screen.queryByText("Customer Review")).not.toBeInTheDocument();
  });

  it("keeps a customer step the CR is currently in even if its flag reads false", () => {
    render(<ChangeRequestLifecycleStepper state="customer_review" customerReviewRequired={false} />);
    expect(document.querySelector('[aria-current="step"]')).toHaveTextContent("Customer Review");
  });

  describe("the line between the stages", () => {
    /** How the connector leading into each stage is drawn, by stage label. */
    function lines(): Record<string, string> {
      const into: Record<string, string> = {};
      for (const item of within(list()).getAllByRole("listitem")) {
        const label = (item.textContent ?? "").split(",")[0]!;
        into[label] = item.querySelectorAll("[data-segment]")[0]!.getAttribute("data-segment")!;
      }
      return into;
    }

    it("fills up to the current stage, and draws what is still ahead plain", () => {
      render(<ChangeRequestLifecycleStepper state="implement" />);
      expect(lines()).toMatchObject({
        Assess: "filled",
        Scheduled: "filled",
        Implement: "filled",
        Review: "plain",
        "Customer Review": "plain",
        Closed: "plain",
      });
    });

    it("dashes the way into Rollback and Canceled while the change is on the path", () => {
      render(<ChangeRequestLifecycleStepper state="implement" />);
      expect(lines()).toMatchObject({ Rollback: "dashed", Canceled: "dashed" });
    });

    it("runs the filled line straight through the faint Rollback to Closed", () => {
      render(<ChangeRequestLifecycleStepper state="closed" />);
      expect(lines()).toMatchObject({ "Customer Review": "filled", Rollback: "filled", Closed: "filled", Canceled: "dashed" });
    });

    it("leads into the current Rollback in the error colour, then dashes what was not taken", () => {
      render(<ChangeRequestLifecycleStepper state="rollback" approvals={[{ stage: "Customer Review", status: "APPROVED" }]} />);
      expect(lines()).toMatchObject({ "Customer Review": "filled", Rollback: "error", Closed: "dashed", Canceled: "dashed" });
    });

    it("fills the line into a stage the customer rejected, and dashes the stages after it", () => {
      render(
        <ChangeRequestLifecycleStepper
          state="canceled"
          approvals={[{ stage: "Customer Approval", status: "REJECTED" }]}
        />,
      );
      expect(lines()).toMatchObject({
        Authorize: "filled",
        "Customer Approval": "filled",
        Scheduled: "dashed",
        "Customer Review": "dashed",
        Canceled: "error",
      });
    });

    it("leads into the current Canceled in the error colour", () => {
      render(<ChangeRequestLifecycleStepper state="canceled" approvals={[]} />);
      expect(lines()).toMatchObject({ Assess: "plain", "Customer Review": "plain", Rollback: "dashed", Closed: "dashed", Canceled: "error" });
    });
  });

  describe("captions", () => {
    it("gives every stage the customer portal's caption as its accessible description", () => {
      render(<ChangeRequestLifecycleStepper state="review" />);
      expect(stage("New")).toHaveAttribute("aria-description", "Change request created");
      expect(stage("Assess")).toHaveAttribute("aria-description", "Technical assessment completed");
      expect(stage("Authorize")).toHaveAttribute("aria-description", "Internal authorization obtained");
      expect(stage("Customer Approval")).toHaveAttribute("aria-description", "Customer approval received");
      expect(stage("Scheduled")).toHaveAttribute("aria-description", "Maintenance window scheduled");
      expect(stage("Implement")).toHaveAttribute("aria-description", "Change implementation");
      expect(stage("Review")).toHaveAttribute("aria-description", "Internal review");
      expect(stage("Customer Review")).toHaveAttribute("aria-description", "Customer validation");
      expect(stage("Rollback")).toHaveAttribute("aria-description", "Change rollback if needed");
      expect(stage("Closed")).toHaveAttribute("aria-description", "Change request completed");
      expect(stage("Canceled")).toHaveAttribute("aria-description", "Change request canceled");
    });

    it("shows the caption in a tooltip on hover", async () => {
      render(<ChangeRequestLifecycleStepper state="review" />);
      fireEvent.mouseOver(stage("Implement"));
      expect(await screen.findByRole("tooltip")).toHaveTextContent(/^Change implementation$/);
    });

    it("adds what a faint or crossed marker means to the tooltip, for the stages whose status the line does not explain", async () => {
      render(<ChangeRequestLifecycleStepper state="review" />);
      fireEvent.mouseOver(stage("Rollback"));
      expect(await screen.findByRole("tooltip")).toHaveTextContent(/^Change rollback if needed \(not taken\)$/);
      // The accessible description stays the plain caption.
      expect(stage("Rollback")).toHaveAttribute("aria-description", "Change rollback if needed");
    });

    it("says history not recorded in the tooltip of a stage a canceled change cannot vouch for", async () => {
      render(<ChangeRequestLifecycleStepper state="canceled" approvals={[]} />);
      fireEvent.mouseOver(stage("Review"));
      expect(await screen.findByRole("tooltip")).toHaveTextContent(/^Internal review \(history not recorded\)$/);
    });
  });
});
