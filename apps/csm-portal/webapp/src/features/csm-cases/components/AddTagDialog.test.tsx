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

import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import AddTagDialog from "@features/csm-cases/components/AddTagDialog";
import { useSearchTags } from "@features/csm-cases/api/useSearchTags";

vi.mock("@features/csm-cases/api/useSearchTags", () => ({
  useSearchTags: vi.fn(),
}));

const mockedUseSearchTags = vi.mocked(useSearchTags);

function mockSearchResult(
  overrides: Partial<ReturnType<typeof useSearchTags>>,
): void {
  mockedUseSearchTags.mockReturnValue({
    data: [],
    isFetching: false,
    isError: false,
    ...overrides,
  } as unknown as ReturnType<typeof useSearchTags>);
}

describe("AddTagDialog — search-and-SELECT tag entry (no free-text creation)", () => {
  beforeEach(() => {
    mockedUseSearchTags.mockReset();
    mockSearchResult({});
  });

  it("disables Add tag until a suggestion is selected", () => {
    render(
      <AddTagDialog
        existingLabels={[]}
        isSaving={false}
        onClose={() => {}}
        onSave={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /add tag/i })).toBeDisabled();
  });

  it("typing alone does not enable Add tag — a suggestion must be selected", () => {
    mockSearchResult({ data: [{ id: "t1", label: "micro-gw" }] });
    render(
      <AddTagDialog
        existingLabels={[]}
        isSaving={false}
        onClose={() => {}}
        onSave={() => {}}
      />,
    );
    fireEvent.change(screen.getByLabelText(/^tag$/i), {
      target: { value: "micro-gw-but-not-quite" },
    });
    expect(screen.getByRole("button", { name: /add tag/i })).toBeDisabled();
  });

  it("submits the label once a suggestion is clicked", () => {
    mockSearchResult({ data: [{ id: "t1", label: "micro-gw" }] });
    const onSave = vi.fn();
    render(
      <AddTagDialog
        existingLabels={[]}
        isSaving={false}
        onClose={() => {}}
        onSave={onSave}
      />,
    );
    fireEvent.change(screen.getByLabelText(/^tag$/i), {
      target: { value: "micro" },
    });
    fireEvent.click(screen.getByText("micro-gw"));
    expect(screen.getByRole("button", { name: /add tag/i })).not.toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: /add tag/i }));
    expect(onSave).toHaveBeenCalledWith("micro-gw");
  });

  it("blocks a case-insensitive duplicate of an existing tag", () => {
    mockSearchResult({ data: [{ id: "t1", label: "Micro-GW" }] });
    const onSave = vi.fn();
    render(
      <AddTagDialog
        existingLabels={["Micro-GW"]}
        isSaving={false}
        onClose={() => {}}
        onSave={onSave}
      />,
    );
    fireEvent.change(screen.getByLabelText(/^tag$/i), {
      target: { value: "micro" },
    });
    // Already-on-the-case labels are filtered out of the suggestion list, so
    // there is nothing to select and Add tag stays disabled.
    expect(screen.queryByText("Micro-GW")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /add tag/i })).toBeDisabled();
    expect(onSave).not.toHaveBeenCalled();
  });

  it("offers matching existing tags as suggestions, excluding ones already on the case", () => {
    mockSearchResult({
      data: [
        { id: "t1", label: "micro-gw" },
        { id: "t2", label: "micro-gw-2" },
        { id: "t3", label: "ws-policy" },
      ],
    });
    render(
      <AddTagDialog
        existingLabels={["ws-policy"]}
        isSaving={false}
        onClose={() => {}}
        onSave={() => {}}
      />,
    );
    fireEvent.change(screen.getByLabelText(/^tag$/i), {
      target: { value: "micro" },
    });
    expect(screen.getByText("micro-gw")).toBeInTheDocument();
    expect(screen.getByText("micro-gw-2")).toBeInTheDocument();
    expect(screen.queryByText("ws-policy")).not.toBeInTheDocument();
  });

  it("calls onClose on Cancel without calling onSave", () => {
    const onSave = vi.fn();
    const onClose = vi.fn();
    render(
      <AddTagDialog
        existingLabels={[]}
        isSaving={false}
        onClose={onClose}
        onSave={onSave}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /^cancel$/i }));
    expect(onClose).toHaveBeenCalled();
    expect(onSave).not.toHaveBeenCalled();
  });
});
