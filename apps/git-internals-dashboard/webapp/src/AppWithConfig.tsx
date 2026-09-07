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
import { BrowserRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "@mui/material/styles";
import CssBaseline from "@mui/material/CssBaseline";
import { AsgardeoProvider } from "@asgardeo/react";
import App from "./App";
import { getAuthConfig } from "@config/authConfig";
import { theme } from "@theme/theme";
import "@theme/global.css";

// `signInSilently()` (from `@asgardeo/react`) recovers an expired access
// token by loading the IdP's authorize URL, with prompt=none, inside a
// hidden, invisible iframe. Because this app's own SPA is served at the
// same redirect_uri as the top-level app, that hidden iframe's document is
// a full second load of THIS SAME bundle — AsgardeoProvider needs to mount
// and initialize there to complete the SDK's internal handshake (it detects
// the silent-sign-in state in the URL and short-circuits), but nothing
// below it should: mounting the router/AuthGuard tree inside that hidden
// iframe too would have it notice "not signed in yet" and start its own
// nested silent sign-in, recursively. This app is never legitimately
// embedded by anything else, so `window.self !== window.top`
// unambiguously means "I am the SDK's own hidden recovery iframe," not a
// real embedding scenario to support. Ported from csm-portal's
// AppWithConfig.tsx, which hit this live (a single token expiry cascaded
// into 7 nested iframe loads).
const isInsideHiddenAuthIframe = typeof window !== "undefined" && window.self !== window.top;

/**
 * Retries only on 502 (Bad Gateway) and 503 (Service Unavailable) — matches
 * csm-portal's shouldRetryQuery (SPEC §11).
 */
function shouldRetryQuery(failureCount: number, error: Error): boolean {
  if (failureCount >= 2) return false;
  const status = (error as Error & { status?: number }).status;
  return status === 502 || status === 503;
}

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: shouldRetryQuery,
      refetchOnWindowFocus: false,
    },
    mutations: {
      retry: false,
    },
  },
});

export default function AppWithConfig(): JSX.Element {
  const authConfig = getAuthConfig();

  return (
    <AsgardeoProvider
      baseUrl={authConfig.baseUrl}
      clientId={authConfig.clientId}
      afterSignInUrl={authConfig.signInRedirectURL}
      afterSignOutUrl={authConfig.signOutRedirectURL}
      scopes={authConfig.scopes}
      preferences={{
        // AsgardeoProvider otherwise calls Asgardeo's own `/scim2/Me` and
        // `/api/users/v1/me/organizations` on every session to populate
        // `user`/`userProfile`/`myOrganizations` — endpoints gated behind the
        // `internal_login` scope, which this app doesn't request (D7: no
        // entitlement gate, any signed-in org user is authorized). Without
        // it those calls 403 forever. This app never reads that state (no
        // CurrentUserProvider/"/users/me" check — see AuthGuard.tsx), so
        // there's nothing to lose by not fetching it. Matches csm-portal's
        // AppWithConfig.tsx, which disables the same two flags for the same
        // reason.
        user: {
          fetchUserProfile: false,
          fetchOrganizations: false,
        },
      }}
    >
      {isInsideHiddenAuthIframe ? null : (
        <BrowserRouter>
          <ThemeProvider theme={theme}>
            <CssBaseline />
            <QueryClientProvider client={queryClient}>
              <App />
            </QueryClientProvider>
          </ThemeProvider>
        </BrowserRouter>
      )}
    </AsgardeoProvider>
  );
}
