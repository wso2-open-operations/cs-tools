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
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { describe, expect, it, vi } from "vitest";
import CallRequestCard from "../CallRequestCard";

describe("CallRequestCard", () => {
  it("invokes approve callback for pending-on-customer state", () => {
    const onApproveClick = vi.fn();
    render(
      <ThemeProvider theme={createTheme()}>
        <CallRequestCard
          call={{ id: "1", state: { label: "Pending on Customer" }, preferredTimes: [] } as never}
          onApproveClick={onApproveClick}
        />
      </ThemeProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: /approve/i }));
    expect(onApproveClick).toHaveBeenCalledTimes(1);
  });
});
