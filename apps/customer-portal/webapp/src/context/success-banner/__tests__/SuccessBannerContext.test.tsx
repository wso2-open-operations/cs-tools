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
  SuccessBannerProvider,
  useSuccessBanner,
} from "@context/success-banner/SuccessBannerContext";

vi.mock("@wso2/oxygen-ui", () => ({
  Alert: ({ children, onClose, severity }: any) => (
    <div data-testid="success-banner" role="alert" data-severity={severity}>
      {children}
      <button data-testid="close-button" onClick={onClose}>
        Close
      </button>
    </div>
  ),
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
}));

const TestConsumer = () => {
  const { showSuccess } = useSuccessBanner();
  return (
    <div>
      <button onClick={() => showSuccess("Case created successfully")}>
        Trigger Success
      </button>
      <button onClick={() => showSuccess("Saved")}>
        Trigger Saved
      </button>
    </div>
  );
};

describe("SuccessBannerContext", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("should show banner when showSuccess is called", () => {
    render(
      <SuccessBannerProvider>
        <TestConsumer />
      </SuccessBannerProvider>,
    );

    expect(screen.queryByTestId("success-banner")).toBeNull();

    fireEvent.click(screen.getByText("Trigger Success"));

    expect(screen.getByTestId("success-banner")).toBeInTheDocument();
    expect(
      screen.getByText("Case created successfully"),
    ).toBeInTheDocument();
  });

  it("should show message passed to showSuccess", () => {
    render(
      <SuccessBannerProvider>
        <TestConsumer />
      </SuccessBannerProvider>,
    );

    fireEvent.click(screen.getByText("Trigger Saved"));

    expect(screen.getByText("Saved")).toBeInTheDocument();
  });

  it("should dismiss banner when close button is clicked", () => {
    render(
      <SuccessBannerProvider>
        <TestConsumer />
      </SuccessBannerProvider>,
    );

    fireEvent.click(screen.getByText("Trigger Success"));
    expect(screen.getByTestId("success-banner")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("close-button"));
    expect(screen.queryByTestId("success-banner")).toBeNull();
  });

  it("should auto-dismiss banner after timeout", () => {
    render(
      <SuccessBannerProvider>
        <TestConsumer />
      </SuccessBannerProvider>,
    );

    fireEvent.click(screen.getByText("Trigger Success"));
    expect(screen.getByTestId("success-banner")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(2900);
    });

    expect(screen.queryByTestId("success-banner")).toBeNull();
  });

  it("should throw if useSuccessBanner is used outside provider", () => {
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    expect(() => render(<TestConsumer />)).toThrow(
      "useSuccessBanner must be used within a SuccessBannerProvider",
    );

    consoleSpy.mockRestore();
  });
});
