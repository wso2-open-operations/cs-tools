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


import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import CreateCasePage from "@features/support/pages/CreateCasePage";

const calls: string[] = [];
const mocks = vi.hoisted(() => ({
  uploadStoredAttachment: vi.fn(),
  confirmStoredUpload: vi.fn(),
  mutate: vi.fn(),
  storedUpload: vi.fn(),
  showError: vi.fn(),
}));

vi.mock("@api/storedAttachments", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@api/storedAttachments")>();
  return {
    ...actual,
    uploadStoredAttachment: mocks.uploadStoredAttachment,
    confirmStoredUpload: mocks.confirmStoredUpload,
  };
});

vi.mock("react-router", () => ({
  useNavigate: () => vi.fn(),
  useLocation: () => ({
    pathname: "/projects/project-1/support/chat/create-case",
    search: "",
    state: { skipChat: true },
  }),
  useParams: () => ({ projectId: "project-1" }),
}));
vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: () => ({ showLoader: vi.fn(), hideLoader: vi.fn() }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: mocks.showError }),
}));
vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));
vi.mock("@hooks/useLogger", () => ({ useLogger: () => ({ error: vi.fn() }) }));
// Stable references: the page's effects key off these by identity.
const PROJECT = { data: { name: "P1", type: { label: "Enterprise" } }, isLoading: false };
const FEATURES = { data: {}, isLoading: false };
const CONTACTS = { data: [] as unknown[], isLoading: false };
const USER = { data: { email: "user@example.com" } };
vi.mock("@api/useGetProjectDetails", () => ({ default: () => PROJECT }));
vi.mock("@api/useGetProjectFeatures", () => ({ default: () => FEATURES }));
const FILTERS = {
  data: {
    issueTypes: [{ id: "1", label: "Query" }],
    severities: [{ id: "4", label: "Low" }],
  },
  isLoading: false,
};
vi.mock("@api/useGetProjectFilters", () => ({ default: () => FILTERS }));

const INFINITE = {
  isLoading: false,
  isError: false,
  isFetchingNextPage: false,
  hasNextPage: false,
  fetchNextPage: vi.fn(),
};
const DEPLOYMENTS = {
  ...INFINITE,
  data: { pages: [{ deployments: [{ id: "dep-1", name: "Production" }] }] },
};
const PRODUCTS = {
  ...INFINITE,
  data: {
    pages: [
      { products: [{ id: "prod-1", product: { label: "Product A" }, version: "1.0.0" }] },
    ],
  },
};
vi.mock("@api/usePostProjectDeploymentsSearch", () => ({
  usePostProjectDeploymentsSearchInfinite: () => DEPLOYMENTS,
}));
vi.mock("@features/project-details/api/usePostDeploymentProductsSearch", () => ({
  usePostDeploymentProductsSearchInfinite: () => PRODUCTS,
  extractDeploymentProducts: (page: { products?: unknown[] }) => page?.products ?? [],
}));
vi.mock("@features/operations/api/usePostCase", () => ({
  usePostCase: () => ({ mutate: mocks.mutate, isPending: false }),
}));
vi.mock("@/hooks/useAuthApiClient", () => ({ useAuthApiClient: () => vi.fn() }));
vi.mock("@features/settings/api/useGetProjectContacts", () => ({
  default: () => CONTACTS,
}));
vi.mock("@features/settings/api/useGetUserDetails", () => ({
  default: () => USER,
}));
vi.mock("@api/useStoredAttachments", () => ({
  useStoredAttachmentUpload: () => ({
    upload: { mutateAsync: mocks.storedUpload, mutate: vi.fn() },
    confirm: { mutateAsync: vi.fn(), mutate: vi.fn() },
  }),
}));
vi.mock("@tanstack/react-query", async (importOriginal) => ({
  ...((await importOriginal()) as object),
  useQueryClient: () => ({ invalidateQueries: vi.fn() }),
}));
vi.mock("@utils/permission", () => ({
  filterDeploymentsForCaseCreation: (items: unknown[]) => items ?? [],
  getProjectSeverityPolicy: () => ({ excludeS0: false, restrictSeverityToLow: false }),
  shouldRestrictToPrimaryProductionDeployments: () => false,
}));
vi.mock("@features/operations/utils/caseRefresh", () => ({
  triggerPostCreationApiCalls: () => Promise.resolve(),
  refreshCaseQueriesAfterCreation: () => Promise.resolve(),
}));
vi.mock("@features/support/hooks/usePiiGuard", () => ({
  usePiiGuard: () => ({
    checkBeforeSubmit: (_text: string, onProceed: () => void) => onProceed(),
    dialogProps: { open: false },
  }),
}));
vi.mock("@features/support/components/dialogs/PiiWarningDialog", () => ({ default: () => null }));
vi.mock("@features/support/components/case-creation-layout/header/CaseCreationHeader", () => ({
  CaseCreationHeader: () => <div />,
}));
vi.mock(
  "@features/support/components/case-creation-layout/form-sections/basic-information-section/BasicInformationSection",
  () => ({
    BasicInformationSection: (props: {
      setDeployment: (v: string) => void;
      setProduct: (v: string) => void;
    }) => (
      <button
        type="button"
        onClick={() => {
          props.setDeployment("Production");
          props.setProduct("prod-1");
        }}
      >
        Pick Product
      </button>
    ),
  }),
);
// The details stub stands in for the editor: it hands over the description and
// registers an image the way the editor does when one is pasted.
vi.mock(
  "@features/support/components/case-creation-layout/form-sections/case-details-section/CaseDetailsSection",
  () => ({
    CaseDetailsSection: (props: {
      setTitle: (v: string) => void;
      setDescription: (v: string) => void;
      setIssueType: (v: string) => void;
      setSeverity: (v: string) => void;
      onImageUpload?: (file: File, previewUrl: string) => Promise<void>;
      onAttachmentClick?: () => void;
    }) => (
      <div>
        <button
        type="button"
          onClick={() => {
            props.setTitle("A title");
            props.setIssueType("Query");
            props.setSeverity("4");
          }}
        >
          Fill Details
        </button>
        <button
        type="button"
          onClick={() => {
            void props.onImageUpload?.(
              new File(["x"], "shot.png", { type: "image/png" }),
              "blob:http://localhost/shot-1",
            );
            props.setDescription('<p>Steps</p><img src="blob:http://localhost/shot-1" alt="shot">');
          }}
        >
          Add Image Description
        </button>
        <button type="button" onClick={() => props.setDescription("<p>No images here</p>")}>
          Add Plain Description
        </button>
        <button type="button" onClick={props.onAttachmentClick}>Open Attachments</button>
      </div>
    ),
  }),
);
vi.mock(
  "@features/support/components/case-creation-layout/form-sections/watch-list-section/WatchListSection",
  () => ({ WatchListSection: () => <div /> }),
);
vi.mock(
  "@features/support/components/case-creation-layout/form-sections/conversation-summary-section/ConversationSummary",
  () => ({ ConversationSummary: () => <div /> }),
);
vi.mock(
  "@features/support/components/case-creation-layout/form-sections/conversation-summary-section/RelatedCaseSummary",
  () => ({ RelatedCaseSummary: () => <div /> }),
);
vi.mock(
  "@features/support/components/case-details/attachments-tab/UploadAttachmentModal",
  () => ({
    default: ({ onSelect }: { onSelect: (f: File) => void }) => (
      <button type="button" onClick={() => onSelect(new File(["y"], "log.txt", { type: "text/plain" }))}>
        Select File
      </button>
    ),
  }),
);
vi.mock("@features/project-details/components/deployments/AddProductModal", () => ({
  default: () => null,
}));
vi.mock("@features/project-details/components/deployments/AddDeploymentWizardModal", () => ({
  default: () => null,
}));

async function fillForm(withImage: boolean) {
  render(<CreateCasePage />);
  fireEvent.click(await screen.findByText("Pick Product"));
  fireEvent.click(screen.getByText("Fill Details"));
  fireEvent.click(screen.getByText(withImage ? "Add Image Description" : "Add Plain Description"));
}

// Waits for the button to be back to its idle label (it reads "Uploading
// images..." while a submit is in flight) before clicking it.
const submit = async () =>
  fireEvent.click(await screen.findByRole("button", { name: /create support case/i }));

describe("CreateCasePage inline images", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    calls.length = 0;
    mocks.uploadStoredAttachment.mockImplementation(async (_f, input) => {
      calls.push(`image:${input.projectId}`);
      return { id: "img-1" };
    });
    mocks.mutate.mockImplementation((payload, options) => {
      calls.push("create");
      options?.onSuccess?.({ id: "case-1", ...payload });
    });
    mocks.storedUpload.mockImplementation(async () => {
      calls.push("attachment");
      return {};
    });
  });

  it("uploads the description's images to the project, then creates the case with /<id> references, then the attachments", async () => {
    await fillForm(true);
    fireEvent.click(screen.getByText("Open Attachments"));
    fireEvent.click(screen.getByText("Select File"));
    await submit();

    await waitFor(() => expect(mocks.storedUpload).toHaveBeenCalled());
    expect(calls).toEqual(["image:project-1", "create", "attachment"]);
    const payload = mocks.mutate.mock.calls[0][0];
    expect(payload.description).toContain('src="/img-1"');
    expect(payload.description).not.toContain("blob:");
    expect(mocks.uploadStoredAttachment.mock.calls[0][1]).toMatchObject({
      type: "comment",
      projectId: "project-1",
    });
  });

  it("does not create the case when an image fails, shows the error, and does not re-upload on retry", async () => {
    mocks.uploadStoredAttachment.mockRejectedValueOnce(new Error("Upload failed."));
    await fillForm(true);
    await submit();

    await waitFor(() =>
      expect(mocks.showError).toHaveBeenCalledWith(
        "Upload failed. The case was not created; try again.",
      ),
    );
    expect(mocks.mutate).not.toHaveBeenCalled();
    expect(await screen.findByRole("button", { name: /create support case/i })).toBeEnabled();

    await submit();
    await waitFor(() => expect(mocks.mutate).toHaveBeenCalledTimes(1));
    expect(mocks.uploadStoredAttachment).toHaveBeenCalledTimes(2);
    expect(mocks.mutate.mock.calls[0][0].description).toContain('src="/img-1"');
  });

  it("keeps an image that uploaded when a retry follows a failed create", async () => {
    mocks.mutate.mockImplementationOnce((_payload, options) => {
      calls.push("create");
      options?.onError?.(new Error("Server said no."));
    });
    await fillForm(true);
    await submit();
    await waitFor(() => expect(mocks.showError).toHaveBeenCalledWith("Server said no."));

    await submit();
    await waitFor(() => expect(mocks.mutate).toHaveBeenCalledTimes(2));
    expect(mocks.uploadStoredAttachment).toHaveBeenCalledTimes(1);
    expect(mocks.mutate.mock.calls[1][0].description).toContain('src="/img-1"');
  });

  it("sends the description untouched and makes no image request when there is no image", async () => {
    await fillForm(false);
    await submit();

    await waitFor(() => expect(mocks.mutate).toHaveBeenCalledTimes(1));
    expect(mocks.uploadStoredAttachment).not.toHaveBeenCalled();
    expect(mocks.mutate.mock.calls[0][0].description).toBe("<p>No images here</p>");
  });
});
