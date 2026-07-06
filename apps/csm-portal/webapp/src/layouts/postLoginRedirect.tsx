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

import { useEffect } from "react";
import { useAsgardeo } from "@asgardeo/react";
import { useLocation } from "react-router";

/**
 * sessionStorage key holding the deep link to restore after the IdP sign-in
 * round-trip. Written at module load (main.tsx) and on `onSignIn` (AuthGuard);
 * consumed by {@link PostLoginRedirectConsumer} once we arrive at the target.
 */
export const POST_LOGIN_REDIRECT_KEY = "post_login_redirect";

/**
 * Clears {@link POST_LOGIN_REDIRECT_KEY} as soon as we have arrived at the
 * stored target, for ANY route — including ones outside the AuthGuard route
 * group (the 401/403/404 pages and the `*` catch-all).
 *
 * This must live above `<Routes>` rather than inside AuthGuard: AuthGuard only
 * mounts for app routes, so a deep link to a non-existent path used to land on
 * the `*` 404 page with AuthGuard never mounting, stranding the key. The stale
 * key then hijacked the next visit to `/`, bouncing the user back to the dead
 * URL on a loop until they typed a valid one.
 *
 * Clearing is gated on `isSignedIn`: when a logged-out user deep-links to a
 * valid page, `here === redirect` is already true on first paint, but the key
 * must survive until the login round-trip completes — so we only consume it
 * once authenticated.
 */
export function PostLoginRedirectConsumer(): null {
  const { isSignedIn } = useAsgardeo();
  const location = useLocation();

  useEffect(() => {
    if (!isSignedIn) return;
    const redirect = sessionStorage.getItem(POST_LOGIN_REDIRECT_KEY);
    if (!redirect) return;
    const here = location.pathname + location.search + location.hash;
    if (here === redirect) {
      sessionStorage.removeItem(POST_LOGIN_REDIRECT_KEY);
    }
  }, [isSignedIn, location.pathname, location.search, location.hash]);

  return null;
}
