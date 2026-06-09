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
import { OxygenUIThemeProvider } from "@wso2/oxygen-ui";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import App from "./App";
import { AsgardeoProvider } from "@asgardeo/react";
import { themeConfig } from "@config/themeConfig";
import { loggerConfig } from "@config/loggerConfig";
import LoggerProvider from "@context/logger/LoggerProvider";
import { MockModeProvider } from "@context/mock-mode/MockModeContext";
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
      <BrowserRouter>
        <LoggerProvider config={loggerConfig}>
          <OxygenUIThemeProvider theme={themeConfig}>
            <QueryClientProvider client={queryClient}>
              <MockModeProvider>
                <App />
              </MockModeProvider>
              {ReactQueryDevtools && (
                <Suspense fallback={null}>
                  <ReactQueryDevtools initialIsOpen={false} />
                </Suspense>
              )}
            </QueryClientProvider>
          </OxygenUIThemeProvider>
        </LoggerProvider>
      </BrowserRouter>
    </AsgardeoProvider>
  );
}
