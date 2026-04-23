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
import { describe, expect, it, vi, afterEach } from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { ClampedTextWithTooltip } from "@features/project-hub/components/project-card/ClampedTextWithTooltip";
import { ClampedTextVariant } from "@features/project-hub/types/projectHub";

vi.mock("@wso2/oxygen-ui", async () => {
  const actual = await vi.importActual<typeof import("@wso2/oxygen-ui")>(
    "@wso2/oxygen-ui",
  );
  return {
    ...actual,
    Tooltip: ({
      children,
      title,
    }: {
      children: ReactNode;
      title: string;
    }) => (
      <span data-testid="tooltip-host">
        {children}
        <span role="tooltip">{title}</span>
      </span>
    ),
  };
});

function renderWithTheme(ui: ReactNode) {
  return render(<ThemeProvider theme={createTheme()}>{ui}</ThemeProvider>);
}

describe("ClampedTextWithTooltip", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("does not render tooltip role when not truncated", () => {
    vi.spyOn(HTMLElement.prototype, "scrollHeight", "get").mockReturnValue(20);
    vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockReturnValue(20);

    renderWithTheme(
      <ClampedTextWithTooltip
        text="Short"
        lineClamp={2}
        variant={ClampedTextVariant.H6}
      />,
    );

    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
    expect(screen.getByText("Short")).toBeInTheDocument();
  });

  it("renders tooltip with full text when truncated (scrollHeight > clientHeight)", () => {
    vi.spyOn(HTMLElement.prototype, "scrollHeight", "get").mockReturnValue(120);
    vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockReturnValue(40);

    const full =
      "Very Long Project Title That Should Trigger Truncation And Tooltip";

    renderWithTheme(
      <ClampedTextWithTooltip
        text={full}
        lineClamp={1}
        variant={ClampedTextVariant.H6}
      />,
    );

    expect(screen.getByRole("tooltip")).toHaveTextContent(full);
  });
});
