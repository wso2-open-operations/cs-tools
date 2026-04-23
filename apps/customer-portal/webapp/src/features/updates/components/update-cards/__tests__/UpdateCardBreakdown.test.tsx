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
import { describe, it, expect } from "vitest";
import { UpdateCardBreakdown } from "@update-cards/UpdateCardBreakdown";

describe("UpdateCardBreakdown", () => {
  it("renders installed and pending breakdowns correctly", () => {
    render(
      <UpdateCardBreakdown
        installedRegular={5}
        installedSecurity={2}
        pendingRegular={3}
        pendingSecurity={2}
        totalPending={5}
      />,
    );

    // Check Installed section
    expect(screen.getByText("Installed")).toBeDefined();
    expect(screen.getByText("7")).toBeDefined();
    expect(screen.getByText("5R • 2S")).toBeDefined();

    // Check Pending section
    expect(screen.getByText("Pending")).toBeDefined();
    const pendingTotal = screen
      .getAllByText("5")
      .find((el) => el.tagName === "P" || el.tagName === "SPAN");
    expect(pendingTotal).toBeDefined();
    expect(screen.getByText("3R • 2S")).toBeDefined();
  });
});
