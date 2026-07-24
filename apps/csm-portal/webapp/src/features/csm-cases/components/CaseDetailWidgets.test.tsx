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
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import {
  TagsWidget,
  WatchersWidget,
} from "@features/csm-cases/components/CaseDetailWidgets";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import type { CaseTag, CaseWatcher } from "@features/csm-cases/types/csmCases";

vi.mock("@features/csm-users/api/useSearchUsers", () => ({
  useSearchUsers: vi.fn(),
}));

const mockUseSearchUsers = vi.mocked(useSearchUsers);

function mockCandidates(): void {
  mockUseSearchUsers.mockReturnValue({
    data: {
      users: [
        {
          id: "u-2",
          userName: "jsmith",
          name: "Jane Smith",
          email: "jane.smith@example.com",
          timezone: null,
          active: true,
        },
      ],
      total: 1,
      limit: 8,
      offset: 0,
      hasMore: false,
    },
    isFetching: false,
    isError: false,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
  } as any);
}

const TAGS: CaseTag[] = [
  { id: "tag-1", label: "micro-gw" },
  { id: "tag-2", label: "ws-policy" },
];

const WATCHERS: CaseWatcher[] = [
  { id: "w-1", name: "Jane Doe", email: "jane.doe@example.com" },
  { id: "w-2", name: "John Smith", isMe: true },
];

describe("TagsWidget", () => {
  it("renders an empty state when there are no tags", () => {
    render(<TagsWidget tags={[]} />);
    expect(screen.getByText("No tags applied.")).toBeInTheDocument();
  });

  it("renders every tag as a chip", () => {
    render(<TagsWidget tags={TAGS} />);
    expect(screen.getByText("micro-gw")).toBeInTheDocument();
    expect(screen.getByText("ws-policy")).toBeInTheDocument();
  });

  it("calls onAdd when the Tag button is clicked", () => {
    const onAdd = vi.fn();
    render(<TagsWidget tags={TAGS} onAdd={onAdd} />);
    fireEvent.click(screen.getByRole("button", { name: /^tag$/i }));
    expect(onAdd).toHaveBeenCalled();
  });

  it("calls onRemove with the tag when its chip delete icon is clicked", () => {
    const onRemove = vi.fn();
    render(<TagsWidget tags={TAGS} onRemove={onRemove} />);
    const chip = screen.getByText("micro-gw").closest(".MuiChip-root");
    const deleteIcon = chip?.querySelector(".MuiChip-deleteIcon");
    expect(deleteIcon).toBeTruthy();
    fireEvent.click(deleteIcon as Element);
    expect(onRemove).toHaveBeenCalledWith(TAGS[0]);
  });

  it("omits the delete affordance when onRemove is not provided", () => {
    render(<TagsWidget tags={TAGS} />);
    const chip = screen.getByText("micro-gw").closest(".MuiChip-root");
    expect(chip?.querySelector(".MuiChip-deleteIcon")).toBeFalsy();
  });
});

describe("WatchersWidget", () => {
  it("renders an empty state when there are no watchers", () => {
    render(<WatchersWidget watchers={[]} />);
    expect(
      screen.getByText("No one is watching this case."),
    ).toBeInTheDocument();
  });

  it("renders every watcher as a chip, marking the current user", () => {
    render(<WatchersWidget watchers={WATCHERS} />);
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByText("John Smith (you)")).toBeInTheDocument();
  });

  it("hides the Add watcher action when onAdd is omitted", () => {
    render(<WatchersWidget watchers={WATCHERS} />);
    expect(
      screen.queryByRole("button", { name: /add watcher/i }),
    ).not.toBeInTheDocument();
  });

  it("omits the per-chip remove affordance when onRemove is omitted", () => {
    render(<WatchersWidget watchers={WATCHERS} />);
    const chip = screen.getByText("Jane Doe").closest(".MuiChip-root");
    expect(chip?.querySelector(".MuiChip-deleteIcon")).toBeFalsy();
  });

  it("calls onRemove with the watcher when its chip delete icon is clicked", () => {
    const onRemove = vi.fn();
    render(<WatchersWidget watchers={WATCHERS} onRemove={onRemove} />);
    const chip = screen.getByText("Jane Doe").closest(".MuiChip-root");
    const deleteIcon = chip?.querySelector(".MuiChip-deleteIcon");
    expect(deleteIcon).toBeTruthy();
    fireEvent.click(deleteIcon as Element);
    expect(onRemove).toHaveBeenCalledWith(WATCHERS[0]);
  });

  it("opens an inline search panel on Add watcher and calls onAdd for a picked candidate — no dialog involved", () => {
    mockCandidates();
    const onAdd = vi.fn();
    render(<WatchersWidget watchers={WATCHERS} onAdd={onAdd} />);

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /add watcher/i }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /jane smith/i }));
    expect(onAdd).toHaveBeenCalledWith("jane.smith@example.com", "Jane Smith");
  });

  it("closes the inline search panel when its cancel button is clicked", () => {
    mockCandidates();
    render(<WatchersWidget watchers={WATCHERS} onAdd={() => {}} />);
    fireEvent.click(screen.getByRole("button", { name: /add watcher/i }));
    expect(
      screen.getByPlaceholderText(/search people to add/i),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /cancel adding a watcher/i }));
    expect(
      screen.queryByPlaceholderText(/search people to add/i),
    ).not.toBeInTheDocument();
  });
});
