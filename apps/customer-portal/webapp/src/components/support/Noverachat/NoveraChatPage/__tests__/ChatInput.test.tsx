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
import ChatInput from "@/components/support/Noverachat/NoveraChatPage/ChatInput";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  IconButton: ({ children, onClick, disabled }: any) => (
    <button data-testid="icon-button" onClick={onClick} disabled={disabled}>
      {children}
    </button>
  ),
  TextField: ({ value, onChange, placeholder, onKeyDown }: any) => (
    <input
      data-testid="text-field"
      value={value}
      placeholder={placeholder}
      onChange={onChange}
      onKeyDown={onKeyDown}
    />
  ),
  Paper: ({ children }: any) => <div data-testid="paper">{children}</div>,
  Typography: ({ children }: any) => (
    <span data-testid="typography">{children}</span>
  ),
  Button: ({ children }: any) => <button>{children}</button>,
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Send: () => <svg data-testid="icon-send" />,
  CircleAlert: () => <svg data-testid="icon-alert" />,
}));

describe("ChatInput", () => {
  it("should render input and send button", () => {
    const onSendMock = vi.fn();
    const setInputMock = vi.fn();
    render(
      <ChatInput
        inputValue=""
        setInputValue={setInputMock}
        onSend={onSendMock}
        showEscalationBanner={false}
      />,
    );

    expect(
      screen.getByPlaceholderText("Type your message..."),
    ).toBeInTheDocument();
    expect(screen.getByTestId("icon-send")).toBeInTheDocument();
  });

  it("should call onSend when clicking send button", () => {
    const onSendMock = vi.fn();
    const setInputMock = vi.fn();
    render(
      <ChatInput
        inputValue="Hi"
        setInputValue={setInputMock}
        onSend={onSendMock}
        showEscalationBanner={false}
      />,
    );

    const sendButton = screen.getByTestId("icon-button");
    fireEvent.click(sendButton);

    expect(onSendMock).toHaveBeenCalled();
  });

  it("should show escalation banner when visible is true", () => {
    const onSendMock = vi.fn();
    const setInputMock = vi.fn();
    render(
      <ChatInput
        inputValue=""
        setInputValue={setInputMock}
        onSend={onSendMock}
        showEscalationBanner={true}
      />,
    );

    expect(screen.getByText(/Need more help?/i)).toBeInTheDocument();
    expect(screen.getByText("Create Case")).toBeInTheDocument();
  });
});
