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
import { LocalStorageKeys } from "@utils/constants";
import { Logger } from "@utils/logger";
import { useUserStore, type User } from "../store/user";

// Token Payload
interface TokenPayload {
  email?: string;
  name?: string;
  groups?: string[];
  given_name?: string;
  family_name?: string;
}

// These would be your actual token storage functions
export const getAccessToken = (): string | null => localStorage.getItem(LocalStorageKeys.accessToken);
export const setAccessToken = (token: string): void => localStorage.setItem(LocalStorageKeys.accessToken, token);
export const getIdToken = (): string | null => localStorage.getItem(LocalStorageKeys.idToken);
export const setIdToken = (token: string): void => localStorage.setItem(LocalStorageKeys.idToken, token);

// Refresh early rather than right at expiry, to cover in-flight request latency.
const TOKEN_EXPIRY_BUFFER_MS = 60_000;

// apiClient calls refreshToken() on every request; without this check that means a native-bridge
// round trip (plus a full user-store re-init) per request even when the current token is still
// valid for a while. Treat undecodable/expired-soon tokens as needing refresh, same as a missing
// token, so this fails toward refreshing rather than toward silently reusing a stale token.
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

/**
 * A function to refresh the token.
 * This is a simplified version of the logic in the original `handleRequestWithNewToken`.
 * Skips the native-bridge round trip when the current ID token is still valid for a while,
 * and only fetches a new token and updates storage when it's missing or near-expiry.
 * @param force - Bypasses the expiry check, e.g. after a 401 proves the current token is
 * already rejected by the backend, regardless of what its own `exp` claim says.
 */
export const refreshToken = (force = false): Promise<string> => {
  const currentIdToken = getIdToken();
  if (!force && !isTokenExpiringSoon(currentIdToken)) {
    return Promise.resolve(currentIdToken as string);
  }

  const idTokenPromise = new Promise<string>((resolve, reject) => {
    getToken((token) => (token ? resolve(token) : reject("ID Token failed")));
  });

  const accessTokenPromise = new Promise<string>((resolve, reject) => {
    getAccessTokenFromBridge((token) => (token ? resolve(token) : reject("Access Token failed")));
  });

  return Promise.all([idTokenPromise, accessTokenPromise])
    .then(([newIdToken, newAccessToken]) => {
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

/**
 * Checks if the user belongs to a given group or groups based on the ID token.
 * This is a direct replacement for `handleCheckGroups`.
 * @param groupNames - A single group name or an array of group names.
 * @returns boolean - True if the user is in at least one of the required groups.
 */
export const checkUserGroups = (groupNames: string | string[]): boolean => {
  const token = getIdToken();

  if (!token) {
    Logger.error("ID token not found for group check.");
    return false;
  }

  try {
    const decoded = jwtDecode<TokenPayload>(token);
    const userGroups = decoded.groups ?? [];
    const requiredGroups = Array.isArray(groupNames) ? groupNames : [groupNames];

    return requiredGroups.some((group) => userGroups.includes(group));
  } catch (error) {
    Logger.error("Failed to decode ID token for group check.", error);
    return false;
  }
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
