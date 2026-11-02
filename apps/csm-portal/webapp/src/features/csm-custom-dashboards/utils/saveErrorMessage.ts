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

import { BackendApiError } from "@api/backend/client";

/**
 * What the Save dialog shows when Save is refused. A name the user already
 * has (409) is fixed in the dialog itself; another refusal the user can act
 * on (4xx, e.g. the 50-dashboard limit) shows the backend's message; anything
 * else is generic.
 */
export function saveErrorMessage(err: unknown, name: string): string {
  if (err instanceof BackendApiError && err.status === 409) {
    return nameTakenMessage(name);
  }
  return refusalMessage(err) ?? "The dashboard couldn't be saved. Try again in a moment.";
}

/**
 * What the rename dialog shows when a rename is refused. A name the user
 * already has (409) is fixed in the dialog itself; another refusal the user
 * can act on (4xx: the dashboard is gone, or was saved under rules that have
 * since changed) shows the backend's message; anything else is generic.
 */
export function renameErrorMessage(err: unknown, name: string): string {
  if (err instanceof BackendApiError && err.status === 409) {
    return nameTakenMessage(name);
  }
  return refusalMessage(err) ?? "The dashboard couldn't be renamed. Try again in a moment.";
}

/** A name the user already has, as both dialogs word it: the fix is in the field above it. */
function nameTakenMessage(name: string): string {
  return `You already have a dashboard named "${name}". Choose a different name.`;
}

/** The backend's message for a 4xx, as a sentence; `null` for anything else. */
function refusalMessage(err: unknown): string | null {
  if (err instanceof BackendApiError && err.status >= 400 && err.status < 500 && err.message) {
    const msg = err.message.charAt(0).toUpperCase() + err.message.slice(1);
    return /[.!?]$/.test(msg) ? msg : `${msg}.`;
  }
  return null;
}
