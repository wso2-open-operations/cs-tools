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
import { MemoryRouter } from "react-router";
import "@testing-library/jest-dom/vitest";

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useQuickCaseSearch.test.tsx). `UserRefLink`
// (used for the attachment uploader) resolves an unknown id through
// `useResolvedUserId`, which calls this client — the attachments in this file
// carry no `uploadedByUser`, so it never actually fires, but the mock keeps
// the hook's `useBackendApi()` call from throwing on missing runtime config.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: vi.fn().mockResolvedValue({ users: [] }) }),
}));

import CaseActivitiesFeed from "@features/csm-cases/components/CaseActivitiesFeed";
import { formatAbsoluteForUser } from "@utils/dateTime";
import type { BeAlertDetail, BeCallRequestView } from "@api/backend/types";
import { useGetAlert, useGetSmartAlert } from "@features/csm-cases/api/useSnLinkEntities";
import type {
  CaseAttachment,
  CaseAuditEntry,
  CaseFeedbackEntry,
  CsmCaseComment,
} from "@features/csm-cases/types/csmCases";
import type { AttachmentPreviewSource } from "@features/csm-cases/utils/attachmentPreview";

vi.mock("@features/csm-cases/api/useSnLinkEntities", () => ({
  useGetAlert: vi.fn(),
  useGetSmartAlert: vi.fn(),
}));

const mockedUseGetAlert = vi.mocked(useGetAlert);
const mockedUseGetSmartAlert = vi.mocked(useGetSmartAlert);

// `UserRefLink` (used for the attachment uploader and the comment/lifecycle
// actor) renders a `react-router` `Link` and resolves its id through
// react-query, so every render needs both a Router and a QueryClient context
// even outside a full app render.
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

// `previewTarget`/`onPreviewTargetChange` (part of the feed's `preview`
// prop) are lifted to the parent page (see CsmCaseDetailPage) so the
// preview dialog is shared with the Attachments tab's widget. This harness
// owns that bit of state locally, standing in for the parent, and keeps the
// flat `onGetPreviewContent` shape for individual tests below so only this
// harness needs to know about the grouped `preview` prop.
function CaseActivitiesFeedHarness({
  onGetPreviewContent,
  ...props
}: Omit<ComponentProps<typeof CaseActivitiesFeed>, "preview"> & {
  onGetPreviewContent?: (
    attachment: CaseAttachment,
  ) => Promise<AttachmentPreviewSource>;
}): JSX.Element {
  const [previewTarget, setPreviewTarget] = useState<CaseAttachment | null>(
    null,
  );
  return (
    <CaseActivitiesFeed
      {...props}
      preview={
        onGetPreviewContent
          ? { onGetPreviewContent, previewTarget, onPreviewTargetChange: setPreviewTarget }
          : undefined
      }
    />
  );
}

describe("CaseActivitiesFeed — feedback lane", () => {
  it("renders a Case Feedback entry with rating, comment, and submitter", () => {
    const feedback: CaseFeedbackEntry = {
      id: "fb-1",
      rating: 5,
      ratingLabel: "Very Satisfied",
      comment: "Great support, thank you!",
      submittedAt: "2026-08-17T06:17:56Z",
      submitterName: "Jane Customer",
      submitterEmail: "jane.customer@example.com",
    };

    renderWithRouter(
      <CaseActivitiesFeed
        comments={[]}
        audit={[]}
        attachments={[]}
        feedback={[feedback]}
      />,
    );

    expect(screen.getByText("Case Feedback")).toBeInTheDocument();
    expect(screen.getByText("Very Satisfied (5/5)")).toBeInTheDocument();
    expect(screen.getByText("Great support, thank you!")).toBeInTheDocument();
    expect(screen.getByText("Jane Customer")).toBeInTheDocument();
  });

  it("falls back to 'Customer' when the submitter could not be resolved", () => {
    const feedback: CaseFeedbackEntry = {
      id: "fb-3",
      rating: 4,
      ratingLabel: "Satisfied",
      comment: null,
      submittedAt: "2026-08-17T06:17:56Z",
      submitterName: null,
      submitterEmail: null,
    };

    renderWithRouter(
      <CaseActivitiesFeed
        comments={[]}
        audit={[]}
        attachments={[]}
        feedback={[feedback]}
      />,
    );

    expect(screen.getByText("Customer")).toBeInTheDocument();
  });

  it("renders no comment line when feedback has none", () => {
    const feedback: CaseFeedbackEntry = {
      id: "fb-2",
      rating: 3,
      ratingLabel: "Neutral",
      comment: null,
      submittedAt: "2026-08-17T06:17:56Z",
    };

    renderWithRouter(
      <CaseActivitiesFeed
        comments={[]}
        audit={[]}
        attachments={[]}
        feedback={[feedback]}
      />,
    );

    expect(screen.getByText("Neutral (3/5)")).toBeInTheDocument();
  });

  it("shows no feedback lane and no filter option when feedback is empty", () => {
    renderWithRouter(
      <CaseActivitiesFeed comments={[]} audit={[]} attachments={[]} feedback={[]} />,
    );

    expect(screen.queryByText("Case Feedback")).not.toBeInTheDocument();
    fireEvent.click(screen.getByText("Filter"));
    expect(screen.queryByText(/^Feedback \(/)).not.toBeInTheDocument();
  });
});

describe("CaseActivitiesFeed", () => {
  it("renders a field_change entry with old/new values", () => {
    const entry: CaseAuditEntry = {
      id: "fc-1",
      kind: "field_change",
      actor: "Jane Doe",
      createdAt: "2026-07-01T00:00:00Z",
      changes: [
        {
          field: "state",
          fieldLabel: "State",
          previousValue: "In Progress",
          newValue: "Resolved",
        },
      ],
    };

    const { container } = renderWithRouter(
      <CaseActivitiesFeed comments={[]} audit={[entry]} attachments={[]} />,
    );

    expect(screen.getByText("State:")).toBeInTheDocument();
    // The line reads "State: In Progress → Resolved" across sibling text nodes
    // (muted old value, arrow, then new value) — assert on combined row text.
    expect(container.textContent).toContain("In Progress");
    expect(container.textContent).toContain("→");
    expect(container.textContent).toContain("Resolved");
    expect(screen.getByText(/Jane Doe/)).toBeInTheDocument();
  });

  it("renders multiple field changes from one field_change entry", () => {
    const entry: CaseAuditEntry = {
      id: "fc-2",
      kind: "field_change",
      actor: "Jane Doe",
      createdAt: "2026-07-01T00:00:00Z",
      changes: [
        {
          field: "state",
          fieldLabel: "State",
          previousValue: "New",
          newValue: "In Progress",
        },
        {
          field: "assignedEngineer",
          fieldLabel: "Assignee",
          previousValue: undefined,
          newValue: "John Smith",
        },
      ],
    };

    renderWithRouter(
      <CaseActivitiesFeed comments={[]} audit={[entry]} attachments={[]} />,
    );

    expect(screen.getByText("State:")).toBeInTheDocument();
    expect(screen.getByText("Assignee:")).toBeInTheDocument();
    expect(screen.getByText("John Smith")).toBeInTheDocument();
    // No previous value on the "set" change — no strike-through text for it.
    expect(screen.queryByText("cleared")).not.toBeInTheDocument();
  });

  it("shows a cleared marker when newValue is empty", () => {
    const entry: CaseAuditEntry = {
      id: "fc-3",
      kind: "field_change",
      actor: "Jane Doe",
      createdAt: "2026-07-01T00:00:00Z",
      changes: [
        {
          field: "assignedEngineer",
          fieldLabel: "Assignee",
          previousValue: "John Smith",
          newValue: undefined,
        },
      ],
    };

    renderWithRouter(
      <CaseActivitiesFeed comments={[]} audit={[entry]} attachments={[]} />,
    );

    expect(screen.getByText("cleared")).toBeInTheDocument();
    expect(screen.getByText("John Smith")).toBeInTheDocument();
  });

  it("renders a comment-style header (author + permalinked time) above the changes", () => {
    const entry: CaseAuditEntry = {
      id: "fc-5",
      kind: "field_change",
      actor: "Jane Doe",
      createdAt: "2026-07-01T10:15:00Z",
      changes: [
        {
          field: "state",
          fieldLabel: "State",
          previousValue: "In Progress",
          newValue: "Resolved",
        },
      ],
    };

    renderWithRouter(
      <CaseActivitiesFeed comments={[]} audit={[entry]} attachments={[]} />,
    );

    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByText("Lifecycle")).toBeInTheDocument();
    // The time is a permalink anchor to the entry, same pattern comments use.
    const permalink = document.querySelector(`a[href="#${entry.id}"]`);
    expect(permalink).not.toBeNull();
  });

  it("links the actor to their profile when actorUser carries a resolvable id", () => {
    const entry: CaseAuditEntry = {
      id: "fc-8",
      kind: "field_change",
      actor: "Jane Doe",
      actorUser: { id: "user-1", email: "jane.doe@example.com", name: "Jane Doe" },
      createdAt: "2026-07-01T00:00:00Z",
      changes: [
        {
          field: "state",
          fieldLabel: "State",
          previousValue: "In Progress",
          newValue: "Resolved",
        },
      ],
    };

    renderWithRouter(
      <CaseActivitiesFeed comments={[]} audit={[entry]} attachments={[]} />,
    );

    const link = screen.getByRole("link", { name: "Jane Doe" });
    expect(link).toHaveAttribute("href", "/people/user-1");
  });

  it("renders the actor as plain text (no link) when the activity's email field is really a username, not an address", () => {
    // Real backend shape: activity entries never resolve `id`, and `email`
    // sometimes holds an automation account name (e.g. "system"/"guest")
    // rather than an address — `isPlausibleEmail` correctly refuses to
    // resolve it, and no id-lookup request should ever fire for it.
    const entry: CaseAuditEntry = {
      id: "fc-9",
      kind: "field_change",
      actor: "System",
      actorUser: { id: null, email: "system", name: "System" },
      createdAt: "2026-07-01T00:00:00Z",
      changes: [
        {
          field: "state",
          fieldLabel: "State",
          previousValue: "In Progress",
          newValue: "Resolved",
        },
      ],
    };

    renderWithRouter(
      <CaseActivitiesFeed comments={[]} audit={[entry]} attachments={[]} />,
    );

    expect(screen.getByText("System")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "System" })).not.toBeInTheDocument();
  });

  it("renders every backend-provided change line, even one matching the entry's own timestamp", () => {
    const entry: CaseAuditEntry = {
      id: "fc-6",
      kind: "field_change",
      actor: "Jane Doe",
      createdAt: "2026-07-01T10:15:00Z",
      changes: [
        {
          field: "state",
          fieldLabel: "State",
          previousValue: "In Progress",
          newValue: "Resolved",
        },
        {
          field: "resolvedAt",
          fieldLabel: "Resolved On",
          previousValue: undefined,
          // Field-level curation is a backend concern now — the FE renders
          // whatever `changes[]` it receives, with no client-side dropping.
          newValue: "2026-07-01 10:15:22",
        },
      ],
    };

    renderWithRouter(
      <CaseActivitiesFeed comments={[]} audit={[entry]} attachments={[]} />,
    );

    expect(screen.getByText("State:")).toBeInTheDocument();
    expect(screen.getByText("Resolved On:")).toBeInTheDocument();
  });

  it("does not suppress a timestamp change when it differs from the entry's own time", () => {
    const entry: CaseAuditEntry = {
      id: "fc-7",
      kind: "field_change",
      actor: "Jane Doe",
      createdAt: "2026-07-01T10:15:00Z",
      changes: [
        {
          field: "dueDate",
          fieldLabel: "Due Date",
          previousValue: undefined,
          newValue: "2026-08-15 09:00:00",
        },
      ],
    };

    const { container } = renderWithRouter(
      <CaseActivitiesFeed comments={[]} audit={[entry]} attachments={[]} />,
    );

    expect(screen.getByText("Due Date:")).toBeInTheDocument();
    // The timestamp value must be routed through the shared user-timezone
    // formatter, not shown as the raw backend string.
    expect(screen.queryByText("2026-08-15 09:00:00")).not.toBeInTheDocument();
    const expected = formatAbsoluteForUser("2026-08-15 09:00:00");
    expect(expected).not.toBeNull();
    expect(container.textContent).toContain(expected);
  });

  it("falls back to description when changes is absent", () => {
    const entry: CaseAuditEntry = {
      id: "fc-4",
      kind: "state_change",
      actor: "System",
      description: "Case moved to In Progress",
      createdAt: "2026-07-01T00:00:00Z",
    };

    renderWithRouter(
      <CaseActivitiesFeed comments={[]} audit={[entry]} attachments={[]} />,
    );

    expect(screen.getByText("Case moved to In Progress")).toBeInTheDocument();
  });
});

describe("CaseActivitiesFeed — call-request link in a comment", () => {
  const SYSID = "7a43e2d43b2a4b5091404c6aa5e45a41";
  const UUID = "7a43e2d4-3b2a-4b50-9140-4c6aa5e45a41";

  function makeComment(bodyHtml: string): CsmCaseComment {
    return {
      id: "c-1",
      caseId: "case-1",
      authorName: "Jane Doe",
      authorRole: "customer",
      bodyHtml,
      createdAt: "2026-07-01T00:00:00Z",
    };
  }

  const CALL_REQUEST: BeCallRequestView = {
    id: UUID,
    number: "CR-1001",
    reason: "Discuss upgrade path",
  };

  it("opens CallRequestDetailModal with the matching call request on marker click", () => {
    renderWithRouter(
      <CaseActivitiesFeed
        comments={[
          makeComment(
            `See https://sn-dev.example.com/sn_customerservice_customer_call.do?sys_id=${SYSID}`,
          ),
        ]}
        audit={[]}
        attachments={[]}
        callRequests={[CALL_REQUEST]}
      />,
    );

    const marker = screen.getByRole("button", { name: "View call request" });
    fireEvent.click(marker);

    expect(screen.getByText("Call request · CR-1001")).toBeInTheDocument();
    expect(screen.getByText("Discuss upgrade path")).toBeInTheDocument();
  });

  it("does nothing when the referenced call request is not in the fetched list", () => {
    renderWithRouter(
      <CaseActivitiesFeed
        comments={[
          makeComment(
            `See https://sn-dev.example.com/sn_customerservice_customer_call.do?sys_id=${SYSID}`,
          ),
        ]}
        audit={[]}
        attachments={[]}
        callRequests={[]}
      />,
    );

    const marker = screen.getByRole("button", { name: "View call request" });
    fireEvent.click(marker);

    expect(screen.queryByText(/^Call request/)).not.toBeInTheDocument();
  });
});

describe("CaseActivitiesFeed — alert/smart-alert link in a comment", () => {
  const ALERT_SYSID = "7a43e2d43b2a4b5091404c6aa5e45a41";
  const ALERT_UUID = "7a43e2d4-3b2a-4b50-9140-4c6aa5e45a41";
  const SMART_ALERT_SYSID = "00000000000000000000000000000001";
  const SMART_ALERT_UUID = "00000000-0000-0000-0000-000000000001";

  function makeComment(bodyHtml: string): CsmCaseComment {
    return {
      id: "c-1",
      caseId: "case-1",
      authorName: "Jane Doe",
      authorRole: "customer",
      bodyHtml,
      createdAt: "2026-07-01T00:00:00Z",
    };
  }

  const ALERT: BeAlertDetail = {
    id: ALERT_UUID,
    number: "ALT0001",
    severity: "Critical",
    source: "Prometheus",
  };

  beforeEach(() => {
    mockedUseGetAlert.mockReturnValue({
      data: ALERT,
      isLoading: false,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
    } as any);
    mockedUseGetSmartAlert.mockReturnValue({
      data: null,
      isLoading: false,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
    } as any);
  });

  it("opens AlertDetailModal on an alert marker click", () => {
    renderWithRouter(
      <CaseActivitiesFeed
        comments={[
          makeComment(
            `See https://sn-dev.example.com/u_custom_alert.do?sys_id=${ALERT_SYSID}`,
          ),
        ]}
        audit={[]}
        attachments={[]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "View alert" }));
    expect(screen.getByText("Alert · ALT0001")).toBeInTheDocument();
    expect(mockedUseGetAlert).toHaveBeenCalledWith(ALERT_UUID);
  });

  it("opens SmartAlertDetailModal (not-found state) on a smart-alert marker click", () => {
    renderWithRouter(
      <CaseActivitiesFeed
        comments={[
          makeComment(
            `https://sn-dev.example.com/u_smart_alert_buffer.do?sys_id=${SMART_ALERT_SYSID}`,
          ),
        ]}
        audit={[]}
        attachments={[]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "View smart alert" }));
    expect(screen.getByText(/could no longer be found/)).toBeInTheDocument();
    expect(mockedUseGetSmartAlert).toHaveBeenCalledWith(SMART_ALERT_UUID);
  });
});

describe("CaseActivitiesFeed — attachment preview affordance", () => {
  const IMAGE_ATTACHMENT: CaseAttachment = {
    id: "att-1",
    filename: "screenshot.png",
    size: 2048,
    contentType: "image/png",
    uploadedBy: "Jane Doe",
    uploadedAt: "2026-01-01T00:00:00Z",
  };
  const ZIP_ATTACHMENT: CaseAttachment = {
    id: "att-2",
    filename: "logs.zip",
    size: 8192,
    contentType: "application/zip",
    uploadedBy: "Jane Doe",
    uploadedAt: "2026-01-02T00:00:00Z",
  };

  beforeEach(() => {
    // jsdom has no object-URL implementation; stub both so the preview
    // dialog's blob -> object URL -> revoke lifecycle can run in tests.
    globalThis.URL.createObjectURL = vi.fn(() => "blob:mock-url");
    globalThis.URL.revokeObjectURL = vi.fn();
  });

  it("shows Preview only for an image attachment, when a fetcher is supplied", () => {
    renderWithRouter(
      <CaseActivitiesFeedHarness
        comments={[]}
        audit={[]}
        attachments={[IMAGE_ATTACHMENT, ZIP_ATTACHMENT]}
        onGetPreviewContent={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: `Preview ${IMAGE_ATTACHMENT.filename}` }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: `Preview ${ZIP_ATTACHMENT.filename}` }),
    ).not.toBeInTheDocument();
  });

  it("hides every Preview affordance when no fetcher is supplied", () => {
    renderWithRouter(
      <CaseActivitiesFeedHarness
        comments={[]}
        audit={[]}
        attachments={[IMAGE_ATTACHMENT]}
      />,
    );
    expect(
      screen.queryByRole("button", { name: /^preview /i }),
    ).not.toBeInTheDocument();
  });

  it("opens the fullscreen preview dialog with the fetched object URL and revokes it on close", async () => {
    const fetchContent = vi
      .fn()
      .mockResolvedValue({ url: "blob:mock-url", revoke: true });
    renderWithRouter(
      <CaseActivitiesFeedHarness
        comments={[]}
        audit={[]}
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

    fireEvent.click(screen.getByRole("button", { name: /close preview/i }));
    await waitFor(() =>
      expect(globalThis.URL.revokeObjectURL).toHaveBeenCalledWith(
        "blob:mock-url",
      ),
    );
  });

  it("still shows Download for a non-previewable attachment", () => {
    const onDownloadAttachment = vi.fn();
    renderWithRouter(
      <CaseActivitiesFeedHarness
        comments={[]}
        audit={[]}
        attachments={[ZIP_ATTACHMENT]}
        onDownloadAttachment={onDownloadAttachment}
        onGetPreviewContent={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: `Download ${ZIP_ATTACHMENT.filename}` }),
    ).toBeInTheDocument();
  });
});
