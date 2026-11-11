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

/** The `POST /dashboards/generate` failures whose message (the backend's
 * `generate*` messages) is shown as it is: 400 the prompt or the draft, 413
 * the draft too large, 429 the caller's own limit, 502 an unusable or cut-off
 * reply or the AI assistant unreachable, 503 generation unavailable or the AI
 * side's usage limit, 504 too slow. A 422 is not among them (see
 * generateErrorMessage). */
const BACKEND_MESSAGE_STATUSES: ReadonlySet<number> = new Set([400, 413, 429, 502, 503, 504]);

/** The start of every `errorCode` the backend puts on a generate failure it
 * words itself. A proxy or gateway in front of it can answer JSON with a
 * `message` too; without this mark, that is not the backend's message. */
const BACKEND_ERROR_CODE_PREFIX = "generate_";

/**
 * What the generate chat answers when a message did not give a dashboard.
 *
 * A 422 is the request itself not working (the model declining it, a
 * refusal, or a team it cannot filter by): the chat says so in its own
 * words, "cannot be generated from the input" (or, with a draft on screen,
 * "cannot be modified with your input"), not the model's sentence.
 *
 * Otherwise the backend's own message when it sent one (its JSON body's
 * `message`, marked as the backend's by an `errorCode` starting `generate_`):
 * that says what went wrong and what to do. Without one (a proxy's or
 * gateway's own body, JSON or not), the failure is worded by where it
 * happened, never as the user's input not working:
 *
 * - a proxy in front of the backend answering its own 504 (its error page,
 *   no JSON): too slow; its own 502 / 503: the AI assistant unreachable;
 *   its own 429 (its rate limit): too many requests, wait and try again;
 *   its own 413 (its body size limit, reached only by a draft): the same
 *   words as the backend's own draft-too-large message;
 * - no answer at all (fetch threw: offline, or the connection was dropped,
 *   as a proxy does when its timeout is shorter than the model call's): the
 *   portal got no answer;
 * - a 500, or anything else: something went wrong, creating or (with a
 *   draft on screen) changing the dashboard.
 */
export function generateErrorMessage(err: unknown, editing: boolean): string {
  if (!(err instanceof BackendApiError)) {
    return "The portal didn't get an answer. Check your connection and try again.";
  }
  if (err.status === 422) {
    return editing
      ? "This dashboard cannot be modified with your input."
      : "The dashboard cannot be generated from the input.";
  }
  const fromBackend =
    BACKEND_MESSAGE_STATUSES.has(err.status) &&
    typeof err.payload?.errorCode === "string" &&
    err.payload.errorCode.startsWith(BACKEND_ERROR_CODE_PREFIX);
  const backendMessage = fromBackend && typeof err.payload?.message === "string" ? err.payload.message : undefined;
  if (backendMessage) {
    return backendMessage;
  }
  if (err.status === 504) {
    return "The AI assistant took too long to answer. Try again.";
  }
  if (err.status === 502 || err.status === 503) {
    return "The AI assistant can't be reached right now. Try again in a few minutes.";
  }
  if (err.status === 429) {
    return "Too many requests right now. Wait a minute and try again.";
  }
  if (err.status === 413) {
    return "The dashboard being edited is too large to change in the chat. Start a new one.";
  }
  return editing
    ? "Something went wrong while changing the dashboard. Try again."
    : "Something went wrong while creating the dashboard. Try again.";
}
