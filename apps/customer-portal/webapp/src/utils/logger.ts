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

// Log levels for the application.
export const LogLevel = {
  DEBUG: 0,
  INFO: 1,
  WARN: 2,
  ERROR: 3,
} as const;

export type LogLevel = (typeof LogLevel)[keyof typeof LogLevel];

// Interface for the Logger service.
export interface ILogger {
  debug(message: string, ...args: any[]): void;
  info(message: string, ...args: any[]): void;
  warn(message: string, ...args: any[]): void;
  error(message: string, ...args: any[]): void;
}

// Logger service that handles application-wide logging.
export class Logger implements ILogger {
  private level: LogLevel;
  private prefix: string;

  constructor(level: LogLevel = LogLevel.INFO, prefix: string = "App") {
    this.level = level;
    this.prefix = prefix;
  }

  // Formats the log message with a timestamp, level, and prefix.
  private formatMessage(level: string, message: string): string {
    const timestamp = new Date().toISOString();
    return `[${timestamp}] [${level}] [${this.prefix}] ${message}`;
  }

  // Core logging method that checks the current log level.
  private log(
    level: LogLevel,
    levelName: string,
    message: string,
    ...args: any[]
  ): void {
    if (level >= this.level) {
      const formattedMessage = this.formatMessage(levelName, message);

      switch (level) {
        case LogLevel.DEBUG:
          console.debug(formattedMessage, ...args);
          break;
        case LogLevel.INFO:
          console.info(formattedMessage, ...args);
          break;
        case LogLevel.WARN:
          console.warn(formattedMessage, ...args);
          break;
        case LogLevel.ERROR:
          console.error(formattedMessage, ...args);
          break;
        default:
          console.log(formattedMessage, ...args);
      }
    }
  }

  debug(message: string, ...args: any[]): void {
    this.log(LogLevel.DEBUG, "DEBUG", message, ...args);
  }

  info(message: string, ...args: any[]): void {
    this.log(LogLevel.INFO, "INFO", message, ...args);
  }

  warn(message: string, ...args: any[]): void {
    this.log(LogLevel.WARN, "WARN", message, ...args);
  }

  error(message: string, ...args: any[]): void {
    this.log(LogLevel.ERROR, "ERROR", message, ...args);
  }

  // Static method to get the log level from string.
  static parseLogLevel(levelString?: string): LogLevel {
    switch (levelString?.toUpperCase()) {
      case "DEBUG":
        return LogLevel.DEBUG;
      case "INFO":
        return LogLevel.INFO;
      case "WARN":
        return LogLevel.WARN;
      case "ERROR":
        return LogLevel.ERROR;
      default:
        return LogLevel.ERROR;
    }
  }
}
