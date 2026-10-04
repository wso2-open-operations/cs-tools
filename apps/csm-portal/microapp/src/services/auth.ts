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

import { jwtDecode } from "jwt-decode";

import { getAccessTokenFromBridge, getToken } from "@components/microapp-bridge";
import { Logger } from "@utils/logger";
import { useUserStore, type User } from "../store/user";
import { queryClient } from "./queryClient";

// Token Payload
interface TokenPayload {
  email?: string;
  name?: string;
  given_name?: string;
  family_name?: string;
  // Either OIDC claim may carry the profile picture URL depending on the IdP — same fallback the
  // webapp uses (apps/csm-portal/webapp/src/utils/userClaims.ts).
  picture?: string;
  profile?: string;
}

// Tokens are held in memory only: the host hands them over on request, so they are never
// persisted to web storage and a new WebView session always starts with none.
let accessToken: string | null = null;
let idToken: string | null = null;

export const getAccessToken = (): string | null => accessToken;
export const setAccessToken = (token: string): void => {
  accessToken = token;
};
export const getIdToken = (): string | null => idToken;
export const setIdToken = (token: string): void => {
  idToken = token;
};

// Earlier builds cached tokens in web storage under per-environment keys. Remove any such
// leftovers once at start-up so they do not outlive the upgrade.
const LEGACY_TOKEN_KEY = /^csm_portal_(accessToken|idToken)(_|$)|^(accessToken|idToken)$/;
try {
  Object.keys(localStorage)
    .filter((key) => LEGACY_TOKEN_KEY.test(key))
    .forEach((key) => localStorage.removeItem(key));
} catch {
  // Storage may be unavailable in some WebView configurations; nothing to clean up then.
}

/**
 * Ends the local session: drops the in-memory tokens, the signed-in user and every cached query.
 * The host calls this (via `window.csmMicroApp.clearSession()`) when the user signs out.
 */
export const clearSession = (): void => {
  accessToken = null;
  idToken = null;
  inFlightRefresh = null;
  sessionGeneration += 1;
  useUserStore.getState().clearUser();
  queryClient.clear();
};

// Refresh early rather than right at expiry, to cover in-flight request latency.
const TOKEN_EXPIRY_BUFFER_MS = 60_000;
function isTokenExpiringSoon(token: string | null): boolean {
  if (!token) return true;
  try {
    const { exp } = jwtDecode<{ exp?: number }>(token);
    if (!exp) return true;
    return exp * 1000 - Date.now() < TOKEN_EXPIRY_BUFFER_MS;
  } catch {
    return true;
  }
}
// Every refresh — the launch-time call in App, the request interceptor and the activity stream
// hook — goes through this single in-flight promise, so overlapping callers share one bridge
// round trip instead of each re-posting to the host.
let inFlightRefresh: Promise<string> | null = null;
// Bumped by clearSession so a refresh that was in flight at sign-out cannot store its tokens.
let sessionGeneration = 0;

const requestTokensFromHost = (): Promise<string> => {
  const generation = sessionGeneration;

  return Promise.all([getToken(), getAccessTokenFromBridge()])
    .then(([newIdToken, newAccessToken]) => {
      if (!newIdToken) throw new Error("ID Token failed");
      if (!newAccessToken) throw new Error("Access Token failed");
      if (generation !== sessionGeneration) throw new Error("Session ended during token refresh");

      setIdToken(newIdToken);
      setAccessToken(newAccessToken);

      try {
        initializeUserFromToken();
        Logger.info("User information updated after full token refresh");
      } catch (error) {
        Logger.warn("Failed to update user information", error);
      }

      return newIdToken;
    })
    .catch((error) => {
      Logger.error("Failed to refresh tokens", error);
      throw error;
    });
};

export const refreshToken = (force = false): Promise<string> => {
  const currentIdToken = getIdToken();
  if (!force && !isTokenExpiringSoon(currentIdToken)) {
    if (!useUserStore.getState().user) {
      decodeTokenAndStoreUser();
    }
    return Promise.resolve(currentIdToken as string);
  }

  if (!inFlightRefresh) {
    const refresh = requestTokensFromHost().finally(() => {
      if (inFlightRefresh === refresh) inFlightRefresh = null;
    });
    inFlightRefresh = refresh;
  }
  return inFlightRefresh;
};

/**
 * Decodes the ID token and extracts user information.
 * Stores the user information in the Zustand user store.
 * @returns User object or null if token is invalid
 */
export const decodeTokenAndStoreUser = (): User | null => {
  try {
    const token = getIdToken();

    if (!token) {
      Logger.error("ID token not found for user decoding.");
      useUserStore.getState().clearUser();
      return null;
    }

    const decoded = jwtDecode<TokenPayload>(token);

    // Extract user information from token
    const user: User = {
      email: decoded.email || "",
      name: `${decoded.given_name || ""} ${decoded.family_name || ""}`,
      avatarUrl: decoded.picture || decoded.profile,
    };

    useUserStore.getState().setUser(user);
    return user;
  } catch (error) {
    Logger.error("Failed to decode token and store user information", error);
    useUserStore.getState().clearUser();
    return null;
  }
};

/**
 * Initializes user data from stored token.
 * Should be called when the app starts or when a new token is received.
 */
export const initializeUserFromToken = (): void => {
  useUserStore.getState().setLoading(true);

  try {
    decodeTokenAndStoreUser();
  } catch (error) {
    Logger.error("Failed to initialize user from token", error);
    useUserStore.getState().clearUser();
  } finally {
    useUserStore.getState().setLoading(false);
  }
};
