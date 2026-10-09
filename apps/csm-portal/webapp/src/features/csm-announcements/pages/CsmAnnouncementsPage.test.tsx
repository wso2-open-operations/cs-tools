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

import type { ReactElement } from "react";
import {
  fireEvent,
  render as rtlRender,
  screen,
  within,
} from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { MemoryRouter } from "react-router";
import "@testing-library/jest-dom/vitest";
import CsmAnnouncementsPage from "@features/csm-announcements/pages/CsmAnnouncementsPage";
import { useSearchAnnouncementRegistry } from "@features/csm-announcements/api/useSearchAnnouncementRegistry";
import { useSearchAnnouncementRequests } from "@features/csm-announcements/api/useSearchAnnouncementRequests";
import type { AnnouncementRegistryRow } from "@features/csm-announcements/types/announcementRegistry";
import type { AnnouncementRequest } from "@features/csm-announcements/types/announcementRequests";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";

// The backend client reads runtime config (`CSM_PORTAL_BACKEND_BASE_URL`) at
// module load, which isn't present under vitest. QueryErrorState imports
// `BackendApiError` from it, so stub the module (same approach as
// useCsmCaseActivities.test.ts).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ post: vi.fn() }),
}));

vi.mock("@features/csm-announcements/api/useSearchAnnouncementRegistry", () => ({
  useSearchAnnouncementRegistry: vi.fn(),
}));

vi.mock("@features/csm-announcements/api/useSearchAnnouncementRequests", () => ({
  useSearchAnnouncementRequests: vi.fn(),
}));

// The dialog pulls in its own web of hooks (get/update/submit/approve/publish)
// that aren't this page's concern — stubbed so the page test only asserts it
// opens with the right id, not what's inside it (see AnnouncementRequestDialog.test.tsx).
vi.mock("@features/csm-announcements/components/AnnouncementRequestDialog", () => ({
  default: ({
    requestId,
    onClose,
    caseMembers,
  }: {
    requestId: string;
    onClose: () => void;
    caseMembers?: { caseId: string; caseNumber: string; wso2CaseId: string; projectName: string }[];
  }) => (
    <div data-testid="request-dialog">
      <span>request dialog: {requestId}</span>
      <span>case members: {(caseMembers ?? []).map((m) => m.caseNumber).join(",")}</span>
      <button onClick={onClose}>close dialog</button>
    </div>
  ),
}));

// The real project picker fetches from the backend; stub it so the page test
// stays focused on the list + its own state filter.
vi.mock("@features/csm-cases/components/AsyncProjectMultiSelect", () => ({
  default: () => <div data-testid="project-filter" />,
}));

// The signed-in user's profile/id-token claims aren't relevant to this page's
// own behavior — only the column picker's storage key derives from them
// (see useColumnPreferences.test.ts for that logic).
// Mutable so the "New announcement" tests below can sign in with different
// portal roles; the page derives its capabilities from these through the real
// usePortalAccess.
let mockRoles: string[] = [];
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1", roles: mockRoles }, isLoading: false, isError: false }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "user@example.test" }),
}));

const mockedUseSearch = vi.mocked(useSearchAnnouncementRegistry);
const mockedUseSearchRequests = vi.mocked(useSearchAnnouncementRequests);
// Number, Reference, Subject, Project, State, Created by, Created, Updated.
const ANNOUNCEMENT_COLUMN_COUNT = 8;

/** `CsmAnnouncementsPage` renders a `RouterLink` per "case" row, so every
 * render here needs router context. `initialPath` lets a test land on a
 * specific URL (e.g. `?tab=pending`, the create form's post-save redirect
 * target). */
function render(ui: ReactElement, initialPath = "/"): ReturnType<typeof rtlRender> {
  return rtlRender(<MemoryRouter initialEntries={[initialPath]}>{ui}</MemoryRouter>);
}

// kind="case" — an announcement-type case with no owning request, rendered
// individually exactly like the old flat `CsmAnnouncementRow` list did.
const ROW: AnnouncementRegistryRow = {
  kind: "case",
  caseId: "a-1",
  caseNumber: "ANN-1001",
  subject: "Scheduled maintenance on Choreo",
  projectName: "IAM Production",
  state: "open",
  createdBy: "jane@example.com",
  createdOn: "2026-07-01T10:00:00Z",
  updatedOn: "2026-07-02T10:00:00Z",
};

// kind="batch" — a published announcement request; every case it created
// collapses into this one row.
const BATCH_ROW: AnnouncementRegistryRow = {
  kind: "batch",
  announcementRequestId: "req-batch-1",
  subject: "Upcoming maintenance window",
  projectCount: 3,
  cases: [
    { caseId: "case-1", caseNumber: "CS0001", wso2CaseId: "ACME-1", projectName: "Acme" },
    { caseId: "case-2", caseNumber: "CS0002", wso2CaseId: "BOLT-1", projectName: "Bolt" },
    { caseId: "case-3", caseNumber: "CS0003", wso2CaseId: "CDR-1", projectName: "Cinder" },
  ],
  createdBy: "jane@example.com",
  createdOn: "2026-07-01T10:00:00Z",
  updatedOn: "2026-07-01T10:00:00Z",
};

function mockResult(
  overrides: Partial<ReturnType<typeof useSearchAnnouncementRegistry>>,
): void {
  mockedUseSearch.mockReturnValue({
    data: undefined,
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    ...overrides,
  } as unknown as ReturnType<typeof useSearchAnnouncementRegistry>);
}

const PENDING_REQUEST: AnnouncementRequest = {
  id: "req-1",
  kind: "customer",
  state: "pending_approval",
  subject: "Upcoming maintenance",
  description: "<p>Details</p>",
  isSecurityAnnouncement: false,
  audienceDefinition: { scope: "specific", projectIds: ["p-1"] },
  createdBy: "jane@example.com",
  createdAt: "2026-07-01T10:00:00Z",
  updatedAt: "2026-07-01T10:00:00Z",
};

beforeEach(() => {
  // Default to a caller who can create announcements, since most of this file
  // exercises the creators' Requests tab; the visibility tests below sign in
  // other roles explicitly.
  mockRoles = ["cs_engineer", "announcement_creator"];
  mockedUseSearch.mockReset();
  mockedUseSearchRequests.mockReset();
  mockedUseSearchRequests.mockReturnValue({
    data: { requests: [], total: 0, limit: 10, offset: 0, hasMore: false },
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
  } as unknown as ReturnType<typeof useSearchAnnouncementRequests>);
  window.localStorage.clear();
});

// Creating an announcement needs the announcement_creator role on top of write
// access (the backend refuses it with a 403 otherwise), so the entry point is
// only offered to callers who can actually use it.
describe("CsmAnnouncementsPage — New announcement button", () => {
  beforeEach(() => {
    mockResult({ data: { rows: [], total: 0, limit: 20, offset: 0, hasMore: false } });
  });

  it("is offered to a CS engineer who holds the announcement creator role", () => {
    mockRoles = ["cs_engineer", "announcement_creator"];
    render(<CsmAnnouncementsPage />);
    expect(screen.getByRole("button", { name: /new announcement/i })).toBeInTheDocument();
  });

  it("is offered to admin without the role", () => {
    mockRoles = ["admin"];
    render(<CsmAnnouncementsPage />);
    expect(screen.getByRole("button", { name: /new announcement/i })).toBeInTheDocument();
  });

  it("is hidden from a CS engineer without the role, who can still read the list", () => {
    mockRoles = ["cs_engineer"];
    render(<CsmAnnouncementsPage />);
    expect(screen.queryByRole("button", { name: /new announcement/i })).not.toBeInTheDocument();
    expect(screen.getByText(/no announcements found/i)).toBeInTheDocument();
  });

  it("is hidden from a holder of the role who cannot write", () => {
    mockRoles = ["viewer", "announcement_creator"];
    render(<CsmAnnouncementsPage />);
    expect(screen.queryByRole("button", { name: /new announcement/i })).not.toBeInTheDocument();
  });
});

// The Requests tab (drafts and requests awaiting approval) is the creators'
// workspace: everyone else sees only the published announcements, and the page
// does not even ask the backend for the request list, which it refuses to them.
describe("CsmAnnouncementsPage — Requests tab visibility", () => {
  beforeEach(() => {
    mockResult({ data: { rows: [ROW], total: 1, limit: 20, offset: 0, hasMore: false } });
  });

  it("shows both tabs to a creator and runs the request search", () => {
    mockRoles = ["cs_engineer", "announcement_creator"];
    render(<CsmAnnouncementsPage />);
    expect(screen.getByRole("tab", { name: "Announcements" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Requests" })).toBeInTheDocument();
    expect(mockedUseSearchRequests.mock.calls.at(-1)?.[3]).toEqual({ enabled: true });
  });

  it("shows admin both tabs without the role", () => {
    mockRoles = ["admin"];
    render(<CsmAnnouncementsPage />);
    expect(screen.getByRole("tab", { name: "Requests" })).toBeInTheDocument();
  });

  for (const [label, roles] of [
    ["a CS engineer without the role", ["cs_engineer"]],
    ["a view-only role", ["viewer"]],
    ["a holder of the role who cannot write", ["viewer", "announcement_creator"]],
  ] as const) {
    it(`hides the tab bar from ${label}, who still sees the published announcements`, () => {
      mockRoles = [...roles];
      render(<CsmAnnouncementsPage />);
      expect(screen.queryByRole("tab", { name: "Requests" })).not.toBeInTheDocument();
      expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
      expect(screen.getByText("Scheduled maintenance on Choreo")).toBeInTheDocument();
    });
  }

  it("does not ask the backend for the request list when the caller cannot see it", () => {
    mockRoles = ["cs_engineer"];
    render(<CsmAnnouncementsPage />);
    expect(mockedUseSearchRequests.mock.calls.length).toBeGreaterThan(0);
    for (const call of mockedUseSearchRequests.mock.calls) {
      expect(call[3]).toEqual({ enabled: false });
    }
  });

  it("keeps a ?tab=pending link on the Announcements tab for a caller without access", () => {
    mockRoles = ["cs_engineer"];
    render(<CsmAnnouncementsPage />, "/announcements?tab=pending");
    expect(screen.getByText("Scheduled maintenance on Choreo")).toBeInTheDocument();
    expect(screen.queryByText("Upcoming maintenance")).not.toBeInTheDocument();
  });
});

describe("CsmAnnouncementsPage — list states", () => {

  it("renders a row from the search result", () => {
    mockResult({
      data: { rows: [ROW], total: 1, limit: 20, offset: 0, hasMore: false },
    });
    render(<CsmAnnouncementsPage />);
    expect(screen.getByText("Scheduled maintenance on Choreo")).toBeInTheDocument();
    expect(screen.getByText("ANN-1001")).toBeInTheDocument();
    expect(screen.getByText("IAM Production")).toBeInTheDocument();
  });

  it("shows the empty state when there are no announcements", () => {
    mockResult({
      data: { rows: [], total: 0, limit: 20, offset: 0, hasMore: false },
    });
    render(<CsmAnnouncementsPage />);
    expect(screen.getByText(/no announcements found/i)).toBeInTheDocument();
  });

  it("surfaces the error message on failure", () => {
    mockResult({ isError: true, error: new Error("boom") });
    render(<CsmAnnouncementsPage />);
    expect(screen.getByText("boom")).toBeInTheDocument();
  });
});

describe("CsmAnnouncementsPage — batch rows (grouped registry)", () => {
  it("renders a batch row's project count instead of a single project, and a Published state", () => {
    mockResult({
      data: { rows: [BATCH_ROW], total: 1, limit: 20, offset: 0, hasMore: false },
    });
    render(<CsmAnnouncementsPage />);
    expect(screen.getByText("Upcoming maintenance window")).toBeInTheDocument();
    expect(screen.getByText("3 projects")).toBeInTheDocument();
    expect(screen.getByText("Published")).toBeInTheDocument();
  });

  it("shows a Security chip next to the subject for a security batch row, not for a non-security one", () => {
    mockResult({
      data: {
        rows: [BATCH_ROW, { ...BATCH_ROW, announcementRequestId: "req-batch-2", isSecurityAnnouncement: true }],
        total: 2,
        limit: 20,
        offset: 0,
        hasMore: false,
      },
    });
    render(<CsmAnnouncementsPage />);
    expect(screen.getAllByText("Security")).toHaveLength(1);
  });

  it("opens the request dialog (not a case route) when a batch row is clicked", () => {
    mockResult({
      data: { rows: [BATCH_ROW], total: 1, limit: 20, offset: 0, hasMore: false },
    });
    render(<CsmAnnouncementsPage />);

    fireEvent.click(screen.getByText("Upcoming maintenance window"));

    expect(screen.getByText(`request dialog: ${BATCH_ROW.announcementRequestId}`)).toBeInTheDocument();
  });

  it("passes the row's own member cases through to the dialog, so it can list each project's CS number", () => {
    mockResult({
      data: { rows: [BATCH_ROW], total: 1, limit: 20, offset: 0, hasMore: false },
    });
    render(<CsmAnnouncementsPage />);

    fireEvent.click(screen.getByText("Upcoming maintenance window"));

    expect(screen.getByText("case members: CS0001,CS0002,CS0003")).toBeInTheDocument();
  });
});

describe("CsmAnnouncementsPage — filters default to show-all", () => {
  it("calls the search with no state/project filters on first render", () => {
    mockResult({
      data: { rows: [ROW], total: 1, limit: 20, offset: 0, hasMore: false },
    });
    render(<CsmAnnouncementsPage />);
    const [filters] = mockedUseSearch.mock.calls[0];
    expect(filters).toEqual(
      expect.objectContaining({ states: [], projectIds: [] }),
    );
  });

  it("pushes a picked state into the search filters", () => {
    mockResult({
      data: { rows: [ROW], total: 1, limit: 20, offset: 0, hasMore: false },
    });
    render(<CsmAnnouncementsPage />);

    fireEvent.mouseDown(screen.getByRole("combobox", { name: /state/i }));
    const listbox = screen.getByRole("listbox");
    fireEvent.click(within(listbox).getByRole("option", { name: /closed/i }));

    // The most recent render's filters carry the selected state.
    const lastCall = mockedUseSearch.mock.calls.at(-1)!;
    expect(lastCall[0].states).toContain("closed");
  });
});

describe("CsmAnnouncementsPage — customise columns", () => {
  it("shows the default columns and hides Created until the user adds it", () => {
    mockResult({
      data: { rows: [ROW], total: 1, limit: 20, offset: 0, hasMore: false },
    });
    render(<CsmAnnouncementsPage />);

    expect(screen.getByRole("columnheader", { name: "Number" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Updated" })).toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: "Created" })).not.toBeInTheDocument();
  });

  it("adds the Created column when checked in the picker, and it renders the row's createdAt", () => {
    mockResult({
      data: { rows: [ROW], total: 1, limit: 20, offset: 0, hasMore: false },
    });
    render(<CsmAnnouncementsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Customise announcements columns" }));
    // Column order is Number, Reference, Subject, Project, State, Created by,
    // Created, Updated — "Created" is the 7th checkbox, one of the two
    // available-but-hidden columns (see DEFAULT_ANNOUNCEMENT_COLUMN_IDS).
    const checkboxes = screen.getAllByRole("checkbox");
    fireEvent.click(checkboxes[6]);

    // The open popover marks the rest of the page `aria-hidden` (MUI's Modal
    // machinery) — `hidden: true` looks past that to the table underneath.
    expect(
      screen.getByRole("columnheader", { name: "Created", hidden: true }),
    ).toBeInTheDocument();
    // Same format the component's own formatDate() uses for createdOn/updatedOn.
    const expectedCreatedAt = formatBackendTimestampForDisplay(ROW.createdOn, {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
    expect(screen.getByText(expectedCreatedAt!)).toBeInTheDocument();
  });

  it("adds the Reference column when checked in the picker, and it renders the row's wso2CaseId", () => {
    mockResult({
      data: {
        rows: [{ ...ROW, wso2CaseId: "ACMESUB-42" }],
        total: 1,
        limit: 20,
        offset: 0,
        hasMore: false,
      },
    });
    render(<CsmAnnouncementsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Customise announcements columns" }));
    // "Reference" is the 2nd checkbox — see the column order comment above.
    const checkboxes = screen.getAllByRole("checkbox");
    fireEvent.click(checkboxes[1]);

    expect(
      screen.getByRole("columnheader", { name: "Reference", hidden: true }),
    ).toBeInTheDocument();
    expect(screen.getByText("ACMESUB-42")).toBeInTheDocument();
  });

  it("never lets every column be unchecked", () => {
    mockResult({
      data: { rows: [ROW], total: 1, limit: 20, offset: 0, hasMore: false },
    });
    render(<CsmAnnouncementsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Customise announcements columns" }));
    // Uncheck every column in turn (re-query each time — a re-render can
    // change stale element references — and skip whichever one the hook has
    // disabled as the last remaining visible column).
    for (let i = 0; i < ANNOUNCEMENT_COLUMN_COUNT; i++) {
      const checkbox = screen.getAllByRole("checkbox")[i];
      if (!checkbox.hasAttribute("disabled") && (checkbox as HTMLInputElement).checked) {
        fireEvent.click(checkbox);
      }
    }

    // At least one column (the last remaining) is still rendered underneath
    // the (still open) popover.
    expect(screen.getAllByRole("columnheader", { hidden: true }).length).toBeGreaterThan(0);
  });
});

describe("CsmAnnouncementsPage — Pending tab", () => {
  it("stays on the Announcements tab (and its table) by default", () => {
    mockResult({
      data: { rows: [ROW], total: 1, limit: 20, offset: 0, hasMore: false },
    });
    render(<CsmAnnouncementsPage />);
    expect(screen.getByText("Scheduled maintenance on Choreo")).toBeInTheDocument();
    expect(screen.queryByTestId("request-dialog")).not.toBeInTheDocument();
  });

  it("switches to the pending table and defaults to the Pending approval state filter", () => {
    mockResult({
      data: { rows: [ROW], total: 1, limit: 20, offset: 0, hasMore: false },
    });
    mockedUseSearchRequests.mockReturnValue({
      data: { requests: [PENDING_REQUEST], total: 1, limit: 10, offset: 0, hasMore: false },
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useSearchAnnouncementRequests>);
    render(<CsmAnnouncementsPage />);

    fireEvent.click(screen.getByRole("tab", { name: "Requests" }));

    expect(screen.getByText("Upcoming maintenance")).toBeInTheDocument();
    // states is the 1st arg of useSearchAnnouncementRequests(states, page, pageSize).
    const lastCall = mockedUseSearchRequests.mock.calls.at(-1)!;
    expect(lastCall[0]).toEqual(["pending_approval"]);
  });

  it("shows a Security chip next to the subject for a security pending request, not for a non-security one", () => {
    mockResult({
      data: { rows: [ROW], total: 1, limit: 20, offset: 0, hasMore: false },
    });
    mockedUseSearchRequests.mockReturnValue({
      data: {
        requests: [PENDING_REQUEST, { ...PENDING_REQUEST, id: "req-2", isSecurityAnnouncement: true }],
        total: 2,
        limit: 10,
        offset: 0,
        hasMore: false,
      },
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useSearchAnnouncementRequests>);
    render(<CsmAnnouncementsPage />);

    fireEvent.click(screen.getByRole("tab", { name: "Requests" }));

    expect(screen.getAllByText("Security")).toHaveLength(1);
  });

  it("opens the request dialog with the clicked row's id", () => {
    mockResult({
      data: { rows: [BATCH_ROW], total: 1, limit: 20, offset: 0, hasMore: false },
    });
    mockedUseSearchRequests.mockReturnValue({
      data: { requests: [PENDING_REQUEST], total: 1, limit: 10, offset: 0, hasMore: false },
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useSearchAnnouncementRequests>);
    render(<CsmAnnouncementsPage />);

    // Select a batch row first (which carries real case members) and close
    // it, so the assertion below actually proves openPendingRow clears that
    // prior selection rather than merely starting from an already-empty one.
    fireEvent.click(screen.getByText("Upcoming maintenance window"));
    fireEvent.click(screen.getByText("close dialog"));

    fireEvent.click(screen.getByRole("tab", { name: "Requests" }));
    fireEvent.click(screen.getByText("Upcoming maintenance"));

    expect(screen.getByText(`request dialog: ${PENDING_REQUEST.id}`)).toBeInTheDocument();
    // The Pending tab's own rows carry no case-member data at all — must not
    // leak the batch row's own members selected just above.
    expect(screen.getByText("case members:")).toBeInTheDocument();
    fireEvent.click(screen.getByText("close dialog"));
    expect(screen.queryByTestId("request-dialog")).not.toBeInTheDocument();
  });

  it("shows the empty state text for the selected pending state", () => {
    mockResult({ data: { rows: [], total: 0, limit: 20, offset: 0, hasMore: false } });
    render(<CsmAnnouncementsPage />);
    fireEvent.click(screen.getByRole("tab", { name: "Requests" }));
    expect(screen.getByText(/no pending approval requests/i)).toBeInTheDocument();
  });

  // Regression coverage: a published request used to have no click-path to
  // AnnouncementRequestDialog at all (the Announcements tab's own row link
  // only opens the underlying case, never this dialog) — see
  // PENDING_STATE_OPTIONS' own doc comment for the full story.
  it("can filter to Published and still open the request dialog", () => {
    mockResult({ data: { rows: [], total: 0, limit: 20, offset: 0, hasMore: false } });
    const publishedRequest: AnnouncementRequest = {
      ...PENDING_REQUEST,
      id: "req-published-1",
      state: "published",
      subject: "Already sent maintenance notice",
      publishedCaseIds: ["case-1", "case-2"],
    };
    mockedUseSearchRequests.mockReturnValue({
      data: { requests: [publishedRequest], total: 1, limit: 10, offset: 0, hasMore: false },
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useSearchAnnouncementRequests>);
    render(<CsmAnnouncementsPage />);

    fireEvent.click(screen.getByRole("tab", { name: "Requests" }));
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /state/i }));
    const listbox = screen.getByRole("listbox");
    // Multi-select: Published is added to the default Pending approval, so
    // narrowing to Published alone means unticking Pending approval too.
    fireEvent.click(within(listbox).getByRole("option", { name: "Pending approval" }));
    fireEvent.click(within(listbox).getByRole("option", { name: "Published" }));

    const lastCall = mockedUseSearchRequests.mock.calls.at(-1)!;
    expect(lastCall[0]).toEqual(["published"]);
    expect(screen.getByText("Already sent maintenance notice")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Already sent maintenance notice"));
    expect(screen.getByText(`request dialog: ${publishedRequest.id}`)).toBeInTheDocument();
  });

  describe("multi-select state filter", () => {
    const openStateFilter = (): HTMLElement => {
      fireEvent.click(screen.getByRole("tab", { name: "Requests" }));
      fireEvent.mouseDown(screen.getByRole("combobox", { name: /state/i }));
      return screen.getByRole("listbox");
    };

    beforeEach(() => {
      mockResult({ data: { rows: [], total: 0, limit: 20, offset: 0, hasMore: false } });
    });

    it("lets a second state be added to the selection instead of swapping it", () => {
      render(<CsmAnnouncementsPage />);
      const listbox = openStateFilter();

      fireEvent.click(within(listbox).getByRole("option", { name: "Approved" }));
      expect(mockedUseSearchRequests.mock.calls.at(-1)![0]).toEqual(["pending_approval", "approved"]);

      fireEvent.click(within(listbox).getByRole("option", { name: "Draft" }));
      expect(mockedUseSearchRequests.mock.calls.at(-1)![0]).toEqual([
        "pending_approval",
        "approved",
        "draft",
      ]);
    });

    it("can untick a state while others stay selected", () => {
      render(<CsmAnnouncementsPage />);
      const listbox = openStateFilter();

      fireEvent.click(within(listbox).getByRole("option", { name: "Approved" }));
      fireEvent.click(within(listbox).getByRole("option", { name: "Pending approval" }));

      expect(mockedUseSearchRequests.mock.calls.at(-1)![0]).toEqual(["approved"]);
    });

    it("treats an empty selection as every state, like the Announcements tab filter", () => {
      render(<CsmAnnouncementsPage />);
      const listbox = openStateFilter();

      fireEvent.click(within(listbox).getByRole("option", { name: "Pending approval" }));

      expect(mockedUseSearchRequests.mock.calls.at(-1)![0]).toEqual([]);
      expect(screen.getByText("No requests.")).toBeInTheDocument();
    });

    it("words the empty state for a multi-state selection", () => {
      render(<CsmAnnouncementsPage />);
      const listbox = openStateFilter();

      fireEvent.click(within(listbox).getByRole("option", { name: "Draft" }));

      expect(screen.getByText("No requests in the selected states.")).toBeInTheDocument();
    });

    it("returns to the first page whenever the selection changes", () => {
      mockedUseSearchRequests.mockReturnValue({
        data: { requests: [PENDING_REQUEST], total: 25, limit: 10, offset: 0, hasMore: true },
        isLoading: false,
        isFetching: false,
        isError: false,
        error: null,
      } as unknown as ReturnType<typeof useSearchAnnouncementRequests>);
      render(<CsmAnnouncementsPage />);
      fireEvent.click(screen.getByRole("tab", { name: "Requests" }));

      fireEvent.click(screen.getByRole("button", { name: /go to next page/i }));
      // useSearchAnnouncementRequests(states, page, pageSize) -- page is the 2nd arg.
      expect(mockedUseSearchRequests.mock.calls.at(-1)![1]).toBe(1);

      fireEvent.mouseDown(screen.getByRole("combobox", { name: /state/i }));
      fireEvent.click(
        within(screen.getByRole("listbox")).getByRole("option", { name: "Approved" }),
      );

      expect(mockedUseSearchRequests.mock.calls.at(-1)![1]).toBe(0);
    });

    it("shows each row's own state, since a merged list mixes states", () => {
      mockedUseSearchRequests.mockReturnValue({
        data: {
          requests: [
            PENDING_REQUEST,
            { ...PENDING_REQUEST, id: "req-d", state: "draft", subject: "A draft notice" },
            { ...PENDING_REQUEST, id: "req-a", state: "approved", subject: "An approved notice" },
            { ...PENDING_REQUEST, id: "req-p", state: "published", subject: "A published notice" },
          ],
          total: 4,
          limit: 10,
          offset: 0,
          hasMore: false,
        },
        isLoading: false,
        isFetching: false,
        isError: false,
        error: null,
      } as unknown as ReturnType<typeof useSearchAnnouncementRequests>);
      render(<CsmAnnouncementsPage />);
      fireEvent.click(screen.getByRole("tab", { name: "Requests" }));

      const table = within(screen.getByRole("table"));
      expect(table.getByRole("columnheader", { name: "State" })).toBeInTheDocument();
      for (const [subject, chip] of [
        ["Upcoming maintenance", "Pending approval"],
        ["A draft notice", "Draft"],
        ["An approved notice", "Approved"],
        ["A published notice", "Published"],
      ]) {
        const row = table.getByRole("row", { name: new RegExp(`View announcement request: ${subject}`) });
        expect(within(row).getByText(chip)).toBeInTheDocument();
      }
    });
  });

  it("lands directly on the Pending tab when opened with ?tab=pending", () => {
    mockResult({ data: { rows: [ROW], total: 1, limit: 20, offset: 0, hasMore: false } });
    mockedUseSearchRequests.mockReturnValue({
      data: { requests: [PENDING_REQUEST], total: 1, limit: 10, offset: 0, hasMore: false },
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useSearchAnnouncementRequests>);
    render(<CsmAnnouncementsPage />, "/announcements?tab=pending");

    // The Pending tab's own content shows immediately — no click needed —
    // and the Announcements tab's row isn't rendered.
    expect(screen.getByText("Upcoming maintenance")).toBeInTheDocument();
    expect(screen.queryByText("Scheduled maintenance on Choreo")).not.toBeInTheDocument();
  });
});

describe("CsmAnnouncementsPage — loading and pagination", () => {
  it("renders skeleton rows while the first page is loading", () => {
    mockResult({ isLoading: true, isFetching: true });
    const { container } = render(<CsmAnnouncementsPage />);
    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBeGreaterThan(0);
    expect(screen.queryByText(/no announcements found/i)).not.toBeInTheDocument();
  });

  it("advances the page when the next-page control is clicked", () => {
    mockResult({
      data: { rows: [ROW], total: 50, limit: 20, offset: 0, hasMore: true },
    });
    render(<CsmAnnouncementsPage />);
    fireEvent.click(screen.getByRole("button", { name: /go to next page/i }));
    // useSearchAnnouncements(filters, page, pageSize) — page is the 2nd arg.
    const lastCall = mockedUseSearch.mock.calls.at(-1)!;
    expect(lastCall[1]).toBe(1);
  });

  it("passes an updated page size when rows-per-page changes", () => {
    mockResult({
      data: { rows: [ROW], total: 50, limit: 20, offset: 0, hasMore: true },
    });
    render(<CsmAnnouncementsPage />);
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /rows per page/i }));
    fireEvent.click(screen.getByRole("option", { name: "50" }));
    // pageSize is the 3rd arg; changing rows-per-page also resets page to 0.
    const lastCall = mockedUseSearch.mock.calls.at(-1)!;
    expect(lastCall[2]).toBe(50);
    expect(lastCall[1]).toBe(0);
  });
});
