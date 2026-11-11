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
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import Editor from "./Editor";

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() }),
}));

// The toolbar reads the error banner; a stub keeps the test free of the provider.
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn(), clearError: vi.fn(), showErrorBanner: vi.fn() }),
}));

const file = (name: string) => new File(["content"], name, { type: "text/plain" });

describe("Editor attachment limit", () => {
  it("keeps the attach button on while the editor holds fewer than maxAttachments", () => {
    render(<Editor onAttachmentClick={() => {}} attachments={[]} maxAttachments={1} />);
    expect(screen.getByRole("button", { name: "Attach File" })).not.toBeDisabled();
  });

  it("turns the attach button off once the editor holds maxAttachments, and says why", () => {
    render(<Editor onAttachmentClick={() => {}} attachments={[file("a.txt")]} maxAttachments={1} />);
    expect(screen.getByRole("button", { name: /only one file allowed/i })).toBeDisabled();
  });

  it("has no limit when maxAttachments is not given", () => {
    render(<Editor onAttachmentClick={() => {}} attachments={[file("a.txt"), file("b.txt")]} />);
    expect(screen.getByRole("button", { name: "Attach File" })).not.toBeDisabled();
  });
});
