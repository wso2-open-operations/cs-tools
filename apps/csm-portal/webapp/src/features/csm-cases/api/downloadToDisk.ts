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


import { downloadToDiskEnabled } from "@config/attachmentLimitsConfig";

/**
 * Saving a download straight to a file the person picked, through the File
 * System Access API (Chrome, Edge), so a large attachment is never held whole in
 * the browser's memory. Browsers without it keep the in-memory path.
 */

/** Largest download that is still saved through the in-memory path with no dialog. */
export const SMALL_DOWNLOAD_BYTES = 8 * 1024 * 1024;

interface WritableFile {
  write(chunk: { type: "write"; position: number; data: Blob }): Promise<void>;
  close(): Promise<void>;
  abort(): Promise<void>;
}

interface SaveFileHandle {
  createWritable(options?: { keepExistingData?: boolean }): Promise<WritableFile>;
}

type ShowSaveFilePicker = (options?: { suggestedName?: string }) => Promise<SaveFileHandle>;

/** Where the bytes of one download are written, by position. */
export interface DiskSink {
  /** Writes `parts` at byte `position`. Calls may overlap; they are written one at a time. */
  write(position: number, parts: Uint8Array[]): Promise<void>;
  /** Finishes the file. */
  close(): Promise<void>;
  /** Drops the half-written file. Never throws. */
  abort(): Promise<void>;
}

export type DiskTarget =
  | { kind: "sink"; sink: DiskSink }
  /** The person closed the save dialog: nothing is downloaded and nothing is shown. */
  | { kind: "cancelled" }
  /** No disk target (no API, switched off, small file, dialog refused): use the in-memory path. */
  | { kind: "unavailable" };

export const DISK_WRITE_FAILED_MESSAGE = "The file could not be saved to disk. Please try again.";

function isDismissed(error: unknown): boolean {
  return (
    typeof error === "object" && error !== null && (error as { name?: unknown }).name === "AbortError"
  );
}

/**
 * Opens the save dialog for `name`. It must run before the first request of the
 * download, while the click is still fresh: a browser only allows the dialog
 * for a few seconds after the click. `size` is the file's size when the caller
 * knows it; a file of {@link SMALL_DOWNLOAD_BYTES} or less skips the dialog.
 */
export async function openDiskTarget(name: string, size?: number): Promise<DiskTarget> {
  const picker = (window as unknown as { showSaveFilePicker?: ShowSaveFilePicker }).showSaveFilePicker;
  if (!downloadToDiskEnabled() || typeof picker !== "function") return { kind: "unavailable" };
  if (size !== undefined && size <= SMALL_DOWNLOAD_BYTES) return { kind: "unavailable" };

  let writable: WritableFile;
  try {
    const handle = await picker.call(window, { suggestedName: name });
    writable = await handle.createWritable({ keepExistingData: false });
  } catch (error) {
    // A closed dialog is a decision; anything else (no click any more, no
    // permission) just means this browser cannot do it now.
    return isDismissed(error) ? { kind: "cancelled" } : { kind: "unavailable" };
  }

  let queue: Promise<void> = Promise.resolve();
  return {
    kind: "sink",
    sink: {
      write(position, parts) {
        const next = queue.then(() =>
          writable.write({ type: "write", position, data: new Blob(parts as BlobPart[]) }),
        );
        queue = next.catch(() => undefined);
        return next.catch(() => {
          throw new Error(DISK_WRITE_FAILED_MESSAGE);
        });
      },
      close() {
        return queue
          .then(() => writable.close())
          .catch(() => {
            throw new Error(DISK_WRITE_FAILED_MESSAGE);
          });
      },
      async abort() {
        try {
          await writable.abort();
        } catch {
          // Already closed or aborted: nothing left to drop.
        }
      },
    },
  };
}
