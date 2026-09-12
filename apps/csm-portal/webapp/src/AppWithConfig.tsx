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

import { type JSX, lazy, Suspense } from "react";
import { BrowserRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import App from "./App";
import AppErrorBoundary from "@components/error/AppErrorBoundary";
import { AsgardeoProvider } from "@asgardeo/react";
import { loggerConfig } from "@config/loggerConfig";
import LoggerProvider from "@context/logger/LoggerProvider";
import { ThemePreferenceProvider } from "@context/theme/ThemePreferenceContext";
import { CaseTabsBehaviorProvider } from "@context/case-tabs/CaseTabsBehaviorContext";
import { authConfig } from "@config/authConfig";

// React-Query devtools ship from a devDependency and must not enter the
// production bundle. Dynamic import + DEV check accomplishes both.
const ReactQueryDevtools = import.meta.env.DEV
  ? lazy(() =>
      import("@tanstack/react-query-devtools").then((m) => ({
        default: m.ReactQueryDevtools,
      })),
    )
  : null;

/**
 * Custom retry function for React Query queries.
 * Only retries on 502 (Bad Gateway) and 503 (Service Unavailable) errors.
 */
function shouldRetryQuery(failureCount: number, error: Error): boolean {
  if (failureCount >= 2) return false;
  const errorWithStatus = error as Error & {
    response?: { status?: number };
    status?: number;
  };
  const statusCode = errorWithStatus.response?.status || errorWithStatus.status;
  return statusCode === 502 || statusCode === 503;
}

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

const queryClient: QueryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: shouldRetryQuery,
      retryDelay: (attemptIndex) => Math.min(1000 * 2 ** attemptIndex, 30000),
      refetchOnWindowFocus: false,
      refetchOnReconnect: false,
      refetchOnMount: true,
    },
    // Mutations default to no retry. Non-idempotent operations (PATCH/PUT/POST
    // with side effects) should opt in per-mutation when retries are safe.
    mutations: {
      retry: false,
    },
  },
});

export default function AppWithConfig(): JSX.Element {
  return (
    <AsgardeoProvider
      baseUrl={authConfig.baseUrl}
      clientId={authConfig.clientId}
      afterSignInUrl={authConfig.signInRedirectURL}
      afterSignOutUrl={authConfig.signOutRedirectURL}
      scopes={["openid", "email", "groups", "profile"]}
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
      {isInsideHiddenAuthIframe ? null : (
        <BrowserRouter>
          <LoggerProvider config={loggerConfig}>
            <ThemePreferenceProvider>
              <CaseTabsBehaviorProvider>
                <QueryClientProvider client={queryClient}>
                  <AppErrorBoundary>
                    <App />
                  </AppErrorBoundary>
                  {ReactQueryDevtools && (
                    <Suspense fallback={null}>
                      <ReactQueryDevtools initialIsOpen={false} />
                    </Suspense>
                  )}
                </QueryClientProvider>
              </CaseTabsBehaviorProvider>
            </ThemePreferenceProvider>
          </LoggerProvider>
        </BrowserRouter>
      )}
    </AsgardeoProvider>
  );
}
