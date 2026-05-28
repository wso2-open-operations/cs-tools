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
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import ProjectDetails from "@features/project-details/pages/ProjectDetails";

vi.mock("react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router")>();
  return {
    ...actual,
    useParams: () => ({ projectId: "proj-1" }),
  };
});

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ debug: vi.fn(), error: vi.fn() }),
}));

vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: () => ({ showLoader: vi.fn(), hideLoader: vi.fn() }),
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ isLoading: false }),
}));

vi.mock("@api/useGetProjects", () => ({
  default: () => ({
    data: { pages: [{ projects: [{ id: "proj-1", type: { label: "Enterprise" } }] }] },
  }),
  flattenProjectPages: (data: { pages: { projects: unknown[] }[] } | undefined) =>
    data?.pages.flatMap((p) => p.projects) ?? [],
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => ({
    data: { id: "proj-1", name: "Demo", key: "DEMO", type: { label: "Enterprise" } },
    isLoading: false,
    error: null,
  }),
}));

vi.mock("@api/useGetProjectFeatures", () => ({
  default: () => ({
    data: {
      hasDeploymentReadAccess: true,
      hasTimeLogsReadAccess: true,
      acceptedSeverityValues: [],
    },
    isLoading: false,
  }),
}));

vi.mock("@features/project-details/api/useGetProjectStat", () => ({
  useGetProjectStat: () => ({
    data: { projectStats: { slaStatus: "Good" } },
    isLoading: false,
    error: null,
  }),
}));

vi.mock("@features/project-details/components/ProjectInformationCard", () => ({
  default: () => <div data-testid="project-information-card" />,
}));

vi.mock("@features/project-details/components/project-overview/contact-info/ContactInfoCard", () => ({
  default: () => <div data-testid="contact-info-card" />,
}));

vi.mock("@features/project-details/components/project-overview/service-hours-allocations/ServiceHoursAllocationsCard", () => ({
  default: () => <div data-testid="service-hours-card" />,
}));

vi.mock("@features/project-details/components/deployments/ProjectDeployments", () => ({
  default: () => <div data-testid="project-deployments" />,
}));

vi.mock("@features/project-details/components/time-tracking/ProjectTimeTracking", () => ({
  default: () => <div data-testid="project-time-tracking" />,
}));

describe("ProjectDetails", () => {
  it("renders overview tab by default", () => {
    render(
      <MemoryRouter>
        <ProjectDetails />
      </MemoryRouter>,
    );
    expect(screen.getByRole("tab", { name: /overview/i })).toBeInTheDocument();
    expect(screen.getByTestId("project-information-card")).toBeInTheDocument();
  });
});
