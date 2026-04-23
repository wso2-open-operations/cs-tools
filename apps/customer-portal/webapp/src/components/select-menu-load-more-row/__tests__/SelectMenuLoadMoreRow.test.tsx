// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
//
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { SelectMenuLoadMoreRow } from "@components/select-menu-load-more-row/SelectMenuLoadMoreRow";

function renderWithTheme(ui: ReactNode) {
  return render(<ThemeProvider theme={createTheme()}>{ui}</ThemeProvider>);
}

describe("SelectMenuLoadMoreRow", () => {
  it("renders nothing when not visible", () => {
    const { container } = renderWithTheme(<SelectMenuLoadMoreRow visible={false} />);
    expect(container.firstChild).toBeNull();
  });

  it("renders a circular progress indicator when visible", () => {
    renderWithTheme(<SelectMenuLoadMoreRow visible />);
    expect(screen.getByRole("progressbar")).toBeInTheDocument();
  });
});
