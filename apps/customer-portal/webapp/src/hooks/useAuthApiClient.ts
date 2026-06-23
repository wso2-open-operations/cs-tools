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

import { useAsgardeo } from "@asgardeo/react";
import { ASGARDEO_UNAUTHENTICATED_CODE } from "@constants/apiConstants";

// Max time to wait for a silent token refresh before giving up. signInSilently()
// drives a hidden auth iframe; if third-party cookies are blocked it can hang
// indefinitely, so we cap it and fall back to a full sign-in redirect.
const SILENT_SIGN_IN_TIMEOUT_MS = 10000;

// Shared across every caller's hook instance. Each useAuthApiClient() call
// creates its own authFetch closure, so the in-flight refresh promise must live
// at module scope to single-flight concurrent refreshes (one signInSilently()
// instead of N, and no duplicate sign-in redirects).
let refreshInFlight: Promise<boolean> | null = null;

// Only the Asgardeo "unauthenticated" code means the token was expired/missing
// when the call ran. Everything else (network failures, real backend 5xx) must
// propagate untouched so existing error handling and error pages still work.
const isTokenExpiredError = (error: unknown): boolean =>
  error != null &&
  typeof error === "object" &&
  "code" in error &&
  (error as { code: string }).code === ASGARDEO_UNAUTHENTICATED_CODE;

// A custom hook that automatically fetches a fresh ID Token from Asgardeo.
export function useAuthApiClient() {
  const { getIdToken, signInSilently, signIn } = useAsgardeo();

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

  const attemptFetch = async (
    input: RequestInfo | URL,
    options?: RequestInit,
  ): Promise<Response> => {
    const token = await getIdToken();
    if (!token) {
      throw new Error("Unable to retrieve ID token");
    }
    return fetch(input, {
      ...options,
      headers: buildRequestHeaders(options, token),
    });
  };

  // Re-mint tokens off the still-live IdP session via the hidden auth iframe.
  // Single-flighted through the module-scoped promise so concurrent callers
  // share one refresh. Resolves false on failure or timeout so the caller can
  // fall back to a full redirect rather than hang.
  const ensureFreshToken = (): Promise<boolean> => {
    if (!refreshInFlight) {
      refreshInFlight = new Promise<boolean>((resolve) => {
        const timeout = setTimeout(
          () => resolve(false),
          SILENT_SIGN_IN_TIMEOUT_MS,
        );
        signInSilently()
          .then((result) => Boolean(result))
          .catch(() => false)
          .then((refreshed) => {
            clearTimeout(timeout);
            resolve(refreshed);
          });
      }).finally(() => {
        refreshInFlight = null;
      });
    }
    return refreshInFlight;
  };

  const authFetch = async (
    input: RequestInfo | URL,
    options?: RequestInit,
  ): Promise<Response> => {
    try {
      return await attemptFetch(input, options);
    } catch (error) {
      // Only an expired/missing token is recoverable here; anything else
      // (network, real backend 5xx) must surface to existing error handling.
      if (!isTokenExpiredError(error)) {
        throw error;
      }

      // The session was dead. Try to silently re-mint tokens off the live IdP
      // session and replay the original request once.
      const refreshed = await ensureFreshToken();
      if (refreshed) {
        return attemptFetch(input, options);
      }

      // The IdP session is gone too — redirect for a full sign-in. Return a
      // never-resolving promise so callers don't fall through to an error page
      // while the browser navigates away.
      await signIn();
      return new Promise<Response>(() => {});
    }
  };

  return authFetch;
}
