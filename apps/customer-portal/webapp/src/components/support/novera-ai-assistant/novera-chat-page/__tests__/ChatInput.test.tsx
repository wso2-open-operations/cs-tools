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
import ChatInput from "@components/support/novera-ai-assistant/novera-chat-page/ChatInput";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  Button: ({ children, onClick }: any) => (
    <button onClick={onClick}>{children}</button>
  ),
  CircularProgress: () => <span data-testid="circular-progress" />,
  IconButton: ({ children, onClick, disabled }: any) => (
    <button data-testid="icon-button" onClick={onClick} disabled={disabled}>
      {children}
    </button>
  ),
  Paper: ({ children }: any) => <div data-testid="paper">{children}</div>,
  Typography: ({ children }: any) => (
    <span data-testid="typography">{children}</span>
  ),
  Stack: ({ children }: any) => <div>{children}</div>,
  colors: {
    orange: {
      700: "#C2410C",
    },
  },
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Send: () => <svg data-testid="icon-send" />,
  Sparkles: () => <svg data-testid="icon-sparkles" />,
}));

// Mock Tooltip component
vi.mock("@wso2/oxygen-ui", async () => {
  const actual: any = await vi.importActual("@wso2/oxygen-ui");
  return {
    ...actual,
    Tooltip: ({ children }: any) => <div data-testid="tooltip">{children}</div>,
  };
});

// Mock Editor as a simple input for testing
vi.mock("@components/common/rich-text-editor/Editor", () => ({
  default: ({ value, onChange, placeholder, onSubmitKeyDown }: any) => (
    <div data-testid="chat-editor">
      <span data-testid="editor-placeholder">{placeholder}</span>
      <input
        data-testid="editor-input"
        value={value}
        onChange={(e) => onChange?.(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            onSubmitKeyDown?.();
          }
        }}
      />
    </div>
  ),
}));

vi.mock("@utils/richTextEditor", () => ({
  htmlToPlainText: (html: string) => html || "",
}));

describe("ChatInput", () => {
  it("should render editor and send button", () => {
    const onSendMock = vi.fn();
    const setInputMock = vi.fn();
    render(
      <ChatInput
        inputValue=""
        setInputValue={setInputMock}
        onSend={onSendMock}
      />,
    );

    expect(screen.getByTestId("editor-placeholder")).toHaveTextContent(
      /Type your message/,
    );
    expect(screen.getByTestId("icon-send")).toBeInTheDocument();
  });

  it("should call onSend when clicking send button with content", () => {
    const onSendMock = vi.fn();
    const setInputMock = vi.fn();
    render(
      <ChatInput
        inputValue="<p>Hi</p>"
        setInputValue={setInputMock}
        onSend={onSendMock}
      />,
    );

    const sendButton = screen.getByTestId("icon-button");
    fireEvent.click(sendButton);

    expect(onSendMock).toHaveBeenCalled();
  });
});
