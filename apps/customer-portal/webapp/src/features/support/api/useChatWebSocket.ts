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

import {
  CONNECT_HANDSHAKE_TIMEOUT_MS,
  WS_CHOREO_OAUTH2_TOKEN,
  WS_CUSTOMER_PORTAL,
} from "@constants/apiConstants";
import type {
  ChatWebSocketEvent,
  ChatWebSocketPayload,
  UseChatWebSocketOptions,
} from "@features/support/types/conversations";
import { useAsgardeo } from "@asgardeo/react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

/**
 * Manages websocket connection lifecycle for Novera chatbot streaming events.
 *
 * @param {UseChatWebSocketOptions} options - Event callbacks.
 * @returns Connection helpers and current state.
 */
export function useChatWebSocket(options: UseChatWebSocketOptions) {
  const { getAccessToken, getIdToken } = useAsgardeo();
  const wsRef = useRef<WebSocket | null>(null);
  const sessionIdRef = useRef<string | null>(null);
  const connectPromiseRef = useRef<Promise<void> | null>(null);
  const [isConnected, setIsConnected] = useState(false);

  const wsBaseUrl = useMemo(() => {
    const raw = window.config?.CUSTOMER_PORTAL_CHATBOT_WEBSOCKET_URL?.trim();
    if (!raw) return "";
    return raw.endsWith("/") ? raw.slice(0, -1) : raw;
  }, []);

  const close = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    sessionIdRef.current = null;
    connectPromiseRef.current = null;
    setIsConnected(false);
  }, []);

  const connect = useCallback(
    async (sessionId: string): Promise<void> => {
      if (!wsBaseUrl) {
        throw new Error(
          "CUSTOMER_PORTAL_CHATBOT_WEBSOCKET_URL is not configured",
        );
      }

      if (
        wsRef.current &&
        wsRef.current.readyState === WebSocket.OPEN &&
        sessionIdRef.current === sessionId
      ) {
        return;
      }

      if (
        wsRef.current &&
        wsRef.current.readyState !== WebSocket.CLOSED &&
        sessionIdRef.current !== sessionId
      ) {
        close();
      }

      if (
        wsRef.current &&
        wsRef.current.readyState === WebSocket.CONNECTING &&
        sessionIdRef.current === sessionId &&
        connectPromiseRef.current
      ) {
        return connectPromiseRef.current;
      }

      const accessToken = await getAccessToken();
      const userIdToken = await getIdToken();
      if (!accessToken || !userIdToken) {
        throw new Error("Unable to retrieve access or ID token");
      }

      const url = `${wsBaseUrl}?sessionId=${encodeURIComponent(sessionId)}`;
      const protocols = [
        WS_CHOREO_OAUTH2_TOKEN,
        accessToken,
        WS_CUSTOMER_PORTAL,
        userIdToken,
      ];

      connectPromiseRef.current = new Promise<void>((resolve, reject) => {
        let settled = false;
        let handshakeTimer: number | undefined;

        const clearHandshakeTimer = () => {
          if (handshakeTimer !== undefined) {
            window.clearTimeout(handshakeTimer);
            handshakeTimer = undefined;
          }
        };

        const finish = (fn: () => void) => {
          if (settled) return;
          settled = true;
          clearHandshakeTimer();
          fn();
        };

        let ws: WebSocket;
        try {
          ws = new WebSocket(url, protocols);
        } catch (err) {
          console.error("[useChatWebSocket] WebSocket constructor failed", err);
          const message =
            err instanceof Error
              ? err.message
              : "Invalid WebSocket URL or subprotocols.";
          options.onError?.(message);
          reject(err instanceof Error ? err : new Error(message));
          return;
        }

        wsRef.current = ws;
        sessionIdRef.current = sessionId;

        handshakeTimer = window.setTimeout(() => {
          if (ws.readyState !== WebSocket.CONNECTING) return;
          console.error(
            "[useChatWebSocket] handshake timeout (still CONNECTING)",
            {
              sessionId,
              timeoutMs: CONNECT_HANDSHAKE_TIMEOUT_MS,
              hint: "Check DevTools → Network → WS for status (expect 101). Gateway may block or drop oversized Sec-WebSocket-Protocol.",
            },
          );
          options.onError?.("WebSocket connection timed out during handshake.");
          finish(() =>
            reject(
              new Error("WebSocket handshake timed out (still CONNECTING)."),
            ),
          );
          try {
            ws.close();
          } catch {
            /* ignore */
          }
        }, CONNECT_HANDSHAKE_TIMEOUT_MS);

        ws.onopen = () => {
          setIsConnected(true);
          finish(() => resolve());
        };

        ws.onerror = (event) => {
          console.error(
            "[useChatWebSocket] error event (handshake or transport)",
            {
              event,
              readyState: ws.readyState,
              sessionId,
            },
          );
          options.onError?.("WebSocket connection error.");
          finish(() => reject(new Error("WebSocket connection error.")));
        };

        ws.onclose = (event) => {
          console.warn("[useChatWebSocket] close", {
            code: event.code,
            reason: event.reason || "(empty)",
            wasClean: event.wasClean,
            sessionId,
          });
          setIsConnected(false);
          connectPromiseRef.current = null;
          options.onClose?.();
          finish(() =>
            reject(
              new Error(
                `WebSocket closed (${event.code}): ${event.reason || "no reason"}`,
              ),
            ),
          );
        };

        const decodeWebSocketData = async (data: unknown): Promise<string> => {
          if (typeof data === "string") return data;
          if (data instanceof Blob) return await data.text();
          if (data instanceof ArrayBuffer) {
            return new TextDecoder("utf-8").decode(new Uint8Array(data));
          }
          return "";
        };

        ws.onmessage = (event) => {
          void (async () => {
            let raw = await decodeWebSocketData(event.data);
            raw = raw.replace(/\r\n/g, "\n");
            raw = raw.replace(/\}\s*\{/g, "}\n{");

            const chunks = raw
              .split("\n")
              .map((s) => s.trim())
              .filter(Boolean);

            if (chunks.length === 0) {
              options.onError?.("Received empty websocket payload.");
              return;
            }

            let hadAnyValid = false;
            for (const chunk of chunks) {
              try {
                const parsed = JSON.parse(chunk) as ChatWebSocketEvent;
                hadAnyValid = true;
                options.onEvent(parsed);
              } catch {
                // Ignore invalid chunk; continue to next.
              }
            }

            if (!hadAnyValid) {
              options.onError?.("Received invalid websocket payload.");
            }
          })();
        };
      });

      return connectPromiseRef.current;
    },
    [close, getAccessToken, getIdToken, options, wsBaseUrl],
  );

  const sendUserMessage = useCallback(
    async (payload: ChatWebSocketPayload): Promise<void> => {
      const ws = wsRef.current;
      if (!ws || ws.readyState !== WebSocket.OPEN) {
        throw new Error("WebSocket is not connected");
      }

      ws.send(JSON.stringify(payload));
    },
    [],
  );

  useEffect(() => () => close(), [close]);

  return {
    connect,
    close,
    sendUserMessage,
    isConnected,
  };
}
