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

/**
 * The Products section's entry in csm-portal's navigation: the product
 * catalogue (products, versions and release lines) behind the public Support
 * Matrix and Release Matrix.
 *
 * Declared here rather than inline in `csmNavItems.ts`, like PLG's section, so
 * a change to this feature's pages does not touch the portal's navigation
 * source. Its children are route-backed tabs (no `tab`), rendered by
 * `ProductsLayout`'s `SectionTabs` strip.
 *
 * The `id`s are a public contract: deployments key
 * `CSM_PORTAL_FEATURE_OVERRIDES` off them, so `products` hides the whole
 * section — which is what a deployment whose entity service has no catalogue
 * yet should do.
 */
import { Package } from "@wso2/oxygen-ui-icons-react";

import type { CsmNavSection } from "@config/csmNavItems";

/** Base path for every Products page. */
export const PRODUCTS_BASE = "/products";

export const PRODUCTS_NAV_SECTION: CsmNavSection = {
  id: "products",
  label: "Products",
  href: PRODUCTS_BASE,
  icon: Package,
  // Staff-only, like Knowledge -- see canViewStaffSections's own doc comment.
  // Reading the catalogue needs only view on the backend.
  requires: "canViewStaffSections",
  children: [
    {
      id: "products.catalogue",
      label: "Catalogue",
      href: `${PRODUCTS_BASE}/catalogue`,
    },
    {
      id: "products.release-lines",
      label: "Release lines",
      href: `${PRODUCTS_BASE}/release-lines`,
    },
  ],
};
