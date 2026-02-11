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

import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  ErrorBannerProvider,
  useErrorBanner,
} from "@context/error-banner/ErrorBannerContext";

vi.mock("@wso2/oxygen-ui", () => ({
  Alert: ({ children, onClose, severity }: any) => (
    <div data-testid="error-banner" role="alert" data-severity={severity}>
      {children}
      <button data-testid="close-button" onClick={onClose}>
        Close
      </button>
    </div>
  ),
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
}));

vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  X: () => <span data-testid="x-icon" />,
}));

const TestConsumer = () => {
  const { showError } = useErrorBanner();
  return (
    <div>
      <button onClick={() => showError("dashboard statistics")}>
        Trigger Error
      </button>
      <button onClick={() => showError("projects")}>
        Trigger Projects Error
      </button>
    </div>
  );
};

describe("ErrorBannerContext", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("should show banner when showError is called", () => {
    render(
      <ErrorBannerProvider>
        <TestConsumer />
      </ErrorBannerProvider>,
    );

    expect(screen.queryByTestId("error-banner")).toBeNull();

    fireEvent.click(screen.getByText("Trigger Error"));

    expect(screen.getByTestId("error-banner")).toBeInTheDocument();
    expect(
      screen.getByText("Error loading dashboard statistics"),
    ).toBeInTheDocument();
  });

  it("should show api name passed to showError", () => {
    render(
      <ErrorBannerProvider>
        <TestConsumer />
      </ErrorBannerProvider>,
    );

    fireEvent.click(screen.getByText("Trigger Projects Error"));

    expect(screen.getByText("Error loading projects")).toBeInTheDocument();
  });

  it("should dismiss banner when close button is clicked", () => {
    render(
      <ErrorBannerProvider>
        <TestConsumer />
      </ErrorBannerProvider>,
    );

    fireEvent.click(screen.getByText("Trigger Error"));
    expect(screen.getByTestId("error-banner")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("close-button"));
    expect(screen.queryByTestId("error-banner")).toBeNull();
  });

  it("should auto-dismiss banner after timeout", () => {
    render(
      <ErrorBannerProvider>
        <TestConsumer />
      </ErrorBannerProvider>,
    );

    fireEvent.click(screen.getByText("Trigger Error"));
    expect(screen.getByTestId("error-banner")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(2900);
    });

    expect(screen.queryByTestId("error-banner")).toBeNull();
  });

  it("should throw if useErrorBanner is used outside provider", () => {
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    expect(() => render(<TestConsumer />)).toThrow(
      "useErrorBanner must be used within an ErrorBannerProvider",
    );

    consoleSpy.mockRestore();
  });
});
