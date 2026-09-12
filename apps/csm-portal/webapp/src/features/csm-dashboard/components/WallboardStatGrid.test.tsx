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
import type { WallboardStatTileProps } from "@features/csm-dashboard/components/WallboardStatTile";

vi.mock("@features/csm-dashboard/components/WallboardStatTile", () => ({
  default: ({ widgetId, displayName, section, variant }: WallboardStatTileProps) => (
    <div data-testid="stat-tile" data-section={section} data-variant={variant ?? "primary"}>
      {widgetId}:{displayName}
    </div>
  ),
}));

import WallboardStatGrid from "@features/csm-dashboard/components/WallboardStatGrid";

function widget(id: string, displayName: string): BeDashboardWidget {
  return { widgetId: id, displayName, resourceType: "incident", shape: "count", gridWidth: 4, query: {} };
}

describe("WallboardStatGrid", () => {
  it("renders one tile per widget, forwarding section and variant to each", () => {
    render(
      <WallboardStatGrid
        widgets={[widget("a", "Open"), widget("b", "SLA Violations")]}
        section="security"
        columns={2}
        variant="sre"
      />,
    );

    const tiles = screen.getAllByTestId("stat-tile");
    expect(tiles).toHaveLength(2);
    expect(tiles[0]).toHaveTextContent("a:Open");
    expect(tiles[0]).toHaveAttribute("data-section", "security");
    expect(tiles[0]).toHaveAttribute("data-variant", "sre");
    expect(tiles[1]).toHaveTextContent("b:SLA Violations");
  });

  it("defaults to variant 'primary' when none is passed", () => {
    render(<WallboardStatGrid widgets={[widget("a", "Open")]} section="cre" columns={2} />);
    expect(screen.getByTestId("stat-tile")).toHaveAttribute("data-variant", "primary");
  });
});
