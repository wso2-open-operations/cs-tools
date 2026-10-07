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
  /**
   * The request's correlation ID, for a support "Tracking ID" copy affordance
   * (see `@utils/correlationId`'s `getErrorReferenceId`). Prefer the value
   * echoed back in the response's `X-CSM-Correlation-ID` header; fall back to
   * the one this client generated and sent, which the backend logged under
   * regardless of whether the response header made it back across CORS.
   */
  public readonly correlationId?: string;
  /**
   * The stable machine-readable name of the refusal (the error body's
   * `errorCode`, e.g. `change_request_on_hold`), when the backend named it.
   * This is what a caller may branch on: `message` is wording for people and
   * can change. Absent when the backend sent none (an older one, or a refusal
   * that has no name): treat that as "no more specific than the status".
   */
  public readonly code?: string;

  constructor(
    status: number,
    statusText: string,
    message?: string,
    correlationId?: string,
    code?: string,
  ) {
    super(message ?? `${status} ${statusText}`);
    this.name = "ApiError";
    this.status = status;
    this.statusText = statusText;
    this.correlationId = correlationId;
    this.code = code;
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

/** Shape of a machine-readable error code: lower-case words joined by underscores. */
const ERROR_CODE_PATTERN = /^[a-z][a-z0-9]*(_[a-z0-9]+)*$/;

/**
 * Extracts the machine-readable name of a refusal from an HTTP error response
 * body: `body.errorCode` when the body is JSON and that is a plain lower-case
 * name (`change_request_on_hold`), else `undefined`. Anything else (no body, not
 * JSON, no `errorCode`, a value of another type or shape) is "no code", so an
 * older backend and a refusal without a name are handled alike.
 *
 * @param text - Raw response body string (may be JSON or plain text).
 * @returns The error code, or undefined when the body names none.
 */
export function parseApiResponseErrorCode(text: string): string | undefined {
  if (!text) return undefined;
  try {
    const parsed: unknown = JSON.parse(text);
    if (parsed !== null && typeof parsed === "object" && "errorCode" in parsed) {
      const code = (parsed as { errorCode: unknown }).errorCode;
      if (
        typeof code === "string" &&
        code.length <= 64 &&
        ERROR_CODE_PATTERN.test(code)
      ) {
        return code;
      }
    }
  } catch {
    // not JSON: no code
  }
  return undefined;
}
