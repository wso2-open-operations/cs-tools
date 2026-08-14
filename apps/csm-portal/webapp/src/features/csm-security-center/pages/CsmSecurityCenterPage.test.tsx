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

/**
 * Exercises CsmSecurityCenterPage's tab strip against the `/security-center`
 * and `/security-center/:tab` routes exactly as declared in App.tsx, plus the
 * legacy `?tab=` redirect — same convention as OperationsPage.test.tsx. Each
 * tab's own content component is mocked to a marker div, since this page's
 * job is picking which one renders, not what they render.
 */

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import "@testing-library/jest-dom/vitest";
import { resetFeatureStatesForTests } from "@config/featureFlags";

function setOverrides(value: unknown): void {
  window.config = {
    ...window.config,
    CSM_PORTAL_FEATURE_OVERRIDES: value,
  } as Window["config"];
  resetFeatureStatesForTests();
}

vi.mock("@features/csm-cases/components/CsmIssuesView", () => ({
  default: () => <div>Security reports content</div>,
}));
vi.mock("@features/csm-security-center/components/ProductVulnerabilitiesTab", () => ({
  default: () => <div>Vulnerabilities content</div>,
}));

import CsmSecurityCenterPage from "@features/csm-security-center/pages/CsmSecurityCenterPage";

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

function renderAt(initialEntry: string) {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <LocationProbe />
      <Routes>
        <Route path="/security-center" element={<CsmSecurityCenterPage />} />
        <Route path="/security-center/:tab" element={<CsmSecurityCenterPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("CsmSecurityCenterPage", () => {
  beforeEach(() => {
    setOverrides(undefined);
  });

  afterEach(() => {
    setOverrides(undefined);
  });

  it("renders the requested tab's content off a known URL segment", () => {
    renderAt("/security-center/vulnerabilities");
    expect(screen.getByText("Vulnerabilities content")).toBeInTheDocument();
  });

  it("defaults to the security reports tab with no segment", () => {
    renderAt("/security-center");
    expect(screen.getByText("Security reports content")).toBeInTheDocument();
  });

  it("switching tabs updates the URL to the new path segment", () => {
    renderAt("/security-center/security_reports");

    fireEvent.click(screen.getByRole("tab", { name: "Vulnerabilities" }));

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/security-center/vulnerabilities",
    );
    expect(screen.getByText("Vulnerabilities content")).toBeInTheDocument();
  });

  it("renders a WIP tab disabled with its chip, and falls back rendering to the first enabled tab", () => {
    setOverrides({ "security-center.vulnerabilities": "wip" });
    renderAt("/security-center/vulnerabilities");

    const wipTab = screen.getByRole("tab", { name: /vulnerabilities/i });
    expect(wipTab).toBeDisabled();
    expect(screen.getByText("WIP")).toBeInTheDocument();
    // Requesting a WIP tab by URL renders the first enabled tab instead.
    expect(screen.getByText("Security reports content")).toBeInTheDocument();
  });

  it("omits a hidden tab from the strip entirely and falls back rendering to the first enabled tab", () => {
    setOverrides({ "security-center.vulnerabilities": "hidden" });
    renderAt("/security-center/vulnerabilities");

    expect(
      screen.queryByRole("tab", { name: /vulnerabilities/i }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Security reports content")).toBeInTheDocument();
  });

  it("falls back to the first enabled tab for an unknown URL segment", () => {
    renderAt("/security-center/not-a-real-tab");
    expect(screen.getByText("Security reports content")).toBeInTheDocument();
  });

  it("redirects a legacy ?tab= URL to the equivalent path-segment URL", async () => {
    renderAt("/security-center?tab=vulnerabilities");

    await waitFor(() =>
      expect(screen.getByTestId("location-probe")).toHaveTextContent(
        "/security-center/vulnerabilities",
      ),
    );
    expect(screen.getByText("Vulnerabilities content")).toBeInTheDocument();
  });

  it("does not redirect when there is no legacy ?tab= query", () => {
    renderAt("/security-center");
    expect(screen.getByTestId("location-probe")).toHaveTextContent("/security-center");
  });
});
