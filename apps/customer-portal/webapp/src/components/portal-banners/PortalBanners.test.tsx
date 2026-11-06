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
import { PortalBannerList } from "@components/portal-banners/PortalBanners";

const store = new Map<string, string>();

const banner = {
  id: "11111111-1111-4111-8111-111111111111",
  sortOrder: 0,
  html: `<div data-banner="1">Hello engineers<button type="button" data-close-banner="true">Close</button></div>`,
  startsAt: null,
  expiresAt: null,
};

describe("PortalBannerList", () => {
  beforeEach(() => {
    store.clear();
    vi.stubGlobal("localStorage", {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => {
        store.set(key, value);
      },
      removeItem: (key: string) => {
        store.delete(key);
      },
      clear: () => {
        store.clear();
      },
    });
  });

  it("shows a current banner and dismisses it from the close button", () => {
    render(<PortalBannerList banners={[banner]} />);
    expect(screen.getByText("Hello engineers")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(screen.queryByText("Hello engineers")).not.toBeInTheDocument();
    expect(store.get(`portal_banner_dismissed:${banner.id}`)).toBe("dismissed");
  });

  it("hides a banner that has not started", () => {
    render(
      <PortalBannerList banners={[{ ...banner, id: "later", startsAt: "2099-01-01T00:00:00.000Z" }]} />,
    );
    expect(screen.queryByText("Hello engineers")).not.toBeInTheDocument();
  });
});
