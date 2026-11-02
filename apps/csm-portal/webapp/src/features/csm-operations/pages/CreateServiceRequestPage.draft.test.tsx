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
import { MemoryRouter, Route, Routes, useNavigate } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { EncodedAttachment } from "@components/attachments/encodeAttachment";
import { NewServiceRequestDraftProvider } from "@context/new-service-request-draft/NewServiceRequestDraftContext";

const navigateMock = vi.fn();
const postCaseMutateAsyncMock = vi.fn();

vi.mock("@hooks/useNavTransition", () => ({ useNavTransition: () => navigateMock }));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));
vi.mock("@hooks/useEngineerDisplayName", () => ({ useEngineerDisplayName: () => "Test Engineer" }));
vi.mock("@features/csm-cases/api/usePostCsmCase", () => ({
  usePostCsmCase: () => ({ mutateAsync: postCaseMutateAsyncMock }),
}));
vi.mock("@features/csm-cases/api/useCsmCaseAttachments", () => ({
  usePostCsmCaseAttachment: () => ({ mutateAsync: vi.fn(), uploadProgress: null }),
  MAX_ATTACHMENT_SIZE_BYTES: 10 * 1024 * 1024,
}));
vi.mock("@features/csm-cases/api/uploadAttachmentsToCase", () => ({
  uploadAttachmentsToCase: () => Promise.resolve(0),
}));
vi.mock("@features/csm-projects/api/useGetProject", () => ({
  useGetProject: () => ({
    data: { id: "proj-a", hasSr: true, subscriptionType: "managed_cloud_subscription" },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));
vi.mock("@features/csm-projects/api/useProjectMetadata", () => ({
  useProjectMetadata: () => ({ data: undefined, isLoading: false, isError: false }),
}));
vi.mock("@features/csm-cases/api/useSearchDeployments", () => ({
  useSearchDeployments: () => ({
    data: [{ id: "dep-1", name: "Production" }],
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));
vi.mock("@features/csm-cases/api/useDeployedProductOptions", () => ({
  useDeployedProductOptions: () => ({
    data: [{ id: "dp-1", label: "Product One", category: null }],
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));
vi.mock("@features/csm-operations/api/useSearchCatalogs", () => ({
  useSearchCatalogs: () => ({
    data: [
      {
        id: "cat-1",
        name: "Support Catalog",
        catalogItems: [{ id: "item-1", name: "Restart service" }],
      },
    ],
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));
vi.mock("@features/csm-operations/api/useCatalogItemVariables", () => ({
  useCatalogItemVariables: () => ({
    data: [
      { id: "var-details", questionText: "Request Details", type: "String", order: 1 },
      { id: "var-description", questionText: "Description", type: "String", order: 2 },
    ],
    isLoading: false,
    isError: false,
    isSuccess: true,
  }),
}));
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({ value, onChange }: { value: string; onChange: (v: string) => void }) => (
    <textarea
      aria-label="Description body"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
}));
vi.mock("@components/attachments/AttachmentsField", () => ({
  default: ({
    attachments,
    onChange,
  }: {
    attachments: EncodedAttachment[];
    onChange: (next: EncodedAttachment[]) => void;
  }) => (
    <div>
      <span data-testid="attachment-names">{attachments.map((a) => a.name).join(",")}</span>
      <button
        onClick={() =>
          onChange([
            ...attachments,
            { name: "notes.txt", file: "eA==", size: 1, raw: new File(["x"], "notes.txt") },
          ])
        }
      >
        Add file
      </button>
    </div>
  ),
}));
vi.mock("@features/csm-cases/components/ProjectSelectionField", () => ({
  default: ({ value, onChange }: { value: string; onChange: (id: string) => void }) => (
    <div>
      <span data-testid="project-value">{value}</span>
      <button onClick={() => onChange("proj-a")}>Pick project A</button>
      <button onClick={() => onChange("proj-b")}>Pick project B</button>
    </div>
  ),
}));
// The page imports BackendApiError from the real client, which reads
// window.config at module load; a real class keeps `instanceof` working.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

// Imported after the mocks above so the module picks them up.
import CreateServiceRequestPage from "@features/csm-operations/pages/CreateServiceRequestPage";

const FORM_PATH = "/operations/service-requests/new";

/** What a case's "Create service request" action hands the form through router state. */
const FROM_CASE_STATE = {
  projectId: "proj-a",
  relatedCaseId: "case-9",
  relatedCaseNumber: "CS0009",
  deploymentId: "dep-1",
  deployedProductId: "dp-1",
};

/** Stands in for the in-app tab strip: routes away from the form and back. */
function TabSwitcher(): React.JSX.Element {
  const navigate = useNavigate();
  return (
    <div>
      <button onClick={() => navigate("/cases/case-1")}>Case tab</button>
      <button onClick={() => navigate(FORM_PATH)}>Service requests tab</button>
      <button onClick={() => navigate(`${FORM_PATH}?projectId=proj-z`)}>Project page</button>
      <button onClick={() => navigate(FORM_PATH, { state: FROM_CASE_STATE })}>
        Create from case
      </button>
    </div>
  );
}

function renderApp(initialEntry: string | { pathname: string; state: unknown } = FORM_PATH): void {
  render(
    <NewServiceRequestDraftProvider>
      <MemoryRouter initialEntries={[initialEntry]}>
        <TabSwitcher />
        <Routes>
          <Route path="/operations/service-requests/new" element={<CreateServiceRequestPage />} />
          <Route path="/cases/:caseId" element={<div>Case detail</div>} />
        </Routes>
      </MemoryRouter>
    </NewServiceRequestDraftProvider>,
  );
}

function pick(label: RegExp, option: string): void {
  fireEvent.mouseDown(screen.getByLabelText(label));
  fireEvent.click(screen.getByRole("option", { name: option }));
}

const DEPLOYMENT = /deployment/i;
const PRODUCT = /deployed product/i;
// The negative lookahead tells the "Catalog" field apart from "Catalog item".
const CATALOG = /^catalog(?!\s*item)/i;
const CATALOG_ITEM = /catalog item/i;

/** Walks the pickers down to the catalog item, which is what reveals the answer and attachment fields. */
function selectThroughCatalogItem(): void {
  fireEvent.click(screen.getByText("Pick project A"));
  pick(DEPLOYMENT, "Production");
  pick(PRODUCT, "Product One");
  pick(CATALOG, "Support Catalog");
  pick(CATALOG_ITEM, "Restart service");
}

function fillForm(): void {
  selectThroughCatalogItem();
  fireEvent.change(screen.getByLabelText(/Request Details/), {
    target: { value: "Please restart the service" },
  });
  fireEvent.change(screen.getByLabelText("Description body"), {
    target: { value: "<p>Needs a restart</p>" },
  });
  fireEvent.click(screen.getByText("Add file"));
}

function switchAwayAndBack(): void {
  fireEvent.click(screen.getByText("Case tab"));
  expect(screen.getByText("Case detail")).toBeInTheDocument();
  fireEvent.click(screen.getByText("Service requests tab"));
}

beforeEach(() => {
  vi.clearAllMocks();
  postCaseMutateAsyncMock.mockResolvedValue({ id: "case-1" });
});

describe("CreateServiceRequestPage draft retention across in-app tab switches", () => {
  it("restores every entered field after visiting a case route and coming back", () => {
    renderApp();
    fillForm();

    switchAwayAndBack();

    expect(screen.getByTestId("project-value")).toHaveTextContent("proj-a");
    expect(screen.getByLabelText(DEPLOYMENT)).toHaveTextContent("Production");
    expect(screen.getByLabelText(PRODUCT)).toHaveTextContent("Product One");
    expect(screen.getByLabelText(CATALOG)).toHaveTextContent("Support Catalog");
    expect(screen.getByLabelText(CATALOG_ITEM)).toHaveTextContent("Restart service");
    expect(screen.getByLabelText(/Request Details/)).toHaveValue("Please restart the service");
    expect(screen.getByLabelText("Description body")).toHaveValue("<p>Needs a restart</p>");
    expect(screen.getByTestId("attachment-names")).toHaveTextContent("notes.txt");
    // Everything restored is enough to submit again without re-entering anything.
    expect(screen.getByRole("button", { name: "Create service request" })).toBeEnabled();
  });

  it("keeps the cascade reset working on a restored draft", () => {
    renderApp();
    fillForm();
    switchAwayAndBack();

    fireEvent.click(screen.getByText("Pick project B"));

    expect(screen.getByLabelText(DEPLOYMENT)).not.toHaveTextContent("Production");
    expect(screen.getByLabelText(PRODUCT)).not.toHaveTextContent("Product One");
    expect(screen.getByLabelText(CATALOG)).not.toHaveTextContent("Support Catalog");
    // The answers belong to the catalog item that was just cleared.
    expect(screen.queryByLabelText(/Request Details/)).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Description body")).not.toBeInTheDocument();
  });

  it("starts fresh when reopened for a different locked project", () => {
    renderApp();
    fillForm();
    fireEvent.click(screen.getByText("Case tab"));
    // Re-enter with a locked project: a different entry than the draft's.
    fireEvent.click(screen.getByText("Project page"));

    expect(screen.getByTestId("project-value")).toHaveTextContent("proj-z");
    expect(screen.getByLabelText(DEPLOYMENT)).not.toHaveTextContent("Production");
    expect(screen.queryByLabelText(/Request Details/)).not.toBeInTheDocument();
    expect(screen.queryByText("notes.txt")).not.toBeInTheDocument();
  });

  it("discards the draft when Cancel is clicked", () => {
    renderApp();
    fillForm();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(navigateMock).toHaveBeenCalledWith("/operations");

    switchAwayAndBack();

    expect(screen.getByTestId("project-value")).toBeEmptyDOMElement();
    expect(screen.queryByLabelText(/Request Details/)).not.toBeInTheDocument();
    // Nothing of the discarded draft is waiting behind the fresh form either.
    selectThroughCatalogItem();
    expect(screen.getByLabelText(/Request Details/)).toHaveValue("");
    expect(screen.getByLabelText("Description body")).toHaveValue("");
    expect(screen.getByTestId("attachment-names")).toBeEmptyDOMElement();
  });

  it("clears the draft after a successful submit", async () => {
    renderApp();
    fillForm();
    fireEvent.click(screen.getByRole("button", { name: "Create service request" }));
    await waitFor(() => expect(navigateMock).toHaveBeenCalled());
    expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(1);
    expect(postCaseMutateAsyncMock).toHaveBeenCalledWith(
      expect.objectContaining({
        type: "service_request",
        projectId: "proj-a",
        deploymentId: "dep-1",
        deployedProductId: "dp-1",
        catalogId: "cat-1",
        catalogItemId: "item-1",
      }),
    );

    switchAwayAndBack();

    expect(screen.getByTestId("project-value")).toBeEmptyDOMElement();
    expect(screen.queryByLabelText(/Request Details/)).not.toBeInTheDocument();
    selectThroughCatalogItem();
    expect(screen.getByLabelText(/Request Details/)).toHaveValue("");
    expect(screen.getByLabelText("Description body")).toHaveValue("");
    expect(screen.getByTestId("attachment-names")).toBeEmptyDOMElement();
  });

  it("keeps the draft when the create fails, so it can be retried", async () => {
    postCaseMutateAsyncMock.mockRejectedValueOnce(new Error("boom"));
    renderApp();
    fillForm();
    fireEvent.click(screen.getByRole("button", { name: "Create service request" }));
    await waitFor(() => expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Create service request" })).toBeEnabled(),
    );
    expect(navigateMock).not.toHaveBeenCalled();

    switchAwayAndBack();

    expect(screen.getByTestId("project-value")).toHaveTextContent("proj-a");
    expect(screen.getByLabelText(/Request Details/)).toHaveValue("Please restart the service");
    expect(screen.getByTestId("attachment-names")).toHaveTextContent("notes.txt");
  });

  it("does not offer a second copy of a create that is still in flight", async () => {
    let finishCreate: (created: { id: string }) => void = () => undefined;
    postCaseMutateAsyncMock.mockReturnValueOnce(
      new Promise((resolve) => {
        finishCreate = resolve;
      }),
    );
    renderApp();
    fillForm();
    fireEvent.click(screen.getByRole("button", { name: "Create service request" }));
    await waitFor(() => expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(1));

    switchAwayAndBack();

    // Reopened mid-create: an empty form, so the same request cannot be sent twice.
    expect(screen.getByTestId("project-value")).toBeEmptyDOMElement();
    expect(screen.queryByLabelText(/Request Details/)).not.toBeInTheDocument();

    finishCreate({ id: "case-1" });
    await waitFor(() => expect(navigateMock).toHaveBeenCalled());
    expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("project-value")).toBeEmptyDOMElement();
  });

  it("keeps a newer draft when the create that was in flight finishes later", async () => {
    let finishCreate: (created: { id: string }) => void = () => undefined;
    postCaseMutateAsyncMock.mockReturnValueOnce(
      new Promise((resolve) => {
        finishCreate = resolve;
      }),
    );
    renderApp();
    fillForm();
    fireEvent.click(screen.getByRole("button", { name: "Create service request" }));
    await waitFor(() => expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(1));

    // Reopened mid-create, the engineer starts a new request...
    switchAwayAndBack();
    fireEvent.click(screen.getByText("Pick project B"));

    // ...and the first one is then created: that must not discard the new draft.
    finishCreate({ id: "case-1" });
    await waitFor(() => expect(navigateMock).toHaveBeenCalled());
    switchAwayAndBack();

    expect(screen.getByTestId("project-value")).toHaveTextContent("proj-b");
  });

  it("keeps a draft under the entry it was started from when the route is reached with another entry", () => {
    renderApp();
    fillForm();

    // Same route, different entry (a locked project): the form stays mounted
    // with what was entered, which belongs to the entry it was started from.
    fireEvent.click(screen.getByText("Project page"));
    switchAwayAndBack();

    expect(screen.getByTestId("project-value")).toHaveTextContent("proj-a");
    expect(screen.getByLabelText(/Request Details/)).toHaveValue("Please restart the service");
  });

  it("does not hand a draft to a different entry of the same route", () => {
    renderApp();
    fillForm();
    fireEvent.click(screen.getByText("Project page"));
    fireEvent.click(screen.getByText("Case tab"));
    fireEvent.click(screen.getByText("Project page"));

    expect(screen.getByTestId("project-value")).toHaveTextContent("proj-z");
    expect(screen.queryByLabelText(/Request Details/)).not.toBeInTheDocument();
  });

  it("drops the draft when a sign-out begins", () => {
    renderApp();
    fillForm();

    window.dispatchEvent(new CustomEvent("app:signing-out"));
    switchAwayAndBack();

    expect(screen.getByTestId("project-value")).toBeEmptyDOMElement();
    expect(screen.queryByLabelText(/Request Details/)).not.toBeInTheDocument();
  });
});

describe("CreateServiceRequestPage opened from a case", () => {
  it("prefills from the originating case and still files the request linked to it", async () => {
    renderApp({ pathname: FORM_PATH, state: FROM_CASE_STATE });

    expect(screen.getByTestId("project-value")).toHaveTextContent("proj-a");
    expect(screen.getByLabelText(DEPLOYMENT)).toHaveTextContent("Production");
    expect(screen.getByLabelText(PRODUCT)).toHaveTextContent("Product One");
    expect(screen.getByText(/linked to case CS0009/)).toBeInTheDocument();

    pick(CATALOG, "Support Catalog");
    pick(CATALOG_ITEM, "Restart service");
    fireEvent.change(screen.getByLabelText(/Request Details/), {
      target: { value: "Linked request" },
    });
    fireEvent.change(screen.getByLabelText("Description body"), {
      target: { value: "<p>Linked</p>" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create service request" }));

    await waitFor(() => expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(1));
    expect(postCaseMutateAsyncMock).toHaveBeenCalledWith(
      expect.objectContaining({
        projectId: "proj-a",
        deploymentId: "dep-1",
        deployedProductId: "dp-1",
        relatedCaseId: "case-9",
      }),
    );
  });

  it("restores a draft started from a case when the form is opened from that case again", () => {
    renderApp({ pathname: FORM_PATH, state: FROM_CASE_STATE });
    pick(CATALOG, "Support Catalog");
    pick(CATALOG_ITEM, "Restart service");
    fireEvent.change(screen.getByLabelText(/Request Details/), {
      target: { value: "Linked request" },
    });

    fireEvent.click(screen.getByText("Case tab"));
    fireEvent.click(screen.getByText("Create from case"));

    expect(screen.getByLabelText(CATALOG)).toHaveTextContent("Support Catalog");
    expect(screen.getByLabelText(/Request Details/)).toHaveValue("Linked request");
  });
});
