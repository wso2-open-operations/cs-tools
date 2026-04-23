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
import { UpdateCardHeader } from "@update-cards/UpdateCardHeader";
import { UpdateProductCardHeaderStatus } from "@features/updates/types/updates";

describe("UpdateCardHeader", () => {
  it("renders product name and version", () => {
    render(
      <UpdateCardHeader
        productName="Test Product"
        productBaseVersion="1.2.3"
        percentage={75}
        statusColor={UpdateProductCardHeaderStatus.Info}
      />,
    );
    expect(screen.getByText("Test Product")).toBeDefined();
    expect(screen.getByText("Version 1.2.3")).toBeDefined();
  });

  it("renders percentage correctly", () => {
    render(
      <UpdateCardHeader
        productName="Test Product"
        productBaseVersion="1.2.3"
        percentage={75}
        statusColor={UpdateProductCardHeaderStatus.Info}
      />,
    );
    expect(screen.getByText("75% Updated")).toBeDefined();
  });

  it("rounds percentage value", () => {
    render(
      <UpdateCardHeader
        productName="Test Product"
        productBaseVersion="1.2.3"
        percentage={66.666}
        statusColor={UpdateProductCardHeaderStatus.Info}
      />,
    );
    expect(screen.getByText("67% Updated")).toBeDefined();
  });
});
