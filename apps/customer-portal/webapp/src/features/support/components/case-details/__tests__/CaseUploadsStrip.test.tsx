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
import { act, render, screen } from "@testing-library/react";
import { cancelCaseUpload, trackCaseUpload, uploadPercent } from "@api/caseUploadProgress";
import { fireEvent } from "@testing-library/react";
import CaseUploadsStrip from "@features/support/components/case-details/CaseUploadsStrip";

describe("uploadPercent", () => {
  it("is the share sent, capped at 100, and 0 for no total", () => {
    expect(uploadPercent(50, 200)).toBe(25);
    expect(uploadPercent(300, 200)).toBe(100);
    expect(uploadPercent(5, 0)).toBe(0);
  });
});

describe("CaseUploadsStrip", () => {
  it("shows nothing while no upload runs", () => {
    const { container } = render(<CaseUploadsStrip caseId="c-none" />);
    expect(container.textContent).toBe("");
  });

  it("shows each upload of the case with its percentage, then clears it", () => {
    render(<CaseUploadsStrip caseId="c-1" />);
    let upload!: ReturnType<typeof trackCaseUpload>;
    let other!: ReturnType<typeof trackCaseUpload>;
    act(() => {
      upload = trackCaseUpload("c-1", "logs.zip");
      other = trackCaseUpload("c-2", "other.zip");
    });
    expect(screen.getByText("Uploading logs.zip")).toBeTruthy();
    expect(screen.getByText("0%")).toBeTruthy();
    expect(screen.queryByText("Uploading other.zip")).toBeNull();

    act(() => upload.update(30, 100));
    expect(screen.getByText("30%")).toBeTruthy();

    act(() => {
      upload.finish();
      other.finish();
    });
    expect(screen.queryByText("Uploading logs.zip")).toBeNull();
  });

  it("offers Cancel for an upload that can be stopped, and stops it", () => {
    const cancel = vi.fn();
    render(<CaseUploadsStrip caseId="c-4" />);
    let upload!: ReturnType<typeof trackCaseUpload>;
    act(() => {
      upload = trackCaseUpload("c-4", "big.zip", { cancel });
    });
    fireEvent.click(screen.getByRole("button", { name: "Cancel upload of big.zip" }));
    expect(cancel).toHaveBeenCalledTimes(1);
    act(() => upload.finish());
  });

  it("shows no Cancel button when an upload cannot be stopped", () => {
    render(<CaseUploadsStrip caseId="c-5" />);
    let upload!: ReturnType<typeof trackCaseUpload>;
    act(() => {
      upload = trackCaseUpload("c-5", "fixed.zip");
    });
    expect(screen.queryByRole("button", { name: /cancel upload/i })).toBeNull();
    act(() => upload.finish());
  });

  it("cancelCaseUpload ignores a key that is not running", () => {
    expect(() => cancelCaseUpload(999999)).not.toThrow();
  });

  it("leaves out the uploads the comment box shows inside its own editor", () => {
    render(<CaseUploadsStrip caseId="c-3" />);
    let own!: ReturnType<typeof trackCaseUpload>;
    act(() => {
      own = trackCaseUpload("c-3", "mine.pdf", { inPlace: true });
    });
    expect(screen.queryByText("Uploading mine.pdf")).toBeNull();
    act(() => own.finish());
  });
});
