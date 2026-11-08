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

import { act, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  DOWNLOAD_STARTED,
  setAttachmentDownloadProgress,
} from "@features/csm-cases/api/attachmentDownloadProgress";
import {
  AttachmentDownloadBar,
  AttachmentDownloadButton,
} from "@features/csm-cases/components/AttachmentDownloadProgress";
import type { CaseAttachment } from "@features/csm-cases/types/csmCases";

const attachment = { id: "att-1", filename: "7mb.png" } as CaseAttachment;

describe("AttachmentDownloadButton / AttachmentDownloadBar", () => {
  afterEach(() => setAttachmentDownloadProgress("att-1", undefined));

  it("shows Download when idle and calls onDownload", () => {
    const onDownload = vi.fn();
    render(<AttachmentDownloadButton attachment={attachment} onDownload={onDownload} />);
    screen.getByRole("button", { name: "Download 7mb.png" }).click();
    expect(onDownload).toHaveBeenCalledWith(attachment);
    expect(screen.queryByText(/%/)).toBeNull();
  });

  it("shows the percentage and disables the button while it downloads", () => {
    render(<AttachmentDownloadButton attachment={attachment} onDownload={vi.fn()} />);
    act(() => setAttachmentDownloadProgress("att-1", 42));
    const button = screen.getByRole("button", { name: "Download 7mb.png" });
    expect((button as HTMLButtonElement).disabled).toBe(true);
    expect(button.textContent).toContain("42%");
    act(() => setAttachmentDownloadProgress("att-1", DOWNLOAD_STARTED));
    expect(button.textContent).toContain("Starting");
  });

  it("draws the bar only while a download runs", () => {
    const { container } = render(<AttachmentDownloadBar id="att-1" />);
    expect(container.querySelector('[role="progressbar"]')).toBeNull();
    act(() => setAttachmentDownloadProgress("att-1", 10));
    expect(container.querySelector('[role="progressbar"]')).not.toBeNull();
  });
});
