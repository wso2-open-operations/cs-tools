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
import { CORRELATION_ID_HEADER } from "@utils/correlationId";

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
  /**
   * Correlation ID for this failed request, read back from the response's
   * `X-CSM-Correlation-ID` header (the backend echoes the ID it logged against).
   * Surface it to users as a support "Reference ID". `undefined` when the
   * gateway does not expose the header on cross-origin responses (it must be
   * listed in `Access-Control-Expose-Headers`); the FE access log still records
   * the ID regardless.
   */
  correlationId?: string;
  constructor(
    status: number,
    message: string,
    payload?: BeErrorPayload,
    correlationId?: string,
  ) {
    super(message);
    this.name = "BackendApiError";
    this.status = status;
    this.payload = payload;
    this.correlationId = correlationId;
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
  const correlationId =
    response.headers.get(CORRELATION_ID_HEADER) ?? undefined;
  return new BackendApiError(response.status, msg, payload, correlationId);
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
  /** Authenticated DELETE. Returns the parsed JSON body (`null` on 204). */
  del<TResponse>(path: string): Promise<TResponse | null>;
  /**
   * Authenticated binary GET. Used for endpoints that stream raw file content
   * (e.g. attachment download) rather than JSON. Unlike {@link BackendApi.get},
   * a 404 throws like any other error — there is no "null" body for a binary.
   */
  getBlob(path: string): Promise<Blob>;
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
      async del<TResponse>(path: string): Promise<TResponse | null> {
        const response = await authFetch(buildUrl(path), { method: "DELETE" });
        if (!response.ok) throw await readError(response);
        if (response.status === 204) return null;
        return (await response.json()) as TResponse;
      },
      async getBlob(path: string): Promise<Blob> {
        const response = await authFetch(buildUrl(path), { method: "GET" });
        if (!response.ok) throw await readError(response);
        return await response.blob();
      },
    }),
    [authFetch, buildUrl],
  );
}
