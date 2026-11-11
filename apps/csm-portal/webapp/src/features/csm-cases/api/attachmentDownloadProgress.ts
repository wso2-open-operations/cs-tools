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


import { useSyncExternalStore } from "react";

/**
 * Which attachment downloads are running and how far each has got, so a list
 * row can show it without the page that started the download passing it down.
 * A value is a percentage (0-100), or {@link DOWNLOAD_STARTED} while the size is
 * not known yet; an attachment that is not downloading has no entry.
 */
export const DOWNLOAD_STARTED = -1;

const running = new Map<string, number>();
const listeners = new Set<() => void>();

function emit(): void {
  listeners.forEach((listener) => listener());
}

/** Records a download's progress; `undefined` clears it. */
export function setAttachmentDownloadProgress(id: string, percent: number | undefined): void {
  if (percent === undefined) {
    if (!running.delete(id)) return;
  } else {
    if (running.get(id) === percent) return;
    running.set(id, percent);
  }
  emit();
}

/** True while an attachment is being downloaded. */
export function isAttachmentDownloading(id: string): boolean {
  return running.has(id);
}

/** The percentage of an attachment's running download, {@link DOWNLOAD_STARTED}, or `undefined` when none runs. */
export function useAttachmentDownloadProgress(id: string): number | undefined {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    () => running.get(id),
  );
}

const aborts = new Map<string, AbortController>();

/** Remembers the controller of an attachment's running download so it can be cancelled; `undefined` forgets it. */
export function setAttachmentDownloadAbort(id: string, controller: AbortController | undefined): void {
  if (controller) aborts.set(id, controller);
  else aborts.delete(id);
}

/** Cancels an attachment's running download, if there is one. Nothing is saved and no error is shown. */
export function cancelAttachmentDownload(id: string): void {
  aborts.get(id)?.abort();
}

/** A percentage from bytes received and the total. */
export function downloadPercent(received: number, total: number): number {
  return total > 0 ? Math.min(100, Math.floor((received / total) * 100)) : 0;
}
