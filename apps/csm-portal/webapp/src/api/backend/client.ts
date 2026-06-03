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

import { useCallback, useMemo } from "react";
import { useAuthApiClient } from "@hooks/useAuthApiClient";
import type { BeErrorPayload } from "@api/backend/types";

/**
 * `true` when the mock toggle is on. Hooks should branch on this and call
 * the backend client only in LIVE mode.
 */
export function isMockMode(): boolean {
  return !!window.config?.CSM_PORTAL_USE_MOCKS;
}

export function backendBaseUrl(): string {
  const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
  if (!baseUrl) {
    throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
  }
  return baseUrl;
}

export class BackendApiError extends Error {
  status: number;
  payload?: BeErrorPayload;
  constructor(status: number, message: string, payload?: BeErrorPayload) {
    super(message);
    this.name = "BackendApiError";
    this.status = status;
    this.payload = payload;
  }
}

async function readError(response: Response): Promise<BackendApiError> {
  let payload: BeErrorPayload | undefined;
  try {
    payload = (await response.json()) as BeErrorPayload;
  } catch {
    /* body not JSON */
  }
  const msg =
    payload?.message ??
    response.statusText ??
    `Request failed with status ${response.status}`;
  return new BackendApiError(response.status, msg, payload);
}

/**
 * Shape of the backend client returned by {@link useBackendApi}. One thin wrapper per
 * HTTP verb that pairs the configured backend base URL with the auth-aware
 * fetch and JSON (de)serialization. 404s on GET resolve to `null` so the
 * common "not found" case can be handled without exceptions.
 */
export interface BackendApi {
  get<T>(path: string): Promise<T | null>;
  post<TRequest, TResponse>(path: string, body: TRequest): Promise<TResponse>;
  patch<TRequest, TResponse>(path: string, body: TRequest): Promise<TResponse>;
  postEmpty<TResponse>(path: string): Promise<TResponse>;
}

/**
 * Returns a backend API client bound to the current auth session. Use inside a
 * hook's `queryFn` / `mutationFn`. Errors are thrown as {@link BackendApiError}
 * carrying the HTTP status and optional error payload.
 */
export function useBackendApi(): BackendApi {
  const authFetch = useAuthApiClient();

  const buildUrl = useCallback((path: string): string => {
    if (path.startsWith("http")) return path;
    const base = backendBaseUrl();
    const sep = path.startsWith("/") ? "" : "/";
    return `${base}${sep}${path}`;
  }, []);

  return useMemo<BackendApi>(
    () => ({
      async get<T>(path: string): Promise<T | null> {
        const response = await authFetch(buildUrl(path), { method: "GET" });
        if (response.status === 404) return null;
        if (!response.ok) throw await readError(response);
        if (response.status === 204) return null;
        return (await response.json()) as T;
      },
      async post<TRequest, TResponse>(
        path: string,
        body: TRequest,
      ): Promise<TResponse> {
        const response = await authFetch(buildUrl(path), {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        });
        if (!response.ok) throw await readError(response);
        return (await response.json()) as TResponse;
      },
      async patch<TRequest, TResponse>(
        path: string,
        body: TRequest,
      ): Promise<TResponse> {
        const response = await authFetch(buildUrl(path), {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        });
        if (!response.ok) throw await readError(response);
        return (await response.json()) as TResponse;
      },
      async postEmpty<TResponse>(path: string): Promise<TResponse> {
        const response = await authFetch(buildUrl(path), {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: "{}",
        });
        if (!response.ok) throw await readError(response);
        return (await response.json()) as TResponse;
      },
    }),
    [authFetch, buildUrl],
  );
}
