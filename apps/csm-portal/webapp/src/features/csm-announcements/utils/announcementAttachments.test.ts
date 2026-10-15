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
import { describe, expect, it } from "vitest";
import {
  ANNOUNCEMENT_ATTACHMENT_ACCEPT,
  MAX_ANNOUNCEMENT_ATTACHMENTS,
  MAX_ANNOUNCEMENT_ATTACHMENT_BYTES,
  announcementAttachmentProblem,
  readFileAsDataUrl,
} from "@features/csm-announcements/utils/announcementAttachments";

/** A file of the given name and reported size, without allocating that many bytes. */
function fileOf(name: string, size = 10): File {
  const file = new File(["x"], name);
  Object.defineProperty(file, "size", { value: size });
  return file;
}

describe("announcementAttachmentProblem", () => {
  it("accepts a PDF, and any case of the extension", () => {
    expect(announcementAttachmentProblem(fileOf("advisory.pdf"), [])).toBeNull();
    expect(announcementAttachmentProblem(fileOf("ADVISORY.PDF"), [])).toBeNull();
  });

  it.each(["doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt", "csv", "png", "jpg", "jpeg", "gif"])(
    "accepts a .%s file",
    (extension) => {
      expect(announcementAttachmentProblem(fileOf(`file.${extension}`), [])).toBeNull();
    },
  );

  it.each(["setup.exe", "script.js", "page.html", "archive.zip", "noextension", "trailing.", "report.pdf.exe"])(
    "refuses %s, which is not a type an announcement carries",
    (name) => {
      expect(announcementAttachmentProblem(fileOf(name), [])).toMatch(/can't be sent with an announcement/);
    },
  );

  it("refuses an empty file", () => {
    expect(announcementAttachmentProblem(fileOf("empty.pdf", 0), [])).toBe(`"empty.pdf" is empty.`);
  });

  it("accepts a file of exactly the maximum size and refuses one byte more", () => {
    expect(announcementAttachmentProblem(fileOf("big.pdf", MAX_ANNOUNCEMENT_ATTACHMENT_BYTES), [])).toBeNull();
    expect(announcementAttachmentProblem(fileOf("big.pdf", MAX_ANNOUNCEMENT_ATTACHMENT_BYTES + 1), [])).toMatch(
      /"big.pdf" is too large\. The maximum is 10\.00 MB per file\./,
    );
  });

  it("refuses a name already attached, whatever its case", () => {
    expect(announcementAttachmentProblem(fileOf("Advisory.pdf"), ["advisory.PDF"])).toBe(
      `A file named "Advisory.pdf" is already attached.`,
    );
  });

  it("refuses a sixth file, and allows the fifth", () => {
    const four = ["a.pdf", "b.pdf", "c.pdf", "d.pdf"];
    expect(announcementAttachmentProblem(fileOf("e.pdf"), four)).toBeNull();
    expect(announcementAttachmentProblem(fileOf("f.pdf"), [...four, "e.pdf"])).toBe(
      `An announcement can carry at most ${MAX_ANNOUNCEMENT_ATTACHMENTS} files. Remove one to add another.`,
    );
  });

  it("states the type problem before anything else, so the reason is the one that matters", () => {
    expect(announcementAttachmentProblem(fileOf("setup.exe", 0), ["setup.exe"])).toMatch(/can't be sent/);
  });
});

describe("ANNOUNCEMENT_ATTACHMENT_ACCEPT", () => {
  it("lists every accepted extension for the file picker", () => {
    expect(ANNOUNCEMENT_ATTACHMENT_ACCEPT.split(",")).toEqual(
      expect.arrayContaining([".pdf", ".png", ".docx", ".xlsx", ".pptx", ".txt", ".csv"]),
    );
    expect(ANNOUNCEMENT_ATTACHMENT_ACCEPT).not.toContain(".exe");
  });
});

describe("readFileAsDataUrl", () => {
  it("reads a file as a base64 data URI that keeps its type and bytes", async () => {
    const uri = await readFileAsDataUrl(new File(["%PDF-1.4 body"], "a.pdf", { type: "application/pdf" }));
    expect(uri.startsWith("data:application/pdf;base64,")).toBe(true);
    expect(atob(uri.split(",")[1])).toBe("%PDF-1.4 body");
  });
});
