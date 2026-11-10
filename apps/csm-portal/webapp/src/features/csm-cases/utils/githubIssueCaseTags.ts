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

import type { BeCreateCaseGithubIssuePayload } from "@api/backend/types";

// Case tags applied once a Git issue has been filed from a case.
//
// Tags are free text — the backend finds or creates one by name — so a
// mistyped label is not rejected, it silently becomes a brand-new tag. That is
// why the labels live here, in one tested place, rather than as string
// literals inside the page.
//
// The case filters and dashboard widgets hide a case that already has a Git
// issue by excluding a tag (`tag notIn [patch]`, `tag notIn [s_dip]`), one per
// kind of issue, so a case filed with any other spelling is not excluded from
// them.

/** The issue type the dialog sends for a patch (`issueTypeLabel`). */
export const GITHUB_PATCH_ISSUE_TYPE = "Type/Patch";

/** Applied to a case a Patch Git issue is filed from. */
export const GITHUB_PATCH_ISSUE_CASE_TAG = "patch";

/** Applied to a case any other Git issue (a Discussion) is filed from. */
export const GITHUB_ISSUE_CASE_TAG = "s_dip";

/** Added when the issue is filed as a regression. */
export const GITHUB_ISSUE_REGRESSION_CASE_TAG = "s_rg";

/** Added when the issue is a migration issue. */
export const GITHUB_ISSUE_MIGRATION_CASE_TAG = "migration";

/** The case tags to add after a Git issue is filed with this payload. */
export function githubIssueCaseTags(
  payload: Pick<BeCreateCaseGithubIssuePayload, "regression" | "reason" | "issueTypeLabel">,
): string[] {
  const isPatch = payload.issueTypeLabel?.trim() === GITHUB_PATCH_ISSUE_TYPE;
  const tags = [isPatch ? GITHUB_PATCH_ISSUE_CASE_TAG : GITHUB_ISSUE_CASE_TAG];
  if (payload.regression) tags.push(GITHUB_ISSUE_REGRESSION_CASE_TAG);
  if (payload.reason === "migration") tags.push(GITHUB_ISSUE_MIGRATION_CASE_TAG);
  return tags;
}
