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
import { UpdateCardLevels } from "@update-cards/UpdateCardLevels";

describe("UpdateCardLevels", () => {
  it("renders all level information", () => {
    render(
      <UpdateCardLevels
        currentUpdateLevel={10}
        recommendedUpdateLevel={15}
        pendingLevels={5}
      />,
    );
    expect(screen.getByText("Current Level")).toBeDefined();
    expect(screen.getByText("U10")).toBeDefined();
    expect(screen.getByText("Latest Level")).toBeDefined();
    expect(screen.getByText("U15")).toBeDefined();
    expect(screen.getByText("Pending Levels")).toBeDefined();
    expect(screen.getByText("5")).toBeDefined();
  });
});
