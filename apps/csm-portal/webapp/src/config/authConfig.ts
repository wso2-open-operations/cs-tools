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

// Extend window interface to include our config
declare global {
  interface Window {
    config: {
      CSM_PORTAL_AUTH_BASE_URL: string;
      CSM_PORTAL_AUTH_CLIENT_ID: string;
      CSM_PORTAL_AUTH_SIGN_IN_REDIRECT_URL: string;
      CSM_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL: string;
      CSM_PORTAL_BACKEND_BASE_URL: string;
      CSM_PORTAL_THEME: string;
      CSM_PORTAL_LOG_LEVEL: string;
      CSM_PORTAL_DISABLE_WIP_FEATURES?: boolean;
      CSM_PORTAL_MAINTENANCE_BANNER_VISIBLE: boolean;
      CSM_PORTAL_MAINTENANCE_BANNER_SEVERITY?: string;
      CSM_PORTAL_MAINTENANCE_BANNER_TITLE?: string;
      CSM_PORTAL_MAINTENANCE_BANNER_MESSAGE?: string;
      CSM_PORTAL_MAINTENANCE_BANNER_ACTION_LABEL?: string;
      CSM_PORTAL_MAINTENANCE_BANNER_ACTION_URL?: string;
      CSM_PORTAL_CHATBOT_WEBSOCKET_URL?: string;
      CSM_PORTAL_FLOATING_NOVERA_ENABLED?: boolean;
      CSM_PORTAL_TOP_BANNER_ENABLED?: boolean;
      CSM_PORTAL_TOP_BANNER_HTML?: string;
      CSM_PORTAL_ANNOUNCEMENT_BANNER_VISIBLE?: boolean;
      CSM_PORTAL_ANNOUNCEMENT_BANNER_STORAGE_KEY?: string;
      CSM_PORTAL_ANNOUNCEMENT_BANNER_HTML?: string;
    };
  }
}

interface AuthConfig {
  baseUrl: string;
  clientId: string;
  signInRedirectURL: string;
  signOutRedirectURL: string;
}

const getAuthConfig = (): AuthConfig => {
  const config = window.config;
  const baseUrl = config?.CSM_PORTAL_AUTH_BASE_URL;
  const clientId = config?.CSM_PORTAL_AUTH_CLIENT_ID;
  const signInRedirectURL = config?.CSM_PORTAL_AUTH_SIGN_IN_REDIRECT_URL;
  const signOutRedirectURL = config?.CSM_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL;

  const missingVars: string[] = [];

  if (!baseUrl) missingVars.push("CSM_PORTAL_AUTH_BASE_URL");
  if (!clientId) missingVars.push("CSM_PORTAL_AUTH_CLIENT_ID");
  if (!signInRedirectURL)
    missingVars.push("CSM_PORTAL_AUTH_SIGN_IN_REDIRECT_URL");
  if (!signOutRedirectURL)
    missingVars.push("CSM_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL");

  if (missingVars.length > 0) {
    throw new Error(
      `Auth Config Error: Missing required configuration: ${missingVars.join(
        ", ",
      )}`,
    );
  }

  return {
    baseUrl,
    clientId,
    signInRedirectURL,
    signOutRedirectURL,
  };
};

export const authConfig = getAuthConfig();
