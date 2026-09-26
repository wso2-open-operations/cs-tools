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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
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

const mockProjectFilters = vi.fn(() => ({
  data: {
    issueTypes: [] as { id: string; label: string }[],
    severities: [] as { id: string; label: string }[],
  },
  isLoading: false,
}));
vi.mock("@api/useGetProjectFilters", () => ({
  default: () => mockProjectFilters(),
}));

const mockDeploymentsQuery = vi.fn();
vi.mock("@api/usePostProjectDeploymentsSearch", () => ({
  usePostProjectDeploymentsSearchInfinite: () => mockDeploymentsQuery(),
}));

const mockDeploymentProductsQuery = vi.fn();
vi.mock("@features/project-details/api/usePostDeploymentProductsSearch", () => ({
  usePostDeploymentProductsSearchInfinite: () => mockDeploymentProductsQuery(),
  extractDeploymentProducts: (page: { products?: unknown[] }) =>
    page?.products ?? [],
}));

vi.mock("@features/operations/api/usePostCase", () => ({
  usePostCase: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock("@/hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => vi.fn(),
}));

// Stable references: the CreateCasePage effects key off of these query
// results by identity, so a mock that returns a fresh array/object literal
// on every render would re-trigger those effects forever.
const EMPTY_PROJECT_CONTACTS: unknown[] = [];
const MOCK_USER_DETAILS = { email: "jane.doe@example.com" };

vi.mock("@features/settings/api/useGetProjectContacts", () => ({
  default: () => ({ data: EMPTY_PROJECT_CONTACTS, isLoading: false }),
}));

vi.mock("@features/settings/api/useGetUserDetails", () => ({
  default: () => ({ data: MOCK_USER_DETAILS }),
}));

vi.mock("@features/support/api/usePostAttachments", () => ({
  usePostAttachments: () => ({ mutateAsync: vi.fn(), mutate: vi.fn() }),
}));

vi.mock("@tanstack/react-query", async (importOriginal) => {
  const actual = (await importOriginal()) as object;
  return {
    ...actual,
    useQueryClient: () => ({ invalidateQueries: vi.fn() }),
  };
});

const mockShouldRestrictToPrimaryProductionDeployments = vi.fn(() => false);
vi.mock("@utils/permission", () => ({
  filterDeploymentsForCaseCreation: (items: unknown[]) => items ?? [],
  getProjectSeverityPolicy: () => ({
    excludeS0: false,
    restrictSeverityToLow: false,
  }),
  shouldRestrictToPrimaryProductionDeployments: () =>
    mockShouldRestrictToPrimaryProductionDeployments(),
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
    BasicInformationSection: (props: {
      deployment?: string;
      product?: string;
      setDeployment?: (value: string) => void;
      onAddDeployment?: () => void;
      onAddProduct?: () => void;
      isDeploymentAutoDetected?: boolean;
      isProductAutoDetected?: boolean;
      isDeploymentLoading?: boolean;
      isProductLoading?: boolean;
    }) => (
      <div>
        Basic Section
        <div>Deployment value: {props.deployment || "(none)"}</div>
        <div>Product value: {props.product || "(none)"}</div>
        <div>
          Deployment auto detected: {String(!!props.isDeploymentAutoDetected)}
        </div>
        <div>Product auto detected: {String(!!props.isProductAutoDetected)}</div>
        <div>Deployment loading: {String(!!props.isDeploymentLoading)}</div>
        <div>Product loading: {String(!!props.isProductLoading)}</div>
        {props.setDeployment && (
          <button onClick={() => props.setDeployment?.("Production")}>
            Select Deployment
          </button>
        )}
        {props.onAddDeployment && (
          <button onClick={props.onAddDeployment}>Add Deployment CTA</button>
        )}
        {props.onAddProduct && (
          <button onClick={props.onAddProduct}>Add Product CTA</button>
        )}
      </div>
    ),
  }),
);

vi.mock(
  "@features/support/components/case-creation-layout/form-sections/case-details-section/CaseDetailsSection",
  () => ({
    CaseDetailsSection: () => <div>Details Section</div>,
  }),
);

vi.mock(
  "@features/support/components/case-creation-layout/form-sections/watch-list-section/WatchListSection",
  () => ({
    WatchListSection: () => <div>Watch List Section</div>,
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

vi.mock(
  "@features/project-details/components/deployments/AddProductModal",
  () => ({
    default: ({
      open,
      onClose,
      onSuccess,
    }: {
      open: boolean;
      onClose: () => void;
      onSuccess?: () => void;
    }) =>
      open ? (
        <div>
          Add Product Modal
          <button
            onClick={() => {
              onClose();
              onSuccess?.();
            }}
          >
            Submit Product Modal
          </button>
          <button onClick={onClose}>Close Product Modal</button>
        </div>
      ) : null,
  }),
);

vi.mock(
  "@features/project-details/components/deployments/AddDeploymentWizardModal",
  () => ({
    default: ({
      open,
      onClose,
      onDeploymentCreated,
      onProductAdded,
    }: {
      open: boolean;
      onClose: () => void;
      onDeploymentCreated?: (name: string) => void;
      onProductAdded?: () => void;
    }) =>
      open ? (
        <div>
          Add Deployment Wizard
          <button onClick={() => onDeploymentCreated?.("Production")}>
            Submit Wizard Deployment Step
          </button>
          <button onClick={() => onDeploymentCreated?.("Staging")}>
            Submit Wizard Deployment Step (Staging)
          </button>
          <button onClick={() => onProductAdded?.()}>
            Submit Wizard Product
          </button>
          <button onClick={onClose}>Wizard Done</button>
        </div>
      ) : null,
  }),
);

vi.mock("@features/support/hooks/usePiiGuard", () => ({
  usePiiGuard: () => ({
    checkBeforeSubmit: (_text: string, onProceed: () => void) => onProceed(),
    dialogProps: { open: false },
  }),
}));

vi.mock("@features/support/components/dialogs/PiiWarningDialog", () => ({
  default: () => null,
}));

const EMPTY_INFINITE_QUERY = {
  data: { pages: [] as unknown[] },
  isLoading: false,
  isError: false,
  isFetchingNextPage: false,
  hasNextPage: false,
  fetchNextPage: vi.fn(),
};

describe("CreateCasePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseParams.mockReturnValue({ projectId: "project-1" });
    mockUseLocation.mockReturnValue({
      pathname: "/projects/project-1/support/chat/create-case",
      search: "",
      state: null,
    });
    mockDeploymentsQuery.mockReturnValue({
      ...EMPTY_INFINITE_QUERY,
      data: { pages: [{ deployments: [{ id: "dep-1", name: "Production" }] }] },
    });
    mockDeploymentProductsQuery.mockReturnValue({
      ...EMPTY_INFINITE_QUERY,
      data: {
        pages: [
          {
            products: [
              { id: "prod-1", product: { label: "API Manager" }, version: "4.2.0" },
            ],
          },
        ],
      },
    });
  });

  it("should render default create support case composition", () => {
    render(<CreateCasePage />);

    expect(
      screen.getByRole("heading", { name: "Create Support Case" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Basic Section")).toBeInTheDocument();
    expect(screen.getByText("Details Section")).toBeInTheDocument();
    expect(screen.getByText("Watch List Section")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /create support case/i }),
    ).toBeInTheDocument();
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

  it("should hide case details, watch list, and submit when the project has no deployments", () => {
    mockDeploymentsQuery.mockReturnValue(EMPTY_INFINITE_QUERY);

    render(<CreateCasePage />);

    expect(screen.getByText("Basic Section")).toBeInTheDocument();
    expect(screen.queryByText("Details Section")).not.toBeInTheDocument();
    expect(screen.queryByText("Watch List Section")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /create support case/i }),
    ).not.toBeInTheDocument();
  });

  it("should open the wizard (not a plain modal) for the persistent 'add another' affordance too, since every new deployment starts with zero products", () => {
    // beforeEach already mocks a non-empty deployment list ("Production").
    render(<CreateCasePage />);
    expect(screen.queryByText("Add Deployment Wizard")).not.toBeInTheDocument();

    fireEvent.click(screen.getByText("Add Deployment CTA"));

    expect(screen.getByText("Add Deployment Wizard")).toBeInTheDocument();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("switches the selected deployment to the newly added one, not the previously auto-selected one", () => {
    // beforeEach already mocks a single deployment ("Production"), which
    // the general singleton effect would have auto-selected.
    render(<CreateCasePage />);
    expect(screen.getByText("Deployment value: Production")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Add Deployment CTA"));
    fireEvent.click(screen.getByText("Submit Wizard Deployment Step (Staging)"));

    expect(screen.getByText("Deployment value: Staging")).toBeInTheDocument();
  });

  describe("Add Deployment wizard", () => {
    it("opens the wizard dialog when the deployment list is empty", () => {
      mockDeploymentsQuery.mockReturnValue(EMPTY_INFINITE_QUERY);

      render(<CreateCasePage />);
      expect(screen.queryByText("Add Deployment Wizard")).not.toBeInTheDocument();

      fireEvent.click(screen.getByText("Add Deployment CTA"));

      expect(screen.getByText("Add Deployment Wizard")).toBeInTheDocument();
      expect(mockNavigate).not.toHaveBeenCalled();
    });

    it("selects the newly created deployment in the case-creation form as soon as the wizard's deployment step succeeds, using the mutation response directly rather than waiting on a query refetch", () => {
      mockDeploymentsQuery.mockReturnValue(EMPTY_INFINITE_QUERY);

      render(<CreateCasePage />);
      fireEvent.click(screen.getByText("Add Deployment CTA"));
      expect(screen.getByText("Add Deployment Wizard")).toBeInTheDocument();

      fireEvent.click(screen.getByText("Submit Wizard Deployment Step"));

      expect(screen.getByText("Deployment value: Production")).toBeInTheDocument();
      // The wizard owns its own step transition internally - it never closes
      // as a side effect of the deployment step succeeding.
      expect(screen.getByText("Add Deployment Wizard")).toBeInTheDocument();
    });

    it("supports adding multiple products in one wizard session without closing, and closes only on Done", () => {
      mockDeploymentsQuery.mockReturnValue(EMPTY_INFINITE_QUERY);

      render(<CreateCasePage />);
      fireEvent.click(screen.getByText("Add Deployment CTA"));
      fireEvent.click(screen.getByText("Submit Wizard Deployment Step"));

      fireEvent.click(screen.getByText("Submit Wizard Product"));
      expect(screen.getByText("Add Deployment Wizard")).toBeInTheDocument();

      fireEvent.click(screen.getByText("Submit Wizard Product"));
      expect(screen.getByText("Add Deployment Wizard")).toBeInTheDocument();

      fireEvent.click(screen.getByText("Wizard Done"));
      expect(
        screen.queryByText("Add Deployment Wizard"),
      ).not.toBeInTheDocument();
    });
  });

  it("should hide case details, watch list, and submit when the selected deployment has no products", () => {
    mockDeploymentProductsQuery.mockReturnValue(EMPTY_INFINITE_QUERY);

    render(<CreateCasePage />);
    fireEvent.click(screen.getByText("Select Deployment"));

    expect(screen.getByText("Basic Section")).toBeInTheDocument();
    expect(screen.queryByText("Details Section")).not.toBeInTheDocument();
    expect(screen.queryByText("Watch List Section")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /create support case/i }),
    ).not.toBeInTheDocument();
  });

  it("should open the Add Product modal inline instead of navigating away when the Add Product CTA is clicked", () => {
    mockDeploymentProductsQuery.mockReturnValue(EMPTY_INFINITE_QUERY);

    render(<CreateCasePage />);
    fireEvent.click(screen.getByText("Select Deployment"));
    expect(screen.queryByText("Add Product Modal")).not.toBeInTheDocument();

    fireEvent.click(screen.getByText("Add Product CTA"));

    expect(screen.getByText("Add Product Modal")).toBeInTheDocument();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("should close the Add Product modal on successful creation", () => {
    mockDeploymentProductsQuery.mockReturnValue(EMPTY_INFINITE_QUERY);

    render(<CreateCasePage />);
    fireEvent.click(screen.getByText("Select Deployment"));
    fireEvent.click(screen.getByText("Add Product CTA"));
    expect(screen.getByText("Add Product Modal")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Submit Product Modal"));

    expect(screen.queryByText("Add Product Modal")).not.toBeInTheDocument();
  });

  it("should show case details and submit when a deployment and its products are available", () => {
    render(<CreateCasePage />);
    fireEvent.click(screen.getByText("Select Deployment"));

    expect(screen.getByText("Details Section")).toBeInTheDocument();
    expect(screen.getByText("Watch List Section")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /create support case/i }),
    ).toBeInTheDocument();
  });

  describe("single-option auto-select", () => {
    // The plain (no-classification) create-case flow already auto-picks
    // `baseDeploymentOptions[0]` unconditionally via a pre-existing effect
    // (see the "!classificationResponse" branch of the init effect), so it
    // can't isolate the *new* singleton auto-select behavior on its own.
    // These deployment-focused tests instead drive the AI-classification
    // path (`classificationResponse` present in location.state), which is
    // exactly the gap the new effect fills: that pre-existing effect does
    // NOT run once a classificationResponse exists, so any resulting
    // deployment selection can only have come from the new effect.
    const CLASSIFICATION_STATE = {
      classificationResponse: {
        issueType: "Bug",
        severityLevel: "S2",
        caseInfo: {},
      },
    };

    it("auto-selects the deployment and reveals case details when there is exactly one deployment option, with no click needed", () => {
      mockUseLocation.mockReturnValue({
        pathname: "/projects/project-1/support/chat/create-case",
        search: "",
        state: CLASSIFICATION_STATE,
      });
      // beforeEach already mocks a single deployment ("Production") and a
      // single product ("API Manager") for the selected deployment.
      render(<CreateCasePage />);

      // No "Select Deployment" click: the single option should be picked
      // automatically, which is what unblocks Details/Watch List/Submit.
      expect(
        screen.getByText("Deployment value: Production"),
      ).toBeInTheDocument();
      expect(screen.getByText("Details Section")).toBeInTheDocument();
      expect(screen.getByText("Watch List Section")).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /create support case/i }),
      ).toBeInTheDocument();

      // Regression: a singleton auto-select is not the same thing as an
      // AI-classification "detection" - reusing the same "Auto detected"
      // chip for both was misleading, since nothing was actually detected
      // here, it was just the only option.
      expect(
        screen.getByText("Deployment auto detected: false"),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Product auto detected: false"),
      ).toBeInTheDocument();
    });

    it("does not mask an already-loaded product behind a loading skeleton just because classification's page search is still in flight", () => {
      // Regression: adding a product to a deployment while Novera's
      // classification suggested some (unrelated/not-yet-matched) product
      // name used to keep the product field stuck in a loading state -
      // masking the just-added product - until something else reset the
      // classification-pending flags (e.g. re-picking the deployment).
      // The classification-apply effect requires a non-empty severities
      // list to run at all.
      mockProjectFilters.mockReturnValue({
        data: {
          issueTypes: [],
          severities: [{ id: "sev-2", label: "S2" }],
        },
        isLoading: false,
      });
      mockUseLocation.mockReturnValue({
        pathname: "/projects/project-1/support/chat/create-case",
        search: "",
        state: {
          classificationResponse: {
            issueType: "Bug",
            severityLevel: "S2",
            caseInfo: { productName: "Some Other Product Not In The List" },
          },
        },
      });
      // The deployment's product query still has more pages left to search
      // for that classification-suggested product, but a real product has
      // already arrived on the first page - it must not be hidden.
      mockDeploymentProductsQuery.mockReturnValue({
        ...EMPTY_INFINITE_QUERY,
        hasNextPage: true,
        data: {
          pages: [
            {
              products: [
                { id: "prod-1", product: { label: "API Manager" }, version: "4.2.0" },
              ],
            },
          ],
        },
      });

      render(<CreateCasePage />);

      expect(screen.getByText("Product loading: false")).toBeInTheDocument();
    });

    it("also switches to a newly-added deployment (not the classification-driven one) when arriving via Novera chat", () => {
      // The Add Deployment wizard / override-on-create fix isn't gated by
      // noAiMode at all, so it should behave identically whether the
      // customer arrived via skipChat (Get Help / New Case) or via Novera
      // chat's classification path - confirming that here rather than just
      // asserting it by code inspection.
      mockUseLocation.mockReturnValue({
        pathname: "/projects/project-1/support/chat/create-case",
        search: "",
        state: CLASSIFICATION_STATE,
      });
      render(<CreateCasePage />);
      expect(
        screen.getByText("Deployment value: Production"),
      ).toBeInTheDocument();

      fireEvent.click(screen.getByText("Add Deployment CTA"));
      fireEvent.click(screen.getByText("Submit Wizard Deployment Step (Staging)"));

      expect(screen.getByText("Deployment value: Staging")).toBeInTheDocument();
    });

    it("auto-selects a pre-existing single deployment in skipChat mode too, not just right after adding one", async () => {
      // Regression: the skipChat init effect used to unconditionally blank
      // `deployment` on load, which meant a project's single PRE-EXISTING
      // deployment never got picked here even though the general singleton
      // effect handles every other flow - only the from-empty
      // add-then-chain path (a different effect) got fixed for skipChat
      // before. This is the plain "customer already has exactly one
      // deployment, opens create-case via Get Help / New Case (both set
      // skipChat: true)" case.
      mockUseLocation.mockReturnValue({
        pathname: "/projects/project-1/support/chat/create-case",
        search: "",
        state: { skipChat: true },
      });
      // beforeEach already mocks a single deployment ("Production").
      render(<CreateCasePage />);

      // The skipChat init effect defers its state updates to a queued
      // microtask, so the value lands one tick after render, not
      // synchronously.
      await waitFor(() =>
        expect(
          screen.getByText("Deployment value: Production"),
        ).toBeInTheDocument(),
      );
    });

    it("does not auto-select the deployment when there are multiple options", () => {
      mockUseLocation.mockReturnValue({
        pathname: "/projects/project-1/support/chat/create-case",
        search: "",
        state: CLASSIFICATION_STATE,
      });
      mockDeploymentsQuery.mockReturnValue({
        ...EMPTY_INFINITE_QUERY,
        data: {
          pages: [
            {
              deployments: [
                { id: "dep-1", name: "Production" },
                { id: "dep-2", name: "Staging" },
              ],
            },
          ],
        },
      });

      render(<CreateCasePage />);

      // No option should be picked on the customer's behalf when there is
      // more than one to choose from.
      expect(
        screen.getByText("Deployment value: (none)"),
      ).toBeInTheDocument();
    });

    it("auto-selects the product when the selected deployment has exactly one product option", () => {
      render(<CreateCasePage />);

      fireEvent.click(screen.getByText("Select Deployment"));

      // The single product should be auto-selected without a manual pick.
      expect(
        screen.getByText("Product value: prod-1"),
      ).toBeInTheDocument();
    });

    it("does not auto-select the product when there are multiple product options for the selected deployment", () => {
      mockDeploymentProductsQuery.mockReturnValue({
        ...EMPTY_INFINITE_QUERY,
        data: {
          pages: [
            {
              products: [
                {
                  id: "prod-1",
                  product: { label: "API Manager" },
                  version: "4.2.0",
                },
                {
                  id: "prod-2",
                  product: { label: "Identity Server" },
                  version: "6.0.0",
                },
              ],
            },
          ],
        },
      });

      render(<CreateCasePage />);
      fireEvent.click(screen.getByText("Select Deployment"));

      // Multiple product options still resolve the form (options exist), but
      // the product itself remains unpicked, matching prior behavior.
      expect(screen.getByText("Details Section")).toBeInTheDocument();
      expect(
        screen.getByText("Product value: (none)"),
      ).toBeInTheDocument();
    });

    it("re-evaluates the product auto-select for the new deployment's option list after the deployment changes", () => {
      render(<CreateCasePage />);

      // First deployment resolves to a single product ("API Manager") from
      // the default beforeEach mock; confirm it auto-selects.
      fireEvent.click(screen.getByText("Select Deployment"));
      expect(
        screen.getByText("Product value: prod-1"),
      ).toBeInTheDocument();

      // Now the deployment's product list is refreshed to multiple options
      // (simulating navigating to a different deployment whose products
      // just loaded). Re-selecting the deployment goes through
      // handleDeploymentChange, which resets `product` to "" so the
      // singleton effect re-evaluates against the *new* list rather than
      // keeping the stale auto-selected value.
      mockDeploymentProductsQuery.mockReturnValue({
        ...EMPTY_INFINITE_QUERY,
        data: {
          pages: [
            {
              products: [
                {
                  id: "prod-1",
                  product: { label: "API Manager" },
                  version: "4.2.0",
                },
                {
                  id: "prod-2",
                  product: { label: "Identity Server" },
                  version: "6.0.0",
                },
              ],
            },
          ],
        },
      });

      fireEvent.click(screen.getByText("Select Deployment"));

      expect(
        screen.getByText("Product value: (none)"),
      ).toBeInTheDocument();
    });

    it("undoes an auto-selection if the product list turns out to have more than one option after all (e.g. resolves incrementally right after adding two products)", () => {
      // Starts as a genuine singleton (matches beforeEach's default single
      // product), so it auto-selects first, same as any other case.
      const { rerender } = render(<CreateCasePage />);
      fireEvent.click(screen.getByText("Select Deployment"));
      expect(
        screen.getByText("Product value: prod-1"),
      ).toBeInTheDocument();

      // The list then resolves to a second product (e.g. the wizard's
      // second "Add another product" catching up). The auto-picked value
      // must be reverted so the user gets a real choice, not left stuck on
      // whichever one happened to load first.
      mockDeploymentProductsQuery.mockReturnValue({
        ...EMPTY_INFINITE_QUERY,
        data: {
          pages: [
            {
              products: [
                { id: "prod-1", product: { label: "API Manager" }, version: "4.2.0" },
                { id: "prod-2", product: { label: "Identity Server" }, version: "6.0.0" },
              ],
            },
          ],
        },
      });
      rerender(<CreateCasePage />);

      expect(
        screen.getByText("Product value: (none)"),
      ).toBeInTheDocument();
    });
  });

  describe("CUSTOMER_PORTAL_ALLOW_DEPLOYMENT_SETUP_DURING_CASE_CREATION kill switch", () => {
    afterEach(() => {
      window.config = {
        ...window.config,
        CUSTOMER_PORTAL_ALLOW_DEPLOYMENT_SETUP_DURING_CASE_CREATION: undefined,
      };
    });

    it("falls back to the legacy disabled dropdown, no alert/CTA, and never blocks the form, when explicitly disabled", () => {
      window.config = {
        ...window.config,
        CUSTOMER_PORTAL_ALLOW_DEPLOYMENT_SETUP_DURING_CASE_CREATION: false,
      };
      mockDeploymentsQuery.mockReturnValue(EMPTY_INFINITE_QUERY);

      render(<CreateCasePage />);

      expect(screen.queryByText("Add Deployment CTA")).not.toBeInTheDocument();
      expect(
        screen.queryByText("Add Deployment Wizard"),
      ).not.toBeInTheDocument();
      // Legacy behavior: the form is never blocked, even with zero
      // deployments, when the feature is switched off.
      expect(screen.getByText("Details Section")).toBeInTheDocument();
      expect(screen.getByText("Watch List Section")).toBeInTheDocument();
    });

    it("does not auto-select a single deployment when disabled", async () => {
      window.config = {
        ...window.config,
        CUSTOMER_PORTAL_ALLOW_DEPLOYMENT_SETUP_DURING_CASE_CREATION: false,
      };
      mockUseLocation.mockReturnValue({
        pathname: "/projects/project-1/support/chat/create-case",
        search: "",
        state: { skipChat: true },
      });
      // beforeEach already mocks a single deployment ("Production").
      render(<CreateCasePage />);

      await waitFor(() =>
        expect(
          screen.getByText("Deployment value: (none)"),
        ).toBeInTheDocument(),
      );
    });
  });

  describe("Cloud Support / Cloud Evaluation Support projects (locked to Primary Production)", () => {
    afterEach(() => {
      // mockReturnValue persists across tests (clearAllMocks doesn't reset
      // it), so restore the default explicitly.
      mockShouldRestrictToPrimaryProductionDeployments.mockReturnValue(false);
    });

    it("does not offer Add Product for a fixed-deployment project, since it doesn't offer Add Deployment either", () => {
      mockShouldRestrictToPrimaryProductionDeployments.mockReturnValue(true);
      // A project locked to Primary Production has no deployment field at
      // all (hideDeploymentField), so there is never an "Add Deployment
      // CTA" to click here - only confirming "Add Product CTA" is also
      // absent.
      render(<CreateCasePage />);

      expect(
        screen.queryByText("Add Product CTA"),
      ).not.toBeInTheDocument();
    });

    it("does not block the form on an empty product list either, since there's no way to add one", () => {
      mockShouldRestrictToPrimaryProductionDeployments.mockReturnValue(true);
      mockDeploymentProductsQuery.mockReturnValue(EMPTY_INFINITE_QUERY);

      render(<CreateCasePage />);

      expect(screen.getByText("Details Section")).toBeInTheDocument();
      expect(screen.getByText("Watch List Section")).toBeInTheDocument();
    });
  });
});

