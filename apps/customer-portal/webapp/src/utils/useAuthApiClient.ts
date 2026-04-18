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

import {
  ASGARDEO_UNAUTHENTICATED_CODE,
  AUTH_NOT_READY_ERROR_MESSAGE,
  TOKEN_RETRY_DELAYS_MS,
} from "@constants/apiConstants";
import { useAsgardeo } from "@asgardeo/react";
import { useLogger } from "@hooks/useLogger";

// Waits for the provided duration.
function sleep(delayMs: number): Promise<void> {
  return new Promise((resolve) => {
    window.setTimeout(resolve, delayMs);
  });
}

// Checks whether an error indicates Asgardeo auth state is not ready yet.
function isAsgardeoUnauthenticatedError(error: unknown): boolean {
  if (!(error instanceof Error)) {
    return false;
  }

  const maybeCode = (error as { code?: unknown }).code;
  return maybeCode === ASGARDEO_UNAUTHENTICATED_CODE;
}

function isNonReplayableBody(body: unknown): boolean {
  if (body == null) {
    return false;
  }
  if (typeof ReadableStream !== "undefined" && body instanceof ReadableStream) {
    return true;
  }
  if (ArrayBuffer.isView(body)) {
    return true;
  }
  return false;
}

// A custom hook that automatically fetches a fresh ID Token from Asgardeo.
export function useAuthApiClient() {
  const { getIdToken } = useAsgardeo();
  const logger = useLogger();

  const resolveIdTokenWithRetry = async (): Promise<string> => {
    let lastError: unknown;
    const delays = TOKEN_RETRY_DELAYS_MS;
    for (let attempt = 0; attempt < delays.length; attempt += 1) {
      try {
        const token = await getIdToken();
        if (token) {
          return token;
        }
        logger.warn("[authFetch] token-unavailable", { attempt: attempt + 1 });
      } catch (error) {
        lastError = error;
        if (isAsgardeoUnauthenticatedError(error)) {
          logger.warn("[authFetch] token-unavailable", {
            attempt: attempt + 1,
            reason: "asgardeo-auth-not-ready",
          });
        } else {
          logger.warn("[authFetch] token-retrieval-error", {
            attempt: attempt + 1,
            error,
          });
        }
      }

      if (attempt < delays.length - 1) {
        await sleep(delays[attempt]);
      }
    }

    if (lastError) {
      if (isAsgardeoUnauthenticatedError(lastError)) {
        throw new Error(AUTH_NOT_READY_ERROR_MESSAGE);
      }
      throw lastError instanceof Error
        ? lastError
        : new Error("Unable to retrieve ID token");
    }
    throw new Error("Unable to retrieve ID token");
  };

  /**
   * Builds request headers with auth and payload defaults.
   *
   * @param {RequestInit | undefined} options - Request init options.
   * @param {string} token - ID token used as bearer and user token header.
   * @returns {Headers} Final headers for request execution.
   */
  const buildRequestHeaders = (
    options: RequestInit | undefined,
    token: string,
  ): Headers => {
    const headers = new Headers(options?.headers);
    headers.set("Authorization", `Bearer ${token}`);
    headers.set("x-user-id-token", token);
    if (!headers.has("Accept")) {
      headers.set("Accept", "application/json");
    }

    const method = options?.method?.toUpperCase() || "GET";
    const body = options?.body;
    if (["POST", "PUT", "PATCH"].includes(method) && body) {
      const isNonJsonType =
        body instanceof FormData ||
        body instanceof Blob ||
        body instanceof ArrayBuffer ||
        (typeof URLSearchParams !== "undefined" &&
          body instanceof URLSearchParams) ||
        (typeof ReadableStream !== "undefined" &&
          body instanceof ReadableStream) ||
        ArrayBuffer.isView(body);

      if (!isNonJsonType && !headers.has("Content-Type")) {
        headers.set("Content-Type", "application/json");
      }
    }

    return headers;
  };

  const authFetch = async (
    input: RequestInfo | URL,
    options?: RequestInit,
  ): Promise<Response> => {
    const token = await resolveIdTokenWithRetry();
    let response = await fetch(input, {
      ...options,
      headers: buildRequestHeaders(options, token),
    });

    const urlLabel =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.href
          : input.url;

    const body = options?.body;
    const canRetry401 =
      (typeof input === "string" || input instanceof URL) &&
      !isNonReplayableBody(body);

    if (response.status === 401 && canRetry401) {
      logger.warn("[authFetch] 401-retried", {
        url: urlLabel,
        method: options?.method ?? "GET",
      });
      const retryToken = await resolveIdTokenWithRetry();
      response = await fetch(input, {
        ...options,
        headers: buildRequestHeaders(options, retryToken),
      });
      if (response.status === 401) {
        logger.error("[authFetch] 401-final-failure", {
          url: urlLabel,
          method: options?.method ?? "GET",
        });
      }
    } else if (response.status === 401 && !canRetry401) {
      logger.warn("[authFetch] 401-skipped-retry", {
        url: urlLabel,
        method: options?.method ?? "GET",
        reason:
          typeof input !== "string" && !(input instanceof URL)
            ? "request-object-input"
            : "non-replayable-body",
      });
    }

    return response;
  };

  return authFetch;
}
