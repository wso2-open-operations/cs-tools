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
import { afterEach, describe, expect, it, vi } from "vitest";
import SettingsPage from "@features/settings/pages/SettingsPage";

const useParamsMock = vi.fn();
const consumePendingSettingsTabMock = vi.fn();
const userDetailsMock = vi.fn();
const projectDetailsMock = vi.fn();

vi.mock("react-router", () => ({
  useParams: () => useParamsMock(),
}));

vi.mock("@components/tab-bar/TabBar", () => ({
  default: ({
    activeTab,
    tabs,
  }: {
    activeTab: string;
    tabs: Array<{ id: string }>;
  }) => (
    <div>
      <div>tab:{activeTab}</div>
      <div>tabs:{tabs.map((t) => t.id).join(",")}</div>
    </div>
  ),
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

vi.mock("@features/settings/components/SettingsVerification", () => ({
  default: () => <div>verification-settings</div>,
}));

vi.mock("@features/settings/api/useGetUserDetails", () => ({
  default: () => userDetailsMock(),
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => projectDetailsMock(),
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
    userDetailsMock.mockReturnValue({ data: { roles: [] } });
    projectDetailsMock.mockReturnValue({ data: {} });
    consumePendingSettingsTabMock.mockReturnValue(null);

    render(<SettingsPage />);
    expect(screen.getByText(/project not found/i)).toBeInTheDocument();
  });

  it("renders user-management tab composition by default", () => {
    useParamsMock.mockReturnValue({ projectId: "p-1" });
    userDetailsMock.mockReturnValue({ data: { roles: [] } });
    projectDetailsMock.mockReturnValue({ data: {} });
    consumePendingSettingsTabMock.mockReturnValue(null);

    render(<SettingsPage />);
    expect(screen.getByText("user-management")).toBeInTheDocument();
  });

  describe("CUSTOMER_PORTAL_ALLOW_PENDING_VERIFICATION_SETTINGS kill switch", () => {
    afterEach(() => {
      window.config = {
        ...window.config,
        CUSTOMER_PORTAL_ALLOW_PENDING_VERIFICATION_SETTINGS: undefined,
      };
    });

    it("shows the Pending Verification tab and renders it when enabled (default)", () => {
      useParamsMock.mockReturnValue({ projectId: "p-1" });
      userDetailsMock.mockReturnValue({ data: { roles: [] } });
      projectDetailsMock.mockReturnValue({ data: {} });
      consumePendingSettingsTabMock.mockReturnValue("verification");

      render(<SettingsPage />);

      expect(screen.getByText(/tabs:.*verification/)).toBeInTheDocument();
      expect(screen.getByText("verification-settings")).toBeInTheDocument();
    });

    it("hides the tab from the tab bar when explicitly disabled", () => {
      window.config = {
        ...window.config,
        CUSTOMER_PORTAL_ALLOW_PENDING_VERIFICATION_SETTINGS: false,
      };
      useParamsMock.mockReturnValue({ projectId: "p-1" });
      userDetailsMock.mockReturnValue({ data: { roles: [] } });
      projectDetailsMock.mockReturnValue({ data: {} });
      consumePendingSettingsTabMock.mockReturnValue(null);

      render(<SettingsPage />);

      const tabsLine = screen.getByText(/^tabs:/);
      expect(tabsLine.textContent).not.toContain("verification");
    });

    it("redirects a stored/deep-linked verification tab to User Management when disabled", () => {
      window.config = {
        ...window.config,
        CUSTOMER_PORTAL_ALLOW_PENDING_VERIFICATION_SETTINGS: false,
      };
      useParamsMock.mockReturnValue({ projectId: "p-1" });
      userDetailsMock.mockReturnValue({ data: { roles: [] } });
      projectDetailsMock.mockReturnValue({ data: {} });
      consumePendingSettingsTabMock.mockReturnValue("verification");

      render(<SettingsPage />);

      expect(screen.queryByText("verification-settings")).not.toBeInTheDocument();
      expect(screen.getByText("user-management")).toBeInTheDocument();
    });
  });
});

