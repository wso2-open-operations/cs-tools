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
import { getToken } from "@components/microapp-bridge";
import { LocalStorageKeys } from "@utils/constants";
import { Logger } from "@utils/logger";
import { useUserStore, type User } from "../store/user";

// Token Payload
interface TokenPayload {
  email?: string;
  name?: string;
  groups?: string[];
  given_name: string;
  family_name: string;
}

// These would be your actual token storage functions
export const getAccessToken = (): string | null => localStorage.getItem(LocalStorageKeys.accessToken);
export const setAccessToken = (token: string): void => localStorage.setItem(LocalStorageKeys.accessToken, token);
export const getIdToken = (): string | null => localStorage.getItem(LocalStorageKeys.idToken);
export const setIdToken = (token: string): void => localStorage.setItem(LocalStorageKeys.idToken, token);

/**
 * A function to refresh the token.
 * This is a simplified version of the logic in the original `handleRequestWithNewToken`.
 * It fetches a new token and updates it in storage.
 */
export const refreshToken = (): Promise<string> => {
  return new Promise((resolve, reject) => {
    getToken((newIdToken: string | undefined) => {
      if (newIdToken) {
        setIdToken(newIdToken);
        setAccessToken(newIdToken);

        // Automatically decode and store user information when token is refreshed
        try {
          initializeUserFromToken();
          Logger.info("User information updated after token refresh");
        } catch (error) {
          Logger.warn("Failed to update user information after token refresh", error);
        }

        resolve(newIdToken);
      } else {
        Logger.error("Failed to refresh token");
        reject("Failed to refresh token");
      }
    });
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

  const decoded = jwtDecode<TokenPayload>(token);
  const userGroups = decoded.groups ?? [];
  const requiredGroups = Array.isArray(groupNames) ? groupNames : [groupNames];

  return requiredGroups.some((group) => userGroups.includes(group));
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
