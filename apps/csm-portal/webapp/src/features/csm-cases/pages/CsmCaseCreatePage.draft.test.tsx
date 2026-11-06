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
import { MemoryRouter, Route, Routes, useNavigate } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { NewCaseDraftProvider } from "@context/new-case-draft/NewCaseDraftContext";
import CsmCaseCreatePage from "./CsmCaseCreatePage";

const navigateMock = vi.fn();
const mutateAsyncMock = vi.fn();

vi.mock("@hooks/useNavTransition", () => ({ useNavTransition: () => navigateMock }));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
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
vi.mock("@features/csm-projects/api/useSearchProjectContacts", () => ({
  useSearchProjectContacts: () => ({
    data: [],
    isLoading: false,
    isSuccess: true,
    isError: false,
    isFetching: false,
    refetch: vi.fn(),
  }),
}));
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({ value, onChange }: { value: string; onChange: (v: string) => void }) => (
    <textarea aria-label="Description body" value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));
vi.mock("@components/attachments/AttachmentsField", () => ({ default: () => null }));
vi.mock("@features/csm-cases/components/CaseWatchListSection", () => ({ default: () => null }));
vi.mock("@features/csm-cases/components/ProjectSelectionField", () => ({
  default: ({ value, onChange }: { value: string; onChange: (id: string) => void }) => (
    <div>
      <span data-testid="project-value">{value}</span>
      <button onClick={() => onChange("proj-a")}>Pick project A</button>
      <button onClick={() => onChange("proj-b")}>Pick project B</button>
    </div>
  ),
}));

/** Stands in for the in-app case tab strip: routes away from the form and back. */
function TabSwitcher(): React.JSX.Element {
  const navigate = useNavigate();
  return (
    <div>
      <button onClick={() => navigate("/cases/case-1")}>Case tab</button>
      <button onClick={() => navigate("/cases/new")}>Support tab</button>
      <button onClick={() => navigate("/cases/new?projectId=proj-z")}>Project page</button>
    </div>
  );
}

function renderApp(initialPath = "/cases/new"): void {
  render(
    <NewCaseDraftProvider>
      <MemoryRouter initialEntries={[initialPath]}>
        <TabSwitcher />
        <Routes>
          <Route path="/cases/new" element={<CsmCaseCreatePage />} />
          <Route path="/cases/:caseId" element={<div>Case detail</div>} />
        </Routes>
      </MemoryRouter>
    </NewCaseDraftProvider>,
  );
}

function pick(label: string, option: RegExp): void {
  fireEvent.mouseDown(screen.getByRole("combobox", { name: new RegExp(label) }));
  fireEvent.click(within(screen.getByRole("listbox")).getByText(option));
}

function fillForm(): void {
  fireEvent.click(screen.getByText("Pick project A"));
  pick("Deployment", /Production/);
  pick("Deployed product", /Product One/);
  pick("Severity", /S3/);
  pick("Issue type", /Question/i);
  fireEvent.change(screen.getByLabelText(/Subject/), { target: { value: "Broken thing" } });
  fireEvent.change(screen.getByLabelText("Description body"), {
    target: { value: "<p>Help</p>" },
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  mutateAsyncMock.mockResolvedValue({ id: "case-1" });
});

describe("CsmCaseCreatePage draft retention across in-app tab switches", () => {
  it("restores every entered field after visiting a case route and coming back", () => {
    renderApp();
    fillForm();

    fireEvent.click(screen.getByText("Case tab"));
    expect(screen.getByText("Case detail")).toBeInTheDocument();
    fireEvent.click(screen.getByText("Support tab"));

    expect(screen.getByTestId("project-value")).toHaveTextContent("proj-a");
    expect(screen.getByLabelText(/Subject/)).toHaveValue("Broken thing");
    expect(screen.getByLabelText("Description body")).toHaveValue("<p>Help</p>");
    expect(screen.getByRole("combobox", { name: /Deployment/ })).toHaveTextContent("Production");
    expect(screen.getByRole("combobox", { name: /Deployed product/ })).toHaveTextContent(
      "Product One",
    );
    expect(screen.getByRole("combobox", { name: /Severity/ })).toHaveTextContent("S3");
    expect(screen.getByRole("combobox", { name: /Issue type/ })).toHaveTextContent(/Question/i);
  });

  it("keeps the project cascade reset working on a restored draft", () => {
    renderApp();
    fillForm();
    fireEvent.click(screen.getByText("Case tab"));
    fireEvent.click(screen.getByText("Support tab"));

    fireEvent.click(screen.getByText("Pick project B"));

    expect(screen.getByRole("combobox", { name: /Deployment/ })).not.toHaveTextContent("Production");
    expect(screen.getByRole("combobox", { name: /Deployed product/ })).not.toHaveTextContent(
      "Product One",
    );
    expect(screen.getByLabelText(/Subject/)).toHaveValue("Broken thing");
  });

  it("starts fresh when reopened for a different locked project", () => {
    renderApp();
    fillForm();
    fireEvent.click(screen.getByText("Case tab"));
    // Re-enter with a locked project: a different entry than the draft's.
    fireEvent.click(screen.getByText("Project page"));

    expect(screen.getByLabelText(/Subject/)).toHaveValue("");
    expect(screen.getByLabelText("Description body")).toHaveValue("");
  });

  it("discards the draft when Cancel is clicked", () => {
    renderApp();
    fillForm();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(navigateMock).toHaveBeenCalledWith("/cases");

    fireEvent.click(screen.getByText("Case tab"));
    fireEvent.click(screen.getByText("Support tab"));

    expect(screen.getByLabelText(/Subject/)).toHaveValue("");
    expect(screen.getByLabelText("Description body")).toHaveValue("");
  });

  it("clears the draft after a successful submit", async () => {
    renderApp();
    fillForm();
    fireEvent.click(screen.getByRole("button", { name: "Create case" }));
    await waitFor(() => expect(navigateMock).toHaveBeenCalled());
    expect(mutateAsyncMock).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByText("Case tab"));
    fireEvent.click(screen.getByText("Support tab"));

    expect(screen.getByLabelText(/Subject/)).toHaveValue("");
    expect(screen.getByLabelText("Description body")).toHaveValue("");
  });
});
