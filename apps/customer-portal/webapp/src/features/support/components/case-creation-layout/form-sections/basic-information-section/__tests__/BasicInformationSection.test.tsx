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
import { BasicInformationSection } from "@features/support/components/case-creation-layout/form-sections/basic-information-section/BasicInformationSection";

function renderSection(
  props: Partial<Parameters<typeof BasicInformationSection>[0]> = {},
) {
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

  it("should render Deployment and Product Version labels", () => {
    renderSection();
    expect(screen.getByText("Deployment")).toBeInTheDocument();
    expect(screen.getByText("Product Version")).toBeInTheDocument();
  });

  it("should show Not available when product list is empty", () => {
    renderSection({
      deployment: "Prod",
      productOptionList: [],
      isProductDropdownDisabled: false,
      isProductLoading: false,
    });
    expect(screen.getByText("Not available")).toBeInTheDocument();
  });

  it("should show an Add Deployment alert instead of the dropdown when there are no deployments and onAddDeployment is provided", () => {
    const onAddDeployment = vi.fn();
    renderSection({
      metadata: { deploymentTypes: [] },
      productOptionList: [{ id: "prod-1", label: "API Manager 4.2.0" }],
      onAddDeployment,
    });

    // The dropdown is hidden entirely in favor of the empty-state Alert.
    expect(screen.queryByText("Not available")).not.toBeInTheDocument();
    expect(
      screen.getByText(
        "No deployments configured for this project. Add a deployment to continue creating your case.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "This information helps us provide you with better support.",
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Add Deployment" }));
    expect(onAddDeployment).toHaveBeenCalledTimes(1);
  });

  it("should render the Add Deployment alert with warning severity", () => {
    renderSection({
      metadata: { deploymentTypes: [] },
      productOptionList: [{ id: "prod-1", label: "API Manager 4.2.0" }],
      onAddDeployment: vi.fn(),
    });

    const alertMessage = screen.getByText(
      "No deployments configured for this project. Add a deployment to continue creating your case.",
    );
    const alertRoot = alertMessage.closest(".MuiAlert-root");
    expect(alertRoot).not.toBeNull();
    expect(alertRoot).toHaveClass("MuiAlert-standardWarning");
  });

  it("should show an Add Deployment option inside the dropdown menu when deployments exist, reachable while the menu is open", () => {
    const onAddDeployment = vi.fn();
    renderSection({
      metadata: { deploymentTypes: ["Prod", "Staging"] },
      onAddDeployment,
    });

    // The dropdown remains visible; the "add new" row lives inside its menu
    // (not a button below the field, which an open menu's overlay would cover).
    expect(screen.getByText("Deployment")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Add Deployment" }),
    ).not.toBeInTheDocument();

    fireEvent.mouseDown(screen.getAllByRole("combobox")[0]);
    const addOption = screen.getByRole("option", { name: /add deployment/i });
    expect(addOption).toBeInTheDocument();

    fireEvent.click(addOption);
    expect(onAddDeployment).toHaveBeenCalledTimes(1);
  });

  it("should not show an Add Deployment option in the menu when the deployment field is disabled", () => {
    renderSection({
      metadata: { deploymentTypes: ["Prod"] },
      isDeploymentDisabled: true,
      onAddDeployment: vi.fn(),
    });

    fireEvent.mouseDown(screen.getAllByRole("combobox")[0]);
    expect(
      screen.queryByRole("option", { name: /add deployment/i }),
    ).not.toBeInTheDocument();
  });

  it("should keep the legacy disabled dropdown when there are no deployments and onAddDeployment is not provided", () => {
    renderSection({
      metadata: { deploymentTypes: [] },
      productOptionList: [{ id: "prod-1", label: "API Manager 4.2.0" }],
    });

    expect(
      screen.queryByText(
        "No deployments configured for this project. Add a deployment to continue creating your case.",
      ),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Not available")).toBeInTheDocument();
  });

  it("should show an Add Product alert instead of the dropdown when the selected deployment has no products and onAddProduct is provided", () => {
    const onAddProduct = vi.fn();
    renderSection({
      deployment: "Prod",
      productOptionList: [],
      isProductDropdownDisabled: false,
      isProductLoading: false,
      onAddProduct,
    });

    expect(screen.queryByText("Not available")).not.toBeInTheDocument();
    expect(
      screen.getByText(
        "No products found for this deployment. Add a product to proceed.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "This information helps us provide you with better support.",
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Add Product" }));
    expect(onAddProduct).toHaveBeenCalledTimes(1);
  });

  it("should render the Add Product alert with warning severity", () => {
    renderSection({
      deployment: "Prod",
      productOptionList: [],
      isProductDropdownDisabled: false,
      isProductLoading: false,
      onAddProduct: vi.fn(),
    });

    const alertMessage = screen.getByText(
      "No products found for this deployment. Add a product to proceed.",
    );
    const alertRoot = alertMessage.closest(".MuiAlert-root");
    expect(alertRoot).not.toBeNull();
    expect(alertRoot).toHaveClass("MuiAlert-standardWarning");
  });

  it("should show an Add Product option inside the dropdown menu when products exist, reachable while the menu is open", () => {
    const onAddProduct = vi.fn();
    renderSection({
      deployment: "Prod",
      productOptionList: [{ id: "prod-1", label: "API Manager 4.2.0" }],
      isProductDropdownDisabled: false,
      isProductLoading: false,
      onAddProduct,
    });

    expect(
      screen.queryByRole("button", { name: "Add Product" }),
    ).not.toBeInTheDocument();

    fireEvent.mouseDown(screen.getAllByRole("combobox")[1]);
    const addOption = screen.getByRole("option", { name: /add product/i });
    expect(addOption).toBeInTheDocument();

    fireEvent.click(addOption);
    expect(onAddProduct).toHaveBeenCalledTimes(1);
  });

  it("should not show an Add Product option in the menu while the product dropdown is disabled (no deployment selected)", () => {
    renderSection({
      deployment: "",
      productOptionList: [],
      isProductDropdownDisabled: true,
      isProductLoading: false,
      onAddProduct: vi.fn(),
    });

    // The disabled Select can't be opened, so the menu (and its "add new"
    // row) never renders.
    fireEvent.mouseDown(screen.getAllByRole("combobox")[1]);
    expect(
      screen.queryByRole("option", { name: /add product/i }),
    ).not.toBeInTheDocument();
  });

  it("should not show the product empty-state while a deployment has not been selected yet", () => {
    const onAddProduct = vi.fn();
    renderSection({
      deployment: "",
      productOptionList: [],
      isProductDropdownDisabled: true,
      isProductLoading: false,
      onAddProduct,
    });

    expect(
      screen.queryByText(
        "No products found for this deployment. Add a product to proceed.",
      ),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Select deployment first")).toBeInTheDocument();
  });
});
