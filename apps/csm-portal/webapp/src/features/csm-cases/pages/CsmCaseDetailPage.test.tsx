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

import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { JSX } from "react";
import {
  MemoryRouter,
  Route,
  Routes,
  useLocation,
  useNavigate,
  type NavigateOptions,
  type To,
} from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import "@testing-library/jest-dom/vitest";
import type { CsmCaseDetail } from "@features/csm-cases/types/csmCases";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";
import type { BeCaseState, BeCaseType } from "@api/backend/types";
import { sanitizeDescriptionHtml } from "@utils/sanitizeHtml";

// `@api/backend/client` -> `useAuthApiClient` -> `@config/apiConfig`, which
// throws at module load when `window.config` isn't set — not present under
// vitest. Same stub other page tests use (e.g. CsmAccountDetailPage.test.tsx).
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

// The backend client reads runtime config at module load, which isn't
// present under vitest. The page imports `BackendApiError` from it directly,
// so stub the module with a real class (so `instanceof` still works) — same
// approach as CsmChangeRequestDetailPage.test.tsx.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

const navigateMock = vi.fn();
vi.mock("@hooks/useNavTransition", () => ({
  useNavTransition: () => navigateMock,
}));

vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({
    email: "jane.doe@example.com",
    name: "Jane Doe",
  }),
}));

const showErrorMock = vi.fn();
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
const CURRENT_USER_ID = "00000000-0000-0000-0000-00000000000c";
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: { id: CURRENT_USER_ID, email: "jane.doe@example.com" },
    isLoading: false,
    isError: false,
    error: null,
  }),
}));
vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));
vi.mock("@features/csm-recent/hooks/useRecentViews", () => ({
  useRecordRecentView: () => vi.fn(),
}));
vi.mock("@utils/useDarkMode", () => ({
  useDarkMode: () => false,
}));

// Builds a minimal but valid CsmCaseDetail whose `id` tracks the currently
// mutated case id, so the page gets past its loading/error gates (the
// `isLoading`/`isError`/`!data` early returns) for whichever case is active.
function buildCase(
  id: string,
  overrides?: {
    caseType?: BeCaseType;
    description?: string;
    state?: BeCaseState;
  },
): CsmCaseDetail {
  return {
    id,
    caseNumber: `CS-${id}`,
    subject: "Sample case subject",
    customer: "Acme Corp",
    accountId: "account-1",
    projectId: "project-1",
    projectName: "Acme Project",
    product: "WSO2 Identity Server",
    severity: "S2",
    state: overrides?.state ?? "open",
    assignee: "Unassigned",
    assigneeIsMe: false,
    slaClockType: "resolution",
    minutesToBreach: 120,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    description: overrides?.description ?? "<p>Sample description</p>",
    caseType: overrides?.caseType,
    assignmentGroup: "Support Team",
    customerContext: {
      accountName: "Acme Corp",
      tier: "enterprise",
      region: "US",
      primaryContact: "Jane Doe",
      primaryContactEmail: "jane.doe@example.com",
      accountManager: "John Smith",
      openCases: 1,
    },
    productContext: {
      product: "WSO2 Identity Server",
      version: "7.0",
      deployment: "prod-cluster",
      environment: "prod",
    },
    watchers: [],
    linkedItems: [],
    tags: [],
    timeLogs: [],
    audit: [],
    attachments: [],
    isWatching: false,
  };
}

const WATCHER_ID = "00000000-0000-0000-0000-000000000001";
const NEW_WATCHER_ID = "00000000-0000-0000-0000-000000000002";

const useGetCsmCaseDetailMock = vi.fn();
vi.mock("@features/csm-cases/api/useGetCsmCaseDetail", () => ({
  useGetCsmCaseDetail: (id: string | undefined) => useGetCsmCaseDetailMock(id),
}));

// A real (non-vi.fn-per-render) mock so a test can capture the onSuccess/
// onError options passed to `.mutate()` and invoke them later, simulating a
// response that arrives after the page has navigated elsewhere.
const requestCaseUpdateMutateMock = vi.fn();
afterEach(() => {
  requestCaseUpdateMutateMock.mockClear();
});
function defaultCaseDetailImpl(id: string | undefined): unknown {
  return {
    data: id ? buildCase(id) : undefined,
    isLoading: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    isFetching: false,
    dataUpdatedAt: 0,
  };
}
useGetCsmCaseDetailMock.mockImplementation(defaultCaseDetailImpl);
// Reset to the shared default after every test — a test that swaps in its own
// implementation (e.g. to set caseType) must not leak that override into
// whichever test runs next.
afterEach(() => {
  useGetCsmCaseDetailMock.mockImplementation(defaultCaseDetailImpl);
});

const patchCaseMutateMock = vi.fn();
vi.mock("@features/csm-cases/api/usePatchCsmCase", () => ({
  usePatchCsmCase: () => ({
    mutate: patchCaseMutateMock,
    mutateAsync: vi.fn(),
    isPending: false,
  }),
  usePatchCsmCaseById: () => vi.fn(),
}));
vi.mock("@features/csm-cases/api/useFindMyOngoingCases", () => ({
  useFindMyOngoingCases: () => vi.fn(),
}));
const useGetCsmCaseCommentsMock = vi.fn();
vi.mock("@features/csm-cases/api/useCsmCaseComments", () => ({
  useGetCsmCaseComments: (id: string | undefined) =>
    useGetCsmCaseCommentsMock(id),
  usePostCsmCaseComment: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
function defaultCommentsImpl(): unknown {
  return {
    data: [],
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
    isFetching: false,
  };
}
useGetCsmCaseCommentsMock.mockImplementation(defaultCommentsImpl);
// Same reset reasoning as useGetCsmCaseDetailMock above — a test that swaps
// in its own comments list must not leak it into the next test.
afterEach(() => {
  useGetCsmCaseCommentsMock.mockImplementation(defaultCommentsImpl);
});
vi.mock("@features/csm-cases/api/useCsmConversationMessages", () => ({
  useGetCsmConversationMessages: () => ({
    data: undefined,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
    isFetching: false,
  }),
}));
vi.mock("@features/csm-cases/api/useCsmCaseActivities", () => ({
  useGetCsmCaseActivities: () => ({
    data: undefined,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
    isFetching: false,
  }),
}));
vi.mock("@features/csm-cases/api/useCsmCaseFeedback", () => ({
  useGetCsmCaseFeedback: () => ({
    data: undefined,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
    isFetching: false,
  }),
}));
vi.mock("@features/csm-cases/api/useCsmCaseAttachments", () => ({
  useGetCsmCaseAttachments: () => ({
    data: undefined,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
    isFetching: false,
    dataUpdatedAt: 0,
  }),
  usePostCsmCaseAttachment: () => ({
    mutateAsync: vi.fn(),
    isPending: false,
    isError: false,
    error: null,
  }),
  useDownloadCsmCaseAttachment: () => vi.fn(),
  useDeleteCsmCaseAttachment: () => ({ mutate: vi.fn(), isPending: false }),
  useGetCsmCaseAttachmentContent: () => vi.fn(),
}));
vi.mock("@features/csm-cases/api/useCsmCaseCallRequests", () => ({
  useGetCsmCaseCallRequests: () => ({
    data: undefined,
    refetch: vi.fn(),
    isFetching: false,
  }),
}));
vi.mock("@features/csm-cases/api/useSearchCaseTasks", () => ({
  useSearchCaseTasks: () => ({ data: undefined }),
}));
vi.mock("@features/csm-cases/api/useSearchDeployments", () => ({
  useSearchDeployments: () => ({
    data: undefined,
    isLoading: false,
    refetch: vi.fn(),
    isFetching: false,
  }),
}));
vi.mock("@features/csm-projects/api/useGetProject", () => ({
  useGetProject: () => ({
    data: undefined,
    isLoading: false,
    refetch: vi.fn(),
    isFetching: false,
  }),
}));
vi.mock("@features/csm-cases/api/useGetCsmCaseSlas", () => ({
  useGetCsmCaseSlas: () => ({ data: undefined }),
}));
vi.mock("@features/csm-cases/api/useCreateCaseTask", () => ({
  useCreateCaseTask: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("@features/csm-cases/api/useCaseTags", () => ({
  useAddCaseTag: () => ({ mutate: vi.fn(), isPending: false }),
  useRemoveCaseTag: () => ({
    mutate: vi.fn(),
    isPending: false,
    variables: undefined,
  }),
}));
vi.mock("@features/csm-cases/api/useCsmCaseGithubIssue", () => ({
  usePostCaseGithubIssue: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("@features/csm-cases/api/useGetCsmCaseEscalations", () => ({
  useGetCsmCaseEscalations: () => ({
    data: [],
    isLoading: false,
    isError: false,
  }),
}));
vi.mock("@features/csm-cases/api/usePostCsmCaseEscalation", () => ({
  usePostCsmCaseEscalation: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("@features/csm-cases/api/useRequestCaseUpdate", () => ({
  useRequestCaseUpdate: () => ({
    mutate: requestCaseUpdateMutateMock,
    isPending: false,
  }),
}));
// Two placeholder cards (shape doesn't matter beyond `.length` -- the tab
// label count is the only thing this page reads from the result;
// CaseTimeCardsPanel itself is stubbed below and never sees this data).
// `total` defaults to `cards.length` (no truncation); a test overriding this
// mock can set `total` higher than `cards.length` to simulate a truncated
// page and assert the tab count still reflects the real total.
const useCaseTimeCardsMock = vi.fn();
function defaultCaseTimeCardsImpl(): unknown {
  return { data: { cards: [{}, {}], total: 2, truncated: false } };
}
useCaseTimeCardsMock.mockImplementation(defaultCaseTimeCardsImpl);
afterEach(() => {
  useCaseTimeCardsMock.mockImplementation(defaultCaseTimeCardsImpl);
});
vi.mock("@features/csm-timecards/api/useTimeCards", () => ({
  usePostTimeCard: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateTimeCard: () => ({ mutate: vi.fn(), isPending: false }),
  useCaseTimeCards: (caseId: string | undefined) =>
    useCaseTimeCardsMock(caseId),
}));

// Simple presentational stubs — none of this test's assertions touch these.
vi.mock("@features/csm-cases/components/CsmCaseCommentInput", () => ({
  default: () => null,
}));
// Probe, not `null`: the change_case_type and request_update tests below
// need a way to open their dialogs the same way a real user would (via the
// action bar's menu). CaseActionBar's own rendering/gating is covered in
// CaseActionBar.test.tsx.
vi.mock("@features/csm-cases/components/CaseActionBar", () => ({
  default: ({ onAction }: { onAction: (action: { secondary: string }) => void }) => (
    <>
      <button type="button" onClick={() => onAction({ secondary: "change_case_type" })}>
        stub open change case type
      </button>
      <button type="button" onClick={() => onAction({ secondary: "request_update" })}>
        stub open request update
      </button>
    </>
  ),
  canAcknowledge: () => false,
}));
vi.mock("@features/csm-cases/components/AssignEngineerDialog", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/ResolutionDialog", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/ChangeSeverityDialog", () => ({
  default: () => null,
}));
// Probe, not `null`: the dialog's own three-step flow is covered in
// ChangeCaseTypeDialog.test.tsx. Here it just hands the page a finished
// submission so these tests can assert the resulting PATCH body(ies).
vi.mock("@features/csm-cases/components/ChangeCaseTypeDialog", () => ({
  default: ({
    onSubmit,
  }: {
    onSubmit: (
      submission:
        | { targetType: "engagement"; engagementType: string; engagementPaymentType: string }
        | { targetType: "case"; severity: string; issueType: string },
    ) => void;
  }) => (
    <>
      <button
        type="button"
        onClick={() =>
          onSubmit({
            targetType: "engagement",
            engagementType: "migration",
            engagementPaymentType: "paid",
          })
        }
      >
        stub transfer to engagement
      </button>
      <button
        type="button"
        onClick={() =>
          onSubmit({ targetType: "case", severity: "S2", issueType: "error" })
        }
      >
        stub transfer to case with severity
      </button>
    </>
  ),
}));
vi.mock("@features/csm-cases/components/SetAutocloseHoldDialog", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/EditCaseDetailsDialog", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/LinkIncidentDialog", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/LinkCaseDialog", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/SetFixEtaDialog", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/CreateTaskDialog", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/AddTagDialog", () => ({
  default: () => null,
}));
// Probe, not `null`: the stale-callback regression test below needs a way to
// submit the dialog the same way a real user would (its own template-preview/
// custom-message UI is covered in RequestUpdateDialog.test.tsx).
vi.mock("@features/csm-cases/components/RequestUpdateDialog", () => ({
  default: ({ onSave }: { onSave: (payload: { stage: string }) => void }) => (
    <button type="button" onClick={() => onSave({ stage: "first" })}>
      stub save request update
    </button>
  ),
}));
vi.mock("@features/csm-cases/components/ChildCasesWidget", () => ({
  ChildCasesWidget: () => null,
}));
vi.mock("@features/csm-cases/components/LinkedServiceRequestsWidget", () => ({
  LinkedServiceRequestsWidget: () => null,
}));
vi.mock("@features/csm-cases/components/LinkedChangeRequestsWidget", () => ({
  LinkedChangeRequestsWidget: () => (
    <div data-testid="linked-change-requests-widget-probe" />
  ),
}));
vi.mock("@features/csm-cases/components/CreateGithubIssueDialog", () => ({
  CreateGithubIssueDialog: () => null,
}));
// Probe, not `null`: several tests below assert on the comments the page
// hands this feed (including the synthetic description entry — see
// `safeComments` in CsmCaseDetailPage), which needs the passed-through
// author name / body / role-chip-suppression to actually render. The feed's
// own layout/filtering/sorting is covered in CaseActivitiesFeed.test.tsx.
vi.mock("@features/csm-cases/components/CaseActivitiesFeed", () => ({
  default: ({
    comments,
  }: {
    comments: Array<{
      id: string;
      authorName: string;
      authorRole: string;
      bodyHtml: string;
      synthetic?: boolean;
    }>;
  }) => (
    <div data-testid="case-activities-feed-probe">
      {comments.map((c) => (
        <div key={c.id} data-testid={`comment-${c.id}`}>
          <span>{c.authorName}</span>
          {!c.synthetic && c.authorRole !== "wso2_engineer" && (
            <span>{c.authorRole === "customer" ? "Customer" : c.authorRole}</span>
          )}
          <span dangerouslySetInnerHTML={{ __html: sanitizeDescriptionHtml(c.bodyHtml) }} />
        </div>
      ))}
    </div>
  ),
}));
vi.mock("@features/csm-cases/components/CaseMetaBand", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/CaseDetailWidgets", () => ({
  AttachmentsWidget: () => null,
  CustomerContextWidget: () => null,
  EscalationWidget: () => null,
  ProductContextWidget: () => null,
  // Stubbed to a probe rather than `null`: the tests below assert only that
  // the page mounts it for a service request and not for a plain case. The
  // widget's own rendering is covered in CaseDetailWidgets.test.tsx.
  RequestDetailsWidget: () => <div data-testid="request-details-widget" />,
  TagsWidget: () => null,
  // Probe, not `null`: the real widget computes the replacement watch list and
  // enforces the per-record-type rules (covered in CaseDetailWidgets.test.tsx).
  // Here it just hands the page a finished list so these tests can see what
  // the page does with it.
  WatchersWidget: ({
    entityKind,
    watchers,
    onReplace,
  }: {
    entityKind: string;
    watchers: Array<{ id: string; name: string }>;
    onReplace?: (nextWatcherIds: string[], action: "add" | "remove") => void;
  }) => (
    <div data-testid="watchers-widget" data-entity-kind={entityKind}>
      <button
        type="button"
        onClick={() =>
          onReplace?.([...watchers.map((w) => w.id), NEW_WATCHER_ID], "add")
        }
      >
        stub add watcher
      </button>
    </div>
  ),
}));
vi.mock("@features/csm-cases/components/CallRequestsWidget", () => ({
  CallRequestsWidget: () => null,
}));
vi.mock("@features/csm-cases/components/TasksWidget", () => ({
  TasksWidget: () => null,
}));
vi.mock("@features/csm-cases/components/CaseSlaTable", () => ({
  CaseSlaTable: () => null,
}));

// The two components at the center of this regression: CaseTimeCardsPanel is
// stubbed to a button that opens the edit dialog for a fake card (mirroring
// the real `onEditTimeCard={setEditTimeCard}` wiring), and LogTimeCardDialog
// is stubbed to a probe that renders the `editingCard` id and `caseId` it was
// actually given — so the test can see whether the dialog is still mounted,
// and against which case, after a route change.
const FAKE_CARD: CsmTimeCard = {
  id: "card-1",
  caseId: "case-1",
  caseNumber: "CS-case-1",
  projectId: "project-1",
  projectName: "Acme Project",
  workDate: "2026-01-01",
  userId: "user-1",
  userName: "Jane Doe",
  state: "submitted",
  billable: true,
  totalMinutes: 60,
} as CsmTimeCard;

vi.mock("@features/csm-timecards/components/CaseTimeCardsPanel", () => ({
  default: ({
    onEditTimeCard,
  }: {
    onEditTimeCard: (card: CsmTimeCard) => void;
  }) => (
    <button type="button" onClick={() => onEditTimeCard(FAKE_CARD)}>
      Edit fake time card
    </button>
  ),
}));
vi.mock("@features/csm-timecards/components/LogTimeCardDialog", () => ({
  default: ({
    caseId,
    editingCard,
  }: {
    caseId: string;
    editingCard?: CsmTimeCard;
  }) => (
    <div data-testid="log-time-card-dialog-probe">
      {`editingCardId=${editingCard?.id ?? "none"} caseId=${caseId}`}
    </div>
  ),
}));

// Imported after the mocks above so the module picks them up.
import { BackendApiError } from "@api/backend/client";
import CsmCaseDetailPage from "@features/csm-cases/pages/CsmCaseDetailPage";

// Renders the real route pathname, so the test can assert the router itself
// actually transitioned (not just that the page re-rendered) — same
// convention as CsmAccountDetailPage.test.tsx's `LocationProbe`.
function LocationProbe(): JSX.Element {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

/** Same idea as `LocationProbe`, surfacing the search string + hash too — for
 * the `?tab=` URL-sync and canonical-redirect tests below. */
function LocationSearchProbe(): JSX.Element {
  const location = useLocation();
  return (
    <div data-testid="location-search-probe">
      {location.pathname}
      {location.search}
      {location.hash}
    </div>
  );
}

// Fires a real `navigate("/cases/case-2")` from inside the router, the same
// way an in-app link (e.g. the sidebar's recent-cases list) would move the
// user from one case to another without unmounting this page — the route
// pattern is identical, only the `:caseId` param changes.
function NavigateToCaseTwoButton(): JSX.Element {
  const navigate = useNavigate();
  return (
    <button type="button" onClick={() => navigate("/cases/case-2")}>
      Go to case 2
    </button>
  );
}

function renderPage(): ReturnType<typeof render> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/cases/case-1"]}>
        <NavigateToCaseTwoButton />
        <LocationProbe />
        <Routes>
          <Route path="/cases/:caseId" element={<CsmCaseDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const DASHED_ID = "56f49f0a-eb1e-c310-fcf5-f5dabad0cdab";
const DASHLESS_ID = "56f49f0aeb1ec310fcf5f5dabad0cdab";

// `useNavTransition` is mocked module-wide (above) so the rest of this
// file's tests can assert on navigate-call arguments without a real router
// transition. For the dashless-id tests specifically we want to prove the
// router's own location actually changes, not just that the mock was
// invoked — so this bridges the mock to the real `useNavigate` from this
// render tree, and `LocationProbe` (already used elsewhere in this file)
// verifies the resulting location.
function NavigateBridge(): null {
  const navigate = useNavigate();
  navigateMock.mockImplementation((to: To, options?: NavigateOptions) =>
    navigate(to, options),
  );
  return null;
}

function renderPageAtCaseId(initialEntry: string): ReturnType<typeof render> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <NavigateBridge />
        <LocationProbe />
        <Routes>
          <Route path="/cases/:caseId" element={<CsmCaseDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

// The five real route mount points this page renders under (App.tsx) — all
// must sync `?tab=` the same way, not just the plain /cases/:caseId one. Each
// carries the `caseType` that makes IT the record's own canonical route (see
// `canonicalDetailPath`/`isMisrouted`), so the page renders straight through
// rather than bouncing through the canonical-redirect skeleton first.
const CASE_ROUTE_MOUNTS: Array<{
  name: string;
  path: string;
  caseId: string;
  caseType?: string;
}> = [
  { name: "cases", path: "/cases/:caseId", caseId: "case-1" },
  {
    name: "operations/service-requests",
    path: "/operations/service-requests/:caseId",
    caseId: "case-1",
    caseType: "service_request",
  },
  {
    name: "engagements",
    path: "/engagements/:caseId",
    caseId: "case-1",
    caseType: "engagement",
  },
  {
    name: "security-center/security-reports",
    path: "/security-center/security-reports/:caseId",
    caseId: "case-1",
    caseType: "security_report_analysis",
  },
  {
    name: "announcements",
    path: "/announcements/:caseId",
    caseId: "case-1",
    caseType: "announcement",
  },
];

function renderPageAt(
  initialEntry: string,
  routePath = "/cases/:caseId",
): ReturnType<typeof render> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <LocationSearchProbe />
        <Routes>
          <Route path={routePath} element={<CsmCaseDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CsmCaseDetailPage — tab lives in the URL", () => {
  it("defaults to the Activities tab when ?tab= is absent", () => {
    renderPageAt("/cases/case-1");

    expect(screen.getByRole("tab", { name: /activities/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("restores the tab named in the URL on a direct/cold load", () => {
    renderPageAt("/cases/case-1?tab=attachments");

    expect(screen.getByRole("tab", { name: /attachments/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("falls back to Activities for an unrecognised ?tab= value, without crashing or looping", () => {
    renderPageAt("/cases/case-1?tab=not-a-real-tab");

    expect(screen.getByRole("tab", { name: /activities/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByTestId("location-search-probe")).toHaveTextContent(
      "tab=not-a-real-tab",
    );
  });

  it("writes the selected tab to ?tab= when switching tabs, replacing rather than pushing a new history entry", () => {
    renderPageAt("/cases/case-1");

    fireEvent.click(screen.getByRole("tab", { name: /watchers/i }));

    expect(screen.getByTestId("location-search-probe")).toHaveTextContent(
      "tab=watchers",
    );
  });

  it("falls back to Activities for ?tab=tasks, the hidden tab with no rendered <Tab>", () => {
    renderPageAt("/cases/case-1?tab=tasks");

    expect(screen.getByRole("tab", { name: /activities/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    // "tasks" isn't a rendered <Tab> at all (it's hidden from the strip).
    expect(
      screen.queryByRole("tab", { name: /^tasks$/i }),
    ).not.toBeInTheDocument();
  });

  it.each(CASE_ROUTE_MOUNTS)(
    // "Attachments" (not e.g. "SLAs") since it's the one tab every case type
    // in this list renders, including an announcement — which hides
    // related/watchers/sla/time/call-requests entirely (see the TAB_DEFS
    // filter and the isAnnouncement force-to-Activities effect).
    "syncs ?tab= the same way under the $name mount point",
    ({ path, caseId, caseType }) => {
      useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
        data: id ? { ...buildCase(id), caseType } : undefined,
        isLoading: false,
        isError: false,
        error: null,
        refetch: vi.fn(),
        isFetching: false,
        dataUpdatedAt: 0,
      }));

      renderPageAt(`${path.replace(":caseId", caseId)}?tab=attachments`, path);

      expect(
        screen.getByRole("tab", { name: /^attachments$/i }),
      ).toHaveAttribute("aria-selected", "true");

      // Restore the default mock so later tests aren't affected.
      useGetCsmCaseDetailMock.mockImplementation((defaultId: string | undefined) => ({
        data: defaultId ? buildCase(defaultId) : undefined,
        isLoading: false,
        isError: false,
        error: null,
        refetch: vi.fn(),
        isFetching: false,
        dataUpdatedAt: 0,
      }));
    },
  );
});

describe("CsmCaseDetailPage — canonical-route redirect carries ?tab= and #hash forward", () => {
  it("carries the current ?tab= and hash onto the canonical route when a case is opened on a non-canonical one", async () => {
    useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
      data: id ? { ...buildCase(id), caseType: "service_request" } : undefined,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isFetching: false,
      dataUpdatedAt: 0,
    }));

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter
          initialEntries={["/cases/case-1?tab=attachments#entry-9"]}
        >
          <NavigateBridge />
          <LocationSearchProbe />
          <Routes>
            <Route path="/cases/:caseId" element={<CsmCaseDetailPage />} />
            <Route
              path="/operations/service-requests/:caseId"
              element={<CsmCaseDetailPage />}
            />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    // `#entry-9` is a permalink fragment, so once the fixed
    // `permalinkForceRef` logic (see CsmCaseDetailPage.tsx) sees it on this
    // cold load it forces the tab to Activities regardless of the `?tab=`
    // the URL was opened with — same as any other cold load with a fragment,
    // canonical route or not. `setActiveTab`'s own `setSearchParams` call
    // doesn't preserve the hash, which is why it's gone from the settled
    // URL too; that's pre-existing behaviour of every tab switch, not new
    // here.
    await waitFor(() =>
      expect(screen.getByTestId("location-search-probe")).toHaveTextContent(
        "/operations/service-requests/case-1?tab=activities",
      ),
    );

    // Restore the default mock for every test after this one.
    useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
      data: id ? buildCase(id) : undefined,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isFetching: false,
      dataUpdatedAt: 0,
    }));
  });
});

describe("CsmCaseDetailPage — permalink fragment forces the Activities tab", () => {
  it("forces Activities on a cold load that already has a permalink fragment, even when ?tab= names a different tab", () => {
    renderPageAt("/cases/case-1?tab=attachments#entry-9");

    expect(screen.getByRole("tab", { name: /activities/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });
});

describe("CsmCaseDetailPage — dashless id normalization", () => {
  it("fetches the case detail with the dashed id and redirects the URL when the route carries a dashless id", async () => {
    useGetCsmCaseDetailMock.mockClear();
    navigateMock.mockClear();

    renderPageAtCaseId(`/cases/${DASHLESS_ID}`);

    // The underlying data-fetch hook must be called with the corrected,
    // dashed id, not the raw dashless one straight off the URL.
    expect(useGetCsmCaseDetailMock).toHaveBeenCalledWith(DASHED_ID);

    // Exercise the real router replacement: the address bar must actually
    // land on the dashed path, not just the mock being called with it.
    await waitFor(() =>
      expect(screen.getByTestId("location-probe")).toHaveTextContent(
        `/cases/${DASHED_ID}`,
      ),
    );
  });

  it("does not redirect or alter an already-dashed id", () => {
    useGetCsmCaseDetailMock.mockClear();
    navigateMock.mockClear();

    renderPageAtCaseId(`/cases/${DASHED_ID}`);

    expect(useGetCsmCaseDetailMock).toHaveBeenCalledWith(DASHED_ID);
    expect(navigateMock).not.toHaveBeenCalled();
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      `/cases/${DASHED_ID}`,
    );
  });
});

describe("CsmCaseDetailPage — tab label counts", () => {
  it("shows the time cards count on the 'Time tracking' tab, sourced from useCaseTimeCards", () => {
    renderPage();

    // Mocked useCaseTimeCards above returns 2 cards -- this was previously
    // always 0 because the count read `c.timeLogs.length`, and `timeLogs` is
    // hardcoded to `[]` in useGetCsmCaseDetail and never populated.
    expect(
      screen.getByRole("tab", { name: /time tracking \(2\)/i }),
    ).toBeInTheDocument();
  });

  it("shows the case's real total on a truncated time-cards page, not the fetched page size", () => {
    // A case with more time cards than the fetch page limit: only 2 cards
    // came back, but the case really has 9. The tab badge must read `total`,
    // not `cards.length`, or it under-reports the case's real count.
    useCaseTimeCardsMock.mockImplementation(() => ({
      data: { cards: [{}, {}], total: 9, truncated: true },
    }));

    renderPage();

    expect(
      screen.getByRole("tab", { name: /time tracking \(9\)/i }),
    ).toBeInTheDocument();
  });
});

describe("CsmCaseDetailPage — time-card edit dialog reset on case change", () => {
  it("stops showing the previous case's edit dialog once the route moves to a new case", () => {
    renderPage();

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/cases/case-1",
    );

    // Switch to the "Time tracking" tab, where CaseTimeCardsPanel (stubbed
    // above) lives, and open the edit dialog for a card on case-1.
    fireEvent.click(screen.getByRole("tab", { name: /time tracking/i }));
    fireEvent.click(
      screen.getByRole("button", { name: /edit fake time card/i }),
    );

    expect(screen.getByTestId("log-time-card-dialog-probe")).toHaveTextContent(
      "editingCardId=card-1 caseId=case-1",
    );

    // Navigate to a different case through a real router transition (same
    // route, only :caseId changes — this page stays mounted, so the
    // render-time reset block is what has to run).
    fireEvent.click(screen.getByRole("button", { name: /go to case 2/i }));

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/cases/case-2",
    );

    // Without `setEditTimeCard(null)` in the reset block, the dialog would
    // still be mounted here, showing case-1's card while the rest of the
    // page has already moved on to case-2.
    expect(
      screen.queryByTestId("log-time-card-dialog-probe"),
    ).not.toBeInTheDocument();
  });
});

// Fires real navigate() calls between case-1 and case-2, both directions —
// the A -> B -> A regression below needs a return trip, unlike
// NavigateToCaseTwoButton (one-way, used by the simpler dialog-reset tests).
function NavigateBetweenCasesButtons(): JSX.Element {
  const navigate = useNavigate();
  return (
    <>
      <button type="button" onClick={() => navigate("/cases/case-2")}>
        Go to case 2
      </button>
      <button type="button" onClick={() => navigate("/cases/case-1")}>
        Go to case 1
      </button>
    </>
  );
}

describe("CsmCaseDetailPage — request-update stale-callback guard", () => {
  it("ignores a request-update response that arrives after navigating away and back to the same case", () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={["/cases/case-1"]}>
          <NavigateBetweenCasesButtons />
          <LocationProbe />
          <Routes>
            <Route path="/cases/:caseId" element={<CsmCaseDetailPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    // Open the dialog and submit while still on case-1; capture the mutate
    // options instead of letting the mock resolve immediately, so the
    // response can be replayed later — simulating a slow network response.
    fireEvent.click(screen.getByRole("button", { name: /stub open request update/i }));
    fireEvent.click(screen.getByRole("button", { name: /stub save request update/i }));
    expect(requestCaseUpdateMutateMock).toHaveBeenCalledTimes(1);
    const [, mutateOptions] = requestCaseUpdateMutateMock.mock.calls[0] as [
      unknown,
      { onSuccess: () => void },
    ];

    // Navigate away and back to case-1 — a real round trip, not a no-op:
    // caseId is identical before and after, which is exactly what a plain
    // caseId comparison in the mutation guard can't tell apart from "never
    // left". The view token (bumped on every transition, including this
    // return trip) is what makes the guard correctly treat the pending
    // request as stale here.
    fireEvent.click(screen.getByRole("button", { name: /go to case 2/i }));
    fireEvent.click(screen.getByRole("button", { name: /go to case 1/i }));
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/cases/case-1",
    );

    // The delayed response for the *original* case-1 visit arrives now.
    act(() => {
      mutateOptions.onSuccess();
    });

    // A caseId-only guard would treat this as still current (case-1 ==
    // case-1) and show the success toast; the view-token guard correctly
    // drops it since this is a different visit to case-1 than the one that
    // submitted the request.
    expect(
      screen.queryByText(/update request posted/i),
    ).not.toBeInTheDocument();
  });
});

// An announcement's own body never arrives as the Activities feed's opening
// comment the way it does for every other case type — it has to be rendered
// directly (see the note above the description card in the page).
function renderCaseDetailPage(
  path: string,
  routePattern: string,
  caseType: BeCaseType | undefined,
  description: string,
  state?: BeCaseState,
): ReturnType<typeof render> {
  useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
    data: id ? buildCase(id, { caseType, description, state }) : undefined,
    isLoading: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    isFetching: false,
    dataUpdatedAt: 0,
  }));
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path={routePattern} element={<CsmCaseDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CsmCaseDetailPage — announcement description rendering", () => {
  it("renders the case description inline in the activity timeline for an announcement, attributed to the creator with no role chip", () => {
    renderCaseDetailPage(
      "/announcements/case-1",
      "/announcements/:caseId",
      "announcement",
      "<p>Long advisory content</p>",
    );

    expect(screen.getByText("Long advisory content")).toBeInTheDocument();
    // Not just presence — the synthetic entry must render below the
    // Activity timeline heading, not above or interleaved with it.
    expect(
      screen
        .getByText("Activity timeline")
        .compareDocumentPosition(screen.getByText("Long advisory content")) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    // Rendered as a comment-like bubble attributed to the case creator
    // (falls back to the customer context's primary contact in this
    // fixture), not a visually distinct "Description" card.
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.queryByText("Description")).not.toBeInTheDocument();
    // The synthetic entry carries `authorRole: "customer"` only to pick the
    // neutral avatar styling — the real role is unknown, so no role chip
    // ("Customer"/"WSO2"/etc.) should render for it.
    expect(screen.queryByText("Customer")).not.toBeInTheDocument();
  });

  it("also injects the description as a synthetic comment entry on the Activities tab for a non-announcement case whose description isn't echoed anywhere", () => {
    // A plain case reaches the same synthetic entry via
    // `descriptionEchoedInOriginComment` rather than the isAnnouncement
    // carve-out — with no comments loaded (the default mock), there's no
    // origin comment to echo the description, so it must still show up here
    // instead of leaving it findable only on the Details tab.
    renderCaseDetailPage(
      "/cases/case-1",
      "/cases/:caseId",
      "case",
      "<p>Long advisory content</p>",
    );

    expect(screen.getByText("Long advisory content")).toBeInTheDocument();
    expect(screen.queryByText("Description")).not.toBeInTheDocument();
  });

  it("does not inject a synthetic entry on the Activities tab for a non-announcement case whose origin comment already echoes the description", () => {
    useGetCsmCaseCommentsMock.mockImplementation(() => ({
      data: [
        {
          id: "comment-1",
          caseId: "case-1",
          authorName: "Jane Doe",
          authorRole: "customer",
          bodyHtml: "<p>Long advisory content</p>",
          createdAt: "2026-01-01T00:00:00Z",
        },
      ],
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
      isFetching: false,
    }));

    renderCaseDetailPage(
      "/cases/case-1",
      "/cases/:caseId",
      "case",
      "<p>Long advisory content</p>",
    );

    // Only the real comment's copy of the content renders, not a second
    // (synthetic) copy — the real comment already covers it.
    expect(screen.getAllByText("Long advisory content")).toHaveLength(1);
  });

  it("does not inject a synthetic entry for an announcement whose origin comment already echoes the description", () => {
    // Regression guard: `safeComments` gates purely on
    // `descriptionEchoedInOriginComment`, with no separate `isAnnouncement`
    // carve-out — if announcement creation ever starts producing a real
    // echoed origin comment (it doesn't today), this must still suppress the
    // synthetic entry rather than always appending it for announcements.
    useGetCsmCaseCommentsMock.mockImplementation(() => ({
      data: [
        {
          id: "comment-1",
          caseId: "case-1",
          authorName: "Jane Doe",
          authorRole: "customer",
          bodyHtml: "<p>Long advisory content</p>",
          createdAt: "2026-01-01T00:00:00Z",
        },
      ],
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
      isFetching: false,
    }));

    renderCaseDetailPage(
      "/announcements/case-1",
      "/announcements/:caseId",
      "announcement",
      "<p>Long advisory content</p>",
    );

    expect(screen.getAllByText("Long advisory content")).toHaveLength(1);
  });

  it("renders nothing when an announcement has a blank description", () => {
    renderCaseDetailPage(
      "/announcements/case-1",
      "/announcements/:caseId",
      "announcement",
      "",
    );

    expect(screen.queryByText("Description")).not.toBeInTheDocument();
  });
});

// The comment/work-note composer used to be suppressed outright for
// announcements (`isAnnouncement ? null : …`) — the entity now accepts
// replies on an announcement the same as any other open case (see the
// backend's announcement carve-out in CreateCaseComment), so the page must
// show the same collapsed-until-clicked composer toggle it already shows for
// a plain case, and only gate it on the case being closed.
describe("CsmCaseDetailPage — announcement comment composer", () => {
  it("shows the collapsed 'Add comment' toggle for an open announcement and reveals the composer on click", () => {
    renderCaseDetailPage(
      "/announcements/case-1",
      "/announcements/:caseId",
      "announcement",
      "<p>Advisory content</p>",
      "open",
    );

    const toggle = screen.getByRole("button", {
      name: /compose a reply to the customer/i,
    });
    expect(toggle).toBeEnabled();

    // Composer content itself is a mocked stub (see the CsmCaseCommentInput
    // mock above) — what this page owns is the reveal chrome around it: the
    // "Reply" header and its Cancel action only exist once composerOpen is
    // true.
    expect(screen.queryByText("Reply")).not.toBeInTheDocument();
    fireEvent.click(toggle);
    expect(screen.getByText("Reply")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /cancel/i }),
    ).toBeInTheDocument();
  });

  it("disables the 'Add comment' toggle for a closed announcement", () => {
    renderCaseDetailPage(
      "/announcements/case-1",
      "/announcements/:caseId",
      "announcement",
      "<p>Advisory content</p>",
      "closed",
    );

    const toggle = screen.getByRole("button", {
      name: /this case is closed — comments and work notes are read-only/i,
    });
    expect(toggle).toBeDisabled();
  });
});

describe("CsmCaseDetailPage — Request details card", () => {
  function mockCaseType(caseType?: string): void {
    useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
      data: id ? { ...buildCase(id), caseType } : undefined,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isFetching: false,
      dataUpdatedAt: 0,
    }));
  }

  afterEach(() => {
    useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
      data: id ? buildCase(id) : undefined,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isFetching: false,
      dataUpdatedAt: 0,
    }));
  });

  it("renders the card in the Details tab of a service request", () => {
    mockCaseType("service_request");

    renderPageAt(
      "/operations/service-requests/case-1?tab=details",
      "/operations/service-requests/:caseId",
    );

    expect(screen.getByTestId("request-details-widget")).toBeInTheDocument();
  });

  it("does not render the card in the Details tab of a plain case", () => {
    mockCaseType(undefined);

    renderPageAt("/cases/case-1?tab=details");

    expect(
      screen.queryByTestId("request-details-widget"),
    ).not.toBeInTheDocument();
  });

  // Not covered here: a service request opened through the generic
  // /cases/:id route. The page's canonical-redirect gate bounces it to
  // /operations/service-requests/:id behind a skeleton before any tab body
  // renders, so `caseType`'s half of the signal can't be observed from that
  // entry point. It is the same `isServiceRequest` value either way — the
  // page computes it once (route || caseType) and this card reuses it.
});

describe("CsmCaseDetailPage — Watchers tab", () => {
  function openWatchers(): void {
    useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
      data: id
        ? {
            ...buildCase(id),
            watchers: [
              {
                id: WATCHER_ID,
                name: "Jane Doe",
                email: "jane.doe@example.com",
              },
            ],
          }
        : undefined,
      isLoading: false,
      isError: false,
    }));
    renderPage();
    fireEvent.click(screen.getByRole("tab", { name: /watchers/i }));
  }

  it("renders the watch list as a case's, so the last-watcher rule applies", () => {
    openWatchers();
    expect(screen.getByTestId("watchers-widget")).toHaveAttribute(
      "data-entity-kind",
      "case",
    );
  });

  it("PATCHes the whole replacement list, keeping the watchers already on it", () => {
    openWatchers();
    fireEvent.click(screen.getByRole("button", { name: /stub add watcher/i }));

    expect(patchCaseMutateMock).toHaveBeenCalledWith(
      { watchList: [WATCHER_ID, NEW_WATCHER_ID] },
      expect.anything(),
    );
  });

  it("surfaces the backend's own message when the write is rejected, leaving the list untouched", () => {
    openWatchers();
    fireEvent.click(screen.getByRole("button", { name: /stub add watcher/i }));

    const handlers = patchCaseMutateMock.mock.calls.at(-1)?.[1] as {
      onError: (err: unknown) => void;
    };
    handlers.onError(
      new BackendApiError(400, 'watchList contains invalid UUID: "not-a-uuid"'),
    );

    expect(showErrorMock).toHaveBeenCalledWith(
      'watchList contains invalid UUID: "not-a-uuid"',
      expect.anything(),
    );
    // No optimistic write happened, so nothing needs unwinding: the widget is
    // still showing the server's list.
    expect(screen.getByTestId("watchers-widget")).toBeInTheDocument();
  });
});

describe("CsmCaseDetailPage — change case type", () => {
  it("PATCHes type, engagementType, and engagementPaymentType together for an engagement transfer", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /stub open change case type/i }));
    fireEvent.click(screen.getByRole("button", { name: /stub transfer to engagement/i }));

    expect(patchCaseMutateMock).toHaveBeenCalledWith(
      { type: "engagement", engagementType: "migration", engagementPaymentType: "paid" },
      expect.anything(),
    );
  });

  it("sends the type, severity and issue type in a single PATCH", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /stub open change case type/i }));
    fireEvent.click(screen.getByRole("button", { name: /stub transfer to case with severity/i }));

    // One atomic call: the backend requires severity and issueType alongside `type`,
    // and rejects a standalone severity patch on a case of any other type.
    expect(patchCaseMutateMock).toHaveBeenCalledWith(
      { type: "case", severity: "high", issueType: "error" },
      expect.anything(),
    );
    // and crucially no standalone severity follow-up, which the backend rejects
    // on a case whose type is not already Incident/Query.
    expect(patchCaseMutateMock).not.toHaveBeenCalledWith(
      { severity: "high" },
      expect.anything(),
    );
  });
});

describe("CsmCaseDetailPage — Linked change requests widget only shows on service requests", () => {
  it("does not render LinkedChangeRequestsWidget for a plain case, even one carrying stale linkedChangeRequests data", () => {
    // A plain case should never carry `linkedChangeRequests` per the field's
    // own doc comment (SR-only), but the guard must not rely on that alone —
    // this proves the render gate itself, not just the data shape, keeps the
    // widget off a plain case.
    useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
      data: id
        ? {
            ...buildCase(id),
            caseType: "incident",
            linkedChangeRequests: [{ id: "cr-1", number: "CR-1", name: "Stale link" }],
          }
        : undefined,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isFetching: false,
      dataUpdatedAt: 0,
    }));

    renderPage();
    fireEvent.click(screen.getByRole("tab", { name: /linked items/i }));

    expect(
      screen.queryByTestId("linked-change-requests-widget-probe"),
    ).not.toBeInTheDocument();
  });

  it("renders LinkedChangeRequestsWidget for a service request", () => {
    useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
      data: id ? { ...buildCase(id), caseType: "service_request" } : undefined,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isFetching: false,
      dataUpdatedAt: 0,
    }));

    // A service request's canonical route is /operations/service-requests/:id
    // (see CASE_ROUTE_MOUNTS above) — mounting it on the plain /cases/:id
    // route instead would trip the canonical-redirect skeleton rather than
    // rendering the tabs.
    renderPageAt(
      "/operations/service-requests/case-1",
      "/operations/service-requests/:caseId",
    );
    fireEvent.click(screen.getByRole("tab", { name: /linked items/i }));

    expect(
      screen.getByTestId("linked-change-requests-widget-probe"),
    ).toBeInTheDocument();
  });
});
