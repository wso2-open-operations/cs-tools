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
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const navigateMock = vi.fn();
const postIncidentMutateMock = vi.fn();
const showErrorMock = vi.fn();

const ME_ID = "11111111-1111-4111-8111-111111111111";
const SERVICE_ID = "22222222-2222-4222-8222-222222222222";

vi.mock("react-router", () => ({
  useNavigate: () => navigateMock,
  useLocation: () => ({ state: undefined }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
vi.mock("@features/csm-operations/api/usePostIncident", () => ({
  usePostIncident: () => ({ mutate: postIncidentMutateMock, isPending: false }),
}));
// Caller defaults to the signed-in user; give it one so Caller is filled.
vi.mock("@features/settings/api/useGetUsersMe", () => ({
  useGetUsersMe: () => ({ data: { id: ME_ID, firstName: "Test", lastName: "User" } }),
}));
// The real client module reads window.config at load; mock it with a real
// class so `instanceof` still works (same as CreateProblemPage.test.tsx).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    payload?: { message?: string; errorCode?: string };
    constructor(status: number, message: string, payload?: { errorCode?: string }) {
      super(message);
      this.status = status;
      this.payload = payload;
    }
  },
}));
// vi.mock factories are hoisted above this file's top-level declarations, so
// a helper they reference directly has to be hoisted with them.
const { emptySearch } = vi.hoisted(() => ({
  emptySearch: (): { data: never[]; isFetching: boolean; isError: boolean } => ({
    data: [],
    isFetching: false,
    isError: false,
  }),
}));
vi.mock("@api/useSearchItServices", () => ({ useSearchItServices: emptySearch }));
vi.mock("@api/useSearchServiceOfferings", () => ({ useSearchServiceOfferings: emptySearch }));
vi.mock("@api/useSearchConfigurationItems", () => ({ useSearchConfigurationItems: emptySearch }));
vi.mock("@api/useSearchUsersByName", () => ({ useSearchInternalUsersByName: emptySearch }));
vi.mock("@api/useSearchGroups", () => ({ useSearchSupportGroups: emptySearch }));

// GET /incidents/create-defaults: the default team, swapped per test.
const DEFAULT_TEAM = { id: "44444444-4444-4444-8444-444444444444", name: "Default Team" };
const createDefaults: {
  current: { defaultServiceId: string | null; defaultGroup: typeof DEFAULT_TEAM | null } | undefined;
} = { current: undefined };
vi.mock("@features/csm-operations/api/useGetIncidentCreateDefaults", () => ({
  useGetIncidentCreateDefaults: () => ({ data: createDefaults.current }),
}));
// AsyncEntitySelect is a full type-ahead Autocomplete; stub it as a plain
// labeled input that reports its id straight through onChange, same as
// CreateProblemPage.test.tsx. The Service field also hands back the picked
// service (with its support group, or none) as the real one does, and the
// Assignment group field the picked group. The stub also exposes what the
// page passes for display: the known label, the pinned option, the helper
// text (rendered, so its "Use it" link is clickable) and the error flag.
const SUPPORT_GROUP = { id: "33333333-3333-4333-8333-333333333333", name: "Platform SRE" };
const OTHER_SUPPORT_GROUP = { id: "55555555-5555-4555-8555-555555555555", name: "Storage Ops" };
const PICKED_GROUP = { id: "66666666-6666-4666-8666-666666666666", name: "Network Ops" };
const SERVICE_NO_GROUP_ID = "77777777-7777-4777-8777-777777777777";
const OTHER_SERVICE_ID = "88888888-8888-4888-8888-888888888888";
const { SERVICES, GROUPS } = vi.hoisted(() => ({
  SERVICES: {} as Record<string, unknown>,
  GROUPS: {} as Record<string, unknown>,
}));
Object.assign(SERVICES, {
  [SERVICE_ID]: { id: SERVICE_ID, name: "API Gateway", supportGroup: SUPPORT_GROUP },
  [SERVICE_NO_GROUP_ID]: { id: SERVICE_NO_GROUP_ID, name: "Billing", supportGroup: null },
  [OTHER_SERVICE_ID]: { id: OTHER_SERVICE_ID, name: "Storage", supportGroup: OTHER_SUPPORT_GROUP },
});
Object.assign(GROUPS, {
  [SUPPORT_GROUP.id]: { ...SUPPORT_GROUP, active: true },
  [OTHER_SUPPORT_GROUP.id]: { ...OTHER_SUPPORT_GROUP, active: true },
  [PICKED_GROUP.id]: { ...PICKED_GROUP, active: true },
});
vi.mock("@components/AsyncEntitySelect", () => ({
  default: ({
    id,
    label,
    value,
    onChange,
    knownLabel,
    helperText,
    placeholder,
    pinnedOption,
    error,
  }: {
    id: string;
    label: string;
    value: string;
    onChange: (next: string, item?: unknown) => void;
    knownLabel?: string;
    helperText?: ReactNode;
    placeholder?: string;
    pinnedOption?: { id: string; label: string; caption: string } | null;
    error?: boolean;
  }) => {
    const lookup = label === "Service" ? SERVICES : label === "Assignment group" ? GROUPS : {};
    return (
      <div data-testid={id}>
        <input
          aria-label={label}
          value={value}
          placeholder={placeholder}
          data-known-label={knownLabel ?? ""}
          aria-invalid={!!error}
          onChange={(e) =>
            e.target.value ? onChange(e.target.value, lookup[e.target.value]) : onChange("")
          }
        />
        {pinnedOption && (
          <button type="button" onClick={() => onChange(pinnedOption.id)}>
            {`Pinned: ${pinnedOption.label} (${pinnedOption.caption})`}
          </button>
        )}
        <div data-testid={`${id}-helper`}>{helperText}</div>
      </div>
    );
  },
}));
vi.mock("@components/AsyncEntityMultiSelect", () => ({
  default: ({ label }: { label: string }) => <input aria-label={label} readOnly />,
}));

// Imported after the mocks above so the module picks them up.
import CreateIncidentPage from "@features/csm-operations/pages/CreateIncidentPage";

const pickOption = (combobox: RegExp, option: string): void => {
  fireEvent.mouseDown(screen.getByRole("combobox", { name: combobox }));
  fireEvent.click(screen.getByRole("option", { name: option }));
};

const pickService = (serviceId: string): void => {
  fireEvent.change(screen.getByLabelText("Service"), { target: { value: serviceId } });
};

/** Fills every required field except Subcategory. */
const fillRequiredFields = (): void => {
  fireEvent.change(screen.getByLabelText(/short description/i), {
    target: { value: "Gateway returning 502s" },
  });
  pickOption(/^category/i, "Service Interruption");
  pickOption(/^channel/i, "Email");
  pickOption(/^impact/i, "High");
  pickOption(/^urgency/i, "Low");
  pickService(SERVICE_ID);
};

const submitButton = (): HTMLElement => screen.getByRole("button", { name: /create incident/i });

describe("CreateIncidentPage subcategory", () => {
  beforeEach(() => {
    navigateMock.mockReset();
    postIncidentMutateMock.mockReset();
    showErrorMock.mockReset();
  });

  it("creates an incident with no subcategory, omitting the field from the payload", () => {
    render(<CreateIncidentPage />);
    fillRequiredFields();

    expect(submitButton()).not.toBeDisabled();
    fireEvent.click(submitButton());

    expect(postIncidentMutateMock).toHaveBeenCalledTimes(1);
    const [payload] = postIncidentMutateMock.mock.calls[0];
    expect(payload).toEqual({
      subject: "Gateway returning 502s",
      category: "SERVICE_INTERRUPTION",
      serviceId: SERVICE_ID,
      contactType: "EMAIL",
      impact: "HIGH",
      urgency: "LOW",
      callerId: ME_ID,
      assignmentGroupId: SUPPORT_GROUP.id,
    });
    expect(payload).not.toHaveProperty("subcategory");
  });

  it("still sends the subcategory when one is picked", () => {
    render(<CreateIncidentPage />);
    fillRequiredFields();
    pickOption(/^subcategory/i, "Partial Outage");

    fireEvent.click(submitButton());

    expect(postIncidentMutateMock).toHaveBeenCalledTimes(1);
    expect(postIncidentMutateMock.mock.calls[0][0]).toMatchObject({
      category: "SERVICE_INTERRUPTION",
      subcategory: "PARTIAL_OUTAGE",
    });
  });

  it("does not mark Subcategory as required, while Category still is", () => {
    render(<CreateIncidentPage />);
    // MUI's required FormControl renders an (aria-hidden) asterisk span inside
    // the field's InputLabel; renderSelect gives each label id `${key}-label`.
    const asterisk = (labelId: string): Element | null =>
      document.getElementById(labelId)?.querySelector(".MuiFormLabel-asterisk") ?? null;
    expect(asterisk("category-label")).not.toBeNull();
    expect(document.getElementById("subcategory-label")).not.toBeNull();
    expect(asterisk("subcategory-label")).toBeNull();
  });
});

describe("CreateIncidentPage channel", () => {
  beforeEach(() => {
    postIncidentMutateMock.mockReset();
  });

  it("labels the field Channel (required), with no Contact type field left", () => {
    render(<CreateIncidentPage />);
    expect(screen.getByRole("combobox", { name: /^channel/i })).toBeInTheDocument();
    expect(
      document.getElementById("channel-label")?.querySelector(".MuiFormLabel-asterisk"),
    ).not.toBeNull();
    expect(screen.queryByText(/contact type/i)).not.toBeInTheDocument();
  });

  it("sends the picked channel as the wire field contactType", () => {
    render(<CreateIncidentPage />);
    fillRequiredFields();
    // Overrides fillRequiredFields' "Email" with a value whose ServiceNow
    // key isn't its own name (SITE_247 -> "2").
    pickOption(/^channel/i, "Site 24/7");
    fireEvent.click(submitButton());

    expect(postIncidentMutateMock).toHaveBeenCalledTimes(1);
    expect(postIncidentMutateMock.mock.calls[0][0]).toMatchObject({ contactType: "SITE_247" });
  });

  it("keeps Create disabled until a channel is picked", () => {
    render(<CreateIncidentPage />);
    fireEvent.change(screen.getByLabelText(/short description/i), {
      target: { value: "Gateway returning 502s" },
    });
    pickOption(/^category/i, "Service Interruption");
    pickOption(/^impact/i, "High");
    pickOption(/^urgency/i, "Low");
    pickService(SERVICE_ID);
    expect(submitButton()).toBeDisabled();

    pickOption(/^channel/i, "Phone");
    expect(submitButton()).not.toBeDisabled();
  });
});

describe("CreateIncidentPage assignment group", () => {
  const groupField = (): HTMLElement => screen.getByLabelText("Assignment group");
  const groupHelper = (): HTMLElement => screen.getByTestId("incident-assignment-group-helper");
  const pickGroup = (groupId: string): void => {
    fireEvent.change(groupField(), { target: { value: groupId } });
  };
  const expectGroup = (group: { id: string; name: string } | null): void => {
    expect(groupField()).toHaveValue(group?.id ?? "");
    expect(groupField()).toHaveAttribute("data-known-label", group?.name ?? "");
  };
  const submittedPayload = (): Record<string, unknown> => {
    fireEvent.click(submitButton());
    expect(postIncidentMutateMock).toHaveBeenCalledTimes(1);
    return postIncidentMutateMock.mock.calls[0][0];
  };

  beforeEach(() => {
    postIncidentMutateMock.mockReset();
    showErrorMock.mockReset();
    createDefaults.current = { defaultServiceId: "svc-default", defaultGroup: DEFAULT_TEAM };
  });

  it("is an enabled, empty picker before a Service is chosen", () => {
    render(<CreateIncidentPage />);
    expectGroup(null);
    expect(groupField()).not.toBeDisabled();
    expect(groupField()).toHaveAttribute(
      "placeholder",
      "Defaults to the selected service's support group",
    );
    expect(groupHelper()).toHaveTextContent("Defaults to the selected service's support group");
    // Before any Service the default team is what is offered first.
    expect(screen.getByRole("button", { name: "Pinned: Default Team (default team)" })).toBeInTheDocument();
  });

  it("follows the picked Service's support group and sends it", () => {
    render(<CreateIncidentPage />);
    fillRequiredFields();

    expectGroup(SUPPORT_GROUP);
    expect(groupHelper()).toHaveTextContent("Defaults to API Gateway's support group");
    expect(
      screen.getByRole("button", { name: "Pinned: Platform SRE (service's support group)" }),
    ).toBeInTheDocument();

    expect(submittedPayload()).toEqual({
      subject: "Gateway returning 502s",
      category: "SERVICE_INTERRUPTION",
      serviceId: SERVICE_ID,
      contactType: "EMAIL",
      impact: "HIGH",
      urgency: "LOW",
      callerId: ME_ID,
      assignmentGroupId: SUPPORT_GROUP.id,
    });
  });

  it("uses the default team for a Service with no support group", () => {
    render(<CreateIncidentPage />);
    fillRequiredFields();
    pickService(SERVICE_NO_GROUP_ID);

    expectGroup(DEFAULT_TEAM);
    expect(groupHelper()).toHaveTextContent("Billing has no support group; using the default team");
    expect(submittedPayload()).toMatchObject({ assignmentGroupId: DEFAULT_TEAM.id });
  });

  it("stays empty and omits the group when there is no support group and no default team", () => {
    createDefaults.current = { defaultServiceId: null, defaultGroup: null };
    render(<CreateIncidentPage />);
    fillRequiredFields();
    pickService(SERVICE_NO_GROUP_ID);

    expectGroup(null);
    expect(groupHelper()).toHaveTextContent("No support group; pick one or it will be unassigned");
    expect(screen.queryByRole("button", { name: /^Pinned:/ })).not.toBeInTheDocument();
    expect(submittedPayload()).not.toHaveProperty("assignmentGroupId");
  });

  it("omits the group while create-defaults has not loaded and the Service has none", () => {
    createDefaults.current = undefined;
    render(<CreateIncidentPage />);
    fillRequiredFields();
    pickService(SERVICE_NO_GROUP_ID);

    expectGroup(null);
    expect(submittedPayload()).not.toHaveProperty("assignmentGroupId");
  });

  it("follows the Service when it changes, while the group is untouched", () => {
    render(<CreateIncidentPage />);
    fillRequiredFields();
    expectGroup(SUPPORT_GROUP);

    pickService(OTHER_SERVICE_ID);
    expectGroup(OTHER_SUPPORT_GROUP);
    expect(groupHelper()).toHaveTextContent("Defaults to Storage's support group");

    pickService(SERVICE_NO_GROUP_ID);
    expectGroup(DEFAULT_TEAM);
  });

  it("keeps a group the user picked, and offers the Service's own with Use it", () => {
    render(<CreateIncidentPage />);
    fillRequiredFields();
    pickGroup(PICKED_GROUP.id);

    expectGroup(PICKED_GROUP);
    expect(groupHelper()).toHaveTextContent("Service support group: Platform SRE");

    // A Service change does not move a picked group.
    pickService(OTHER_SERVICE_ID);
    expectGroup(PICKED_GROUP);
    expect(groupHelper()).toHaveTextContent("Service support group: Storage Ops");
    expect(submittedPayload()).toMatchObject({ assignmentGroupId: PICKED_GROUP.id });
  });

  it("goes back to following the Service after Use it", () => {
    render(<CreateIncidentPage />);
    fillRequiredFields();
    pickGroup(PICKED_GROUP.id);

    fireEvent.click(within(groupHelper()).getByRole("button", { name: "Use it" }));
    expectGroup(SUPPORT_GROUP);
    expect(groupHelper()).toHaveTextContent("Defaults to API Gateway's support group");

    // Untouched again: the next Service change moves it.
    pickService(OTHER_SERVICE_ID);
    expectGroup(OTHER_SUPPORT_GROUP);
  });

  it("names the default team in the hint when a picked group overrides it", () => {
    render(<CreateIncidentPage />);
    fillRequiredFields();
    pickService(SERVICE_NO_GROUP_ID);
    pickGroup(PICKED_GROUP.id);

    expect(groupHelper()).toHaveTextContent("Default team: Default Team");
    fireEvent.click(within(groupHelper()).getByRole("button", { name: "Use it" }));
    expectGroup(DEFAULT_TEAM);
  });

  it("keeps a group picked before any Service once a Service is chosen", () => {
    render(<CreateIncidentPage />);
    pickGroup(PICKED_GROUP.id);
    expectGroup(PICKED_GROUP);

    fillRequiredFields();
    expectGroup(PICKED_GROUP);
    expect(groupHelper()).toHaveTextContent("Service support group: Platform SRE");
  });

  it("goes back to following the Service when the user clears the group", () => {
    render(<CreateIncidentPage />);
    fillRequiredFields();
    pickGroup(PICKED_GROUP.id);

    pickGroup("");
    expectGroup(SUPPORT_GROUP);
    pickService(OTHER_SERVICE_ID);
    expectGroup(OTHER_SUPPORT_GROUP);
  });

  it("treats picking the pinned support group as following the Service", () => {
    render(<CreateIncidentPage />);
    fillRequiredFields();
    fireEvent.click(
      screen.getByRole("button", { name: "Pinned: Platform SRE (service's support group)" }),
    );

    expectGroup(SUPPORT_GROUP);
    pickService(OTHER_SERVICE_ID);
    expectGroup(OTHER_SUPPORT_GROUP);
  });

  type ErrorCtor = new (
    status: number,
    message: string,
    payload?: { message?: string; errorCode?: string },
  ) => Error;
  const failCreateWith = async (
    status: number,
    message: string,
    payload?: { message?: string; errorCode?: string },
  ): Promise<void> => {
    const { BackendApiError } = await import("@api/backend/client");
    postIncidentMutateMock.mockImplementation(
      (_payload: unknown, opts: { onError: (err: unknown) => void }) =>
        opts.onError(new (BackendApiError as unknown as ErrorCtor)(status, message, payload)),
    );
  };
  const GROUP_REFUSED = "incident_assignment_group_not_allowed";

  it("shows a refused group (by its errorCode) on the field and keeps the form", async () => {
    const words = "The assignment group must be the support group of a service.";
    await failCreateWith(400, words, { message: words, errorCode: GROUP_REFUSED });
    render(<CreateIncidentPage />);
    fillRequiredFields();
    pickGroup(PICKED_GROUP.id);
    fireEvent.click(submitButton());

    expect(groupHelper()).toHaveTextContent(words);
    expect(groupField()).toHaveAttribute("aria-invalid", "true");
    expect(showErrorMock).not.toHaveBeenCalled();
    expectGroup(PICKED_GROUP);
    expect(screen.getByLabelText(/short description/i)).toHaveValue("Gateway returning 502s");
    expect(screen.getByLabelText("Service")).toHaveValue(SERVICE_ID);

    // Picking another group clears the error.
    pickGroup(OTHER_SUPPORT_GROUP.id);
    expect(groupField()).toHaveAttribute("aria-invalid", "false");
  });

  it("falls back to its own words when the refusal carries no message", async () => {
    await failCreateWith(400, "", { errorCode: GROUP_REFUSED });
    render(<CreateIncidentPage />);
    fillRequiredFields();
    fireEvent.click(submitButton());

    expect(groupHelper()).toHaveTextContent(
      "This group can't be assigned. Pick a group listed for a service.",
    );
    expect(groupField()).toHaveAttribute("aria-invalid", "true");
    expect(showErrorMock).not.toHaveBeenCalled();
  });

  it("sends a 400 without the code to the banner, even if it mentions the group", async () => {
    const words = "Invalid assignment group";
    await failCreateWith(400, words, { message: words });
    render(<CreateIncidentPage />);
    fillRequiredFields();
    fireEvent.click(submitButton());

    expect(showErrorMock).toHaveBeenCalledWith(words, expect.anything());
    expect(groupField()).toHaveAttribute("aria-invalid", "false");
    expect(groupHelper()).toHaveTextContent("Defaults to API Gateway's support group");
  });

  it("does not treat the code on a non-400 as a group refusal", async () => {
    await failCreateWith(409, "Conflict", { message: "Conflict", errorCode: GROUP_REFUSED });
    render(<CreateIncidentPage />);
    fillRequiredFields();
    fireEvent.click(submitButton());

    expect(showErrorMock).toHaveBeenCalledWith("Conflict", expect.anything());
    expect(groupField()).toHaveAttribute("aria-invalid", "false");
  });
});
