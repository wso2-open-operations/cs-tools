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

import { useCallback } from "react";
import { useAsgardeo } from "@asgardeo/react";
import { apiConfig } from "@config/apiConfig";
import { AUTH_NOT_READY_ERROR_MESSAGE } from "@constants/apiConstants";

/**
 * True when `getAccessToken()` failed because the Asgardeo SDK had not finished
 * initializing yet (code `SPA-AUTH_CLIENT-VM-NF01`, "The SDK must be
 * initialized first"). This is a transient race on first paint — the silent
 * refresh added in @asgardeo/react 0.25.5 can ask for a token a tick before the
 * SDK is ready — so callers should treat it as "auth not ready, retry", not a
 * hard error.
 */
function isSdkNotInitializedError(error: unknown): boolean {
  if (!(error instanceof Error)) return false;
  if ((error as { code?: string }).code === "SPA-AUTH_CLIENT-VM-NF01") {
    return true;
  }
  return /SDK (?:must be initialized|is not initialized)/i.test(
    `${error.name} ${error.message}`,
  );
}

// Origin we are willing to attach the bearer token to. Computed once at module
// load so we don't accidentally send credentials anywhere else.
const trustedBackendOrigin = (() => {
  try {
    return new URL(apiConfig.backendUrl).origin;
  } catch {
    return "";
  }
})();

function resolveRequestUrl(input: RequestInfo | URL): URL {
  if (input instanceof Request) return new URL(input.url, window.location.origin);
  if (input instanceof URL) return input;
  return new URL(input.toString(), window.location.origin);
}

function buildRequestHeaders(
  input: RequestInfo | URL,
  options: RequestInit | undefined,
  token: string,
  idToken: string,
): Headers {
  // When `input` is a Request, `init.headers` on the outer fetch call REPLACES
  // the request's headers wholesale — it does not merge. Seed the headers from
  // the Request and let any explicit option-level headers override.
  const headers =
    input instanceof Request ? new Headers(input.headers) : new Headers();
  if (options?.headers) {
    new Headers(options.headers).forEach((value, key) => {
      headers.set(key, value);
    });
  }

  headers.set("Authorization", `Bearer ${token}`);
  // The ID token travels alongside the access token (same convention as the
  // customer portal): the gateway validates the bearer, while the backend
  // reads the user's identity claims from `x-user-id-token`.
  headers.set("x-user-id-token", idToken);
  if (!headers.has("Accept")) {
    headers.set("Accept", "application/json");
  }

  // Inherit method/body from the Request when callers omit them in `options`.
  const method =
    options?.method?.toUpperCase() ||
    (input instanceof Request ? input.method.toUpperCase() : "GET");
  const body =
    options?.body ?? (input instanceof Request ? input.body : undefined);

  if (["POST", "PUT", "PATCH"].includes(method) && body) {
    const isNonJsonType =
      body instanceof FormData ||
      body instanceof Blob ||
      body instanceof ArrayBuffer ||
      (typeof URLSearchParams !== "undefined" && body instanceof URLSearchParams) ||
      (typeof ReadableStream !== "undefined" && body instanceof ReadableStream) ||
      ArrayBuffer.isView(body);

    if (!isNonJsonType && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
  }

  return headers;
}

// Fetch wrapper that attaches a fresh IdP access token as the bearer and the
// ID token as `x-user-id-token` (the customer portal's convention). The
// Choreo gateway validates the access token and forwards it upstream as
// `x-jwt-assertion`, which csm-portal-backend reads in its auth middleware;
// `x-user-id-token` passes through to the backend untouched.
// The tokens are only attached when the request origin matches the configured
// backend; calls to any other origin are refused so credentials can't be
// leaked to third-party hosts.
export function useAuthApiClient() {
  const { getAccessToken, getIdToken } = useAsgardeo();

  return useCallback(
    async (input: RequestInfo | URL, options?: RequestInit): Promise<Response> => {
      const url = resolveRequestUrl(input);
      if (!trustedBackendOrigin || url.origin !== trustedBackendOrigin) {
        throw new Error(
          `Refusing to send access token to untrusted origin ${url.origin}`,
        );
      }

      let token: string | undefined;
      let idToken: string | undefined;
      try {
        [token, idToken] = await Promise.all([
          getAccessToken(),
          getIdToken(),
        ]);
      } catch (error) {
        // Normalise the SDK-not-initialized race into the shared "auth not
        // ready" signal so callers warn-and-retry instead of surfacing a raw
        // AsgardeoAuthException as a hard error.
        if (isSdkNotInitializedError(error)) {
          throw new Error(AUTH_NOT_READY_ERROR_MESSAGE);
        }
        throw error;
      }
      if (!token) {
        throw new Error("Unable to retrieve access token");
      }
      if (!idToken) {
        throw new Error("Unable to retrieve ID token");
      }

      return fetch(input, {
        ...options,
        headers: buildRequestHeaders(input, options, token, idToken),
      });
    },
    [getAccessToken, getIdToken],
  );
}
