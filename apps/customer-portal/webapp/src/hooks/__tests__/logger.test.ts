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

import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { Logger, LogLevel } from "@hooks/logger";

describe("Logger", () => {
  let consoleDebugSpy: any;
  let consoleInfoSpy: any;
  let consoleWarnSpy: any;
  let consoleErrorSpy: any;

  beforeEach(() => {
    consoleDebugSpy = vi.spyOn(console, "debug").mockImplementation(() => {});
    consoleInfoSpy = vi.spyOn(console, "info").mockImplementation(() => {});
    consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-01-31T12:00:00Z"));
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  describe("LogLevel filtering", () => {
    it("should log DEBUG and above when level is DEBUG", () => {
      const logger = new Logger(LogLevel.DEBUG, "Test");
      logger.debug("debug message");
      logger.info("info message");
      logger.warn("warn message");
      logger.error("error message");

      expect(consoleDebugSpy).toHaveBeenCalled();
      expect(consoleInfoSpy).toHaveBeenCalled();
      expect(consoleWarnSpy).toHaveBeenCalled();
      expect(consoleErrorSpy).toHaveBeenCalled();
    });

    it("should log INFO and above when level is INFO", () => {
      const logger = new Logger(LogLevel.INFO, "Test");
      logger.debug("debug message");
      logger.info("info message");

      expect(consoleDebugSpy).not.toHaveBeenCalled();
      expect(consoleInfoSpy).toHaveBeenCalled();
    });

    it("should log WARN and above when level is WARN", () => {
      const logger = new Logger(LogLevel.WARN, "Test");
      logger.info("info message");
      logger.warn("warn message");

      expect(consoleInfoSpy).not.toHaveBeenCalled();
      expect(consoleWarnSpy).toHaveBeenCalled();
    });

    it("should log only ERROR when level is ERROR", () => {
      const logger = new Logger(LogLevel.ERROR, "Test");
      logger.warn("warn message");
      logger.error("error message");

      expect(consoleWarnSpy).not.toHaveBeenCalled();
      expect(consoleErrorSpy).toHaveBeenCalled();
    });
  });

  describe("Message formatting", () => {
    it("should format the message correctly with timestamp, level and prefix", () => {
      const logger = new Logger(LogLevel.DEBUG, "TestPrefix");
      logger.info("test message");

      const expectedMessage =
        "[2026-01-31T12:00:00.000Z] [INFO] [TestPrefix] test message";
      expect(consoleInfoSpy).toHaveBeenCalledWith(expectedMessage);
    });

    it("should pass additional arguments to console methods", () => {
      const logger = new Logger(LogLevel.DEBUG, "Test");
      const extraArg = { data: 123 };
      logger.debug("message", extraArg);

      expect(consoleDebugSpy).toHaveBeenCalledWith(
        expect.any(String),
        extraArg,
      );
    });
  });

  describe("parseLogLevel", () => {
    it("should parse log levels correctly", () => {
      expect(Logger.parseLogLevel("DEBUG")).toBe(LogLevel.DEBUG);
      expect(Logger.parseLogLevel("info")).toBe(LogLevel.INFO);
      expect(Logger.parseLogLevel("Warn")).toBe(LogLevel.WARN);
      expect(Logger.parseLogLevel("ERROR")).toBe(LogLevel.ERROR);
    });

    it("should return ERROR for invalid or undefined levels", () => {
      expect(Logger.parseLogLevel("INVALID")).toBe(LogLevel.ERROR);
      expect(Logger.parseLogLevel(undefined)).toBe(LogLevel.ERROR);
      expect(Logger.parseLogLevel("")).toBe(LogLevel.ERROR);
    });
  });
});
