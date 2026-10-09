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
  fileToBase64,
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

    it("uploads all attachments with base64 content and custom names", async () => {
      const uploadAttachment = vi.fn().mockResolvedValue(undefined);
      const encodeFile = vi.fn().mockImplementation(async (f: File) => `base64-${f.name}`);
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
        encodeFile,
      });

      expect(failed).toEqual([]);
      expect(encodeFile).toHaveBeenCalledTimes(2);
      expect(uploadAttachment).toHaveBeenCalledTimes(2);
      expect(uploadAttachment).toHaveBeenNthCalledWith(1, {
        caseId: "sr-999",
        body: {
          name: "custom_log.txt",
          type: "text/plain",
          content: "base64-log.txt",
        },
      });
      expect(uploadAttachment).toHaveBeenNthCalledWith(2, {
        caseId: "sr-999",
        body: {
          name: "screenshot.png",
          type: "image/png",
          content: "base64-screenshot.png",
        },
      });
    });

    it("handles upload rejection, logs error, and returns failed attachment names without aborting others", async () => {
      const logger = { error: vi.fn() };
      const uploadAttachment = vi
        .fn()
        .mockRejectedValueOnce(new Error("Network upload failed"))
        .mockResolvedValueOnce(undefined);
      const encodeFile = vi.fn().mockResolvedValue("base64-data");

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
        encodeFile,
        logger,
      });

      expect(failed).toEqual(["fail.pdf"]);
      expect(uploadAttachment).toHaveBeenCalledTimes(2);
      expect(logger.error).toHaveBeenCalledWith(
        "Failed to upload attachment fail.pdf",
        expect.any(Error),
      );
    });

    it("handles file encoding error gracefully and returns failed attachment name", async () => {
      const logger = { error: vi.fn() };
      const uploadAttachment = vi.fn().mockResolvedValue(undefined);
      const encodeFile = vi.fn().mockRejectedValue(new Error("FileReader error"));

      const file = new File(["data"], "corrupted.bin", { type: "application/octet-stream" });
      const attachments: AttachmentUploadItem[] = [{ id: "att-1", file }];

      const failed = await uploadServiceRequestAttachments({
        caseId: "sr-200",
        attachments,
        attachmentNames: new Map(),
        uploadAttachment,
        encodeFile,
        logger,
      });

      expect(failed).toEqual(["corrupted.bin"]);
      expect(uploadAttachment).not.toHaveBeenCalled();
      expect(logger.error).toHaveBeenCalledWith(
        "Failed to upload attachment corrupted.bin",
        expect.any(Error),
      );
    });
  });

  describe("fileToBase64", () => {
    it("converts a file to base64 string without data prefix", async () => {
      const file = new File(["hello world"], "hello.txt", { type: "text/plain" });
      const result = await fileToBase64(file);
      expect(typeof result).toBe("string");
      expect(result).not.toContain("data:");
      expect(result).not.toContain(";base64,");
      // "hello world" in base64 is "aGVsbG8gd29ybGQ="
      expect(result).toBe(btoa("hello world"));
    });
  });
});
