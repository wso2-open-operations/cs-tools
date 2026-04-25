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
import { Pin } from "@wso2/oxygen-ui-icons-react";
import { NOVERA_WEBSOCKET_INITIALIZATION_ENDPOINT } from "../config/endpoints";
import { useMe } from "../context/me";
import type { FinalNoveraResponse, NoveraResponse } from "../types/novera.dto";
import { getAccessToken, getIdToken } from "../services/auth";

dayjs.extend(relativeTime);

export default function ChatPage() {
  const navigate = useNavigate();
  const { projectId, projectTypeId } = useProject();
  const { id: userId } = useMe();
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

  const [activeStreamingMessage, setActiveStreamingMessage] = useState<ChatMessage | null>(null);
  const [conversationId, setConversationId] = useState<string | null>(null);
  const [ws, setWs] = useState<WebSocket | null>(null);

  useEffect(() => {
    let websocket: WebSocket;

    const init = async () => {
      try {
        const accessToken = getAccessToken();
        const idToken = getIdToken();

        websocket = new WebSocket(NOVERA_WEBSOCKET_INITIALIZATION_ENDPOINT(projectId!), [
          "choreo-oauth2-token",
          accessToken as string,
          "cs-customer-portal",
          idToken as string,
        ]);

        setWs(websocket);

        websocket.onmessage = (event) => {
          try {
            const data: NoveraResponse = JSON.parse(event.data);
            handleNoveraResponse(data);
          } catch (error) {
            console.error("Failed to parse Novera response:", error);
          }
        };

        websocket.onerror = (error) => {
          console.error("WebSocket error observed:", error);
          websocket.close();
        };
      } catch (error) {
        console.error("Initialization failed", error);
      }
    };

    init();

    return () => {
      if (websocket) {
        websocket.close();
      }
    };
  }, [projectId]);

  const { data: deployments = [], isLoading: deploymentsLoading } = useQuery(projects.deployments(projectId!));

  const productQueries = useQueries({
    queries: deployments.map((deployment) => ({
      ...projects.products(deployment.id),
      enabled: !!deployment.id,
    })),
  });

  const productsLoading = productQueries.every((query) => query.isLoading);

  const mutation = useMutation({
    ...cases.classify,
    onSuccess: (response) => {
      setTimeout(() => {
        navigate("/create", { state: { messages, classifications: response } });
      }, 500);
    },
    onSettled: () => setIsAwaitingCreateCase(false),
  });

  const envProducts = deployments.reduce((acc, deployment, index) => {
    const products = productQueries[index]?.data ?? [];
    const productNames = products.map((p) => p.name);

    return {
      ...acc,
      [deployment.name]: productNames,
    };
  }, {});

  const [isWaitingForAnimation, setIsWaitingForAnimation] = useState(false);
  const [pendingFinalData, setPendingFinalData] = useState<NoveraResponse | null>(null);

  const handleNoveraResponse = (response: NoveraResponse) => {
    switch (response.type) {
      case "conversation_created":
        setConversationId(response.conversationId);
        break;

      case "thinking_start":
        setActiveStreamingMessage({
          author: "assistant",
          blocks: [{ type: "text", value: "" }],
          thinking: true,
          animated: true,
        });
        break;

      case "thinking_step":
        setActiveStreamingMessage((prev) => (prev ? { ...prev, thinking: response.label } : null));
        break;

      case "token":
        setActiveStreamingMessage((prev) => {
          if (!prev) return null;
          return {
            ...prev,
            blocks: prev.blocks.map((b) => (b.type === "text" ? { ...b, value: b.value + response.content } : b)),
          };
        });
        break;

      case "final":
        setIsWaitingForAnimation(true);
        setPendingFinalData(response);

        break;
    }
  };

  const handleAnimationComplete = () => {
    if (isWaitingForAnimation && activeStreamingMessage && pendingFinalData) {
      setMessages((prev) => [
        ...prev,
        {
          animated: false,
          thinking: false,
          author: "assistant",
          blocks: [{ type: "text", value: (pendingFinalData as FinalNoveraResponse).payload.message }],
          timestamp: dayjs().fromNow(),
        },
      ]);
      setActiveStreamingMessage(null);
      setIsWaitingForAnimation(false);
      setPendingFinalData(null);
    }
  };

  const sendMessage = (message: string) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(
        JSON.stringify({
          type: "user_message",
          accountId: userId,
          conversationId: conversationId ?? "",
          message: message,
          envProducts,
        }),
      );
    }
  };

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

    sendMessage(comment);
  };

  const [isAwaitingCreateCase, setIsAwaitingCreateCase] = useState(false);

  const handleCreateCase = () => {
    setIsAwaitingCreateCase(true);
  };

  useEffect(() => {
    if (isAwaitingCreateCase) {
      mutation.mutate({
        projectTypeId,
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
    }
  }, [isAwaitingCreateCase, deploymentsLoading, productsLoading]);

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
        open={isAwaitingCreateCase && (deploymentsLoading || productsLoading || mutation.isPending)}
      >
        <CircularProgress color="inherit" />
      </Backdrop>

      <Stack mb={20} gap={2}>
        {messages.map((message, index) => (
          <MessageBubble key={index} {...message} />
        ))}

        {activeStreamingMessage && (
          <MessageBubble {...activeStreamingMessage} onAnimationComplete={handleAnimationComplete} />
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
