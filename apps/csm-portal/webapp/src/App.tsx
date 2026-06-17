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

import { type JSX, lazy } from "react";
import { Navigate, Route, Routes } from "react-router";
import AuthGuard from "@layouts/AuthGuard";
import {
  POST_LOGIN_REDIRECT_KEY,
  PostLoginRedirectConsumer,
} from "@layouts/postLoginRedirect";
import ErrorLayout from "@layouts/ErrorLayout";
import CsmComingSoonPage from "@features/csm-coming-soon/pages/CsmComingSoonPage";
import Error401Page from "@components/error/Error401Page";
import Error403Page from "@components/error/Error403Page";
import Error404Page from "@components/error/Error404Page";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
import { SuccessBannerProvider } from "@context/success-banner/SuccessBannerContext";
import { LoaderProvider } from "@context/linear-loader/LoaderContext";
import { ErrorPageProvider } from "@context/error-page/ErrorPageContext";

/*
 * Authenticated feature pages are lazily loaded so each lands in its own chunk
 * and is fetched only when its route is visited, instead of being bundled into
 * the initial entry chunk. They all render inside AppLayout's Outlet, which
 * owns the Suspense boundary that covers the load. Error pages and the shared
 * CsmComingSoonPage stay eager: they are tiny and act as immediate fallbacks.
 */
const CsmDashboardPage = lazy(
  () => import("@features/csm-dashboard/pages/CsmDashboardPage"),
);
const CsmCasesPage = lazy(
  () => import("@features/csm-cases/pages/CsmCasesPage"),
);
const CsmCaseCreatePage = lazy(
  () => import("@features/csm-cases/pages/CsmCaseCreatePage"),
);
const CsmCaseDetailPage = lazy(
  () => import("@features/csm-cases/pages/CsmCaseDetailPage"),
);
const CsmAdminLayout = lazy(
  () => import("@features/csm-admin/pages/CsmAdminLayout"),
);
const CsmUsersPage = lazy(
  () => import("@features/csm-users/pages/CsmUsersPage"),
);
const CsmAccountsPage = lazy(
  () => import("@features/csm-accounts/pages/CsmAccountsPage"),
);
const CsmAccountDetailPage = lazy(
  () => import("@features/csm-accounts/pages/CsmAccountDetailPage"),
);
const CsmProjectsPage = lazy(
  () => import("@features/csm-projects/pages/CsmProjectsPage"),
);
const CsmProjectDetailPage = lazy(
  () => import("@features/csm-projects/pages/CsmProjectDetailPage"),
);
const CsmUpdatesPage = lazy(
  () => import("@features/updates/pages/CsmUpdatesPage"),
);

/**
 * Landing for `/`. Defers to AuthGuard's post-login deep-link restore when a
 * redirect is pending (rendering nothing so it doesn't race the restore);
 * otherwise sends the user to the default `/dashboard` landing. A pure read of
 * sessionStorage — AuthGuard owns clearing the key.
 */
function RootLanding(): JSX.Element | null {
  const pending = sessionStorage.getItem(POST_LOGIN_REDIRECT_KEY);
  return pending ? null : <Navigate to="/dashboard" replace />;
}

export default function App(): JSX.Element {
  return (
    <LoaderProvider>
      <ErrorBannerProvider>
        <SuccessBannerProvider>
          <ErrorPageProvider>
            <PostLoginRedirectConsumer />
            <Routes>
              <Route
                path="/401"
                element={
                  <ErrorLayout>
                    <Error401Page />
                  </ErrorLayout>
                }
              />
              <Route
                path="/403"
                element={
                  <ErrorLayout>
                    <Error403Page />
                  </ErrorLayout>
                }
              />
              <Route
                path="/404"
                element={
                  <ErrorLayout>
                    <Error404Page />
                  </ErrorLayout>
                }
              />

              <Route element={<AuthGuard />}>
                <Route path="/" element={<RootLanding />} />

                {/* BFF-backed pages (entity-service search + by-id endpoints) */}
                <Route path="accounts" element={<CsmAccountsPage />} />
                <Route path="accounts/:id" element={<CsmAccountDetailPage />} />
                <Route path="projects" element={<CsmProjectsPage />} />
                <Route path="projects/:id" element={<CsmProjectDetailPage />} />

                {/* Administration — Users tab is real, others are WIP */}
                <Route path="admin" element={<CsmAdminLayout />}>
                  <Route index element={<Navigate to="/admin/users" replace />} />
                  <Route path="users" element={<CsmUsersPage />} />
                  <Route
                    path="roles"
                    element={
                      <CsmComingSoonPage
                        title="Roles"
                        description="Role-based access control: define roles and their permission sets."
                        blockedOn="csm-portal/backend roles endpoints"
                      />
                    }
                  />
                  <Route
                    path="groups"
                    element={
                      <CsmComingSoonPage
                        title="Groups"
                        description="User groups for bulk role assignment and access control."
                        blockedOn="csm-portal/backend groups endpoints"
                      />
                    }
                  />
                  <Route
                    path="permissions"
                    element={
                      <CsmComingSoonPage
                        title="Permissions"
                        description="Fine-grained permission catalog and assignment view."
                        blockedOn="csm-portal/backend permissions endpoints"
                      />
                    }
                  />
                </Route>

                <Route path="dashboard" element={<CsmDashboardPage />} />
                <Route path="cases" element={<CsmCasesPage />} />
                <Route path="cases/new" element={<CsmCaseCreatePage />} />
                <Route path="cases/:caseId" element={<CsmCaseDetailPage />} />

                {/* WIP placeholders for top-level features awaiting BFF support */}
                <Route
                  path="operations"
                  element={
                    <CsmComingSoonPage
                      title="Operations"
                      description="Service requests and change requests across customers."
                      blockedOn="csm-portal/backend operations endpoints"
                    />
                  }
                />
                <Route
                  path="engagements"
                  element={
                    <CsmComingSoonPage
                      title="Engagements"
                      description="Professional services engagements (migration, implementation, onboarding, training) across customers."
                      blockedOn="csm-portal/backend engagements endpoint"
                    />
                  }
                />
                <Route path="updates" element={<CsmUpdatesPage />} />
                <Route
                  path="security-center"
                  element={
                    <CsmComingSoonPage
                      title="Security center"
                      description="Vulnerability posture across customer deployments."
                      blockedOn="csm-portal/backend security endpoint"
                    />
                  }
                />
                <Route
                  path="time-cards"
                  element={
                    <CsmComingSoonPage
                      title="Time cards"
                      description="Engineer time-tracking entries against cases, with approval flow."
                      blockedOn="csm-portal/backend time-cards endpoints"
                    />
                  }
                />
              </Route>

              <Route
                path="*"
                element={
                  <ErrorLayout>
                    <Error404Page />
                  </ErrorLayout>
                }
              />
            </Routes>
          </ErrorPageProvider>
        </SuccessBannerProvider>
      </ErrorBannerProvider>
    </LoaderProvider>
  );
}
