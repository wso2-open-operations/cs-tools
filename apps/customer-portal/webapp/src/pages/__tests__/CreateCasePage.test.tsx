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

import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
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

// Mock case-creation-layout components (paths may not exist in repo)
// Enhanced mocks to verify props
vi.mock(
  "@components/support/case-creation-layout/form-sections/basic-information-section/BasicInformationSection",
  () => ({
    BasicInformationSection: ({ product, deployment }: any) => (
      <div data-testid="basic-info-section">
        <span data-testid="product-val">{product}</span>
        <span data-testid="deployment-val">{deployment}</span>
      </div>
    ),
  }),
);

vi.mock(
  "@components/support/case-creation-layout/form-sections/case-details-section/CaseDetailsSection",
  () => ({
    CaseDetailsSection: ({ title, description, issueType, severity }: any) => (
      <div data-testid="case-details-section">
        <span data-testid="title-val">{title}</span>
        <span data-testid="desc-val">{description}</span>
        <span data-testid="issue-val">{issueType}</span>
        <span data-testid="severity-val">{severity}</span>
      </div>
    ),
  }),
);

vi.mock(
  "@components/support/case-creation-layout/form-sections/conversation-summary-section/ConversationSummary",
  () => ({ ConversationSummary: () => null }),
);

vi.mock(
  "@components/support/case-creation-layout/header/CaseCreationHeader",
  () => ({ CaseCreationHeader: () => null }),
);

import { MemoryRouter, Routes, Route } from "react-router";

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
});

describe("CreateCasePage", () => {
  it("should render all sections correctly", async () => {
    render(
      <QueryClientProvider client={queryClient}>
        <LoaderProvider>
          <ErrorBannerProvider>
            <SuccessBannerProvider>
              <MemoryRouter>
                <CreateCasePage />
              </MemoryRouter>
            </SuccessBannerProvider>
          </ErrorBannerProvider>
        </LoaderProvider>
      </QueryClientProvider>,
    );

    // Mock useGetProjectDetails hook
    vi.mock("../../api/useGetProjectDetails", () => ({
      default: vi.fn(() => ({
        data: { id: "123", name: "WSO2 Super App", key: "WSA" },
        isLoading: false,
        error: null,
      })),
    }));

    // Mock useGetProjectFilters hook
    vi.mock("../../api/useGetProjectFilters", () => ({
      default: vi.fn(() => ({
        data: {
          statuses: [{ id: "1", label: "Open" }],
          severities: [
            { id: "60", label: "S0", description: "S0 desc" },
            { id: "61", label: "S1", description: "S1 desc" },
          ],
          issueTypes: [{ id: "6", label: "Error" }],
          deploymentTypes: [{ id: "1", label: "Production" }],
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
    vi.mock("../../api/useGetProjectDeployments", () => ({
      useGetProjectDeployments: () => ({
        data: [
          { id: "dep-1", name: "Staging", type: { id: "3", label: "Staging" } },
        ],
      }),
    }));

    // Mock useGetDeploymentsProducts (products for selected deployment)
    vi.mock("../../api/useGetDeploymentsProducts", () => ({
      useGetDeploymentsProducts: vi.fn(() => ({
        data: [
          {
            id: "dp-1",
            product: { id: "prod-1", label: "WSO2 API Manager - v4.2.0" },
            deployment: { id: "dep-1", label: "Staging" },
          },
        ],
        isLoading: false,
        isError: false,
      })),
      fetchDeploymentProducts: vi.fn().mockResolvedValue([]),
    }));

    // Mock usePostCase
    vi.mock("../../api/usePostCase", () => ({
      usePostCase: () => ({
        mutate: vi.fn(),
        isPending: false,
      }),
    }));

    // Mock usePostAttachments
    vi.mock("../../api/usePostAttachments", () => ({
      usePostAttachments: () => ({
        mutateAsync: vi.fn(),
        mutate: vi.fn(),
        isPending: false,
      }),
    }));

    // Page renders with create case form and submit button
    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /Create Support Case/i }),
      ).toBeInTheDocument();
    });
  });

  it("should populate form with classification data when available", async () => {
    const classificationState = {
      classificationResponse: {
        issueType: "Error",
        severityLevel: "S4",
        caseInfo: {
          description:
            "I am experiencing an issue with the WSO2 API Manager 2.0.0 Key Manager in the Staging environment. I need assistance in resolving this matter as soon as possible.",
          shortDescription:
            "Issue with WSO2 API Manager 2.0.0 Key Manager in Staging environment.",
          productName: "WSO2 API Manager",
          productVersion: "2.0.0",
          environment: "Staging",
          tier: "Tier 1",
          region: "EU",
        },
      },
    };

    render(
      <QueryClientProvider client={queryClient}>
        <LoaderProvider>
          <ErrorBannerProvider>
            <SuccessBannerProvider>
              <MemoryRouter
                initialEntries={[
                  {
                    pathname: "/123/support/create",
                    state: classificationState,
                  },
                ]}
              >
                <Routes>
                  <Route
                    path="/:projectId/support/create"
                    element={<CreateCasePage />}
                  />
                </Routes>
              </MemoryRouter>
            </SuccessBannerProvider>
          </ErrorBannerProvider>
        </LoaderProvider>
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("title-val")).toHaveTextContent(
        "Issue with WSO2 API Manager 2.0.0 Key Manager in Staging environment.",
      );
      expect(screen.getByTestId("desc-val")).toHaveTextContent(
        "I am experiencing an issue with the WSO2 API Manager 2.0.0 Key Manager in the Staging environment. I need assistance in resolving this matter as soon as possible.",
      );
      expect(screen.queryByText("Support case")).not.toBeInTheDocument();
      expect(
        screen.queryByText("Please describe your issue here."),
      ).not.toBeInTheDocument();
    });
  });
});
