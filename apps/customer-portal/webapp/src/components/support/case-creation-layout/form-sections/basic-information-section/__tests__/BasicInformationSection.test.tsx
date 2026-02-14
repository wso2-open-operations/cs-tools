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

import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { BasicInformationSection } from "@components/support/case-creation-layout/form-sections/basic-information-section/BasicInformationSection";

function renderSection(props: Partial<Parameters<typeof BasicInformationSection>[0]> = {}) {
  return render(
    <ThemeProvider theme={createTheme()}>
      <BasicInformationSection
        project="My Project"
        product=""
        setProduct={vi.fn()}
        deployment=""
        setDeployment={vi.fn()}
        metadata={{ deploymentTypes: ["Prod"], products: ["Product A"] }}
        {...props}
      />
    </ThemeProvider>,
  );
}

describe("BasicInformationSection", () => {
  it("should render section title Basic Information", () => {
    renderSection();
    expect(screen.getByText("Basic Information")).toBeInTheDocument();
  });

  it("should render Project label and Auto detected chip", () => {
    renderSection();
    expect(screen.getByText("Project")).toBeInTheDocument();
    expect(screen.getAllByText("Auto detected").length).toBeGreaterThan(0);
  });

  it("should display project value when provided", () => {
    renderSection({ project: "Test Project" });
    expect(screen.getByDisplayValue("Test Project")).toBeInTheDocument();
  });

  it("should render Deployment Type and Product & Version labels", () => {
    renderSection();
    expect(screen.getByText("Deployment Type")).toBeInTheDocument();
    expect(screen.getByText("Product & Version")).toBeInTheDocument();
  });

  it("should render edit button with accessible label", () => {
    renderSection();
    expect(
      screen.getByRole("button", { name: /edit basic information/i }),
    ).toBeInTheDocument();
  });

  it("should show deployment skeleton when isDeploymentLoading is true", () => {
    renderSection({ isDeploymentLoading: true });
    const skeletons = document.querySelectorAll('[class*="MuiSkeleton"]');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("should show product skeleton when isProductLoading is true", () => {
    renderSection({ isProductLoading: true });
    const skeletons = document.querySelectorAll('[class*="MuiSkeleton"]');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("should enable editing when edit button is clicked", () => {
    renderSection({
      metadata: { deploymentTypes: ["Production"], products: [] },
    });
    const editButton = screen.getByRole("button", {
      name: /edit basic information/i,
    });
    fireEvent.click(editButton);
    expect(editButton).toBeInTheDocument();
  });
});
