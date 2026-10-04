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

import { describe, expect, it, vi } from "vitest";
import {
  CommentPartiallySentError,
  sendCommentWithAttachments,
} from "@features/csm-cases/components/commentSend";

const file = (n: string) => ({ file: new File(["x"], n), name: n });

describe("sendCommentWithAttachments", () => {
  it("posts the comment once and uploads every file", async () => {
    const postComment = vi.fn().mockResolvedValue(undefined);
    const postAttachment = vi.fn().mockResolvedValue(undefined);
    await sendCommentWithAttachments("<p>hi</p>", false, [file("a"), file("b")], {
      postComment,
      postAttachment,
    });
    expect(postComment).toHaveBeenCalledTimes(1);
    expect(postAttachment).toHaveBeenCalledTimes(2);
  });

  it("skips the comment POST for an attachment-only send", async () => {
    const postComment = vi.fn();
    const postAttachment = vi.fn().mockResolvedValue(undefined);
    await sendCommentWithAttachments("<p>&nbsp;</p>", false, [file("a")], {
      postComment,
      postAttachment,
    });
    expect(postComment).not.toHaveBeenCalled();
  });

  it("after a failed upload, a retry with the remaining files posts exactly one comment", async () => {
    const postComment = vi.fn().mockResolvedValue(undefined);
    const postAttachment = vi
      .fn()
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(new Error("413"))
      .mockResolvedValue(undefined);
    const files = [file("a"), file("b"), file("c")];

    let partial: CommentPartiallySentError | undefined;
    try {
      await sendCommentWithAttachments("<p>hi</p>", false, files, {
        postComment,
        postAttachment,
      });
    } catch (e) {
      partial = e as CommentPartiallySentError;
    }
    expect(partial).toBeInstanceOf(CommentPartiallySentError);
    expect(partial!.remaining.map((f) => f.name)).toEqual(["b", "c"]);

    // The composer clears the body and keeps only `remaining`.
    await sendCommentWithAttachments("", false, partial!.remaining, {
      postComment,
      postAttachment,
    });
    expect(postComment).toHaveBeenCalledTimes(1);
    expect(postAttachment).toHaveBeenCalledTimes(4);
  });

  it("rethrows the plain error when nothing was sent yet", async () => {
    const err = new Error("boom");
    await expect(
      sendCommentWithAttachments("", false, [file("a")], {
        postComment: vi.fn(),
        postAttachment: vi.fn().mockRejectedValue(err),
      }),
    ).rejects.toBe(err);
  });
});
