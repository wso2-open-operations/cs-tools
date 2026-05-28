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

import { render, screen, fireEvent } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import CreateCasePage from "@features/support/pages/CreateCasePage";

const mockNavigate = vi.fn();
const mockShowLoader = vi.fn();
const mockHideLoader = vi.fn();

const mockUseLocation = vi.fn();
const mockUseParams = vi.fn();

vi.mock("react-router", () => ({
  useNavigate: () => mockNavigate,
  useLocation: () => mockUseLocation(),
  useParams: () => mockUseParams(),
}));

vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: () => ({ showLoader: mockShowLoader, hideLoader: mockHideLoader }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ error: vi.fn() }),
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => ({ data: { name: "P1", type: { label: "Enterprise" } }, isLoading: false }),
}));

vi.mock("@api/useGetProjectFeatures", () => ({
  default: () => ({ data: {}, isLoading: false }),
}));

vi.mock("@api/useGetProjectFilters", () => ({
  default: () => ({ data: { issueTypes: [], severities: [] }, isLoading: false }),
}));

vi.mock("@api/usePostProjectDeploymentsSearch", () => ({
  usePostProjectDeploymentsSearchInfinite: () => ({
    data: { pages: [] },
    isLoading: false,
    isFetchingNextPage: false,
    hasNextPage: false,
    fetchNextPage: vi.fn(),
  }),
}));

vi.mock("@features/project-details/api/usePostDeploymentProductsSearch", () => ({
  usePostDeploymentProductsSearchInfinite: () => ({
    data: { pages: [] },
    isLoading: false,
    isError: false,
    isFetchingNextPage: false,
    hasNextPage: false,
    fetchNextPage: vi.fn(),
  }),
  extractDeploymentProducts: () => [],
}));

vi.mock("@features/operations/api/usePostCase", () => ({
  usePostCase: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock("@/hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => vi.fn(),
}));

vi.mock("@tanstack/react-query", async (importOriginal) => {
  const actual = (await importOriginal()) as object;
  return {
    ...actual,
    useQueryClient: () => ({ invalidateQueries: vi.fn() }),
  };
});

vi.mock("@utils/permission", () => ({
  filterDeploymentsForCaseCreation: (items: unknown[]) => items ?? [],
  getProjectSeverityPolicy: () => ({
    excludeS0: false,
    restrictSeverityToLow: false,
  }),
  shouldRestrictToPrimaryProductionDeployments: () => false,
}));

vi.mock("@features/support/components/case-creation-layout/header/CaseCreationHeader", () => ({
  CaseCreationHeader: ({
    title,
    subtitle,
    onBack,
  }: {
    title?: string;
    subtitle: string;
    onBack: () => void;
  }) => (
    <div>
      <h1>{title ?? "Create Support Case"}</h1>
      <p>{subtitle}</p>
      <button onClick={onBack}>Back Header</button>
    </div>
  ),
}));

vi.mock(
  "@features/support/components/case-creation-layout/form-sections/basic-information-section/BasicInformationSection",
  () => ({
    BasicInformationSection: () => <div>Basic Section</div>,
  }),
);

vi.mock(
  "@features/support/components/case-creation-layout/form-sections/case-details-section/CaseDetailsSection",
  () => ({
    CaseDetailsSection: () => <div>Details Section</div>,
  }),
);

vi.mock(
  "@features/support/components/case-creation-layout/form-sections/conversation-summary-section/ConversationSummary",
  () => ({
    ConversationSummary: () => <div>Conversation Summary</div>,
  }),
);

vi.mock(
  "@features/support/components/case-creation-layout/form-sections/conversation-summary-section/RelatedCaseSummary",
  () => ({
    RelatedCaseSummary: () => <div>Related Case Summary</div>,
  }),
);

vi.mock(
  "@features/support/components/case-details/attachments-tab/UploadAttachmentModal",
  () => ({
    default: () => null,
  }),
);

describe("CreateCasePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseParams.mockReturnValue({ projectId: "project-1" });
    mockUseLocation.mockReturnValue({
      pathname: "/projects/project-1/support/chat/create-case",
      search: "",
      state: null,
    });
  });

  it("should render default create support case composition", () => {
    render(<CreateCasePage />);

    expect(
      screen.getByRole("heading", { name: "Create Support Case" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Basic Section")).toBeInTheDocument();
    expect(screen.getByText("Details Section")).toBeInTheDocument();
  });

  it("should navigate using returnTo on back action", () => {
    mockUseLocation.mockReturnValue({
      pathname: "/projects/project-1/support/chat/create-case",
      search: "",
      state: { returnTo: "/projects/project-1/support/chat/conv-1" },
    });

    render(<CreateCasePage />);
    fireEvent.click(screen.getByText("Back Header"));

    expect(mockNavigate).toHaveBeenCalledWith("/projects/project-1/support/chat/conv-1");
  });
});

