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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import type { UserDetails } from "@features/settings/types/users";
import { useLogger } from "@hooks/useLogger";
import { AUTH_NOT_READY_ERROR_MESSAGE } from "@constants/apiConstants";
import { setUserPreferredTimeZone } from "@utils/dateTime";
import { ApiError } from "@utils/ApiError";

/**
 * Pull OIDC claims out of the session token that the auth SDK persists in
 * sessionStorage. Falls back to the Asgardeo context's `user` field, then to
 * an awaited `getDecodedIdToken()`. Used only in mock mode (no backend exists
 * to call). Returns an empty object if no token is available yet.
 */
async function readSessionClaims(
  asgardeo: ReturnType<typeof useAsgardeo>,
): Promise<Record<string, unknown>> {
  // 1. Look for the id_token in sessionStorage — the SDK stores it there.
  try {
    if (typeof sessionStorage !== "undefined") {
      const key = Object.keys(sessionStorage).find((k) =>
        k.startsWith("session_data-"),
      );
      if (key) {
        const raw = sessionStorage.getItem(key);
        if (raw) {
          const parsed = JSON.parse(raw) as { id_token?: string };
          if (parsed.id_token) {
            const claims = decodeJwtPayload(parsed.id_token);
            if (claims) return claims;
          }
        }
      }
    }
  } catch {
    // ignore — try next source
  }
  // 2. The context's `user` field may already hold parsed claims.
  if (asgardeo.user && typeof asgardeo.user === "object") {
    return asgardeo.user as Record<string, unknown>;
  }
  // 3. Last resort: ask the SDK to decode. May reject if it isn't ready.
  try {
    const decoded = await asgardeo.getDecodedIdToken();
    if (decoded && typeof decoded === "object") {
      return decoded as Record<string, unknown>;
    }
  } catch {
    // ignore
  }
  return {};
}

function decodeJwtPayload(jwt: string): Record<string, unknown> | null {
  const parts = jwt.split(".");
  if (parts.length < 2) return null;
  try {
    const padded = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const json = atob(padded + "===".slice((padded.length + 3) % 4));
    return JSON.parse(json) as Record<string, unknown>;
  } catch {
    return null;
  }
}

async function deriveUserDetailsFromAuth(
  asgardeo: ReturnType<typeof useAsgardeo>,
): Promise<UserDetails> {
  const claims = await readSessionClaims(asgardeo);

  const pick = (...keys: string[]): string | undefined => {
    for (const k of keys) {
      const v = claims[k];
      if (typeof v === "string" && v.length > 0) return v;
    }
    return undefined;
  };

  const email = pick("email", "preferred_username", "username", "sub") ?? "";
  const firstName =
    pick("given_name", "firstName", "first_name") ??
    (email ? email.split("@")[0]?.split(/[._-]/)[0] ?? "" : "");
  const lastName =
    pick("family_name", "lastName", "last_name") ??
    (email ? email.split("@")[0]?.split(/[._-]/).slice(1).join(" ") ?? "" : "");
  const id = pick("sub", "id", "user_id") ?? email;

  return {
    id,
    email,
    firstName: capitalize(firstName),
    lastName: capitalize(lastName),
    timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
  };
}

function capitalize(s: string): string {
  if (!s) return s;
  return s.charAt(0).toUpperCase() + s.slice(1);
}

/**
 * Hook to get user details.
 *
 * @returns {UseQueryResult<UserDetails, Error>} The user details.
 */
const useGetUserDetails = (): UseQueryResult<UserDetails, Error> => {
  const authFetch = useAuthApiClient();
  const logger = useLogger();
  const asgardeo = useAsgardeo();

  return useQuery({
    queryKey: ["userDetails"],
    queryFn: async (): Promise<UserDetails> => {
      logger.debug("[useGetUserDetails] Fetching user details...");

      // Mock mode: no backend exists. Derive identity from the authenticated
      // session's claims so the header shows the real signed-in user.
      if (window.config?.CSM_PORTAL_USE_MOCKS) {
        const details = await deriveUserDetailsFromAuth(asgardeo);
        setUserPreferredTimeZone(details.timeZone);
        return details;
      }

      try {
        const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
        }
        const requestUrl = `${baseUrl}/users/me`;

        logger.debug(`[useGetUserDetails] URL: ${requestUrl}`);

        const response = await authFetch(requestUrl, {
          method: "GET",
          cache: "no-store",
        });

        logger.debug(`[useGetUserDetails] Response status: ${response.status}`);

        if (!response.ok) {
          let apiMessage: string | undefined;
          try {
            const errBody = await response.json();
            if (typeof errBody?.message === "string") {
              apiMessage = errBody.message;
            }
          } catch {
            // ignore – body may not be JSON
          }
          throw new ApiError(
            response.status,
            response.statusText,
            apiMessage ??
              `Error fetching user details: ${response.status} ${response.statusText}`,
          );
        }

        const data = (await response.json()) as Record<string, unknown>;
        logger.debug("[useGetUserDetails] Data received:", data);
        const tzRaw = data.timeZone ?? data.timezone ?? data.time_zone;
        const timeZone =
          typeof tzRaw === "string"
            ? tzRaw
            : tzRaw != null
              ? String(tzRaw)
              : "";
        setUserPreferredTimeZone(timeZone);
        return { ...data, timeZone } as UserDetails;
      } catch (error) {
        if (
          error instanceof Error &&
          error.message.includes(AUTH_NOT_READY_ERROR_MESSAGE)
        ) {
          logger.warn("[useGetUserDetails] Auth not ready yet; will retry.");
        } else {
          logger.error("[useGetUserDetails] Error:", error);
        }
        throw error;
      }
    },
    retry: (failureCount, error) => {
      if (failureCount >= 3) {
        return false;
      }
      if (
        error instanceof Error &&
        error.message.includes(AUTH_NOT_READY_ERROR_MESSAGE)
      ) {
        return true;
      }
      const status = (error as Error & { status?: number }).status;
      if (typeof status === "number") {
        if (status >= 500) {
          return true;
        }
        if (status >= 400 && status < 500) {
          return false;
        }
      }
      if (error instanceof TypeError) {
        return true;
      }
      return false;
    },
    retryDelay: (attemptIndex) => Math.min(400 * 2 ** attemptIndex, 3000),
    staleTime: 0,
  });
};

export default useGetUserDetails;
