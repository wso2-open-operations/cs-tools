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
import UpdateLevelsReportModal from "@features/updates/components/all-updates/UpdateLevelsReportModal";

vi.mock("@utils/useDarkMode", () => ({ useDarkMode: () => false }));
vi.mock("@features/updates/utils/updateLevelsReportPdf", () => ({
  generateUpdateLevelsReportPdf: vi.fn(),
  isSafeHttpUrl: () => true,
  parseBugFixes: () => [],
}));

const reportData = {
  generatedStr: "May 01, 2026",
  productName: "wso2am",
  productVersion: "4.4.0",
  startLevel: 0,
  endLevel: 10,
  totalUpdates: 1,
  levelCount: 1,
  levelsRange: "0-10",
  securityCount: 1,
  regularCount: 0,
  mixedCount: 0,
  securityEntries: [],
  entriesWithInstructions: [],
  allEntries: [],
} as never;

const rawData = {
  7: {
    updateType: "security",
    updateDescriptionLevels: [
      {
        updateNumber: 700,
        description: "security fix",
        bugFixes: "[]",
        securityAdvisories: [],
        timestamp: 1710000000000,
      },
    ],
  },
} as never;

describe("UpdateLevelsReportModal", () => {
  it("does not render when report data is null", () => {
    const { container } = render(
      <UpdateLevelsReportModal open reportData={null} rawData={undefined} onClose={vi.fn()} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("renders and invokes level view callback", () => {
    const onClose = vi.fn();
    const onView = vi.fn();
    render(
      <UpdateLevelsReportModal
        open
        reportData={reportData}
        rawData={rawData}
        onClose={onClose}
        onView={onView}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /Level 7/i }));
    expect(onClose).toHaveBeenCalled();
    expect(onView).toHaveBeenCalledWith("7");
  });
});

