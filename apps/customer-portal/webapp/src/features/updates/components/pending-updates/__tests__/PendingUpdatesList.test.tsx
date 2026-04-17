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

import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { PendingUpdatesList } from "@features/updates/components/pending-updates/PendingUpdatesList";
import {
  PENDING_UPDATES_LIST_EMPTY_DESCRIPTION,
  PENDING_UPDATES_LIST_ERROR_MESSAGE,
} from "@features/updates/constants/updatesConstants";
import type { UpdateLevelsSearchResponse } from "@features/updates/types/updates";

const mockData: UpdateLevelsSearchResponse = {
  "7": {
    updateType: "regular",
    updateDescriptionLevels: [
      {
        updateLevel: 7,
        productName: "wso2am",
        productVersion: "4.2.0",
        channel: "full",
        updateType: "regular",
        updateNumber: 912,
        description: "Fix for dashboard issue.",
        instructions: "N/A",
        bugFixes: "[]",
        filesAdded: "[]",
        filesModified: "[]",
        filesRemoved: "[]",
        bundlesInfoChanges: null,
        dependantReleases: null,
        timestamp: 1618554310000,
        securityAdvisories: [],
      },
    ],
  },
  "8": {
    updateType: "security",
    updateDescriptionLevels: [
      {
        updateLevel: 8,
        productName: "wso2am",
        productVersion: "4.2.0",
        channel: "full",
        updateType: "security",
        updateNumber: 973,
        description: "Security patch for HSTS headers.",
        instructions: "N/A",
        bugFixes: "[]",
        filesAdded: "[]",
        filesModified: "[]",
        filesRemoved: "[]",
        bundlesInfoChanges: null,
        dependantReleases: null,
        timestamp: 1619148655000,
        securityAdvisories: [
          {
            id: "WSO2-2021-1402",
            overview: "HSTS header enhancement.",
            severity: "Low",
            description: "Missing HSTS header.",
            impact: "Info disclosure.",
            solution: "Apply patch.",
            notes: "Tested.",
            credits: "-",
          },
        ],
      },
    ],
  },
};

describe("PendingUpdatesList", () => {
  it("renders empty state when data is null", () => {
    render(<PendingUpdatesList data={null} isError={false} onView={vi.fn()} />);
    expect(
      screen.getByText(PENDING_UPDATES_LIST_EMPTY_DESCRIPTION),
    ).toBeDefined();
  });

  it("renders empty state when data is empty object", () => {
    render(<PendingUpdatesList data={{}} isError={false} onView={vi.fn()} />);
    expect(
      screen.getByText(PENDING_UPDATES_LIST_EMPTY_DESCRIPTION),
    ).toBeDefined();
  });

  it("renders error state when isError is true", () => {
    render(<PendingUpdatesList data={null} isError={true} onView={vi.fn()} />);
    expect(screen.getByText(PENDING_UPDATES_LIST_ERROR_MESSAGE)).toBeDefined();
  });

  it("renders summary text with correct counts", () => {
    render(<PendingUpdatesList data={mockData} isError={false} onView={vi.fn()} />);
    expect(screen.getByText(/updates with/)).toBeDefined();
    expect(screen.getByText(/security/)).toBeDefined();
    expect(screen.getByText(/regular/)).toBeDefined();
    expect(screen.getByText(/updates\./)).toBeDefined();
  });

  it("renders table header columns", () => {
    render(<PendingUpdatesList data={mockData} isError={false} onView={vi.fn()} />);
    expect(screen.getByText("Update Level")).toBeDefined();
    expect(screen.getByText("Update Type")).toBeDefined();
    expect(screen.getByText("Details")).toBeDefined();
  });

  it("renders each update level row with label and chip", () => {
    render(<PendingUpdatesList data={mockData} isError={false} onView={vi.fn()} />);
    expect(screen.getByText("7")).toBeDefined();
    expect(screen.getByText("8")).toBeDefined();
    expect(screen.getByText("Regular")).toBeDefined();
    expect(screen.getByText("Security")).toBeDefined();
    expect(screen.getAllByText("View").length).toBe(2);
  });

  it("invokes onView with the correct levelKey when View button is clicked", () => {
    const onView = vi.fn();
    render(<PendingUpdatesList data={mockData} isError={false} onView={onView} />);
    const viewButtons = screen.getAllByText("View");
    expect(viewButtons).toHaveLength(2);
    fireEvent.click(viewButtons[0]);
    expect(onView).toHaveBeenCalledWith("7");
    fireEvent.click(viewButtons[1]);
    expect(onView).toHaveBeenCalledWith("8");
  });
});
