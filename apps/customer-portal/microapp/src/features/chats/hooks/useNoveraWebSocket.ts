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

import { useEffect, useState } from "react";
import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import { NOVERA_WEBSOCKET_INITIALIZATION_ENDPOINT } from "@config/endpoints";
import { useMe } from "@context/me";
import { getAccessToken, getIdToken } from "@infrastructure/api/auth";
import type { ChatMessage } from "@features/chats/components";
import type { FinalNoveraResponse, NoveraResponse } from "@features/chats/types/novera.dto";

dayjs.extend(relativeTime);

export function useNoveraWebSocket(projectId: string) {
  const { id: userId } = useMe();
  const [ws, setWs] = useState<WebSocket | null>(null);
  const [conversationId, setConversationId] = useState<string | null>(null);
  const [activeStreamingMessage, setActiveStreamingMessage] = useState<ChatMessage | null>(null);
  const [isWaitingForAnimation, setIsWaitingForAnimation] = useState(false);
  const [pendingFinalData, setPendingFinalData] = useState<NoveraResponse | null>(null);

  useEffect(() => {
    let websocket: WebSocket;

    const init = async () => {
      try {
        const accessToken = getAccessToken();
        const idToken = getIdToken();

        websocket = new WebSocket(NOVERA_WEBSOCKET_INITIALIZATION_ENDPOINT(projectId), [
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

  const handleAnimationComplete = (onMessage: (msg: ChatMessage) => void) => {
    if (isWaitingForAnimation && activeStreamingMessage && pendingFinalData) {
      onMessage({
        animated: false,
        thinking: false,
        author: "assistant",
        blocks: [{ type: "text", value: (pendingFinalData as FinalNoveraResponse).payload.message }],
        timestamp: dayjs().fromNow(),
      });
      setActiveStreamingMessage(null);
      setIsWaitingForAnimation(false);
      setPendingFinalData(null);
    }
  };

  const sendMessage = (message: string, envProducts: object) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(
        JSON.stringify({
          type: "user_message",
          accountId: userId,
          conversationId: conversationId ?? "",
          message,
          envProducts,
        }),
      );
    }
  };

  return { ws, activeStreamingMessage, handleAnimationComplete, sendMessage };
}
