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

import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";
import ChangeRequestLifecycleStepper from "@features/csm-operations/components/ChangeRequestLifecycleStepper";

describe("ChangeRequestLifecycleStepper", () => {
  it("renders all 9 forward states as list items", () => {
    render(<ChangeRequestLifecycleStepper state="new" />);
    expect(screen.getAllByRole("listitem")).toHaveLength(9);
  });

  it("marks the CR's current state with aria-current='step'", () => {
    render(<ChangeRequestLifecycleStepper state="implement" />);
    const current = screen.getByText("Implement").closest('[aria-current="step"]');
    expect(current).not.toBeNull();
  });

  it("marks every state before the current one as complete, and none after", () => {
    render(<ChangeRequestLifecycleStepper state="implement" />);
    const list = screen.getByRole("list", { name: /change request lifecycle/i });

    // Prior states (New, Assess, Authorize, Customer Approval, Scheduled) each
    // render a check icon (svg) inside their step marker.
    ["New", "Assess", "Authorize", "Customer Approval", "Scheduled"].forEach((label) => {
      const item = within(list).getByText(label).closest('[role="listitem"]')!;
      expect((item as HTMLElement).querySelector("svg")).not.toBeNull();
    });

    // The current step itself carries aria-current, states after it don't
    // and have no check icon.
    const review = within(list).getByText("Review").closest('[role="listitem"]')!;
    expect(review).not.toHaveAttribute("aria-current");
    expect((review as HTMLElement).querySelector("svg")).toBeNull();
  });

  it("does not plot rollback/canceled on the forward line", () => {
    render(<ChangeRequestLifecycleStepper state="implement" />);
    expect(screen.queryByText("Rollback")).not.toBeInTheDocument();
    expect(screen.queryByText("Canceled")).not.toBeInTheDocument();
  });

  it("shows a separate off-ramp note instead of a forward-path highlight when canceled", () => {
    render(<ChangeRequestLifecycleStepper state="canceled" />);
    expect(screen.getByText(/diverted from the standard path/i)).toBeInTheDocument();
    // The label appears in the off-ramp note itself.
    expect(screen.getByText("Canceled")).toBeInTheDocument();
    // Nothing on the forward line claims to be the current step.
    expect(document.querySelector('[aria-current="step"]')).toBeNull();
  });

  it("shows the off-ramp note for rollback too", () => {
    render(<ChangeRequestLifecycleStepper state="rollback" />);
    expect(screen.getByText(/diverted from the standard path/i)).toBeInTheDocument();
    expect(screen.getByText("Rollback")).toBeInTheDocument();
  });

  it("renders no off-ramp note for a normal forward state", () => {
    render(<ChangeRequestLifecycleStepper state="scheduled" />);
    expect(screen.queryByText(/diverted from the standard path/i)).not.toBeInTheDocument();
  });

  it("shows a current-state note for a state that is neither a forward state nor a recognized off-ramp", () => {
    render(<ChangeRequestLifecycleStepper state="some_future_state" />);
    expect(screen.getByText(/current state:/i)).toBeInTheDocument();
    expect(screen.getByText("some future state")).toBeInTheDocument();
    // No forward-line marker claims to be current, and none render complete.
    expect(document.querySelector('[aria-current="step"]')).toBeNull();
    expect(screen.queryByText(/diverted from the standard path/i)).not.toBeInTheDocument();
  });

  it("shows no current-state note when the CR has no state yet", () => {
    render(<ChangeRequestLifecycleStepper state={null} />);
    expect(screen.queryByText(/current state:/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/diverted from the standard path/i)).not.toBeInTheDocument();
  });
});
