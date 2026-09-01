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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import AddDeploymentWizardModal from "@features/project-details/components/deployments/AddDeploymentWizardModal";
import { usePostCreateDeployment } from "@features/project-details/api/usePostCreateDeployment";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { useGetProducts } from "@features/project-details/api/useGetProducts";
import { useSearchProductVersions } from "@features/project-details/api/useSearchProductVersions";
import { usePostDeploymentProduct } from "@features/project-details/api/usePostDeploymentProduct";

vi.mock("@features/project-details/api/usePostCreateDeployment");
vi.mock("@api/useGetProjectFilters");
vi.mock("@features/project-details/api/useGetProducts");
vi.mock("@features/project-details/api/useSearchProductVersions");
vi.mock("@features/project-details/api/usePostDeploymentProduct");

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
});

function renderWithProviders(ui: React.ReactElement) {
  return render(ui, {
    wrapper: ({ children }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    ),
  });
}

const mockFiltersData = {
  deploymentTypes: [{ id: "1", label: "Primary Production" }],
};

const mockProducts = [
  { id: "prod-api-mgr", label: "WSO2 API Manager", name: "API Manager" },
];

const mockVersions = [
  { id: "ver-781", version: "7.8.0", product: { id: "prod-api-mgr", label: "WSO2 API Manager" } },
];

describe("AddDeploymentWizardModal", () => {
  const mockCreateDeploymentMutate = vi.fn();
  const mockPostProductMutateAsync = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();

    vi.mocked(usePostCreateDeployment).mockReturnValue({
      mutate: mockCreateDeploymentMutate,
      isPending: false,
    } as unknown as ReturnType<typeof usePostCreateDeployment>);

    vi.mocked(useGetProjectFilters).mockReturnValue({
      data: mockFiltersData,
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useGetProjectFilters>);

    // Static, unchanging offset/limit means the query never refetches after
    // the initial load - this is the exact condition that exposed the
    // "Add another product" empty-list regression, since resetForm used to
    // wipe the local `products` state without anything to repopulate it.
    vi.mocked(useGetProducts).mockReturnValue({
      data: { products: mockProducts, totalRecords: mockProducts.length, offset: 0, limit: 10 },
      isLoading: false,
      isFetching: false,
      isError: false,
    } as unknown as ReturnType<typeof useGetProducts>);

    vi.mocked(useSearchProductVersions).mockImplementation(
      (productId: string) =>
        ({
          data: productId
            ? { versions: mockVersions, totalRecords: mockVersions.length, offset: 0, limit: 10 }
            : undefined,
          isLoading: false,
          isFetching: false,
          isError: false,
        }) as unknown as ReturnType<typeof useSearchProductVersions>,
    );

    vi.mocked(usePostDeploymentProduct).mockReturnValue({
      mutateAsync: mockPostProductMutateAsync,
      isPending: false,
    } as unknown as ReturnType<typeof usePostDeploymentProduct>);
  });

  async function advanceToProductStep() {
    fireEvent.change(screen.getByLabelText(/Deployment Name/i), {
      target: { value: "Production US-East" },
    });
    fireEvent.mouseDown(screen.getByLabelText(/Deployment Type/i));
    fireEvent.click(await screen.findByRole("option", { name: "Primary Production" }));
    fireEvent.change(screen.getByLabelText(/Description/i), {
      target: { value: "New deployment" },
    });

    mockCreateDeploymentMutate.mockImplementation((_body, { onSuccess }) => {
      onSuccess({ id: "dep-new" });
    });
    fireEvent.click(screen.getByRole("button", { name: "Next: Add Products" }));

    await screen.findByText(/Add one or more WSO2 products/);
  }

  async function selectProductAndVersion() {
    const productCombobox = screen.getByRole("combobox", { name: /Product Name/ });
    fireEvent.mouseDown(productCombobox);
    fireEvent.click(
      await screen.findByRole("option", { name: /WSO2 API Manager/, hidden: true }),
    );

    const versionCombobox = screen.getByRole("combobox", { name: /Version/ });
    await waitFor(() =>
      expect(versionCombobox).not.toHaveAttribute("aria-disabled", "true"),
    );
    fireEvent.mouseDown(versionCombobox);
    fireEvent.click(await screen.findByRole("option", { name: /7\.8\.0/, hidden: true }));
  }

  it("advances from the deployment step to the product step in the same dialog, without a second popup", async () => {
    renderWithProviders(
      <AddDeploymentWizardModal open={true} projectId="proj-1" onClose={vi.fn()} />,
    );

    expect(screen.getAllByRole("dialog")).toHaveLength(1);
    await advanceToProductStep();
    expect(screen.getAllByRole("dialog")).toHaveLength(1);
    expect(screen.getByText("Add WSO2 Product")).toBeInTheDocument();
  });

  it("keeps the product options populated after adding one and clicking Add another product (regression: list must not go empty)", async () => {
    mockPostProductMutateAsync.mockResolvedValue(undefined);
    renderWithProviders(
      <AddDeploymentWizardModal open={true} projectId="proj-1" onClose={vi.fn()} />,
    );

    await advanceToProductStep();
    await selectProductAndVersion();

    fireEvent.click(screen.getByRole("button", { name: "Add another product" }));

    await waitFor(() => expect(mockPostProductMutateAsync).toHaveBeenCalledTimes(1));

    // The just-added product should show in the running list...
    expect(screen.getByTestId("added-products-list")).toHaveTextContent(
      "WSO2 API Manager",
    );

    // ...and the product dropdown for the NEXT entry must still list real
    // options, not an empty list.
    const productCombobox = screen.getByRole("combobox", { name: /Product Name/ });
    fireEvent.mouseDown(productCombobox);
    expect(
      await screen.findByRole("option", { name: /WSO2 API Manager/, hidden: true }),
    ).toBeInTheDocument();
  });

  it("adds multiple products before Done closes the wizard", async () => {
    mockPostProductMutateAsync.mockResolvedValue(undefined);
    const onClose = vi.fn();
    renderWithProviders(
      <AddDeploymentWizardModal open={true} projectId="proj-1" onClose={onClose} />,
    );

    await advanceToProductStep();
    await selectProductAndVersion();
    fireEvent.click(screen.getByRole("button", { name: "Add another product" }));
    await waitFor(() => expect(mockPostProductMutateAsync).toHaveBeenCalledTimes(1));

    await selectProductAndVersion();
    fireEvent.click(screen.getByRole("button", { name: "Done" }));

    await waitFor(() => expect(mockPostProductMutateAsync).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });
});
