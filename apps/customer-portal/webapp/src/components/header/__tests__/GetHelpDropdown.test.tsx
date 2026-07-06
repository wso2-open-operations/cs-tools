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
import GetHelpDropdown from "@components/header/GetHelpDropdown";

vi.mock("react-router", () => ({
  useNavigate: () => vi.fn(),
  useParams: () => ({ projectId: "project-1" }),
}));

vi.mock("@api/useGetProjects", () => ({
  default: () => ({
    data: { pages: [{ projects: [], totalRecords: 0 }] },
    isLoading: false,
    isFetching: false,
  }),
  flattenProjectPages: () => [],
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => ({
    data: { closureState: "ACTIVE", hasAgent: false },
    isLoading: false,
  }),
}));

vi.mock("@api/useGetProjectFeatures", () => ({
  default: () => ({ data: {}, isLoading: false }),
}));

vi.mock("@utils/permission", () => ({
  getProjectPermissions: () => ({ hasSR: true, hasSraWriteAccess: true }),
  isProjectRestricted: () => false,
}));

vi.mock("@wso2/oxygen-ui-icons-react", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@wso2/oxygen-ui-icons-react")>();
  return { ...actual };
});

describe("GetHelpDropdown", () => {
  it("should render an accessible Get Help control with responsive label", () => {
    render(<GetHelpDropdown />);

    expect(screen.getByRole("button", { name: "Get Help" })).toBeInTheDocument();
    expect(screen.getByText("Get Help")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "More help options" }),
    ).toBeInTheDocument();
  });
});
