// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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
import "@src/index.css";
import App from "@src/App";
import { AuthProvider } from "@asgardeo/auth-react";
import { ASGARDEO_BASE_URL, CLIENT_ID, SIGN_IN_REDIRECT_URL, SIGN_OUT_REDIRECT_URL } from "@config/config";
import { CssBaseline, ThemeProvider } from "@mui/material";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import theme from "@src/theme";

const authConfig = {
  clientID: CLIENT_ID || "",
  baseUrl: ASGARDEO_BASE_URL || "",
  signInRedirectURL: SIGN_IN_REDIRECT_URL || "",
  signOutRedirectURL: SIGN_OUT_REDIRECT_URL || "",
  scope: ["openid", "profile", "email"],
};

const queryClient = new QueryClient();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <AuthProvider config={authConfig}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <QueryClientProvider client={queryClient}>
          <App />
        </QueryClientProvider>
      </ThemeProvider>
    </AuthProvider>
  </StrictMode>,
);
