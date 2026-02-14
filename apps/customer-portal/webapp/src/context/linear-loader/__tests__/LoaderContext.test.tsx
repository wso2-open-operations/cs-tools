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

import { render, screen, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LoaderProvider, useLoader } from "@context/linear-loader/LoaderContext";

// Mock Oxygen UI
vi.mock("@wso2/oxygen-ui", () => ({
  LinearProgress: () => <div data-testid="linear-progress" />,
  Box: ({ children, sx }: any) => (
    <div data-testid="loader-box" style={sx}>
      {children}
    </div>
  ),
}));

// Test component to consume the context
const TestComponent = () => {
  const { showLoader, hideLoader, isVisible } = useLoader();

  return (
    <div>
      <span data-testid="is-visible">{isVisible.toString()}</span>
      <button onClick={showLoader}>Show</button>
      <button onClick={hideLoader}>Hide</button>
    </div>
  );
};

describe("LoaderContext", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("should provide loader state and functions", () => {
    render(
      <LoaderProvider>
        <TestComponent />
      </LoaderProvider>,
    );

    expect(screen.getByTestId("is-visible")).toHaveTextContent("false");

    // Show loader
    act(() => {
      screen.getByText("Show").click();
    });
    expect(screen.getByTestId("is-visible")).toHaveTextContent("true");

    // Hide loader (implementation uses 500ms delay before hiding)
    act(() => {
      screen.getByText("Hide").click();
    });
    act(() => {
      vi.advanceTimersByTime(500);
    });
    expect(screen.getByTestId("is-visible")).toHaveTextContent("false");
  });

  it("should handle concurrent requests correctly (ref counting)", () => {
    render(
      <LoaderProvider>
        <TestComponent />
      </LoaderProvider>,
    );

    const showBtn = screen.getByText("Show");
    const hideBtn = screen.getByText("Hide");
    const isVisibleSpan = screen.getByTestId("is-visible");

    // Initial state
    expect(isVisibleSpan).toHaveTextContent("false");

    // First request
    act(() => {
      showBtn.click();
    });
    expect(isVisibleSpan).toHaveTextContent("true");

    // Second request (simulating concurrent operation)
    act(() => {
      showBtn.click();
    });
    expect(isVisibleSpan).toHaveTextContent("true");

    // First completion
    act(() => {
      hideBtn.click();
    });
    expect(isVisibleSpan).toHaveTextContent("true"); // Should still be visible

    // Second completion (implementation uses 500ms delay before hiding)
    act(() => {
      hideBtn.click();
    });
    act(() => {
      vi.advanceTimersByTime(500);
    });
    expect(isVisibleSpan).toHaveTextContent("false"); // Should now be hidden
  });

  it("should throw error if used outside provider", () => {
    // Suppress console.error for this test as React logs the error
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    expect(() => render(<TestComponent />)).toThrow(
      "useLoader must be used within a LoaderProvider",
    );

    consoleSpy.mockRestore();
  });

  it("should render children", () => {
    render(
      <LoaderProvider>
        <div data-testid="child">Child Content</div>
      </LoaderProvider>,
    );

    expect(screen.getByTestId("child")).toBeInTheDocument();
  });
});
