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

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import IdleTimeoutProvider from "@providers/IdleTimeoutProvider";

const activateMock = vi.fn();

vi.mock("react-idle-timer", () => ({
  useIdleTimer: () => ({ activate: activateMock }),
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ error: vi.fn(), debug: vi.fn(), warn: vi.fn() }),
}));

vi.mock("@components/SessionWarningDialog", () => ({
  default: ({ open }: { open: boolean }) =>
    open ? <div data-testid="session-warning">warning</div> : null,
}));

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
});
