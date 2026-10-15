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
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const showErrorMock = vi.fn();
const listMock = vi.fn();
const uploadMutateAsyncMock = vi.fn();
const deleteMutateAsyncMock = vi.fn();
const downloadMock = vi.fn();
const attachMutateAsyncMock = vi.fn();

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
vi.mock("@features/csm-announcements/api/useListAnnouncementRequestAttachments", () => ({
  useListAnnouncementRequestAttachments: (...args: unknown[]) => listMock(...args),
}));
vi.mock("@features/csm-announcements/api/useUploadAnnouncementRequestAttachment", () => ({
  useUploadAnnouncementRequestAttachment: () => ({ mutateAsync: uploadMutateAsyncMock }),
}));
vi.mock("@features/csm-announcements/api/useDeleteAnnouncementRequestAttachment", () => ({
  useDeleteAnnouncementRequestAttachment: () => ({ mutateAsync: deleteMutateAsyncMock }),
}));
vi.mock("@features/csm-announcements/api/useDownloadAnnouncementRequestAttachment", () => ({
  useDownloadAnnouncementRequestAttachment: () => downloadMock,
}));
vi.mock("@features/csm-announcements/api/useAttachAnnouncementRequestFilesToCase", () => ({
  useAttachAnnouncementRequestFilesToCase: () => ({ mutateAsync: attachMutateAsyncMock }),
}));

import { useAnnouncementAttachments } from "@features/csm-announcements/api/useAnnouncementAttachments";
import type { AnnouncementRequestAttachment } from "@features/csm-announcements/types/announcementRequests";

const pdf = (name: string): File => new File(["%PDF-1.4"], name, { type: "application/pdf" });

function saved(name: string): AnnouncementRequestAttachment {
  return {
    id: `att-${name}`,
    announcementRequestId: "req-1",
    name,
    type: "application/pdf",
    sizeBytes: 8,
    createdBy: "jane@example.com",
    createdOn: "2026-07-01T10:00:00Z",
  };
}

function list(names: string[] = [], extra: Record<string, unknown> = {}) {
  return { data: names.map(saved), isLoading: false, isError: false, ...extra };
}

beforeEach(() => {
  showErrorMock.mockReset();
  listMock.mockReset();
  listMock.mockReturnValue(list());
  uploadMutateAsyncMock.mockReset();
  uploadMutateAsyncMock.mockResolvedValue({});
  deleteMutateAsyncMock.mockReset();
  deleteMutateAsyncMock.mockResolvedValue(undefined);
  downloadMock.mockReset();
  downloadMock.mockResolvedValue(undefined);
  attachMutateAsyncMock.mockReset();
  attachMutateAsyncMock.mockResolvedValue({ attached: 1, alreadyAttached: 0 });
});

describe("useAnnouncementAttachments — before the draft exists", () => {
  it("only holds the picked files, in order, and uploads nothing", async () => {
    const { result } = renderHook(() => useAnnouncementAttachments(undefined));

    await act(async () => {
      await result.current.addFiles([pdf("a.pdf"), pdf("b.pdf")]);
    });

    expect(result.current.items.map((i) => [i.name, i.status])).toEqual([
      ["a.pdf", "staged"],
      ["b.pdf", "staged"],
    ]);
    expect(result.current.count).toBe(2);
    expect(uploadMutateAsyncMock).not.toHaveBeenCalled();
  });

  it("never reads the server's list, and never reports one as loading or failed", () => {
    listMock.mockReturnValue(list(["ghost.pdf"], { isLoading: true, isError: true }));
    const { result } = renderHook(() => useAnnouncementAttachments(undefined));

    expect(listMock).toHaveBeenCalledWith(undefined);
    expect(result.current.items).toEqual([]);
    expect(result.current.isLoading).toBe(false);
    expect(result.current.loadFailed).toBe(false);
  });

  it("tells the sender why a file is refused, and keeps the good ones of the same pick", async () => {
    const { result } = renderHook(() => useAnnouncementAttachments(undefined));

    await act(async () => {
      await result.current.addFiles([pdf("good.pdf"), pdf("setup.exe")]);
    });

    expect(result.current.items.map((i) => i.name)).toEqual(["good.pdf"]);
    expect(showErrorMock).toHaveBeenCalledTimes(1);
    expect(showErrorMock).toHaveBeenCalledWith(expect.stringMatching(/"setup.exe" can't be sent with an announcement/));
  });

  it("refuses a duplicate name within one pick, whatever its case", async () => {
    const { result } = renderHook(() => useAnnouncementAttachments(undefined));
    await act(async () => {
      await result.current.addFiles([pdf("a.pdf"), pdf("A.PDF")]);
    });
    expect(result.current.items.map((i) => i.name)).toEqual(["a.pdf"]);
    expect(showErrorMock).toHaveBeenCalledWith(expect.stringMatching(/already attached/));
  });

  it("refuses a sixth file across picks", async () => {
    const { result } = renderHook(() => useAnnouncementAttachments(undefined));
    await act(async () => {
      await result.current.addFiles(["a", "b", "c", "d", "e"].map((n) => pdf(`${n}.pdf`)));
    });
    await act(async () => {
      await result.current.addFiles([pdf("f.pdf")]);
    });
    expect(result.current.count).toBe(5);
    expect(showErrorMock).toHaveBeenCalledWith(expect.stringMatching(/at most 5 files/));
  });

  it("removes a held file without any request to the server", async () => {
    const { result } = renderHook(() => useAnnouncementAttachments(undefined));
    await act(async () => {
      await result.current.addFiles([pdf("a.pdf"), pdf("b.pdf")]);
    });
    await act(async () => {
      await result.current.remove(result.current.items[0]);
    });

    expect(result.current.items.map((i) => i.name)).toEqual(["b.pdf"]);
    expect(deleteMutateAsyncMock).not.toHaveBeenCalled();
  });

  describe("flushStaged", () => {
    it("uploads every held file to the new request, in order, and leaves none held", async () => {
      const { result } = renderHook(() => useAnnouncementAttachments(undefined));
      await act(async () => {
        await result.current.addFiles([pdf("a.pdf"), pdf("b.pdf")]);
      });

      let ok: boolean | undefined;
      await act(async () => {
        ok = await result.current.flushStaged("req-9");
      });

      expect(ok).toBe(true);
      expect(uploadMutateAsyncMock.mock.calls.map((c) => [c[0].id, c[0].file.name])).toEqual([
        ["req-9", "a.pdf"],
        ["req-9", "b.pdf"],
      ]);
      expect(result.current.items).toEqual([]);
      expect(showErrorMock).not.toHaveBeenCalled();
    });

    it("is a no-op that succeeds when nothing is held (a form with no files behaves as before)", async () => {
      const { result } = renderHook(() => useAnnouncementAttachments(undefined));
      let ok: boolean | undefined;
      await act(async () => {
        ok = await result.current.flushStaged("req-9");
      });
      expect(ok).toBe(true);
      expect(uploadMutateAsyncMock).not.toHaveBeenCalled();
    });

    it("stops at the first refusal, names the file, and keeps it and the rest for a retry", async () => {
      uploadMutateAsyncMock.mockResolvedValueOnce({}).mockRejectedValueOnce(new Error("too big"));
      const { result } = renderHook(() => useAnnouncementAttachments(undefined));
      await act(async () => {
        await result.current.addFiles([pdf("a.pdf"), pdf("b.pdf"), pdf("c.pdf")]);
      });

      let ok: boolean | undefined;
      await act(async () => {
        ok = await result.current.flushStaged("req-9");
      });

      expect(ok).toBe(false);
      expect(uploadMutateAsyncMock).toHaveBeenCalledTimes(2);
      expect(showErrorMock).toHaveBeenCalledWith(`"b.pdf" could not be attached: too big`);
      expect(result.current.items.map((i) => [i.name, i.status])).toEqual([
        ["b.pdf", "staged"],
        ["c.pdf", "staged"],
      ]);

      // Calling it again retries only what is left; "a.pdf" is not uploaded twice.
      uploadMutateAsyncMock.mockClear();
      await act(async () => {
        ok = await result.current.flushStaged("req-9");
      });
      expect(ok).toBe(true);
      expect(uploadMutateAsyncMock.mock.calls.map((c) => c[0].file.name)).toEqual(["b.pdf", "c.pdf"]);
      expect(result.current.items).toEqual([]);
    });
  });
});

describe("useAnnouncementAttachments — on a request that exists", () => {
  it("shows the request's saved files, and counts them", () => {
    listMock.mockReturnValue(list(["a.pdf", "b.pdf"]));
    const { result } = renderHook(() => useAnnouncementAttachments("req-1"));

    expect(listMock).toHaveBeenCalledWith("req-1");
    expect(result.current.items.map((i) => [i.name, i.status])).toEqual([
      ["a.pdf", "saved"],
      ["b.pdf", "saved"],
    ]);
    expect(result.current.count).toBe(2);
  });

  it("reports the list loading or failing, so a count is never taken for the real one", () => {
    listMock.mockReturnValue(list([], { isLoading: true }));
    const { result, rerender } = renderHook(() => useAnnouncementAttachments("req-1"));
    expect(result.current.isLoading).toBe(true);

    listMock.mockReturnValue(list([], { isError: true }));
    rerender();
    expect(result.current.loadFailed).toBe(true);
    expect(result.current.isLoading).toBe(false);
  });

  it("uploads a picked file at once, one at a time, showing it as uploading meanwhile", async () => {
    let release!: () => void;
    uploadMutateAsyncMock.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          release = () => resolve({});
        }),
    );
    const { result } = renderHook(() => useAnnouncementAttachments("req-1"));

    let done!: Promise<void>;
    act(() => {
      done = result.current.addFiles([pdf("a.pdf"), pdf("b.pdf")]);
    });

    expect(result.current.busy).toBe(true);
    expect(result.current.items.map((i) => [i.name, i.status])).toEqual([["a.pdf", "uploading"]]);
    expect(uploadMutateAsyncMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      release();
      await done;
    });

    expect(uploadMutateAsyncMock.mock.calls.map((c) => c[0])).toEqual([
      { id: "req-1", file: expect.objectContaining({ name: "a.pdf" }) },
      { id: "req-1", file: expect.objectContaining({ name: "b.pdf" }) },
    ]);
    expect(result.current.busy).toBe(false);
  });

  it("reports a refused upload with the server's reason and carries on with the next file", async () => {
    uploadMutateAsyncMock.mockRejectedValueOnce(new Error("a file named a.pdf is already attached")).mockResolvedValueOnce({});
    const { result } = renderHook(() => useAnnouncementAttachments("req-1"));

    await act(async () => {
      await result.current.addFiles([pdf("a.pdf"), pdf("b.pdf")]);
    });

    expect(showErrorMock).toHaveBeenCalledWith("a file named a.pdf is already attached");
    expect(uploadMutateAsyncMock).toHaveBeenCalledTimes(2);
    expect(result.current.busy).toBe(false);
  });

  it("falls back to a plain message when the failure carries none", async () => {
    uploadMutateAsyncMock.mockRejectedValueOnce(new Error("  "));
    const { result } = renderHook(() => useAnnouncementAttachments("req-1"));
    await act(async () => {
      await result.current.addFiles([pdf("a.pdf")]);
    });
    expect(showErrorMock).toHaveBeenCalledWith(`Could not attach "a.pdf". Please try again.`);
  });

  it("judges a new file against the files the request already has", async () => {
    listMock.mockReturnValue(list(["advisory.pdf"]));
    const { result } = renderHook(() => useAnnouncementAttachments("req-1"));

    await act(async () => {
      await result.current.addFiles([pdf("Advisory.pdf")]);
    });

    expect(uploadMutateAsyncMock).not.toHaveBeenCalled();
    expect(showErrorMock).toHaveBeenCalledWith(expect.stringMatching(/already attached/));
  });

  it("removes a saved file on the server, marking it while it goes", async () => {
    listMock.mockReturnValue(list(["a.pdf"]));
    let release!: () => void;
    deleteMutateAsyncMock.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          release = () => resolve();
        }),
    );
    const { result } = renderHook(() => useAnnouncementAttachments("req-1"));

    let done!: Promise<void>;
    act(() => {
      done = result.current.remove(result.current.items[0]);
    });
    expect(result.current.items[0].removing).toBe(true);
    expect(result.current.busy).toBe(true);

    await act(async () => {
      release();
      await done;
    });

    expect(deleteMutateAsyncMock).toHaveBeenCalledWith({ id: "req-1", attachmentId: "att-a.pdf" });
    expect(result.current.items[0].removing).toBe(false);
    expect(result.current.busy).toBe(false);
  });

  it("reports a refused removal and frees the file again", async () => {
    listMock.mockReturnValue(list(["a.pdf"]));
    deleteMutateAsyncMock.mockRejectedValueOnce(new Error("some projects have already received this announcement"));
    const { result } = renderHook(() => useAnnouncementAttachments("req-1"));

    await act(async () => {
      await result.current.remove(result.current.items[0]);
    });

    expect(showErrorMock).toHaveBeenCalledWith("some projects have already received this announcement");
    expect(result.current.items[0].removing).toBe(false);
  });

  it("downloads a saved file, and reports a refusal instead of dropping it", async () => {
    listMock.mockReturnValue(list(["a.pdf"]));
    const { result } = renderHook(() => useAnnouncementAttachments("req-1"));
    await act(async () => {
      await result.current.download(result.current.items[0]);
    });
    expect(downloadMock).toHaveBeenCalledWith("req-1", saved("a.pdf"));

    downloadMock.mockRejectedValueOnce(new Error("forbidden"));
    await act(async () => {
      await result.current.download(result.current.items[0]);
    });
    expect(showErrorMock).toHaveBeenCalledWith("forbidden");
  });

  it("does not download a file that is only held in the form", async () => {
    const { result } = renderHook(() => useAnnouncementAttachments(undefined));
    await act(async () => {
      await result.current.addFiles([pdf("a.pdf")]);
    });
    await act(async () => {
      await result.current.download(result.current.items[0]);
    });
    expect(downloadMock).not.toHaveBeenCalled();
  });
});

describe("useAnnouncementAttachments — attachToDryRunCase", () => {
  it("has the server copy the files onto the dry-run case", async () => {
    const { result } = renderHook(() => useAnnouncementAttachments("req-1"));
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.attachToDryRunCase("req-1", "case-5");
    });
    expect(ok).toBe(true);
    expect(attachMutateAsyncMock).toHaveBeenCalledWith({ id: "req-1", caseId: "case-5" });
    expect(showErrorMock).not.toHaveBeenCalled();
  });

  it("reports that the case exists but lacks its files, and that submitting again retries", async () => {
    attachMutateAsyncMock.mockRejectedValueOnce(new Error("downstream refused"));
    const { result } = renderHook(() => useAnnouncementAttachments("req-1"));
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.attachToDryRunCase("req-1", "case-5");
    });

    expect(ok).toBe(false);
    expect(showErrorMock).toHaveBeenCalledWith(
      "The dry-run case was created, but its attachments couldn't be added: downstream refused Submit again to retry.",
    );
  });
});
