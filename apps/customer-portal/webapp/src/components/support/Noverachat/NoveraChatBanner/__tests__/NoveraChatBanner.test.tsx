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
import { describe, expect, it, vi } from "vitest";
import NoveraChatBanner from "../NoveraChatBanner";

const mockNavigate = vi.fn();

// Mock react-router
vi.mock("react-router", () => ({
  useNavigate: () => mockNavigate,
}));

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  Button: ({ children, onClick }: any) => (
    <button data-testid="button" onClick={onClick}>
      {children}
    </button>
  ),
  Paper: ({ children }: any) => <div data-testid="paper">{children}</div>,
  Typography: ({ children }: any) => (
    <span data-testid="typography">{children}</span>
  ),
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Bot: () => <svg data-testid="icon-bot" />,
}));

describe("NoveraChatBanner", () => {
  it("should render correctly with title and description", () => {
    render(<NoveraChatBanner />);

    expect(
      screen.getByText("Need help with something new?"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Chat with Novera to get instant assistance/i),
    ).toBeInTheDocument();
    expect(screen.getByTestId("icon-bot")).toBeInTheDocument();
  });

  it("should navigate to chat page when clicking 'Start New Chat'", () => {
    render(<NoveraChatBanner />);

    const button = screen.getByRole("button", { name: /Start New Chat/i });
    fireEvent.click(button);

    expect(mockNavigate).toHaveBeenCalledWith("chat");
  });
});
