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
import "@testing-library/jest-dom/vitest";
import AssigneeName from "@components/AssigneeName";

describe("AssigneeName", () => {
  it("shows the name without the circled W, and the support engineer icon", () => {
    render(<AssigneeName name={"Jane Doe ⓦ"} />);
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Support engineer" })).toBeInTheDocument();
  });

  it("shows the fallback, with no icon, when there is no assignee", () => {
    render(<AssigneeName name={undefined} fallback="Unassigned" />);
    expect(screen.getByText("Unassigned")).toBeInTheDocument();
    expect(screen.queryByRole("img", { name: "Support engineer" })).not.toBeInTheDocument();
  });

  it("defaults the fallback to a dash", () => {
    render(<AssigneeName name={null} />);
    expect(screen.getByText("—")).toBeInTheDocument();
  });
});
