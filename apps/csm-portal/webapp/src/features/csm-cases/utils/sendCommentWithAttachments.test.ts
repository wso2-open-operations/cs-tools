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
  PartialSendError,
  sendCommentWithAttachments,
} from "@features/csm-cases/utils/sendCommentWithAttachments";

const file = (name: string) => ({ file: new File(["x"], name), name });

describe("sendCommentWithAttachments", () => {
  it("posts the comment and uploads every file", async () => {
    const postComment = vi.fn().mockResolvedValue(undefined);
    const uploadAttachment = vi.fn().mockResolvedValue(undefined);
    await sendCommentWithAttachments({
      hasComment: true,
      postComment,
      attachments: [file("a.txt"), file("b.txt")],
      uploadAttachment,
    });
    expect(postComment).toHaveBeenCalledTimes(1);
    expect(uploadAttachment).toHaveBeenCalledTimes(2);
  });

  it("skips the comment for an attachment-only send", async () => {
    const postComment = vi.fn();
    await sendCommentWithAttachments({
      hasComment: false,
      postComment,
      attachments: [file("a.txt")],
      uploadAttachment: vi.fn().mockResolvedValue(undefined),
    });
    expect(postComment).not.toHaveBeenCalled();
  });

  it("still uploads the files when the comment fails, and says the comment was not posted", async () => {
    const uploadAttachment = vi.fn().mockResolvedValue(undefined);
    const a = file("a.txt");
    const error = await sendCommentWithAttachments({
      hasComment: true,
      postComment: vi.fn().mockRejectedValue(new Error("Case is closed.")),
      attachments: [a],
      uploadAttachment,
    }).catch((e: unknown) => e);
    expect(uploadAttachment).toHaveBeenCalledTimes(1);
    expect(error).toBeInstanceOf(PartialSendError);
    const partial = error as PartialSendError;
    expect(partial.commentPosted).toBe(false);
    expect(partial.uploaded).toEqual([a]);
    expect(partial.message).toContain("The comment was not posted (Case is closed.)");
  });

  it("keeps the comment when a file fails, names the file, and goes on with the others", async () => {
    const a = file("a.txt");
    const b = file("b.txt");
    const uploadAttachment = vi
      .fn()
      .mockRejectedValueOnce(new Error("The upload was interrupted."))
      .mockResolvedValueOnce(undefined);
    const error = (await sendCommentWithAttachments({
      hasComment: true,
      postComment: vi.fn().mockResolvedValue(undefined),
      attachments: [a, b],
      uploadAttachment,
    }).catch((e: unknown) => e)) as PartialSendError;
    expect(error.commentPosted).toBe(true);
    expect(error.uploaded).toEqual([b]);
    expect(error.message).toContain('"a.txt": The upload was interrupted.');
  });
});
