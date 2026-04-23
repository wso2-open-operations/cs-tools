/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useContext } from "react";
import LoggerContext from "@context/logger/LoggerContext";
import LoggerProvider from "@context/logger/LoggerProvider";
import { type ILogger } from "@hooks/logger";

const TestConsumer = () => {
  const context = useContext(LoggerContext);
  return (
    <div data-testid="logger-value">
      {context ? "logger-available" : "logger-null"}
    </div>
  );
};

describe("LoggerProvider", () => {
  it("should render children correctly", () => {
    render(
      <LoggerProvider>
        <div data-testid="child">Child Content</div>
      </LoggerProvider>,
    );

    expect(screen.getByTestId("child")).toHaveTextContent("Child Content");
  });

  it("should provide a logger instance by default", () => {
    render(
      <LoggerProvider>
        <TestConsumer />
      </LoggerProvider>,
    );

    expect(screen.getByTestId("logger-value")).toHaveTextContent(
      "logger-available",
    );
  });

  it("should provide the custom logger when passed as a prop", () => {
    const mockLogger: ILogger = {
      debug: vi.fn(),
      info: vi.fn(),
      warn: vi.fn(),
      error: vi.fn(),
    };

    const CustomConsumer = () => {
      const logger = useContext(LoggerContext);
      return (
        <button onClick={() => logger?.info("test-message")}>Click Me</button>
      );
    };

    render(
      <LoggerProvider logger={mockLogger}>
        <CustomConsumer />
      </LoggerProvider>,
    );

    screen.getByRole("button").click();
    expect(mockLogger.info).toHaveBeenCalledWith("test-message");
  });
});
