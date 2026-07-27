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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import ChatMessageBubble from "@features/support/components/novera-ai-assistant/novera-chat-page/ChatMessageBubble";
import type { ChatMessageBubbleProps } from "@features/support/types/supportComponents";
import {
  ChatSender,
  type Message,
} from "@features/support/types/conversations";

vi.mock("react-markdown", () => ({
  default: ({ children }: { children: string }) => (
    <span data-testid="markdown">{children}</span>
  ),
}));

function renderBubble(
  message: Message,
  props: Partial<ChatMessageBubbleProps> = {},
) {
  return render(
    <ThemeProvider theme={createTheme()}>
      <ChatMessageBubble message={message} {...props} />
    </ThemeProvider>,
  );
}

const botAnswer = (over: Partial<Message> = {}): Message => ({
  id: "bot-1",
  text: "Here is your answer",
  sender: ChatSender.BOT,
  timestamp: new Date(),
  feedbackMessageId: "msg-123",
  ...over,
});

describe("ChatMessageBubble", () => {
  it("should render user message correctly", () => {
    renderBubble({
      id: "1",
      text: "Hello Bot",
      sender: ChatSender.USER,
      timestamp: new Date(),
    });

    expect(screen.getByText("Hello Bot")).toBeInTheDocument();
  });

  it("should render bot message with avatar correctly", () => {
    renderBubble({
      id: "2",
      text: "Hello User",
      sender: ChatSender.BOT,
      timestamp: new Date(),
    });

    expect(screen.getByText("Hello User")).toBeInTheDocument();
  });

  it("should render error state", () => {
    renderBubble({
      id: "2",
      text: "",
      sender: ChatSender.BOT,
      timestamp: new Date(),
      isError: true,
    });

    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
  });

  it("should show a usage-limit message for credit/token errors", () => {
    renderBubble({
      id: "3",
      text: "Anthropic API error: Your credit balance is too low to access the Anthropic API.",
      sender: ChatSender.BOT,
      timestamp: new Date(),
      isError: true,
    });

    expect(
      screen.getByText(
        "The AI assistant is temporarily unavailable due to usage limits. Please try again later.",
      ),
    ).toBeInTheDocument();
  });

  it("should not leak raw error text for non usage-limit errors", () => {
    renderBubble({
      id: "4",
      text: "NullPointerException at line 42",
      sender: ChatSender.BOT,
      timestamp: new Date(),
      isError: true,
    });

    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
    expect(
      screen.queryByText("NullPointerException at line 42"),
    ).not.toBeInTheDocument();
  });

  describe("answer feedback (👍/👎)", () => {
    it("renders thumbs on a completed answer with a feedbackMessageId", () => {
      renderBubble(botAnswer(), {
        onThumbsUp: vi.fn(),
        onThumbsDown: vi.fn(),
      });

      expect(screen.getByLabelText("Good response")).toBeInTheDocument();
      expect(screen.getByLabelText("Bad response")).toBeInTheDocument();
    });

    it("calls onThumbsUp / onThumbsDown with the feedbackMessageId", () => {
      const onThumbsUp = vi.fn();
      const onThumbsDown = vi.fn();
      renderBubble(botAnswer(), { onThumbsUp, onThumbsDown });

      fireEvent.click(screen.getByLabelText("Good response"));
      expect(onThumbsUp).toHaveBeenCalledWith("msg-123");

      fireEvent.click(screen.getByLabelText("Bad response"));
      expect(onThumbsDown).toHaveBeenCalledWith("msg-123");
    });

    it("reflects the chosen rating via aria-pressed", () => {
      renderBubble(botAnswer({ feedbackRating: 1 }), {
        onThumbsUp: vi.fn(),
        onThumbsDown: vi.fn(),
      });

      expect(screen.getByLabelText("Good response")).toHaveAttribute(
        "aria-pressed",
        "true",
      );
      expect(screen.getByLabelText("Bad response")).toHaveAttribute(
        "aria-pressed",
        "false",
      );
    });

    it("hides thumbs when there is no feedbackMessageId", () => {
      renderBubble(botAnswer({ feedbackMessageId: undefined }), {
        onThumbsUp: vi.fn(),
        onThumbsDown: vi.fn(),
      });

      expect(screen.queryByLabelText("Good response")).not.toBeInTheDocument();
    });

    it("hides thumbs while the answer is still streaming", () => {
      renderBubble(botAnswer({ isStreaming: true }), {
        onThumbsUp: vi.fn(),
        onThumbsDown: vi.fn(),
      });

      expect(screen.queryByLabelText("Good response")).not.toBeInTheDocument();
    });
  });
});
