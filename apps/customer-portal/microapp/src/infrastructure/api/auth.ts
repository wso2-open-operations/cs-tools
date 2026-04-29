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
import { getAccessTokenFromBridge, getToken } from "@bridge/index";
import { LocalStorageKeys } from "@shared/constants/app.constants";
import { Logger } from "@infrastructure/logging/logger";

interface TokenPayload {
  email?: string;
  name?: string;
  groups?: string[];
  given_name: string;
  family_name: string;
}

export const getAccessToken = (): string | null => localStorage.getItem(LocalStorageKeys.accessToken);
export const setAccessToken = (token: string): void => localStorage.setItem(LocalStorageKeys.accessToken, token);
export const getIdToken = (): string | null => localStorage.getItem(LocalStorageKeys.idToken);
export const setIdToken = (token: string): void => localStorage.setItem(LocalStorageKeys.idToken, token);

export const refreshToken = (): Promise<string> => {
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
        decodeTokenAndStoreUser();
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

export interface DecodedUser {
  email: string;
  name: string;
}

export const decodeTokenAndStoreUser = (): DecodedUser | null => {
  try {
    const token = getIdToken();

    if (!token) {
      Logger.error("ID token not found for user decoding.");
      return null;
    }

    const decoded = jwtDecode<TokenPayload>(token);

    return {
      email: decoded.email || "",
      name: `${decoded.given_name || ""} ${decoded.family_name || ""}`,
    };
  } catch (error) {
    Logger.error("Failed to decode token and store user information", error);
    return null;
  }
};
