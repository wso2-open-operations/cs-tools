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
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import ChatInput from "@features/support/components/novera-ai-assistant/novera-chat-page/ChatInput";

vi.mock("@features/support/utils/richTextEditor", () => ({
  htmlToPlainText: (html: string) => html || "",
}));

function renderInput(
  props: Partial<Parameters<typeof ChatInput>[0]> = {},
) {
  const onSend = props.onSend ?? vi.fn();
  const setInputValue = props.setInputValue ?? vi.fn();
  return render(
    <ThemeProvider theme={createTheme()}>
      <ChatInput
        inputValue={props.inputValue ?? ""}
        setInputValue={setInputValue}
        onSend={onSend}
        {...props}
      />
    </ThemeProvider>,
  );
}

describe("ChatInput", () => {
  it("should render editor and send button", () => {
    renderInput();
    expect(
      screen.getByPlaceholderText("Type your message..."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /send message/i })).toBeInTheDocument();
  });

  it("should call onSend when clicking send button with content", () => {
    const onSendMock = vi.fn();
    renderInput({ inputValue: "Hi", onSend: onSendMock });

    fireEvent.click(screen.getByRole("button", { name: /send message/i }));

    expect(onSendMock).toHaveBeenCalled();
  });
});
