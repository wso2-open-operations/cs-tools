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
import { describe, expect, it, vi, beforeEach } from "vitest";
import { MemoryRouter, Routes, Route } from "react-router";
import NoveraChatPage from "@pages/NoveraChatPage";

// Mock mockFunctions
vi.mock("@/models/mockFunctions", () => ({
  getNoveraResponse: () => "I am a mock response.",
}));

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div>{children}</div>,
  Button: ({ children, onClick, startIcon }: any) => (
    <button onClick={onClick}>
      {startIcon}
      {children}
    </button>
  ),
  CircularProgress: () => <div data-testid="circular-progress" />,
  Paper: ({ children }: any) => <div>{children}</div>,
  Typography: ({ children }: any) => <div>{children}</div>,
  TextField: ({ value, onChange, placeholder, onKeyPress }: any) => (
    <input
      value={value}
      onChange={onChange}
      placeholder={placeholder}
      onKeyPress={onKeyPress}
    />
  ),
  IconButton: ({ children, onClick, disabled }: any) => (
    <button onClick={onClick} disabled={disabled}>
      {children}
    </button>
  ),
  Divider: () => <hr />,
  colors: {
    orange: {
      700: "#C2410C",
    },
  },
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Bot: () => <svg data-testid="bot-icon" />,
  ArrowLeft: () => <svg data-testid="back-icon" />,
  Send: () => <svg data-testid="send-icon" />,
  CircleAlert: () => <svg data-testid="alert-icon" />,
}));

vi.mock("@api/usePostCaseClassifications", () => ({
  usePostCaseClassifications: () => ({
    mutateAsync: vi.fn().mockResolvedValue({}),
    isPending: false,
  }),
}));

vi.mock("@api/useGetDeployments", () => ({
  useGetDeployments: () => ({
    data: { deployments: [] },
  }),
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => ({
    data: { subscription: { supportTier: "" } },
    isLoading: false,
    error: null,
  }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({
    showError: vi.fn(),
  }),
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  }),
}));

const renderWithRouter = () => {
  return render(
    <MemoryRouter initialEntries={["/project-1/support/chat"]}>
      <Routes>
        <Route path="/:projectId/support/chat" element={<NoveraChatPage />} />
        <Route path="/:projectId/support" element={<div>Support Page</div>} />
        <Route
          path="/:projectId/dashboard"
          element={<div>Dashboard Page</div>}
        />
      </Routes>
    </MemoryRouter>,
  );
};

describe("NoveraChatPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Mock scrollIntoView as it's not implemented in JSDOM
    window.HTMLElement.prototype.scrollIntoView = vi.fn();
  });

  it("should render the initial state correctly", () => {
    renderWithRouter();

    expect(screen.getByText("Chat with Novera")).toBeInTheDocument();
    expect(
      screen.getByText(/Hi! I'm Novera, your AI support assistant/i),
    ).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText("Type your message..."),
    ).toBeInTheDocument();
  });

  it("should navigate back when clicking 'Back to Support'", () => {
    renderWithRouter();

    const backButton = screen.getByText("Back to Support");
    fireEvent.click(backButton);

    expect(screen.getByText("Support Page")).toBeInTheDocument();
  });

  it("should show escalation banner after sending several messages", async () => {
    renderWithRouter();

    const input = screen.getByPlaceholderText("Type your message...");
    const sendButton = screen.getByTestId("send-icon").parentElement!;

    // Send 1st message (Total: 2)
    fireEvent.change(input, { target: { value: "Message 1" } });
    fireEvent.click(sendButton);

    // Send 2nd message (Total: 3 - Bot haven't replied yet, but wait, the test doesn't wait for bot)
    fireEvent.change(input, { target: { value: "Message 2" } });
    fireEvent.click(sendButton);

    // Send 3rd message (Total: 4)
    fireEvent.change(input, { target: { value: "Message 3" } });
    fireEvent.click(sendButton);

    // Send 4th message (Total: 5)
    fireEvent.change(input, { target: { value: "Message 4" } });
    fireEvent.click(sendButton);

    // Now messages container has 5 items. Escalation banner should appear.
    await waitFor(() => {
      expect(screen.getByText(/Need more help\?/i)).toBeInTheDocument();
      expect(screen.getByText("Create Case")).toBeInTheDocument();
    });

    // Branding should still be visible as per latest manual changes
    expect(screen.getByText("Chat with Novera")).toBeInTheDocument();
  });

  it("should send a message and receive a bot response", async () => {
    renderWithRouter();

    const input = screen.getByPlaceholderText("Type your message...");
    const sendButton = screen.getByTestId("send-icon").parentElement!;

    fireEvent.change(input, { target: { value: "Hello Novera!" } });
    fireEvent.click(sendButton);

    expect(screen.getByText("Hello Novera!")).toBeInTheDocument();

    // Wait for the simulated bot response
    await waitFor(
      () => {
        expect(screen.getByText("I am a mock response.")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });
});
