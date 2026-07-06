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

// Shared across every caller's hook instance. Each useAuthApiClient() call
// creates its own authFetch closure, so this lives at module scope to ensure
// only ONE full sign-in redirect is triggered even when many concurrent calls
// fail authentication at once.
let signInInFlight = false;

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
  const { getIdToken, signIn } = useAsgardeo();

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

  // Redirect to a full sign-in, single-flighted so concurrent auth failures
  // don't fire multiple redirects. Returns a never-resolving promise so callers
  // don't fall through to an error page while the browser navigates away.
  const redirectToSignIn = (): Promise<Response> => {
    if (!signInInFlight) {
      signInInFlight = true;
      // Best-effort: navigation takes over from here. Reset on the rare chance
      // the redirect itself rejects so a later attempt can retry.
      void Promise.resolve(signIn()).finally(() => {
        signInInFlight = false;
      });
    }
    return new Promise<Response>(() => {});
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

      // A concurrent caller, or the provider's periodic background refresh
      // (periodicTokenRefresh on AsgardeoProvider), may have re-minted the
      // token in the meantime, so retry once to pick it up. getIdToken() itself
      // only reads the stored token; if nothing refreshed it the retry fails
      // again and we fall through to the sign-in redirect below.
      try {
        return await attemptFetch(input, options);
      } catch (retryError) {
        // Retry failed for a non-auth reason (e.g. a transient network blip on
        // the second attempt): surface it to existing error handling instead of
        // bouncing the user to sign-in.
        if (!isTokenExpiredError(retryError)) {
          throw retryError;
        }

        // Still unauthenticated after the retry — the session is gone. Redirect
        // for a full sign-in.
        return redirectToSignIn();
      }
    }
  };

  return authFetch;
}
