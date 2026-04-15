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
import type { ReactNode } from "react";
import CaseDetailsCard from "@case-details-details/CaseDetailsCard";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

function renderCard(props: {
  title: string;
  icon: ReactNode;
  children: ReactNode;
}) {
  return render(
    <ThemeProvider theme={createTheme()}>
      <CaseDetailsCard {...props} />
    </ThemeProvider>,
  );
}

describe("CaseDetailsCard", () => {
  it("should render title and children", () => {
    renderCard({
      title: "Case Overview",
      icon: <span data-testid="icon">i</span>,
      children: <p>Card body content</p>,
    });
    expect(screen.getByText("Case Overview")).toBeInTheDocument();
    expect(screen.getByText("Card body content")).toBeInTheDocument();
    expect(screen.getByTestId("icon")).toBeInTheDocument();
  });

  it("should render another title and content", () => {
    renderCard({
      title: "Product & Environment",
      icon: <span />,
      children: <div>Product details</div>,
    });
    expect(screen.getByText("Product & Environment")).toBeInTheDocument();
    expect(screen.getByText("Product details")).toBeInTheDocument();
  });
});
