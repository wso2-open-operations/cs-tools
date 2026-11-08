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

import { useState } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import AttachmentsField from "@components/attachments/AttachmentsField";
import type { PendingAttachment } from "@components/attachments/pendingAttachment";

const fileOf = (name: string, size = 100) => new File([new Uint8Array(size)], name);

function Harness({
  maxFiles,
  onError,
  initial = [],
}: {
  maxFiles?: number;
  onError: (message: string) => void;
  initial?: PendingAttachment[];
}) {
  const [attachments, setAttachments] = useState<PendingAttachment[]>(initial);
  return <AttachmentsField attachments={attachments} onChange={setAttachments} onError={onError} maxFiles={maxFiles} />;
}

const input = (container: HTMLElement) => container.querySelector('input[type="file"]') as HTMLInputElement;
const pick = (container: HTMLElement, ...files: File[]) =>
  fireEvent.change(input(container), { target: { files } });

describe("AttachmentsField with a file limit", () => {
  it("takes a single file at a time when the limit is 1", () => {
    const { container } = render(<Harness maxFiles={1} onError={vi.fn()} />);
    expect(input(container).multiple).toBe(false);
    expect(screen.getByText("Add attachment")).toBeInTheDocument();
  });

  it("adds the file, then turns adding off until it is removed", () => {
    const { container } = render(<Harness maxFiles={1} onError={vi.fn()} />);
    pick(container, fileOf("a.txt"));
    expect(screen.getByText("a.txt")).toBeInTheDocument();
    expect(input(container)).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Remove a.txt" }));
    expect(input(container)).not.toBeDisabled();
  });

  it("keeps only the first file of a pick that is over the limit, and says so", () => {
    const onError = vi.fn();
    const { container } = render(<Harness maxFiles={1} onError={onError} />);
    pick(container, fileOf("a.txt"), fileOf("b.txt"));
    expect(screen.getByText("a.txt")).toBeInTheDocument();
    expect(screen.queryByText("b.txt")).toBeNull();
    expect(onError).toHaveBeenCalledWith(expect.stringMatching(/Only 1 attachment can be added/));
  });

  it("adds nothing and says so when the field is already full", () => {
    const onError = vi.fn();
    const initial = [{ name: "a.txt", size: 100, raw: fileOf("a.txt") }];
    const { container } = render(<Harness maxFiles={1} onError={onError} initial={initial} />);
    pick(container, fileOf("b.txt"));
    expect(screen.queryByText("b.txt")).toBeNull();
    expect(onError).toHaveBeenCalledTimes(1);
  });

  it("has no limit when none is given", () => {
    const { container } = render(<Harness onError={vi.fn()} />);
    expect(input(container).multiple).toBe(true);
    pick(container, fileOf("a.txt"), fileOf("b.txt"), fileOf("c.txt"));
    expect(screen.getByText("c.txt")).toBeInTheDocument();
    expect(input(container)).not.toBeDisabled();
  });
});
