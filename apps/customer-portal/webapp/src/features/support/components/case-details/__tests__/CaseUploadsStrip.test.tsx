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
import { act, render, screen } from "@testing-library/react";
import { trackCaseUpload, uploadPercent } from "@api/caseUploadProgress";
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

  it("keeps the uploads a form shows itself out of the page's strip, and shows them in place", () => {
    render(
      <>
        <CaseUploadsStrip caseId="c-3" />
        <CaseUploadsStrip caseId="c-3" inPlace />
      </>,
    );
    let own!: ReturnType<typeof trackCaseUpload>;
    act(() => {
      own = trackCaseUpload("c-3", "mine.pdf", { inPlace: true });
    });
    // Shown once: by the in-place strip, not by the page's.
    expect(screen.getAllByText("Uploading mine.pdf")).toHaveLength(1);
    act(() => own.finish());
  });
});
