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

/**
 * Error thrown by API hooks when the backend returns a non-OK response.
 * Carries the HTTP status so callers can branch on it (e.g. render a
 * 403 page instead of a toast).
 */
export class ApiError extends Error {
  public readonly status: number;
  public readonly statusText: string;

  constructor(status: number, statusText: string, message?: string) {
    super(message ?? `${status} ${statusText}`);
    this.name = "ApiError";
    this.status = status;
    this.statusText = statusText;
  }
}

export function isForbiddenError(error: unknown): boolean {
  return error instanceof ApiError && error.status === 403;
}

export function isUnauthorizedError(error: unknown): boolean {
  return error instanceof ApiError && error.status === 401;
}

export function isNotFoundError(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404;
}

export function isBadRequestError(error: unknown): boolean {
  return error instanceof ApiError && error.status === 400;
}

/**
 * Extracts the human-readable message from an ApiError (parsed from the
 * API response body, e.g. `{"message":"..."}`).  Returns undefined when the
 * error is not an ApiError or carries no specific message.
 */
export function getApiErrorMessage(error: unknown): string | undefined {
  if (error instanceof ApiError) {
    const defaultMsg = `${error.status} ${error.statusText}`;
    return error.message !== defaultMsg ? error.message : undefined;
  }
  return undefined;
}

/** @deprecated Use getApiErrorMessage instead. */
export const getForbiddenMessage = getApiErrorMessage;

/**
 * Extracts a clean human-readable message from an HTTP error response body.
 * Tries to parse the body as JSON and returns `body.message` when present.
 * Falls back to `statusText`, then `HTTP {status}`.
 *
 * @param text - Raw response body string (may be JSON or plain text).
 * @param status - HTTP status code.
 * @param statusText - HTTP status text.
 * @returns Human-readable error message.
 */
export function parseApiResponseMessage(
  text: string,
  status: number,
  statusText: string,
): string {
  if (text) {
    try {
      const parsed: unknown = JSON.parse(text);
      if (
        parsed !== null &&
        typeof parsed === "object" &&
        "message" in parsed &&
        typeof (parsed as { message: unknown }).message === "string" &&
        (parsed as { message: string }).message.trim()
      ) {
        return (parsed as { message: string }).message.trim();
      }
    } catch {
      // not JSON — fall through
    }
  }
  return statusText?.trim() || `HTTP ${status}`;
}
