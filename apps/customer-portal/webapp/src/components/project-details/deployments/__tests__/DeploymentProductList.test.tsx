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
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactElement } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import DeploymentProductList from "@components/project-details/deployments/DeploymentProductList";
import { useGetDeploymentsProducts } from "@api/useGetDeploymentsProducts";
import { useGetProducts } from "@api/useGetProducts";
import { useSearchProductVersions } from "@api/useSearchProductVersions";
import { usePostDeploymentProduct } from "@api/usePostDeploymentProduct";
import { usePatchDeploymentProduct } from "@api/usePatchDeploymentProduct";
import type { DeploymentProductItem } from "@models/responses";

vi.mock("@api/useGetDeploymentsProducts");
vi.mock("@api/useGetProducts");
vi.mock("@api/useSearchProductVersions");
vi.mock("@api/usePostDeploymentProduct");
vi.mock("@api/usePatchDeploymentProduct");

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

function renderWithProviders(ui: ReactElement) {
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>,
  );
}

const defaultSelectionProps = {
  selectedProduct: null,
  onToggleProductSelect: vi.fn(),
};

const mockProducts: DeploymentProductItem[] = [
  {
    id: "prod-1",
    createdOn: "2026-01-17",
    updatedOn: "2026-01-17",
    description: "API Gateway",
    product: { id: "p1", label: "WSO2 API Manager" },
    deployment: { id: "dep-1", label: "Staging" },
  },
  {
    id: "prod-2",
    createdOn: "2026-01-17",
    updatedOn: "2026-01-17",
    description: "IAM",
    product: { id: "p2", label: "WSO2 Identity Server" },
    deployment: { id: "dep-1", label: "Staging" },
  },
];

describe("DeploymentProductList", () => {
  beforeEach(() => {
    vi.mocked(useGetProducts).mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useGetProducts>);
    vi.mocked(useSearchProductVersions).mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useSearchProductVersions>);
    vi.mocked(usePostDeploymentProduct).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof usePostDeploymentProduct>);
    vi.mocked(usePatchDeploymentProduct).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof usePatchDeploymentProduct>);
  });

  it("should render products count and Add Product button", () => {
    vi.mocked(useGetDeploymentsProducts).mockReturnValue({
      data: mockProducts,
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useGetDeploymentsProducts>);

    renderWithProviders(
      <DeploymentProductList
        deploymentId="dep-123"
        projectId="proj-1"
        {...defaultSelectionProps}
      />,
    );

    expect(screen.getByText("WSO2 Products")).toBeInTheDocument();
    expect(screen.getByText("(2)")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Add Product/i }),
    ).toBeInTheDocument();
  });

  it("should render product labels and descriptions", () => {
    vi.mocked(useGetDeploymentsProducts).mockReturnValue({
      data: mockProducts,
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useGetDeploymentsProducts>);

    renderWithProviders(
      <DeploymentProductList
        deploymentId="dep-123"
        projectId="proj-1"
        {...defaultSelectionProps}
      />,
    );

    expect(screen.getByText("WSO2 API Manager")).toBeInTheDocument();
    expect(screen.getByText("API Gateway")).toBeInTheDocument();
    expect(screen.getByText("WSO2 Identity Server")).toBeInTheDocument();
    expect(screen.getByText("IAM")).toBeInTheDocument();
  });

  it("should display No products added yet when products array is empty", () => {
    vi.mocked(useGetDeploymentsProducts).mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useGetDeploymentsProducts>);

    renderWithProviders(
      <DeploymentProductList
        deploymentId="dep-123"
        projectId="proj-1"
        {...defaultSelectionProps}
      />,
    );

    expect(screen.getByText("WSO2 Products")).toBeInTheDocument();
    expect(screen.getByText("(0)")).toBeInTheDocument();
    expect(screen.getByText("No products added yet")).toBeInTheDocument();
  });

  it("should display Not Available for null product label", () => {
    vi.mocked(useGetDeploymentsProducts).mockReturnValue({
      data: [
        {
          id: "p1",
          createdOn: "",
          updatedOn: "",
          description: null,
          product: { id: "p1", label: "" },
          deployment: { id: "d1", label: "" },
        },
      ],
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useGetDeploymentsProducts>);

    renderWithProviders(
      <DeploymentProductList
        deploymentId="dep-123"
        projectId="proj-1"
        {...defaultSelectionProps}
      />,
    );

    expect(screen.getAllByText("Not Available").length).toBeGreaterThan(0);
  });

  it("should show loading state", () => {
    vi.mocked(useGetDeploymentsProducts).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
    } as unknown as ReturnType<typeof useGetDeploymentsProducts>);

    renderWithProviders(
      <DeploymentProductList
        deploymentId="dep-123"
        projectId="proj-1"
        {...defaultSelectionProps}
      />,
    );

    const skeletons = document.querySelectorAll(".MuiSkeleton-root");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("should show error state when products fetch fails", () => {
    vi.mocked(useGetDeploymentsProducts).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
    } as unknown as ReturnType<typeof useGetDeploymentsProducts>);

    renderWithProviders(
      <DeploymentProductList
        deploymentId="dep-123"
        projectId="proj-1"
        {...defaultSelectionProps}
      />,
    );

    expect(screen.getByText("Failed to load products")).toBeInTheDocument();
  });

  it("should open Add Product modal when button is clicked", () => {
    vi.mocked(useGetDeploymentsProducts).mockReturnValue({
      data: mockProducts,
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useGetDeploymentsProducts>);

    renderWithProviders(
      <DeploymentProductList
        deploymentId="dep-123"
        projectId="proj-1"
        {...defaultSelectionProps}
      />,
    );

    const addButton = screen.getByRole("button", { name: /Add Product/i });
    fireEvent.click(addButton);

    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });
});
