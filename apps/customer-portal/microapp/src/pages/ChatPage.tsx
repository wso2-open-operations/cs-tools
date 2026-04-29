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

import { useEffect, useRef, useState } from "react";
import { Backdrop, Button, CircularProgress, colors, pxToRem, Stack, Typography } from "@wso2/oxygen-ui";
import { StickyCommentBar } from "@components/detail";
import { MessageBubble, type ChatMessage } from "@features/chats/components";
import { useProject } from "@context/project";
import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import { Pin } from "@wso2/oxygen-ui-icons-react";
import { useNoveraWebSocket } from "@features/chats/hooks/useNoveraWebSocket";
import { useChatData } from "@features/chats/hooks/useChatData";

dayjs.extend(relativeTime);

export default function ChatPage() {
  const { projectId } = useProject();
  const bottomRef = useRef<HTMLDivElement | null>(null);
  const [comment, setComment] = useState("");
  const [messages, setMessages] = useState<ChatMessage[]>([
    {
      animated: false,
      thinking: false,
      author: "assistant",
      blocks: [
        {
          type: "text",
          value:
            "Hi! I'm Novera, your AI-powered support assistant. How can I help you today? Please describe the issue you're experiencing.",
        },
      ],
    },
  ]);

  const { ws, activeStreamingMessage, handleAnimationComplete, sendMessage } = useNoveraWebSocket(projectId!);

  const { deploymentsLoading, productsLoading, envProducts, classifyMutation, isAwaitingCreateCase, handleCreateCase } =
    useChatData(messages);

  const handleSend = () => {
    if (!comment.trim()) return;

    setMessages((prev) => [
      ...prev,
      {
        animated: false,
        thinking: false,
        author: "you",
        blocks: [{ type: "text", value: comment }],
        timestamp: dayjs().fromNow(),
      },
    ]);

    setComment("");

    sendMessage(comment, envProducts);
  };

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, activeStreamingMessage]);

  return (
    <>
      <Backdrop
        sx={{
          color: "primary.contrastText",
          zIndex: (theme) => theme.zIndex.drawer + 1,
          flexDirection: "column",
          gap: 2,
        }}
        open={isAwaitingCreateCase && (deploymentsLoading || productsLoading || classifyMutation.isPending)}
      >
        <CircularProgress color="inherit" />
      </Backdrop>

      <Stack mb={20} gap={2}>
        {messages.map((message, index) => (
          <MessageBubble key={index} {...message} />
        ))}

        {activeStreamingMessage && (
          <MessageBubble
            {...activeStreamingMessage}
            onAnimationComplete={() =>
              handleAnimationComplete((msg) => setMessages((prev) => [...prev, msg]))
            }
          />
        )}
      </Stack>
      <div ref={bottomRef} />

      <StickyCommentBar
        loading={!!activeStreamingMessage}
        disabled={ws?.readyState !== WebSocket.OPEN}
        value={comment}
        placeholder="Type your message"
        onChange={setComment}
        onSend={handleSend}
        topSlot={
          messages.length > 2 && (
            <Stack
              direction="row"
              alignItems="center"
              gap={2}
              p={2}
              sx={{ borderBottom: `1px solid ${colors.grey[200]}`, position: "relative" }}
            >
              <Pin
                size={pxToRem(12)}
                fill={colors.grey[500]}
                style={{ color: colors.grey[500], position: "absolute", right: 3, top: 5 }}
              />
              <Typography variant="body2">I can create a support case with all the details we've discussed.</Typography>
              <Button variant="contained" sx={{ textTransform: "initial", flexShrink: 0 }} onClick={handleCreateCase}>
                Create Case
              </Button>
            </Stack>
          )
        }
      />
    </>
  );
}

