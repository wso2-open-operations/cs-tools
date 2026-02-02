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

import { type BaseURLAuthClientConfig } from "@asgardeo/auth-react";

// Configuration for the Auth service.
const authBaseUrl = import.meta.env.CUSTOMER_PORTAL_AUTH_BASE_URL;
const authClientId = import.meta.env.CUSTOMER_PORTAL_AUTH_CLIENT_ID;
const signInRedirectURL = import.meta.env
  .CUSTOMER_PORTAL_AUTH_SIGN_IN_REDIRECT_URL;
const signOutRedirectURL = import.meta.env
  .CUSTOMER_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL;

if (
  !authBaseUrl ||
  !authClientId ||
  !signInRedirectURL ||
  !signOutRedirectURL
) {
  throw new Error(
    "Missing required auth env variables: baseUrl, clientID, signInRedirectURL, or signOutRedirectURL.",
  );
}

export const authConfig: BaseURLAuthClientConfig = {
  scope: ["openid", "email", "groups"],
  baseUrl: authBaseUrl,
  clientID: authClientId,
  signInRedirectURL,
  signOutRedirectURL,
};
