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
// Cross-cutting: dark and light theming, across every page.
//
// ✅ READ-ONLY. Only navigation and reads.
//
// ⚠️ THERE IS NO IN-APP THEME TOGGLE — verified 2026-10-08.
//
// Nothing in `src/` writes `<html data-color-scheme>`; the portal only ever
// READS it (useDarkMode.ts, Brand.tsx). Oxygen UI sets it, from the browser's
// `prefers-color-scheme`. There is no switch in the header, the side nav, or the
// Settings → Display tab, which offers font size only.
//
// So this drives the mechanism that actually exists: Playwright emulates the OS
// preference per context, and the test asserts the portal honours it everywhere.
// A test that hunted for a toggle would have nothing to click.
//
// WHAT "ACROSS PAGES" IS WORTH PROVING
//
// Each page is reached by its own URL, so every one is a cold boot — the theme
// has to be established before first paint rather than inherited from whatever
// the previous page left behind. One page is then navigated to IN-APP as well,
// because a client-side route change does not re-run the boot path and is where
// a theme would realistically be dropped.
//
// And the two schemes are compared against each other: the attribute alone could
// be set and ignored, so the rendered background must actually differ.
//

import {
  test,
  expect,
  hasSession,
  sessionOrigin,
  type Page,
} from "../../fixtures/test";
import {
  readSessionBundle,
  replayWithoutCookies,
} from "../../utils/sessionBundle";
import { PROJECTS, ProjectType } from "../../config/testData";
import { CASE_DETAIL, SIDE_NAV } from "../../utils/selectors";
import { SideNavPage } from "../../pages/SideNavPage";

const project = PROJECTS[ProjectType.SUBSCRIPTION];

/** A cold shell load per page. */
const LOAD_TIMEOUT_MS = 60_000;

/** The attribute Oxygen UI stamps on `<html>`, and the portal reads. */
const SCHEME_ATTRIBUTE = "data-color-scheme";

/** Pages to check, as paths under the project. Spread across features on
 * purpose — the theme is applied by the shell, so a page that got it wrong
 * would be one rendering outside it. */
const PAGE_PATHS = [
  "dashboard",
  "support",
  "project-details",
  "settings",
] as const;

/**
 * Reads the applied scheme and the rendered page background.
 *
 * Both together, because they answer different questions: the attribute says
 * what the app DECIDED, the background says whether anything acted on it.
 *
 * @param page - The page to read.
 * @returns The scheme attribute and the body's computed background colour.
 */
async function readTheme(
  page: Page,
): Promise<{ scheme: string | null; background: string }> {
  return page.evaluate((attribute) => {
    return {
      scheme: document.documentElement.getAttribute(attribute),
      background: window.getComputedStyle(document.body).backgroundColor,
    };
  }, SCHEME_ATTRIBUTE);
}

test.describe("Theme", () => {
  // Several cold page loads per scheme.
  test.describe.configure({ timeout: 180_000 });

  test.beforeEach(() => {
    test.skip(
      !hasSession(),
      `No captured session. See tests/e2e/auth/README.md.`,
    );
    test.skip(!project.id, `${ProjectType.SUBSCRIPTION} needs a project id.`);
  });

  /** Backgrounds seen per scheme, so the two runs can be compared at the end. */
  const backgrounds = new Map<string, string>();

  for (const scheme of ["light", "dark"] as const) {
    test(`${scheme} theme applies across pages`, async ({
      browser,
      baseURL,
    }) => {
      const captured = sessionOrigin();
      const target = baseURL ? new URL(baseURL).origin : undefined;
      test.skip(
        !!captured && !!target && captured !== target,
        `Session was captured against ${captured} but this run targets ${target}.`,
      );

      // Its own context: `colorScheme` is a context-level emulation, so it
      // cannot be applied to the shared page the other specs use.
      const context = await browser.newContext({ colorScheme: scheme });
      try {
        await replayWithoutCookies(context, readSessionBundle());
        const page = await context.newPage();

        for (const path of PAGE_PATHS) {
          await page.goto(`/projects/${project.id}/${path}`);

          // Wait for the shell, so the read happens on a rendered page rather
          // than on an empty document that would report a default.
          await expect(page.getByTestId(CASE_DETAIL.mainTestId)).toBeVisible({
            timeout: LOAD_TIMEOUT_MS,
          });

          const { scheme: applied, background } = await readTheme(page);
          expect(
            applied,
            `${path} should render in ${scheme} — the shell applies the ` +
              `browser's prefers-color-scheme, so a page disagreeing with it ` +
              `is rendering outside the shell`,
          ).toBe(scheme);

          // Every page agrees on the background too, which is what makes this a
          // consistent theme rather than one attribute set four times.
          const seen = backgrounds.get(scheme);
          if (seen) {
            expect(
              background,
              `${path} renders a different background from the other ` +
                `${scheme} pages`,
            ).toBe(seen);
          } else {
            backgrounds.set(scheme, background);
          }
        }

        //
        // Client-side navigation, which does not re-run the boot path — the
        // realistic place for a theme to be dropped.
        //
        const sideNav = new SideNavPage(page);
        await sideNav.clickItem(
          SIDE_NAV.items.dashboard,
          new RegExp(`/projects/${project.id}/dashboard`),
        );

        const afterNav = await readTheme(page);
        expect(
          afterNav.scheme,
          `the theme should survive an in-app navigation`,
        ).toBe(scheme);
        expect(afterNav.background).toBe(backgrounds.get(scheme));

        // And a reload, which re-boots on the same URL.
        await page.reload();
        await expect(page.getByTestId(CASE_DETAIL.mainTestId)).toBeVisible({
          timeout: LOAD_TIMEOUT_MS,
        });
        const afterReload = await readTheme(page);
        expect(afterReload.scheme, `the theme should survive a reload`).toBe(
          scheme,
        );

        console.log(
          `Theme [${scheme}]: applied on ${PAGE_PATHS.length} pages, after ` +
            `in-app nav and reload (background ${afterReload.background})`,
        );
      } finally {
        await context.close();
      }
    });
  }

  test("dark and light render differently", async () => {
    // Depends on both runs above. Skipping rather than failing when one did not
    // run keeps the cause readable — this test has nothing of its own to say if
    // it has only one scheme to compare.
    test.skip(
      backgrounds.size < 2,
      `Both theme tests must run before their backgrounds can be compared.`,
    );

    const light = backgrounds.get("light");
    const dark = backgrounds.get("dark");

    // The point: without this, a build that stamped the attribute and themed
    // nothing would pass every assertion above.
    expect(
      dark,
      `dark and light rendered the same background (${light}), so the scheme ` +
        `attribute is being set but not applied`,
    ).not.toBe(light);

    console.log(`Theme: light ${light} vs dark ${dark}`);
  });
});
