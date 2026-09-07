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

import { windowConfig } from "./windowConfig";

interface AuthConfig {
  baseUrl: string;
  clientId: string;
  signInRedirectURL: string;
  signOutRedirectURL: string;
  scopes: string[];
}

// Resolved lazily (not at module load) so importing this module doesn't
// throw before window.config has been read — callers invoke getAuthConfig()
// from inside AppWithConfig's render, after config.js has loaded.
export function getAuthConfig(): AuthConfig {
  return {
    baseUrl: windowConfig.authBaseUrl(),
    clientId: windowConfig.authClientId(),
    signInRedirectURL: windowConfig.authSignInRedirectUrl(),
    signOutRedirectURL: windowConfig.authSignOutRedirectUrl(),
    scopes: windowConfig.authScopes().split(" ").filter(Boolean),
  };
}
