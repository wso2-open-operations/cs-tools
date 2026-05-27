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
  options: RequestInit | undefined,
  token: string,
): Headers {
  const headers = new Headers(options?.headers);
  headers.set("Authorization", `Bearer ${token}`);
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
      (typeof URLSearchParams !== "undefined" && body instanceof URLSearchParams) ||
      (typeof ReadableStream !== "undefined" && body instanceof ReadableStream) ||
      ArrayBuffer.isView(body);

    if (!isNonJsonType && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
  }

  return headers;
}

// Fetch wrapper that attaches a fresh IdP access token as the bearer. The
// Choreo gateway validates the access token and forwards it upstream as
// `x-jwt-assertion`, which csm-portal-backend reads in its auth middleware.
// The token is only attached when the request origin matches the configured
// backend; calls to any other origin go through bare fetch so credentials
// can't be leaked to third-party hosts.
export function useAuthApiClient() {
  const { getAccessToken } = useAsgardeo();

  return useCallback(
    async (input: RequestInfo | URL, options?: RequestInit): Promise<Response> => {
      const url = resolveRequestUrl(input);
      if (!trustedBackendOrigin || url.origin !== trustedBackendOrigin) {
        throw new Error(
          `Refusing to send access token to untrusted origin ${url.origin}`,
        );
      }

      const token = await getAccessToken();
      if (!token) {
        throw new Error("Unable to retrieve access token");
      }

      return fetch(input, {
        ...options,
        headers: buildRequestHeaders(options, token),
      });
    },
    [getAccessToken],
  );
}
