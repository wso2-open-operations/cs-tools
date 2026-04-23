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
import { beforeEach, describe, expect, it, vi } from "vitest";
import AddDeploymentModal from "@features/project-details/components/deployments/AddDeploymentModal";
import { usePostCreateDeployment } from "@features/project-details/api/usePostCreateDeployment";
import useGetProjectFilters from "@api/useGetProjectFilters";

vi.mock("@features/project-details/api/usePostCreateDeployment");
vi.mock("@api/useGetProjectFilters");

let mockMutate: any;
let defaultModalProps: any;

const mockFiltersData = {
  deploymentTypes: [
    { id: "1", label: "Development" },
    { id: "2", label: "QA" },
    { id: "3", label: "Staging" },
    { id: "4", label: "Primary Production" },
  ],
};

vi.mock("@components/error-banner/ErrorBanner", () => ({
  default: ({ message, onClose }: { message: string; onClose: () => void }) => (
    <div data-testid="error-banner">
      {message}
      <button onClick={onClose}>Dismiss Banner</button>
    </div>
  ),
}));

describe("AddDeploymentModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    mockMutate = vi.fn();
    defaultModalProps = {
      open: true,
      projectId: "project-123",
      onClose: vi.fn(),
      onSuccess: vi.fn(),
      onError: vi.fn(),
    };

    vi.mocked(usePostCreateDeployment).mockReturnValue({
      mutate: mockMutate,
      isPending: false,
    } as unknown as ReturnType<typeof usePostCreateDeployment>);

    vi.mocked(useGetProjectFilters).mockReturnValue({
      data: mockFiltersData,
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useGetProjectFilters>);
  });

  it("should render the modal with title and description", () => {
    render(<AddDeploymentModal {...defaultModalProps} />);

    expect(screen.getByText("Add New Deployment")).toBeInTheDocument();
    expect(
      screen.getByText("Create a new deployment environment for your project."),
    ).toBeInTheDocument();
  });

  it("should render Deployment Name and Description fields", () => {
    render(<AddDeploymentModal {...defaultModalProps} />);

    expect(screen.getByLabelText(/Deployment Name/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/Description/i)).toBeInTheDocument();
  });

  it("should render deployment type select from the API", () => {
    render(<AddDeploymentModal {...defaultModalProps} />);

    expect(screen.getByLabelText(/Deployment Type/i)).toBeInTheDocument();
  });

  it("should show skeleton while deployment types are loading", () => {
    vi.mocked(useGetProjectFilters).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
    } as unknown as ReturnType<typeof useGetProjectFilters>);

    render(<AddDeploymentModal {...defaultModalProps} />);

    // While loading, the Deployment Type select should not be rendered
    expect(screen.queryByLabelText(/Deployment Type/i)).not.toBeInTheDocument();
  });

  it("should show ErrorIndicator inside modal when deployment types fail to load", () => {
    vi.mocked(useGetProjectFilters).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
    } as unknown as ReturnType<typeof useGetProjectFilters>);

    render(<AddDeploymentModal {...defaultModalProps} />);

    expect(screen.getByTestId("error-indicator")).toBeInTheDocument();
  });

  it("should show ErrorBanner outside modal when deployment types fail to load", () => {
    vi.mocked(useGetProjectFilters).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
    } as unknown as ReturnType<typeof useGetProjectFilters>);

    render(<AddDeploymentModal {...defaultModalProps} />);

    expect(screen.getByTestId("error-banner")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Failed to load deployment types. Please close and try again.",
      ),
    ).toBeInTheDocument();
  });

  it("should dismiss ErrorBanner when dismiss button is clicked", () => {
    vi.mocked(useGetProjectFilters).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
    } as unknown as ReturnType<typeof useGetProjectFilters>);

    render(<AddDeploymentModal {...defaultModalProps} />);

    expect(screen.getByTestId("error-banner")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Dismiss Banner"));

    expect(screen.queryByTestId("error-banner")).not.toBeInTheDocument();
  });

  it("should disable Add Deployment button when form is incomplete", () => {
    render(<AddDeploymentModal {...defaultModalProps} />);

    const addButton = screen.getByRole("button", { name: /Add Deployment/i });
    expect(addButton).toBeDisabled();
  });

  it("should disable Add Deployment button when filters failed to load", () => {
    vi.mocked(useGetProjectFilters).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
    } as unknown as ReturnType<typeof useGetProjectFilters>);

    render(<AddDeploymentModal {...defaultModalProps} />);

    const addButton = screen.getByRole("button", { name: /Add Deployment/i });
    expect(addButton).toBeDisabled();
  });

  it("should call onClose when Cancel button is clicked", () => {
    render(<AddDeploymentModal {...defaultModalProps} />);

    fireEvent.click(screen.getByRole("button", { name: /Cancel/i }));

    expect(defaultModalProps.onClose).toHaveBeenCalledTimes(1);
  });

  it("should call onClose when X icon button is clicked", () => {
    render(<AddDeploymentModal {...defaultModalProps} />);

    fireEvent.click(screen.getByRole("button", { name: /close/i }));

    expect(defaultModalProps.onClose).toHaveBeenCalledTimes(1);
  });

  it("should show loading state when mutation is pending", () => {
    vi.mocked(usePostCreateDeployment).mockReturnValue({
      mutate: mockMutate,
      isPending: true,
    } as unknown as ReturnType<typeof usePostCreateDeployment>);

    render(<AddDeploymentModal {...defaultModalProps} />);

    expect(screen.getByText("Creating...")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Cancel/i })).toBeDisabled();
  });

  it("should not render when open is false", () => {
    render(<AddDeploymentModal {...defaultModalProps} open={false} />);

    expect(screen.queryByText("Add New Deployment")).not.toBeInTheDocument();
  });

  it("should keep Add Deployment button disabled when only name is filled", () => {
    render(<AddDeploymentModal {...defaultModalProps} />);

    // Fill only the name field — button should still be disabled
    fireEvent.change(screen.getByLabelText(/Deployment Name/i), {
      target: { value: "My Deployment" },
    });

    const addButton = screen.getByRole("button", { name: /Add Deployment/i });
    expect(addButton).toBeDisabled();
  });

  it("should submit form with correct data when all fields are valid", () => {
    render(<AddDeploymentModal {...defaultModalProps} />);

    fireEvent.change(screen.getByLabelText(/Deployment Name/i), {
      target: { value: "Production West" },
    });

    fireEvent.mouseDown(screen.getByLabelText(/Deployment Type/i));
    fireEvent.click(screen.getByRole("option", { name: "Development" }));
    fireEvent.change(screen.getByLabelText(/Description/i), {
      target: { value: "Primary dev env" },
    });

    const addButton = screen.getByRole("button", { name: /Add Deployment/i });
    expect(addButton).not.toBeDisabled();
    fireEvent.click(addButton);
    expect(mockMutate).toHaveBeenCalledTimes(1);
    expect(mockMutate).toHaveBeenCalledWith(
      {
        name: "Production West",
        deploymentTypeKey: 1,
        description: "Primary dev env",
      },
      expect.objectContaining({
        onSuccess: expect.any(Function),
        onError: expect.any(Function),
      }),
    );
  });
});
