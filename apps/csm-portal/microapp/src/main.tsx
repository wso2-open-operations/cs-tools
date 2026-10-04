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
import { QueryClientProvider } from "@tanstack/react-query";
import { OxygenUIThemeProvider } from "@wso2/oxygen-ui";
import App from "@src/App";
import { AppErrorBoundary } from "@components/common/AppErrorBoundary";
import { clearSession } from "@src/services/auth";
import { queryClient } from "@src/services/queryClient";
import theme from "./theme";
import "@src/index.css";

// Entry point for the host app to end the session when the user signs out.
window.csmMicroApp = { clearSession };

const container = document.getElementById("root");
if (!container) throw new Error("Root container missing");

createRoot(container).render(
  <StrictMode>
    <OxygenUIThemeProvider theme={theme}>
      <QueryClientProvider client={queryClient}>
        <AppErrorBoundary>
          <App />
        </AppErrorBoundary>
      </QueryClientProvider>
    </OxygenUIThemeProvider>
  </StrictMode>,
);
