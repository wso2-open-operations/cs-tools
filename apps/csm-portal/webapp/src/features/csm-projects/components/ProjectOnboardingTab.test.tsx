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

import { fireEvent, render as rtlRender, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import ProjectOnboardingTab from "@features/csm-projects/components/ProjectOnboardingTab";
import type { ProjectDetails } from "@features/csm-projects/types/csmProjects";

// UserRefLink resolves unknown ids through the backend client, which reads
// runtime config at module load (absent under vitest).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ post: vi.fn(), get: vi.fn() }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

function render(ui: ReactElement) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return rtlRender(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

const PROJECT = {
  id: "proj-1",
  onboardingStatus: "In-Progress",
  onboardingOwner: {
    id: "00000000-0000-0000-0000-000000000000",
    name: "Jane Doe",
    email: "jane.doe@example.com",
  },
  onboardingStartDate: "2026-03-01T00:00:00Z",
  goLivePlanDate: "2026-04-15T00:00:00Z",
  goLiveDate: null,
  onboardingExpiryDate: "2026-12-31T00:00:00Z",
} as unknown as ProjectDetails;

describe("ProjectOnboardingTab", () => {
  it("is read-only without edit rights: no Edit button", () => {
    render(<ProjectOnboardingTab project={PROJECT} canEdit={false} onEdit={() => {}} />);
    expect(screen.getByText("In Progress")).toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Edit" })).not.toBeInTheDocument();
  });

  it("shows an Edit button that calls onEdit when the caller can edit", () => {
    const onEdit = vi.fn();
    render(<ProjectOnboardingTab project={PROJECT} canEdit onEdit={onEdit} />);
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    expect(onEdit).toHaveBeenCalledTimes(1);
  });

  it("renders each date as its UTC calendar day and a dash when empty", () => {
    render(<ProjectOnboardingTab project={PROJECT} canEdit={false} onEdit={() => {}} />);
    const valueOf = (label: string): string | null | undefined =>
      screen.getByText(label).nextElementSibling?.textContent;
    expect(valueOf("Onboarding start date")).toBe("Mar 1, 2026");
    expect(valueOf("Go-live plan date")).toBe("Apr 15, 2026");
    expect(valueOf("Go-live date")).toBe("—");
    expect(valueOf("Onboarding expiry date")).toBe("Dec 31, 2026");
  });

  it("shows dashes for a project with no onboarding data", () => {
    render(
      <ProjectOnboardingTab
        project={{ id: "proj-2" } as unknown as ProjectDetails}
        canEdit={false}
        onEdit={() => {}}
      />,
    );
    expect(screen.getAllByText("—")).toHaveLength(5);
  });

  describe("onboarding owner visibility", () => {
    const show = (p: ProjectDetails, canEdit = false) =>
      render(<ProjectOnboardingTab project={p} canEdit={canEdit} onEdit={() => {}} />);

    it.each(["Not-Applicable", undefined, null])(
      "hides the owner cell when status is %s, keeping Edit for editors",
      (status) => {
        show({ ...PROJECT, onboardingStatus: status } as unknown as ProjectDetails, true);
        expect(screen.queryByText("Onboarding owner")).not.toBeInTheDocument();
        expect(screen.queryByText("Jane Doe")).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Edit" })).toBeInTheDocument();
      },
    );

    it("shows the owner when onboarding is enabled", () => {
      show(PROJECT);
      expect(screen.getByText("Onboarding owner")).toBeInTheDocument();
      expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    });
  });

  describe("onboarding hours", () => {
    const withHours = (hours: Partial<ProjectDetails>): ProjectDetails =>
      ({ ...PROJECT, ...hours }) as ProjectDetails;
    const show = (p: ProjectDetails) =>
      render(<ProjectOnboardingTab project={p} canEdit={false} onEdit={() => {}} />);

    it("renders total, consumed, remaining and a progress bar when set", () => {
      show(
        withHours({
          totalOnboardingHours: 40,
          consumedOnboardingHours: 10.5,
          remainingOnboardingHours: 29.5,
        }),
      );
      expect(screen.getByText("Onboarding hours")).toBeInTheDocument();
      expect(screen.getByText("Total").nextElementSibling).toHaveTextContent("40 h");
      expect(screen.getByText("Consumed").nextElementSibling).toHaveTextContent("10.5 h");
      expect(screen.getByText("Remaining").nextElementSibling).toHaveTextContent("29.5 h");
      const bar = screen.getByRole("progressbar", { name: "Onboarding hours consumed" });
      expect(bar).toHaveAttribute("aria-valuenow", "26");
    });

    it("omits the group when no hours value is set", () => {
      show(
        withHours({
          totalOnboardingHours: null,
          consumedOnboardingHours: null,
          remainingOnboardingHours: null,
        }),
      );
      expect(screen.queryByText("Onboarding hours")).not.toBeInTheDocument();
      expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    });

    it("clamps the bar to 100% when consumed exceeds total", () => {
      show(
        withHours({
          totalOnboardingHours: 10,
          consumedOnboardingHours: 25,
          remainingOnboardingHours: 0,
        }),
      );
      expect(screen.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "100");
    });

    it("shows no bar when total is 0 or null", () => {
      const { unmount } = show(withHours({ totalOnboardingHours: 0, consumedOnboardingHours: 3 }));
      expect(screen.getByText("Onboarding hours")).toBeInTheDocument();
      expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
      unmount();
      show(withHours({ totalOnboardingHours: null, consumedOnboardingHours: 3 }));
      expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    });
  });
});
