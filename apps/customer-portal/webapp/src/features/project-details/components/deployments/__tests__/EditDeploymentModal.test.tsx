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

import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import EditDeploymentModal from "@features/project-details/components/deployments/EditDeploymentModal";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { usePatchDeployment } from "@features/project-details/api/usePatchDeployment";
import type { ProjectDeploymentItem } from "@features/project-details/types/deployments";

vi.mock("@api/useGetProjectFilters");
vi.mock("@features/project-details/api/usePatchDeployment");

const mockDeployment: ProjectDeploymentItem = {
  id: "dep-1",
  name: "Production",
  createdOn: "2026-01-17",
  updatedOn: "2026-01-17",
  description: "Primary production env",
  url: "https://api.example.com",
  project: { id: "proj-1", label: "Test Project" },
  type: { id: "6", label: "Primary Production" },
};

const mockFiltersData = {
  deploymentTypes: [
    { id: "1", label: "Development" },
    { id: "2", label: "QA" },
    { id: "3", label: "Staging" },
    { id: "6", label: "Primary Production" },
  ],
};

const defaultProps = {
  open: true,
  deployment: mockDeployment,
  projectId: "proj-1",
  onClose: vi.fn(),
  onSuccess: vi.fn(),
  onError: vi.fn(),
};

describe("EditDeploymentModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useGetProjectFilters).mockReturnValue({
      data: mockFiltersData,
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useGetProjectFilters>);
    vi.mocked(usePatchDeployment).mockReturnValue({
      mutateAsync: vi.fn().mockResolvedValue(undefined),
      isPending: false,
    } as unknown as ReturnType<typeof usePatchDeployment>);
  });

  it("should render modal with pre-filled deployment data", () => {
    render(<EditDeploymentModal {...defaultProps} />);

    expect(screen.getByText("Edit Deployment")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Production")).toBeInTheDocument();
    expect(
      screen.getByDisplayValue("Primary production env"),
    ).toBeInTheDocument();
    expect(screen.getByRole("combobox")).toHaveTextContent(
      "Primary Production",
    );
  });

  it("should call PATCH with correct body when Update is clicked", async () => {
    const mutateAsync = vi.fn().mockResolvedValue(undefined);
    vi.mocked(usePatchDeployment).mockReturnValue({
      mutateAsync,
      isPending: false,
    } as unknown as ReturnType<typeof usePatchDeployment>);

    render(<EditDeploymentModal {...defaultProps} />);

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Update" }));
    });

    expect(mutateAsync).toHaveBeenCalledWith({
      projectId: "proj-1",
      deploymentId: "dep-1",
      body: {
        name: "Production",
        description: "Primary production env",
        typeKey: 6,
        active: true,
      },
    });
  });

  it("should disable submit when name is empty", () => {
    render(
      <EditDeploymentModal
        {...defaultProps}
        deployment={{ ...mockDeployment, name: "" }}
      />,
    );

    const nameInput = screen.getByLabelText(/Deployment Name/);
    fireEvent.change(nameInput, { target: { value: "" } });

    expect(screen.getByRole("button", { name: "Update" })).toBeDisabled();
  });
});
