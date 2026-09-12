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
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { CloneChangeRequestNavState } from "@features/csm-operations/utils/changeRequests";
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
vi.mock("@components/AsyncEntitySelect", () => ({
  default: ({
    label,
    value,
    onChange,
  }: {
    label: string;
    value: string;
    onChange: (next: string) => void;
  }) => (
    <input aria-label={label} value={value} onChange={(e) => onChange(e.target.value)} />
  ),
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

/**
 * Fill the one field the form requires, so a test can reach the submit path
 * without restating unrelated input for every case.
 */
function fillSubject(): void {
  fireEvent.change(screen.getByLabelText(/subject/i), {
    target: { value: "Roll out fix to production" },
  });
}

describe("CreateChangeRequestPage — Clone prefill", () => {
  beforeEach(() => {
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
    expect(screen.getByText("Emergency")).toBeInTheDocument();
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

  it("always resets state to 'new' regardless of the clone source", () => {
    locationState = { subject: "Upgrade the gateway cluster" };
    render(<CreateChangeRequestPage />);
    expect(screen.getByText("New")).toBeInTheDocument();
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
    locationState = undefined;
    navigateMock.mockReset();
    postChangeRequestMutateMock.mockReset();
    patchChangeRequestMutateMock.mockReset();
    showErrorMock.mockReset();
  });

  it("renders the originating service request picker inside the optional section", () => {
    render(<CreateChangeRequestPage />);
    expect(screen.getByLabelText(/originating service request/i)).toBeInTheDocument();
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
      target: { value: "sr-123" },
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
      target: { value: "sr-123" },
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
    expect(screen.getByLabelText(/originating service request/i)).toHaveValue("sr-789");
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
