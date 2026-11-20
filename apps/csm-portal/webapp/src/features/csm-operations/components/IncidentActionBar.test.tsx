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
import IncidentActionBar from "@features/csm-operations/components/IncidentActionBar";
import { getLegalNextIncidentStates } from "@features/csm-operations/utils/incidents";
import type { BeIncidentDetail, BeIncidentState } from "@api/backend/types";

// Only `getLegalNextIncidentStates` is overridden (per-test, via
// `mockReturnValueOnce` below) so the "single-target" branch can be tested
// independently of the real transition table.
// Everything else (icons, labels) uses the real module. Statically mocking
// like this, instead of `vi.resetModules()` + a dynamic `import()` of the
// component under test, avoids the risk of that reset also evicting React
// itself from the module cache and re-evaluating a second copy.
vi.mock("@features/csm-operations/utils/incidents", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@features/csm-operations/utils/incidents")>();
  return {
    ...actual,
    getLegalNextIncidentStates: vi.fn(actual.getLegalNextIncidentStates),
  };
});

function incidentInState(state: BeIncidentState): BeIncidentDetail {
  return {
    id: "inc-1",
    number: "INC0012345",
    openedOn: "2026-01-01T00:00:00Z",
    subject: "Gateway 502s",
    priority: null,
    state,
    category: null,
  };
}

describe("IncidentActionBar — button set per state", () => {
  it("NEW offers In Progress and Cancelled via a Change-state menu", () => {
    render(
      <IncidentActionBar incident={incidentInState("NEW")} isPending={false} onAction={() => {}} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /change state/i }));
    expect(screen.getByRole("menuitem", { name: /in progress/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /cancelled/i })).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: /^new$/i })).not.toBeInTheDocument();
  });

  it("IN_PROGRESS offers On Hold, Resolved, Cancelled via a Change-state menu", () => {
    render(
      <IncidentActionBar
        incident={incidentInState("IN_PROGRESS")}
        isPending={false}
        onAction={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /change state/i }));
    expect(screen.getByRole("menuitem", { name: /on hold/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /resolved/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /cancelled/i })).toBeInTheDocument();
  });

  it("ON_HOLD offers In Progress and Cancelled via a Change-state menu", () => {
    render(
      <IncidentActionBar
        incident={incidentInState("ON_HOLD")}
        isPending={false}
        onAction={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /change state/i }));
    expect(screen.getByRole("menuitem", { name: /in progress/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /cancelled/i })).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: /resolved/i })).not.toBeInTheDocument();
  });

  it("RESOLVED offers a single button: Closed (reopen is via the Edit dialog only)", () => {
    render(
      <IncidentActionBar
        incident={incidentInState("RESOLVED")}
        isPending={false}
        onAction={() => {}}
      />,
    );
    // Two legal targets (Closed, In Progress) -> Change-state menu.
    fireEvent.click(screen.getByRole("button", { name: /change state/i }));
    expect(screen.getByRole("menuitem", { name: /^closed$/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /in progress/i })).toBeInTheDocument();
  });

  it("CLOSED renders nothing — terminal state", () => {
    const { container } = render(
      <IncidentActionBar
        incident={incidentInState("CLOSED")}
        isPending={false}
        onAction={() => {}}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("CANCELLED renders nothing — terminal state", () => {
    const { container } = render(
      <IncidentActionBar
        incident={incidentInState("CANCELLED")}
        isPending={false}
        onAction={() => {}}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });
});

describe("IncidentActionBar — dispatch", () => {
  it("calls onAction with the target BeIncidentState when a menu item is clicked", () => {
    const onAction = vi.fn();
    render(
      <IncidentActionBar incident={incidentInState("IN_PROGRESS")} isPending={false} onAction={onAction} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /change state/i }));
    fireEvent.click(screen.getByRole("menuitem", { name: /cancelled/i }));
    expect(onAction).toHaveBeenCalledWith("CANCELLED");
  });

  it("disables the primary/change-state button while a transition is pending", () => {
    render(
      <IncidentActionBar incident={incidentInState("IN_PROGRESS")} isPending={true} onAction={() => {}} />,
    );
    expect(screen.getByRole("button", { name: /change state/i })).toBeDisabled();
  });
});

describe("IncidentActionBar — single-target rendering (no menu needed)", () => {
  // Exercised through the hoisted mock so this stays independent of the real
  // transition table.
  it("renders one contained button (no 'Change state' menu) and dispatches on click", () => {
    vi.mocked(getLegalNextIncidentStates).mockReturnValueOnce(["NEW", "IN_PROGRESS"]);
    const onAction = vi.fn();
    render(
      <IncidentActionBar incident={incidentInState("NEW")} isPending={false} onAction={onAction} />,
    );
    expect(screen.queryByRole("button", { name: /change state/i })).not.toBeInTheDocument();
    const button = screen.getByRole("button", { name: /in progress/i });
    fireEvent.click(button);
    expect(onAction).toHaveBeenCalledWith("IN_PROGRESS");
  });
});
