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
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { BeDashboardWidget } from "@api/backend/types";
import type { WallboardStatGridProps } from "@features/csm-dashboard/components/WallboardStatGrid";
import type { WallboardSecondaryStatProps } from "@features/csm-dashboard/components/WallboardSecondaryStat";

vi.mock("@features/csm-dashboard/components/WallboardStatGrid", () => ({
  default: ({ widgets, section }: WallboardStatGridProps) => (
    <div data-testid="primary-grid" data-section={section}>
      {widgets.map((w) => w.displayName).join(",")}
    </div>
  ),
}));
vi.mock("@features/csm-dashboard/components/WallboardSecondaryStat", () => ({
  default: ({ displayName }: WallboardSecondaryStatProps) => <div data-testid="secondary-tile">{displayName}</div>,
}));

import WallboardCreSection from "@features/csm-dashboard/components/WallboardCreSection";

function widget(id: string, displayName: string): BeDashboardWidget {
  return { widgetId: id, displayName, resourceType: "case", shape: "count", gridWidth: 4, query: {} };
}

describe("WallboardCreSection", () => {
  it("splits widgets into the primary 2x2 grid (fixed CRE_PRIMARY_ORDER) and everything else as secondary tiles", () => {
    render(
      <WallboardCreSection
        widgets={[
          // Deliberately out of CRE_PRIMARY_ORDER's own order, and with
          // secondary widgets interleaved, to prove the split is by name
          // membership, not array position.
          widget("w1", "Escalations"),
          widget("w2", "At WSO2 Incidents"),
          widget("w3", "Open"),
          widget("w4", "Waiting on WSO2"),
          widget("w5", "SLA Violations"),
          widget("w6", "In-Progress"),
        ]}
      />,
    );

    // Primary grid gets exactly the 4 CRE_PRIMARY_ORDER names, in that
    // fixed order — not the order they appeared in `widgets`.
    expect(screen.getByTestId("primary-grid")).toHaveTextContent("Open,In-Progress,SLA Violations,Escalations");
    expect(screen.getByTestId("primary-grid")).toHaveAttribute("data-section", "cre");

    // Everything else renders as a plain secondary tile.
    const secondaryTiles = screen.getAllByTestId("secondary-tile").map((el) => el.textContent);
    expect(secondaryTiles).toEqual(["At WSO2 Incidents", "Waiting on WSO2"]);
  });

  it("omits a CRE_PRIMARY_ORDER slot entirely when the dashboard doesn't configure that widget, rather than crashing", () => {
    render(<WallboardCreSection widgets={[widget("w1", "Open"), widget("w2", "Escalations")]} />);
    expect(screen.getByTestId("primary-grid")).toHaveTextContent("Open,Escalations");
  });

  it("renders the secondary tier in CRE_SECONDARY_ORDER's own fixed order, regardless of input array order", () => {
    render(
      <WallboardCreSection
        widgets={[
          // "Awaiting Info" listed FIRST in the input — must still render
          // LAST, matching the original's own card positions (and "Waiting
          // on WSO2" must render well before it, not after).
          widget("w1", "Awaiting Info"),
          widget("w2", "Migration"),
          widget("w3", "Waiting on WSO2"),
          widget("w4", "At WSO2 Incidents"),
        ]}
      />,
    );
    const secondaryTiles = screen.getAllByTestId("secondary-tile").map((el) => el.textContent);
    expect(secondaryTiles).toEqual(["At WSO2 Incidents", "Waiting on WSO2", "Migration", "Awaiting Info"]);
  });
});
