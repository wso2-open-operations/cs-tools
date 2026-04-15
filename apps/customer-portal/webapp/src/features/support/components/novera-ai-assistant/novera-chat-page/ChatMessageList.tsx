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

import { Box, Skeleton } from "@wso2/oxygen-ui";
import {
  type JSX,
  type RefObject,
  useCallback,
  useEffect,
  useRef,
} from "react";
import ChatMessageBubble from "@features/support/components/novera-ai-assistant/novera-chat-page/ChatMessageBubble";
import LoadingDotsBubble from "@features/support/components/novera-ai-assistant/novera-chat-page/LoadingDotsBubble";
import type { Message } from "@features/support/types/conversations";

interface ChatMessageListProps {
  messages: Message[];
  messagesEndRef: RefObject<HTMLDivElement | null>;
  onCreateCase?: () => void;
  onThumbsUp?: (messageId: string) => void;
  onThumbsDown?: (messageId: string) => void;
  onFetchOlder?: () => void;
  isFetchingOlder?: boolean;
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
  onCreateCase,
  onThumbsUp,
  onThumbsDown,
  onFetchOlder,
  isFetchingOlder = false,
}: ChatMessageListProps): JSX.Element {
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const pendingPrependRef = useRef(false);
  const prevScrollHeightRef = useRef(0);

  const handleScroll = useCallback(() => {
    const node = scrollRef.current;
    if (!node || !onFetchOlder || isFetchingOlder) return;
    if (node.scrollTop <= 0) {
      pendingPrependRef.current = true;
      prevScrollHeightRef.current = node.scrollHeight;
      onFetchOlder();
    }
  }, [onFetchOlder, isFetchingOlder]);

  useEffect(() => {
    const node = scrollRef.current;
    if (!node) return;
    // When older messages are prepended, keep the user's viewport anchored.
    if (pendingPrependRef.current) {
      const newHeight = node.scrollHeight;
      const delta = newHeight - prevScrollHeightRef.current;
      node.scrollTop = delta;
      pendingPrependRef.current = false;
    }
  }, [messages.length]);

  return (
    <Box
      ref={scrollRef}
      onScroll={handleScroll}
      sx={{
        flex: 1,
        overflowY: "auto",
        p: 3,
        display: "flex",
        flexDirection: "column",
        gap: 3,
      }}
    >
      {isFetchingOlder && (
        <Box sx={{ display: "flex", justifyContent: "center" }}>
          <Skeleton variant="rounded" width={220} height={28} />
        </Box>
      )}
      {messages.map((msg) =>
        msg.isLoading ? (
          <LoadingDotsBubble key={msg.id} />
        ) : (
          <ChatMessageBubble
            key={msg.id}
            message={msg}
            onCreateCase={onCreateCase}
            onThumbsUp={onThumbsUp}
            onThumbsDown={onThumbsDown}
          />
        ),
      )}
      <div ref={messagesEndRef} />
    </Box>
  );
}
