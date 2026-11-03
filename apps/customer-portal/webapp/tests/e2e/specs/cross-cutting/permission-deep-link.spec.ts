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

//
// Cross-cutting: typing the URL of a feature the project does not have.
//
// ✅ READ-ONLY. Nothing is created or changed.
//
// WHY THIS NEEDS ITS OWN TEST
//
// Hiding the side-nav item is NOT the same as blocking the route. The routes in
// App.tsx all sit under ProjectGuard, which resolves the project and nothing
// else — none of them are individually permission-guarded. So every per-feature
// route is reachable by typing its URL, and what stops a user seeing a feature
// their project has not got is whatever that page chooses to render. The nav
// tests (SIDE_NAV_VISIBILITY) cover the menu; only a deep link covers this.
//
// The control case is the point. Each route is opened twice — once on a project
// WITHOUT the feature and once on a project WITH it — because "the guard text is
// on the page" proves nothing on its own: a page that showed it unconditionally
// would pass. Only the pair shows the guard is driven by permissions.
//
// ⚠️ FINDINGS — verified live 2026-10-08, deliberately NOT asserted here
// because they are product decisions rather than test failures:
//
//   • `/projects/{id}/operations` on a project with neither SR nor CR access
//     renders an EMPTY page — no cards, no message, just the footer. A user who
//     follows an old link sees a blank screen with no explanation.
//   • `/projects/{id}/updates` on Cloud Support — which has no updates access,
//     and no Updates item in its side nav — renders the "Search Update Levels"
//     page as though permitted.
//
// Both are routes the nav hides but the URL exposes. The two covered below are
// the ones that do implement a guard; extend this list as the others gain one.
//

import {
  test,
  expect,
  withSession,
  type Locator,
  type Page,
} from "../../fixtures/test";
import { PROJECTS, ProjectType } from "../../config/testData";
import {
  CREATE_SERVICE_REQUEST,
  PERMISSION_GUARDS,
  SECURITY_CENTER,
} from "../../utils/selectors";

withSession(test);

/** A route, the guard it shows without the feature, and a marker that only the
 * working page renders. */
interface DeepLink {
  /** Names the feature in the test title. */
  feature: string;
  /** Path after `/projects/{id}/`. */
  path: string;
  /** Shown in place of the feature. */
  guard: string;
  /** Something only the WORKING page renders, so the control case can prove the
   * guard is conditional rather than always on.
   *
   * A locator rather than text: the markers that genuinely distinguish the two
   * states are not all plain strings, and several headings render in BOTH —
   * the guarded security page still shows its "Security Report Analysis"
   * title, so matching that would never tell the states apart. */
  featureMarker: (page: Page) => Locator;
  /** A project that lacks the feature, and one that has it. */
  withoutFeature: ProjectType;
  withFeature: ProjectType;
}

const DEEP_LINKS: DeepLink[] = [
  {
    feature: "service requests",
    path: "operations/service-requests/create",
    guard: PERMISSION_GUARDS.serviceRequests,
    // The submit button, not a heading. The guarded page still renders the
    // "New Service Request" header, and the form itself is STAGED — "Select
    // Request Type" only appears once a deployment and product are chosen, so
    // it is absent on a freshly deep-linked page even when permitted. The
    // submit button is on the working page from the start and never on the
    // guarded one. Verified live.
    featureMarker: (page) =>
      page.getByRole("button", {
        name: CREATE_SERVICE_REQUEST.submitButton,
        exact: true,
      }),
    // Subscription's Operations nav item is hidden precisely because it has
    // neither SR nor CR access; Managed Cloud Subscription has SR.
    withoutFeature: ProjectType.SUBSCRIPTION,
    withFeature: ProjectType.MANAGED_CLOUD_SUBSCRIPTION,
  },
  {
    feature: "security report analysis",
    path: SECURITY_CENTER.pathSegment,
    guard: PERMISSION_GUARDS.securityReportAnalysis,
    // The list's search box, which only the working analysis renders — unlike
    // the page title, which the guarded state shows too.
    featureMarker: (page) =>
      page.getByPlaceholder(SECURITY_CENTER.searchPlaceholder),
    withoutFeature: ProjectType.CLOUD_SUPPORT,
    withFeature: ProjectType.SUBSCRIPTION,
  },
];

test.describe("Permission deep links", () => {
  // A cold load per project, plus the project-features query each guard waits
  // on — permissions start restrictive and the page re-renders once they land.
  test.describe.configure({ timeout: 180_000 });

  for (const link of DEEP_LINKS) {
    test(`deep-linking to ${link.feature} without permission shows the guard state`, async ({
      page,
    }) => {
      const denied = PROJECTS[link.withoutFeature];
      const allowed = PROJECTS[link.withFeature];
      test.skip(
        !denied.id || !allowed.id,
        `Needs project ids for ${link.withoutFeature} and ${link.withFeature}.`,
      );

      //
      // Without the feature: the guard, and not the feature.
      //
      await page.goto(`/projects/${denied.id}/${link.path}`);

      await expect(
        page.getByText(link.guard),
        `${link.withoutFeature} has no ${link.feature}, so the route should ` +
          `explain that rather than render the feature or nothing at all`,
      ).toBeVisible({ timeout: 60_000 });

      // Still on the route. The guard is rendered IN PLACE — nothing redirects
      // — so a bounce back to the dashboard would be a different behaviour that
      // happened to leave the user somewhere reasonable.
      expect(new URL(page.url()).pathname).toContain(link.path);

      await expect(
        link.featureMarker(page),
        `the ${link.feature} UI must not render for a project without it`,
      ).toHaveCount(0);

      //
      // With the feature: no guard. This is what makes the above a permission
      // check rather than an assertion that some text exists.
      //
      await page.goto(`/projects/${allowed.id}/${link.path}`);

      await expect(
        link.featureMarker(page),
        `${link.withFeature} has ${link.feature}, so the route should render it`,
      ).toBeVisible({ timeout: 60_000 });

      await expect(
        page.getByText(link.guard),
        `${link.withFeature} has ${link.feature}, so the guard must not show — ` +
          `if it does, the guard is unconditional and the case above proves nothing`,
      ).toHaveCount(0, { timeout: 60_000 });

      console.log(
        `Deep link [${link.feature}]: guarded on ${link.withoutFeature}, ` +
          `open on ${link.withFeature}`,
      );
    });
  }
});
