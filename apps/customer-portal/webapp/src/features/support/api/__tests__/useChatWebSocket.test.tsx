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

import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useChatWebSocket } from "@features/support/api/useChatWebSocket";

const mockGetAccessToken = vi.fn().mockResolvedValue("access-token");
const mockGetIdToken = vi.fn().mockResolvedValue("id-token");

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getAccessToken: mockGetAccessToken,
    getIdToken: mockGetIdToken,
  }),
}));

const wsInstances: MockWebSocket[] = [];

class MockWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;
  readyState = MockWebSocket.CONNECTING;
  onopen: (() => void) | null = null;
  onclose: ((event: { code: number; reason: string; wasClean: boolean }) => void) | null =
    null;
  onerror: ((event: unknown) => void) | null = null;
  onmessage: ((event: { data: unknown }) => void) | null = null;
  send = vi.fn();
  close = vi.fn(() => {
    this.readyState = MockWebSocket.CLOSED;
  });

  constructor(
    public readonly url: string,
    public readonly protocols: string[],
  ) {
    wsInstances.push(this);
  }
}

describe("useChatWebSocket", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    wsInstances.length = 0;
    vi.stubGlobal("config", {
      CUSTOMER_PORTAL_CHATBOT_WEBSOCKET_URL: "wss://socket.test/ws",
    });
    vi.stubGlobal("WebSocket", MockWebSocket as unknown as typeof WebSocket);
  });

  it("should connect, send payload, and close websocket", async () => {
    const onEvent = vi.fn();
    const onError = vi.fn();
    const { result } = renderHook(() =>
      useChatWebSocket({ onEvent, onError }),
    );

    let connectPromise: Promise<void>;
    await act(async () => {
      connectPromise = result.current.connect("session-1");
    });

    const ws = wsInstances[0];
    ws.readyState = MockWebSocket.OPEN;
    ws.onopen?.();

    await connectPromise!;
    await waitFor(() => expect(result.current.isConnected).toBe(true));
    expect(ws.url).toBe("wss://socket.test/ws?sessionId=session-1");

    await act(async () => {
      await result.current.sendUserMessage({ type: "USER_MESSAGE", content: "hello" } as never);
    });
    expect(ws.send).toHaveBeenCalledWith(
      JSON.stringify({ type: "USER_MESSAGE", content: "hello" }),
    );

    act(() => result.current.close());
    expect(ws.close).toHaveBeenCalled();
    expect(result.current.isConnected).toBe(false);
  });
});
