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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useState, type ComponentProps, type JSX, type ReactElement } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import "@testing-library/jest-dom/vitest";

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useQuickCaseSearch.test.tsx). `UserRefLink`
// (used for the watcher chip and the attachment uploader) resolves an
// unknown id through `useResolvedUserId`, which calls this client.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: vi.fn().mockResolvedValue({ users: [] }) }),
}));

import {
  AttachmentsWidget,
  CustomerContextWidget,
  EscalationWidget,
  RequestDetailsWidget,
  TagsWidget,
  WatchersWidget,
} from "@features/csm-cases/components/CaseDetailWidgets";
import { useSearchUsersByName } from "@api/useSearchUsersByName";
import type { WatchListMember } from "@features/csm-cases/components/CaseDetailWidgets";
import type {
  CaseAttachment,
  CaseCustomerContext,
  CaseEscalationRecord,
  CaseRequestVariable,
  CaseTag,
} from "@features/csm-cases/types/csmCases";
import type { ProjectDetails } from "@features/csm-projects/types/csmProjects";

// `previewTarget`/`onPreviewTargetChange` (part of the widget's `preview`
// prop) are lifted to the parent page (see CsmCaseDetailPage) so the preview
// dialog resets on case-to-case navigation. This harness owns that bit of
// state locally, standing in for the parent, and keeps the flat
// `onGetPreviewContent` shape for individual tests below so only this
// harness needs to know about the grouped `preview` prop.
function AttachmentsWidgetHarness({
  onGetPreviewContent,
  ...props
}: Omit<ComponentProps<typeof AttachmentsWidget>, "preview"> & {
  onGetPreviewContent?: (attachment: CaseAttachment) => Promise<Blob>;
}): JSX.Element {
  const [previewTarget, setPreviewTarget] = useState<CaseAttachment | null>(
    null,
  );
  return (
    <AttachmentsWidget
      {...props}
      preview={
        onGetPreviewContent
          ? { onGetPreviewContent, previewTarget, onPreviewTargetChange: setPreviewTarget }
          : undefined
      }
    />
  );
}

vi.mock("@api/useSearchUsersByName", () => ({
  useSearchUsersByName: vi.fn(),
}));

const mockUseSearchUsersByName = vi.mocked(useSearchUsersByName);

/** Two searchable people, one of whom (`WATCHER_TWO_ID`) is already watching. */
function mockCandidates(): void {
  mockUseSearchUsersByName.mockReturnValue({
    data: [
      {
        id: CANDIDATE_ID,
        userName: "jsmith",
        firstName: "Jane",
        lastName: "Smith",
        email: "jane.smith@example.com",
      },
      {
        id: WATCHER_TWO_ID,
        userName: "jsmith2",
        firstName: "John",
        lastName: "Smith",
        email: "john.smith@example.com",
      },
    ],
    isFetching: false,
    isError: false,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
  } as any);
}

const TAGS: CaseTag[] = [
  { id: "tag-1", label: "micro-gw" },
  { id: "tag-2", label: "ws-policy" },
];

// Watch-list entries are keyed by platform user UUID — that id is exactly what
// a write resends, so the fixtures use real UUID shapes rather than "w-1".
const WATCHER_ONE_ID = "00000000-0000-0000-0000-000000000001";
const WATCHER_TWO_ID = "00000000-0000-0000-0000-000000000002";
const CANDIDATE_ID = "00000000-0000-0000-0000-000000000003";

const WATCHERS: WatchListMember[] = [
  { id: WATCHER_ONE_ID, name: "Jane Doe", email: "jane.doe@example.com" },
  { id: WATCHER_TWO_ID, name: "John Smith", isMe: true },
];

const ONE_WATCHER: WatchListMember[] = [WATCHERS[0]];

// `WatchersWidget` links each watcher's name to their profile page via
// `UserRefLink`, which renders a `react-router` `Link` and resolves its id
// through react-query — needs both a Router and a QueryClient context even
// outside a full app render.
function renderWithRouter(ui: ReactElement): ReturnType<typeof render> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

// Renders `path`'s current location as plain text, so a test can assert a
// link actually navigated (not just that a href/route prop is present)
// without mocking `useNavigate`.
function LocationProbe(): JSX.Element {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

// Same intent as `renderWithRouter`, but wires up real routes plus a
// destination probe so account/project/team links can be asserted by
// clicking through to the target route, rather than mocking navigation.
function renderWithRoutes(
  ui: ReactElement,
  routes: string[],
): ReturnType<typeof render> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/case"]}>
        <Routes>
          <Route path="/case" element={ui} />
          {routes.map((path) => (
            <Route key={path} path={path} element={<LocationProbe />} />
          ))}
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("TagsWidget", () => {
  it("renders an empty state when there are no tags", () => {
    render(<TagsWidget tags={[]} />);
    expect(screen.getByText("No tags applied.")).toBeInTheDocument();
  });

  it("renders every tag as a chip", () => {
    render(<TagsWidget tags={TAGS} />);
    expect(screen.getByText("micro-gw")).toBeInTheDocument();
    expect(screen.getByText("ws-policy")).toBeInTheDocument();
  });

  it("calls onAdd when the Tag button is clicked", () => {
    const onAdd = vi.fn();
    render(<TagsWidget tags={TAGS} onAdd={onAdd} />);
    fireEvent.click(screen.getByRole("button", { name: /^tag$/i }));
    expect(onAdd).toHaveBeenCalled();
  });

  it("calls onRemove with the tag when its chip delete icon is clicked", () => {
    const onRemove = vi.fn();
    render(<TagsWidget tags={TAGS} onRemove={onRemove} />);
    const chip = screen.getByText("micro-gw").closest(".MuiChip-root");
    const deleteIcon = chip?.querySelector(".MuiChip-deleteIcon");
    expect(deleteIcon).toBeTruthy();
    fireEvent.click(deleteIcon as Element);
    expect(onRemove).toHaveBeenCalledWith(TAGS[0]);
  });

  it("omits the delete affordance when onRemove is not provided", () => {
    render(<TagsWidget tags={TAGS} />);
    const chip = screen.getByText("micro-gw").closest(".MuiChip-root");
    expect(chip?.querySelector(".MuiChip-deleteIcon")).toBeFalsy();
  });
});

const ESCALATION_HISTORY: CaseEscalationRecord[] = [
  {
    id: "esc-2",
    currentLevel: "2",
    previousLevel: "1",
    createdBy: "jane.doe@example.com",
    createdOn: "2026-08-02T00:00:00Z",
    reason: "Still unresolved after 24h.",
  },
  {
    id: "esc-1",
    currentLevel: "1",
    previousLevel: "0",
    createdBy: "john.smith@example.com",
    createdOn: "2026-08-01T00:00:00Z",
    reason: "Customer escalated via phone.",
  },
];

describe("EscalationWidget", () => {
  it("renders the current level badge", () => {
    render(<EscalationWidget currentLevel="2" history={[]} />);
    expect(screen.getByText("EL2 — Technology Unit Head")).toBeInTheDocument();
  });

  it("renders 'Not escalated' when currentLevel is null", () => {
    render(<EscalationWidget currentLevel={null} history={[]} />);
    expect(screen.getByText("Not escalated")).toBeInTheDocument();
  });

  it("renders an empty state when there is no escalation history", () => {
    render(<EscalationWidget currentLevel="0" history={[]} />);
    expect(screen.getByText("No escalations on this case.")).toBeInTheDocument();
  });

  it("renders every history entry with its level transition, actor, and reason", () => {
    render(<EscalationWidget currentLevel="2" history={ESCALATION_HISTORY} />);
    expect(screen.getByText("EL1 → EL2")).toBeInTheDocument();
    expect(screen.getByText("Not escalated → EL1")).toBeInTheDocument();
    expect(screen.getByText(/jane.doe@example.com/)).toBeInTheDocument();
    expect(screen.getByText(/john.smith@example.com/)).toBeInTheDocument();
    expect(screen.getByText("Still unresolved after 24h.")).toBeInTheDocument();
    expect(screen.getByText("Customer escalated via phone.")).toBeInTheDocument();
  });

  it("shows a loading state instead of the history while isHistoryLoading", () => {
    render(
      <EscalationWidget
        currentLevel="1"
        history={[]}
        isHistoryLoading
      />,
    );
    expect(screen.getByText("Loading escalation history…")).toBeInTheDocument();
  });

  it("shows an error state instead of the history when isHistoryError", () => {
    render(
      <EscalationWidget currentLevel="1" history={[]} isHistoryError />,
    );
    expect(
      screen.getByText("Could not load the escalation history."),
    ).toBeInTheDocument();
  });

  it("shows only Escalate at EL0", () => {
    const onEscalate = vi.fn();
    render(
      <EscalationWidget
        currentLevel="0"
        history={[]}
        onEscalate={onEscalate}
      />,
    );
    expect(screen.getByRole("button", { name: "Escalate" })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "De-escalate" }),
    ).not.toBeInTheDocument();
  });

  it("shows only De-escalate at EL5", () => {
    const onDeescalate = vi.fn();
    render(
      <EscalationWidget
        currentLevel="5"
        history={[]}
        onDeescalate={onDeescalate}
      />,
    );
    expect(
      screen.getByRole("button", { name: "De-escalate" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Escalate" }),
    ).not.toBeInTheDocument();
  });

  it("shows both actions between EL1 and EL4, each firing its own callback", () => {
    const onEscalate = vi.fn();
    const onDeescalate = vi.fn();
    render(
      <EscalationWidget
        currentLevel="2"
        history={[]}
        onEscalate={onEscalate}
        onDeescalate={onDeescalate}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Escalate" }));
    expect(onEscalate).toHaveBeenCalledTimes(1);
    expect(onDeescalate).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "De-escalate" }));
    expect(onDeescalate).toHaveBeenCalledTimes(1);
    expect(onEscalate).toHaveBeenCalledTimes(1);
  });

  it("disables both actions with a tooltip reason when actionDisabledReason is set", () => {
    render(
      <EscalationWidget
        currentLevel="2"
        history={[]}
        onEscalate={vi.fn()}
        onDeescalate={vi.fn()}
        actionDisabledReason="This case is closed — it's read-only."
      />,
    );
    expect(screen.getByRole("button", { name: "Escalate" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "De-escalate" })).toBeDisabled();
  });
});

/**
 * Opens the "Add a watcher" type-ahead and returns its listbox options. The
 * picker only queries once the dropdown is open, so a test has to open it
 * before any candidate is on screen.
 */
function openWatcherPicker(): HTMLElement[] {
  fireEvent.mouseDown(screen.getByRole("combobox", { name: /add a watcher/i }));
  return screen.getAllByRole("option");
}

function removeButton(name: string): HTMLElement {
  return screen.getByRole("button", {
    name: new RegExp(`remove ${name} from the watch list`, "i"),
  });
}

describe("WatchersWidget", () => {
  beforeEach(() => {
    mockCandidates();
  });

  it("names the record type in its empty state", () => {
    renderWithRouter(<WatchersWidget entityKind="case" watchers={[]} />);
    expect(screen.getByText("No one is watching this case.")).toBeInTheDocument();

    renderWithRouter(<WatchersWidget entityKind="incident" watchers={[]} />);
    expect(
      screen.getByText("No one is watching this incident."),
    ).toBeInTheDocument();
  });

  it("lists every watcher, marking the current user", () => {
    renderWithRouter(<WatchersWidget entityKind="case" watchers={WATCHERS} />);
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByText("John Smith (you)")).toBeInTheDocument();
  });

  it("renders read-only — no picker, no remove controls — when onReplace is omitted", () => {
    renderWithRouter(<WatchersWidget entityKind="case" watchers={WATCHERS} />);
    expect(
      screen.queryByRole("combobox", { name: /add a watcher/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /from the watch list/i }),
    ).not.toBeInTheDocument();
  });

  it("still offers the picker when nobody is watching yet, so the first watcher can be added", () => {
    const onReplace = vi.fn();
    renderWithRouter(
      <WatchersWidget entityKind="case" watchers={[]} onReplace={onReplace} />,
    );
    fireEvent.click(openWatcherPicker()[0]);
    expect(onReplace).toHaveBeenCalledWith([CANDIDATE_ID], "add");
  });

  it("adds by sending the whole existing list plus the new user, not just the new one", () => {
    const onReplace = vi.fn();
    renderWithRouter(
      <WatchersWidget entityKind="case" watchers={WATCHERS} onReplace={onReplace} />,
    );
    fireEvent.click(openWatcherPicker()[0]);
    expect(onReplace).toHaveBeenCalledWith(
      [WATCHER_ONE_ID, WATCHER_TWO_ID, CANDIDATE_ID],
      "add",
    );
  });

  it("keeps people who are already watching out of the picker", () => {
    renderWithRouter(
      <WatchersWidget entityKind="case" watchers={WATCHERS} onReplace={vi.fn()} />,
    );
    const options = openWatcherPicker();
    expect(options).toHaveLength(1);
    expect(options[0]).toHaveTextContent("Jane Smith");
  });

  it("removes by sending the whole list minus that watcher", () => {
    const onReplace = vi.fn();
    renderWithRouter(
      <WatchersWidget entityKind="case" watchers={WATCHERS} onReplace={onReplace} />,
    );
    fireEvent.click(removeButton("Jane Doe"));
    expect(onReplace).toHaveBeenCalledWith([WATCHER_TWO_ID], "remove");
  });

  it("blocks removing a case's only watcher, with the reason reachable by assistive tech", () => {
    const onReplace = vi.fn();
    renderWithRouter(
      <WatchersWidget entityKind="case" watchers={ONE_WATCHER} onReplace={onReplace} />,
    );

    const button = removeButton("Jane Doe");
    expect(button).toHaveAttribute("aria-disabled", "true");
    // Focusable, not `disabled` — a disabled button leaves the tab order and
    // takes the explanation for its own state with it.
    expect(button).not.toBeDisabled();
    const reason = document.getElementById(
      button.getAttribute("aria-describedby") ?? "",
    );
    expect(reason).toHaveTextContent("A case must keep at least one watcher.");

    fireEvent.click(button);
    expect(onReplace).not.toHaveBeenCalled();
  });

  it("allows removing an incident's only watcher, clearing the list", () => {
    const onReplace = vi.fn();
    renderWithRouter(
      <WatchersWidget
        entityKind="incident"
        watchers={ONE_WATCHER}
        onReplace={onReplace}
      />,
    );

    const button = removeButton("Jane Doe");
    expect(button).not.toHaveAttribute("aria-disabled");
    fireEvent.click(button);
    expect(onReplace).toHaveBeenCalledWith([], "remove");
  });

  it("blocks add and remove while a write is in flight, so a double-click can't fire two replacements", () => {
    const onReplace = vi.fn();
    renderWithRouter(
      <WatchersWidget
        entityKind="case"
        watchers={WATCHERS}
        onReplace={onReplace}
        isSaving
      />,
    );

    fireEvent.click(removeButton("Jane Doe"));
    expect(onReplace).not.toHaveBeenCalled();
    expect(
      screen.getByRole("combobox", { name: /add a watcher/i }),
    ).toBeDisabled();
  });

  describe("self-subscribe", () => {
    it("omits the Follow/Unfollow control when no currentUserId is supplied", () => {
      renderWithRouter(
        <WatchersWidget entityKind="case" watchers={WATCHERS} onReplace={vi.fn()} />,
      );
      expect(
        screen.queryByRole("button", { name: /^follow case updates$/i }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: /^unfollow case updates$/i }),
      ).not.toBeInTheDocument();
    });

    it("shows Follow when the signed-in engineer isn't a watcher, and adds them on click", () => {
      const onReplace = vi.fn();
      // Neither fixture watcher is flagged `isMe` here — CANDIDATE_ID (the
      // signed-in engineer in this test) isn't on the list at all, matching
      // how the caller would populate `isMe` for a real not-yet-following user.
      renderWithRouter(
        <WatchersWidget
          entityKind="case"
          watchers={ONE_WATCHER}
          onReplace={onReplace}
          currentUserId={CANDIDATE_ID}
        />,
      );
      expect(
        screen.queryByRole("button", { name: /^unfollow case updates$/i }),
      ).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: /^follow case updates$/i }));
      expect(onReplace).toHaveBeenCalledWith([WATCHER_ONE_ID, CANDIDATE_ID], "add");
    });

    it("shows Unfollow when the signed-in engineer is already a watcher and not auto-added, and removes them on click", () => {
      const onReplace = vi.fn();
      renderWithRouter(
        <WatchersWidget
          entityKind="case"
          watchers={WATCHERS}
          onReplace={onReplace}
          currentUserId={WATCHER_TWO_ID}
        />,
      );
      expect(
        screen.queryByRole("button", { name: /^follow case updates$/i }),
      ).not.toBeInTheDocument();
      const button = screen.getByRole("button", { name: /^unfollow case updates$/i });
      expect(button).not.toHaveAttribute("aria-disabled");
      fireEvent.click(button);
      expect(onReplace).toHaveBeenCalledWith([WATCHER_ONE_ID], "remove");
    });

    it("blocks Unfollow, with the reason reachable by assistive tech, when the caller reports an auto-added membership (e.g. the case's assignee)", () => {
      const onReplace = vi.fn();
      renderWithRouter(
        <WatchersWidget
          entityKind="case"
          watchers={WATCHERS}
          onReplace={onReplace}
          currentUserId={WATCHER_TWO_ID}
          autoWatchingReason="You're on this case's watch list as its assigned engineer."
        />,
      );
      const button = screen.getByRole("button", { name: /^unfollow case updates$/i });
      expect(button).toHaveAttribute("aria-disabled", "true");
      const reason = document.getElementById(
        button.getAttribute("aria-describedby") ?? "",
      );
      expect(reason).toHaveTextContent(
        "You're on this case's watch list as its assigned engineer.",
      );
      fireEvent.click(button);
      expect(onReplace).not.toHaveBeenCalled();
    });
  });
});

describe("AttachmentsWidget — preview affordance", () => {
  const IMAGE_ATTACHMENT: CaseAttachment = {
    id: "att-1",
    filename: "screenshot.png",
    size: 2048,
    contentType: "image/png",
    uploadedBy: "Jane Doe",
    uploadedAt: "2026-01-01T00:00:00Z",
  };
  const VIDEO_ATTACHMENT: CaseAttachment = {
    id: "att-2",
    filename: "repro.mp4",
    size: 4096,
    contentType: "video/mp4",
    uploadedBy: "Jane Doe",
    uploadedAt: "2026-01-02T00:00:00Z",
  };
  const ZIP_ATTACHMENT: CaseAttachment = {
    id: "att-3",
    filename: "logs.zip",
    size: 8192,
    contentType: "application/zip",
    uploadedBy: "Jane Doe",
    uploadedAt: "2026-01-03T00:00:00Z",
  };

  beforeEach(() => {
    // jsdom has no object-URL implementation; stub both so the preview
    // dialog's blob -> object URL -> revoke lifecycle can run in tests.
    globalThis.URL.createObjectURL = vi.fn(() => "blob:mock-url");
    globalThis.URL.revokeObjectURL = vi.fn();
  });

  it("shows Preview for an image but not for a video or a zip, when a fetcher is supplied", () => {
    renderWithRouter(
      <AttachmentsWidgetHarness
        attachments={[IMAGE_ATTACHMENT, VIDEO_ATTACHMENT, ZIP_ATTACHMENT]}
        onGetPreviewContent={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: `Preview ${IMAGE_ATTACHMENT.filename}` }),
    ).toBeInTheDocument();
    // Video is not previewable: the backend's safe-content-type allowlist
    // (`safeAttachmentTypes` in case_handler.go) has no video/* entry, so
    // GET /attachments/{id}/content always coerces a video response to
    // application/octet-stream. Offering a preview button here would rely
    // on the uploader-controlled metadata `contentType` instead of the
    // backend-verified one, defeating that allowlist.
    expect(
      screen.queryByRole("button", { name: `Preview ${VIDEO_ATTACHMENT.filename}` }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: `Preview ${ZIP_ATTACHMENT.filename}` }),
    ).not.toBeInTheDocument();
  });

  it("hides every Preview affordance when no fetcher is supplied", () => {
    renderWithRouter(
      <AttachmentsWidgetHarness attachments={[IMAGE_ATTACHMENT, VIDEO_ATTACHMENT]} />,
    );
    expect(screen.queryByRole("button", { name: /^preview /i })).not.toBeInTheDocument();
  });

  it("opens the preview dialog, fetches content, and renders it as an image", async () => {
    const fetchContent = vi
      .fn()
      .mockResolvedValue(new Blob(["fake"], { type: "image/png" }));
    renderWithRouter(
      <AttachmentsWidgetHarness
        attachments={[IMAGE_ATTACHMENT]}
        onGetPreviewContent={fetchContent}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: `Preview ${IMAGE_ATTACHMENT.filename}` }),
    );

    expect(fetchContent).toHaveBeenCalledWith(IMAGE_ATTACHMENT);
    await waitFor(() =>
      expect(screen.getByAltText(IMAGE_ATTACHMENT.filename)).toBeInTheDocument(),
    );
    expect(screen.getByAltText(IMAGE_ATTACHMENT.filename)).toHaveAttribute(
      "src",
      "blob:mock-url",
    );
  });

  it("shows an error message when the preview fetch fails", async () => {
    const fetchContent = vi.fn().mockRejectedValue(new Error("network down"));
    renderWithRouter(
      <AttachmentsWidgetHarness
        attachments={[IMAGE_ATTACHMENT]}
        onGetPreviewContent={fetchContent}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: `Preview ${IMAGE_ATTACHMENT.filename}` }),
    );

    await waitFor(() =>
      expect(screen.getByText("network down")).toBeInTheDocument(),
    );
  });

  it("still shows Download for every attachment regardless of preview support", () => {
    renderWithRouter(
      <AttachmentsWidgetHarness
        attachments={[IMAGE_ATTACHMENT, ZIP_ATTACHMENT]}
        onDownload={vi.fn()}
        onGetPreviewContent={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: `Download ${IMAGE_ATTACHMENT.filename}` }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: `Download ${ZIP_ATTACHMENT.filename}` }),
    ).toBeInTheDocument();
  });
});

describe("CustomerContextWidget", () => {
  const CTX: CaseCustomerContext = {
    accountName: "Acme Corp",
    tier: "Enterprise",
    region: "US East",
    primaryContact: "Jane Doe",
    primaryContactEmail: "jane.doe@example.com",
    accountManager: "John Smith",
    openCases: 2,
  };

  const PROJECT: ProjectDetails = {
    id: "proj-1",
    account: {
      id: "acct-1",
      name: "Acme Corp",
      activationDate: null,
      tier: "Enterprise",
      agentEnabled: true,
      kbReferencesEnabled: true,
    },
    sfId: "sf-1",
    name: "Acme - Managed Cloud",
    key: "ACME",
    subscriptionType: "cloud_support",
    startDate: "2026-01-01",
    endDate: "2026-12-31",
    createdOn: "2026-01-01T00:00:00Z",
    updatedOn: "2026-01-01T00:00:00Z",
    closureState: null,
  };

  it("renders the account name as plain text when no accountId is supplied", () => {
    renderWithRouter(<CustomerContextWidget ctx={CTX} />);
    expect(screen.getByText("Acme Corp")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Acme Corp" })).not.toBeInTheDocument();
  });

  it("links the account name to its detail page when accountId is supplied", () => {
    renderWithRoutes(
      <CustomerContextWidget ctx={CTX} accountId="acct-1" />,
      ["/customers/accounts/:id"],
    );
    fireEvent.click(screen.getByRole("link", { name: "Acme Corp" }));
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/customers/accounts/acct-1",
    );
  });

  it("links the project name to its detail page", () => {
    renderWithRoutes(
      <CustomerContextWidget ctx={CTX} project={PROJECT} />,
      ["/customers/projects/:id"],
    );
    fireEvent.click(screen.getByRole("link", { name: PROJECT.name }));
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/customers/projects/proj-1",
    );
  });

  it("renders no CRE/SRE row when neither team is set", () => {
    renderWithRouter(<CustomerContextWidget ctx={CTX} />);
    expect(screen.queryByText("CRE / SRE team")).not.toBeInTheDocument();
  });

  it("renders CRE and SRE team chips as links to the team directory page", () => {
    renderWithRoutes(
      <CustomerContextWidget
        ctx={{
          ...CTX,
          creTeam: { id: "team-cre-1", name: "CRE Alpha" },
          sreTeam: { id: "team-sre-1", name: "SRE Beta" },
        }}
      />,
      ["/admin/teams/:id"],
    );
    expect(screen.getByText("CRE / SRE team")).toBeInTheDocument();

    fireEvent.click(screen.getByText("CRE Alpha"));
    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/admin/teams/team-cre-1",
    );
  });

  it("renders only the SRE chip when only the SRE team is set", () => {
    renderWithRouter(
      <CustomerContextWidget
        ctx={{ ...CTX, sreTeam: { id: "team-sre-1", name: "SRE Beta" } }}
      />,
    );
    expect(screen.getByText("SRE Beta")).toBeInTheDocument();
    expect(screen.queryByText("CRE Alpha")).not.toBeInTheDocument();
  });
});

describe("RequestDetailsWidget", () => {
  const CATALOG = { id: "catalog-1", name: "Managed Cloud" };
  const CATALOG_ITEM = { id: "catalog-item-1", name: "Product Update" };

  // Deliberately not in alphabetical order — the widget must preserve the
  // order the backing data source returned, not impose one of its own.
  const VARIABLES: CaseRequestVariable[] = [
    { name: "Reason For Migration", value: "End of support" },
    { name: "Approval Reference", value: "CHG-0001" },
    { name: "Additional Notes", value: "" },
  ];

  it("renders the catalog and catalog item", () => {
    renderWithRouter(
      <RequestDetailsWidget
        catalog={CATALOG}
        catalogItem={CATALOG_ITEM}
        variables={VARIABLES}
      />,
    );

    expect(screen.getByText("Catalog")).toBeInTheDocument();
    expect(screen.getByText("Managed Cloud")).toBeInTheDocument();
    expect(screen.getByText("Catalog item")).toBeInTheDocument();
    expect(screen.getByText("Product Update")).toBeInTheDocument();
  });

  it("renders an em dash for a catalog/catalog item the record does not carry", () => {
    renderWithRouter(
      <RequestDetailsWidget
        variables={[{ name: "Reason For Migration", value: "End of support" }]}
      />,
    );

    // Exactly two: the Catalog cell and the Catalog item cell. The single
    // answer is non-blank, so it contributes none.
    expect(screen.getAllByText("\u2014")).toHaveLength(2);
  });

  it("renders the answers in the order the backend returned them", () => {
    const { container } = renderWithRouter(
      <RequestDetailsWidget
        catalog={CATALOG}
        catalogItem={CATALOG_ITEM}
        variables={VARIABLES}
      />,
    );

    const questions = Array.from(container.querySelectorAll("dt")).map(
      (dt) => dt.textContent,
    );
    expect(questions).toEqual([
      "Reason For Migration",
      "Approval Reference",
      "Additional Notes",
    ]);
  });

  it("pairs each question with its answer", () => {
    const { container } = renderWithRouter(
      <RequestDetailsWidget
        catalog={CATALOG}
        catalogItem={CATALOG_ITEM}
        variables={VARIABLES}
      />,
    );

    const answers = Array.from(container.querySelectorAll("dd")).map(
      (dd) => dd.textContent,
    );
    expect(answers).toEqual(["End of support", "CHG-0001", "\u2014"]);
  });

  it("renders an em dash — not nothing — for a question that was asked and left blank", () => {
    const { container } = renderWithRouter(
      <RequestDetailsWidget
        catalog={CATALOG}
        catalogItem={CATALOG_ITEM}
        variables={[{ name: "Additional Notes", value: "" }]}
      />,
    );

    const dd = container.querySelector("dd");
    expect(dd).not.toBeNull();
    expect(dd).toHaveTextContent("\u2014");
  });

  it("renders the empty state rather than hiding the card when there are no answers", () => {
    renderWithRouter(
      <RequestDetailsWidget catalog={CATALOG} catalogItem={CATALOG_ITEM} />,
    );

    expect(screen.getByText("Request details")).toBeInTheDocument();
    expect(screen.getByText("No request details captured.")).toBeInTheDocument();
    expect(screen.getByText("Managed Cloud")).toBeInTheDocument();
  });

  it("renders the empty state for an explicitly empty answer list too", () => {
    renderWithRouter(<RequestDetailsWidget variables={[]} />);

    expect(screen.getByText("No request details captured.")).toBeInTheDocument();
  });
});
