/**
 * PLG's routes, as one element csm-portal's router can mount.
 *
 * PAGES ARE IMPORTED EAGERLY, not through `React.lazy`. That is csm-portal's
 * convention and it is a deliberate one: per-route lazy loading fetched a fresh
 * chunk on the critical path the first time an engineer visited a route in a
 * session, freezing the UI mid-navigation. The app ships as one bundle behind
 * index.html's boot loader instead. PLG's standalone build did lazy-load these
 * six pages; inside csm-portal it must not.
 *
 * WHAT ELSE THE MERGE CHANGED. Standalone, PLG's App.tsx owned a
 * `BrowserRouter`, an AppShell, a theme provider, a logger provider and a user
 * switcher. All five are csm-portal's now. What is left is the route table,
 * which is the only part that was ever PLG's own.
 */
import { Route } from "react-router";

import DashboardPage from "./pages/dashboard/DashboardPage";
import OrganizationDetailPage from "./pages/organizations/OrganizationDetailPage";
import OrganizationsPage from "./pages/organizations/OrganizationsPage";
import PlaybooksPage from "./pages/playbooks/PlaybooksPage";
import NewRegistrationsPage from "./pages/registrations/NewRegistrationsPage";
import WorkQueuePage from "./pages/work-queue/WorkQueuePage";

/**
 * PLG's route subtree. Spread into csm-portal's `<Routes>`, inside the same
 * `AuthGuard` / `FeatureRouteGuard` pair every other section sits under:
 *
 * ```tsx
 * {plgRoutes()}
 * ```
 *
 * A function rather than a component because `<Routes>` reads its children as
 * configuration — a wrapper component would be invisible to the router.
 */
export function plgRoutes() {
  return (
    <Route path="plg">
      {/* Standalone, PLG's landing page was its Workspace dashboard at "/".
          Keeping that here means /plg opens on the same page it always did. */}
      <Route index element={<DashboardPage />} />
      <Route path="dashboard" element={<DashboardPage />} />
      <Route path="work-queue" element={<WorkQueuePage />} />
      <Route path="organizations" element={<OrganizationsPage />} />
      {/* One organisation, optionally opened on one platform's tab. */}
      <Route path="organizations/:organizationId" element={<OrganizationDetailPage />} />
      <Route path="organizations/:organizationId/:productCode" element={<OrganizationDetailPage />} />
      <Route path="new-registrations" element={<NewRegistrationsPage />} />
      <Route path="playbooks" element={<PlaybooksPage />} />
      {/* The leadership view — the SAME component as /plg/dashboard above, with
          fewer tiles. It reads the last path segment to decide which, so this
          route is the switch: "overview" drops the two queue counts, because
          those are a to-do list and a leadership dashboard is not one. */}
      <Route path="overview" element={<DashboardPage />} />
    </Route>
  );
}
