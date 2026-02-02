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

import { Box } from "@wso2/oxygen-ui";
import { type JSX, type RefObject } from "react";
import ChatMessageBubble from "@/components/support/noveraAIAssistant/noveraChatPage/ChatMessageBubble";

interface Message {
  id: string;
  text: string;
  sender: "user" | "bot";
  timestamp: Date;
}

interface ChatMessageListProps {
  messages: Message[];
  messagesEndRef: RefObject<HTMLDivElement | null>;
}

/**
 * Renders the list of chat messages.
 *
 * Displays messages in chronological order with appropriate styling
 * for user and bot messages.
 *
 * @returns The ChatMessageList JSX element.
 */
export default function ChatMessageList({
  messages,
  messagesEndRef,
}: ChatMessageListProps): JSX.Element {
  return (
    <Box
      sx={{
        flex: 1,
        overflowY: "auto",
        p: 3,
        display: "flex",
        flexDirection: "column",
        gap: 3,
      }}
    >
      {messages.map((msg) => (
        <ChatMessageBubble key={msg.id} message={msg} />
      ))}
      <div ref={messagesEndRef} />
    </Box>
  );
}
