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

import { useState } from "react";
import { Backdrop, Button, CircularProgress, Stack, Typography } from "@wso2/oxygen-ui";
import { StickyCommentBar } from "@components/features/detail";
import { MessageBubble, type ChatMessage } from "@components/features/chat";
import { useNavigate } from "react-router-dom";
import { useMutation, useQueries, useQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import { projects } from "@src/services/projects";
import { useProject } from "@context/project";

export default function ChatPage() {
  const navigate = useNavigate();
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

  const { projectId } = useProject();
  const { data: deployments = [] } = useQuery(projects.deployments(projectId!));

  const productQueries = useQueries({
    queries: deployments.map((deployment) => ({
      ...projects.products(deployment.id),
      enabled: !!deployment.id,
    })),
  });

  const mutation = useMutation({
    ...cases.classify,
    onSuccess: (response) => {
      setTimeout(() => {
        navigate("/create", { state: { messages, classifications: response } });
      }, 500);
    },
  });

  const handleCreateCase = () => {
    mutation.mutate({
      chatHistory: toString(messages),
      envProducts: deployments.reduce((acc, deployment, index) => {
        const products = productQueries[index]?.data ?? [];
        const productNames = products.map((p) => p.name);

        return {
          ...acc,
          [deployment.name]: productNames,
        };
      }, {}),
    });
  };

  return (
    <>
      <Backdrop
        sx={{
          color: "primary.contrastText",
          zIndex: (theme) => theme.zIndex.drawer + 1,
          flexDirection: "column",
          gap: 2,
        }}
        open={mutation.isPending}
      >
        <CircularProgress color="inherit" />
      </Backdrop>

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

const toString = (messages: ChatMessage[]): string => {
  return messages
    .map((msg) => {
      const role = msg.author === "you" ? "User" : "Assistant";
      const textContent = msg.blocks
        .filter((block) => block.type === "text")
        .map((block) => block.value)
        .join(" ");

      return `${role}: ${textContent}`;
    })
    .join("\n");
};
