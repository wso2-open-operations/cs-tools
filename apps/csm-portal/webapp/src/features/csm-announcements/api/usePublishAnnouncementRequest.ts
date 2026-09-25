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

import { useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useAddTagToCase } from "@features/csm-cases/api/useCaseTags";
import { usePostCsmCase } from "@features/csm-cases/api/usePostCsmCase";
import { SECURITY_ANNOUNCEMENT_TAG_LABEL } from "@features/csm-announcements/components/CreateCustomerAnnouncementForm";
import {
  ANNOUNCEMENT_CASE_CREATE_CONCURRENCY_LIMIT,
  settleWithConcurrencyLimit,
} from "@features/csm-announcements/utils/settleWithConcurrencyLimit";
import { useListAnnouncementRequestDeliveries } from "@features/csm-announcements/api/useListAnnouncementRequestDeliveries";
import { useRecordAnnouncementRequestDeliveries } from "@features/csm-announcements/api/useRecordAnnouncementRequestDeliveries";
import type {
  AnnouncementRequest,
  RecordAnnouncementRequestDeliveryEntry,
} from "@features/csm-announcements/types/announcementRequests";

export interface PublishProgress {
  completed: number;
  total: number;
}

export interface UsePublishAnnouncementRequestResult {
  /** True while a fan-out (initial attempt or retry) is in flight. */
  publishing: boolean;
  /** Live counter for the current fan-out; `null` when nothing is in flight. */
  progress: PublishProgress | null;
  /** Every resolved project id that has a real case so far, across all attempts. */
  succeededProjectIds: string[];
  /** Only the projects still outstanding after the most recent attempt — call `handlePublish` again to retry just these. */
  failedProjectIds: string[];
  /**
   * Projects whose case was created but the mandatory security-announcement
   * tag failed to attach. Non-empty blocks `handlePublish` from reaching the
   * final `/publish` call for a security announcement — a case that's
   * missing its security tag once the request is `published` (terminal,
   * no further edits or sends) has no in-app way to fix afterward. Calling
   * `handlePublish` again retries just the tag attach for these, using the
   * case already created — it never creates another one.
   */
  failedTagProjectIds: string[];
  /** Set once every resolved project has a case and the backend has marked the request published. */
  published: AnnouncementRequest | null;
  handlePublish: () => Promise<void>;
  /**
   * Marks the request published using only the projects that have already
   * succeeded, permanently giving up on whatever's still in
   * `failedProjectIds` — for when one or more projects can never be
   * delivered to (a broken/invalid project id, an account that will never
   * accept a case) and waiting for a retry that will never succeed is
   * blocking every already-succeeded customer from being reachable via
   * "Post an update". The backend's own `/publish` call has no "every
   * project must have succeeded" requirement of its own — that rule lives
   * entirely in `handlePublish` above, so this is a deliberate, explicit
   * bypass of it, not a workaround. Requires at least one succeeded
   * project and, unlike `handlePublish`, does *not* retry or wait on
   * `failedProjectIds` at all — they keep their last-recorded "failed"
   * ledger status. Still refuses while `failedTagProjectIds` is non-empty:
   * a missing mandatory security tag is a content-completeness guarantee,
   * not just a delivery attempt count, so it can't be bypassed the same
   * way.
   */
  publishGivingUpOnFailed: () => Promise<void>;
  /**
   * False while `request` is `approved` and the delivery ledger hasn't been
   * loaded (and folded into local state) yet for this request id —
   * `handlePublish` refuses to run in that window, since `succeededProjectIds`
   * would still be empty and every already-succeeded project would be sent a
   * duplicate case. True for any other state (nothing to gate) or once
   * hydration has completed.
   */
  readyToPublish: boolean;
  /** True while ledger hydration is still loading for the current request. */
  hydratingDeliveries: boolean;
  /** True once ledger hydration has failed for the current request — call `retryHydration` to try again. */
  hydrationFailed: boolean;
  retryHydration: () => void;
}

/**
 * The real "send" for an `approved` announcement request: creates one real
 * case per id in `resolvedProjectIds` (the audience snapshot frozen at
 * submit time — see that field's own doc comment), then marks the request
 * `published` once every project has succeeded. This is the same
 * `settleWithConcurrencyLimit` fan-out `CreateCustomerAnnouncementForm`/
 * `CreateEolAnnouncementForm` already use for their own immediate-send
 * button (see those components' `handleSubmit`), pulled out here so the
 * pending-request dialog's Publish action gets the identical mechanism
 * rather than a re-implementation that could drift. Those two forms keep
 * their own copy until they're changed to create drafts instead of sending
 * immediately (a later PR) — this hook isn't extracted *from* them yet, just
 * built to the same shape so that extraction is a pure move once it happens.
 *
 * Unlike those forms' one-shot "succeeded or list the failures" ending, this
 * tracks cumulative success across attempts, so calling `handlePublish` again
 * after a partial failure only resends to the projects still outstanding —
 * already-succeeded projects are never sent a duplicate case.
 *
 * Every per-project outcome from each fan-out pass (initial attempt or
 * retry) is also recorded to a durable backend ledger
 * (`useRecordAnnouncementRequestDeliveries` — see entity-service's
 * `announcement_request_deliveries` table for the full "why"), and on
 * mount this hook reads that same ledger back
 * (`useListAnnouncementRequestDeliveries`) to seed its local state — so if
 * the dialog is closed mid-retry and reopened, `handlePublish` resumes from
 * exactly where it left off instead of resending a case to every resolved
 * project again. Hydration happens once (guarded by `hydratedRef`): a
 * background refetch of the deliveries list must never clobber progress
 * this hook's own fan-out has already made locally since that first load.
 */
export function usePublishAnnouncementRequest(
  request: AnnouncementRequest | null | undefined,
): UsePublishAnnouncementRequestResult {
  const api = useBackendApi();
  const queryClient = useQueryClient();
  const { showError } = useErrorBanner();
  const postCase = usePostCsmCase();
  const addTag = useAddTagToCase();
  const recordDeliveries = useRecordAnnouncementRequestDeliveries();
  // approved: the state a fan-out actually runs in. published: so a
  // just-completed request's dialog can still show its own final ledger —
  // handlePublish itself is a no-op by then (see its own early return).
  const deliveriesQuery = useListAnnouncementRequestDeliveries(
    request?.id,
    request?.state === "approved" || request?.state === "published",
  );

  const [publishing, setPublishing] = useState(false);
  const [progress, setProgress] = useState<PublishProgress | null>(null);
  const [succeededProjectIds, setSucceededProjectIds] = useState<string[]>([]);
  // The real case id created for each succeeded project, across every
  // attempt — this is what gets sent to /publish's caseIds so a later
  // "add update" fan-out (usePostAnnouncementUpdateComments) knows exactly
  // which cases to target, without re-deriving them.
  const [caseIdByProjectId, setCaseIdByProjectId] = useState<Record<string, string>>({});
  const [failedProjectIds, setFailedProjectIds] = useState<string[]>([]);
  const [failedTagProjectIds, setFailedTagProjectIds] = useState<string[]>([]);
  // The case id created for each project in failedTagProjectIds, so a retry
  // can call addTag directly on the case that already exists instead of
  // going through the case-create fan-out again (which would send a
  // duplicate case to a project that already has one).
  const [failedTagCaseIds, setFailedTagCaseIds] = useState<Record<string, string>>({});
  const [published, setPublished] = useState<AnnouncementRequest | null>(null);

  // Seeds local state from the persisted ledger exactly once per request id
  // — see this hook's own doc comment for why a later background refetch
  // must not re-run this. Gated on `isSuccess`, not just `data` being
  // truthy: an errored fetch must never be treated as "hydrated" (that
  // would leave succeededProjectIds empty and let handlePublish resend a
  // case to every project, including ones that already succeeded in an
  // earlier session).
  const [hydratedRequestId, setHydratedRequestId] = useState<string | null>(null);
  const hydratedRequestIdRef = useRef<string | null>(null);
  useEffect(() => {
    if (!request?.id || !deliveriesQuery.isSuccess) return;
    if (hydratedRequestIdRef.current === request.id) return;
    hydratedRequestIdRef.current = request.id;
    setHydratedRequestId(request.id);

    const succeeded: string[] = [];
    const caseIds: Record<string, string> = {};
    const failed: string[] = [];
    const failedTags: string[] = [];
    const failedTagCases: Record<string, string> = {};

    for (const d of deliveriesQuery.data?.deliveries ?? []) {
      if (d.status === "succeeded") {
        succeeded.push(d.projectId);
        if (d.caseId) caseIds[d.projectId] = d.caseId;
      } else if (d.status === "tag_failed") {
        // The case is real either way — must not be re-created by a
        // future fan-out pass, only its tag retried (see failedTagCaseIds).
        succeeded.push(d.projectId);
        failedTags.push(d.projectId);
        if (d.caseId) {
          caseIds[d.projectId] = d.caseId;
          failedTagCases[d.projectId] = d.caseId;
        }
      } else {
        failed.push(d.projectId);
      }
    }

    if (succeeded.length > 0) {
      setSucceededProjectIds(succeeded);
      setCaseIdByProjectId(caseIds);
    }
    if (failed.length > 0) setFailedProjectIds(failed);
    if (failedTags.length > 0) {
      setFailedTagProjectIds(failedTags);
      setFailedTagCaseIds(failedTagCases);
    }
  }, [request?.id, deliveriesQuery.data, deliveriesQuery.isSuccess]);

  const readyToPublish =
    !request || request.state !== "approved" || hydratedRequestId === request.id;
  const hydratingDeliveries = !readyToPublish && !deliveriesQuery.isError;
  const hydrationFailed = !readyToPublish && deliveriesQuery.isError;
  const retryHydration = (): void => {
    void deliveriesQuery.refetch();
  };

  /**
   * Upserts this pass's outcomes to the durable ledger. Best-effort: a
   * failure here is logged (via showError, non-blocking) but never
   * re-throws — the real cases already exist or don't regardless of
   * whether this bookkeeping call itself succeeds, the same "don't imply
   * the send needs retrying" reasoning the final /publish call below
   * already uses for its own failure.
   */
  const recordDeliveryOutcomes = async (entries: RecordAnnouncementRequestDeliveryEntry[]): Promise<void> => {
    if (!request || entries.length === 0) return;
    try {
      await recordDeliveries.mutateAsync({ id: request.id, payload: { deliveries: entries } });
    } catch {
      showError(
        "Sent, but this progress couldn't be saved — if you close this dialog before finishing, you may need to resend to every project on reopen.",
      );
    }
  };

  const handlePublish = async (): Promise<void> => {
    if (!request || request.state !== "approved" || publishing) return;
    if (!readyToPublish) return;

    const allProjectIds = request.resolvedProjectIds ?? [];
    if (allProjectIds.length === 0) {
      showError("This request has no resolved audience to publish to.");
      return;
    }
    const pendingProjectIds = allProjectIds.filter((id) => !succeededProjectIds.includes(id));

    setPublishing(true);

    // Retry any security-tag attach that failed on an *earlier* call first —
    // the case already exists (see failedTagCaseIds), so this calls addTag
    // directly rather than going through the case-create fan-out again
    // (which would send a duplicate case to a project that already has
    // one). This is attempted once per handlePublish call, same as the
    // case-create fan-out below never retries its own failures within the
    // same call — a tag that fails again here just blocks, it isn't looped.
    if (request.isSecurityAnnouncement && failedTagProjectIds.length > 0) {
      const stillFailingTags: string[] = [];
      const tagRetryEntries: RecordAnnouncementRequestDeliveryEntry[] = [];
      for (const projectId of failedTagProjectIds) {
        const caseId = failedTagCaseIds[projectId];
        if (!caseId) continue;
        try {
          await addTag.mutateAsync({ caseId, label: SECURITY_ANNOUNCEMENT_TAG_LABEL });
          tagRetryEntries.push({ projectId, caseId, status: "succeeded" });
        } catch {
          stillFailingTags.push(projectId);
          tagRetryEntries.push({ projectId, caseId, status: "tag_failed" });
        }
      }
      setFailedTagProjectIds(stillFailingTags);
      await recordDeliveryOutcomes(tagRetryEntries);

      if (stillFailingTags.length > 0) {
        setPublishing(false);
        showError(
          `The security label still couldn't be attached for project${
            stillFailingTags.length === 1 ? "" : "s"
          } ${stillFailingTags.join(", ")}. Retry to try again — this won't resend the case${
            stillFailingTags.length === 1 ? "" : "s"
          }.`,
        );
        return;
      }
    }

    // Skip the fan-out entirely when every project already has a case from an
    // earlier attempt — but still fall through to the publish-marking call
    // below, since a retry here is exactly for the case where the fan-out
    // fully succeeded last time but *that* call failed. Returning early
    // instead (as this used to) left that state permanently stuck: nothing
    // was ever outstanding to retry, yet the request was never marked
    // published either.
    // Case ids for THIS call's /publish body: whatever succeeded in earlier
    // attempts (the closure's own caseIdByProjectId, read once at call
    // start same as succeededProjectIds above) plus whatever the fan-out
    // below adds — never read back from state after setting it, since
    // React state updates aren't visible synchronously within this same
    // function body.
    const caseIdsForPublish: Record<string, string> = { ...caseIdByProjectId };

    if (pendingProjectIds.length > 0) {
      setProgress({ completed: 0, total: pendingProjectIds.length });
      const newlyFailedTagIds: string[] = [];
      const newlyFailedTagCaseIds: Record<string, string> = {};
      const passEntries: RecordAnnouncementRequestDeliveryEntry[] = [];

      const results = await settleWithConcurrencyLimit(
        pendingProjectIds,
        ANNOUNCEMENT_CASE_CREATE_CONCURRENCY_LIMIT,
        async (projectId) => {
          const created = await postCase.mutateAsync({
            type: "announcement",
            projectId,
            subject: request.subject,
            description: request.description,
            isSecurityAnnouncement: request.isSecurityAnnouncement,
          });
          // The case genuinely exists the moment postCase succeeds,
          // independent of whether the security tag below then fails —
          // tracked unconditionally so a later tag-only retry (which
          // reuses this same case rather than recreating it) still has it.
          caseIdsForPublish[projectId] = created.id;
          if (request.isSecurityAnnouncement) {
            try {
              await addTag.mutateAsync({ caseId: created.id, label: SECURITY_ANNOUNCEMENT_TAG_LABEL });
            } catch {
              newlyFailedTagIds.push(projectId);
              newlyFailedTagCaseIds[projectId] = created.id;
              passEntries.push({ projectId, caseId: created.id, status: "tag_failed" });
              return created;
            }
          }
          passEntries.push({ projectId, caseId: created.id, status: "succeeded" });
          return created;
        },
        (completed, total) => setProgress({ completed, total }),
      );

      const newlySucceeded = pendingProjectIds.filter((_, i) => results[i].status === "fulfilled");
      const stillFailing = pendingProjectIds.filter((_, i) => results[i].status === "rejected");
      for (const projectId of stillFailing) {
        passEntries.push({ projectId, status: "failed" });
      }

      setSucceededProjectIds((prev) => [...prev, ...newlySucceeded]);
      setCaseIdByProjectId((prev) => ({ ...prev, ...caseIdsForPublish }));
      setFailedProjectIds(stillFailing);
      // failedTagProjectIds is empty at this point (either there was nothing
      // to retry above, or the retry pass fully succeeded), so this is a
      // plain assignment, not a merge with what was just retried away.
      setFailedTagProjectIds(newlyFailedTagIds);
      setFailedTagCaseIds((prev) => ({ ...prev, ...newlyFailedTagCaseIds }));
      setProgress(null);
      await recordDeliveryOutcomes(passEntries);

      if (stillFailing.length > 0) {
        setPublishing(false);
        showError(
          `Sent to ${newlySucceeded.length} of ${pendingProjectIds.length} remaining project${
            pendingProjectIds.length === 1 ? "" : "s"
          } — failed for project${stillFailing.length === 1 ? "" : "s"} ${stillFailing.join(
            ", ",
          )}. Retry to resend just those.`,
        );
        return;
      }

      // A security announcement must not reach the terminal `published`
      // state (no further edits or sends possible after) with any case
      // still missing its security tag — block here and let the next call
      // retry just the tag (handled by the retry pass at the top of this
      // function), never re-creating the case.
      if (newlyFailedTagIds.length > 0) {
        setPublishing(false);
        showError(
          `Sent to ${newlySucceeded.length} of ${pendingProjectIds.length} remaining project${
            pendingProjectIds.length === 1 ? "" : "s"
          }, but the security label couldn't be attached for project${
            newlyFailedTagIds.length === 1 ? "" : "s"
          } ${newlyFailedTagIds.join(", ")}. Retry to try again — this won't resend the case${
            newlyFailedTagIds.length === 1 ? "" : "s"
          }.`,
        );
        return;
      }
    }

    try {
      const result = await api.post<{ caseIds: string[] }, AnnouncementRequest>(
        `/announcement-requests/${encodeURIComponent(request.id)}/publish`,
        { caseIds: Object.values(caseIdsForPublish) },
      );
      setPublished(result);
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUEST_DETAIL, request.id],
      });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUESTS_SEARCH] });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ANNOUNCEMENTS] });
    } catch {
      // The cases are real and already sent either way — only the request's
      // own bookkeeping row failed to flip, so this must not imply the send
      // itself needs retrying (it would duplicate every case).
      showError(
        "Every project received the announcement, but marking the request published failed. Try again — it won't resend the cases.",
      );
    } finally {
      setPublishing(false);
    }
  };

  const publishGivingUpOnFailed = async (): Promise<void> => {
    if (!request || request.state !== "approved" || publishing) return;
    if (!readyToPublish) return;
    if (failedTagProjectIds.length > 0) return;
    if (succeededProjectIds.length === 0) return;

    setPublishing(true);
    try {
      const result = await api.post<{ caseIds: string[] }, AnnouncementRequest>(
        `/announcement-requests/${encodeURIComponent(request.id)}/publish`,
        { caseIds: Object.values(caseIdByProjectId) },
      );
      setPublished(result);
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUEST_DETAIL, request.id],
      });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUESTS_SEARCH] });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ANNOUNCEMENTS] });
    } catch {
      // Same reasoning as handlePublish's own catch: the succeeded cases are
      // real either way, only the request's own bookkeeping row failed to
      // flip — retrying this call never resends anything.
      showError(
        "Marking the request published failed. Try again — it won't resend any cases.",
      );
    } finally {
      setPublishing(false);
    }
  };

  return {
    publishing,
    progress,
    succeededProjectIds,
    failedProjectIds,
    failedTagProjectIds,
    published,
    handlePublish,
    publishGivingUpOnFailed,
    readyToPublish,
    hydratingDeliveries,
    hydrationFailed,
    retryHydration,
  };
}
