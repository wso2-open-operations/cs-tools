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
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const navigateMock = vi.fn();
const postCaseMutateAsyncMock = vi.fn();
const showErrorMock = vi.fn();
const uploadImagesMock = vi.fn();
const releaseImagesMock = vi.fn();

vi.mock("react-router", () => ({
  useLocation: () => ({ state: undefined }),
  useSearchParams: () => [new URLSearchParams()],
}));
vi.mock("@hooks/useNavTransition", () => ({ useNavTransition: () => navigateMock }));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
vi.mock("@features/csm-cases/api/usePreCreateInlineImages", () => ({
  usePreCreateInlineImages: (projectId: string | undefined) => ({
    onImageUpload: projectId ? () => "blob:test" : undefined,
    uploadImages: uploadImagesMock,
    release: releaseImagesMock,
  }),
  preCreateImagesErrorMessage: (err: unknown) => (err instanceof Error ? err.message : "failed"),
}));
vi.mock("@features/csm-cases/api/usePostCsmCase", () => ({
  usePostCsmCase: () => ({ mutateAsync: postCaseMutateAsyncMock }),
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
    data: [{ id: "dp-1", label: "API Manager 4.3.0" }],
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));
vi.mock("@features/csm-cases/components/ProjectSelectionField", () => ({
  default: ({
    value,
    onChange,
  }: {
    value: string;
    onChange: (next: string) => void;
  }) => (
    <input
      aria-label="Project"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
}));
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({ value, onChange }: { value: string; onChange: (v: string) => void }) => (
    <textarea aria-label="editor" value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));
// CreateSecurityReportPage imports BackendApiError from the real API client
// module, which reads window.config at module load and throws outside a
// configured runtime. Mock it with a real class (so `instanceof` still
// works), mirroring CreateChangeRequestPage.test.tsx.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({ value, onChange }: { value: string; onChange: (v: string) => void }) => (
    <textarea aria-label="editor" value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));

import CsmEngagementCreatePage from "@features/csm-engagements/pages/CsmEngagementCreatePage";

function fillForm(): void {
  fireEvent.change(screen.getByLabelText("Project"), { target: { value: "proj-1" } });
  fireEvent.mouseDown(screen.getByLabelText(/deployment/i));
  fireEvent.click(screen.getByRole("option", { name: "Production" }));
  fireEvent.mouseDown(screen.getByLabelText(/deployed product/i));
  fireEvent.click(screen.getByRole("option", { name: "API Manager 4.3.0" }));
  fireEvent.mouseDown(screen.getByLabelText(/^engagement type/i));
  fireEvent.click(screen.getByRole("option", { name: "Consultancy" }));
  fireEvent.mouseDown(screen.getByLabelText(/engagement payment type/i));
  fireEvent.click(screen.getByRole("option", { name: "Paid" }));
  fireEvent.change(screen.getByLabelText(/subject/i), { target: { value: "Architecture review" } });
  fireEvent.change(screen.getByLabelText("editor"), { target: { value: "<p>Scope notes.</p>" } });
}

describe("CsmEngagementCreatePage description images", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    uploadImagesMock.mockImplementation((html: string) => Promise.resolve(html));
    postCaseMutateAsyncMock.mockResolvedValue({ id: "eng-1" });
  });

  it("uploads the description's images first, then creates the engagement with the rewritten HTML", async () => {
    const order: string[] = [];
    uploadImagesMock.mockImplementation(async () => {
      order.push("images");
      return '<p>Scope notes.</p><img src="/img-1">';
    });
    postCaseMutateAsyncMock.mockImplementation(async () => {
      order.push("create");
      return { id: "eng-1" };
    });
    render(<CsmEngagementCreatePage />);
    fillForm();

    fireEvent.click(screen.getByRole("button", { name: "Create engagement" }));

    await vi.waitFor(() => expect(navigateMock).toHaveBeenCalled());
    expect(order).toEqual(["images", "create"]);
    expect(postCaseMutateAsyncMock.mock.calls[0][0]).toMatchObject({
      type: "engagement",
      description: '<p>Scope notes.</p><img src="/img-1">',
    });
  });

  it("creates nothing and keeps the form when an image cannot be uploaded", async () => {
    uploadImagesMock.mockRejectedValueOnce(new Error("Image storage is busy."));
    render(<CsmEngagementCreatePage />);
    fillForm();

    fireEvent.click(screen.getByRole("button", { name: "Create engagement" }));

    await vi.waitFor(() => expect(showErrorMock).toHaveBeenCalledWith("Image storage is busy."));
    expect(postCaseMutateAsyncMock).not.toHaveBeenCalled();
    expect(navigateMock).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Create engagement" })).toBeEnabled();
  });
});
