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
// software distributed under the License is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
import { SuccessBannerProvider } from "@context/success-banner/SuccessBannerContext";
import LoggerProvider from "@context/logger/LoggerProvider";
import SettingsPage from "@pages/SettingsPage";

vi.mock("react-router", () => ({
  useParams: () => ({ projectId: "project-1" }),
}));

vi.mock("@api/useGetProjectContacts", () => ({
  default: () => ({
    data: [],
    isFetching: false,
    error: null,
  }),
}));

vi.mock("@api/usePostProjectContact", () => ({
  usePostProjectContact: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}));

vi.mock("@api/useGetUserDetails", () => ({
  default: () => ({
    data: { roles: ["sn_customerservice.customer_admin"] },
  }),
}));


function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
}

function renderWithTheme(ui: React.ReactElement) {
  const queryClient = createTestQueryClient();
  return render(
    <ThemeProvider theme={createTheme({})}>
      <QueryClientProvider client={queryClient}>
        <LoggerProvider>
          <ErrorBannerProvider>
            <SuccessBannerProvider>{ui}</SuccessBannerProvider>
          </ErrorBannerProvider>
        </LoggerProvider>
      </QueryClientProvider>
    </ThemeProvider>,
  );
}

describe("SettingsPage", () => {
  it("renders User Management and AI Assistant tabs", () => {
    renderWithTheme(<SettingsPage />);
    expect(screen.getByRole("tab", { name: /User Management/i })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /AI Assistant/i })).toBeInTheDocument();
  });

  it("renders Add User button when User Management tab is active", () => {
    renderWithTheme(<SettingsPage />);
    expect(screen.getByRole("button", { name: /Add User/i })).toBeInTheDocument();
  });

  it("renders Role Permissions section when User Management tab is active", () => {
    renderWithTheme(<SettingsPage />);
    expect(screen.getByText("Role Permissions")).toBeInTheDocument();
  });

  it("renders AI content when AI Assistant tab is clicked", () => {
    renderWithTheme(<SettingsPage />);
    const aiTab = screen.getByRole("tab", { name: /AI Assistant/i });
    fireEvent.click(aiTab);
    expect(
      screen.getByText("AI-Powered Support Assistant"),
    ).toBeInTheDocument();
  });
});
