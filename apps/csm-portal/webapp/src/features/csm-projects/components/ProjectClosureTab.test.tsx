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

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import ProjectClosureTab from "@features/csm-projects/components/ProjectClosureTab";
import type { ProjectDetails } from "@features/csm-projects/types/csmProjects";

const { MockBackendApiError } = vi.hoisted(() => ({
  MockBackendApiError: class MockBackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));
vi.mock("@api/backend/client", () => ({
  BackendApiError: MockBackendApiError,
  useBackendApi: () => ({ patch: vi.fn() }),
}));

type MutateOptions = {
  onSuccess?: (res: unknown) => void;
  onSettled?: () => void;
};
const mutateMock = vi.fn();
const resetMock = vi.fn();
const patchState: { isPending: boolean; isError: boolean; error: unknown } = {
  isPending: false,
  isError: false,
  error: null,
};
vi.mock("@features/csm-projects/api/usePatchProjectSuspensionOverride", () => ({
  usePatchProjectSuspensionOverride: () => ({
    mutate: mutateMock,
    reset: resetMock,
    isPending: patchState.isPending,
    isError: patchState.isError,
    error: patchState.error,
  }),
}));

const SWITCH_NAME = "Exclude this project from automatic closure";

function project(overrides: Record<string, unknown> = {}): ProjectDetails {
  return {
    id: "proj-1",
    name: "Acme Subscription",
    key: "ACME",
    subscriptionType: "managed_cloud_subscription",
    startDate: "2025-01-01T00:00:00Z",
    endDate: "2026-01-31T00:00:00Z",
    createdOn: "2025-01-01T00:00:00Z",
    updatedOn: "2025-06-01T00:00:00Z",
    closureState: "open",
    endDateClosureState: "notify",
    invoiceDueDateClosureState: "open",
    complianceViolationClosureState: "suspended",
    complianceViolationDate: "2026-02-10T00:00:00Z",
    suspensionProcessOverridden: false,
    suspensionProcessState: null,
    ...overrides,
  } as unknown as ProjectDetails;
}

beforeEach(() => {
  mutateMock.mockReset();
  resetMock.mockReset();
  patchState.isPending = false;
  patchState.isError = false;
  patchState.error = null;
});

describe("ProjectClosureTab — state and tracks", () => {
  it("shows the overall state, dates and one card per track with its chip", () => {
    render(<ProjectClosureTab project={project()} canEdit={false} />);
    expect(screen.getByText("Overall closure state")).toBeInTheDocument();
    expect(screen.getByText("Jan 1, 2025")).toBeInTheDocument();
    const tracks = within(screen.getByRole("region", { name: "Closure tracks" }));
    const items = tracks.getAllByRole("listitem");
    expect(items).toHaveLength(3);
    expect(within(items[0]!).getByText("Subscription end date")).toBeInTheDocument();
    expect(within(items[0]!).getByText("End date Jan 31, 2026")).toBeInTheDocument();
    expect(within(items[0]!).getByText("Notify")).toBeInTheDocument();
    expect(within(items[1]!).getByText("Invoice due date")).toBeInTheDocument();
    expect(within(items[1]!).getByText("Open")).toBeInTheDocument();
    expect(within(items[2]!).getByText("Compliance violation")).toBeInTheDocument();
    expect(within(items[2]!).getByText("Violation date Feb 10, 2026")).toBeInTheDocument();
    expect(within(items[2]!).getByText("Suspended")).toBeInTheDocument();
  });

  it.each([
    ["open", "Open"],
    ["notify", "Notify"],
    ["read_only", "Read only"],
    ["restricted", "Restricted"],
    ["suspended", "Suspended"],
    ["closed", "Closed"],
    ["brand_new_state", "Brand new state"],
  ])("renders the %s state as a chip labelled %s without crashing", (state, label) => {
    render(
      <ProjectClosureTab
        project={project({
          closureState: null,
          endDateClosureState: state,
          invoiceDueDateClosureState: null,
          complianceViolationClosureState: null,
        })}
        canEdit={false}
      />,
    );
    const tracks = within(screen.getByRole("region", { name: "Closure tracks" }));
    expect(tracks.getByText(label)).toBeInTheDocument();
  });

  it("renders a dash for missing states and dates", () => {
    render(
      <ProjectClosureTab
        project={project({
          closureState: null,
          startDate: null,
          endDate: null,
          endDateClosureState: null,
          invoiceDueDateClosureState: null,
          complianceViolationClosureState: null,
          complianceViolationDate: null,
        })}
        canEdit={false}
      />,
    );
    expect(screen.getAllByText("—").length).toBeGreaterThanOrEqual(5);
    expect(screen.getByText("No violation date")).toBeInTheDocument();
  });

  it("tolerates a project without the closure sub-fields at all (older backend)", () => {
    render(
      <ProjectClosureTab
        project={
          {
            id: "proj-1",
            name: "x",
            closureState: "open",
            startDate: null,
            endDate: null,
          } as unknown as ProjectDetails
        }
        canEdit={false}
      />,
    );
    expect(screen.getByText("Closure tracks")).toBeInTheDocument();
    expect(screen.getByText("No closure process record yet.")).toBeInTheDocument();
  });
});

describe("ProjectClosureTab — automatic closure control", () => {
  it("is read-only for a non-admin: text, no switch, no mutation", () => {
    render(
      <ProjectClosureTab project={project({ suspensionProcessOverridden: true })} canEdit={false} />,
    );
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    expect(screen.getByText("Excluded from automatic closure")).toBeInTheDocument();
    expect(mutateMock).not.toHaveBeenCalled();
  });

  it("read-only shows Included when the flag is false and a dash when it is absent", () => {
    const { rerender } = render(<ProjectClosureTab project={project()} canEdit={false} />);
    expect(screen.getByText("Included in automatic closure")).toBeInTheDocument();
    rerender(
      <ProjectClosureTab
        project={project({ suspensionProcessOverridden: undefined })}
        canEdit={false}
      />,
    );
    expect(screen.queryByText("Included in automatic closure")).not.toBeInTheDocument();
    expect(screen.queryByText("Excluded from automatic closure")).not.toBeInTheDocument();
  });

  it("is an accessible, described switch for an admin, reflecting the saved value", () => {
    render(
      <ProjectClosureTab project={project({ suspensionProcessOverridden: true })} canEdit />,
    );
    const sw = screen.getByRole("checkbox", { name: SWITCH_NAME });
    expect(sw).toBeChecked();
    expect(sw).toBeEnabled();
    expect(sw).toHaveAccessibleDescription(/never sends notices/);
  });

  it("treats an undefined flag as off and does not crash", () => {
    render(
      <ProjectClosureTab project={project({ suspensionProcessOverridden: undefined })} canEdit />,
    );
    expect(screen.getByRole("checkbox", { name: SWITCH_NAME })).not.toBeChecked();
  });

  it("toggling on saves immediately with true, no confirm step", () => {
    render(<ProjectClosureTab project={project()} canEdit />);
    fireEvent.click(screen.getByRole("checkbox", { name: SWITCH_NAME }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(mutateMock).toHaveBeenCalledTimes(1);
    expect(mutateMock.mock.calls[0]?.[0]).toBe(true);
  });

  it("toggling off saves with false", () => {
    render(
      <ProjectClosureTab project={project({ suspensionProcessOverridden: true })} canEdit />,
    );
    fireEvent.click(screen.getByRole("checkbox", { name: SWITCH_NAME }));
    expect(mutateMock.mock.calls[0]?.[0]).toBe(false);
  });

  it("shows the new value and a busy, disabled switch while saving", () => {
    mutateMock.mockImplementation(() => {
      patchState.isPending = true;
    });
    const { rerender } = render(<ProjectClosureTab project={project()} canEdit />);
    fireEvent.click(screen.getByRole("checkbox", { name: SWITCH_NAME }));
    rerender(<ProjectClosureTab project={project()} canEdit />);
    const sw = screen.getByRole("checkbox", { name: SWITCH_NAME });
    expect(sw).toBeDisabled();
    expect(sw).toBeChecked();
    expect(sw).toHaveAttribute("aria-busy", "true");
    expect(screen.getByLabelText("Saving")).toBeInTheDocument();
  });

  it("on success keeps the confirmed value and shows success feedback", () => {
    mutateMock.mockImplementation((_v: boolean, opts: MutateOptions) => {
      act(() => {
        opts.onSuccess?.({ message: "ok", project: { id: "proj-1", suspensionProcessOverridden: true } });
        opts.onSettled?.();
      });
    });
    render(<ProjectClosureTab project={project()} canEdit />);
    fireEvent.click(screen.getByRole("checkbox", { name: SWITCH_NAME }));
    expect(screen.getByRole("checkbox", { name: SWITCH_NAME })).toBeChecked();
    expect(screen.getByRole("status")).toHaveTextContent(
      "This project is now excluded from automatic closure.",
    );
  });

  it("uses the backend's returned value over the one requested", () => {
    mutateMock.mockImplementation((_v: boolean, opts: MutateOptions) => {
      act(() => {
        opts.onSuccess?.({ project: { id: "proj-1", suspensionProcessOverridden: false } });
        opts.onSettled?.();
      });
    });
    render(<ProjectClosureTab project={project()} canEdit />);
    fireEvent.click(screen.getByRole("checkbox", { name: SWITCH_NAME }));
    expect(screen.getByRole("checkbox", { name: SWITCH_NAME })).not.toBeChecked();
  });

  it("once the refetched project changes, the project's own value wins again", () => {
    mutateMock.mockImplementation((_v: boolean, opts: MutateOptions) => {
      act(() => {
        opts.onSuccess?.({ project: { id: "proj-1", suspensionProcessOverridden: true } });
        opts.onSettled?.();
      });
    });
    const { rerender } = render(<ProjectClosureTab project={project()} canEdit />);
    fireEvent.click(screen.getByRole("checkbox", { name: SWITCH_NAME }));
    rerender(
      <ProjectClosureTab project={project({ suspensionProcessOverridden: false })} canEdit />,
    );
    // The project says false again (e.g. a later edit elsewhere): trust it.
    expect(screen.getByRole("checkbox", { name: SWITCH_NAME })).toBeChecked();
    rerender(<ProjectClosureTab project={project({ suspensionProcessOverridden: true })} canEdit />);
    expect(screen.getByRole("checkbox", { name: SWITCH_NAME })).toBeChecked();
  });

  it("on a client error shows the backend's message and reverts to the saved value", () => {
    mutateMock.mockImplementation((_v: boolean, opts: MutateOptions) => {
      patchState.isError = true;
      patchState.error = new MockBackendApiError(403, "Admin role required");
      act(() => {
        opts.onSettled?.();
      });
    });
    const { rerender } = render(<ProjectClosureTab project={project()} canEdit />);
    fireEvent.click(screen.getByRole("checkbox", { name: SWITCH_NAME }));
    // The real mutation state change re-renders the component; model it.
    rerender(<ProjectClosureTab project={project()} canEdit />);
    expect(screen.getByRole("checkbox", { name: SWITCH_NAME })).not.toBeChecked();
    expect(screen.getByRole("alert")).toHaveTextContent("Admin role required");
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("on a 5xx hides the raw upstream text behind a generic message", () => {
    patchState.isError = true;
    patchState.error = new MockBackendApiError(500, "pq: connection refused at 10.0.0.1");
    render(<ProjectClosureTab project={project()} canEdit />);
    const alert = screen.getByRole("alert");
    expect(alert).not.toHaveTextContent("pq:");
    expect(alert).toHaveTextContent("Could not update automatic closure for this project.");
  });

  it("a non-API error also gets the generic message", () => {
    patchState.isError = true;
    patchState.error = new Error("network down");
    render(<ProjectClosureTab project={project()} canEdit />);
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Could not update automatic closure for this project.",
    );
  });
});

describe("ProjectClosureTab — closure process record", () => {
  function record() {
    return within(screen.getByRole("region", { name: "Closure process record" }));
  }

  it.each([[null], [undefined], [""], [{}]])("shows the empty message for %j", (state) => {
    render(<ProjectClosureTab project={project({ suspensionProcessState: state })} canEdit={false} />);
    expect(record().getByText("No closure process record yet.")).toBeInTheDocument();
    expect(document.querySelector("pre")).toBeNull();
  });

  it.each([
    ["30_days_notice", "30-day notice sent"],
    ["90_days_notice", "90-day notice sent"],
    ["suspend_notice", "Suspension notice sent"],
    ["open", "Open (no notice yet)"],
    ["totally_unknown", "totally_unknown"],
  ])("labels the %s event as %s", (eventType, label) => {
    render(
      <ProjectClosureTab
        project={project({
          suspensionProcessState: { based_on_due_invoices: { event_type: eventType } },
        })}
        canEdit={false}
      />,
    );
    expect(record().getByText("Due invoices")).toBeInTheDocument();
    expect(record().getByText(label)).toBeInTheDocument();
  });

  it("labels a suspend event with its date", () => {
    render(
      <ProjectClosureTab
        project={project({
          suspensionProcessState: {
            based_on_compliance: {
              event_type: "suspend",
              suspended_on: "2026-03-05",
              actionSuspendProject: "SUCCESSFUL",
            },
          },
        })}
        canEdit={false}
      />,
    );
    expect(record().getByText("Suspended on Mar 5, 2026")).toBeInTheDocument();
    expect(record().getByText("Project suspension")).toBeInTheDocument();
    expect(record().getByText("Successful")).toBeInTheDocument();
  });

  it("renders every track with its action result chips", () => {
    render(
      <ProjectClosureTab
        project={project({
          suspensionProcessState: {
            based_on_subscription_end_date: {
              event_type: "30_days_notice",
              actionSendEmailNotification: "SUCCESSFUL",
              actionServicePortalAnnouncement: "IGNORED",
            },
            based_on_due_invoices: { event_type: "suspend_notice", actionSendEmailNotification: "FAILED" },
            based_on_compliance: { event_type: "open" },
          },
        })}
        canEdit={false}
      />,
    );
    expect(record().getAllByRole("listitem").filter((li) => li.querySelector("p"))).toHaveLength(3);
    expect(record().getByText("Subscription end date")).toBeInTheDocument();
    expect(record().getByText("Compliance")).toBeInTheDocument();
    expect(record().getByText("Ignored")).toBeInTheDocument();
    expect(record().getByText("Failed")).toBeInTheDocument();
    expect(record().getAllByText("Email notification").length).toBe(2);
  });

  it("reads a JSON string instead of an object", () => {
    render(
      <ProjectClosureTab
        project={project({
          suspensionProcessState: JSON.stringify({ based_on_compliance: { event_type: "open" } }),
        })}
        canEdit={false}
      />,
    );
    expect(record().getByText("Open (no notice yet)")).toBeInTheDocument();
  });

  it("shows an unrecognised shape as collapsed, pretty-printed JSON, never hidden", () => {
    render(
      <ProjectClosureTab
        project={project({ suspensionProcessState: { surprise: { a: 1 } } })}
        canEdit={false}
      />,
    );
    const details = document.querySelector("details")!;
    expect(details).not.toHaveAttribute("open");
    expect(record().getByText(/unrecognised format/)).toBeInTheDocument();
    const pre = screen.getByLabelText("Raw closure process record");
    expect(pre.tagName).toBe("PRE");
    expect(pre).toHaveAttribute("tabindex", "0");
    expect(pre.textContent).toBe(JSON.stringify({ surprise: { a: 1 } }, null, 2));
    expect(record().queryByText("No closure process record yet.")).not.toBeInTheDocument();
  });

  it("shows unknown keys next to recognised tracks in the collapsed raw view", () => {
    render(
      <ProjectClosureTab
        project={project({
          suspensionProcessState: { based_on_compliance: { event_type: "open" }, extra: 5 },
        })}
        canEdit={false}
      />,
    );
    expect(record().getByText("Open (no notice yet)")).toBeInTheDocument();
    expect(record().getByText("Other fields in the closure process record")).toBeInTheDocument();
    expect(screen.getByLabelText("Raw closure process record").textContent).toBe(
      JSON.stringify({ extra: 5 }, null, 2),
    );
  });

  it("shows unparseable text verbatim", () => {
    render(
      <ProjectClosureTab project={project({ suspensionProcessState: "not json" })} canEdit={false} />,
    );
    expect(screen.getByLabelText("Raw closure process record").textContent).toBe("not json");
  });
});
