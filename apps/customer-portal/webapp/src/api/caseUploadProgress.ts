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


import { useMemo, useSyncExternalStore } from "react";

/**
 * The uploads running for a case and how far each has got, so a case page can
 * show them whichever tab is open and whichever page started them (a file still
 * sending after the create-case page moved on, the Attachments tab's own upload).
 */
export interface CaseUpload {
  key: number;
  caseId: string;
  name: string;
  /** 0-100. */
  percent: number;
  /** Started from a form that shows its own progress (the comment box), so the page's strip leaves it out. */
  inPlace: boolean;
  /** Stops the upload; absent when it cannot be stopped. */
  cancel?: () => void;
}

let uploads: readonly CaseUpload[] = [];
let nextKey = 1;
const listeners = new Set<() => void>();

function publish(next: readonly CaseUpload[]): void {
  uploads = next;
  listeners.forEach((listener) => listener());
}

/** A percentage from bytes sent and the total. */
export function uploadPercent(sent: number, total: number): number {
  return total > 0 ? Math.min(100, Math.floor((sent / total) * 100)) : 0;
}

/**
 * Records an upload of `name` to a case. Call `update` with the bytes sent and
 * `finish` when the upload ends, however it ends.
 */
export function trackCaseUpload(
  caseId: string,
  name: string,
  options: { inPlace?: boolean; cancel?: () => void } = {},
): {
  update: (sent: number, total: number) => void;
  /** Takes the Cancel away: the file is already sent and the upload is being finished. */
  lockCancel: () => void;
  finish: () => void;
} {
  const key = nextKey++;
  publish([
    ...uploads,
    { key, caseId, name, percent: 0, inPlace: options.inPlace === true, cancel: options.cancel },
  ]);
  return {
    update(sent, total) {
      const percent = uploadPercent(sent, total);
      const current = uploads.find((u) => u.key === key);
      if (!current || current.percent === percent) return;
      publish(uploads.map((u) => (u.key === key ? { ...u, percent } : u)));
    },
    lockCancel() {
      if (!uploads.some((u) => u.key === key && u.cancel)) return;
      publish(uploads.map((u) => (u.key === key ? { ...u, cancel: undefined } : u)));
    },
    finish() {
      if (!uploads.some((u) => u.key === key)) return;
      publish(uploads.filter((u) => u.key !== key));
    },
  };
}

/** Stops the running upload with this key, if it can be stopped. */
export function cancelCaseUpload(key: number): void {
  uploads.find((u) => u.key === key)?.cancel?.();
}

/** The uploads running for `caseId`, oldest first. */
export function useCaseUploads(caseId: string): readonly CaseUpload[] {
  const all = useSyncExternalStore(
    (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    () => uploads,
  );
  return useMemo(() => all.filter((u) => u.caseId === caseId), [all, caseId]);
}
