// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
//
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

import { BaseURLAuthClientConfig } from '@asgardeo/auth-react';

/** Keys read from `public/config.js` as `window.config`. */
declare global {
  interface Window {
    /** Runtime configuration injected before the React bundle loads. */
    config: {
      /** Application title shown in the browser tab. */
      APP_NAME: string;
      /** Asgardeo organization URL (e.g. `https://api.asgardeo.io/t/{org}`). */
      ASGARDEO_BASE_URL: string;
      /** OIDC client ID registered in Asgardeo for this SPA. */
      ASGARDEO_CLIENT_ID: string;
      /** Post-login redirect URI (must match Asgardeo app registration; typically site root). */
      AUTH_SIGN_IN_REDIRECT_URL: string;
      /** Post-logout redirect URI. */
      AUTH_SIGN_OUT_REDIRECT_URL: string;
      /** Origin of the Ballerina backend (e.g. `http://localhost:9090`). */
      BACKEND_BASE_URL: string;
    };
  }
}

/** Asgardeo Auth React provider configuration built from `window.config`. */
export const AsgardeoConfig: BaseURLAuthClientConfig = {
  signInRedirectURL: window.config?.AUTH_SIGN_IN_REDIRECT_URL ?? '',
  signOutRedirectURL: window.config?.AUTH_SIGN_OUT_REDIRECT_URL ?? '',
  clientID: window.config?.ASGARDEO_CLIENT_ID ?? '',
  baseUrl: window.config?.ASGARDEO_BASE_URL ?? '',
  // Match webapps/webapp-template: `email` and `groups` must appear on the ID token for the Ballerina `CustomJwtPayload`.
  scope: ['openid', 'email', 'groups'],
};

const serviceBaseUrl = window.config?.BACKEND_BASE_URL ?? '';

/** Default app title if `APP_NAME` is not set in `config.js`. */
export const APP_NAME = window.config?.APP_NAME ?? 'Security Advisory Patches Portal';

/** Base URL for `GET /files/{id}` (share path as one encoded segment). */
export const AppConfig = {
  downloadFilesBaseUrl: `${serviceBaseUrl}/files`,
};
