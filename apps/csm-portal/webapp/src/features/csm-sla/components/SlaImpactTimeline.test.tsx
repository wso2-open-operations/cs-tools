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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { resetFeatureStatesForTests } from "@config/featureFlags";
import { writePreviewOverride } from "@config/featurePreview";
import { SlaImpactTimeline } from "@features/csm-sla/components/SlaImpactTimeline";
import type { CaseAuditEntry } from "@features/csm-cases/types/csmCases";

const session = vi.hoisted(() => ({ roles: ["cs_engineer"] as string[] }));

vi.mock("@context/current-user/usePortalAccess", async () => {
  const { getPortalAccess } = await import("@context/current-user/portalAccess");
  return {
    usePortalAccess: () => getPortalAccess(session.roles),
    useSafeCurrentUserId: () => "user-1",
  };
});

const ENTRIES: CaseAuditEntry[] = [
  {
    id: "a1",
    kind: "field_change",
    actor: "Jane Doe",
    createdAt: "2026-07-01T10:00:00Z",
    changes: [
      { field: "state", fieldLabel: "State", previousValue: "Work In Progress", newValue: "Awaiting Info" },
    ],
  },
  {
    id: "a2",
    kind: "field_change",
    actor: "Jane Doe",
    createdAt: "2026-07-02T10:00:00Z",
    changes: [{ field: "workaround_provided_on", fieldLabel: "Workaround Provided", newValue: "2026-07-02T10:00:00Z" }],
  },
];

function renderTimeline(overrides: Partial<Parameters<typeof SlaImpactTimeline>[0]> = {}) {
  return render(
    <SlaImpactTimeline entries={ENTRIES} isLoading={false} isError={false} onRetry={vi.fn()} {...overrides} />,
  );
}

function setOverrides(value: unknown): void {
  window.config = { CSM_PORTAL_FEATURE_OVERRIDES: value } as unknown as typeof window.config;
  resetFeatureStatesForTests();
}

beforeEach(() => {
  session.roles = ["cs_engineer"];
  localStorage.clear();
});

afterEach(() => {
  setOverrides(undefined);
  localStorage.clear();
});

describe("SlaImpactTimeline", () => {
  it("renders one focusable marker per event, named with title, values, actor and time", () => {
    renderTimeline();
    const markers = screen.getAllByRole("button").filter((b) => b.getAttribute("aria-label"));
    expect(markers).toHaveLength(2);
    const state = screen.getByRole("button", { name: /Waiting on customer/ });
    expect(state).toHaveAccessibleName(/State: Work in progress to Awaiting info/);
    expect(state).toHaveAccessibleName(/Jane Doe/);
    expect(state).toHaveAccessibleName(/2026/);
    expect(screen.getByRole("button", { name: /Workaround provided/ })).toBeInTheDocument();
    state.focus();
    expect(state).toHaveFocus();
  });

  it("shows a legend for the three track colours", () => {
    renderTimeline();
    for (const label of ["WSO2 side", "Customer side", "Closed"]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it("reveals the details in a tooltip on hover", async () => {
    renderTimeline();
    fireEvent.mouseOver(screen.getByRole("button", { name: /Waiting on customer/ }));
    expect(await screen.findByRole("tooltip")).toHaveTextContent("Customer side");
    expect(screen.getByRole("tooltip")).toHaveTextContent("Awaiting info");
  });

  it("shows a loading state", () => {
    renderTimeline({ isLoading: true, entries: undefined });
    expect(screen.getByText("SLA impact timeline")).toBeInTheDocument();
    expect(screen.queryByRole("group")).not.toBeInTheDocument();
  });

  it("shows an empty state when nothing SLA-relevant was recorded", () => {
    renderTimeline({ entries: [] });
    expect(screen.getByText(/No SLA-affecting changes/)).toBeInTheDocument();
  });

  it("shows an error state with a working retry", () => {
    const onRetry = vi.fn();
    renderTimeline({ isError: true, onRetry });
    expect(screen.getByText(/Could not load the SLA impact timeline/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("is shown by default and when the override says enabled", () => {
    const { unmount } = renderTimeline();
    expect(screen.getByText("SLA impact timeline")).toBeInTheDocument();
    unmount();
    setOverrides({ "support.case-sla-timeline": "enabled" });
    renderTimeline();
    expect(screen.getByText("SLA impact timeline")).toBeInTheDocument();
  });

  it("renders nothing when the feature is overridden to hidden", () => {
    setOverrides({ "support.case-sla-timeline": "hidden" });
    const { container } = renderTimeline();
    expect(container).toBeEmptyDOMElement();
  });

  it("also renders nothing for wip, and ignores unrelated overrides", () => {
    setOverrides({ "support.case-sla-timeline": "wip" });
    const { container, unmount } = renderTimeline();
    expect(container).toBeEmptyDOMElement();
    unmount();
    setOverrides({ operations: "hidden" });
    renderTimeline();
    expect(screen.getByText("SLA impact timeline")).toBeInTheDocument();
  });

  describe("admin preview of a restricted timeline", () => {
    const FEATURE = "support.case-sla-timeline";

    it("stays hidden for a non-admin even with a stored preview value", () => {
      setOverrides({ [FEATURE]: "hidden" });
      writePreviewOverride("user-1", FEATURE, true);
      const { container } = renderTimeline();
      expect(container).toBeEmptyDOMElement();
    });

    it("is shown to an admin with the preview on, marked as a preview", () => {
      session.roles = ["admin"];
      setOverrides({ [FEATURE]: "hidden" });
      writePreviewOverride("user-1", FEATURE, true);
      renderTimeline();
      expect(screen.getByText("SLA impact timeline")).toBeInTheDocument();
      expect(screen.getByText("Preview")).toBeInTheDocument();
    });

    it("stays hidden for an admin with the preview off, and for wip too", () => {
      session.roles = ["admin"];
      setOverrides({ [FEATURE]: "wip" });
      const { container } = renderTimeline();
      expect(container).toBeEmptyDOMElement();
    });

    it("shows no Preview marker when the deployment does not restrict it", () => {
      session.roles = ["admin"];
      writePreviewOverride("user-1", FEATURE, true);
      renderTimeline();
      expect(screen.queryByText("Preview")).not.toBeInTheDocument();
    });
  });

  describe("time on each side", () => {
    it("shows calendar time and share per side, and says it is calendar time", () => {
      renderTimeline({ openedAt: "2026-06-29T10:00:00Z" });
      expect(screen.getByText(/WSO2 side:/)).toHaveTextContent("WSO2 side: 2d 0h");
      expect(screen.getByText(/Customer side:/)).toBeInTheDocument();
      expect(screen.getByText(/Open period:/)).toBeInTheDocument();
      expect(
        screen.getByText(/Calendar time; the SLA table above uses business time\./),
      ).toBeInTheDocument();
      expect(screen.getByRole("img", { name: /Share of the open period/ })).toBeInTheDocument();
    });

    it("says there is not enough history instead of inventing numbers", () => {
      renderTimeline({ entries: [ENTRIES[1]] });
      expect(screen.getByText(/Not enough history/)).toBeInTheDocument();
      expect(screen.queryByText(/Open period:/)).not.toBeInTheDocument();
    });

    it("uses the current state when no state change was audited", () => {
      renderTimeline({ entries: [ENTRIES[1]], currentState: "awaiting_info" });
      expect(screen.queryByText(/Not enough history/)).not.toBeInTheDocument();
      expect(screen.getByText(/Customer side:/)).toBeInTheDocument();
    });
  });
});
