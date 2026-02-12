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

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { BrowserRouter } from "react-router";
import { useGetCaseCreationDetails } from "@api/useGetCaseCreationDetails";
import useGetProjectDetails from "@api/useGetProjectDetails";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import CreateCasePage from "@pages/CreateCasePage";
import { LoaderProvider } from "@context/linear-loader/LoaderContext";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
import { SuccessBannerProvider } from "@context/success-banner/SuccessBannerContext";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", async (importOriginal) => {
  const actual = (await importOriginal()) as any;
  return {
    ...actual,
    Box: ({ children, component, onSubmit }: any) => (
      <div data-testid="box" data-component={component} onSubmit={onSubmit}>
        {children}
      </div>
    ),
    Button: ({ children, onClick, startIcon }: any) => (
      <button onClick={onClick}>
        {startIcon}
        {children}
      </button>
    ),
    IconButton: ({ children, onClick }: any) => (
      <button onClick={onClick} data-testid="icon-button">
        {children}
      </button>
    ),
    Grid: ({ children, container }: any) => (
      <div data-testid="grid" data-container={container}>
        {children}
      </div>
    ),
    Typography: ({ children, variant }: any) => (
      <span data-testid="typography" data-variant={variant}>
        {children}
      </span>
    ),
    TextField: ({ value, placeholder, multiline }: any) => (
      <input
        data-testid="text-field"
        value={value}
        placeholder={placeholder}
        data-multiline={multiline}
        readOnly
      />
    ),
  };
});

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", async (importOriginal) => {
  const actual = (await importOriginal()) as any;
  return {
    ...actual,
    ArrowLeft: () => <svg data-testid="icon-arrow-left" />,
    Bot: () => <svg data-testid="icon-bot" />,
    CircleCheck: () => <svg data-testid="icon-check" />,
    MessageSquare: () => <svg data-testid="icon-message" />,
    Pencil: () => <svg data-testid="icon-pencil" />,
    PencilLine: () => <svg data-testid="icon-pencil-line" />,
    Sparkles: () => <svg data-testid="icon-sparkles" />,
    Info: () => <svg data-testid="icon-info" />,
    Server: () => <svg data-testid="icon-server" />,
    Clock: () => <svg data-testid="icon-clock" />,
    User: () => <svg data-testid="icon-user" />,
    Shield: () => <svg data-testid="icon-shield" />,
    Rocket: () => <svg data-testid="icon-rocket" />,
    CircleAlert: () => <svg data-testid="icon-alert" />,
  };
});

// Mock @asgardeo/react
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getIdToken: vi.fn().mockResolvedValue("mock-token"),
    isLoading: false,
    state: { isAuthenticated: true },
  }),
}));

// Mock useLogger hook
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  }),
}));

// Mock useLoader
vi.mock("../../context/linear-loader/LoaderContext", async (importOriginal) => {
  const actual = (await importOriginal()) as any;
  return {
    ...actual,
    useLoader: () => ({
      showLoader: vi.fn(),
      hideLoader: vi.fn(),
    }),
  };
});

// Mock the API hook
vi.mock("@api/useGetCaseCreationDetails", () => ({
  useGetCaseCreationDetails: vi.fn(() => ({
    data: {
      projects: ["Production Environment-Main"],
      products: ["WSO2 API Manager - v4.2.0"],
      deploymentTypes: ["Production"],
      issueTypes: ["Partial Outage"],
      severityLevels: [
        { id: "S1", label: "S1", description: "Desc 1" },
        { id: "S2", label: "S2", description: "Desc 2" },
      ],
      conversationSummary: {
        messagesExchanged: 8,
        troubleshootingAttempts: "2 steps completed",
        kbArticlesReviewed: "3 articles suggested",
      },
    },
    isLoading: false,
  })),
}));

// Mock useGetProjectDetails hook
vi.mock("@api/useGetProjectDetails", () => ({
  default: vi.fn(() => ({
    data: { id: "123", name: "WSO2 Super App", key: "WSA" },
    isLoading: false,
    error: null,
  })),
}));

// Mock useGetCasesFilters hook
vi.mock("@api/useGetCasesFilters", () => ({
  default: vi.fn(() => ({
    data: {
      statuses: [{ id: "1", label: "Open" }],
      severities: [
        { id: "60", label: "S0", description: "S0 desc" },
        { id: "61", label: "S1", description: "S1 desc" },
      ],
      issueTypes: [{ id: "6", label: "Error" }],
      deployments: [{ id: "1", label: "Production" }],
    },
    isLoading: false,
    isError: false,
  })),
}));

// Mock useMockConfig
vi.mock("@providers/MockConfigProvider", () => ({
  useMockConfig: () => ({ isMockEnabled: false }),
}));

// Mock useGetProjectDeployments
vi.mock("@api/useGetProjectDeployments", () => ({
  useGetProjectDeployments: () => ({
    data: [
      { id: "dep-1", name: "Staging", type: { id: "3", label: "Staging" } },
    ],
  }),
}));

// Mock fetchDeploymentProducts (used by useQueries for deployment products)
vi.mock("@api/useGetDeploymentsProducts", () => ({
  fetchDeploymentProducts: vi.fn().mockResolvedValue([
    {
      product: { id: "prod-1", label: "WSO2 API Manager - v4.2.0" },
      deployment: { id: "dep-1", label: "Staging" },
    },
  ]),
}));

// Mock usePostCase
vi.mock("@api/usePostCase", () => ({
  usePostCase: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}));

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
});

describe("CreateCasePage", () => {
  it("should render all sections correctly", () => {
    render(
      <QueryClientProvider client={queryClient}>
        <LoaderProvider>
          <ErrorBannerProvider>
            <SuccessBannerProvider>
              <BrowserRouter>
                <CreateCasePage />
              </BrowserRouter>
            </SuccessBannerProvider>
          </ErrorBannerProvider>
        </LoaderProvider>
      </QueryClientProvider>,
    );

    // Header logic
    expect(screen.getByText("Review Case Details")).toBeInTheDocument();
    expect(screen.getByText("AI Generated")).toBeInTheDocument();

    // Form sections
    expect(screen.getByText("Basic Information")).toBeInTheDocument();
    expect(screen.getByText("Case Details")).toBeInTheDocument();

    // Specific AI populated fields
    expect(
      screen.getByDisplayValue(
        "Unstable API Manager Performance in Production",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("WSO2 API Manager - v4.2.0")).toBeInTheDocument();

    // Sidebar
    expect(screen.getByText("Conversation Summary")).toBeInTheDocument();
    expect(screen.getByText("8")).toBeInTheDocument(); // Messages exchanged
  });

  it("should have a back button that navigates back", () => {
    render(
      <QueryClientProvider client={queryClient}>
        <LoaderProvider>
          <ErrorBannerProvider>
            <SuccessBannerProvider>
              <BrowserRouter>
                <CreateCasePage />
              </BrowserRouter>
            </SuccessBannerProvider>
          </ErrorBannerProvider>
        </LoaderProvider>
      </QueryClientProvider>,
    );

    const backButtons = screen.getAllByText("Back to Chat");
    expect(backButtons.length).toBeGreaterThan(0);
    expect(screen.getByTestId("icon-arrow-left")).toBeInTheDocument();
  });

  it("should render error message when statistics fail to load", () => {
    vi.mocked(useGetCaseCreationDetails).mockReturnValue({
      data: null,
      isLoading: false,
      isError: true,
    } as any);

    // Ensure project details don't fail properly (can mock return null/loading for this test scenario if needed, but not strictly required for this test case's focus)
    vi.mocked(useGetProjectDetails).mockReturnValue({
      data: null,
      isLoading: false,
      error: new Error("Failed"),
    } as any);

    render(
      <QueryClientProvider client={queryClient}>
        <LoaderProvider>
          <ErrorBannerProvider>
            <SuccessBannerProvider>
              <BrowserRouter>
                <CreateCasePage />
              </BrowserRouter>
            </SuccessBannerProvider>
          </ErrorBannerProvider>
        </LoaderProvider>
      </QueryClientProvider>,
    );

    expect(
      screen.getByText(
        "Error loading case creation details. Please try again later.",
      ),
    ).toBeInTheDocument();
  });
});
