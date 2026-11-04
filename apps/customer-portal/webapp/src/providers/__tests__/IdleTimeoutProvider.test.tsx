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

import { act, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import IdleTimeoutProvider from "@providers/IdleTimeoutProvider";
import { useIsIdle } from "@providers/IdleStateContext";

const activateMock = vi.fn();
let capturedOnPrompt: (() => void) | undefined;

const authState = vi.hoisted(() => ({ isSignedIn: true, isLoading: false }));
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ signOut: vi.fn(), ...authState }),
}));

vi.mock("react-idle-timer", () => ({
  useIdleTimer: (opts: { onPrompt: () => void }) => {
    capturedOnPrompt = opts.onPrompt;
    return { activate: activateMock };
  },
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ error: vi.fn(), debug: vi.fn(), warn: vi.fn() }),
}));

vi.mock("@components/SessionWarningDialog", () => ({
  default: ({ open, onContinue }: { open: boolean; onContinue: () => void }) =>
    open ? (
      <div data-testid="session-warning">
        <button onClick={onContinue}>continue</button>
      </div>
    ) : null,
}));

function IdleProbe() {
  return <span data-testid="idle">{String(useIsIdle())}</span>;
}

describe("IdleTimeoutProvider", () => {
  it("renders children and wires idle timer", () => {
    render(
      <IdleTimeoutProvider>
        <span>App</span>
      </IdleTimeoutProvider>,
    );
    expect(screen.getByText("App")).toBeInTheDocument();
    expect(activateMock).toBeDefined();
  });

  describe("idle state", () => {
    beforeEach(() => {
      authState.isSignedIn = true;
      authState.isLoading = false;
      activateMock.mockClear();
    });

    const renderProbe = () =>
      render(
        <IdleTimeoutProvider>
          <IdleProbe />
        </IdleTimeoutProvider>,
      );

    it("is not idle until the prompt opens", () => {
      renderProbe();
      expect(screen.getByTestId("idle")).toHaveTextContent("false");
    });

    it("becomes idle when the prompt opens and clears on Continue", () => {
      renderProbe();
      act(() => capturedOnPrompt?.());
      expect(screen.getByTestId("idle")).toHaveTextContent("true");

      act(() => screen.getByText("continue").click());
      expect(screen.getByTestId("idle")).toHaveTextContent("false");
      expect(activateMock).toHaveBeenCalled();
    });

    it("ignores the prompt when signed out or still loading", () => {
      authState.isSignedIn = false;
      const { unmount } = renderProbe();
      act(() => capturedOnPrompt?.());
      expect(screen.getByTestId("idle")).toHaveTextContent("false");
      unmount();

      authState.isSignedIn = true;
      authState.isLoading = true;
      renderProbe();
      act(() => capturedOnPrompt?.());
      expect(screen.getByTestId("idle")).toHaveTextContent("false");
    });
  });
});
