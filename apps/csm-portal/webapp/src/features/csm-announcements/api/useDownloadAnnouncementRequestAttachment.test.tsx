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
import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const getBlobMock = vi.fn();
const saveBlobMock = vi.fn();
vi.mock("@api/backend/client", () => ({ useBackendApi: () => ({ getBlob: getBlobMock }) }));
vi.mock("@utils/saveBlob", () => ({ saveBlob: (...args: unknown[]) => saveBlobMock(...args) }));

import { useDownloadAnnouncementRequestAttachment } from "@features/csm-announcements/api/useDownloadAnnouncementRequestAttachment";

const ATTACHMENT = {
  id: "att-1",
  announcementRequestId: "req-1",
  name: "Security advisory.pdf",
  type: "application/pdf",
  sizeBytes: 10,
  createdBy: "jane@example.com",
  createdOn: "2026-07-01T10:00:00Z",
};

beforeEach(() => {
  getBlobMock.mockReset();
  saveBlobMock.mockReset();
});

describe("useDownloadAnnouncementRequestAttachment", () => {
  it("fetches the content as an authenticated blob and saves it under the file's own name", async () => {
    const blob = new Blob(["%PDF-1.4"], { type: "application/pdf" });
    getBlobMock.mockResolvedValue(blob);
    const { result } = renderHook(() => useDownloadAnnouncementRequestAttachment());

    await result.current("req-1", ATTACHMENT);

    expect(getBlobMock).toHaveBeenCalledWith("/announcement-requests/req-1/attachments/att-1/content");
    expect(saveBlobMock).toHaveBeenCalledWith(blob, "Security advisory.pdf");
  });

  it("saves nothing, and lets the caller report it, when the download is refused", async () => {
    getBlobMock.mockRejectedValue(new Error("forbidden"));
    const { result } = renderHook(() => useDownloadAnnouncementRequestAttachment());

    await expect(result.current("req-1", ATTACHMENT)).rejects.toThrow("forbidden");
    expect(saveBlobMock).not.toHaveBeenCalled();
  });
});
