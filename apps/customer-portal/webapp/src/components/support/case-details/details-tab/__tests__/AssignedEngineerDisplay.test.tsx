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
import { describe, expect, it } from "vitest";
import AssignedEngineerDisplay from "@case-details-details/AssignedEngineerDisplay";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

function renderAssignedEngineer(assignedEngineer: string | null | undefined) {
  return render(
    <ThemeProvider theme={createTheme()}>
      <AssignedEngineerDisplay assignedEngineer={assignedEngineer} />
    </ThemeProvider>,
  );
}

describe("AssignedEngineerDisplay", () => {
  it("should render engineer name and initials in avatar", () => {
    renderAssignedEngineer("John Doe");
    expect(screen.getByText("John Doe")).toBeInTheDocument();
    expect(screen.getByText("JD")).toBeInTheDocument();
  });

  it("should render -- when assignedEngineer is null", () => {
    renderAssignedEngineer(null);
    const dashes = screen.getAllByText("--");
    expect(dashes.length).toBeGreaterThanOrEqual(1);
  });

  it("should render -- when assignedEngineer is undefined", () => {
    renderAssignedEngineer(undefined);
    const dashes = screen.getAllByText("--");
    expect(dashes.length).toBeGreaterThanOrEqual(1);
  });
});
