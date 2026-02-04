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

import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { MockConfigProvider, useMockConfig } from "../MockConfigProvider";

// Test component to consume context
const TestComponent = () => {
  const { isMockEnabled, setMockEnabled } = useMockConfig();
  return (
    <div>
      <div data-testid="mock-status">{isMockEnabled.toString()}</div>
      <button onClick={() => setMockEnabled(!isMockEnabled)}>
        Toggle Mock
      </button>
    </div>
  );
};

describe("MockConfigProvider", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
  });

  it("should provide default value of false", () => {
    render(
      <MockConfigProvider>
        <TestComponent />
      </MockConfigProvider>,
    );
    expect(screen.getByTestId("mock-status")).toHaveTextContent("false");
  });

  it("should initialize from localStorage", () => {
    localStorage.setItem("isMockEnabled", "true");
    render(
      <MockConfigProvider>
        <TestComponent />
      </MockConfigProvider>,
    );
    expect(screen.getByTestId("mock-status")).toHaveTextContent("true");
  });

  it("should update value and persist to localStorage", () => {
    render(
      <MockConfigProvider>
        <TestComponent />
      </MockConfigProvider>,
    );

    const button = screen.getByText("Toggle Mock");
    fireEvent.click(button);

    expect(screen.getByTestId("mock-status")).toHaveTextContent("true");
    expect(localStorage.getItem("isMockEnabled")).toBe("true");

    fireEvent.click(button);
    expect(screen.getByTestId("mock-status")).toHaveTextContent("false");
    expect(localStorage.getItem("isMockEnabled")).toBe("false");
  });

  it("should throw error if used outside provider", () => {
    const spy = vi.spyOn(console, "error");
    spy.mockImplementation(() => {});

    expect(() => render(<TestComponent />)).toThrow(
      "useMockConfig must be used within a MockConfigProvider",
    );

    spy.mockRestore();
  });
});
