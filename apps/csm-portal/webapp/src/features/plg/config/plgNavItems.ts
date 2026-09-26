/**
 * PLG's entry in csm-portal's navigation.
 *
 * WHAT THE MERGE CHANGED. Standalone, PLG had its own sidebar with three
 * groups — Overview, Workspace, Manage — because it was a whole application.
 * Inside csm-portal it is one section beside Support and Operations, and those
 * three groups become its five rail entries.
 *
 * EVERY CHILD CARRIES A `tab`, AND IT HAS TO. `CsmSideBar.isSubmenuSection`
 * renders a section's children as rail entries only when *every* child has one
 * — it is the structural marker that distinguishes a rail submenu (Operations,
 * Security Center) from a section whose children live in an in-page tab strip
 * (Customers, Settings). Without it PLG rendered as a single flat item and its
 * other four pages were reachable only through search.
 *
 * PLG's children are route-backed rather than query-param tabs, which is the
 * less common of the two shapes this field supports. `navNodeHref` handles it:
 * given both a `tab` and a `routes` entry it navigates to `routes[0]`, so these
 * land on real paths rather than `?tab=` queries.
 *
 * Declared here rather than inline in `csmNavItems.ts` so the merge adds a file
 * and changes one line, and so a later change to PLG's pages does not touch
 * csm-portal's navigation source.
 *
 * The `id`s are a public contract: deployments key
 * `CSM_PORTAL_FEATURE_OVERRIDES` off them to hide or disable a page, so
 * renaming one is a breaking config change. `plg` hides the whole section —
 * which is what a deployment without the PLG backend should do.
 */
import {
  Building2,
  ChartColumn,
  Gauge,
  LayoutDashboard,
  Rocket,
  Sparkles,
  SquareCheckBig,
} from "@wso2/oxygen-ui-icons-react";

import type { CsmNavSection } from "@config/csmNavItems";

/** Base path for every PLG page. Matches the `/plg` API prefix, deliberately. */
export const PLG_BASE = "/plg";

export const PLG_NAV_SECTION: CsmNavSection = {
  id: "plg",
  label: "PLG",
  href: `${PLG_BASE}/dashboard`,
  icon: Rocket,
  children: [
    {
      id: "plg.overview",
      tab: "overview",
      label: "Leadership Dashboard",
      href: `${PLG_BASE}/overview`,
      routes: [`${PLG_BASE}/overview`],
      icon: Gauge,
    },
    {
      // Standalone this was Workspace > Dashboard, and PLG's landing page. It
      // shares a component with the leadership view above and differs by two
      // tiles — the queue counts, which belong to whoever works the queue.
      id: "plg.dashboard",
      tab: "dashboard",
      label: "Dashboard",
      href: `${PLG_BASE}/dashboard`,
      routes: [`${PLG_BASE}/dashboard`],
      icon: ChartColumn,
    },
    {
      id: "plg.work-queue",
      tab: "work_queue",
      label: "My Work",
      href: `${PLG_BASE}/work-queue`,
      routes: [`${PLG_BASE}/work-queue`],
      icon: SquareCheckBig,
    },
    {
      id: "plg.organizations",
      tab: "organizations",
      label: "Organisations",
      href: `${PLG_BASE}/organizations`,
      // The detail route is listed so the sidebar keeps this tab selected while
      // an engineer is inside one organisation's pairing view.
      routes: [`${PLG_BASE}/organizations`],
      icon: Building2,
    },
    {
      id: "plg.new-registrations",
      tab: "new_registrations",
      label: "New Registrations",
      href: `${PLG_BASE}/new-registrations`,
      routes: [`${PLG_BASE}/new-registrations`],
      icon: Sparkles,
    },
    {
      id: "plg.playbooks",
      tab: "playbooks",
      label: "Manage Playbooks",
      href: `${PLG_BASE}/playbooks`,
      routes: [`${PLG_BASE}/playbooks`],
      icon: LayoutDashboard,
    },
  ],
};
