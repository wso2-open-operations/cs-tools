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

import { type JSX } from "react";
import { AsgardeoProvider } from "@asgardeo/react";
import { authConfig } from "@config/authConfig";
import { AUTH_SCOPES } from "@config/tokenExchangeConfig";
import AppTree from "./AppTree";

// `signInSilently()` (from `@asgardeo/react`) recovers an expired access
// token by loading the IdP's authorize URL, with prompt=none, inside a
// hidden, invisible iframe. Because this app's own SPA is served at the
// same redirect_uri as the top-level app, that hidden iframe's document is
// a full second load of THIS SAME bundle — `AsgardeoProvider` needs to
// mount and initialize there to complete the SDK's internal handshake
// (it detects the silent-sign-in state in the URL and short-circuits), but
// nothing below it should. Without this guard, the router/`AuthGuard`/page
// tree fully mounts inside that hidden iframe too, and `AuthGuard`'s own
// (correct, needed) recovery logic then notices "not signed in yet" INSIDE
// that iframe and starts its own silent sign-in — recursively nesting
// another hidden iframe inside this one. Observed live: a single token
// expiry cascaded into 7 nested iframe loads and dropped an in-flight POST
// (a case comment was never created) somewhere in that churn. This app is
// never legitimately embedded by anything else, so `window.self !==
// window.top` unambiguously means "I am the SDK's own hidden recovery
// iframe," not a real embedding scenario to support.
const isInsideHiddenAuthIframe =
  typeof window !== "undefined" && window.self !== window.top;

export default function AppWithConfig(): JSX.Element {
  return (
    <AsgardeoProvider
      baseUrl={authConfig.baseUrl}
      clientId={authConfig.clientId}
      afterSignInUrl={authConfig.signInRedirectURL}
      afterSignOutUrl={authConfig.signOutRedirectURL}
      scopes={[...AUTH_SCOPES]}
      preferences={{
        theme: {
          inheritFromBranding: false,
        },
        user: {
          fetchUserProfile: false,
          fetchOrganizations: false,
        },
      }}
    >
      {isInsideHiddenAuthIframe ? null : <AppTree />}
    </AsgardeoProvider>
  );
}
