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

import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import IdleTimeoutProvider from "./IdleTimeoutProvider";

const signOut = vi.fn().mockResolvedValue(undefined);
const auth = { signOut, isSignedIn: true, isLoading: false };
const idleOptions: { current: Record<string, unknown> | null } = { current: null };

vi.mock("@asgardeo/react", () => ({ useAsgardeo: () => auth }));
vi.mock("react-idle-timer", () => ({
  useIdleTimer: (options: Record<string, unknown>) => {
    idleOptions.current = options;
    return { activate: vi.fn() };
  },
}));
vi.mock("@/hooks/useLogger", () => ({
  useLogger: () => ({ error: vi.fn(), info: vi.fn(), warn: vi.fn(), debug: vi.fn() }),
}));
vi.mock("@components/SessionWarningDialog", () => ({ default: () => null }));

describe("IdleTimeoutProvider", () => {
  beforeEach(() => {
    signOut.mockClear();
    auth.isSignedIn = true;
    idleOptions.current = null;
  });

  it("shares the idle timer across tabs", () => {
    render(<IdleTimeoutProvider>x</IdleTimeoutProvider>);
    expect(idleOptions.current?.crossTab).toBe(true);
  });

  it("announces the sign-out and signs out when the timer goes idle", async () => {
    const events: string[] = [];
    const listener = (): void => {
      events.push("signing-out");
    };
    window.addEventListener("app:signing-out", listener);
    signOut.mockImplementationOnce(async () => {
      events.push("signOut");
    });
    render(<IdleTimeoutProvider>x</IdleTimeoutProvider>);

    (idleOptions.current?.onIdle as () => void)();
    await vi.waitFor(() => expect(signOut).toHaveBeenCalledTimes(1));
    expect(events).toEqual(["signing-out", "signOut"]);
    window.removeEventListener("app:signing-out", listener);
  });

  it("does nothing on idle when nobody is signed in", () => {
    auth.isSignedIn = false;
    render(<IdleTimeoutProvider>x</IdleTimeoutProvider>);
    (idleOptions.current?.onIdle as () => void)();
    expect(signOut).not.toHaveBeenCalled();
  });
});
