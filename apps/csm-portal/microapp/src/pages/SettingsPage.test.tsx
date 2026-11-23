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

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { getPortalAccess } from "@src/access/portalAccess";

const state = vi.hoisted(() => ({ access: undefined as unknown }));

vi.mock("@src/access/usePortalAccess", () => ({
  usePortalAccess: () => ({ access: state.access }),
}));
vi.mock("@components/settings/Settings", () => ({
  default: () => <p>the user list</p>,
}));
vi.mock("@wso2/oxygen-ui", () => {
  const box = ({ children }: { children?: React.ReactNode }) => <div>{children}</div>;
  return { Stack: box, Typography: box };
});
vi.mock("@wso2/oxygen-ui-icons-react", () => ({ Ban: () => <i /> }));

import SettingsPage from "./SettingsPage";

describe("SettingsPage", () => {
  it("does not mount the user list for a viewer", () => {
    state.access = getPortalAccess(["viewer"]);
    expect(renderToStaticMarkup(<SettingsPage />)).not.toContain("the user list");
  });

  it("does not mount the user list for a catalogue manager without canViewSettings", () => {
    state.access = getPortalAccess(["viewer", "product_catalog_manager"]);
    expect(renderToStaticMarkup(<SettingsPage />)).not.toContain("the user list");
  });

  it("mounts the user list for a CS engineer", () => {
    state.access = getPortalAccess(["cs_engineer"]);
    expect(renderToStaticMarkup(<SettingsPage />)).toContain("the user list");
  });
});
