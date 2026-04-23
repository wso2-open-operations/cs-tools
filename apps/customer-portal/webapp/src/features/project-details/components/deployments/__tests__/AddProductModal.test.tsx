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
import AddProductModal from "@features/project-details/components/deployments/AddProductModal";
import { useGetProducts } from "@features/project-details/api/useGetProducts";
import { useSearchProductVersions } from "@features/project-details/api/useSearchProductVersions";
import { usePostDeploymentProduct } from "@features/project-details/api/usePostDeploymentProduct";

vi.mock("@features/project-details/api/useGetProducts");
vi.mock("@features/project-details/api/useSearchProductVersions");
vi.mock("@features/project-details/api/usePostDeploymentProduct");

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
});

function renderWithProviders(
  ui: React.ReactElement,
  options?: { queryClient?: QueryClient },
) {
  const client = options?.queryClient ?? queryClient;
  return render(ui, {
    wrapper: ({ children }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    ),
  });
}

const mockProducts = [
  { id: "prod-api-mgr", label: "WSO2 API Manager", name: "API Manager" },
  { id: "prod-id", label: "WSO2 Identity Server", name: "Identity Server" },
];

const mockVersions = [
  { id: "ver-781", version: "7.8.0", product: { id: "prod-api-mgr", label: "WSO2 API Manager" } },
  { id: "ver-450", version: "4.5.0", product: { id: "prod-api-mgr", label: "WSO2 API Manager" } },
];

const mockProductsPage = {
  products: mockProducts,
  totalRecords: mockProducts.length,
  offset: 0,
  limit: 10,
};

const mockVersionsPage = {
  versions: mockVersions,
  totalRecords: mockVersions.length,
  offset: 0,
  limit: 10,
};

describe("AddProductModal", () => {
  const mockOnClose = vi.fn();
  const mockOnSuccess = vi.fn();
  const mockMutateAsync = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useGetProducts).mockReturnValue({
      data: mockProductsPage,
      isLoading: false,
      isFetching: false,
      isError: false,
    } as unknown as ReturnType<typeof useGetProducts>);
    vi.mocked(useSearchProductVersions).mockImplementation(
      (productId: string) =>
        ({
          data: productId ? mockVersionsPage : undefined,
          isLoading: false,
          isFetching: false,
          isError: false,
        }) as unknown as ReturnType<typeof useSearchProductVersions>,
    );
    vi.mocked(usePostDeploymentProduct).mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
    } as unknown as ReturnType<typeof usePostDeploymentProduct>);
  });

  it("should not render when open is false", () => {
    renderWithProviders(
      <AddProductModal
        open={false}
        deploymentId="dep-1"
        projectId="proj-1"
        onClose={mockOnClose}
      />,
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("should render correctly when open is true", () => {
    renderWithProviders(
      <AddProductModal
        open={true}
        deploymentId="dep-1"
        projectId="proj-1"
        onClose={mockOnClose}
      />,
    );

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByText("Add WSO2 Product")).toBeInTheDocument();
    expect(screen.getByLabelText(/Product Name/)).toBeInTheDocument();
    expect(screen.getByLabelText(/Version/)).toBeInTheDocument();
    expect(screen.getByLabelText(/Core Count/)).toBeInTheDocument();
    expect(screen.getByLabelText(/TPS/)).toBeInTheDocument();
    expect(screen.getByLabelText(/Description/)).toBeInTheDocument();
  });

  it("should disable version dropdown until product is selected", () => {
    renderWithProviders(
      <AddProductModal
        open={true}
        deploymentId="dep-1"
        projectId="proj-1"
        onClose={mockOnClose}
      />,
    );

    const versionSelect = screen.getByLabelText(/Version/);
    expect(versionSelect).toHaveAttribute("aria-disabled", "true");
  });

  it("should validate required fields", () => {
    renderWithProviders(
      <AddProductModal
        open={true}
        deploymentId="dep-1"
        projectId="proj-1"
        onClose={mockOnClose}
      />,
    );

    const submitButton = screen.getByRole("button", { name: "Add Product" });
    expect(submitButton).toBeDisabled();
  });

  it("should enable submit and call API when product and version selected", async () => {
    mockMutateAsync.mockResolvedValue(undefined);
    renderWithProviders(
      <AddProductModal
        open={true}
        deploymentId="dep-1"
        projectId="proj-1"
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    const productSelect = screen.getByLabelText(/Product Name/);
    fireEvent.mouseDown(productSelect);
    fireEvent.click(
      await screen.findByRole("option", {
        name: /WSO2 API Manager/,
        hidden: true,
      }),
    );

    const versionSelect = screen.getByLabelText(/Version/);
    await waitFor(() =>
      expect(versionSelect).not.toHaveAttribute("aria-disabled", "true"),
    );
    fireEvent.mouseDown(versionSelect);
    fireEvent.click(
      await screen.findByRole("option", { name: /7\.8\.0/, hidden: true }),
    );

    const submitButton = screen.getByRole("button", { name: "Add Product" });
    await waitFor(() => expect(submitButton).not.toBeDisabled());

    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        deploymentId: "dep-1",
        body: {
          productId: "prod-api-mgr",
          versionId: "ver-781",
          projectId: "proj-1",
          cores: undefined,
          tps: undefined,
        },
      });
    });
    expect(mockOnSuccess).toHaveBeenCalled();
    expect(mockOnClose).toHaveBeenCalled();
  });

  it("should include cores and tps when provided", async () => {
    mockMutateAsync.mockResolvedValue(undefined);
    renderWithProviders(
      <AddProductModal
        open={true}
        deploymentId="dep-1"
        projectId="proj-1"
        onClose={mockOnClose}
        onSuccess={mockOnSuccess}
      />,
    );

    fireEvent.mouseDown(screen.getByLabelText(/Product Name/));
    fireEvent.click(
      await screen.findByRole("option", {
        name: /WSO2 API Manager/,
        hidden: true,
      }),
    );

    const versionField = screen.getByLabelText(/Version/);
    await waitFor(() =>
      expect(versionField).not.toHaveAttribute("aria-disabled", "true"),
    );
    fireEvent.mouseDown(versionField);
    fireEvent.click(
      await screen.findByRole("option", { name: /7\.8\.0/, hidden: true }),
    );

    fireEvent.change(screen.getByLabelText(/Core Count/), {
      target: { value: "8" },
    });
    fireEvent.change(screen.getByLabelText(/TPS/), {
      target: { value: "5000" },
    });

    const submitButton = screen.getByRole("button", { name: "Add Product" });
    await waitFor(() => expect(submitButton).not.toBeDisabled());
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        deploymentId: "dep-1",
        body: {
          productId: "prod-api-mgr",
          versionId: "ver-781",
          projectId: "proj-1",
          cores: 8,
          tps: 5000,
        },
      });
    });
  });

  it("should reset form on close", async () => {
    const { rerender } = renderWithProviders(
      <AddProductModal
        open={true}
        deploymentId="dep-1"
        projectId="proj-1"
        onClose={mockOnClose}
      />,
    );

    const productCombobox = screen.getByRole("combobox", {
      name: /Product Name/,
    });
    fireEvent.mouseDown(productCombobox);
    fireEvent.click(
      await screen.findByRole("option", {
        name: /WSO2 API Manager/,
        hidden: true,
      }),
    );
    expect(productCombobox).toHaveTextContent(/WSO2 API Manager/);

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(mockOnClose).toHaveBeenCalled();

    // Simulate parent closing and reopening modal; form should be reset.
    rerender(
      <AddProductModal
        open={false}
        deploymentId="dep-1"
        projectId="proj-1"
        onClose={mockOnClose}
      />,
    );
    rerender(
      <AddProductModal
        open={true}
        deploymentId="dep-1"
        projectId="proj-1"
        onClose={mockOnClose}
      />,
    );

    // After reset, Product Name should be cleared (empty selection)
    expect(
      screen.getByRole("combobox", { name: /Product Name/ }),
    ).not.toHaveTextContent("WSO2 API Manager");
  });

  it("should show a fresh product list when remounted with a new key after close", async () => {
    const { rerender } = renderWithProviders(
      <AddProductModal
        key={0}
        open={true}
        deploymentId="dep-1"
        projectId="proj-1"
        onClose={mockOnClose}
      />,
    );

    const productCombobox = screen.getByRole("combobox", {
      name: /Product Name/,
    });
    fireEvent.mouseDown(productCombobox);
    fireEvent.click(
      await screen.findByRole("option", {
        name: /WSO2 API Manager/,
        hidden: true,
      }),
    );
    expect(productCombobox).toHaveTextContent(/WSO2 API Manager/);

    rerender(
      <AddProductModal
        key={0}
        open={false}
        deploymentId="dep-1"
        projectId="proj-1"
        onClose={mockOnClose}
      />,
    );
    rerender(
      <AddProductModal
        key={1}
        open={true}
        deploymentId="dep-1"
        projectId="proj-1"
        onClose={mockOnClose}
      />,
    );

    expect(
      screen.getByRole("combobox", { name: /Product Name/ }),
    ).not.toHaveTextContent("WSO2 API Manager");
  });
});
