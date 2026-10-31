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

import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { BeProjectContact, BeUser } from "@api/backend/types";
import type { CreateRelatedCaseNavState } from "@features/csm-cases/types/csmCases";
import CsmCaseCreatePage from "./CsmCaseCreatePage";

const navigateMock = vi.fn();
const showErrorMock = vi.fn();
const mutateAsyncMock = vi.fn();
let locationState: CreateRelatedCaseNavState | undefined;

vi.mock("react-router", () => ({
  useLocation: () => ({ state: locationState }),
  useSearchParams: () => [new URLSearchParams()],
}));
vi.mock("@hooks/useNavTransition", () => ({ useNavTransition: () => navigateMock }));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
vi.mock("@hooks/useEngineerDisplayName", () => ({ useEngineerDisplayName: () => "Test Engineer" }));
vi.mock("@features/csm-cases/api/usePostCsmCase", () => ({
  usePostCsmCase: () => ({ mutateAsync: mutateAsyncMock }),
}));
vi.mock("@features/csm-cases/api/useCsmCaseAttachments", () => ({
  usePostCsmCaseAttachment: () => ({ mutateAsync: vi.fn() }),
}));
vi.mock("@features/csm-cases/api/uploadAttachmentsToCase", () => ({
  uploadAttachmentsToCase: () => Promise.resolve(0),
}));
vi.mock("@features/csm-projects/api/useGetProject", () => ({
  useGetProject: () => ({ data: { subscriptionType: "subscription" }, isFetching: false }),
}));
vi.mock("@features/csm-cases/api/useSearchDeployments", () => ({
  useSearchDeployments: () => ({
    data: [{ id: "dep-1", name: "Production", type: "primary_production" }],
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));
vi.mock("@features/csm-cases/api/useDeployedProductOptions", () => ({
  useDeployedProductOptions: () => ({
    data: [{ id: "dp-1", label: "Product One" }],
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({
    value,
    onChange,
    ariaRequired,
  }: {
    value: string;
    onChange: (v: string) => void;
    ariaRequired?: boolean;
  }) => (
    <textarea
      aria-label="Description body"
      aria-required={ariaRequired ? "true" : undefined}
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
}));
vi.mock("@components/attachments/AttachmentsField", () => ({ default: () => null }));
// The project picker is stubbed as one button per project.
vi.mock("@features/csm-cases/components/ProjectSelectionField", () => ({
  default: ({ onChange }: { onChange: (id: string) => void }) => (
    <div>
      <button onClick={() => onChange("proj-a")}>Pick project A</button>
      <button onClick={() => onChange("proj-b")}>Pick project B</button>
    </div>
  ),
}));

// Project contacts per project; `undefined` models a fetch still in flight.
let contactsByProject: Record<string, BeProjectContact[] | undefined> = {};
vi.mock("@features/csm-projects/api/useSearchProjectContacts", () => ({
  useSearchProjectContacts: (projectId: string | undefined) => {
    const data = projectId ? contactsByProject[projectId] : undefined;
    return {
      data,
      isLoading: !!projectId && data === undefined,
      isSuccess: data !== undefined,
      isError: false,
      isFetching: false,
      refetch: vi.fn(),
    };
  },
}));
let internalUsers: BeUser[] = [];
vi.mock("@api/useSearchUsersByName", () => ({
  useSearchInternalUsersByName: () => ({ data: internalUsers, isFetching: false, isError: false }),
}));
// The add picker is stubbed as one button per candidate its (real) search hook
// returns, so the candidate sources and exclusion are exercised for real.
vi.mock("@components/AsyncEntitySelect", () => ({
  default: function AddPickerStub({
    useSearch,
    searchExtra,
    getId,
    getLabel,
    excludeIds,
    onChange,
    disabled,
  }: {
    useSearch: (q: string, enabled: boolean, extra?: string) => { data: unknown[] | undefined };
    searchExtra?: string;
    getId: (i: unknown) => string;
    getLabel: (i: unknown) => string;
    excludeIds?: string[];
    onChange: (id: string, item?: unknown) => void;
    disabled?: boolean;
  }) {
    const { data } = useSearch("", true, searchExtra);
    return (
      <div data-testid="add-picker" aria-disabled={disabled}>
        {(data ?? [])
          .filter((i) => !excludeIds?.includes(getId(i)))
          .map((i) => (
            <button key={getId(i)} disabled={disabled} onClick={() => onChange(getId(i), i)}>
              {`Add ${getLabel(i)}`}
            </button>
          ))}
      </div>
    );
  },
}));

const registered = (name: string, email: string, extra: Partial<BeProjectContact> = {}) => ({
  name,
  email,
  registrationState: "registered",
  roles: ["PORTAL_USER"],
  ...extra,
});

const JANE = registered("Jane Doe", "jane.doe@example.com");
const JOHN = registered("John Smith", "john.smith@example.com");
const INVITED = registered("Ivy Invited", "ivy.invited@example.com", { registrationState: "invited" });
const SECURITY = registered("Sam Security", "sam.security@example.com", {
  roles: ["SECURITY_CONTACT"],
});

const list = (): HTMLElement | null => screen.queryByRole("list", { name: "Watch list" });
const rows = (): string[] =>
  within(list()!)
    .getAllByRole("listitem")
    .map((li) => li.textContent ?? "");

beforeEach(() => {
  vi.clearAllMocks();
  mutateAsyncMock.mockResolvedValue({ id: "case-1" });
  locationState = undefined;
  contactsByProject = { "proj-a": [JANE, JOHN, INVITED, SECURITY], "proj-b": [] };
  internalUsers = [{ id: "u-1", firstName: "Eve", lastName: "Engineer", email: "eve.engineer@example.com" }];
});

/** Opens the form from a "Create related case" route so every required field is
 * already set, leaving only the description to type. */
function renderPrefilledForm(): void {
  locationState = {
    projectId: "proj-a",
    relatedCaseId: "case-0",
    deploymentId: "dep-1",
    deployedProductId: "dp-1",
    severity: "S3",
    issueType: "question",
    subject: "Broken thing",
  };
  render(<CsmCaseCreatePage />);
  fireEvent.change(screen.getByLabelText("Description body"), { target: { value: "<p>Help</p>" } });
}

describe("CsmCaseCreatePage required Description", () => {
  it("marks the rich-text Description required: an asterisk on its caption and aria-required on the editor", () => {
    render(<CsmCaseCreatePage />);
    const caption = screen.getByText("Description");
    expect(caption.querySelector(".MuiFormLabel-asterisk")).not.toBeNull();
    expect(screen.getByLabelText("Description body")).toHaveAttribute("aria-required", "true");
  });
});

describe("CsmCaseCreatePage watch list", () => {
  it("pre-fills with the project's registered eligible contacts when a project is selected", () => {
    render(<CsmCaseCreatePage />);
    expect(screen.getByText(/Select a project to pre-fill/)).toBeInTheDocument();

    fireEvent.click(screen.getByText("Pick project A"));

    expect(rows()).toEqual([
      expect.stringContaining("Jane Doe (jane.doe@example.com)"),
      expect.stringContaining("John Smith (john.smith@example.com)"),
    ]);
  });

  it("excludes invited and security-only contacts from the list and the add picker", () => {
    render(<CsmCaseCreatePage />);
    fireEvent.click(screen.getByText("Pick project A"));

    expect(screen.queryByText(/ivy.invited@example.com/)).not.toBeInTheDocument();
    expect(screen.queryByText(/sam.security@example.com/)).not.toBeInTheDocument();
  });

  it("shows loading, then the empty state, for a project with no eligible contacts", () => {
    contactsByProject["proj-b"] = undefined;
    const { rerender } = render(<CsmCaseCreatePage />);
    fireEvent.click(screen.getByText("Pick project B"));
    expect(screen.getByText("Loading project contacts…")).toBeInTheDocument();

    contactsByProject["proj-b"] = [];
    rerender(<CsmCaseCreatePage />);
    expect(screen.getByText(/no registered contacts to pre-fill/)).toBeInTheDocument();
    expect(list()).toBeNull();
  });

  it("removes an entry", () => {
    render(<CsmCaseCreatePage />);
    fireEvent.click(screen.getByText("Pick project A"));

    fireEvent.click(
      screen.getByRole("button", {
        name: "Remove Jane Doe (jane.doe@example.com) from the watch list",
      }),
    );

    expect(rows()).toHaveLength(1);
    expect(rows()[0]).toContain("John Smith");
  });

  it("adds a project contact removed earlier and an internal user, and offers no one else", () => {
    render(<CsmCaseCreatePage />);
    fireEvent.click(screen.getByText("Pick project A"));
    fireEvent.click(
      screen.getByRole("button", {
        name: "Remove Jane Doe (jane.doe@example.com) from the watch list",
      }),
    );

    const picker = within(screen.getByTestId("add-picker"));
    expect(picker.getAllByRole("button").map((b) => b.textContent)).toEqual([
      "Add Jane Doe (jane.doe@example.com)",
      "Add Eve Engineer (eve.engineer@example.com)",
    ]);

    fireEvent.click(picker.getByText("Add Eve Engineer (eve.engineer@example.com)"));
    fireEvent.click(picker.getByText("Add Jane Doe (jane.doe@example.com)"));

    // A re-added contact returns to its pre-fill position; internal users follow.
    expect(rows().map((r) => r.replace(" · internal", ""))).toEqual([
      expect.stringContaining("Jane Doe"),
      expect.stringContaining("John Smith"),
      expect.stringContaining("Eve Engineer"),
    ]);
    expect(rows()[2]).toContain("internal");
  });

  it("does not clobber edits when the contacts finish loading later", () => {
    contactsByProject["proj-a"] = undefined;
    const { rerender } = render(<CsmCaseCreatePage />);
    fireEvent.click(screen.getByText("Pick project A"));
    // Added while contacts are still loading.
    fireEvent.click(screen.getByText("Add Eve Engineer (eve.engineer@example.com)"));

    contactsByProject["proj-a"] = [JANE, JOHN];
    rerender(<CsmCaseCreatePage />);

    expect(rows().map((r) => r.replace(" · internal", ""))).toEqual([
      expect.stringContaining("Jane Doe"),
      expect.stringContaining("John Smith"),
      expect.stringContaining("Eve Engineer"),
    ]);
  });

  it("resets edits when the project changes", () => {
    render(<CsmCaseCreatePage />);
    fireEvent.click(screen.getByText("Pick project A"));
    fireEvent.click(
      screen.getByRole("button", {
        name: "Remove Jane Doe (jane.doe@example.com) from the watch list",
      }),
    );
    fireEvent.click(screen.getByText("Pick project B"));
    fireEvent.click(screen.getByText("Pick project A"));

    expect(rows()).toHaveLength(2);
  });

  it("sends the emails as watchList on create", async () => {
    renderPrefilledForm();
    fireEvent.click(
      screen.getByRole("button", {
        name: "Remove John Smith (john.smith@example.com) from the watch list",
      }),
    );
    fireEvent.click(screen.getByText("Add Eve Engineer (eve.engineer@example.com)"));

    fireEvent.click(screen.getByRole("button", { name: "Create case" }));

    await waitFor(() => expect(mutateAsyncMock).toHaveBeenCalledTimes(1));
    expect(mutateAsyncMock.mock.calls[0][0]).toMatchObject({
      type: "case",
      projectId: "proj-a",
      watchList: ["jane.doe@example.com", "eve.engineer@example.com"],
    });
  });

  it("omits watchList when the list is empty", async () => {
    renderPrefilledForm();
    for (const who of ["Jane Doe (jane.doe@example.com)", "John Smith (john.smith@example.com)"]) {
      fireEvent.click(screen.getByRole("button", { name: `Remove ${who} from the watch list` }));
    }

    fireEvent.click(screen.getByRole("button", { name: "Create case" }));

    await waitFor(() => expect(mutateAsyncMock).toHaveBeenCalledTimes(1));
    expect(mutateAsyncMock.mock.calls[0][0]).not.toHaveProperty("watchList");
  });

  it("holds submit while the project's contacts are still loading", () => {
    contactsByProject["proj-a"] = undefined;
    renderPrefilledForm();
    expect(screen.getByRole("button", { name: "Create case" })).toBeDisabled();
  });
});
