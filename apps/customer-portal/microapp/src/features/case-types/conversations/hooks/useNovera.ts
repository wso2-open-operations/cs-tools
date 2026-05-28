import { useEffect, useState } from "react";

import { useMe } from "@context/me";

import { NOVERA_WEBSOCKET_INITIALIZATION_ENDPOINT } from "@config/endpoints";

import { getAccessToken, getIdToken } from "@infrastructure/api/auth";

import { useEnvProducts } from "@features/case-types/conversations/hooks";
import type { NoveraResponse } from "@features/case-types/conversations/types/novera.dto";

export function useNovera(projectId: string, onData: (data: NoveraResponse) => void) {
  const { id: userId } = useMe();
  const { envProducts } = useEnvProducts();
  const [ws, setWs] = useState<WebSocket | null>(null);
  const [conversationId, setConversationId] = useState<string | null>(null);

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
            if (data.type === "conversation_created") {
              setConversationId(data.conversationId);
            } else {
              onData(data);
            }
          } catch (error) {
            console.error("Failed to parse response:", error);
          }
        };

        websocket.onerror = (error) => {
          console.error("WebSocket error:", error);
          websocket.close();
        };
      } catch (error) {
        console.error("Initialization failed:", error);
      }
    };

    init();

    return () => {
      if (websocket) websocket.close();
    };
  }, [projectId]);

  const send = (message: string) => {
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

  return {
    status: ws?.readyState ?? WebSocket.CLOSED,
    send,
  };
}
