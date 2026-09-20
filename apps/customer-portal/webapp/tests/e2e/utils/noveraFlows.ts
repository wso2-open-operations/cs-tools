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
// Turning the Novera chat assistant on and off.
//
// ⚠️ This WRITES `hasAgent` on the project, and the effect reaches well beyond
// the settings page: with the assistant on, the header's Get Help opens the chat
// instead of the create-case form (`handleIssue` in GetHelpDropdown). So the
// create-case, case-matrix and close-case specs all depend on it being off.
//
// Any caller that switches it on MUST switch it back, from a `finally`.
//

import { expect, type Page, type Response } from "../fixtures/test";
import { sessionOrigin } from "../fixtures/test";
import { SettingsPage } from "../pages/SettingsPage";
import { SETTINGS } from "./selectors";

/** How long to allow for the settings page and its post-mutation refetch. */
const LOAD_TIMEOUT_MS = 30_000;

/**
 * Sets the Novera assistant's state and returns whether it was already there.
 *
 * Idempotent: a project already in the wanted state is left alone and no request
 * is sent, so a caller can use this for setup without knowing where it started.
 *
 * @param page - Test page.
 * @param projectId - Project to update.
 * @param enabled - The state to leave it in.
 * @returns The update response, or null when nothing needed changing.
 */
export async function setNoveraEnabled(
  page: Page,
  projectId: string,
  enabled: boolean,
): Promise<Response | null> {
  const settings = new SettingsPage(page);
  await settings.open(projectId);
  await settings.openTab(SETTINGS.tabs.aiAssistant);

  // The switch is disabled while the project details load, so this waits for it
  // to settle rather than reading a state that is still arriving.
  await expect(settings.noveraToggle()).toBeEnabled({
    timeout: LOAD_TIMEOUT_MS,
  });

  if ((await settings.noveraToggle().isChecked()) === enabled) return null;

  const response = await settings.setNovera(projectId, enabled);

  // Read back from the control, so a caller can trust the state on return.
  if (enabled) {
    await expect(settings.noveraToggle()).toBeChecked({
      timeout: LOAD_TIMEOUT_MS,
    });
  } else {
    await expect(settings.noveraToggle()).not.toBeChecked({
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  return response;
}

/**
 * Turns the AI Chat Assistant (Novera) on or off by calling the backend
 * directly, rather than by clicking the Settings toggle.
 *
 * WHY NOT THE TOGGLE: the switch issues `PATCH /projects/{id}` from the page,
 * and that request currently dies in the browser — the backend answers 404 and
 * the error response carries no CORS headers, so the fetch aborts with
 * `net::ERR_FAILED`, the switch reverts, and the chip stays Inactive. Driving
 * the UI therefore cannot enable the assistant at all.
 *
 * Going through `page.request` sidesteps the browser's CORS layer and, just as
 * importantly, surfaces the real status instead of an opaque network error — so
 * when this fails it fails with a number and a body rather than a timeout.
 *
 * ⚠️ This is a deliberate bypass of the UI. It is setup for specs whose subject
 * is the chat, not the settings page — the toggle itself is covered by
 * settings.spec.ts, which must keep driving the real control.
 *
 * The page must already be on an app URL: both the backend base URL
 * (`window.config`) and the bearer token (sessionStorage) are read from it.
 *
 * @param page - Test page, already navigated to the portal.
 * @param projectId - Project to update.
 * @param enabled - Desired `hasAgent` state.
 */
export async function setNoveraViaApi(
  page: Page,
  projectId: string,
  enabled: boolean,
): Promise<void> {
  const context = await page.evaluate(() => {
    const config = (window as unknown as {
      config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string };
    }).config;

    // The Asgardeo SDK stores its bundle under a key that varies by client id,
    // so this scans for the id token rather than assuming the key.
    let token: string | null = null;
    for (let i = 0; i < sessionStorage.length; i++) {
      const raw = sessionStorage.getItem(sessionStorage.key(i)!) ?? "";
      const match = raw.match(/"id_token"\s*:\s*"([^"]+)"/);
      if (match) {
        token = match[1];
        break;
      }
    }
    return { baseUrl: config?.CUSTOMER_PORTAL_BACKEND_BASE_URL, token };
  });

  // An expired session redirects to the identity provider, where neither
  // window.config nor the token exists. Without this check the call below would
  // be built against an undefined base, resolve relative to whatever origin the
  // page happens to be on, and come back 404 — indistinguishable from the
  // backend genuinely not exposing the route. That misdiagnosis is expensive, so
  // it is worth naming explicitly.
  //
  // The expected origin comes from the captured session rather than a hardcoded
  // host list: the suite runs against staging, a local dev server, or any other
  // deployment, and a baked-in list silently rejects the others. Skipped when
  // the bundle records no origin — the config and token checks below then do the
  // diagnosing on their own.
  const pageUrl = page.url();
  const expectedOrigin = sessionOrigin();
  if (expectedOrigin && new URL(pageUrl).origin !== expectedOrigin) {
    throw new Error(
      `Not on the portal — the page is at ${pageUrl}, but the captured session ` +
        `belongs to ${expectedOrigin}. This is what an expired session looks ` +
        "like: the app redirected to sign-in before any backend call could be " +
        "made. Re-capture the session and retry.",
    );
  }

  if (!context.baseUrl || !context.token) {
    throw new Error(
      "Cannot reach the backend directly: " +
        `${context.baseUrl ? "" : "window.config has no CUSTOMER_PORTAL_BACKEND_BASE_URL; "}` +
        `${context.token ? "" : "no id token in sessionStorage (session expired?); "}` +
        `page is at ${pageUrl}.`,
    );
  }

  // Relative bases silently resolve against the frontend origin, which answers
  // 404 for every API path — the same misleading result as above.
  if (!/^https?:\/\//.test(context.baseUrl)) {
    throw new Error(
      `Backend base URL is not absolute: "${context.baseUrl}". A relative base ` +
        "resolves against the portal origin and returns a 404 that looks like " +
        "a missing backend route.",
    );
  }

  const response = await page.request.fetch(
    `${context.baseUrl}/projects/${projectId}`,
    {
      method: "PATCH",
      headers: {
        Authorization: `Bearer ${context.token}`,
        "Content-Type": "application/json",
      },
      data: { hasAgent: enabled },
    },
  );

  if (!response.ok()) {
    const body = await response.text().catch(() => "(body unavailable)");
    throw new Error(
      `Could not ${enabled ? "enable" : "disable"} the AI Chat Assistant: ` +
        `PATCH ${context.baseUrl}/projects/${projectId} returned ` +
        `${response.status()}. ${body}\n` +
        "The route exists in backend/service.bal, so a 404 here means the " +
        "deployed backend does not expose it — a deployment fault, not a " +
        "test one.",
    );
  }
}
