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
import {
  Navigate,
  Outlet,
  Route,
  Routes,
  useLocation,
  useParams,
  useSearchParams,
} from "react-router";
import AuthGuard from "@layouts/AuthGuard";
import {
  LegacyQueryTabRedirect,
  SectionIndexRedirect,
} from "@components/section-tabs/SectionTabs";
import { navNodeForPath } from "@config/csmNavItems";
import {
  featureStateForPath,
  firstEnabledDestination,
} from "@config/featureFlags";
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
const CsMonitorDashboardPage = lazy(
  () => import("@features/csm-dashboard/pages/CsMonitorDashboardPage"),
);
const DashboardWidgetPreviewPage = lazy(
  () => import("@features/csm-dashboard/pages/DashboardWidgetPreviewPage"),
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
const OperationsPage = lazy(
  () => import("@features/csm-operations/pages/OperationsPage"),
);
const CreateServiceRequestPage = lazy(
  () => import("@features/csm-operations/pages/CreateServiceRequestPage"),
);
const CsmChangeRequestDetailPage = lazy(
  () => import("@features/csm-operations/pages/CsmChangeRequestDetailPage"),
);
const CreateChangeRequestPage = lazy(
  () => import("@features/csm-operations/pages/CreateChangeRequestPage"),
);
const CsmIncidentDetailPage = lazy(
  () => import("@features/csm-operations/pages/CsmIncidentDetailPage"),
);
const CreateIncidentPage = lazy(
  () => import("@features/csm-operations/pages/CreateIncidentPage"),
);
const ProblemDetailPage = lazy(
  () => import("@features/csm-operations/pages/ProblemDetailPage"),
);
const CreateProblemPage = lazy(
  () => import("@features/csm-operations/pages/CreateProblemPage"),
);
const CsmAdminLayout = lazy(
  () => import("@features/csm-admin/pages/CsmAdminLayout"),
);
const CsmUserManagementLandingPage = lazy(
  () => import("@features/csm-admin/pages/CsmUserManagementLandingPage"),
);
const CsmUsersPage = lazy(
  () => import("@features/csm-users/pages/CsmUsersPage"),
);
const UserProfilePage = lazy(
  () => import("@features/csm-users/pages/UserProfilePage"),
);
const CsmRolesPage = lazy(
  () => import("@features/csm-admin/pages/CsmRolesPage"),
);
const RoleMembersPage = lazy(
  () => import("@features/csm-admin/pages/RoleMembersPage"),
);
const CsmGroupsPage = lazy(
  () => import("@features/csm-admin/pages/CsmGroupsPage"),
);
const GroupMembersPage = lazy(
  () => import("@features/csm-admin/pages/GroupMembersPage"),
);
const CsmTeamsPage = lazy(
  () => import("@features/csm-admin/pages/CsmTeamsPage"),
);
const TeamMembersPage = lazy(
  () => import("@features/csm-admin/pages/TeamMembersPage"),
);
const DashboardBuilderRouteGuard = lazy(
  () => import("@features/csm-admin/dashboards/pages/DashboardBuilderRouteGuard"),
);
const CsmDashboardBuilderListPage = lazy(
  () => import("@features/csm-admin/dashboards/pages/CsmDashboardBuilderListPage"),
);
const CsmDashboardBuilderEditorPage = lazy(
  () => import("@features/csm-admin/dashboards/pages/CsmDashboardBuilderEditorPage"),
);
const CsmCustomersLayout = lazy(
  () => import("@features/csm-customers/pages/CsmCustomersLayout"),
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
const ConversationDetailPage = lazy(
  () => import("@features/csm-projects/pages/ConversationDetailPage"),
);
const CsmUpdatesPage = lazy(
  () => import("@features/updates/pages/CsmUpdatesPage"),
);
const CsmSecurityCenterPage = lazy(
  () => import("@features/csm-security-center/pages/CsmSecurityCenterPage"),
);
const CreateSecurityReportPage = lazy(
  () => import("@features/csm-security-center/pages/CreateSecurityReportPage"),
);
const ProductVulnerabilityDetailPage = lazy(
  () => import("@features/csm-security-center/pages/ProductVulnerabilityDetailPage"),
);
const CsmEngagementsPage = lazy(
  () => import("@features/csm-engagements/pages/CsmEngagementsPage"),
);
const CsmEngagementCreatePage = lazy(
  () => import("@features/csm-engagements/pages/CsmEngagementCreatePage"),
);
const CsmTimeCardsPage = lazy(
  () => import("@features/csm-timecards/pages/CsmTimeCardsPage"),
);
const CsmAnnouncementsPage = lazy(
  () => import("@features/csm-announcements/pages/CsmAnnouncementsPage"),
);

/**
 * Landing for `/`. Defers to AuthGuard's post-login deep-link restore when a
 * redirect is pending (rendering nothing so it doesn't race the restore).
 *
 * Also defers — rendering nothing rather than navigating to `/dashboard` —
 * while a `?goto=`/`?q=` deep link is still being resolved by `QuickNav`
 * (mounted in the persistent header, above this route's `Outlet`). Without
 * this, `/?goto=CS0441150` would navigate straight to `/dashboard` and start
 * fetching every dashboard widget in the background, purely to sit behind
 * the search palette while it resolves — wasted work for a link whose whole
 * point is to jump straight to one record. `QuickNav` clears `goto`/`q` from
 * the URL itself once it actually navigates away (the single-match case) or
 * once the user closes the palette without picking anything (see its
 * `close()`); either way, this component naturally proceeds to the normal
 * `/dashboard` redirect on its next render once the params are gone.
 *
 * A pure read of sessionStorage/search params — AuthGuard and QuickNav own
 * clearing their respective keys.
 */
function RootLanding(): JSX.Element | null {
  const pending = sessionStorage.getItem(POST_LOGIN_REDIRECT_KEY);
  const [searchParams] = useSearchParams();
  // A present-but-empty `?q=`/`?goto=` (e.g. a link with a blank query) must
  // not defer the redirect forever: QuickNav's own initial-params effect
  // already no-ops on an empty value (`if (!q && !goto) return`), so nothing
  // would ever clear the param and this component would sit blank
  // indefinitely on `.has()` alone. Require an actual non-blank value.
  const hasDeepLinkSearch = ["goto", "q"].some((key) =>
    Boolean(searchParams.get(key)?.trim()),
  );
  return pending || hasDeepLinkSearch ? null : <Navigate to="/dashboard" replace />;
}

/**
 * Layout guard honouring the `CSM_PORTAL_FEATURE_OVERRIDES` runtime config, so
 * a direct, pinned or shared link can't reach a page the deployment restricts.
 *
 * A `wip` page renders the shared "coming soon" message in place — the URL
 * survives and the wording matches the nav's "work in progress" tooltip. The
 * exception is a page that already renders a more specific unavailable message
 * of its own (`rendersOwnWipPage`), which is let through rather than downgraded
 * to the generic one.
 *
 * A `hidden` page has no nav entry at all, so there is nothing to stay
 * consistent with and the link is bounced to the first destination this
 * deployment does offer. That target is never assumed to exist: a config that
 * hides everything, or that hides the target itself, falls through to `/404`,
 * which sits outside this guard and so cannot bounce back here.
 *
 * The path resolves to the most specific nav node, so restricting a section
 * does not restrict a finished tab inside it (and vice versa).
 */
function FeatureRouteGuard(): JSX.Element {
  const { pathname } = useLocation();
  const node = navNodeForPath(pathname);
  const state = featureStateForPath(pathname);

  if (state === "hidden") {
    const fallback = firstEnabledDestination();
    const samePath = fallback !== undefined && fallback.split(/[?#]/)[0] === pathname;
    return <Navigate to={!fallback || samePath ? "/404" : fallback} replace />;
  }

  if (state === "wip" && !node?.rendersOwnWipPage) {
    const label = node?.label ?? "This section";
    return (
      <CsmComingSoonPage
        title={label}
        description={`${label} is still a work in progress and isn't available yet.`}
      />
    );
  }

  return <Outlet />;
}

/**
 * Redirects a legacy detail path (`/accounts/:id`, `/projects/:id`) to its new
 * home under `/customers`, preserving the id. Exists only so old pinned/deep
 * links survive the Accounts+Projects → Customers menu merge.
 */
function LegacyDetailRedirect({ to }: { to: string }): JSX.Element {
  const { id } = useParams();
  // Preserve any query/hash so legacy deep links (e.g. /accounts/:id?tab=…#…)
  // keep their context through the compatibility redirect.
  const { search, hash } = useLocation();
  const target = id ? `${to}/${id}` : to;
  return <Navigate to={`${target}${search}${hash}`} replace />;
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

              {/* Standalone full-screen wallboard/kiosk view of the
                  cs-overview dashboard — real sign-in required (still
                  routed through AuthGuard), but no portal chrome at all:
                  no header, sidebar, banners, or dashboard switcher, just
                  this one page. Deliberately its own AuthGuard instance
                  (`bare`) and OUTSIDE FeatureRouteGuard — it has no nav
                  entry to be WIP-gated against, unlike every route in the
                  main authenticated block below. See
                  CsMonitorDashboardPage.tsx's own doc comment. */}
              <Route element={<AuthGuard bare />}>
                <Route
                  path="cs-monitor-dashboard"
                  element={<CsMonitorDashboardPage />}
                />
              </Route>

              <Route element={<AuthGuard />}>
                <Route element={<FeatureRouteGuard />}>
                  <Route path="/" element={<RootLanding />} />

                  {/* Customers — Accounts + Projects under one tabbed section.
                      BFF-backed pages (entity-service search + by-id endpoints).
                      Detail pages render full-width (outside the tab layout). */}
                  <Route path="customers" element={<CsmCustomersLayout />}>
                    <Route
                      index
                      element={<SectionIndexRedirect sectionId="customers" />}
                    />
                    <Route path="accounts" element={<CsmAccountsPage />} />
                    <Route path="projects" element={<CsmProjectsPage />} />
                  </Route>
                  <Route
                    path="customers/accounts/:id"
                    element={<CsmAccountDetailPage />}
                  />
                  <Route
                    path="customers/projects/:id"
                    element={<CsmProjectDetailPage />}
                  />

                  {/* Legacy paths kept alive so pinned/deep links don't 404. */}
                  <Route
                    path="accounts"
                    element={<Navigate to="/customers/accounts" replace />}
                  />
                  <Route
                    path="accounts/:id"
                    element={<LegacyDetailRedirect to="/customers/accounts" />}
                  />
                  <Route
                    path="projects"
                    element={<Navigate to="/customers/projects" replace />}
                  />
                  <Route
                    path="projects/:id"
                    element={<LegacyDetailRedirect to="/customers/projects" />}
                  />

                  {/* Administration — "User management" groups the
                      Users/Roles/Groups/Teams/Permissions directory pages
                      (Users/Roles/Groups/Teams are real, Permissions is still
                      WIP) behind a tile-grid landing page; Dashboards is a
                      sibling tab. */}
                  <Route path="admin" element={<CsmAdminLayout />}>
                    <Route
                      index
                      element={<SectionIndexRedirect sectionId="admin" />}
                    />
                    <Route
                      path="user-management"
                      element={<CsmUserManagementLandingPage />}
                    />
                    <Route path="user-management/users" element={<CsmUsersPage />} />
                    <Route path="user-management/roles" element={<CsmRolesPage />} />
                    <Route path="user-management/groups" element={<CsmGroupsPage />} />
                    <Route path="user-management/teams" element={<CsmTeamsPage />} />
                    <Route
                      path="user-management/permissions"
                      element={
                        <CsmComingSoonPage
                          title="Permissions"
                          description="Fine-grained permission catalog and assignment view."
                          blockedOn="csm-portal/backend permissions endpoints"
                        />
                      }
                    />
                    {/* Dashboard builder — admin-role-gated, unlike every
                        sibling tab above (see DashboardBuilderRouteGuard's
                        own doc comment for why). Persists to localStorage
                        only; there is no backend behind this feature. */}
                    <Route path="dashboards" element={<DashboardBuilderRouteGuard />}>
                      <Route index element={<CsmDashboardBuilderListPage />} />
                      <Route path="new" element={<CsmDashboardBuilderEditorPage />} />
                      <Route path=":draftId" element={<CsmDashboardBuilderEditorPage />} />
                    </Route>
                  </Route>

                  {/* Legacy Settings paths kept alive so a pinned/deep link to
                      the pre-"User management" layout doesn't dead-end. Not
                      requested explicitly — a judgment call to match the
                      /accounts, /projects legacy-redirect convention above;
                      revert this block alone if unwanted. */}
                  <Route
                    path="admin/users"
                    element={<Navigate to="/admin/user-management/users" replace />}
                  />
                  <Route
                    path="admin/roles"
                    element={<Navigate to="/admin/user-management/roles" replace />}
                  />
                  <Route
                    path="admin/groups"
                    element={<Navigate to="/admin/user-management/groups" replace />}
                  />
                  <Route
                    path="admin/teams"
                    element={<Navigate to="/admin/user-management/teams" replace />}
                  />
                  <Route
                    path="admin/permissions"
                    element={<Navigate to="/admin/user-management/permissions" replace />}
                  />

                  {/* Role/group/team member lists, one level below the
                      directory pages above. Not admin-permission-gated:
                      standing project rule is to show the action and let the
                      backend reject it, never gate in the frontend. Left at
                      their original /admin/<kind>/:id paths — out of scope
                      for the User management nesting above — so every
                      `routeBase="/admin/roles"` etc. link elsewhere in the app
                      keeps working unchanged. */}
                  <Route path="admin/roles/:id" element={<RoleMembersPage />} />
                  <Route path="admin/groups/:id" element={<GroupMembersPage />} />
                  <Route path="admin/teams/:id" element={<TeamMembersPage />} />

                  {/* Person profile — reachable by clicking any user reference
                      (case creator, assignee, watchers, comment authors,
                      attachment uploaders). Keyed on the user id (not the
                      email): most `UserReference` sites don't resolve an id
                      themselves, so `UserRefLink` only ever links once one is
                      known or resolved (see useResolvedUserId). Not
                      admin-gated: any signed-in CS engineer can look up any
                      user. */}
                  <Route
                    path="people/:id"
                    element={<UserProfilePage />}
                  />

                  {/* Dashboard selection is a real path segment
                      (`/dashboard/:dashboardId`), and — for a team-based
                      dashboard (a dashboard config's own `isTeamBased` flag) —
                      the selected team is a SECOND path segment
                      (`/dashboard/:dashboardId/:teamId`), since both are a
                      genuinely different content set from one another, not a
                      panel switch on the same view. The bare `/dashboard`
                      index still renders CsmDashboardPage itself: which
                      dashboard/team to default to depends on data that isn't
                      loaded yet at route-match time (the dashboard list, the
                      signed-in user's own team) — see the page's own doc
                      comment — so it resolves that itself and then replaces
                      the URL with the canonical one-or-two-segment path once
                      it can, rather than a synchronous route redirect here. */}
                  <Route path="dashboard" element={<CsmDashboardPage />} />
                  <Route path="dashboard/:dashboardId" element={<CsmDashboardPage />} />
                  <Route
                    path="dashboard/:dashboardId/:teamId"
                    element={<CsmDashboardPage />}
                  />
                  {/* Dashboard widget "View more" preview — :previewSlug is
                      one of WIDGET_RESOURCE_CONFIG's own previewSlug values
                      (e.g. "cases"), resolved back to a resourceType by
                      resourceTypeForPreviewSlug. Distinct from the resource's
                      own real list route (e.g. /cases). Under its own
                      "preview" prefix, not directly under /dashboard/:x —
                      that shape collides with /dashboard/:dashboardId above
                      (both a single dynamic segment at the same depth; only
                      one of two same-shape sibling routes can ever match a
                      given value, so they can't share it). */}
                  <Route
                    path="dashboard/preview/:previewSlug"
                    element={<DashboardWidgetPreviewPage />}
                  />
                  <Route path="cases" element={<CsmCasesPage />} />
                  <Route path="cases/new" element={<CsmCaseCreatePage />} />
                  <Route path="cases/:caseId" element={<CsmCaseDetailPage />} />

                  {/* A project's chat sessions ("Conversations" sub-tab of
                      Work items) each get a dedicated detail page, flat at
                      the top level like /cases/:caseId rather than nested
                      under /customers/projects/:id — matching how every
                      other work-item type (service requests, change
                      requests, engagements, ...) routes below. */}
                  <Route path="conversations/:id" element={<ConversationDetailPage />} />

                  {/* Operations' own Service requests / Change requests /
                      Incidents / Problems switch is a real path segment
                      (`/operations/:tab`) rather than `?tab=` — see
                      `usePathSectionTabs` — since it's a genuinely different
                      content set each time, not a panel switch on the same
                      record. The bare `/operations` index also still handles
                      an OLD `?tab=` link (this section's shape before this
                      change): `LegacyQueryTabRedirect` translates it to the
                      new path form, or lands on the first usable tab if
                      there's no `?tab=` at all. */}
                  <Route path="operations">
                    <Route
                      index
                      element={
                        <LegacyQueryTabRedirect sectionId="operations" basePath="/operations" />
                      }
                    />
                    <Route path=":tab" element={<OperationsPage />} />
                    <Route path="service-requests/new" element={<CreateServiceRequestPage />} />
                    <Route
                      path="service-requests/:caseId"
                      element={<CsmCaseDetailPage />}
                    />
                    <Route
                      path="change-requests/new"
                      element={<CreateChangeRequestPage />}
                    />
                    <Route
                      path="change-requests/:id"
                      element={<CsmChangeRequestDetailPage />}
                    />
                    <Route path="incidents/new" element={<CreateIncidentPage />} />
                    <Route path="incidents/:id" element={<CsmIncidentDetailPage />} />
                    <Route path="problems/new" element={<CreateProblemPage />} />
                    <Route path="problems/:id" element={<ProblemDetailPage />} />
                  </Route>

                  <Route path="engagements" element={<CsmEngagementsPage />} />
                  <Route path="engagements/new" element={<CsmEngagementCreatePage />} />
                  <Route path="engagements/:caseId" element={<CsmCaseDetailPage />} />
                  <Route path="updates" element={<CsmUpdatesPage />} />
                  {/* Security Center's own Security reports / Vulnerabilities
                      switch — same path-segment + legacy-`?tab=`-redirect
                      treatment as Operations above. */}
                  <Route path="security-center">
                    <Route
                      index
                      element={
                        <LegacyQueryTabRedirect
                          sectionId="security-center"
                          basePath="/security-center"
                        />
                      }
                    />
                    <Route path=":tab" element={<CsmSecurityCenterPage />} />
                    <Route path="reports/new" element={<CreateSecurityReportPage />} />
                    <Route
                      path="vulnerabilities/:id"
                      element={<ProductVulnerabilityDetailPage />}
                    />
                    <Route
                      path="security-reports/:caseId"
                      element={<CsmCaseDetailPage />}
                    />
                  </Route>
                  <Route path="time-cards" element={<CsmTimeCardsPage />} />
                  <Route path="announcements" element={<CsmAnnouncementsPage />} />
                  <Route
                    path="announcements/:caseId"
                    element={<CsmCaseDetailPage />}
                  />
                </Route>
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
