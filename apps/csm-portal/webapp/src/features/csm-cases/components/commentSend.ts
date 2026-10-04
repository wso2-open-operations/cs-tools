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

/** A file staged for upload, with the display name chosen in the modal. */
export interface CommentSendAttachment {
  file: File;
  name: string;
}

/**
 * Thrown by a comment-send handler when the comment itself was already
 * created but one of its attachment uploads failed. The composer reads it to
 * clear the body (so a retry never re-posts the comment) and to keep only the
 * files that still need uploading.
 */
export class CommentPartiallySentError extends Error {
  readonly remaining: CommentSendAttachment[];

  constructor(message: string, remaining: CommentSendAttachment[]) {
    super(message);
    this.name = "CommentPartiallySentError";
    this.remaining = remaining;
  }
}

interface SendDeps {
  postComment: (input: {
    bodyHtml: string;
    internal: boolean;
  }) => Promise<unknown>;
  postAttachment: (input: CommentSendAttachment) => Promise<unknown>;
}

function hasVisibleText(bodyHtml: string): boolean {
  return (
    bodyHtml
      .replace(/<[^>]*>/g, "")
      .replace(/&nbsp;/g, " ")
      .trim().length > 0
  );
}

/**
 * Posts the comment (when there is text) and then uploads each attachment
 * sequentially. If an upload fails after the comment exists, throws a
 * {@link CommentPartiallySentError} listing the files not yet uploaded
 * (the failed one included) so the caller's retry uploads only those.
 */
export async function sendCommentWithAttachments(
  bodyHtml: string,
  internal: boolean,
  attachments: CommentSendAttachment[],
  { postComment, postAttachment }: SendDeps,
): Promise<void> {
  const commentPosted = hasVisibleText(bodyHtml);
  if (commentPosted) {
    await postComment({ bodyHtml, internal });
  }
  for (let i = 0; i < attachments.length; i += 1) {
    try {
      await postAttachment(attachments[i]);
    } catch (e) {
      const reason = e instanceof Error ? e.message : "Upload failed.";
      if (!commentPosted && i === 0) throw e;
      const lead = commentPosted ? "Comment posted, but" : "Some files were uploaded, but";
      throw new CommentPartiallySentError(
        `${lead} "${attachments[i].name}" could not be uploaded: ${reason} Send again to upload the remaining files.`,
        attachments.slice(i),
      );
    }
  }
}
