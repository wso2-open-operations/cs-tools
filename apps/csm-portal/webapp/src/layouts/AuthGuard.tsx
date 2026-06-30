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

import { type JSX, useEffect } from "react";
import { useAsgardeo } from "@asgardeo/react";
import { ProtectedRoute } from "@asgardeo/react-router";
import { useLocation, useNavigate } from "react-router";
import AppLayout from "@layouts/AppLayout";
import { POST_LOGIN_REDIRECT_KEY } from "@layouts/postLoginRedirect";
import { CurrentUserProvider } from "@context/current-user/CurrentUserContext";

/**
 * AuthGuard renders AppLayout (header/footer) so loading state is visible
 * and the IdP authentication flow can be observed. Redirects to home only
 * when not signed in and auth check is complete.
 *
 * Preserves the intended URL across the IdP sign-in redirect so that
 * deep-links (e.g. ServiceNow case links) land on the correct page after auth.
 *
 * Note: the customer-portal behaviour of auto-redirecting `/` to the last
 * visited project's dashboard is intentionally NOT replicated here. CSM is
 * engineer-scoped, so the landing route `/` resolves to the ABT dashboard
 * via App.tsx instead.
 *
 * @returns {JSX.Element} AppLayout or redirect to home.
 */
export default function AuthGuard(): JSX.Element {
  const { isSignedIn } = useAsgardeo();
  const location = useLocation();
  const navigate = useNavigate();

  // After login, restore the saved deep link so it survives the Asgardeo SDK
  // reloading the page to `afterSignInUrl` ("/") after the callback (which would
  // otherwise drop us on the default landing). The key is consumed by
  // PostLoginRedirectConsumer once we arrive at the target — that consumer runs
  // above <Routes> so it also clears the key for routes AuthGuard never mounts
  // (e.g. the 404 page); clearing here would strand the key on a dead deep link
  // and bounce the next `/` visit back to it. The default `/` landing is
  // deferred to RootLanding in App.tsx while a redirect is pending.
  useEffect(() => {
    if (!isSignedIn) return;
    const redirect = sessionStorage.getItem(POST_LOGIN_REDIRECT_KEY);
    if (!redirect) return;
    // Compare (and restore) the full location including the hash, so anchor
    // permalinks like `/cases/:id#description` are honoured, not stripped.
    const here = location.pathname + location.search + location.hash;
    if (here !== redirect) {
      void navigate(redirect, { replace: true });
    }
  }, [isSignedIn, navigate, location.pathname, location.search, location.hash]);

  return (
    <ProtectedRoute
      loader={<AppLayout />}
      onSignIn={(defaultSignIn, signInOptions) => {
        const intended =
          location.pathname + location.search + location.hash;
        if (intended !== "/") {
          sessionStorage.setItem(POST_LOGIN_REDIRECT_KEY, intended);
        }
        defaultSignIn(signInOptions);
      }}
    >
      <CurrentUserProvider>
        <AppLayout />
      </CurrentUserProvider>
    </ProtectedRoute>
  );
}
