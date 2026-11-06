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

import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import DeploymentsTab from "@features/csm-projects/components/DeploymentsTab";
import type { BeDeployment } from "@api/backend/types";

const access = { canWrite: true };
let deployments: BeDeployment[] = [];
const setAirGappedMutate = vi.fn();

// The details/create dialogs pull in hooks that import the backend client and
// runtime config, which throws at import time without a backend URL.
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@api/backend/client", () => ({ useBackendApi: () => ({}) }));
vi.mock("@context/current-user/usePortalAccess", () => ({
  usePortalAccess: () => access,
}));
vi.mock("@features/csm-cases/api/useSearchDeployments", () => ({
  useSearchDeployments: () => ({
    data: deployments,
    isLoading: false,
    isError: false,
    error: null,
    isFetching: false,
  }),
}));
vi.mock("@features/csm-projects/api/useUpdateDeployment", () => ({
  useUpdateDeployment: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("@features/csm-projects/api/useCreateDeployment", () => ({
  useCreateDeployment: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("@features/csm-projects/api/useSetDeploymentAirGapped", () => ({
  useSetDeploymentAirGapped: () => ({ mutate: setAirGappedMutate, isPending: false }),
}));

const DEPLOYMENT: BeDeployment = {
  id: "11111111-1111-1111-1111-111111111111",
  name: "Prod",
  type: "primary_production",
  airGapped: false,
};

function openActions(name = "Prod"): void {
  fireEvent.click(screen.getByRole("button", { name: `Actions for ${name}` }));
}

describe("DeploymentsTab air-gapped mark", () => {
  beforeEach(() => {
    access.canWrite = true;
    setAirGappedMutate.mockReset();
  });

  it("shows the Air-gapped chip only on air-gapped deployments", () => {
    deployments = [
      { ...DEPLOYMENT, airGapped: true },
      { ...DEPLOYMENT, id: "22222222-2222-2222-2222-222222222222", name: "QA", airGapped: false },
    ];
    render(<DeploymentsTab projectId="proj-1" />);
    const prodRow = screen.getByText("Prod").closest("tr") as HTMLElement;
    const qaRow = screen.getByText("QA").closest("tr") as HTMLElement;
    expect(within(prodRow).getByText("Air-gapped")).toBeInTheDocument();
    expect(within(qaRow).queryByText("Air-gapped")).not.toBeInTheDocument();
  });

  it("marks a connected deployment air-gapped after confirmation", () => {
    deployments = [DEPLOYMENT];
    render(<DeploymentsTab projectId="proj-1" />);
    openActions();
    fireEvent.click(screen.getByRole("menuitem", { name: /mark air-gapped/i }));
    fireEvent.click(screen.getByRole("button", { name: "Mark air-gapped" }));
    expect(setAirGappedMutate).toHaveBeenCalledWith(
      { deploymentId: DEPLOYMENT.id, airGapped: true },
      expect.anything(),
    );
  });

  it("clears the mark on an air-gapped deployment", () => {
    deployments = [{ ...DEPLOYMENT, airGapped: true }];
    render(<DeploymentsTab projectId="proj-1" />);
    openActions();
    fireEvent.click(screen.getByRole("menuitem", { name: /clear air-gapped mark/i }));
    fireEvent.click(screen.getByRole("button", { name: "Clear mark" }));
    expect(setAirGappedMutate).toHaveBeenCalledWith(
      { deploymentId: DEPLOYMENT.id, airGapped: false },
      expect.anything(),
    );
  });

  it("hides the action when the flag is unavailable (ServiceNow-backed)", () => {
    deployments = [{ ...DEPLOYMENT, airGapped: null }];
    render(<DeploymentsTab projectId="proj-1" />);
    openActions();
    expect(screen.queryByRole("menuitem", { name: /air-gapped/i })).not.toBeInTheDocument();
  });

  it("hides the action from read-only users", () => {
    access.canWrite = false;
    deployments = [DEPLOYMENT];
    render(<DeploymentsTab projectId="proj-1" />);
    openActions();
    expect(screen.queryByRole("menuitem", { name: /air-gapped/i })).not.toBeInTheDocument();
  });
});
