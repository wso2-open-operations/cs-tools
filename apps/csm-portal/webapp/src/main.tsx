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

import Prism from "prismjs";
import React from "react";
import { createRoot } from "react-dom/client";
import AppWithConfig from "./AppWithConfig";
import { POST_LOGIN_REDIRECT_KEY } from "@layouts/postLoginRedirect";

if (typeof window !== "undefined") {
  (window as unknown as { Prism: typeof Prism }).Prism = Prism;
}

// Capture the entry deep link at module load — before React, routing, or the
// Asgardeo SDK run — so it can be restored after the IdP round-trip even when
// the SDK restores the session synchronously (in which case ProtectedRoute's
// `onSignIn` never fires and the SDK navigates straight to `afterSignInUrl`).
// AuthGuard reads/clears `post_login_redirect` once signed in. Skip the bare
// root and the OAuth callback (which carries `?code=&state=`).
if (typeof window !== "undefined") {
  try {
    // Include the hash so permalinks like `/cases/:id#description` (per-comment /
    // per-section anchors) survive the IdP round-trip, not just pathname+search.
    const entry =
      window.location.pathname + window.location.search + window.location.hash;
    // Only the OAuth callback carries BOTH `code` and `state`; requiring both
    // avoids misclassifying legitimate deep links that happen to use a business
    // `state` (or `code`) query param and dropping their post-login restore.
    const params = new URLSearchParams(window.location.search);
    const isOauthCallback = params.has("code") && params.has("state");
    if (entry !== "/" && !isOauthCallback) {
      window.sessionStorage.setItem(POST_LOGIN_REDIRECT_KEY, entry);
    }
  } catch {
    /* sessionStorage may be unavailable; deep-link restore is best-effort. */
  }
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <AppWithConfig />
  </React.StrictMode>,
);
