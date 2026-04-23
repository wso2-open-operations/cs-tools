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
import { useLocation, useNavigate } from "react-router";
import AppLayout from "@layouts/AppLayout";
import { getLastSelectedProjectId } from "@features/settings/utils/settingsStorage";

const POST_LOGIN_REDIRECT_KEY = "post_login_redirect";

/**
 * AuthGuard renders AppLayout (header/footer) so loading state is visible
 * and Asgardeo authentication flow can be observed. Redirects to home only
 * when not signed in and auth check is complete.
 *
 * Preserves the intended URL across the Asgardeo sign-in redirect so that
 * deep-links (e.g. ServiceNow case links) land on the correct page after auth.
 *
 * @returns {JSX.Element} AppLayout or redirect to home.
 */
export default function AuthGuard(): JSX.Element {
  const { isSignedIn, isLoading: isAuthLoading, signIn } = useAsgardeo();
  const location = useLocation();
  const navigate = useNavigate();

  useEffect(() => {
    if (!isSignedIn && !isAuthLoading) {
      const intended = location.pathname + location.search;
      if (intended !== "/" && intended !== "/home") {
        sessionStorage.setItem(POST_LOGIN_REDIRECT_KEY, intended);
      }
      void signIn();
    }
  }, [isSignedIn, isAuthLoading, signIn, location]);

  useEffect(() => {
    if (isSignedIn) {
      const redirect = sessionStorage.getItem(POST_LOGIN_REDIRECT_KEY);
      if (redirect) {
        sessionStorage.removeItem(POST_LOGIN_REDIRECT_KEY);
        void navigate(redirect, { replace: true });
        return;
      }
      const lastProjectId = getLastSelectedProjectId();
      const fromHeader = (location.state as { fromHeader?: boolean })?.fromHeader;
      if (
        lastProjectId &&
        location.pathname === "/" &&
        !fromHeader
      ) {
        void navigate(`/projects/${lastProjectId}/dashboard`, {
          replace: true,
        });
      }
    }
  }, [isSignedIn, navigate, location.pathname]);

  return <AppLayout />;
}
