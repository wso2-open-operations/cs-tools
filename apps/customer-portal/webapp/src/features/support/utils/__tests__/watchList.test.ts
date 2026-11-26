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


import { describe, expect, it } from "vitest";
import {
  diffWatchList,
  isEligibleWatcher,
  isRegisteredContact,
} from "@features/support/utils/watchList";

const base = {
  isCsAdmin: false,
  isCsIntegrationUser: false,
  isPortalUser: true,
  isSecurityContact: false,
  membershipStatus: "REGISTERED",
};

describe("isEligibleWatcher", () => {
  it("accepts a registered contact with an eligible role", () => {
    expect(isEligibleWatcher(base)).toBe(true);
  });

  it.each(["INVITED", "RE-INVITED", "DEACTIVATED", "", "unknown"])(
    "rejects membership status %j",
    (membershipStatus) => {
      expect(isEligibleWatcher({ ...base, membershipStatus })).toBe(false);
    },
  );

  it("rejects a missing membership status", () => {
    expect(
      isEligibleWatcher({
        ...base,
        membershipStatus: undefined as unknown as string,
      }),
    ).toBe(false);
  });

  it("compares the status trimmed and case-insensitively", () => {
    expect(isEligibleWatcher({ ...base, membershipStatus: " registered " })).toBe(
      true,
    );
  });

  it("rejects a registered security-only contact", () => {
    expect(
      isEligibleWatcher({
        ...base,
        isPortalUser: false,
        isSecurityContact: true,
      }),
    ).toBe(false);
  });

  it.each([{ isCsAdmin: true }, { isCsIntegrationUser: true }])(
    "accepts a registered security contact with role %j",
    (role) => {
      expect(
        isEligibleWatcher({
          ...base,
          isPortalUser: false,
          isSecurityContact: true,
          ...role,
        }),
      ).toBe(true);
    },
  );
});

describe("isRegisteredContact", () => {
  it.each(["REGISTERED", "registered", "  Registered "])("accepts %j", (membershipStatus) => {
    expect(isRegisteredContact({ membershipStatus })).toBe(true);
  });

  it.each(["INVITED", "RE-INVITED", "DEACTIVATED", "", undefined])(
    "rejects %j",
    (membershipStatus) => {
      expect(
        isRegisteredContact({ membershipStatus: membershipStatus as string }),
      ).toBe(false);
    },
  );
});

describe("diffWatchList", () => {
  it("returns only what was added and what was removed", () => {
    expect(diffWatchList(["a@x.com", "b@x.com"], ["b@x.com", "c@x.com"])).toEqual({
      addWatchers: ["c@x.com"],
      removeWatchers: ["a@x.com"],
    });
  });

  it("is empty when nothing changed, so nothing needs to be sent", () => {
    expect(diffWatchList(["a@x.com"], ["a@x.com"])).toEqual({ addWatchers: [], removeWatchers: [] });
  });

  it("never names someone who was left alone, so a watcher a colleague added meanwhile is untouched", () => {
    const { addWatchers, removeWatchers } = diffWatchList(["a@x.com"], ["a@x.com", "new@x.com"]);
    expect(addWatchers).toEqual(["new@x.com"]);
    expect(removeWatchers).toEqual([]);
  });

  it("compares emails ignoring case and surrounding spaces", () => {
    expect(diffWatchList(["Alice@X.com"], [" alice@x.com "])).toEqual({
      addWatchers: [],
      removeWatchers: [],
    });
  });

  it("can remove the last watcher", () => {
    expect(diffWatchList(["a@x.com"], [])).toEqual({ addWatchers: [], removeWatchers: ["a@x.com"] });
  });

  it("does not repeat an email listed twice", () => {
    expect(diffWatchList([], ["a@x.com", "A@x.com"]).addWatchers).toEqual(["a@x.com"]);
  });
});
