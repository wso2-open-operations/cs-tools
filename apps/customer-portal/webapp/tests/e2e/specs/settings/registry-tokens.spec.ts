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
// Settings → Registry Tokens: generating a user token, and the rule that a
// token name must be unique.
//
// ⚠️ WRITES registry tokens (live container-registry credentials). Each test
// creates its own and deletes it again (in `finally` where deletion is not the
// thing under test), so a run leaves nothing behind unless a delete itself
// fails — every name carries the run's timestamp to find it by.
//
import { test, expect, withSession, type Page } from "../../fixtures/test";
import { SettingsPage } from "../../pages/SettingsPage";
import {
  PROJECTS,
  ProjectType,
  REGISTRY_SERVICE_TOKEN_INPUT,
} from "../../config/testData";
import {
  REGISTRY_SERVICE_TOKENS,
  REGISTRY_TOKEN_ACTIONS,
  REGISTRY_TOKENS,
} from "../../utils/selectors";

withSession(test);

const LOAD_TIMEOUT_MS = 60_000;
const project = PROJECTS[ProjectType.SUBSCRIPTION];

/**
 * A unique, valid token name: a random word plus the time to the minute,
 * e.g. "e2e-k3x9-202610221432".
 *
 * @returns The name.
 */
function tokenName(): string {
  const now = new Date();
  const pad = (n: number) => String(n).padStart(2, "0");
  const stamp =
    `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}` +
    `${pad(now.getHours())}${pad(now.getMinutes())}`;
  const random = Math.random().toString(36).slice(2, 6);
  return `e2e-${random}-${stamp}`;
}

/**
 * Opens Settings from the side menu, then the Registry Tokens tab.
 *
 * @param page - The page under test.
 * @returns Resolves once the Generate User Token button is ready.
 */
async function openRegistryTokens(page: Page): Promise<void> {
  const settings = new SettingsPage(page);
  await settings.openViaSideNav(project.id);
  await settings.openTab(REGISTRY_TOKENS.tab);
  await expect(
    page.getByRole("button", { name: REGISTRY_TOKENS.generateUserButton }),
  ).toBeEnabled({ timeout: LOAD_TIMEOUT_MS });
}

/**
 * Opens the Generate User Token dialog and types a name into it.
 *
 * @param page - The page under test.
 * @param name - Token name to enter.
 * @returns The dialog.
 */
async function startGenerating(page: Page, name: string) {
  await page.getByRole("button", { name: REGISTRY_TOKENS.generateUserButton }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText(REGISTRY_TOKENS.generateDialogTitle, { exact: true })).toBeVisible();
  const input = dialog.getByLabel(REGISTRY_TOKENS.nameLabel);
  await input.fill(name);
  // The name is validated on blur.
  await input.blur();
  return dialog;
}

/**
 * Generates a user token through the dialog and closes the secret step.
 *
 * @param page - The page under test, on the Registry Tokens tab.
 * @param name - Token name.
 * @returns The secret the dialog showed.
 */
async function generateToken(page: Page, name: string): Promise<string> {
  const dialog = await startGenerating(page, name);
  const createResponse = page.waitForResponse(
    (r) =>
      r.request().method() === "POST" &&
      /\/projects\/[^/]+\/registry-tokens$/.test(new URL(r.url()).pathname),
    { timeout: LOAD_TIMEOUT_MS },
  );
  await dialog.getByRole("button", { name: REGISTRY_TOKENS.generateButton }).click();
  const response = await createResponse;
  expect(response.ok(), `creating the token returned ${response.status()}`).toBe(true);
  await expect(
    dialog.getByText(REGISTRY_TOKENS.createdDialogTitle, { exact: true }),
  ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  const secret = await dialog.getByLabel(REGISTRY_TOKENS.secretLabel).inputValue();
  expect(secret, "the new token should show its secret").not.toBe("");
  await dialog.getByRole("button", { name: REGISTRY_TOKENS.closeButton, exact: true }).click();
  await expect(dialog).toHaveCount(0);
  await expect(tokenRow(page, name)).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  return secret;
}

/** The list row for a token, by name. */
function tokenRow(page: Page, name: string) {
  return page.getByRole("row").filter({ hasText: name });
}

/**
 * Opens a token's row actions and picks an item.
 *
 * @param page - The page under test.
 * @param name - Token name.
 * @param item - Menu item label.
 */
async function rowAction(page: Page, name: string, item: string): Promise<void> {
  await tokenRow(page, name)
    .getByRole("button", { name: REGISTRY_TOKENS.rowActions })
    .click();
  await page.getByRole("menuitem", { name: item }).click();
}

/**
 * Deletes a token through Token actions → Delete Token → Delete.
 *
 * @param page - The page under test.
 * @param name - Token name.
 */
async function deleteToken(page: Page, name: string): Promise<void> {
  await rowAction(page, name, REGISTRY_TOKENS.deleteMenuItem);
  const confirm = page.getByRole("dialog");
  await expect(
    confirm.getByText(REGISTRY_TOKENS.deleteDialogTitle, { exact: true }),
  ).toBeVisible();
  const deleted = page.waitForResponse(
    (r) =>
      r.request().method() === "DELETE" &&
      /\/registry-tokens\/[^/]+$/.test(new URL(r.url()).pathname),
    { timeout: LOAD_TIMEOUT_MS },
  );
  await confirm.getByRole("button", { name: REGISTRY_TOKENS.deleteButton, exact: true }).click();
  const response = await deleted;
  expect(response.ok(), `deleting the token returned ${response.status()}`).toBe(true);
  await expect(tokenRow(page, name)).toHaveCount(0, { timeout: LOAD_TIMEOUT_MS });
}

test.describe("Settings — Registry Tokens", () => {
  test.describe.configure({ timeout: 180_000 });

  test.beforeEach(() => {
    test.skip(!project.id, `${ProjectType.SUBSCRIPTION} needs a project id.`);
  });

  test("generates a user token and rejects a duplicate name", async ({ page }) => {
    await openRegistryTokens(page);
    const name = tokenName();
    let created = false;
    try {
      await generateToken(page, name);
      created = true;

      // The same name is refused before anything is sent.
      const again = await startGenerating(page, name);
      await expect(again.getByText(REGISTRY_TOKENS.duplicateError, { exact: true })).toBeVisible();
      await expect(
        again.getByRole("button", { name: REGISTRY_TOKENS.generateButton }),
      ).toBeDisabled();
      await again.getByRole("button", { name: REGISTRY_TOKENS.cancelButton, exact: true }).click();
      await expect(again).toHaveCount(0);
      console.log(`Registry Tokens: generated "${name}"; duplicate name refused`);
    } finally {
      if (created) await deleteToken(page, name);
    }
  });

  test("regenerates a token's secret", async ({ page }) => {
    await openRegistryTokens(page);
    const name = tokenName();
    let created = false;
    try {
      const original = await generateToken(page, name);
      created = true;

      await rowAction(page, name, REGISTRY_TOKEN_ACTIONS.regenerateMenuItem);
      const dialog = page.getByRole("dialog");
      await expect(
        dialog.getByText(REGISTRY_TOKEN_ACTIONS.regenerateDialogTitle, { exact: true }),
      ).toBeVisible();
      const regenerated = page.waitForResponse(
        (r) =>
          r.request().method() === "POST" &&
          /\/registry-tokens\/[^/]+\/regenerate$/.test(new URL(r.url()).pathname),
        { timeout: LOAD_TIMEOUT_MS },
      );
      await dialog
        .getByRole("button", { name: REGISTRY_TOKEN_ACTIONS.regenerateButton, exact: true })
        .click();
      const response = await regenerated;
      expect(response.ok(), `regenerating returned ${response.status()}`).toBe(true);

      await expect(
        dialog.getByText(REGISTRY_TOKEN_ACTIONS.regeneratedDialogTitle, { exact: true }),
      ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
      const secret = await dialog.getByLabel(REGISTRY_TOKEN_ACTIONS.newSecretLabel).inputValue();
      expect(secret, "a new secret should be shown").not.toBe("");
      expect(secret, "the new secret should replace the old one").not.toBe(original);
      await dialog.getByRole("button", { name: REGISTRY_TOKENS.closeButton, exact: true }).click();
      await expect(dialog).toHaveCount(0);
      await expect(tokenRow(page, name)).toBeVisible();
      console.log(`Registry Tokens: regenerated the secret of "${name}"`);
    } finally {
      if (created) await deleteToken(page, name);
    }
  });

  test("deletes a token, after a cancelled first attempt", async ({ page }) => {
    await openRegistryTokens(page);
    const name = tokenName();
    await generateToken(page, name);

    // Cancel keeps it.
    await rowAction(page, name, REGISTRY_TOKENS.deleteMenuItem);
    const confirm = page.getByRole("dialog");
    await expect(
      confirm.getByText(REGISTRY_TOKENS.deleteDialogTitle, { exact: true }),
    ).toBeVisible();
    await confirm.getByRole("button", { name: REGISTRY_TOKENS.cancelButton, exact: true }).click();
    await expect(confirm).toHaveCount(0);
    await expect(tokenRow(page, name)).toBeVisible();

    // Delete removes it, and it stays gone after a reload.
    await deleteToken(page, name);
    await page.reload();
    await openRegistryTokens(page);
    await expect(tokenRow(page, name)).toHaveCount(0);
    console.log(`Registry Tokens: deleted "${name}"`);
  });

  test("generates a service token for a system user", async ({ page }) => {
    await openRegistryTokens(page);
    const serviceTab = page.getByRole("tab", { name: REGISTRY_SERVICE_TOKENS.tab });
    test.skip(
      (await serviceTab.count()) === 0,
      "Service Tokens is shown to admins only; this account does not see it.",
    );
    await serviceTab.click();
    await expect(serviceTab).toHaveAttribute("aria-selected", "true");

    const name = tokenName();
    const user = REGISTRY_SERVICE_TOKEN_INPUT.createdFor;
    let created = false;
    try {
      await page.getByRole("button", { name: REGISTRY_SERVICE_TOKENS.generateButton }).click();
      const dialog = page.getByRole("dialog");
      await expect(
        dialog.getByText(REGISTRY_SERVICE_TOKENS.generateDialogTitle, { exact: true }),
      ).toBeVisible();

      const nameInput = dialog.getByLabel(REGISTRY_TOKENS.nameLabel);
      await nameInput.fill(name);
      await nameInput.blur();

      // A service token needs its system user; Generate stays off until one is
      // chosen.
      const generate = dialog.getByRole("button", { name: REGISTRY_TOKENS.generateButton });
      await expect(generate).toBeDisabled();
      const createdFor = dialog.getByRole("combobox", {
        name: REGISTRY_SERVICE_TOKENS.createdForLabel,
      });
      await createdFor.click();
      await createdFor.fill(user);
      await page.getByRole("option", { name: user, exact: true }).click();
      await expect(createdFor).toHaveValue(user);
      await expect(generate).toBeEnabled();

      const createResponse = page.waitForResponse(
        (r) =>
          r.request().method() === "POST" &&
          /\/projects\/[^/]+\/registry-tokens$/.test(new URL(r.url()).pathname),
        { timeout: LOAD_TIMEOUT_MS },
      );
      await generate.click();
      const response = await createResponse;
      expect(response.ok(), `creating the token returned ${response.status()}`).toBe(true);
      expect(response.request().postDataJSON()).toMatchObject({
        robotName: name,
        createdFor: user,
      });
      created = true;

      await expect(
        dialog.getByText(REGISTRY_TOKENS.createdDialogTitle, { exact: true }),
      ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
      await expect(dialog.getByLabel(REGISTRY_TOKENS.secretLabel)).not.toHaveValue("");
      await dialog.getByRole("button", { name: REGISTRY_TOKENS.closeButton, exact: true }).click();
      await expect(dialog).toHaveCount(0);

      // Listed under Service Tokens, against its system user.
      const row = tokenRow(page, name);
      await expect(row).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
      await expect(row).toContainText(user);
      console.log(`Registry Tokens: generated service token "${name}" for ${user}`);
    } finally {
      if (created) await deleteToken(page, name);
    }
  });
});
