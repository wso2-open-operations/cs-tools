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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

/** Why a new tag was not added: the server's own words for a refusal the lead
 *  can act on (400: the name or a choice is wrong, or which family it is for;
 *  403; 409: the name is taken), else a plain retry line. The error is read by
 *  its shape (status and message), as the backend client raises it. */
export function addKindRefusal(err: unknown): string {
  const { status, reason } = serverReason(err, [400, 403, 409]);
  if (reason) return reason;
  if (status === 403) return "Only a team lead can add a tag.";
  return "The tag was not added. Try again.";
}

/** Why a tag was not deleted: the server's own words for a 403 (whose tag it
 *  is: another family's, or a shared one only a rota admin may delete) or a
 *  409 (how many entries still use it), else a plain line for that status. */
export function deleteKindRefusal(err: unknown): string {
  const { status, reason } = serverReason(err, [403, 409]);
  if (reason) return reason;
  if (status === 409) return "That tag is still used on the rota. Remove those entries first.";
  if (status === 403) return "You cannot delete this tag. A built-in tag can never be deleted.";
  return "The tag was not deleted. Try again.";
}

/** The error's status, and its message as a sentence when the status is one
 *  whose message is written for the person acting. */
function serverReason(err: unknown, statuses: number[]): { status?: number; reason: string } {
  const e = err as { status?: unknown; message?: unknown } | null;
  const status = typeof e?.status === "number" ? e.status : undefined;
  const message = typeof e?.message === "string" ? e.message.trim() : "";
  if (status === undefined || !statuses.includes(status) || !message) return { status, reason: "" };
  return { status, reason: `${message.charAt(0).toUpperCase()}${message.slice(1)}${/[.!?]$/.test(message) ? "" : "."}` };
}
