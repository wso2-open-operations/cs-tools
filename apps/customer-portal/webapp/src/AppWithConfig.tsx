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

import type { JSX } from "react";
import { BrowserRouter } from "react-router";
import { OxygenUIThemeProvider } from "@wso2/oxygen-ui";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import App from "./App";
import { AsgardeoProvider } from "@asgardeo/react";
import { themeConfig } from "@config/themeConfig";
import { loggerConfig } from "@config/loggerConfig";
import LoggerProvider from "@context/logger/LoggerProvider";
import { authConfig } from "@config/authConfig";

/**
 * Custom retry function for React Query.
 * Only retries on 502 (Bad Gateway) and 503 (Service Unavailable) errors.
 *
 * @param {number} failureCount - The number of times the request has failed.
 * @param {Error} error - The error that occurred.
 * @returns {boolean} True if the request should be retried, false otherwise.
 */
function shouldRetry(failureCount: number, error: Error): boolean {
  // Max 3 retries
  if (failureCount >= 2) {
    return false;
  }

  // Check if error has a status code property
  const statusCode = (error as any)?.response?.status || (error as any)?.status;

  // Only retry on 502 (Bad Gateway) and 503 (Service Unavailable)
  return statusCode === 502 || statusCode === 503;
}

const queryClient: QueryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: shouldRetry,
      retryDelay: (attemptIndex) => Math.min(1000 * 2 ** attemptIndex, 30000),
    },
    mutations: {
      retry: shouldRetry,
      retryDelay: (attemptIndex) => Math.min(1000 * 2 ** attemptIndex, 30000),
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
      scopes={["openid", "email", "groups"]}
      periodicTokenRefresh={true}
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
              <App />
              <ReactQueryDevtools initialIsOpen={false} />
            </QueryClientProvider>
          </OxygenUIThemeProvider>
        </LoggerProvider>
      </BrowserRouter>
    </AsgardeoProvider>
  );
}
