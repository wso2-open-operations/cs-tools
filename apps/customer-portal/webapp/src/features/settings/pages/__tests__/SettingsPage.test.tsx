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
import SettingsPage from "@features/settings/pages/SettingsPage";

const useParamsMock = vi.fn();
const consumePendingSettingsTabMock = vi.fn();
const useGetUserDetailsMock = vi.fn();
const useGetProjectDetailsMock = vi.fn();

vi.mock("react-router", () => ({
  useParams: () => useParamsMock(),
}));

vi.mock("@components/tab-bar/TabBar", () => ({
  default: ({ activeTab }: { activeTab: string }) => <div>tab:{activeTab}</div>,
}));

vi.mock("@features/settings/components/SettingsAiAssistant", () => ({
  default: () => <div>ai-settings</div>,
}));

vi.mock("@features/settings/components/SettingsUserManagement", () => ({
  default: () => <div>user-management</div>,
}));

vi.mock("@features/settings/components/SettingsRegistryTokens", () => ({
  default: () => <div>registry-settings</div>,
}));

vi.mock("@features/settings/api/useGetUserDetails", () => ({
  default: () => useGetUserDetailsMock(),
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => useGetProjectDetailsMock(),
}));

vi.mock("@features/settings/utils/settingsStorage", () => ({
  consumePendingSettingsTab: () => consumePendingSettingsTabMock(),
}));

vi.mock("@utils/permission", () => ({
  isProjectRestricted: () => false,
}));

describe("SettingsPage", () => {
  it("renders not-found message when project id is missing", () => {
    useParamsMock.mockReturnValue({});
    useGetUserDetailsMock.mockReturnValue({ data: { roles: [] } });
    useGetProjectDetailsMock.mockReturnValue({ data: {} });
    consumePendingSettingsTabMock.mockReturnValue(null);

    render(<SettingsPage />);
    expect(screen.getByText(/project not found/i)).toBeInTheDocument();
  });

  it("renders user-management tab composition by default", () => {
    useParamsMock.mockReturnValue({ projectId: "p-1" });
    useGetUserDetailsMock.mockReturnValue({ data: { roles: [] } });
    useGetProjectDetailsMock.mockReturnValue({ data: {} });
    consumePendingSettingsTabMock.mockReturnValue(null);

    render(<SettingsPage />);
    expect(screen.getByText("user-management")).toBeInTheDocument();
  });
});

