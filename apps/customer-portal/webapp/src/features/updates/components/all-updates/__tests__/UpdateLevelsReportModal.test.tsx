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
import { describe, it, expect, vi } from "vitest";
import UpdateLevelsReportModal from "@features/updates/components/all-updates/UpdateLevelsReportModal";
import type { UpdateLevelsReportData } from "@features/updates/types/updates";

const mockReportData: UpdateLevelsReportData = {
  generatedStr: "February 23, 2026 at 8:00 PM",
  productName: "WSO2 API Manager",
  productVersion: "4.4.0",
  startLevel: 2,
  endLevel: 19,
  securityCount: 14,
  regularCount: 4,
  mixedCount: 0,
  totalUpdates: 39,
  levelCount: 18,
  levelsRange: "2 - 19",
  tableRows: [
    { levelKey: "2", typeLabel: "Regular", updatesCount: 2, releaseDate: "Jan 22, 2024", applied: "Yes" },
    { levelKey: "3", typeLabel: "Security", updatesCount: 1, releaseDate: "Feb 5, 2024", applied: "Yes" },
  ],
};

describe("UpdateLevelsReportModal", () => {
  it("renders report content when open with reportData", () => {
    render(
      <UpdateLevelsReportModal
        open={true}
        reportData={mockReportData}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("Update Levels Report")).toBeDefined();
    expect(screen.getByText(/Generated on:/)).toBeDefined();
    expect(screen.getByText(/Update Level Range: 2 to 19/)).toBeDefined();
    expect(screen.getByRole("button", { name: /Download PDF/i })).toBeDefined();
    expect(screen.getAllByRole("button", { name: /Close/i }).length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("Search Criteria")).toBeDefined();
    expect(screen.getByText("Update Levels Details")).toBeDefined();
    expect(screen.getByText("Level")).toBeDefined();
    expect(screen.getByText("Type")).toBeDefined();
  });

  it("renders nothing visible when reportData is null", () => {
    render(
      <UpdateLevelsReportModal
        open={true}
        reportData={null}
        onClose={vi.fn()}
      />,
    );

    expect(screen.queryByText("Update Levels Report")).toBeNull();
  });
});
