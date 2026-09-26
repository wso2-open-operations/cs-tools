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

import { useState } from "react";
import { usePostCsmCaseComment } from "@features/csm-cases/api/useCsmCaseComments";
import {
  ANNOUNCEMENT_CASE_CREATE_CONCURRENCY_LIMIT,
  settleWithConcurrencyLimit,
} from "@features/csm-announcements/utils/settleWithConcurrencyLimit";

export interface UpdateCommentsProgress {
  completed: number;
  total: number;
}

export interface UsePostAnnouncementUpdateCommentsResult {
  posting: boolean;
  progress: UpdateCommentsProgress | null;
  /** Case ids that got the comment so far, across every attempt — for the
   * *current* update only (see reset()). */
  succeededCaseIds: string[];
  /** Only the case ids still outstanding after the most recent attempt. */
  failedCaseIds: string[];
  done: boolean;
  /** authorName is display-only (see usePostCsmCaseComment's own doc
   * comment — it's never sent upstream), so any caller-supplied string is
   * fine here. */
  handlePost: (caseIds: string[], contentHtml: string, authorName: string) => Promise<void>;
  /**
   * Clears succeededCaseIds/failedCaseIds/done. The caller (see
   * AnnouncementRequestDialog) must call this before starting a genuinely
   * *new* update — this hook instance is reused for the dialog's whole
   * lifetime, so without a reset, posting a second update after the first
   * one fully succeeded would see every case already in succeededCaseIds
   * from the *previous* update and silently skip the fan-out entirely
   * (`done` would flip true having never actually posted the new content).
   * Never call this mid-retry of the *same* update — that would defeat the
   * "only resend to cases still outstanding" tracking this hook exists for.
   */
  reset: () => void;
}

/**
 * The real "send" for an announcement update: posts one real comment per
 * case id in the published request's own publishedCaseIds — the same
 * bounded-concurrency fan-out usePublishAnnouncementRequest already uses
 * for case creation, applied here to comment creation instead. Tracks
 * cumulative success across attempts the same way, so calling handlePost
 * again after a partial failure only resends to the cases still
 * outstanding.
 */
export function usePostAnnouncementUpdateComments(): UsePostAnnouncementUpdateCommentsResult {
  const postComment = usePostCsmCaseComment();

  const [posting, setPosting] = useState(false);
  const [progress, setProgress] = useState<UpdateCommentsProgress | null>(null);
  const [succeededCaseIds, setSucceededCaseIds] = useState<string[]>([]);
  const [failedCaseIds, setFailedCaseIds] = useState<string[]>([]);
  const [done, setDone] = useState(false);

  const handlePost = async (caseIds: string[], contentHtml: string, authorName: string): Promise<void> => {
    if (posting || caseIds.length === 0) return;

    const pending = caseIds.filter((id) => !succeededCaseIds.includes(id));
    if (pending.length === 0) {
      setDone(true);
      return;
    }

    setPosting(true);
    setProgress({ completed: 0, total: pending.length });

    const results = await settleWithConcurrencyLimit(
      pending,
      ANNOUNCEMENT_CASE_CREATE_CONCURRENCY_LIMIT,
      (caseId) => postComment.mutateAsync({ caseId, bodyHtml: contentHtml, authorName }),
      (completed, total) => setProgress({ completed, total }),
    );

    const newlySucceeded = pending.filter((_, i) => results[i].status === "fulfilled");
    const stillFailing = pending.filter((_, i) => results[i].status === "rejected");

    setSucceededCaseIds((prev) => [...prev, ...newlySucceeded]);
    setFailedCaseIds(stillFailing);
    setProgress(null);
    setPosting(false);
    setDone(stillFailing.length === 0);
  };

  const reset = (): void => {
    setSucceededCaseIds([]);
    setFailedCaseIds([]);
    setDone(false);
  };

  return { posting, progress, succeededCaseIds, failedCaseIds, done, handlePost, reset };
}
