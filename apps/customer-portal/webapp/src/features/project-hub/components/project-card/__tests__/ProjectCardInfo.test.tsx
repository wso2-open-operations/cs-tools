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
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi, afterEach } from "vitest";
import type { ReactNode } from "react";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import ProjectCardInfo from "@features/project-hub/components/project-card/ProjectCardInfo";

vi.mock("@wso2/oxygen-ui", async () => {
  const actual = await vi.importActual<typeof import("@wso2/oxygen-ui")>(
    "@wso2/oxygen-ui",
  );
  return {
    ...actual,
    Form: {
      ...actual.Form,
      CardHeader: ({ title }: { title: ReactNode }) => (
        <div data-testid="card-header">
          <div data-testid="title">{title}</div>
        </div>
      ),
    },
    Tooltip: ({
      children,
      title,
    }: {
      children: React.ReactNode;
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

describe("ProjectCardInfo", () => {
  it("renders project title", () => {
    renderWithTheme(<ProjectCardInfo title="My Project" />);
    expect(screen.getByText("My Project")).toBeInTheDocument();
  });

  it("uses h6 typography for title", () => {
    renderWithTheme(<ProjectCardInfo title="Title" />);
    expect(screen.getByText("Title").closest(".MuiTypography-h6")).toBeTruthy();
  });

  it("displays '--' fallback for empty title", () => {
    renderWithTheme(<ProjectCardInfo title="" />);
    expect(screen.getByText("--")).toBeInTheDocument();
  });
});

describe("ProjectCardInfo truncation tooltip (ClampedTextWithTooltip path)", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("exposes tooltip with full title when line-clamped content overflows", () => {
    vi.spyOn(HTMLElement.prototype, "scrollHeight", "get").mockReturnValue(200);
    vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockReturnValue(24);

    const full =
      "Extremely Long Hub Card Title For Tooltip Regression Coverage Text";

    renderWithTheme(<ProjectCardInfo title={full} />);

    expect(screen.getByRole("tooltip")).toHaveTextContent(full);
  });
});
