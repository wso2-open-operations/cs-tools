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
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import ChatMessageList from "@features/support/components/novera-ai-assistant/novera-chat-page/ChatMessageList";
import {
  ChatSender,
  type Message,
} from "@features/support/types/conversations";

vi.mock("react-markdown", () => ({
  default: ({ children }: { children: string }) => (
    <span data-testid="markdown">{children}</span>
  ),
}));

function renderList(messages: Message[]) {
  const messagesEndRef = createRef<HTMLDivElement>();
  return render(
    <ThemeProvider theme={createTheme()}>
      <ChatMessageList messages={messages} messagesEndRef={messagesEndRef} />
    </ThemeProvider>,
  );
}

describe("ChatMessageList", () => {
  it("should render a list of messages", () => {
    renderList([
      { id: "1", text: "Msg 1", sender: ChatSender.BOT, timestamp: new Date() },
      { id: "2", text: "Msg 2", sender: ChatSender.USER, timestamp: new Date() },
    ]);

    expect(screen.getByText("Msg 1")).toBeInTheDocument();
    expect(screen.getByText("Msg 2")).toBeInTheDocument();
  });

  it("should render LoadingDotsBubble for loading messages", () => {
    renderList([
      { id: "1", text: "User msg", sender: ChatSender.USER, timestamp: new Date() },
      {
        id: "2",
        text: "",
        sender: ChatSender.BOT,
        timestamp: new Date(),
        isLoading: true,
      },
    ]);

    expect(screen.getByText("User msg")).toBeInTheDocument();
    expect(
      screen.getByRole("status", { name: /Novera is preparing a response/i }),
    ).toBeInTheDocument();
  });
});
