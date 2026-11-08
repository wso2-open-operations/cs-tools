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

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";
import EditorUploadProgress from "@components/rich-text-editor/EditorUploadProgress";

describe("EditorUploadProgress", () => {
  it("shows nothing when no file is uploading", () => {
    const { container } = render(<EditorUploadProgress uploads={[]} waiting={2} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows each file with its percentage and a progress bar", () => {
    render(<EditorUploadProgress uploads={[{ name: "5mb.webp", percent: 40 }]} />);
    expect(screen.getByText("Uploading 5mb.webp")).toBeInTheDocument();
    expect(screen.getByText("40%")).toBeInTheDocument();
    expect(screen.getByRole("progressbar", { name: "Uploading 5mb.webp" })).toHaveAttribute(
      "aria-valuenow",
      "40",
    );
  });

  it("says how many files are waiting behind the running one", () => {
    render(<EditorUploadProgress uploads={[{ name: "a.pdf", percent: 0 }]} waiting={1} />);
    expect(screen.getByText("1 more file waiting to upload")).toBeInTheDocument();
  });
});
