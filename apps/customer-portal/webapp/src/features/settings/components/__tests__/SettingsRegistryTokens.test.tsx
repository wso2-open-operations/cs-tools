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
import SettingsRegistryTokens from "@features/settings/components/SettingsRegistryTokens";

vi.mock("@components/tab-bar/TabBar", () => ({
  default: ({ tabs, activeTab, onTabChange }: any) => (
    <div>
      <span>{activeTab}</span>
      {tabs.map((tab: any) => (
        <button key={tab.id} onClick={() => onTabChange(tab.id)}>
          {tab.label}
        </button>
      ))}
    </div>
  ),
}));

vi.mock("@features/settings/api/useSearchRegistryTokens", () => ({
  useSearchRegistryTokens: () => ({
    data: [{ id: 1, name: "token-a", tokenType: "UserToken", disable: false }],
    isLoading: false,
    error: null,
  }),
}));

vi.mock("@features/settings/components/GenerateTokenModal", () => ({
  default: ({ open }: { open: boolean }) => (open ? <div>generate-modal</div> : null),
}));
vi.mock("@features/settings/components/DeleteTokenModal", () => ({
  default: () => null,
}));
vi.mock("@features/settings/components/RegenerateTokenModal", () => ({
  default: () => null,
}));

describe("SettingsRegistryTokens", () => {
  it("renders and opens generate modal", () => {
    render(<SettingsRegistryTokens projectId="p-1" isAdmin />);
    fireEvent.click(screen.getByRole("button", { name: /generate user token/i }));
    expect(screen.getByText("generate-modal")).toBeInTheDocument();
  });
});

