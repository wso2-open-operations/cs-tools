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

import { useEffect, useMemo, useState } from "react";
import { useBackendApi } from "@api/backend/client";
import type { BeProjectSearchPayload, BeProjectSearchResponse } from "@api/backend/types";
import { DRY_RUN_TEST_PROJECT_KEY } from "@config/announcementDryRunConfig";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useAddTagToCase } from "@features/csm-cases/api/useCaseTags";
import { usePostCsmCase } from "@features/csm-cases/api/usePostCsmCase";

/**
 * Fixed tag attached to every dry-run case, regardless of which flow ran it,
 * so it's identifiable (and, in a later pass, excludable from customer-facing
 * counts/registry views the way the ServiceNow process already marks its own
 * dry-run cases) — see DRY_RUN_TEST_PROJECT_KEY's own doc comment for the
 * project side of this. "Dry Run" rather than an invented phrase: the
 * ServiceNow flow this replaces is itself literally named "DRY RUN - Create
 * Announcements in All Customer Projects," so this keeps the same term
 * engineers already associate with this step.
 */
export const DRY_RUN_TAG_LABEL = "Dry Run";

/** The rich-text editor emits `<p></p>` when empty; check the stripped text. */
function isEmptyHtml(html: string): boolean {
  return html.replace(/<[^>]*>/g, "").replace(/&nbsp;/g, " ").trim().length === 0;
}

export interface DryRunResult {
  caseId: string;
  displayId: string;
}

export interface UseAnnouncementDryRunArgs {
  subject: string;
  description: string;
  /** Every tag attached to the dry-run case once it's created, e.g. `["Dry Run"]` or `["Dry Run", "Security Announcement"]`. */
  tagLabels: string[];
  /** Extra guard on top of the built-in subject/description non-empty check, e.g. "a product version is selected." */
  extraCanRun?: boolean;
}

export interface UseAnnouncementDryRun {
  runningDryRun: boolean;
  dryRunResult: DryRunResult | null;
  canRunDryRun: boolean;
  /** Resolves with the created result directly (not just via `dryRunResult` state) — lets a caller chain further work off it without waiting on a re-render, e.g. the create forms' "Submit for approval" orchestration. `null` on any failure or when the guard blocks it. */
  handleRunDryRun: () => Promise<DryRunResult | null>;
}

/**
 * Shared "dry run" mechanism for every announcement create flow (Option 1
 * "Create announcement for customers" and Option 2 "Product version / EOL
 * announcement" alike): creates exactly one real case, tagged with the
 * caller's given labels, in a single fixed test project (see
 * DRY_RUN_TEST_PROJECT_KEY) — mirroring the ServiceNow process's own
 * `Project Key = DCPSUB` dry-run step (that flow is literally named
 * "DRY RUN - Create Announcements in All Customer Projects," which is why
 * this keeps the same term rather than a paraphrase like "send test") — so
 * an engineer can open the real case and check formatting/rendering before
 * sending to actual customer projects. It's independent of whichever
 * audience mechanism the calling form uses: no project needs to be picked
 * or resolved to dry-run, since the test project is fixed regardless.
 *
 * Extracted out of CreateCustomerAnnouncementForm so every announcement flow
 * gets the exact same dry-run behavior and UI (see AnnouncementDryRunCard)
 * rather than a re-implementation that could visually or behaviorally drift.
 */
export function useAnnouncementDryRun({
  subject,
  description,
  tagLabels,
  extraCanRun = true,
}: UseAnnouncementDryRunArgs): UseAnnouncementDryRun {
  const { showError } = useErrorBanner();
  const api = useBackendApi();
  const postCase = usePostCsmCase();
  const addTag = useAddTagToCase();

  const [runningDryRun, setRunningDryRun] = useState(false);
  const [dryRunResult, setDryRunResult] = useState<DryRunResult | null>(null);

  // A dry-run confirmation is scoped to the content it was actually run
  // against — clear it once the draft changes so the "view dry run" link
  // never implies it reflects content the requester has since edited.
  // tagLabels counts as "content" too: e.g. toggling the security checkbox
  // after running a dry run changes which labels a real send would attach,
  // so a stale confirmation from before the toggle must not linger as if it
  // still reflected the current label set. Callers pass a memoized array
  // (see CreateCustomerAnnouncementForm's dryRunTagLabels), so this doesn't
  // re-run on every render.
  useEffect(() => {
    setDryRunResult(null);
  }, [subject, description, tagLabels]);

  const canRunDryRun = useMemo(
    () =>
      subject.trim().length > 0 &&
      !isEmptyHtml(description) &&
      extraCanRun &&
      !runningDryRun,
    [subject, description, extraCanRun, runningDryRun],
  );

  const handleRunDryRun = async (): Promise<DryRunResult | null> => {
    if (!canRunDryRun) return null;
    setRunningDryRun(true);
    setDryRunResult(null);

    try {
      const searchRes = await api.post<BeProjectSearchPayload, BeProjectSearchResponse>(
        "/projects/search",
        { searchQuery: DRY_RUN_TEST_PROJECT_KEY, pagination: { offset: 0, limit: 10 } },
      );
      const testProject = (searchRes.projects ?? []).find(
        (p) => p.key?.toLowerCase() === DRY_RUN_TEST_PROJECT_KEY.toLowerCase(),
      );
      if (!testProject) {
        showError(
          `Could not find the test project "${DRY_RUN_TEST_PROJECT_KEY}". Check the CSM_PORTAL_ANNOUNCEMENT_TEST_PROJECT_KEY configuration.`,
        );
        return null;
      }

      const created = await postCase.mutateAsync({
        type: "announcement",
        projectId: testProject.id,
        subject: subject.trim(),
        description,
      });
      // Best-effort: the dry-run case already exists even if a tag fails to
      // attach, so a tag failure here doesn't block reporting success — same
      // "the case is the source of truth, not the tag" reasoning as the real
      // send path. Unlike the real send path, though, a dry run's entire
      // purpose is to let the sender verify labels/formatting before the
      // real thing — silently dropping a label here without saying so would
      // let a dry run report "succeeded" while the exact thing it exists to
      // check (does the label actually attach) quietly didn't happen. So
      // this still reports success (the case is real and viewable either
      // way), but surfaces which label(s) failed, mirroring the real send
      // path's own "create succeeded, but the label couldn't be attached —
      // add it manually" message.
      const tagResults = await Promise.allSettled(
        tagLabels.map((label) => addTag.mutateAsync({ caseId: created.id, label })),
      );
      const failedLabels = tagLabels.filter((_, i) => tagResults[i].status === "rejected");
      if (failedLabels.length > 0) {
        showError(
          `The dry-run case was created, but the label${
            failedLabels.length === 1 ? "" : "s"
          } ${failedLabels.join(", ")} couldn't be attached — add ${
            failedLabels.length === 1 ? "it" : "them"
          } manually before trusting this preview.`,
        );
      }

      const result: DryRunResult = {
        caseId: created.id,
        displayId: created.internalId || created.number || created.id,
      };
      setDryRunResult(result);
      return result;
    } catch {
      showError("Could not run the dry run. Please try again.");
      return null;
    } finally {
      setRunningDryRun(false);
    }
  };

  return { runningDryRun, dryRunResult, canRunDryRun, handleRunDryRun };
}
