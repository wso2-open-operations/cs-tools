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
import { BrowserRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import { useGetCaseCreationDetails } from "@/api/useGetCaseCreationDetails";
import CreateCasePage from "@/pages/CreateCasePage";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
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
  Card: ({ children }: any) => <div data-testid="card">{children}</div>,
  CardContent: ({ children }: any) => (
    <div data-testid="card-content">{children}</div>
  ),
  Chip: ({ label, icon }: any) => (
    <div data-testid="chip">
      {icon}
      {label}
    </div>
  ),
  colors: {
    orange: { 700: "#c2410c" },
  },
  ComplexSelect: Object.assign(
    ({ children, value }: any) => <select value={value}>{children}</select>,
    {
      MenuItem: Object.assign(
        ({ value }: any) => <option value={value}>{value}</option>,
        {
          Icon: () => null,
          Text: ({ primary }: any) => primary,
        },
      ),
    },
  ),
  Divider: () => <hr />,
  Form: {
    ElementWrapper: ({ children, label }: any) => (
      <div data-testid="form-element-wrapper">
        <label>{label}</label>
        {children}
      </div>
    ),
  },
  FormControl: ({ children }: any) => (
    <div data-testid="form-control">{children}</div>
  ),
  Grid: ({ children, container }: any) => (
    <div data-testid="grid" data-container={container}>
      {children}
    </div>
  ),
  InputLabel: ({ children }: any) => <label>{children}</label>,
  MenuItem: ({ children, value }: any) => (
    <option value={value}>{children}</option>
  ),
  Paper: ({ children }: any) => <div data-testid="paper">{children}</div>,
  Select: ({ children, value }: any) => (
    <select value={value} disabled>
      {children}
    </select>
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
  Typography: ({ children, variant }: any) => (
    <span data-testid="typography" data-variant={variant}>
      {children}
    </span>
  ),
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  ArrowLeft: () => <svg data-testid="icon-arrow-left" />,
  Bot: () => <svg data-testid="icon-bot" />,
  CircleCheck: () => <svg data-testid="icon-check" />,
  MessageSquare: () => <svg data-testid="icon-message" />,
  Pencil: () => <svg data-testid="icon-pencil" />,
  Sparkles: () => <svg data-testid="icon-sparkles" />,
}));

// Mock useLogger hook
vi.mock("@/hooks/useLogger", () => ({
  useLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  }),
}));

// Mock the API hook
vi.mock("@/api/useGetCaseCreationDetails", () => ({
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

describe("CreateCasePage", () => {
  it("should render all sections correctly", () => {
    render(
      <BrowserRouter>
        <CreateCasePage />
      </BrowserRouter>,
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
      <BrowserRouter>
        <CreateCasePage />
      </BrowserRouter>,
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

    render(
      <BrowserRouter>
        <CreateCasePage />
      </BrowserRouter>,
    );

    expect(
      screen.getByText(
        "Error loading case creation details. Please try again later.",
      ),
    ).toBeInTheDocument();
  });
});
