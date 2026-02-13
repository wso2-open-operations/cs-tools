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
import CaseDetailsSkeleton, {
  CaseDetailsHeaderSkeleton,
} from "@case-details-details/CaseDetailsSkeleton";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

describe("CaseDetailsSkeleton", () => {
  it("should render header and action row skeletons only (no sub nav tab skeletons)", () => {
    const { container } = render(
      <ThemeProvider theme={createTheme()}>
        <CaseDetailsSkeleton />
      </ThemeProvider>,
    );

    const skeletons = container.querySelectorAll(".MuiSkeleton-root");
    expect(skeletons.length).toBeGreaterThan(5);

    const dividers = container.querySelectorAll(".MuiDivider-root");
    expect(dividers.length).toBeGreaterThan(0);

    expect(container.querySelectorAll(".MuiCard-root").length).toBe(0);
  });

  it("should render Support Engineer label", () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <CaseDetailsSkeleton />
      </ThemeProvider>,
    );

    expect(screen.getByText("Support Engineer")).toBeInTheDocument();
  });
});

describe("CaseDetailsHeaderSkeleton", () => {
  it("should render header skeleton elements", () => {
    const { container } = render(
      <ThemeProvider theme={createTheme()}>
        <CaseDetailsHeaderSkeleton />
      </ThemeProvider>,
    );

    const skeletons = container.querySelectorAll(".MuiSkeleton-root");
    expect(skeletons.length).toBeGreaterThan(0);
  });
});
