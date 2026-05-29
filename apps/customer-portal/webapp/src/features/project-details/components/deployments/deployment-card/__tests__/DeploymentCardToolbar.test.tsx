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
import { describe, expect, it, vi } from "vitest";
import DeploymentCardToolbar from "@features/project-details/components/deployments/deployment-card/DeploymentCardToolbar";

describe("DeploymentCardToolbar", () => {
  it("renders edit and delete actions", () => {
    render(
      <DeploymentCardToolbar
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        isDeleteDisabled={false}
      />,
    );
    expect(screen.getByRole("button", { name: "Edit deployment" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Delete deployment" })).toBeInTheDocument();
  });
});
