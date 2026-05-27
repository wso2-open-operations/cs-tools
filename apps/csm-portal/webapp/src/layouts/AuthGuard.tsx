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

const POST_LOGIN_REDIRECT_KEY = "post_login_redirect";

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

  useEffect(() => {
    if (isSignedIn) {
      const redirect = sessionStorage.getItem(POST_LOGIN_REDIRECT_KEY);
      if (redirect) {
        sessionStorage.removeItem(POST_LOGIN_REDIRECT_KEY);
        void navigate(redirect, { replace: true });
      }
    }
  }, [isSignedIn, navigate]);

  return (
    <ProtectedRoute
      loader={<AppLayout />}
      onSignIn={(defaultSignIn, signInOptions) => {
        const intended = location.pathname + location.search;
        if (intended !== "/") {
          sessionStorage.setItem(POST_LOGIN_REDIRECT_KEY, intended);
        }
        defaultSignIn(signInOptions);
      }}
    >
      <AppLayout />
    </ProtectedRoute>
  );
}
