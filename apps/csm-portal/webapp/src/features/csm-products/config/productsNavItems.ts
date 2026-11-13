/**
 * The product catalogue's entry in csm-portal's navigation: Settings ▸
 * Product catalogue (products, versions and release lines), the data behind
 * the public Support Matrix and Release Matrix. It is a Settings tab, next to
 * SLA Engine: business data a few people manage, not a daily-work menu.
 *
 * Declared here rather than inline in `csmNavItems.ts`, so a change to this
 * feature's pages does not touch the portal's navigation source. Its children
 * are route-backed tabs (no `tab`), rendered by `ProductsLayout`'s second-level
 * `SectionTabs` strip under Settings' own.
 *
 * The `id`s are a public contract: deployments key
 * `CSM_PORTAL_FEATURE_OVERRIDES` off them, so `admin.product-catalog` hides
 * the whole catalogue, which is what a deployment whose entity service has no
 * catalogue yet should do. (Until 2026-11 this was a top-level "Products"
 * section with ids `products`, `products.catalogue`, `products.release-lines`;
 * its `/products/...` links redirect here.)
 */
import type { CsmNavNode } from "@config/csmNavItems";

/** Base path for every product catalogue page. */
export const PRODUCT_CATALOG_BASE = "/admin/product-catalog";

export const PRODUCT_CATALOG_NAV_NODE: CsmNavNode = {
  id: "admin.product-catalog",
  label: "Product catalogue",
  href: PRODUCT_CATALOG_BASE,
  // Catalogue managers only: the tab, its pages (ProductCatalogRouteGuard)
  // and its help topic. Admins hold every permission, so they see it too.
  requires: "canManageProductCatalog",
  children: [
    {
      id: "admin.product-catalog.products",
      label: "Products",
      href: `${PRODUCT_CATALOG_BASE}/products`,
    },
    {
      id: "admin.product-catalog.release-lines",
      label: "Release lines",
      href: `${PRODUCT_CATALOG_BASE}/release-lines`,
    },
  ],
};
