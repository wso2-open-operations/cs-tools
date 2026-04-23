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
import ChatMessageBubble from "@features/support/components/novera-ai-assistant/novera-chat-page/ChatMessageBubble";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  Paper: ({ children }: any) => <div data-testid="paper">{children}</div>,
  Typography: ({ children }: any) => (
    <span data-testid="typography">{children}</span>
  ),
  Button: ({ children, onClick }: any) => (
    <button onClick={onClick} data-testid="button">
      {children}
    </button>
  ),
  IconButton: ({ children, onClick }: any) => (
    <button onClick={onClick} data-testid="icon-button">
      {children}
    </button>
  ),
  CircularProgress: () => <span data-testid="circular-progress" />,
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Bot: () => <svg data-testid="icon-bot" />,
  User: () => <svg data-testid="icon-user" />,
  ThumbsUp: () => <svg data-testid="icon-thumbs-up" />,
  ThumbsDown: () => <svg data-testid="icon-thumbs-down" />,
  FileText: () => <svg data-testid="icon-filetext" />,
  Copy: () => <svg data-testid="icon-copy" />,
}));

// Mock ReactMarkdown
vi.mock("react-markdown", () => ({
  default: ({ children }: any) => <span data-testid="markdown">{children}</span>,
}));

describe("ChatMessageBubble", () => {
  it("should render user message correctly", () => {
    const userMsg: any = {
      id: "1",
      text: "Hello Bot",
      sender: "user",
      timestamp: new Date(),
    };
    render(<ChatMessageBubble message={userMsg} />);

    expect(screen.getByText("Hello Bot")).toBeInTheDocument();
    expect(screen.getByTestId("icon-user")).toBeInTheDocument();
    expect(screen.queryByTestId("icon-bot")).not.toBeInTheDocument();
  });

  it("should render bot message with avatar correctly", () => {
    const botMsg: any = {
      id: "2",
      text: "Hello User",
      sender: "bot",
      timestamp: new Date(),
    };
    render(<ChatMessageBubble message={botMsg} />);

    expect(screen.getByText("Hello User")).toBeInTheDocument();
    expect(screen.getByTestId("icon-bot")).toBeInTheDocument();
    expect(screen.getAllByTestId("icon-button").length).toBeGreaterThanOrEqual(1);
  });

  it("should render error state", () => {
    const botMsg: any = {
      id: "2",
      text: "",
      sender: "bot",
      timestamp: new Date(),
      isError: true,
    };
    render(<ChatMessageBubble message={botMsg} />);

    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
  });
});
