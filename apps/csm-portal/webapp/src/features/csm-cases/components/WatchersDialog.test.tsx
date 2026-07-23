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
import WatchersDialog from "@features/csm-cases/components/WatchersDialog";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import type { CaseWatcher } from "@features/csm-cases/types/csmCases";

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

const EXISTING_WATCHERS: CaseWatcher[] = [
  { id: "u-1", name: "John Doe", email: "john.doe@example.com" },
];

describe("WatchersDialog — full-list-replace submit", () => {
  it("seeds the current watch list as removable chips", () => {
    mockCandidates();
    render(
      <WatchersDialog
        currentWatchers={EXISTING_WATCHERS}
        isSaving={false}
        onClose={() => {}}
        onSave={() => {}}
      />,
    );
    expect(screen.getByText("John Doe")).toBeInTheDocument();
    // No changes yet — Save stays disabled until the set actually changes.
    expect(screen.getByRole("button", { name: /save watchers/i })).toBeDisabled();
  });

  it("enables Save once a new watcher is added, and submits the full replacement set", () => {
    mockCandidates();
    const onSave = vi.fn();
    render(
      <WatchersDialog
        currentWatchers={EXISTING_WATCHERS}
        isSaving={false}
        onClose={() => {}}
        onSave={onSave}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /jane smith/i }));
    const saveButton = screen.getByRole("button", { name: /save watchers/i });
    expect(saveButton).toBeEnabled();
    fireEvent.click(saveButton);
    expect(onSave).toHaveBeenCalledWith(
      expect.arrayContaining(["john.doe@example.com", "jane.smith@example.com"]),
    );
  });

  it("removing the only watcher enables Save with an empty list", () => {
    mockCandidates();
    const onSave = vi.fn();
    render(
      <WatchersDialog
        currentWatchers={EXISTING_WATCHERS}
        isSaving={false}
        onClose={() => {}}
        onSave={onSave}
      />,
    );
    // Chip delete affordance renders as an "X" icon inside the chip.
    const chip = screen.getByText("John Doe").closest(".MuiChip-root");
    const deleteIcon = chip?.querySelector(".MuiChip-deleteIcon");
    expect(deleteIcon).toBeTruthy();
    fireEvent.click(deleteIcon as Element);
    fireEvent.click(screen.getByRole("button", { name: /save watchers/i }));
    expect(onSave).toHaveBeenCalledWith([]);
  });

  it("calls onClose on Cancel without calling onSave", () => {
    mockCandidates();
    const onSave = vi.fn();
    const onClose = vi.fn();
    render(
      <WatchersDialog
        currentWatchers={EXISTING_WATCHERS}
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
