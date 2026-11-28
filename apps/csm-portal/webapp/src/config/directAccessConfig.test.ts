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
import { afterEach, describe, expect, it } from "vitest";
import { directAccessEnabled, hostAppAddressFor, hostAppUrl } from "@config/directAccessConfig";

function setConfig(config: Record<string, unknown>): void {
  window.config = config as unknown as typeof window.config;
}

afterEach(() => {
  window.config = {} as typeof window.config;
});

const page = { origin: "https://portal.example", pathname: "/", search: "", hash: "" };

describe("directAccessEnabled", () => {
  it("is on when the key is not set, so an existing deployment is unchanged", () => {
    setConfig({});
    expect(directAccessEnabled()).toBe(true);
  });

  it("is on for true, the string true and anything unrecognised", () => {
    for (const on of [true, "true", "yes", "", 1, null]) {
      setConfig({ CSM_PORTAL_DIRECT_ACCESS_ENABLED: on });
      expect(directAccessEnabled()).toBe(true);
    }
  });

  it("is off for false or the string false", () => {
    for (const off of [false, "false", " False "]) {
      setConfig({ CSM_PORTAL_DIRECT_ACCESS_ENABLED: off });
      expect(directAccessEnabled()).toBe(false);
    }
  });
});

describe("hostAppUrl", () => {
  it("returns the configured address without a trailing slash", () => {
    setConfig({ CSM_PORTAL_HOST_APP_URL: " https://host.example/csm/ " });
    expect(hostAppUrl()).toBe("https://host.example/csm");
  });

  it("returns nothing when unset, not a string, relative or not http(s)", () => {
    for (const bad of [undefined, 42, "", "/csm", "host.example/csm", "javascript:alert(1)"]) {
      setConfig({ CSM_PORTAL_HOST_APP_URL: bad });
      expect(hostAppUrl()).toBe("");
    }
  });
});

describe("hostAppAddressFor", () => {
  it("points the portal's home at the host's mount point", () => {
    setConfig({ CSM_PORTAL_HOST_APP_URL: "https://host.example/csm" });
    expect(hostAppAddressFor(page)).toBe("https://host.example/csm");
  });

  it("keeps the page, query and anchor of a deep link", () => {
    setConfig({ CSM_PORTAL_HOST_APP_URL: "https://host.example/csm/" });
    expect(hostAppAddressFor({ ...page, pathname: "/cases/123", search: "?tab=activity", hash: "#description" })).toBe(
      "https://host.example/csm/cases/123?tab=activity#description",
    );
  });

  it("puts the page on the path of a host address that has its own query", () => {
    setConfig({ CSM_PORTAL_HOST_APP_URL: "https://host.example/csm?tenant=demo" });
    expect(hostAppAddressFor({ ...page, pathname: "/cases/123", search: "?tab=activity", hash: "#description" })).toBe(
      "https://host.example/csm/cases/123?tenant=demo&tab=activity#description",
    );
    expect(hostAppAddressFor(page)).toBe("https://host.example/csm?tenant=demo");
  });

  it("puts the page on the path of a host address that has its own anchor", () => {
    setConfig({ CSM_PORTAL_HOST_APP_URL: "https://host.example/csm#top" });
    expect(hostAppAddressFor({ ...page, pathname: "/cases/123" })).toBe("https://host.example/csm/cases/123#top");
    expect(hostAppAddressFor({ ...page, pathname: "/cases/123", hash: "#description" })).toBe(
      "https://host.example/csm/cases/123#description",
    );
  });

  it("works for a host that mounts the portal at its root", () => {
    setConfig({ CSM_PORTAL_HOST_APP_URL: "https://host.example/" });
    expect(hostAppAddressFor({ ...page, pathname: "/cases/123" })).toBe("https://host.example/cases/123");
  });

  it("has nowhere to send anyone without a usable host address", () => {
    setConfig({});
    expect(hostAppAddressFor(page)).toBeNull();
  });

  it("does not send anyone back to this portal's own origin", () => {
    setConfig({ CSM_PORTAL_HOST_APP_URL: "https://portal.example/csm" });
    expect(hostAppAddressFor(page)).toBeNull();
  });
});
