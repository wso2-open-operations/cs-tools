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


import type { CommentAttachmentDraft } from "@features/csm-cases/components/CsmCaseCommentInput";

/**
 * Thrown when the comment and the attachments, which are separate requests, did
 * not all go through. The composer then drops what was stored (the comment when
 * it was posted, the files that were uploaded) and keeps the rest, so pressing
 * Send again repeats only what failed and never posts the comment twice.
 */
export class PartialSendError extends Error {
  commentPosted: boolean;
  uploaded: CommentAttachmentDraft[];
  constructor(message: string, commentPosted: boolean, uploaded: CommentAttachmentDraft[]) {
    super(message);
    this.name = "PartialSendError";
    this.commentPosted = commentPosted;
    this.uploaded = uploaded;
  }
}

/** Whether the person stopped the upload (the upload hook wraps the abort as the error's cause). */
function isCancelled(error: unknown): boolean {
  if (!(error instanceof Error)) return false;
  return error.name === "AbortError" || (error.cause instanceof Error && error.cause.name === "AbortError");
}

function messageOf(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback;
}

/**
 * Sends a comment and its attached files as the separate requests they are,
 * started together: a comment that fails does not hold the files back and a file
 * that fails does not lose the comment. The files go one after another, so a
 * failure names its file. Resolves when everything went through; otherwise
 * throws a {@link PartialSendError} saying what was stored and what was not.
 */
export async function sendCommentWithAttachments(args: {
  /** Whether the comment has text or an image; an attachment-only send skips it. */
  hasComment: boolean;
  postComment: () => Promise<unknown>;
  attachments: CommentAttachmentDraft[];
  uploadAttachment: (attachment: CommentAttachmentDraft) => Promise<unknown>;
  /**
   * Called once the comment is posted, or at the start when there is no comment
   * to post, while the files may still be uploading: nothing is left in the
   * composer to protect, so it can empty itself and let the upload progress show.
   * Not called when the comment failed.
   */
  onCommentSettled?: () => void;
  /** Called before each file with how many are queued behind it. */
  onWaiting?: (waiting: number) => void;
}): Promise<void> {
  const commentJob: Promise<string | undefined> = args.hasComment
    ? args.postComment().then(
        () => {
          args.onCommentSettled?.();
          return undefined;
        },
        (error: unknown) => messageOf(error, "Failed to post comment."),
      )
    : Promise.resolve(args.onCommentSettled?.()).then(() => undefined);
  const uploadJob = (async () => {
    const uploaded: CommentAttachmentDraft[] = [];
    const failures: string[] = [];
    for (const [index, attachment] of args.attachments.entries()) {
      args.onWaiting?.(args.attachments.length - index - 1);
      try {
        await args.uploadAttachment(attachment);
        uploaded.push(attachment);
      } catch (error) {
        // Stopped by the person: the file leaves the queue and is not an error to report.
        if (isCancelled(error)) {
          uploaded.push(attachment);
          continue;
        }
        failures.push(`"${attachment.name}": ${messageOf(error, "the upload failed")}`);
      }
    }
    args.onWaiting?.(0);
    return { uploaded, failures };
  })();
  const [commentError, { uploaded, failures }] = await Promise.all([commentJob, uploadJob]);

  const problems: string[] = [];
  if (commentError) {
    problems.push(
      `The comment was not posted (${commentError}). It is still in the editor; press Send to try again.`,
    );
  }
  if (failures.length > 0) {
    problems.push(
      `${failures.length === 1 ? "An attachment was" : "Some attachments were"} not uploaded: ${failures.join("; ")}. ${
        failures.length === 1 ? "It is" : "They are"
      } still attached; press Send to try again.`,
    );
  }
  if (problems.length > 0) {
    throw new PartialSendError(problems.join(" "), args.hasComment && !commentError, uploaded);
  }
}
