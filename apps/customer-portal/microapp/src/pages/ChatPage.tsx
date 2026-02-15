// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import { useState } from "react";
import { Button, Stack, Typography } from "@wso2/oxygen-ui";
import { Link } from "react-router-dom";
import { StickyCommentBar } from "@components/features/detail";
import { MessageBubble, type ChatMessage } from "@components/features/chat";

export default function ChatPage() {
  const [comment, setComment] = useState("");
  const [messages, setMessages] = useState<ChatMessage[]>([
    {
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

  const handleSend = () => {
    if (!comment.trim()) return;

    const userMessage: ChatMessage = {
      author: "you",
      blocks: [{ type: "text", value: comment }],
    };

    const assistantMessage: ChatMessage = {
      author: "assistant",
      blocks: [
        {
          type: "text",
          value: "Thanks for those details. Based on what you've shared, here are a few things to check:",
        },
        {
          type: "checklist",
          items: [
            "Verify your backend service timeout configurations",
            "Check system resource utilization CPU, memory",
            "Review recent deployment or configuration changes",
          ],
        },
        {
          type: "kb",
          items: [
            { id: "KB-1234", title: "Troubleshooting API Gateway Timeouts" },
            { id: "KB-1234", title: "Troubleshooting API Gateway Timeouts" },
          ],
        },
      ],
    };

    setMessages((prev) => [...prev, userMessage]);
    setTimeout(() => setMessages((prev) => [...prev, assistantMessage]), 2000);
  };

  return (
    <>
      <Stack mb={20} gap={2}>
        {messages.map((message, index) => (
          <MessageBubble key={index} {...message} />
        ))}
      </Stack>

      <StickyCommentBar
        value={comment}
        placeholder="Type your message"
        onChange={setComment}
        onSend={handleSend}
        topSlot={
          messages.length > 2 && (
            <Stack direction="row" alignItems="center" gap={2}>
              <Typography variant="body2">I can create a support case with all the details we've discussed.</Typography>
              <Button
                component={Link}
                to="/create"
                variant="contained"
                state={{ messages }}
                sx={{ textTransform: "initial", flexShrink: 0 }}
              >
                Create Case
              </Button>
            </Stack>
          )
        }
      />
    </>
  );
}
