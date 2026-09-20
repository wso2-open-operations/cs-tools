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
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { BeProjectMetadata } from "@api/backend/types";

const navigateMock = vi.fn();
const postCaseMutateAsyncMock = vi.fn();
const showErrorMock = vi.fn();

// Mutable per-test fixture for the project-metadata query, mirroring the
// getter pattern used for mutation `isPending` flags elsewhere (e.g.
// CreateChangeRequestPage.test.tsx) so each `it` can set its own shape
// without a fresh vi.mock per test.
let projectMetadataResult: {
  data: BeProjectMetadata | null | undefined;
  isLoading: boolean;
  isError: boolean;
} = { data: undefined, isLoading: false, isError: false };

vi.mock("react-router", () => ({
  useNavigate: () => navigateMock,
  useLocation: () => ({ state: undefined }),
  useSearchParams: () => [new URLSearchParams()],
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
vi.mock("@features/csm-cases/api/usePostCsmCase", () => ({
  usePostCsmCase: () => ({ mutateAsync: postCaseMutateAsyncMock }),
}));
vi.mock("@features/csm-cases/api/useCsmCaseAttachments", () => ({
  usePostCsmCaseAttachment: () => ({ mutateAsync: vi.fn(), uploadProgress: null }),
  // AttachmentsField (rendered once a catalog item is selected, per the
  // read-access gating tests below driving selection that far) reads this
  // constant directly, not through the hook.
  MAX_ATTACHMENT_SIZE_BYTES: 10 * 1024 * 1024,
}));
vi.mock("@hooks/useEngineerDisplayName", () => ({
  useEngineerDisplayName: () => "Test Engineer",
}));
vi.mock("@features/csm-projects/api/useGetProject", () => ({
  useGetProject: () => ({
    data: { id: "proj-1", hasSr: true, subscriptionType: "managed_cloud_subscription" },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));
vi.mock("@features/csm-projects/api/useProjectMetadata", () => ({
  useProjectMetadata: () => projectMetadataResult,
}));
vi.mock("@features/csm-cases/api/useSearchDeployments", () => ({
  useSearchDeployments: () => ({
    data: [{ id: "dep-1", name: "Production" }],
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));
// Three fixed deployed-product options spanning: a category matching the
// project's srProductCategories restriction, a category that doesn't, and no
// category at all — enough to exercise every branch of the client-side filter.
vi.mock("@features/csm-cases/api/useDeployedProductOptions", () => ({
  useDeployedProductOptions: () => ({
    data: [
      { id: "dp-ms", label: "API Manager 4.3.0", category: "ms" },
      { id: "dp-pc", label: "Choreo Connect 1.0", category: "pc" },
      { id: "dp-none", label: "Unknown Product", category: null },
    ],
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));
// A single catalog with a single catalog item, so the read-access gating
// tests below can drive selection all the way to a submittable state and
// prove canSubmit's own gate, not just an earlier field being empty.
vi.mock("@features/csm-operations/api/useSearchCatalogs", () => ({
  useSearchCatalogs: () => ({
    data: [{ id: "cat-1", name: "Support Catalog", catalogItems: [{ id: "item-1", name: "Restart service" }] }],
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));
vi.mock("@features/csm-operations/api/useCatalogItemVariables", () => ({
  useCatalogItemVariables: () => ({ data: [], isLoading: false, isError: false }),
}));
vi.mock("@features/csm-cases/components/ProjectSelectionField", () => ({
  default: ({
    value,
    onChange,
  }: {
    value: string;
    onChange: (next: string) => void;
  }) => (
    <input aria-label="Project" value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));
// CreateServiceRequestPage imports BackendApiError from the real API client
// module, which reads window.config at module load and throws outside a
// configured runtime. Mock it with a real class (so `instanceof` still
// works), mirroring CreateSecurityReportPage.test.tsx.
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

/** Selects the project and deployment, which is enough to unlock (enable) the
 * deployed-product dropdown these tests exercise. */
function selectProjectAndDeployment(): void {
  fireEvent.change(screen.getByLabelText("Project"), { target: { value: "proj-1" } });
  fireEvent.mouseDown(screen.getByLabelText(/deployment/i));
  fireEvent.click(screen.getByRole("option", { name: "Production" }));
}

/** Drives every field canSubmit requires (project, deployment, deployed
 * product, catalog, catalog item) so the read-access gating tests exercise
 * canSubmit's own hasNoSrReadAccess check — not a disabled button that's
 * actually disabled because an earlier, unrelated field is still empty. */
function selectFullFlow(): void {
  selectProjectAndDeployment();
  fireEvent.mouseDown(screen.getByLabelText(/deployed product/i));
  fireEvent.click(screen.getByRole("option", { name: "API Manager 4.3.0" }));
  // Negative lookahead distinguishes the "Catalog" field's accessible name
  // (which MUI renders with a trailing required-asterisk, e.g. "Catalog *")
  // from the separate "Catalog item" field, which a plain /^catalog/i would
  // also match.
  fireEvent.mouseDown(screen.getByLabelText(/^catalog(?!\s*item)/i));
  fireEvent.click(screen.getByRole("option", { name: "Support Catalog" }));
  fireEvent.mouseDown(screen.getByLabelText(/catalog item/i));
  fireEvent.click(screen.getByRole("option", { name: "Restart service" }));
}

describe("CreateServiceRequestPage — deployed-product filtering by srProductCategories", () => {
  beforeEach(() => {
    navigateMock.mockReset();
    postCaseMutateAsyncMock.mockReset();
    showErrorMock.mockReset();
    projectMetadataResult = { data: undefined, isLoading: false, isError: false };
  });

  it("shows only deployed products whose category is in the project's srProductCategories", () => {
    projectMetadataResult = {
      data: { features: { srProductCategories: ["ms"] } },
      isLoading: false,
      isError: false,
    };
    render(<CreateServiceRequestPage />);
    selectProjectAndDeployment();

    fireEvent.mouseDown(screen.getByLabelText(/deployed product/i));
    expect(screen.getByRole("option", { name: "API Manager 4.3.0" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Choreo Connect 1.0" })).not.toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Unknown Product" })).not.toBeInTheDocument();
  });

  it("shows every deployed product when srProductCategories is absent (no restriction configured)", () => {
    projectMetadataResult = {
      data: { features: {} },
      isLoading: false,
      isError: false,
    };
    render(<CreateServiceRequestPage />);
    selectProjectAndDeployment();

    fireEvent.mouseDown(screen.getByLabelText(/deployed product/i));
    expect(screen.getByRole("option", { name: "API Manager 4.3.0" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Choreo Connect 1.0" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Unknown Product" })).toBeInTheDocument();
  });

  it("shows every deployed product when srProductCategories is an empty array", () => {
    projectMetadataResult = {
      data: { features: { srProductCategories: [] } },
      isLoading: false,
      isError: false,
    };
    render(<CreateServiceRequestPage />);
    selectProjectAndDeployment();

    fireEvent.mouseDown(screen.getByLabelText(/deployed product/i));
    expect(screen.getByRole("option", { name: "API Manager 4.3.0" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Choreo Connect 1.0" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Unknown Product" })).toBeInTheDocument();
  });

  it("fails open (shows every deployed product) when the metadata fetch itself errors", () => {
    projectMetadataResult = { data: undefined, isLoading: false, isError: true };
    render(<CreateServiceRequestPage />);
    selectProjectAndDeployment();

    fireEvent.mouseDown(screen.getByLabelText(/deployed product/i));
    expect(screen.getByRole("option", { name: "API Manager 4.3.0" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Choreo Connect 1.0" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Unknown Product" })).toBeInTheDocument();
    // A metadata-fetch failure must never surface as a form-blocking error —
    // it's a narrowing-only feature, so it fails open silently.
    expect(screen.queryByText(/failed to load deployed products/i)).not.toBeInTheDocument();
  });
});

describe("CreateServiceRequestPage — gating on hasServiceRequestReadAccess", () => {
  beforeEach(() => {
    navigateMock.mockReset();
    postCaseMutateAsyncMock.mockReset();
    showErrorMock.mockReset();
    projectMetadataResult = { data: undefined, isLoading: false, isError: false };
  });

  it("blocks submission and shows the ineligibility error once metadata resolves hasServiceRequestReadAccess: false", () => {
    projectMetadataResult = {
      data: { features: { hasServiceRequestReadAccess: false } },
      isLoading: false,
      isError: false,
    };
    render(<CreateServiceRequestPage />);
    // Every other canSubmit requirement is satisfied here, so a disabled
    // button below can only be explained by hasNoSrReadAccess itself.
    selectFullFlow();

    expect(
      screen.getByText(/isn't eligible to raise service requests/i),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /create service request/i })).toBeDisabled();
  });

  it("allows submission when metadata resolves hasServiceRequestReadAccess: true", () => {
    projectMetadataResult = {
      data: { features: { hasServiceRequestReadAccess: true } },
      isLoading: false,
      isError: false,
    };
    render(<CreateServiceRequestPage />);
    selectFullFlow();

    expect(
      screen.queryByText(/isn't eligible to raise service requests/i),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /create service request/i })).toBeEnabled();
  });

  it("fails open (no error, not blocked on this check alone) while metadata is still undefined", () => {
    projectMetadataResult = { data: undefined, isLoading: false, isError: false };
    render(<CreateServiceRequestPage />);
    selectFullFlow();

    expect(
      screen.queryByText(/isn't eligible to raise service requests/i),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /create service request/i })).toBeEnabled();
  });
});
