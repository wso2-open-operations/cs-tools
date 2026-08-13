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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
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

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
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
function buildCase(id: string): CsmCaseDetail {
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
    state: "open",
    assignee: "Unassigned",
    assigneeIsMe: false,
    slaClockType: "resolution",
    minutesToBreach: 120,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    description: "<p>Sample description</p>",
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

const useGetCsmCaseDetailMock = vi.fn();
vi.mock("@features/csm-cases/api/useGetCsmCaseDetail", () => ({
  useGetCsmCaseDetail: (id: string | undefined) => useGetCsmCaseDetailMock(id),
}));
useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
  data: id ? buildCase(id) : undefined,
  isLoading: false,
  isError: false,
  error: null,
  refetch: vi.fn(),
  isFetching: false,
  dataUpdatedAt: 0,
}));

vi.mock("@features/csm-cases/api/usePatchCsmCase", () => ({
  usePatchCsmCase: () => ({
    mutate: vi.fn(),
    mutateAsync: vi.fn(),
    isPending: false,
  }),
  usePatchCsmCaseById: () => vi.fn(),
}));
vi.mock("@features/csm-cases/api/useFindMyOngoingCases", () => ({
  useFindMyOngoingCases: () => vi.fn(),
}));
vi.mock("@features/csm-cases/api/useCsmCaseComments", () => ({
  useGetCsmCaseComments: () => ({
    data: [],
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
    isFetching: false,
  }),
  usePostCsmCaseComment: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
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
vi.mock("@features/csm-timecards/api/useTimeCards", () => ({
  usePostTimeCard: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateTimeCard: () => ({ mutate: vi.fn(), isPending: false }),
}));

// Simple presentational stubs — none of this test's assertions touch these.
vi.mock("@features/csm-cases/components/CsmCaseCommentInput", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/CaseActionBar", () => ({
  default: () => null,
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
vi.mock("@features/csm-cases/components/ChildCasesWidget", () => ({
  ChildCasesWidget: () => null,
}));
vi.mock("@features/csm-cases/components/LinkedServiceRequestsWidget", () => ({
  LinkedServiceRequestsWidget: () => null,
}));
vi.mock("@features/csm-cases/components/LinkedChangeRequestsWidget", () => ({
  LinkedChangeRequestsWidget: () => null,
}));
vi.mock("@features/csm-cases/components/CreateGithubIssueDialog", () => ({
  CreateGithubIssueDialog: () => null,
}));
vi.mock("@features/csm-cases/components/CaseActivitiesFeed", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/CaseMetaBand", () => ({
  default: () => null,
}));
vi.mock("@features/csm-cases/components/CaseDetailWidgets", () => ({
  AttachmentsWidget: () => null,
  CustomerContextWidget: () => null,
  ProductContextWidget: () => null,
  TagsWidget: () => null,
  WatchersWidget: () => null,
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
import CsmCaseDetailPage from "@features/csm-cases/pages/CsmCaseDetailPage";

// Renders the real route pathname, so the test can assert the router itself
// actually transitioned (not just that the page re-rendered) — same
// convention as CsmAccountDetailPage.test.tsx's `LocationProbe`.
function LocationProbe(): JSX.Element {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
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

const DASHED_ID = "56f49f0a-eb1e-c310-fcf5-f5dabad0cdab";
const DASHLESS_ID = "56f49f0aeb1ec310fcf5f5dabad0cdab";

// `useNavTransition` is mocked module-wide (above) so the rest of this
// file's tests can assert on navigate-call arguments without a real router
// transition. The tab strip is now a real URL path segment (`usePathTabs`),
// so — same as the dashless-id tests below — the mock is bridged to the real
// `useNavigate` from this render tree, and `LocationProbe` verifies the
// resulting location, rather than mocking `useNavigate`/react-router
// wholesale (a full mock can't reflect a URL-driven tab).
function NavigateBridge(): null {
  const navigate = useNavigate();
  navigateMock.mockImplementation((to: To, options?: NavigateOptions) =>
    navigate(to, options),
  );
  return null;
}

function renderPage(): ReturnType<typeof render> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/cases/case-1"]}>
        <NavigateBridge />
        <NavigateToCaseTwoButton />
        <LocationProbe />
        <Routes>
          <Route path="/cases/:caseId/:tab?" element={<CsmCaseDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
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
          <Route path="/cases/:caseId/:tab?" element={<CsmCaseDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

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

describe("CsmCaseDetailPage — tab is a real URL path segment", () => {
  it("defaults to the Activities tab when the URL carries no tab segment", () => {
    renderPage();
    expect(screen.getByRole("tab", { name: /activities/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("clicking a tab navigates to that tab's own URL", () => {
    renderPage();

    fireEvent.click(screen.getByRole("tab", { name: /details/i }));

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/cases/case-1/details",
    );
    expect(screen.getByRole("tab", { name: /details/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("loading the page directly at a tab's URL opens on that tab", () => {
    render(
      <QueryClientProvider
        client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
      >
        <MemoryRouter initialEntries={["/cases/case-1/attachments"]}>
          <NavigateBridge />
          <LocationProbe />
          <Routes>
            <Route path="/cases/:caseId/:tab?" element={<CsmCaseDetailPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(screen.getByRole("tab", { name: /attachments/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("falls back to Activities for an unknown tab segment, without redirecting the URL", () => {
    render(
      <QueryClientProvider
        client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
      >
        <MemoryRouter initialEntries={["/cases/case-1/bogus-tab"]}>
          <NavigateBridge />
          <LocationProbe />
          <Routes>
            <Route path="/cases/:caseId/:tab?" element={<CsmCaseDetailPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(screen.getByRole("tab", { name: /activities/i })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    // No redirect loop / URL correction — the caller sees exactly the URL it
    // asked for, just rendered as the default tab.
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/cases/case-1/bogus-tab",
    );
  });

  it("preserves the case's own mount point (not /cases) when switching tabs from a non-canonical route", () => {
    // Engagements render CsmCaseDetailPage too, under their own mount point —
    // `usePathTabs`'s `basePath` is this page's already-resolved `detailPath`,
    // not a hardcoded "/cases" prefix, so a tab switch from an engagement must
    // stay under /engagements rather than jumping to the case's own path.
    // `mockImplementationOnce` isn't enough here: the page re-renders (and
    // re-calls this hook) more than once before the first paint settles, so a
    // one-shot override would revert to the default (non-engagement) case on
    // a later render and trip the page's own case-type/route mismatch
    // redirect. Set it for the duration of this test and restore the shared
    // default afterward so later tests in this file aren't affected.
    const defaultImpl = useGetCsmCaseDetailMock.getMockImplementation();
    useGetCsmCaseDetailMock.mockImplementation((id: string | undefined) => ({
      data: id ? { ...buildCase(id), caseType: "engagement" } : undefined,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
      isFetching: false,
      dataUpdatedAt: 0,
    }));

    render(
      <QueryClientProvider
        client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
      >
        <MemoryRouter initialEntries={["/engagements/case-1"]}>
          <NavigateBridge />
          <LocationProbe />
          <Routes>
            <Route path="/engagements/:caseId/:tab?" element={<CsmCaseDetailPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    fireEvent.click(screen.getByRole("tab", { name: /details/i }));

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/engagements/case-1/details",
    );

    if (defaultImpl) useGetCsmCaseDetailMock.mockImplementation(defaultImpl);
  });
});
