/**
 * PLG's API client, bound to csm-portal's authenticated session.
 *
 * WHAT THE MERGE CHANGED. Standalone, this was a module-level `fetch` that put
 * the acting engineer's email in an `X-PLG-User` header — a header the browser
 * chose and the backend believed. That is gone. Requests now go through
 * csm-portal's `useAuthApiClient`, which attaches the real Asgardeo access
 * token as a bearer and the ID token alongside it; the backend derives the
 * caller from the validated token and PLG's resolver turns that into a
 * `"user".id`. There is no header for a browser to set.
 *
 * That is why this is a hook rather than a module-level object: the token is
 * session state, and a singleton bound to it at import time would be bound to
 * whatever session happened to exist first.
 *
 * The shape is otherwise unchanged — `get`, `post`, `patch`, `put`, `del` —
 * so the 23 hooks in hooks.ts read exactly as they did.
 */
import { useCallback, useMemo } from "react";

import { useAuthApiClient } from "@hooks/useAuthApiClient";
import { BACKEND_BASE_URL } from "@config/apiConfig";

/** Thrown for any non-2xx response, carrying the status the caller may branch on. */
export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

interface ErrorBody {
  code?: number;
  message?: string;
}

/**
 * Every PLG route lives under /plg on the backend.
 *
 * The prefix exists because PLG's handlers share a mux with csm-portal's 116
 * own routes, and `/products` means the platform catalogue to one and a
 * ServiceNow product to the other. Applied here rather than in each of the 23
 * call sites, so the hooks still read as the portal's own vocabulary.
 */
const API_PREFIX = "/plg";

export interface PlgApi {
  get<T>(path: string): Promise<T>;
  post<T>(path: string, body?: unknown): Promise<T>;
  patch<T>(path: string, body?: unknown): Promise<T>;
  put<T>(path: string, body?: unknown): Promise<T>;
  del<T>(path: string): Promise<T>;
}

/** Returns a PLG API client bound to the current auth session. */
export function usePlgApi(): PlgApi {
  const authFetch = useAuthApiClient();

  const request = useCallback(
    async <T,>(method: string, path: string, body?: unknown): Promise<T> => {
      const response = await authFetch(`${BACKEND_BASE_URL}${API_PREFIX}${path}`, {
        method,
        headers: { "Content-Type": "application/json" },
        body: body === undefined ? undefined : JSON.stringify(body),
      });

      if (response.status === 204) {
        return undefined as T;
      }

      const text = await response.text();

      // The body is not guaranteed to be JSON, and the status is the thing worth
      // keeping. A gateway that times out or fails in front of this service
      // answers with HTML, not with the portal's error shape — parsing that
      // first would throw a SyntaxError before the status was ever read, so the
      // caller would see a parse error instead of the 502 that caused it. Every
      // hook in this feature branches on ApiError.status, and a SyntaxError
      // satisfies none of them.
      let parsed: unknown = null;
      let unreadable = false;
      if (text) {
        try {
          parsed = JSON.parse(text);
        } catch {
          unreadable = true;
        }
      }

      if (!response.ok) {
        const message =
          (parsed as ErrorBody | null)?.message ?? `Request failed (${response.status})`;
        throw new ApiError(response.status, message);
      }

      // A 2xx whose body will not parse is still a failed request from the
      // caller's point of view: returning it would hand a component `undefined`
      // where it expects data, and fail somewhere further away instead.
      if (unreadable) {
        throw new ApiError(response.status, "The server sent a response this page could not read.");
      }
      return parsed as T;
    },
    [authFetch],
  );

  return useMemo<PlgApi>(
    () => ({
      get: <T,>(path: string) => request<T>("GET", path),
      post: <T,>(path: string, body?: unknown) => request<T>("POST", path, body),
      patch: <T,>(path: string, body?: unknown) => request<T>("PATCH", path, body),
      put: <T,>(path: string, body?: unknown) => request<T>("PUT", path, body),
      del: <T,>(path: string) => request<T>("DELETE", path),
    }),
    [request],
  );
}
