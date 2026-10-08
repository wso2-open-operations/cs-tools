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
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { clearUserPreferredTimeZone, setUserPreferredTimeZone } from "@utils/dateTime";
import type {
  CloneChangeRequestNavState,
  CreateChangeRequestFromIncidentNavState,
} from "@features/csm-operations/utils/changeRequests";
import type { CreateChangeRequestFromCaseNavState } from "@features/csm-cases/types/csmCases";

const navigateMock = vi.fn();
const postChangeRequestMutateMock = vi.fn();
const patchChangeRequestMutateMock = vi.fn();
const showErrorMock = vi.fn();
const postIsPending = false;
const patchIsPending = false;
let locationState:
  | CloneChangeRequestNavState
  | CreateChangeRequestFromCaseNavState
  | CreateChangeRequestFromIncidentNavState
  | { from?: string }
  | undefined;

vi.mock("react-router", () => ({
  useNavigate: () => navigateMock,
  useLocation: () => ({ state: locationState }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
vi.mock("@features/csm-operations/api/usePostChangeRequest", () => ({
  usePostChangeRequest: () => ({
    mutate: postChangeRequestMutateMock,
    get isPending() {
      return postIsPending;
    },
  }),
}));
vi.mock("@features/csm-operations/api/usePatchChangeRequest", () => ({
  usePatchChangeRequest: () => ({
    mutate: patchChangeRequestMutateMock,
    get isPending() {
      return patchIsPending;
    },
  }),
}));
// The originating service request's detail, which the form infers its project,
// deployment, subject and description from. Undefined = not loaded / none.
let sourceCaseFixture: Record<string, unknown> | undefined;
vi.mock("@features/csm-cases/api/useGetCsmCaseDetail", () => ({
  useGetCsmCaseDetail: (id: string | undefined) => ({
    data: id && sourceCaseFixture && sourceCaseFixture.id === id ? sourceCaseFixture : undefined,
  }),
}));
vi.mock("@features/settings/api/useGetUsersMe", () => ({
  useGetUsersMe: () => ({ data: undefined }),
}));
// CreateChangeRequestPage imports BackendApiError from the real API client
// module, which reads window.config at module load and throws outside a
// configured runtime. Mock it with a real class (so `instanceof` still
// works), mirroring CreateProblemPage.test.tsx.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));
// Every record-reference field on this form (Service / Service offering /
// Configuration item / Assignment group / Assigned to / Requested by /
// Originating service request) is a generic AsyncEntitySelect — out of scope
// to drive through its real search/dropdown interaction here, so it's stubbed
// as a plain labeled input that reports its id straight through onChange,
// same technique as CreateProblemPage.test.tsx.
// A few ids resolve to a named item, as a real search result would, so the
// picked option's display name can be asserted where it matters.
const PICKED_ITEMS: Record<string, { id: string; name: string }> = {};
vi.mock("@components/AsyncEntitySelect", () => ({
  default: ({
    label,
    value,
    knownLabel,
    onChange,
  }: {
    label: string;
    value: string;
    knownLabel?: string;
    onChange: (next: string, item?: { id: string; name: string }) => void;
  }) => (
    <input
      aria-label={label}
      data-known-label={knownLabel ?? ""}
      value={value}
      onChange={(e) => onChange(e.target.value, PICKED_ITEMS[e.target.value])}
    />
  ),
}));
// The Customer Project picker is a generic async project search; stubbed as a
// plain labeled input that reports the typed id (and its display name, as the
// real picker does) straight through onChange.
const PROJECT_NAMES: Record<string, string> = { "proj-a": "Acme Project", "proj-b": "Beta Project" };
vi.mock("@features/csm-cases/components/AsyncProjectSelect", () => ({
  default: ({
    label,
    value,
    onChange,
  }: {
    label: string;
    value: string;
    onChange: (next: string, name?: string) => void;
  }) => (
    <input
      aria-label={label}
      value={value}
      onChange={(e) => onChange(e.target.value, PROJECT_NAMES[e.target.value])}
    />
  ),
}));
// The project -> deployments / deployment products / customer contacts lookup.
// The fake mirrors the real hook's contract: a project's deployments always
// come back; products are known only for the deployments currently chosen; the
// contacts (the read-only Customer Group) are the project's own, per project.
const SCOPE_FIXTURE: Record<
  string,
  Array<{ id: string; label: string; products: Array<{ id: string; label: string }> }>
> = {
  "proj-a": [
    {
      id: "dep-prod",
      label: "Acme Production",
      products: [
        { id: "dp-apim", label: "API Manager 4.3.0" },
        { id: "dp-is", label: "Identity Server 7.0.0" },
      ],
    },
    {
      id: "dep-stg",
      label: "Acme Staging",
      products: [{ id: "dp-apim-stg", label: "API Manager 4.2.0" }],
    },
  ],
  "proj-b": [
    {
      id: "dep-b",
      label: "Beta Development",
      products: [{ id: "dp-b", label: "Choreo 1.0" }],
    },
  ],
  "proj-c": [],
};
const CONTACTS_FIXTURE: Record<string, Array<{ id: string; name: string; email?: string }>> = {
  "proj-a": [
    { id: "pc-1", name: "Alice Aaron", email: "alice@acme.example" },
    { id: "pc-2", name: "Bob Bell" },
  ],
  "proj-b": [{ id: "pc-9", name: "Carol Cook" }],
  "proj-c": [],
};
let scopeLookupError = false;
vi.mock("@features/csm-operations/api/useChangeRequestScopeLookups", () => ({
  useChangeRequestScopeLookups: (projectId: string | undefined, deploymentIds: string[]) => ({
    deployments: (projectId ? (SCOPE_FIXTURE[projectId] ?? []) : []).map((d) => ({
      id: d.id,
      label: d.label,
      products: deploymentIds.includes(d.id) ? d.products : undefined,
    })),
    customerContacts: projectId ? (CONTACTS_FIXTURE[projectId] ?? []) : [],
    contactsReady: !!projectId,
    isLoading: false,
    isError: !!projectId && scopeLookupError,
    refetch: vi.fn(),
  }),
}));
// This form's Lexical-based editor renders real content in a browser but not
// under jsdom in a way vitest can drive reliably — stub it to a plain
// textarea, same technique as EditCaseDetailsDialog.test.tsx.
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({ value, onChange }: { value: string; onChange: (v: string) => void }) => (
    <textarea aria-label="editor" value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));

// Imported after the mocks above so the module picks them up.
import CreateChangeRequestPage from "@features/csm-operations/pages/CreateChangeRequestPage";
import {
  changeRequestDraftKey,
  encodeParentRecordValue,
  saveChangeRequestDraft,
  type ChangeRequestDraft,
} from "@features/csm-operations/utils/changeRequests";

/** Pick one of the three change types in the "What type of change is
 * required?" radio group. */
function selectType(label: "Normal" | "Standard" | "Emergency" = "Normal"): void {
  fireEvent.click(screen.getByRole("radio", { name: new RegExp(`^${label}`, "i") }));
}

/**
 * Fill the fields the form requires (a change type and a subject), so a test
 * can reach the submit path without restating unrelated input for every case.
 */
function fillSubject(type: "Normal" | "Standard" | "Emergency" = "Normal"): void {
  selectType(type);
  fireEvent.change(screen.getByLabelText(/subject/i), {
    target: { value: "Roll out fix to production" },
  });
}

describe("CreateChangeRequestPage — Clone prefill", () => {
  // The page now persists an in-progress draft to sessionStorage (see the
  // "in-progress draft" describe block below) — cleared before every test so
  // one test's edits can never leak into the next via real, unmocked
  // sessionStorage.
  beforeEach(() => {
    sessionStorage.clear();
    navigateMock.mockReset();
    postChangeRequestMutateMock.mockReset();
    patchChangeRequestMutateMock.mockReset();
    showErrorMock.mockReset();
  });

  it("renders a blank form with no clone banner when opened directly (not cloned)", () => {
    locationState = undefined;
    render(<CreateChangeRequestPage />);
    expect(screen.getByLabelText(/subject/i)).toHaveValue("");
    expect(screen.queryByText(/cloned from/i)).not.toBeInTheDocument();
  });

  it("prefills subject, type, and impact from the clone state", () => {
    locationState = {
      sourceNumber: "CHG0009988",
      subject: "Upgrade the gateway cluster",
      type: "emergency",
      impact: "high",
    };
    render(<CreateChangeRequestPage />);
    expect(screen.getByLabelText(/subject/i)).toHaveValue("Upgrade the gateway cluster");
    expect(screen.getByRole("radio", { name: /^emergency/i })).toBeChecked();
    expect(screen.getByText("High")).toBeInTheDocument();
  });

  it("shows a banner naming the source record and the fields that could not be copied", () => {
    locationState = { sourceNumber: "CHG0009988", subject: "Upgrade the gateway cluster" };
    render(<CreateChangeRequestPage />);
    expect(screen.getByText(/cloned from chg0009988/i)).toBeInTheDocument();
    expect(screen.getByText(/priority, implementation plan/i)).toBeInTheDocument();
  });

  it("shows a generic banner when the source number is unavailable", () => {
    locationState = { subject: "Upgrade the gateway cluster" };
    render(<CreateChangeRequestPage />);
    expect(screen.getByText(/cloned from an existing change request/i)).toBeInTheDocument();
  });

  it("never offers a state picker, even when cloning -- every change request starts at New", () => {
    locationState = { subject: "Upgrade the gateway cluster" };
    render(<CreateChangeRequestPage />);
    expect(screen.queryByText("State")).not.toBeInTheDocument();
  });

  it("leaves the planned start/end schedule empty even when cloning", () => {
    locationState = { subject: "Upgrade the gateway cluster" };
    const { container } = render(<CreateChangeRequestPage />);
    // The MUI date-time picker renders a segmented group (day/month/year/…)
    // rather than a single-value input, so "empty" shows up as every segment
    // carrying an `aria-valuetext="Empty"` — there's no cloned start/end
    // date to display, for either the start or the end picker.
    const emptySegments = container.querySelectorAll('[aria-valuetext="Empty"]');
    expect(emptySegments.length).toBeGreaterThan(0);
  });
});

describe("CreateChangeRequestPage — originating service request", () => {
  beforeEach(() => {
    sessionStorage.clear();
    locationState = undefined;
    navigateMock.mockReset();
    postChangeRequestMutateMock.mockReset();
    patchChangeRequestMutateMock.mockReset();
    showErrorMock.mockReset();
  });

  it("renders the originating service request picker in its own callout, visible with no interaction needed", () => {
    render(<CreateChangeRequestPage />);
    expect(screen.getByLabelText(/originating service request/i)).toBeVisible();
  });

  // Regression: this field used to sit at the bottom of a collapsed
  // Accordion, indistinguishable from the other "More options" fields and
  // easy to never see at all. It's now pulled into its own heavier-weight
  // section — a heading of its own, a bordered/tinted panel, and positioned
  // above the Type/Priority/Impact/State row rather than at the very bottom
  // of the form — so it reads as more important than a plain inlined field,
  // not just "no longer collapsed".
  it("gives the originating service request field its own heading and callout, positioned above Priority/Impact, not a same-weight field among the rest of 'More options'", () => {
    render(<CreateChangeRequestPage />);
    const heading = screen.getByText("Originating service request or incident");
    const priorityField = screen.getByLabelText(/^priority$/i);
    // The callout's own heading sits earlier in the DOM than the Priority field —
    // i.e. above the core fields, not buried after them.
    expect(
      heading.compareDocumentPosition(priorityField) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  // Regression: this section used to be a collapsed Accordion the user had
  // to expand before "Assignment group"/"Assigned to"/"Requested by" were
  // reachable at all. They're now rendered inline like every other field
  // (the Originating service request field has since moved to its own
  // callout above, see the test above), so all three must be visible without
  // any click, and there must be no expand/collapse control left behind.
  it("renders the remaining 'More options' fields inline with no collapsed/expandable control", () => {
    render(<CreateChangeRequestPage />);
    expect(screen.getByLabelText(/^assignment group$/i)).toBeVisible();
    expect(screen.getByLabelText(/^assigned to$/i)).toBeVisible();
    expect(screen.getByLabelText(/^requested by$/i)).toBeVisible();
    expect(screen.queryByRole("button", { name: /more options/i })).not.toBeInTheDocument();
  });

  it("does not PATCH when no originating service request was picked — navigates straight to the created change request", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));

    const [, options] = postChangeRequestMutateMock.mock.calls[0];
    options.onSuccess({ changeRequest: { id: "chg-1", number: "CHG0000001" } });

    expect(patchChangeRequestMutateMock).not.toHaveBeenCalled();
    expect(navigateMock).toHaveBeenCalledWith("/operations/change-requests/chg-1", {
      state: { from: "/operations?tab=change_requests" },
    });
  });

  it("PATCHes the created change request with caseId when a service request was picked, then navigates on success", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    fireEvent.change(screen.getByLabelText(/originating service request/i), {
      target: { value: encodeParentRecordValue("service_request", "sr-123") },
    });
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));

    // POST /change-requests never carries caseId — it isn't a create field.
    const [postPayload, postOptions] = postChangeRequestMutateMock.mock.calls[0];
    expect(postPayload).not.toHaveProperty("caseId");

    postOptions.onSuccess({ changeRequest: { id: "chg-1", number: "CHG0000001" } });

    expect(patchChangeRequestMutateMock).toHaveBeenCalledWith(
      { id: "chg-1", patch: { caseId: "sr-123" } },
      expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) }),
    );

    const [, patchOptions] = patchChangeRequestMutateMock.mock.calls[0];
    patchOptions.onSuccess();
    expect(navigateMock).toHaveBeenCalledWith("/operations/change-requests/chg-1", {
      state: { from: "/operations?tab=change_requests" },
    });
  });

  it("still navigates and surfaces a non-silent error when the follow-up PATCH fails", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    fireEvent.change(screen.getByLabelText(/originating service request/i), {
      target: { value: encodeParentRecordValue("service_request", "sr-123") },
    });
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));

    const [, postOptions] = postChangeRequestMutateMock.mock.calls[0];
    postOptions.onSuccess({ changeRequest: { id: "chg-1", number: "CHG0000001" } });

    const [, patchOptions] = patchChangeRequestMutateMock.mock.calls[0];
    patchOptions.onError(new Error("link failed"));

    expect(showErrorMock).toHaveBeenCalledWith(expect.stringContaining("linking it to the originating service request failed"));
    expect(navigateMock).toHaveBeenCalledWith("/operations/change-requests/chg-1", {
      state: { from: "/operations?tab=change_requests" },
    });
  });

  it("surfaces a create-mutation error via the shared error banner", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));
    const [, options] = postChangeRequestMutateMock.mock.calls[0];
    options.onError(new Error("network down"));
    expect(showErrorMock).toHaveBeenCalledWith(
      "Could not create the change request. Please try again.",
      expect.any(Error),
    );
  });
});

// Regression tests: Back/Cancel used to always navigate to the hardcoded
// change-requests tab and never forward a return path to the newly created
// record, unlike its 4 sibling create pages (case/service request/
// engagement/security report).
describe("CreateChangeRequestPage — Back navigation", () => {
  beforeEach(() => {
    sessionStorage.clear();
    locationState = undefined;
    navigateMock.mockReset();
    postChangeRequestMutateMock.mockReset();
    patchChangeRequestMutateMock.mockReset();
    showErrorMock.mockReset();
  });

  it("falls back to the change-requests tab when opened with no origin", () => {
    render(<CreateChangeRequestPage />);
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(navigateMock).toHaveBeenCalledWith("/operations?tab=change_requests");
  });

  it("returns to the captured origin, and forwards it to the newly created change request, when one is known", () => {
    locationState = { from: "/customers/projects/proj-1?tab=workItems" };
    render(<CreateChangeRequestPage />);

    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(navigateMock).toHaveBeenCalledWith("/customers/projects/proj-1?tab=workItems");

    fillSubject();
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));
    const [, options] = postChangeRequestMutateMock.mock.calls[0];
    options.onSuccess({ changeRequest: { id: "chg-1", number: "CHG0000001" } });

    expect(navigateMock).toHaveBeenCalledWith("/operations/change-requests/chg-1", {
      state: { from: "/customers/projects/proj-1?tab=workItems" },
    });
  });
});

describe("CreateChangeRequestPage — opened from a service request's own 'Create change request…' action", () => {
  beforeEach(() => {
    sessionStorage.clear();
    navigateMock.mockReset();
    postChangeRequestMutateMock.mockReset();
    patchChangeRequestMutateMock.mockReset();
    showErrorMock.mockReset();
  });

  it("pre-selects the originating service request and surfaces a banner naming it", () => {
    locationState = {
      caseId: "sr-789",
      caseNumber: "CS-4321",
      caseSubject: "Cluster is unresponsive",
      projectId: "prj-1",
    };
    render(<CreateChangeRequestPage />);
    expect(screen.getByLabelText(/originating service request/i)).toHaveValue(
      encodeParentRecordValue("service_request", "sr-789"),
    );
    expect(screen.getByText(/linking to cs-4321/i)).toBeInTheDocument();
  });

  it("still lets the pre-selected service request be changed or cleared", () => {
    locationState = { caseId: "sr-789", caseNumber: "CS-4321" };
    render(<CreateChangeRequestPage />);
    fireEvent.change(screen.getByLabelText(/originating service request/i), {
      target: { value: "" },
    });
    expect(screen.getByLabelText(/originating service request/i)).toHaveValue("");
  });

  it("PATCHes the created change request with the pre-selected caseId on submit", () => {
    locationState = { caseId: "sr-789", caseNumber: "CS-4321" };
    render(<CreateChangeRequestPage />);
    fillSubject();
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));

    const [, postOptions] = postChangeRequestMutateMock.mock.calls[0];
    postOptions.onSuccess({ changeRequest: { id: "chg-2", number: "CHG0000002" } });

    expect(patchChangeRequestMutateMock).toHaveBeenCalledWith(
      { id: "chg-2", patch: { caseId: "sr-789" } },
      expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) }),
    );
  });

  it("shows no clone banner and no service-request banner when opened directly", () => {
    locationState = undefined;
    render(<CreateChangeRequestPage />);
    expect(screen.queryByText(/cloned from/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/linking to/i)).not.toBeInTheDocument();
    expect(screen.getByLabelText(/originating service request/i)).toHaveValue("");
  });
});

// Coverage for the unified service-request/incident picker's backend
// constraint: `PATCH /change-requests/{id} { caseId }` only ever resolves
// against the case table (see `changeRequests.ts`'s "Originating service
// request picker" section) — an incident can be *found* by the picker, but
// selecting one must block submit entirely rather than let the create-then-
// PATCH flow 404.
describe("CreateChangeRequestPage — incident selected as the parent record is gated", () => {
  beforeEach(() => {
    sessionStorage.clear();
    locationState = undefined;
    navigateMock.mockReset();
    postChangeRequestMutateMock.mockReset();
    patchChangeRequestMutateMock.mockReset();
    showErrorMock.mockReset();
  });

  it("disables the Create button and shows an inline warning once an incident is picked from the unified search", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    expect(screen.getByRole("button", { name: /create change request/i })).toBeEnabled();

    fireEvent.change(screen.getByLabelText(/originating service request/i), {
      target: { value: encodeParentRecordValue("incident", "inc-1") },
    });

    expect(
      screen.getByText(/linking a change request directly to an incident isn't available yet/i),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /create change request/i })).toBeDisabled();
  });

  it("re-enables the Create button once the incident selection is cleared", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    fireEvent.change(screen.getByLabelText(/originating service request/i), {
      target: { value: encodeParentRecordValue("incident", "inc-1") },
    });
    expect(screen.getByRole("button", { name: /create change request/i })).toBeDisabled();

    fireEvent.change(screen.getByLabelText(/originating service request/i), {
      target: { value: "" },
    });

    expect(screen.getByRole("button", { name: /create change request/i })).toBeEnabled();
    expect(postChangeRequestMutateMock).not.toHaveBeenCalled();
  });

  it("never calls the create mutation while an incident parent is selected, even if Create is clicked", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    fireEvent.change(screen.getByLabelText(/originating service request/i), {
      target: { value: encodeParentRecordValue("incident", "inc-1") },
    });
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));

    expect(postChangeRequestMutateMock).not.toHaveBeenCalled();
  });
});

describe("CreateChangeRequestPage — opened from an incident's own 'Create change request…' action", () => {
  beforeEach(() => {
    sessionStorage.clear();
    navigateMock.mockReset();
    postChangeRequestMutateMock.mockReset();
    patchChangeRequestMutateMock.mockReset();
    showErrorMock.mockReset();
  });

  it("pre-selects the incident, surfaces a gating banner, and disables Create", () => {
    locationState = {
      incidentId: "inc-1",
      incidentNumber: "INC0012345",
      incidentSubject: "Gateway 502s",
    };
    render(<CreateChangeRequestPage />);
    fillSubject();

    expect(screen.getByLabelText(/originating service request/i)).toHaveValue(
      encodeParentRecordValue("incident", "inc-1"),
    );
    expect(screen.getByText(/opened from inc0012345/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /create change request/i })).toBeDisabled();
  });

  it("lets the pre-selected incident be replaced with a service request, which un-gates submit", () => {
    locationState = { incidentId: "inc-1", incidentNumber: "INC0012345" };
    render(<CreateChangeRequestPage />);
    fillSubject();
    fireEvent.change(screen.getByLabelText(/originating service request/i), {
      target: { value: encodeParentRecordValue("service_request", "sr-1") },
    });

    expect(screen.getByRole("button", { name: /create change request/i })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));

    const [, postOptions] = postChangeRequestMutateMock.mock.calls[0];
    postOptions.onSuccess({ changeRequest: { id: "chg-3", number: "CHG0000003" } });

    expect(patchChangeRequestMutateMock).toHaveBeenCalledWith(
      { id: "chg-3", patch: { caseId: "sr-1" } },
      expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) }),
    );
  });
});

// Regression: this route unmounts whenever the user navigates to another
// operations tab, and remounts fresh on the way back — it used to re-seed
// every field from the original clone/service-request/incident source again,
// silently discarding anything the user had typed in between. `unmount()`
// followed by a second `render()` with the same `locationState` simulates
// exactly that: a real remount, not just a re-render of the same instance.
describe("CreateChangeRequestPage — in-progress draft survives navigating away and back", () => {
  beforeEach(() => {
    sessionStorage.clear();
    navigateMock.mockReset();
    postChangeRequestMutateMock.mockReset();
    patchChangeRequestMutateMock.mockReset();
    showErrorMock.mockReset();
  });

  it("restores an edited subject after unmount/remount for the same clone source", () => {
    locationState = { sourceNumber: "CHG0009988", subject: "Original subject" };
    const { unmount } = render(<CreateChangeRequestPage />);
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "Edited subject" },
    });
    unmount();

    render(<CreateChangeRequestPage />);
    expect(screen.getByLabelText(/subject/i)).toHaveValue("Edited subject");
  });

  it("restores an edited rich-text field (e.g. Description) after unmount/remount", () => {
    locationState = { sourceNumber: "CHG0009988", subject: "Original subject" };
    const { unmount } = render(<CreateChangeRequestPage />);
    // Every rich-text Planning field uses the same stubbed Editor (see the
    // module mock above), so all six render with the same aria-label —
    // Description is the first in DOM order.
    fireEvent.change(screen.getAllByLabelText("editor")[0], {
      target: { value: "<p>In-progress plan notes</p>" },
    });
    unmount();

    render(<CreateChangeRequestPage />);
    expect(screen.getAllByLabelText("editor")[0]).toHaveValue("<p>In-progress plan notes</p>");
  });

  it("never restores a draft into a different clone source's form", () => {
    locationState = { sourceNumber: "CHG0009988", subject: "Original subject A" };
    const { unmount } = render(<CreateChangeRequestPage />);
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "Edited subject for A" },
    });
    unmount();

    // A different source record, cloned next — its own subject must win, not
    // the draft left over from A.
    locationState = { sourceNumber: "CHG0011111", subject: "Original subject B" };
    render(<CreateChangeRequestPage />);
    expect(screen.getByLabelText(/subject/i)).toHaveValue("Original subject B");
  });

  it("never leaks a draft between the from-scratch path and an unrelated originating service request", () => {
    locationState = undefined;
    const { unmount } = render(<CreateChangeRequestPage />);
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "From-scratch draft" },
    });
    unmount();

    locationState = { caseId: "sr-789", caseNumber: "CS-4321" };
    render(<CreateChangeRequestPage />);
    expect(screen.getByLabelText(/subject/i)).toHaveValue("");
  });

  it("clears the draft once the change request is created, so re-opening the same clone source starts clean", () => {
    locationState = { sourceNumber: "CHG0009988", subject: "Original subject" };
    const { unmount } = render(<CreateChangeRequestPage />);
    selectType("Normal");
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "Edited subject" },
    });
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));
    const [, options] = postChangeRequestMutateMock.mock.calls[0];
    options.onSuccess({ changeRequest: { id: "chg-1", number: "CHG0000001" } });
    unmount();

    render(<CreateChangeRequestPage />);
    expect(screen.getByLabelText(/subject/i)).toHaveValue("Original subject");
  });

  it("clears the draft when Back is clicked, so returning starts clean", () => {
    locationState = { sourceNumber: "CHG0009988", subject: "Original subject" };
    const { unmount } = render(<CreateChangeRequestPage />);
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "Edited subject" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    unmount();

    render(<CreateChangeRequestPage />);
    expect(screen.getByLabelText(/subject/i)).toHaveValue("Original subject");
  });

  it("clears the draft when Cancel is clicked, so returning starts clean", () => {
    locationState = { sourceNumber: "CHG0009988", subject: "Original subject" };
    const { unmount } = render(<CreateChangeRequestPage />);
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "Edited subject" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    unmount();

    render(<CreateChangeRequestPage />);
    expect(screen.getByLabelText(/subject/i)).toHaveValue("Original subject");
  });
});

describe("CreateChangeRequestPage — planned dates are sent as UTC", () => {
  beforeEach(() => {
    sessionStorage.clear();
    navigateMock.mockReset();
    postChangeRequestMutateMock.mockReset();
    patchChangeRequestMutateMock.mockReset();
    showErrorMock.mockReset();
    locationState = undefined;
  });

  afterEach(() => {
    clearUserPreferredTimeZone();
  });

  it("converts the picker's wall-clock values from the user's time zone to UTC", () => {
    // Asia/Colombo is UTC+05:30 with no daylight saving.
    setUserPreferredTimeZone("Asia/Colombo");
    const draft: ChangeRequestDraft = {
      subject: "Roll out fix to production",
      type: "normal",
      impact: "low",
      priority: "",
      plannedStartDate: "2030-03-01T15:30",
      plannedEndDate: "2030-03-01T17:30",
      description: "",
      justification: "",
      implementationPlan: "",
      riskImpactAnalysis: "",
      backoutPlan: "",
      testPlan: "",
      isPlanningVisibleToCustomers: false,
      groupId: "",
      assignedEngineerId: "",
      requestedById: "",
      parentValue: "",
    };
    saveChangeRequestDraft(changeRequestDraftKey({ kind: "new" }), draft);

    render(<CreateChangeRequestPage />);
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));

    const [payload] = postChangeRequestMutateMock.mock.calls[0];
    expect(payload.plannedStartDate).toBe("2030-03-01 10:00:00");
    expect(payload.plannedEndDate).toBe("2030-03-01 12:00:00");
  });
});

describe("CreateChangeRequestPage — the 'in the past' hint follows the profile time zone", () => {
  const PAST_HINT = /this date is in the past/i;

  function seedDraft(plannedStartDate: string, plannedEndDate: string): void {
    saveChangeRequestDraft(changeRequestDraftKey({ kind: "new" }), {
      subject: "Roll out fix to production",
      type: "normal",
      impact: "low",
      priority: "",
      plannedStartDate,
      plannedEndDate,
      description: "",
      justification: "",
      implementationPlan: "",
      riskImpactAnalysis: "",
      backoutPlan: "",
      testPlan: "",
      isPlanningVisibleToCustomers: false,
      groupId: "",
      assignedEngineerId: "",
      requestedById: "",
      parentValue: "",
    });
  }

  beforeAll(() => {
    // Pin the browser zone so it differs from the profile zone under test.
    vi.stubEnv("TZ", "UTC");
  });

  afterAll(() => {
    vi.unstubAllEnvs();
  });

  beforeEach(() => {
    sessionStorage.clear();
    locationState = undefined;
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2030-03-01T12:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
    clearUserPreferredTimeZone();
  });

  it("warns when the value is past in the profile zone but would look future in the browser zone", () => {
    setUserPreferredTimeZone("Asia/Colombo");
    // 15:30 Colombo is 10:00Z (past); as browser (UTC) digits it is 15:30Z (future).
    seedDraft("2030-03-01T15:30", "2030-03-01T15:30");
    render(<CreateChangeRequestPage />);
    expect(screen.getAllByText(PAST_HINT)).toHaveLength(2);
  });

  it("does not warn when the value is future in the profile zone but would look past in the browser zone", () => {
    setUserPreferredTimeZone("America/Los_Angeles");
    // 08:00 Los Angeles is 16:00Z (future); as browser (UTC) digits it is 08:00Z (past).
    seedDraft("2030-03-01T08:00", "2030-03-01T09:00");
    render(<CreateChangeRequestPage />);
    expect(screen.queryByText(PAST_HINT)).not.toBeInTheDocument();
  });
});

describe("CreateChangeRequestPage — required change type (Normal / Standard / Emergency)", () => {
  beforeEach(() => {
    sessionStorage.clear();
    locationState = undefined;
    navigateMock.mockReset();
    postChangeRequestMutateMock.mockReset();
    patchChangeRequestMutateMock.mockReset();
    showErrorMock.mockReset();
  });

  it("opens the form with the type question first, offering exactly Normal, Standard, Emergency in that order", () => {
    render(<CreateChangeRequestPage />);
    const group = screen.getByRole("group", { name: /what type of change is required/i });
    const radios = within(group).getAllByRole("radio");
    expect(radios.map((r) => (r as HTMLInputElement).value)).toEqual(["normal", "standard", "emergency"]);
    // The type question comes before the Subject field.
    expect(
      group.compareDocumentPosition(screen.getByLabelText(/subject/i)) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    // No Model / Site reliability ops / Azure leftovers anywhere.
    expect(screen.queryByText(/^azure$/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/site reliability/i)).not.toBeInTheDocument();
  });

  it("shows each type's short description beside it", () => {
    render(<CreateChangeRequestPage />);
    expect(screen.getByText(/general purpose change type that requires one or more approvals/i)).toBeInTheDocument();
    expect(screen.getByText(/do not require approval/i)).toBeInTheDocument();
    expect(screen.getByText(/must be implemented as soon as possible/i)).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /^normal/i })).toHaveAccessibleDescription(
      /requires one or more approvals/i,
    );
  });

  it("starts with no type selected and blocks Create, even with a subject filled in", () => {
    render(<CreateChangeRequestPage />);
    screen.getAllByRole("radio").forEach((r) => expect(r).not.toBeChecked());
    fireEvent.change(screen.getByLabelText(/subject/i), { target: { value: "Roll out fix" } });
    const create = screen.getByRole("button", { name: /create change request/i });
    expect(create).toBeDisabled();
    expect(screen.getByText(/select a change type to continue/i)).toBeInTheDocument();
    fireEvent.click(create);
    expect(postChangeRequestMutateMock).not.toHaveBeenCalled();
  });

  it("blocks Create when only a type is chosen but the subject is blank", () => {
    render(<CreateChangeRequestPage />);
    selectType("Standard");
    expect(screen.getByRole("button", { name: /create change request/i })).toBeDisabled();
  });

  it("enables Create once a type and a subject are given, and clears the 'select a type' prompt", () => {
    render(<CreateChangeRequestPage />);
    fillSubject("Emergency");
    expect(screen.getByRole("button", { name: /create change request/i })).toBeEnabled();
    expect(screen.queryByText(/select a change type to continue/i)).not.toBeInTheDocument();
  });

  it.each([
    ["Normal", "normal"],
    ["Standard", "standard"],
    ["Emergency", "emergency"],
  ] as const)("sends type=%s as the backend enum value '%s' in the create payload", (label, value) => {
    render(<CreateChangeRequestPage />);
    fillSubject(label);
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));
    expect(postChangeRequestMutateMock).toHaveBeenCalledTimes(1);
    expect(postChangeRequestMutateMock.mock.calls[0]![0]).toEqual(
      expect.objectContaining({ subject: "Roll out fix to production", type: value }),
    );
  });

  it("sends the changed type when the selection is switched before submitting", () => {
    render(<CreateChangeRequestPage />);
    fillSubject("Normal");
    selectType("Emergency");
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));
    expect(postChangeRequestMutateMock.mock.calls[0]![0]).toEqual(
      expect.objectContaining({ type: "emergency" }),
    );
  });

  it("surfaces the backend's 400 'type is required' message verbatim if it ever slips through", async () => {
    const { BackendApiError } = await import("@api/backend/client");
    render(<CreateChangeRequestPage />);
    fillSubject("Standard");
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));
    const [, options] = postChangeRequestMutateMock.mock.calls[0];
    const message = "type is required: a change request must be one of standard, normal or emergency";
    const err = new (BackendApiError as unknown as new (s: number, m: string) => Error)(400, message);
    options.onError(err);
    expect(showErrorMock).toHaveBeenCalledWith(message, err);
  });

  it("pre-selects a cloned change's type when it is one of the three", () => {
    locationState = { subject: "Clone me", type: "standard" };
    render(<CreateChangeRequestPage />);
    expect(screen.getByRole("radio", { name: /^standard/i })).toBeChecked();
    expect(screen.getByRole("button", { name: /create change request/i })).toBeEnabled();
  });

  it("leaves the type unselected when a clone's source type is not one of the three (e.g. azure)", () => {
    locationState = { subject: "Clone me", type: "azure" };
    render(<CreateChangeRequestPage />);
    screen.getAllByRole("radio").forEach((r) => expect(r).not.toBeChecked());
    expect(screen.getByRole("button", { name: /create change request/i })).toBeDisabled();
  });

  it("restores the chosen type from an in-progress draft", () => {
    locationState = undefined;
    const { unmount } = render(<CreateChangeRequestPage />);
    selectType("Emergency");
    unmount();
    render(<CreateChangeRequestPage />);
    expect(screen.getByRole("radio", { name: /^emergency/i })).toBeChecked();
  });
});

describe("CreateChangeRequestPage — Customer Approval / Customer Review checkboxes", () => {
  beforeEach(() => {
    sessionStorage.clear();
    locationState = undefined;
    navigateMock.mockReset();
    postChangeRequestMutateMock.mockReset();
    patchChangeRequestMutateMock.mockReset();
    showErrorMock.mockReset();
  });

  function submittedPayload(): Record<string, unknown> {
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));
    expect(postChangeRequestMutateMock).toHaveBeenCalledTimes(1);
    return postChangeRequestMutateMock.mock.calls[0]![0] as Record<string, unknown>;
  }

  it("renders both as real checkboxes (not switches), unchecked by default", () => {
    render(<CreateChangeRequestPage />);
    const approval = screen.getByRole("checkbox", { name: "Customer Approval" });
    const review = screen.getByRole("checkbox", { name: "Customer Review" });
    expect(approval).not.toBeChecked();
    expect(review).not.toBeChecked();
    // A MUI Switch would expose role="switch"; these must not.
    expect(screen.queryByRole("switch", { name: /customer (approval|review)/i })).not.toBeInTheDocument();
    expect((approval as HTMLInputElement).type).toBe("checkbox");
    expect((review as HTMLInputElement).type).toBe("checkbox");
  });

  it("shows each one's helper line", () => {
    render(<CreateChangeRequestPage />);
    expect(
      screen.getByText("Adds a customer approval step after internal approval, before scheduling."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Adds a customer review step after Review, before closing."),
    ).toBeInTheDocument();
    // The helper text describes its checkbox for assistive tech.
    expect(screen.getByRole("checkbox", { name: "Customer Approval" })).toHaveAccessibleDescription(
      /customer approval step/i,
    );
  });

  it("always sends both flags, false/false when neither is checked", () => {
    render(<CreateChangeRequestPage />);
    fillSubject("Normal");
    const payload = submittedPayload();
    expect(payload).toHaveProperty("customerApprovalRequired", false);
    expect(payload).toHaveProperty("customerReviewRequired", false);
  });

  it.each([
    [true, false],
    [false, true],
    [true, true],
  ])("sends customerApprovalRequired=%s and customerReviewRequired=%s", (approval, review) => {
    render(<CreateChangeRequestPage />);
    fillSubject("Normal");
    if (approval) fireEvent.click(screen.getByRole("checkbox", { name: "Customer Approval" }));
    if (review) fireEvent.click(screen.getByRole("checkbox", { name: "Customer Review" }));
    const payload = submittedPayload();
    expect(payload.customerApprovalRequired).toBe(approval);
    expect(payload.customerReviewRequired).toBe(review);
  });

  it("sits with Priority and Impact, above the Planning section", () => {
    render(<CreateChangeRequestPage />);
    const approval = screen.getByRole("checkbox", { name: "Customer Approval" });
    expect(
      screen.getByText("Planning").compareDocumentPosition(approval) & Node.DOCUMENT_POSITION_PRECEDING,
    ).toBeTruthy();
    expect(
      screen.getByRole("combobox", { name: /impact/i }).compareDocumentPosition(approval) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("restores both checked boxes from an in-progress draft after unmount/remount", () => {
    const { unmount } = render(<CreateChangeRequestPage />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Customer Approval" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Customer Review" }));
    unmount();
    render(<CreateChangeRequestPage />);
    expect(screen.getByRole("checkbox", { name: "Customer Approval" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Customer Review" })).toBeChecked();
  });

  it("starts unchecked from a draft saved before these checkboxes existed", () => {
    const legacyDraft: ChangeRequestDraft = {
      subject: "Old draft",
      type: "normal",
      impact: "low",
      priority: "",
      plannedStartDate: "",
      plannedEndDate: "",
      description: "",
      justification: "",
      implementationPlan: "",
      riskImpactAnalysis: "",
      backoutPlan: "",
      testPlan: "",
      isPlanningVisibleToCustomers: false,
      groupId: "",
      assignedEngineerId: "",
      requestedById: "",
      parentValue: "",
    };
    saveChangeRequestDraft(changeRequestDraftKey({ kind: "new" }), legacyDraft);
    render(<CreateChangeRequestPage />);
    expect(screen.getByRole("checkbox", { name: "Customer Approval" })).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Customer Review" })).not.toBeChecked();
  });

  it("pre-checks the boxes a clone's source had, and sends them", () => {
    locationState = {
      sourceNumber: "CHG0009988",
      subject: "Clone me",
      type: "normal",
      customerApprovalRequired: true,
      customerReviewRequired: false,
    };
    render(<CreateChangeRequestPage />);
    expect(screen.getByRole("checkbox", { name: "Customer Approval" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Customer Review" })).not.toBeChecked();
    const payload = submittedPayload();
    expect(payload.customerApprovalRequired).toBe(true);
    expect(payload.customerReviewRequired).toBe(false);
  });
});

describe("CreateChangeRequestPage — an Emergency change proceeds without customer approval or review", () => {
  const EMERGENCY_LINE = "Emergency changes proceed without customer approval or review.";

  beforeEach(() => {
    sessionStorage.clear();
    locationState = undefined;
    navigateMock.mockReset();
    postChangeRequestMutateMock.mockReset();
    patchChangeRequestMutateMock.mockReset();
    showErrorMock.mockReset();
  });

  function submittedPayload(): Record<string, unknown> {
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));
    expect(postChangeRequestMutateMock).toHaveBeenCalledTimes(1);
    return postChangeRequestMutateMock.mock.calls[0]![0] as Record<string, unknown>;
  }

  it("disables both boxes, unticked, and says why in one line, as soon as Emergency is chosen", () => {
    render(<CreateChangeRequestPage />);
    // Nothing chosen yet: the boxes are live and the line is not there.
    expect(screen.getByRole("checkbox", { name: "Customer Approval" })).toBeEnabled();
    expect(screen.queryByText(EMERGENCY_LINE)).not.toBeInTheDocument();

    selectType("Emergency");

    const approval = screen.getByRole("checkbox", { name: "Customer Approval" });
    const review = screen.getByRole("checkbox", { name: "Customer Review" });
    expect(approval).toBeDisabled();
    expect(review).toBeDisabled();
    expect(approval).not.toBeChecked();
    expect(review).not.toBeChecked();
    expect(screen.getAllByText(EMERGENCY_LINE)).toHaveLength(1);
    // The line describes the boxes for assistive tech, beside each box's own helper.
    expect(approval).toHaveAccessibleDescription(new RegExp(EMERGENCY_LINE.replace(/\./g, "\\.")));
    expect(review).toHaveAccessibleDescription(new RegExp(EMERGENCY_LINE.replace(/\./g, "\\.")));
  });

  it("clears boxes that were ticked when the type is switched to Emergency", () => {
    render(<CreateChangeRequestPage />);
    selectType("Normal");
    fireEvent.click(screen.getByRole("checkbox", { name: "Customer Approval" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Customer Review" }));
    expect(screen.getByRole("checkbox", { name: "Customer Approval" })).toBeChecked();

    selectType("Emergency");
    expect(screen.getByRole("checkbox", { name: "Customer Approval" })).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Customer Review" })).not.toBeChecked();
  });

  it("leaves them off, and tickable again, when the type is switched away from Emergency", () => {
    render(<CreateChangeRequestPage />);
    selectType("Normal");
    fireEvent.click(screen.getByRole("checkbox", { name: "Customer Approval" }));
    selectType("Emergency");
    selectType("Normal");

    const approval = screen.getByRole("checkbox", { name: "Customer Approval" });
    const review = screen.getByRole("checkbox", { name: "Customer Review" });
    expect(approval).toBeEnabled();
    expect(review).toBeEnabled();
    expect(approval).not.toBeChecked();
    expect(review).not.toBeChecked();
    expect(screen.queryByText(EMERGENCY_LINE)).not.toBeInTheDocument();
    fireEvent.click(approval);
    expect(approval).toBeChecked();
  });

  it("sends an Emergency change with both flags false", () => {
    render(<CreateChangeRequestPage />);
    fillSubject("Emergency");
    const payload = submittedPayload();
    expect(payload).toHaveProperty("type", "emergency");
    expect(payload).toHaveProperty("customerApprovalRequired", false);
    expect(payload).toHaveProperty("customerReviewRequired", false);
  });

  it("sends false even after boxes were ticked on another type first", () => {
    render(<CreateChangeRequestPage />);
    fillSubject("Normal");
    fireEvent.click(screen.getByRole("checkbox", { name: "Customer Approval" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Customer Review" }));
    selectType("Emergency");
    const payload = submittedPayload();
    expect(payload.customerApprovalRequired).toBe(false);
    expect(payload.customerReviewRequired).toBe(false);
  });

  it("does not let a click tick a disabled box", () => {
    render(<CreateChangeRequestPage />);
    selectType("Emergency");
    fireEvent.click(screen.getByRole("checkbox", { name: "Customer Approval" }));
    expect(screen.getByRole("checkbox", { name: "Customer Approval" })).not.toBeChecked();
  });

  it("clones an Emergency change with both boxes off, whatever its source had ticked", () => {
    locationState = {
      sourceNumber: "CHG0009988",
      subject: "Clone me",
      type: "emergency",
      customerApprovalRequired: true,
      customerReviewRequired: true,
    };
    render(<CreateChangeRequestPage />);
    expect(screen.getByRole("radio", { name: /^emergency/i })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Customer Approval" })).toBeDisabled();
    expect(screen.getByRole("checkbox", { name: "Customer Approval" })).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Customer Review" })).not.toBeChecked();
    const payload = submittedPayload();
    expect(payload.customerApprovalRequired).toBe(false);
    expect(payload.customerReviewRequired).toBe(false);
  });

  it("restores an Emergency draft saved with ticked boxes with both boxes off", () => {
    const draft: ChangeRequestDraft = {
      subject: "Drafted before the rule",
      type: "emergency",
      impact: "low",
      priority: "",
      plannedStartDate: "",
      plannedEndDate: "",
      description: "",
      justification: "",
      implementationPlan: "",
      riskImpactAnalysis: "",
      backoutPlan: "",
      testPlan: "",
      isPlanningVisibleToCustomers: false,
      customerApprovalRequired: true,
      customerReviewRequired: true,
      groupId: "",
      assignedEngineerId: "",
      requestedById: "",
      parentValue: "",
    };
    saveChangeRequestDraft(changeRequestDraftKey({ kind: "new" }), draft);
    render(<CreateChangeRequestPage />);
    expect(screen.getByRole("checkbox", { name: "Customer Approval" })).toBeDisabled();
    expect(screen.getByRole("checkbox", { name: "Customer Approval" })).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Customer Review" })).not.toBeChecked();
    expect(screen.getAllByText(EMERGENCY_LINE)).toHaveLength(1);
  });

  it("keeps Normal and Standard as before: both boxes live, and ticked ones are sent", () => {
    for (const type of ["Normal", "Standard"] as const) {
      postChangeRequestMutateMock.mockReset();
      const { unmount } = render(<CreateChangeRequestPage />);
      fillSubject(type);
      expect(screen.getByRole("checkbox", { name: "Customer Approval" })).toBeEnabled();
      expect(screen.queryByText(EMERGENCY_LINE)).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole("checkbox", { name: "Customer Review" }));
      const payload = submittedPayload();
      expect(payload.customerApprovalRequired, type).toBe(false);
      expect(payload.customerReviewRequired, type).toBe(true);
      unmount();
      sessionStorage.clear();
    }
  });
});

describe("CreateChangeRequestPage — customer project, deployments, deployment products, customer group", () => {
  beforeEach(() => {
    sessionStorage.clear();
    locationState = undefined;
    scopeLookupError = false;
    navigateMock.mockReset();
    postChangeRequestMutateMock.mockReset();
    patchChangeRequestMutateMock.mockReset();
    showErrorMock.mockReset();
  });

  function pickProject(id: string): void {
    fireEvent.change(screen.getByLabelText("Customer Project"), { target: { value: id } });
  }

  /** Opens a multi-select and ticks the named options. */
  function pickOptions(field: string, names: string[]): void {
    const input = screen.getByRole("combobox", { name: field });
    fireEvent.mouseDown(input);
    for (const name of names) fireEvent.click(screen.getByRole("option", { name }));
    fireEvent.keyDown(input, { key: "Escape" });
  }

  /** The names an open multi-select offers. */
  function offered(field: string): string[] {
    const input = screen.getByRole("combobox", { name: field });
    fireEvent.mouseDown(input);
    const names = screen.queryAllByRole("option").map((o) => o.textContent ?? "");
    fireEvent.keyDown(input, { key: "Escape" });
    return names;
  }

  /** The names currently selected in a multi-select, read off its chips. */
  function chips(field: string): string[] {
    const root = screen.getByRole("combobox", { name: field }).closest(".MuiInputBase-root");
    return Array.from(root?.querySelectorAll(".MuiChip-label") ?? []).map((c) => c.textContent ?? "");
  }

  function productChips(): string[] {
    const root = screen.getByLabelText("Deployment products").closest(".MuiInputBase-root");
    return Array.from(root?.querySelectorAll(".MuiChip-label") ?? []).map((c) => c.textContent ?? "");
  }

  function submittedPayload(): Record<string, unknown> {
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));
    expect(postChangeRequestMutateMock).toHaveBeenCalledTimes(1);
    return postChangeRequestMutateMock.mock.calls[0]![0] as Record<string, unknown>;
  }

  function groupChips(): string[] {
    const root = screen.getByLabelText("Customer Group").closest(".MuiInputBase-root");
    return Array.from(root?.querySelectorAll(".MuiChip-label") ?? []).map((c) => c.textContent ?? "");
  }

  it("lays the fields out like ServiceNow: Customer Project below Priority/Impact, then Deployments, Deployment products, Customer Group", () => {
    render(<CreateChangeRequestPage />);
    const order = [
      screen.getByRole("combobox", { name: /impact/i }),
      screen.getByLabelText("Customer Project"),
      screen.getByRole("combobox", { name: "Deployments" }),
      screen.getByLabelText("Deployment products"),
      screen.getByLabelText("Customer Group"),
      screen.getByRole("combobox", { name: "Category" }),
      screen.getByRole("checkbox", { name: "Customer Approval" }),
    ];
    for (let i = 0; i < order.length - 1; i++) {
      expect(order[i]!.compareDocumentPosition(order[i + 1]!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    }
  });

  it("has no Environments field: a deployment carries its own environment", () => {
    render(<CreateChangeRequestPage />);
    expect(screen.queryByRole("combobox", { name: "Environments" })).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Environments")).not.toBeInTheDocument();
  });

  it("keeps Deployments disabled until a project is chosen", () => {
    render(<CreateChangeRequestPage />);
    expect(screen.getByRole("combobox", { name: "Deployments" })).toBeDisabled();
    expect(screen.getAllByText("Select a Customer Project first.").length).toBeGreaterThan(0);

    pickProject("proj-a");
    expect(screen.getByRole("combobox", { name: "Deployments" })).toBeEnabled();
  });

  it("populates Deployments from the selected project", () => {
    render(<CreateChangeRequestPage />);
    pickProject("proj-a");
    expect(offered("Deployments")).toEqual(["Acme Production", "Acme Staging"]);
  });

  it("shows Deployment products as a read-only, locked field with its 'derived' helper", () => {
    render(<CreateChangeRequestPage />);
    const field = screen.getByLabelText("Deployment products");
    expect(field).toHaveAttribute("readonly");
    expect(field).toHaveAttribute("aria-readonly", "true");
    expect(screen.getByText("Derived from the selected deployments")).toBeInTheDocument();
    // Typing does nothing: there is no way to pick products by hand.
    fireEvent.change(field, { target: { value: "anything" } });
    expect(field).toHaveValue("");
  });

  it("derives the Deployment products from the chosen deployments", () => {
    render(<CreateChangeRequestPage />);
    pickProject("proj-a");
    pickOptions("Deployments", ["Acme Production"]);

    expect(chips("Deployments")).toEqual(["Acme Production"]);
    expect(productChips()).toEqual(["API Manager 4.3.0", "Identity Server 7.0.0"]);

    pickOptions("Deployments", ["Acme Staging"]);
    expect(productChips()).toEqual(["API Manager 4.3.0", "Identity Server 7.0.0", "API Manager 4.2.0"]);
  });

  it("drops the products of a deployment that is removed", () => {
    render(<CreateChangeRequestPage />);
    pickProject("proj-a");
    pickOptions("Deployments", ["Acme Production", "Acme Staging"]);
    pickOptions("Deployments", ["Acme Production"]); // toggles Production off
    expect(chips("Deployments")).toEqual(["Acme Staging"]);
    expect(productChips()).toEqual(["API Manager 4.2.0"]);
  });

  it("clears the dependents when the project changes, and offers the new project's deployments", () => {
    render(<CreateChangeRequestPage />);
    pickProject("proj-a");
    pickOptions("Deployments", ["Acme Production"]);
    expect(productChips()).not.toHaveLength(0);

    pickProject("proj-b");
    expect(chips("Deployments")).toEqual([]);
    expect(productChips()).toEqual([]);
    expect(offered("Deployments")).toEqual(["Beta Development"]);
  });

  it("clears the dependents and disables Deployments again when the project is cleared", () => {
    render(<CreateChangeRequestPage />);
    pickProject("proj-a");
    pickOptions("Deployments", ["Acme Production"]);
    pickProject("");
    expect(screen.getByRole("combobox", { name: "Deployments" })).toBeDisabled();
    expect(chips("Deployments")).toEqual([]);
    expect(productChips()).toEqual([]);
  });

  it("offers a retry when the deployments cannot be loaded", () => {
    scopeLookupError = true;
    render(<CreateChangeRequestPage />);
    pickProject("proj-a");
    expect(screen.getByText(/couldn.t load deployments/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  it("does not newly require any of the new fields", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    expect(screen.getByRole("button", { name: /create change request/i })).toBeEnabled();
    expect(screen.getByLabelText("Customer Project")).not.toBeRequired();
  });

  it("sends projectId, deploymentIds and deploymentProductIds with the exact wire names", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    pickProject("proj-a");
    pickOptions("Deployments", ["Acme Production", "Acme Staging"]);
    const payload = submittedPayload();
    expect(payload).toMatchObject({
      projectId: "proj-a",
      deploymentIds: ["dep-prod", "dep-stg"],
      deploymentProductIds: ["dp-apim", "dp-is", "dp-apim-stg"],
    });
    // The removed fields are never on the wire.
    expect(payload).not.toHaveProperty("environmentIds");
    expect(payload).not.toHaveProperty("customerGroupId");
  });

  it("omits every new field from the payload when none was filled in (arrays only when non-empty)", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    const payload = submittedPayload();
    for (const key of [
      "projectId",
      "deploymentIds",
      "deploymentProductIds",
      "comment",
      "workNote",
    ]) {
      expect(payload).not.toHaveProperty(key);
    }
  });

  it("sends a project alone, without empty arrays, when no deployment was chosen", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    pickProject("proj-b");
    const payload = submittedPayload();
    expect(payload).toHaveProperty("projectId", "proj-b");
    expect(payload).not.toHaveProperty("deploymentIds");
    expect(payload).not.toHaveProperty("deploymentProductIds");
  });

  it("never sends deployments of a project that was since changed", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    pickProject("proj-a");
    pickOptions("Deployments", ["Acme Production"]);
    pickProject("proj-b");
    const payload = submittedPayload();
    expect(payload).toHaveProperty("projectId", "proj-b");
    expect(payload).not.toHaveProperty("deploymentIds");
    expect(payload).not.toHaveProperty("deploymentProductIds");
  });

  it("surfaces the backend's 400 about an inconsistent combination verbatim", async () => {
    const { BackendApiError } = await import("@api/backend/client");
    render(<CreateChangeRequestPage />);
    fillSubject();
    pickProject("proj-a");
    pickOptions("Deployments", ["Acme Production"]);
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));
    const [, options] = postChangeRequestMutateMock.mock.calls[0];
    const message = "deploymentIds: deployment dep-prod does not belong to project proj-a";
    const err = new (BackendApiError as unknown as new (s: number, m: string) => Error)(400, message);
    options.onError(err);
    expect(showErrorMock).toHaveBeenCalledWith(message, err);
  });

  it("surfaces the backend's refusal when nobody on the project can be asked, verbatim, in the same error banner", async () => {
    const { BackendApiError } = await import("@api/backend/client");
    render(<CreateChangeRequestPage />);
    fillSubject();
    pickProject("proj-a");
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));
    const [, options] = postChangeRequestMutateMock.mock.calls[0];
    const message =
      "customer approval is required but nobody on this project can be asked (no registered contact other than the requester): register a contact for the project first";
    const err = new (BackendApiError as unknown as new (s: number, m: string) => Error)(400, message);
    options.onError(err);
    expect(showErrorMock).toHaveBeenCalledWith(message, err);
  });

  describe("Customer Group: the project's registered contacts, read-only", () => {
    it("is a locked, read-only field that says it is derived, and cannot be typed into", () => {
      render(<CreateChangeRequestPage />);
      const field = screen.getByLabelText("Customer Group");
      expect(field).toHaveAttribute("readonly");
      expect(field).toHaveAttribute("aria-readonly", "true");
      fireEvent.change(field, { target: { value: "anything" } });
      expect(field).toHaveValue("");
    });

    it("says to choose a Customer Project first, and lists nobody, until one is chosen", () => {
      render(<CreateChangeRequestPage />);
      expect(groupChips()).toEqual([]);
      // The same helper sits under Deployments and Customer Group.
      expect(screen.getAllByText("Select a Customer Project first.")).toHaveLength(2);
    });

    it("lists the chosen project's registered contacts, with the 'derived' helper", () => {
      render(<CreateChangeRequestPage />);
      pickProject("proj-a");
      expect(groupChips()).toEqual(["Alice Aaron", "Bob Bell"]);
      expect(screen.getByText("Derived from the customer project's registered contacts")).toBeInTheDocument();
      expect(screen.getByText("Alice Aaron").closest(".MuiChip-root")).toHaveAttribute("title", "alice@acme.example");
    });

    it("re-derives when the project changes: another customer's contacts never carry over", () => {
      render(<CreateChangeRequestPage />);
      pickProject("proj-a");
      pickProject("proj-b");
      expect(groupChips()).toEqual(["Carol Cook"]);
    });

    it("empties again when the project is cleared", () => {
      render(<CreateChangeRequestPage />);
      pickProject("proj-a");
      pickProject("");
      expect(groupChips()).toEqual([]);
      expect(screen.getAllByText("Select a Customer Project first.")).toHaveLength(2);
    });

    it("says so when the project has no registered contacts", () => {
      render(<CreateChangeRequestPage />);
      pickProject("proj-c");
      expect(groupChips()).toEqual([]);
      expect(screen.getByText(/No registered contacts on this project/)).toBeInTheDocument();
    });

    it("is never sent: no customerGroupId, whatever the project's contacts", () => {
      render(<CreateChangeRequestPage />);
      fillSubject();
      pickProject("proj-a");
      expect(groupChips()).toHaveLength(2);
      const payload = submittedPayload();
      expect(payload).toHaveProperty("projectId", "proj-a");
      expect(payload).not.toHaveProperty("customerGroupId");
      expect(payload).not.toHaveProperty("customerContacts");
    });

    it("shows the backend's 400 about the removed customerGroupId verbatim", async () => {
      const { BackendApiError } = await import("@api/backend/client");
      render(<CreateChangeRequestPage />);
      fillSubject();
      fireEvent.click(screen.getByRole("button", { name: /create change request/i }));
      const [, options] = postChangeRequestMutateMock.mock.calls[0];
      const message =
        "customerGroupId is no longer accepted: the customer group is derived from the customer project's registered contacts";
      const err = new (BackendApiError as unknown as new (s: number, m: string) => Error)(400, message);
      options.onError(err);
      expect(showErrorMock).toHaveBeenCalledWith(message, err);
    });
  });
});

describe("CreateChangeRequestPage — Category, Additional comments, Work notes", () => {
  beforeEach(() => {
    sessionStorage.clear();
    locationState = undefined;
    scopeLookupError = false;
    navigateMock.mockReset();
    postChangeRequestMutateMock.mockReset();
    patchChangeRequestMutateMock.mockReset();
    showErrorMock.mockReset();
  });

  function submittedPayload(): Record<string, unknown> {
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));
    expect(postChangeRequestMutateMock).toHaveBeenCalledTimes(1);
    return postChangeRequestMutateMock.mock.calls[0]![0] as Record<string, unknown>;
  }

  function chooseCategory(label: string): void {
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Category" }));
    fireEvent.click(screen.getByRole("option", { name: label }));
  }

  it("defaults Category to Other, like the ServiceNow form, and sends it", () => {
    render(<CreateChangeRequestPage />);
    expect(screen.getByRole("combobox", { name: "Category" })).toHaveTextContent("Other");
    fillSubject();
    expect(submittedPayload()).toHaveProperty("category", "other");
  });

  it("offers every ServiceNow category and sends the chosen enum value", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Category" }));
    const labels = screen.getAllByRole("option").map((o) => o.textContent);
    expect(labels).toEqual(
      expect.arrayContaining(["Hardware", "Network", "Other", "Regular Release - Cloud", "Hotfix Release - Cloud", "DevOps", "Cloud Computing"]),
    );
    fireEvent.click(screen.getByRole("option", { name: "Regular Release - Cloud" }));
    expect(submittedPayload()).toHaveProperty("category", "regular_release_cloud");
  });

  it("omits the category when it is cleared back to '-- Select --'", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Category" }));
    fireEvent.click(screen.getByRole("option", { name: "-- Select --" }));
    expect(submittedPayload()).not.toHaveProperty("category");
  });

  it("sends Additional comments (customer visible) and Work notes as comment and workNote, trimmed", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    fireEvent.change(screen.getByLabelText("Additional comments (Customer visible)"), {
      target: { value: "  Maintenance window is 02:00-04:00 UTC.  " },
    });
    fireEvent.change(screen.getByLabelText("Work notes"), { target: { value: "Pre-checks done." } });
    expect(submittedPayload()).toMatchObject({
      comment: "Maintenance window is 02:00-04:00 UTC.",
      workNote: "Pre-checks done.",
    });
  });

  it("omits a blank or whitespace-only comment / work note", () => {
    render(<CreateChangeRequestPage />);
    fillSubject();
    fireEvent.change(screen.getByLabelText("Additional comments (Customer visible)"), { target: { value: "   " } });
    fireEvent.change(screen.getByLabelText("Work notes"), { target: { value: "\n" } });
    const payload = submittedPayload();
    expect(payload).not.toHaveProperty("comment");
    expect(payload).not.toHaveProperty("workNote");
  });

  it("shows a 4000-character counter on both and stops accepting input at the limit", () => {
    render(<CreateChangeRequestPage />);
    expect(screen.getByText(/Visible to the customer\. Characters left: 4000/)).toBeInTheDocument();
    expect(screen.getByText(/Internal only\. Characters left: 4000/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Work notes"), { target: { value: "x".repeat(4100) } });
    expect(screen.getByLabelText("Work notes")).toHaveValue("x".repeat(4000));
    expect(screen.getByText(/Internal only\. Characters left: 0/)).toBeInTheDocument();
  });

  it("labels the Communication area and keeps it after the schedule", () => {
    render(<CreateChangeRequestPage />);
    expect(
      screen.getByText("Schedule").compareDocumentPosition(screen.getByText("Communication")) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("restores the category once changed away from the default", () => {
    const { unmount } = render(<CreateChangeRequestPage />);
    chooseCategory("Network");
    unmount();
    render(<CreateChangeRequestPage />);
    expect(screen.getByRole("combobox", { name: "Category" })).toHaveTextContent("Network");
  });
});

describe("CreateChangeRequestPage — scope fields in the in-progress draft and Clone", () => {
  beforeEach(() => {
    sessionStorage.clear();
    locationState = undefined;
    scopeLookupError = false;
    navigateMock.mockReset();
    postChangeRequestMutateMock.mockReset();
    patchChangeRequestMutateMock.mockReset();
    showErrorMock.mockReset();
  });

  function pickOptions(field: string, names: string[]): void {
    const input = screen.getByRole("combobox", { name: field });
    fireEvent.mouseDown(input);
    for (const name of names) fireEvent.click(screen.getByRole("option", { name }));
    fireEvent.keyDown(input, { key: "Escape" });
  }

  function chips(field: string): string[] {
    const root = screen.getByRole("combobox", { name: field }).closest(".MuiInputBase-root");
    return Array.from(root?.querySelectorAll(".MuiChip-label") ?? []).map((c) => c.textContent ?? "");
  }

  function groupChips(): string[] {
    const root = screen.getByLabelText("Customer Group").closest(".MuiInputBase-root");
    return Array.from(root?.querySelectorAll(".MuiChip-label") ?? []).map((c) => c.textContent ?? "");
  }

  it("restores project, deployments, products, comment and work note after unmount/remount, with the group re-derived from the project", () => {
    const first = render(<CreateChangeRequestPage />);
    fireEvent.change(screen.getByLabelText("Customer Project"), { target: { value: "proj-a" } });
    pickOptions("Deployments", ["Acme Production", "Acme Staging"]);
    fireEvent.change(screen.getByLabelText("Additional comments (Customer visible)"), { target: { value: "hello" } });
    fireEvent.change(screen.getByLabelText("Work notes"), { target: { value: "internal" } });
    first.unmount();

    render(<CreateChangeRequestPage />);
    expect(screen.getByLabelText("Customer Project")).toHaveValue("proj-a");
    expect(chips("Deployments")).toEqual(["Acme Production", "Acme Staging"]);
    expect(groupChips()).toEqual(["Alice Aaron", "Bob Bell"]);
    expect(screen.getByLabelText("Additional comments (Customer visible)")).toHaveValue("hello");
    expect(screen.getByLabelText("Work notes")).toHaveValue("internal");
    // The restored products are the derived ones.
    const productRoot = screen.getByLabelText("Deployment products").closest(".MuiInputBase-root");
    expect(Array.from(productRoot!.querySelectorAll(".MuiChip-label")).map((c) => c.textContent)).toEqual([
      "API Manager 4.3.0",
      "Identity Server 7.0.0",
      "API Manager 4.2.0",
    ]);
  });

  it("keeps no customer group in the draft, and drops one a draft saved earlier still holds", () => {
    const first = render(<CreateChangeRequestPage />);
    fireEvent.change(screen.getByLabelText("Customer Project"), { target: { value: "proj-b" } });
    const draft = JSON.parse(sessionStorage.getItem(changeRequestDraftKey({ kind: "new" }))!) as Record<string, unknown>;
    expect(draft).toMatchObject({ projectId: "proj-b" });
    expect(draft).not.toHaveProperty("customerGroupId");
    expect(draft).not.toHaveProperty("customerGroupLabel");
    expect(draft).not.toHaveProperty("customerContacts");
    first.unmount();

    // A draft written by the previous version of the form carried a picked group
    // (and environments) of proj-a, and the project; restoring it shows the
    // project's own contacts and never sends the stale ids.
    saveChangeRequestDraft(changeRequestDraftKey({ kind: "new" }), {
      ...(draft as unknown as ChangeRequestDraft),
      subject: "Old draft",
      type: "normal",
      projectId: "proj-a",
      customerGroupId: "grp-stale",
      customerGroupLabel: "Stale Customers",
      environmentIds: ["env-prod"],
    } as ChangeRequestDraft);
    render(<CreateChangeRequestPage />);
    expect(groupChips()).toEqual(["Alice Aaron", "Bob Bell"]);
    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));
    const payload = postChangeRequestMutateMock.mock.calls[0]![0] as Record<string, unknown>;
    expect(payload).toHaveProperty("projectId", "proj-a");
    expect(payload).not.toHaveProperty("customerGroupId");
    expect(payload).not.toHaveProperty("environmentIds");
  });

  it("persists the scope into the draft with display names, so a restore never shows raw ids", () => {
    render(<CreateChangeRequestPage />);
    fireEvent.change(screen.getByLabelText("Customer Project"), { target: { value: "proj-a" } });
    pickOptions("Deployments", ["Acme Production"]);
    const raw = sessionStorage.getItem(changeRequestDraftKey({ kind: "new" }));
    const draft = JSON.parse(raw!) as ChangeRequestDraft;
    expect(draft).toMatchObject({
      projectId: "proj-a",
      projectLabel: "Acme Project",
      deploymentIds: ["dep-prod"],
      deploymentLabels: { "dep-prod": "Acme Production" },
      deploymentProductIds: ["dp-apim", "dp-is"],
      category: "other",
    });
  });

  it("restores a draft saved before these fields existed with everything blank and Category on Other", () => {
    saveChangeRequestDraft(changeRequestDraftKey({ kind: "new" }), {
      subject: "Old draft",
      type: "normal",
      impact: "low",
      priority: "",
      plannedStartDate: "",
      plannedEndDate: "",
      description: "",
      justification: "",
      implementationPlan: "",
      riskImpactAnalysis: "",
      backoutPlan: "",
      testPlan: "",
      isPlanningVisibleToCustomers: false,
      groupId: "",
      assignedEngineerId: "",
      requestedById: "",
      parentValue: "",
    });
    render(<CreateChangeRequestPage />);
    expect(screen.getByLabelText(/subject/i)).toHaveValue("Old draft");
    expect(screen.getByLabelText("Customer Project")).toHaveValue("");
    expect(chips("Deployments")).toEqual([]);
    expect(screen.getByRole("combobox", { name: "Category" })).toHaveTextContent("Other");
  });

  it("clones the source's project and category, but leaves deployments / products to choose and derives the group from the project", () => {
    locationState = {
      sourceNumber: "CHG0001234",
      subject: "Promote the fix",
      type: "normal",
      projectId: "proj-a",
      projectLabel: "Acme Project",
      // A stale clone state that still names a group: ignored.
      ...({ customerGroupId: "grp-stale", customerGroupLabel: "Stale Customers" } as object),
      category: "devops",
    };
    render(<CreateChangeRequestPage />);
    expect(screen.getByLabelText("Customer Project")).toHaveValue("proj-a");
    expect(groupChips()).toEqual(["Alice Aaron", "Bob Bell"]);
    expect(screen.getByRole("combobox", { name: "Category" })).toHaveTextContent("DevOps");
    // Deployments are the new target: enabled (a project is known) but empty.
    expect(screen.getByRole("combobox", { name: "Deployments" })).toBeEnabled();
    expect(chips("Deployments")).toEqual([]);

    fireEvent.click(screen.getByRole("button", { name: /create change request/i }));
    const payload = postChangeRequestMutateMock.mock.calls[0]![0] as Record<string, unknown>;
    expect(payload).toMatchObject({ projectId: "proj-a", category: "devops" });
    expect(payload).not.toHaveProperty("customerGroupId");
    expect(payload).not.toHaveProperty("deploymentIds");
    expect(payload).not.toHaveProperty("environmentIds");
    expect(payload).not.toHaveProperty("deploymentProductIds");
  });

  it("mentions the cloned project/category and the deliberately blank deployments in the clone banner", () => {
    locationState = { sourceNumber: "CHG0001234", subject: "Promote the fix" };
    render(<CreateChangeRequestPage />);
    expect(screen.getByText(/customer project, category/i)).toBeInTheDocument();
    expect(screen.getByText(/Deployments, deployment products, schedule/i)).toBeInTheDocument();
    expect(screen.getByText(/The Customer Group follows the customer project/i)).toBeInTheDocument();
  });
});

describe("CreateChangeRequestPage — fields inferred from the originating service request", () => {
  beforeEach(() => {
    sessionStorage.clear();
    postChangeRequestMutateMock.mockReset();
    sourceCaseFixture = {
      id: "sr-1",
      projectId: "proj-a",
      projectName: "Acme Project",
      subject: "Upgrade the gateway",
      description: "<p>Needs a change</p>",
      productContext: { deploymentId: SCOPE_FIXTURE["proj-a"][0].id },
    };
  });
  afterEach(() => {
    sourceCaseFixture = undefined;
  });

  it("fills project, deployment, subject and description when opened from the service request", () => {
    locationState = { caseId: "sr-1", caseNumber: "CS-1", projectId: "proj-a" };
    render(<CreateChangeRequestPage />);
    expect(screen.getByDisplayValue("Upgrade the gateway")).toBeInTheDocument();
    expect(screen.getByDisplayValue("<p>Needs a change</p>")).toBeInTheDocument();
    expect(screen.getByDisplayValue("proj-a")).toBeInTheDocument();
  });

  it("infers the same fields when the service request is picked in the field instead", () => {
    locationState = undefined;
    render(<CreateChangeRequestPage />);
    fireEvent.change(screen.getByLabelText(/originating service request/i), {
      target: { value: encodeParentRecordValue("service_request", "sr-1") },
    });
    expect(screen.getByDisplayValue("Upgrade the gateway")).toBeInTheDocument();
    expect(screen.getByDisplayValue("proj-a")).toBeInTheDocument();
  });

  it("never overwrites a subject the user already typed", () => {
    locationState = undefined;
    render(<CreateChangeRequestPage />);
    fillSubject();
    const typed = (screen.getByLabelText(/short description|subject/i) as HTMLInputElement).value;
    fireEvent.change(screen.getByLabelText(/originating service request/i), {
      target: { value: encodeParentRecordValue("service_request", "sr-1") },
    });
    expect((screen.getByLabelText(/short description|subject/i) as HTMLInputElement).value).toBe(typed);
  });
});
