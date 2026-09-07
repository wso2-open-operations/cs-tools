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

// Typed accessor for window.config (SPEC §10), loaded by index.html's
// <script src="/config.js"> before this bundle runs. csm-portal pattern:
// public/config.js is per-environment and gitignored — public/config.js.example
// is the committed template.
declare global {
  interface Window {
    config: {
      GID_AUTH_BASE_URL: string;
      GID_AUTH_CLIENT_ID: string;
      GID_AUTH_SIGN_IN_REDIRECT_URL: string;
      GID_AUTH_SIGN_OUT_REDIRECT_URL: string;
      GID_AUTH_SCOPES: string;
      GID_BACKEND_BASE_URL: string;
      GID_LOG_LEVEL: string;
    };
  }
}

function required(key: keyof Window["config"]): string {
  const value = window.config?.[key];
  if (!value) {
    throw new Error(
      `Missing required runtime configuration: ${key}. Did you copy public/config.js.example to public/config.js?`,
    );
  }
  return value;
}

export const windowConfig = {
  authBaseUrl: (): string => required("GID_AUTH_BASE_URL"),
  authClientId: (): string => required("GID_AUTH_CLIENT_ID"),
  authSignInRedirectUrl: (): string => required("GID_AUTH_SIGN_IN_REDIRECT_URL"),
  authSignOutRedirectUrl: (): string => required("GID_AUTH_SIGN_OUT_REDIRECT_URL"),
  authScopes: (): string => window.config?.GID_AUTH_SCOPES || "openid profile",
  backendBaseUrl: (): string => required("GID_BACKEND_BASE_URL"),
  logLevel: (): string => window.config?.GID_LOG_LEVEL || "ERROR",
};
