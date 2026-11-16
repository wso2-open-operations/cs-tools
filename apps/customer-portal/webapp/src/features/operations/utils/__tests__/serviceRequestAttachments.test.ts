// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import { describe, expect, it, vi } from "vitest";
import {
  uploadServiceRequestAttachments,
  ATTACHMENT_UPLOAD_WAIT_MS,
  type AttachmentUploadItem,
} from "../serviceRequestAttachments";

describe("serviceRequestAttachments", () => {
  it("defines ATTACHMENT_UPLOAD_WAIT_MS as 5000", () => {
    expect(ATTACHMENT_UPLOAD_WAIT_MS).toBe(5000);
  });

  describe("uploadServiceRequestAttachments", () => {
    it("returns empty array and makes no calls when attachments array is empty", async () => {
      const uploadAttachment = vi.fn().mockResolvedValue(undefined);
      const failed = await uploadServiceRequestAttachments({
        caseId: "case-123",
        attachments: [],
        attachmentNames: new Map(),
        uploadAttachment,
      });

      expect(failed).toEqual([]);
      expect(uploadAttachment).not.toHaveBeenCalled();
    });

    it("uploads every file as a work item attachment of the case, with custom names", async () => {
      const uploadAttachment = vi.fn().mockResolvedValue(undefined);
      const file1 = new File(["content1"], "log.txt", { type: "text/plain" });
      const file2 = new File(["content2"], "screenshot.png", { type: "image/png" });

      const attachments: AttachmentUploadItem[] = [
        { id: "att-1", file: file1 },
        { id: "att-2", file: file2 },
      ];
      const attachmentNames = new Map<string, string>([
        ["att-1", "custom_log.txt"],
      ]);

      const failed = await uploadServiceRequestAttachments({
        caseId: "sr-999",
        attachments,
        attachmentNames,
        uploadAttachment,
      });

      expect(failed).toEqual([]);
      expect(uploadAttachment).toHaveBeenCalledTimes(2);
      expect(uploadAttachment).toHaveBeenNthCalledWith(1, {
        type: "work_item",
        parentId: "sr-999",
        caseId: "sr-999",
        file: file1,
        body: { name: "custom_log.txt" },
      });
      expect(uploadAttachment).toHaveBeenNthCalledWith(2, {
        type: "work_item",
        parentId: "sr-999",
        caseId: "sr-999",
        file: file2,
        body: { name: "screenshot.png" },
      });
    });

    it("handles upload rejection, logs error, and returns failed attachment names without aborting others", async () => {
      const logger = { error: vi.fn() };
      const uploadAttachment = vi
        .fn()
        .mockRejectedValueOnce(new Error("Network upload failed"))
        .mockResolvedValueOnce(undefined);

      const file1 = new File(["1"], "fail.pdf", { type: "application/pdf" });
      const file2 = new File(["2"], "success.pdf", { type: "application/pdf" });

      const attachments: AttachmentUploadItem[] = [
        { id: "att-1", file: file1 },
        { id: "att-2", file: file2 },
      ];

      const failed = await uploadServiceRequestAttachments({
        caseId: "sr-100",
        attachments,
        attachmentNames: new Map(),
        uploadAttachment,
        logger,
      });

      expect(failed).toEqual(["fail.pdf"]);
      expect(uploadAttachment).toHaveBeenCalledTimes(2);
      expect(logger.error).toHaveBeenCalledWith(
        "Failed to upload attachment fail.pdf",
        expect.any(Error),
      );
    });
  });
});
