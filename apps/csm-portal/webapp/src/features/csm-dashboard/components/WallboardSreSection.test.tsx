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
import type { BeDashboardWidget, BeWidgetResourceType } from "@api/backend/types";
import type { WallboardStatTileProps } from "@features/csm-dashboard/components/WallboardStatTile";

vi.mock("@features/csm-dashboard/components/WallboardStatTile", () => ({
  default: ({ widgetId, displayName, section, variant }: WallboardStatTileProps) => (
    <div data-testid="stat-tile" data-section={section} data-variant={variant}>
      {widgetId}:{displayName}
    </div>
  ),
}));

import WallboardSreSection from "@features/csm-dashboard/components/WallboardSreSection";

function widget(id: string, displayName: string, resourceType: BeWidgetResourceType): BeDashboardWidget {
  return { widgetId: id, displayName, resourceType, shape: "count", gridWidth: 4, query: {} };
}

describe("WallboardSreSection", () => {
  it("groups widgets into their sub-row by resourceType, each sub-row with its own heading, in SRE_SUBSECTION_ORDER", () => {
    render(
      <WallboardSreSection
        widgets={[
          // Deliberately out of sub-row order in the input array.
          widget("w1", "Open SR", "case"),
          widget("w2", "New CR", "change_request"),
          widget("w3", "Open", "incident"),
          widget("w4", "Open Problems", "problem"),
        ]}
      />,
    );

    // Exact match, not a substring search — "Open Problems" (a tile's own
    // text) must not be picked up by a loose match on "Problems".
    for (const heading of ["Incidents", "Problems", "Change Requests", "Service Requests"]) {
      expect(screen.getByText(heading, { exact: true })).toBeInTheDocument();
    }

    const tiles = screen.getAllByTestId("stat-tile");
    expect(tiles.every((t) => t.getAttribute("data-section") === "sre" && t.getAttribute("data-variant") === "sre")).toBe(
      true,
    );
    expect(tiles.map((t) => t.textContent)).toEqual(["w3:Open", "w4:Open Problems", "w2:New CR", "w1:Open SR"]);
  });

  it("renders Problems in the original's fixed order (In-Progress Problems before Open Problems), regardless of input array order", () => {
    render(
      <WallboardSreSection
        widgets={[
          // "Open Problems" listed FIRST in the input — must still render
          // second, matching the original's own card positions.
          widget("w1", "Open Problems", "problem"),
          widget("w2", "In-Progress Problems", "problem"),
        ]}
      />,
    );
    const tiles = screen.getAllByTestId("stat-tile");
    expect(tiles.map((t) => t.textContent)).toEqual(["w2:In-Progress Problems", "w1:Open Problems"]);
  });

  it("keeps a widget with an unrecognized displayName in its original relative position, appended after every named one", () => {
    render(
      <WallboardSreSection
        widgets={[
          widget("w1", "Some New CR Metric", "change_request"),
          widget("w2", "New CR", "change_request"),
        ]}
      />,
    );
    const tiles = screen.getAllByTestId("stat-tile");
    expect(tiles.map((t) => t.textContent)).toEqual(["w2:New CR", "w1:Some New CR Metric"]);
  });

  it("skips a sub-row entirely when no widget maps to it, rather than rendering an empty heading", () => {
    render(<WallboardSreSection widgets={[widget("w1", "Open", "incident")]} />);
    expect(screen.getByText("Incidents")).toBeInTheDocument();
    expect(screen.queryByText("Problems")).not.toBeInTheDocument();
    expect(screen.queryByText("Change Requests")).not.toBeInTheDocument();
    expect(screen.queryByText("Service Requests")).not.toBeInTheDocument();
  });
});
