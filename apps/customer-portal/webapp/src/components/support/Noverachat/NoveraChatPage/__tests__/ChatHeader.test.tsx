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
import ChatHeader from "@/components/support/Noverachat/NoveraChatPage/ChatHeader";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  Button: ({ children, onClick, startIcon }: any) => (
    <button data-testid="button" onClick={onClick}>
      {startIcon}
      {children}
    </button>
  ),
  Typography: ({ children }: any) => (
    <span data-testid="typography">{children}</span>
  ),
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Bot: () => <svg data-testid="icon-bot" />,
  ArrowLeft: () => <svg data-testid="icon-back" />,
}));

describe("ChatHeader", () => {
  it("should render identity and back button correctly", () => {
    const onBackMock = vi.fn();
    render(<ChatHeader onBack={onBackMock} />);

    expect(screen.getByText("Chat with Novera")).toBeInTheDocument();
    expect(
      screen.getByText("AI-powered support assistant"),
    ).toBeInTheDocument();
    expect(screen.getByText("Back to Support")).toBeInTheDocument();
    expect(screen.getByTestId("icon-bot")).toBeInTheDocument();
    expect(screen.getByTestId("icon-back")).toBeInTheDocument();
  });

  it("should call onBack when back button is clicked", () => {
    const onBackMock = vi.fn();
    render(<ChatHeader onBack={onBackMock} />);

    const backButton = screen.getByRole("button", { name: /Back to Support/i });
    fireEvent.click(backButton);

    expect(onBackMock).toHaveBeenCalled();
  });
});
