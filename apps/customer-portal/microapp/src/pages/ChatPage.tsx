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
import { StickyCommentBar } from "@components/features/detail";
import { MessageBubble, type ChatMessage } from "@components/features/chat";
import { useNavigate } from "react-router-dom";
import { useMutation, useQueries, useQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import { projects } from "@src/services/projects";
import { useProject } from "@context/project";
import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import { chats } from "../services/chats";
import type { MessageDispatchDTO } from "../types/chat.dto";
import { Pin } from "@wso2/oxygen-ui-icons-react";

dayjs.extend(relativeTime);

export default function ChatPage() {
  const navigate = useNavigate();
  const { projectId } = useProject();
  const bottomRef = useRef<HTMLDivElement | null>(null);
  const [comment, setComment] = useState("");
  const [conversationId, setConversationId] = useState<string | undefined>(undefined);
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

  const { mutate: createConversation, isPending: isCreatingConversation } = useMutation({
    ...chats.initiate(projectId!),
    onSuccess: (response) => {
      setConversationId(response.conversationId);
      setMessages((prev) => [
        ...prev,
        {
          author: "assistant",
          blocks: [{ type: "text", value: response.content }],
          timestamp: dayjs(response.timestamp).fromNow(),
        },
      ]);
    },
  });

  const { mutate: createMessage, isPending: isCreatingMessage } = useMutation({
    ...chats.send(projectId!, conversationId!),
    onSuccess: (response) => {
      setMessages((prev) => [
        ...prev,
        {
          author: "assistant",
          blocks: [{ type: "text", value: response.content }],
          timestamp: dayjs(response.timestamp).fromNow(),
        },
      ]);
    },
  });

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

  const envProducts = deployments.reduce((acc, deployment, index) => {
    const products = productQueries[index]?.data ?? [];
    const productNames = products.map((p) => p.name);

    return {
      ...acc,
      [deployment.name]: productNames,
    };
  }, {});

  const handleSend = () => {
    if (!comment.trim()) return;

    setMessages((prev) => [
      ...prev,
      {
        author: "you",
        blocks: [{ type: "text", value: comment }],
        timestamp: dayjs().fromNow(),
      },
    ]);

    const payload: Omit<MessageDispatchDTO, "region" | "tier"> = { message: comment, envProducts: envProducts };
    setComment("");

    if (conversationId) createMessage(payload);
    else createConversation(payload);
  };

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

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

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
      <div ref={bottomRef} />

      <StickyCommentBar
        loading={isCreatingConversation || isCreatingMessage}
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
