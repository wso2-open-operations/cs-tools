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
import { afterEach, describe, expect, it, vi } from "vitest";
import ProjectDeployments from "@features/project-details/components/deployments/ProjectDeployments";
import type { DeploymentCardProps } from "@features/project-details/types/projectDetailsComponents";

const state = vi.hoisted(() => ({
  srProductCategories: ["ms"] as string[] | null,
  navigate: vi.fn(),
}));

afterEach(() => {
  state.srProductCategories = ["ms"];
  state.navigate.mockClear();
});

vi.mock("react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router")>();
  return { ...actual, useNavigate: () => state.navigate };
});

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => ({
    data: { type: { label: "Managed Cloud Subscription" }, closureState: null },
  }),
}));

vi.mock("@api/useGetProjectFeatures", () => ({
  default: () => ({
    data: {
      hasDeploymentReadAccess: true,
      hasDeploymentWriteAccess: true,
      srProductCategories: state.srProductCategories,
    },
  }),
}));

vi.mock("@api/usePostProjectDeploymentsSearch", () => ({
  usePostProjectDeploymentsSearchInfinite: () => ({
    data: {
      pages: [
        {
          totalRecords: 1,
          deployments: [
            {
              id: "dep-1",
              name: "Prod",
              type: { id: "t1", label: "Primary Production" },
              project: { id: "proj-1", label: "Demo" },
            },
          ],
        },
      ],
    },
    isLoading: false,
    isPending: false,
    isError: false,
    isFetchingNextPage: false,
    hasNextPage: false,
    fetchNextPage: vi.fn(),
  }),
}));

// A stand-in card: one button picks a product, as a product's checkbox would.
vi.mock("@features/project-details/components/deployments/DeploymentCard", () => ({
  default: ({ deployment, onToggleProductSelect }: DeploymentCardProps) => (
    <button
      type="button"
      onClick={() => onToggleProductSelect(deployment.id, "item-1")}
    >
      Pick product
    </button>
  ),
}));

vi.mock("@features/project-details/components/deployments/AddDeploymentModal", () => ({
  default: () => null,
}));

describe("ProjectDeployments service request selection", () => {
  it("creates a service request for the picked product", () => {
    render(<ProjectDeployments projectId="proj-1" />);

    fireEvent.click(screen.getByRole("button", { name: "Pick product" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Create Service Request (1)" }),
    );

    expect(state.navigate).toHaveBeenCalledWith(
      expect.stringContaining("productId=item-1"),
    );
  });

  it("drops the selection when the project's service request rules change", () => {
    const { rerender } = render(<ProjectDeployments projectId="proj-1" />);
    fireEvent.click(screen.getByRole("button", { name: "Pick product" }));
    expect(
      screen.getByRole("button", { name: "Create Service Request (1)" }),
    ).toBeEnabled();

    state.srProductCategories = ["pc"];
    rerender(<ProjectDeployments projectId="proj-1" />);

    const create = screen.getByRole("button", { name: "Create Service Request" });
    expect(create).toBeDisabled();
    fireEvent.click(create);
    expect(state.navigate).not.toHaveBeenCalled();
  });
});
