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
// Multi-step case flows shared by more than one spec. These sit above the page
// objects: they drive a whole user journey and assert it succeeded, so a spec
// that merely needs a case to exist can call one instead of restating every
// step.
//

import {
  expect,
  test,
  type Page,
  type Response,
} from "../fixtures/test";
import { CaseCreatePage } from "../pages/CaseCreatePage";
import type { CaseInput, ProjectFixture } from "../config/testData";
import { CREATE_CASE } from "./selectors";
import { idPattern, projectPathPattern } from "./ids";

/** A case as identified by the API response that created it. */
export interface CreatedCase {
  id?: string;
  number?: string;
}

/**
 * Whether a response status means the mutation actually succeeded.
 *
 * Deliberately 2xx-only rather than `< 400`: a 3xx is not a completed write, and
 * matching one would hand the caller a redirect to parse as JSON.
 *
 * @param status - HTTP status code.
 * @returns True for 200-299.
 */
export function isSuccess(status: number): boolean {
  return status >= 200 && status < 300;
}

/**
 * Asserts a mutation succeeded, quoting the server's message when it did not.
 *
 * The body is read ONLY on failure. Reading it eagerly — to build the message
 * up front — throws once the page has navigated, because Playwright discards
 * bodies for responses that were navigated away from, and a successful create
 * usually does navigate or refetch.
 *
 * @param response - The mutation's response.
 * @param action - What was attempted, for the failure message.
 */
export async function expectSuccess(
  response: Response,
  action: string,
): Promise<void> {
  const status = response.status();
  if (isSuccess(status)) return;
  const body = await response
    .text()
    .catch(() => "(response body unavailable)");
  expect(isSuccess(status), `${action} failed: ${status} ${body}`).toBe(true);
}

/** Fixture fields a create-case run cannot proceed without. `deployment` is
 * required only when the form actually offers the field — types that
 * auto-select it legitimately leave it empty. */
export function missingFields(project: ProjectFixture): string[] {
  const required: (keyof ProjectFixture)[] = ["id", "productVersion"];
  if (!project.autoSelectsDeployment) required.push("deployment");
  return required.filter((field) => !project[field]);
}

/**
 * Skips the current test when the project's fixture is incomplete. Without
 * this a run would navigate to /projects//dashboard, or try to pick an option
 * labelled "", and fail with a confusing element-not-found error rather than
 * naming the missing configuration.
 *
 * @param project - Fixture to validate.
 */
export function skipWhenUnconfigured(project: ProjectFixture): void {
  const missing = missingFields(project);
  test.skip(
    missing.length > 0,
    `${project.type} fixture is missing ${missing.join(", ")}. ` +
      `Fill it in tests/e2e/config/testData.ts.`,
  );
}

/**
 * Runs the whole create-case flow from the "Get Help" button and asserts the
 * case was really created.
 *
 * ⚠️ Creates a permanent record — `POST /cases` has no delete counterpart.
 *
 * @param page - Test page.
 * @param project - Project to create the case under.
 * @param caseInput - Case content to submit.
 * @returns The created case's id and number, straight from the API response.
 */
export async function createCaseViaGetHelp(
  page: Page,
  project: ProjectFixture,
  caseInput: CaseInput,
): Promise<CreatedCase> {
  const createCase = new CaseCreatePage(page);

  await createCase.openViaGetHelp(project.id);

  if (project.autoSelectsDeployment) {
    // The form must hide Deployment entirely for this project type and lock it
    // to primary production; asserting it stays hidden is the point, since a
    // regression here would silently widen deployment choice.
    await expect(createCase.deploymentSelect()).toBeHidden();
  } else {
    await createCase.selectDeployment(project.deployment);
  }
  await createCase.selectProductVersion(project.productVersion);
  await createCase.fillTitle(caseInput.title);
  await createCase.fillDescription(caseInput.description);
  await createCase.selectIssueType(caseInput.issueType);
  await createCase.selectSeverity(caseInput.severity);

  await expect(createCase.submitButton()).toBeEnabled();

  // Capture the created case's id from the response so the assertions below
  // prove the backend accepted the case, not just that the UI moved on.
  const [createResponse] = await Promise.all([
    page.waitForResponse(
      (r) =>
        r.url().includes("/cases") &&
        r.request().method() === "POST" &&
        isSuccess(r.status()),
    ),
    createCase.submit(),
  ]);

  const created = (await createResponse.json()) as CreatedCase;
  expect(created.id, "backend returned no case id").toBeTruthy();

  // On success CreateCasePage shows a banner and routes to the new case's
  // detail page (`/projects/:projectId/support/cases/:caseId`).
  await expect(page.getByText(CREATE_CASE.successMessage)).toBeVisible();
  await expect(page).toHaveURL(
    projectPathPattern(project.id, `support/cases/${idPattern(created.id!)}`),
  );

  // The detail page must render the case we just submitted.
  await expect(
    page.getByText(caseInput.title, { exact: false }).first(),
  ).toBeVisible();

  return created;
}
