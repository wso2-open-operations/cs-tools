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
import ChatMessageList from "@features/support/components/novera-ai-assistant/novera-chat-page/ChatMessageList";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  Paper: ({ children }: any) => <div data-testid="paper">{children}</div>,
  Typography: ({ children }: any) => (
    <span data-testid="typography">{children}</span>
  ),
  Button: ({ children }: any) => <button>{children}</button>,
  IconButton: ({ children }: any) => <button>{children}</button>,
  CircularProgress: () => <span data-testid="circular-progress" />,
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Bot: () => <svg data-testid="icon-bot" />,
  User: () => <svg data-testid="icon-user" />,
  FileText: () => <svg data-testid="icon-filetext" />,
  Copy: () => <svg data-testid="icon-copy" />,
}));

// Mock ReactMarkdown
vi.mock("react-markdown", () => ({
  default: ({ children }: any) => <span data-testid="markdown">{children}</span>,
}));

describe("ChatMessageList", () => {
  it("should render a list of messages", () => {
    const messages: any[] = [
      { id: "1", text: "Msg 1", sender: "bot", timestamp: new Date() },
      { id: "2", text: "Msg 2", sender: "user", timestamp: new Date() },
    ];
    const ref: any = { current: null };
    render(<ChatMessageList messages={messages} messagesEndRef={ref} />);

    expect(screen.getByText("Msg 1")).toBeInTheDocument();
    expect(screen.getByText("Msg 2")).toBeInTheDocument();
  });

  it("should render LoadingDotsBubble for loading messages", () => {
    const messages: any[] = [
      { id: "1", text: "User msg", sender: "user", timestamp: new Date() },
      {
        id: "2",
        text: "",
        sender: "bot",
        timestamp: new Date(),
        isLoading: true,
      },
    ];
    const ref: any = { current: null };
    render(<ChatMessageList messages={messages} messagesEndRef={ref} />);

    expect(screen.getByText("User msg")).toBeInTheDocument();
    expect(screen.getByTestId("icon-bot")).toBeInTheDocument();
  });
});
