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
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import CaseStateConfirmDialog from "../CaseStateConfirmDialog";

describe("CaseStateConfirmDialog", () => {
  it("invokes confirm callback when confirm button is clicked", () => {
    const onClose = vi.fn();
    const onConfirm = vi.fn();

    render(
      <ThemeProvider theme={createTheme()}>
        <CaseStateConfirmDialog
          open
          actionLabel="Close"
          isPending={false}
          onClose={onClose}
          onConfirm={onConfirm}
        />
      </ThemeProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Confirm" }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });
});
