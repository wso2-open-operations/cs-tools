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
// The owner picker's directory search is covered by its own hook test.
vi.mock("@features/csm-projects/api/useSearchInternalUsers", () => ({
  useSearchInternalUsers: () => ({
    data: [
      { id: "11111111-1111-1111-1111-111111111111", name: "John Roe", email: "john.roe@example.com" },
    ],
    isFetching: false,
    isError: false,
  }),
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
  productionReadinessCompletionDate: "2026-05-20T00:00:00Z",
} as unknown as ProjectDetails;

const DATE_LABELS = [
  "Onboarding start date",
  "Go-live plan date",
  "Go-live date",
  "Onboarding expiry date",
  "Production readiness completion date",
];

/** The picker's field is a group of sections; its value lives in a hidden
 *  input beside it, which is what accepts a typed/pasted full date. */
function dateInput(label: string): HTMLInputElement {
  const group = screen.getByRole("group", { name: label });
  const input = group.parentElement?.querySelector("input");
  if (!input) throw new Error(`no input for ${label}`);
  return input;
}

function typeDate(label: string, value: string): void {
  fireEvent.change(dateInput(label), { target: { value } });
}

describe("ProjectOnboardingTab", () => {
  it("is read-only without edit rights: no edit pencils", () => {
    render(<ProjectOnboardingTab project={PROJECT} canEdit={false} onSave={() => {}} />);
    expect(screen.getByText("In Progress")).toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Edit / })).not.toBeInTheDocument();
  });

  it("shows a pencil on each of the seven fields for an editor, and no input until one is clicked", () => {
    render(<ProjectOnboardingTab project={PROJECT} canEdit onSave={() => {}} />);
    for (const label of ["Onboarding status", "Onboarding owner", ...DATE_LABELS]) {
      expect(screen.getByRole("button", { name: `Edit ${label}` })).toBeInTheDocument();
    }
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Save" })).not.toBeInTheDocument();
  });

  it("renders each date as its UTC calendar day and a dash when empty", () => {
    render(<ProjectOnboardingTab project={PROJECT} canEdit={false} onSave={() => {}} />);
    const valueOf = (label: string): string | null | undefined =>
      screen.getByText(label).nextElementSibling?.textContent;
    expect(valueOf("Onboarding start date")).toBe("Mar 1, 2026");
    expect(valueOf("Go-live plan date")).toBe("Apr 15, 2026");
    expect(valueOf("Go-live date")).toBe("—");
    expect(valueOf("Onboarding expiry date")).toBe("Dec 31, 2026");
    expect(valueOf("Production readiness completion date")).toBe("May 20, 2026");
  });

  it("shows dashes for a project with no onboarding data", () => {
    render(
      <ProjectOnboardingTab
        project={{ id: "proj-2" } as unknown as ProjectDetails}
        canEdit={false}
        onSave={() => {}}
      />,
    );
    // Status chip fallback plus the five dates.
    expect(screen.getAllByText("—")).toHaveLength(6);
  });

  describe("onboarding owner visibility", () => {
    const show = (p: ProjectDetails, canEdit = false) =>
      render(<ProjectOnboardingTab project={p} canEdit={canEdit} onSave={() => {}} />);

    it.each(["Not-Applicable", undefined, null])(
      "hides the owner cell when status is %s, keeping Edit for editors",
      (status) => {
        show({ ...PROJECT, onboardingStatus: status } as unknown as ProjectDetails, true);
        expect(screen.queryByText("Onboarding owner")).not.toBeInTheDocument();
        expect(screen.queryByText("Jane Doe")).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Edit Onboarding status" })).toBeInTheDocument();
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
      render(<ProjectOnboardingTab project={p} canEdit={false} onSave={() => {}} />);

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

  describe("per-field inline editing", () => {
    const open = (label: string, onSave = vi.fn(), extra: Partial<Parameters<typeof ProjectOnboardingTab>[0]> = {}) => {
      render(<ProjectOnboardingTab project={PROJECT} canEdit onSave={onSave} {...extra} />);
      fireEvent.click(screen.getByRole("button", { name: `Edit ${label}` }));
      return onSave;
    };
    const save = (): void => {
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
    };

    it("swaps only the clicked field to its input", () => {
      open("Go-live date");
      expect(dateInput("Go-live date")).toBeInTheDocument();
      expect(screen.queryByRole("group", { name: "Go-live plan date" })).not.toBeInTheDocument();
      expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    });

    it("seeds the date picker from the UTC calendar day of the stored value", () => {
      open("Go-live plan date");
      expect(dateInput("Go-live plan date")).toHaveValue("04/15/2026");
    });

    it("sends only that date under its request key, with a leave-edit callback", () => {
      const onSave = open("Go-live date");
      typeDate("Go-live date", "06/15/2026");
      save();
      expect(onSave).toHaveBeenCalledTimes(1);
      expect(onSave).toHaveBeenCalledWith({ onboardingGoLiveDate: "2026-06-15" }, expect.any(Function));
      // Still editing until the caller reports success.
      expect(screen.getByRole("button", { name: "Save" })).toBeInTheDocument();
    });

    it("closes the editor when the save succeeds", async () => {
      const onSave = vi.fn((_patch, onSuccess: () => void) => onSuccess());
      open("Go-live date", onSave);
      typeDate("Go-live date", "06/15/2026");
      save();
      expect(await screen.findByRole("button", { name: "Edit Go-live date" })).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Save" })).not.toBeInTheDocument();
    });

    it("sends the production readiness completion date under its request key", () => {
      const onSave = open("Production readiness completion date");
      typeDate("Production readiness completion date", "06/30/2026");
      save();
      expect(onSave).toHaveBeenCalledWith(
        { onboardingProductionReadinessCompletionDate: "2026-06-30" },
        expect.any(Function),
      );
    });

    it("sends only the status when the status is edited", () => {
      const onSave = open("Onboarding status");
      fireEvent.mouseDown(screen.getByRole("combobox", { name: /onboarding status/i }));
      fireEvent.click(screen.getByRole("option", { name: "On Hold" }));
      save();
      expect(onSave).toHaveBeenCalledWith({ onboardingStatus: "On-Hold" }, expect.any(Function));
    });

    it("sends only the owner, by id, when the owner is edited", () => {
      const onSave = open("Onboarding owner");
      fireEvent.mouseDown(screen.getByRole("combobox", { name: /onboarding owner/i }));
      fireEvent.click(screen.getByRole("option", { name: "John Roe" }));
      save();
      expect(onSave).toHaveBeenCalledWith(
        { onboardingOwner: "11111111-1111-1111-1111-111111111111" },
        expect.any(Function),
      );
    });

    it("cancel closes the field, sends nothing and resets the error", () => {
      const onSave = vi.fn();
      const onResetError = vi.fn();
      open("Go-live date", onSave, { onResetError });
      typeDate("Go-live date", "06/15/2026");
      fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
      expect(onSave).not.toHaveBeenCalled();
      expect(onResetError).toHaveBeenCalled();
      expect(screen.queryByRole("group", { name: "Go-live date" })).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Edit Go-live date" }));
      expect(dateInput("Go-live date")).toHaveValue("");
    });

    it("blocks Save when the date is earlier than the stored start date", () => {
      open("Go-live date");
      typeDate("Go-live date", "02/15/2026");
      expect(screen.getByText(/earlier than the onboarding start date/i)).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    });

    it("applies the order rule to the readiness date too", () => {
      open("Production readiness completion date");
      typeDate("Production readiness completion date", "02/15/2026");
      expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    });

    it("blocks moving the start date after a stored later date", () => {
      open("Onboarding start date");
      typeDate("Onboarding start date", "05/01/2026");
      expect(screen.getByText("Can't be later than the stored go-live plan date.")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    });

    it("allows moving the start date earlier", () => {
      const onSave = open("Onboarding start date");
      typeDate("Onboarding start date", "01/15/2026");
      save();
      expect(onSave).toHaveBeenCalledWith({ onboardingStartDate: "2026-01-15" }, expect.any(Function));
    });

    it("treats an emptied date as unchanged: Save stays disabled, nothing is sent", () => {
      const onSave = open("Go-live plan date");
      typeDate("Go-live plan date", "");
      expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
      expect(onSave).not.toHaveBeenCalled();
    });

    it("offers no clear buttons", () => {
      open("Go-live plan date");
      expect(screen.queryByRole("button", { name: /clear/i })).not.toBeInTheDocument();
    });

    it("surfaces the server's message next to the open field and keeps the draft", () => {
      open("Go-live plan date", vi.fn(), {
        saveError: "go-live plan date must not be earlier than the onboarding start date",
      });
      expect(screen.getByRole("alert")).toHaveTextContent("must not be earlier than the onboarding start date");
      expect(screen.getByRole("group", { name: "Go-live plan date" })).toBeInTheDocument();
    });

    it("shows no error when no field is open", () => {
      render(<ProjectOnboardingTab project={PROJECT} canEdit onSave={() => {}} saveError="boom" />);
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it("disables Save and Cancel while the save is in flight", () => {
      const { rerender } = render(<ProjectOnboardingTab project={PROJECT} canEdit onSave={() => {}} />);
      fireEvent.click(screen.getByRole("button", { name: "Edit Go-live date" }));
      rerender(
        <QueryClientProvider client={new QueryClient()}>
          <MemoryRouter>
            <ProjectOnboardingTab project={PROJECT} canEdit isSaving onSave={() => {}} />
          </MemoryRouter>
        </QueryClientProvider>,
      );
      expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
      expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    });

    it("keeps the open-time baseline when the project refreshes under the editor", () => {
      const onSave = vi.fn();
      const { rerender } = render(<ProjectOnboardingTab project={PROJECT} canEdit onSave={onSave} />);
      fireEvent.click(screen.getByRole("button", { name: "Edit Go-live plan date" }));
      rerender(
        <QueryClientProvider client={new QueryClient()}>
          <MemoryRouter>
            <ProjectOnboardingTab
              project={{ ...PROJECT, goLivePlanDate: "2026-09-09T00:00:00Z" } as ProjectDetails}
              canEdit
              onSave={onSave}
            />
          </MemoryRouter>
        </QueryClientProvider>,
      );
      expect(dateInput("Go-live plan date")).toHaveValue("04/15/2026");
    });

    it("is reachable for an editor on a project with no onboarding status yet", () => {
      render(
        <ProjectOnboardingTab
          project={{ id: "proj-3" } as unknown as ProjectDetails}
          canEdit
          onSave={() => {}}
        />,
      );
      fireEvent.click(screen.getByRole("button", { name: "Edit Onboarding status" }));
      expect(screen.getByRole("combobox", { name: /onboarding status/i })).toBeInTheDocument();
    });
  });

});
