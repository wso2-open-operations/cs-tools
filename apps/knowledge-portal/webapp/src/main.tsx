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

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { OxygenUIThemeProvider } from "@wso2/oxygen-ui";
import App from "./App";
import { shouldRetryQuery } from "@api/queryRetry";
import { getBackendBaseUrl } from "@config/apiConfig";
import { themeConfig } from "@config/themeConfig";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 60_000,
      retry: shouldRetryQuery,
      refetchOnWindowFocus: false,
    },
  },
});

const root = document.getElementById("root")!;

// Fail loudly, with a visible message, if the runtime config is missing or
// malformed. Without it every request would fail with an opaque network error.
function renderConfigError(message: string): void {
  const box = document.createElement("div");
  box.setAttribute("role", "alert");
  box.style.cssText = "font-family:sans-serif;max-width:40rem;margin:4rem auto;padding:0 1rem";
  const heading = document.createElement("h1");
  heading.textContent = "Knowledge base is not configured";
  const text = document.createElement("p");
  text.textContent = message;
  box.append(heading, text);
  root.replaceChildren(box);
}

try {
  getBackendBaseUrl();
  createRoot(root).render(
    <StrictMode>
      <OxygenUIThemeProvider theme={themeConfig}>
        <QueryClientProvider client={queryClient}>
          <BrowserRouter>
            <App />
          </BrowserRouter>
        </QueryClientProvider>
      </OxygenUIThemeProvider>
    </StrictMode>,
  );
} catch (error) {
  const message = error instanceof Error ? error.message : String(error);
  console.error(message);
  renderConfigError(message);
}
