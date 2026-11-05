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
  failedUploadsMessage,
  uploadAttachmentsToCase,
} from "@features/csm-cases/api/uploadAttachmentsToCase";

function pending(name: string) {
  const raw = new File(["x"], name);
  return { name, size: raw.size, raw };
}

describe("uploadAttachmentsToCase", () => {
  it("uploads nothing and reports nothing failed for an empty list", async () => {
    const upload = vi.fn();
    expect(await uploadAttachmentsToCase(upload, "case-1", [], "Jane Doe")).toEqual([]);
    expect(upload).not.toHaveBeenCalled();
  });

  it("uploads every file to the case and returns the names of those that failed", async () => {
    const upload = vi.fn().mockImplementation(({ name }: { name: string }) =>
      name === "bad.log" ? Promise.reject(new Error("boom")) : Promise.resolve(),
    );
    const failed = await uploadAttachmentsToCase(
      upload,
      "case-1",
      [pending("a.txt"), pending("bad.log"), pending("c.pdf")],
      "Jane Doe",
    );

    expect(failed).toEqual(["bad.log"]);
    expect(upload).toHaveBeenCalledTimes(3);
    expect(upload.mock.calls[0][0]).toMatchObject({ caseId: "case-1", name: "a.txt", uploadedBy: "Jane Doe" });
  });
});

describe("failedUploadsMessage", () => {
  it("says the record exists and names the failed files", () => {
    expect(failedUploadsMessage("case", ["a.txt"])).toBe(
      'The case was created, but 1 attachment failed to upload ("a.txt"). You can add it from the case page.',
    );
    expect(failedUploadsMessage("security report", ["a.txt", "b.txt"])).toBe(
      'The security report was created, but 2 attachments failed to upload ("a.txt", "b.txt"). You can add them from the security report page.',
    );
  });
});
