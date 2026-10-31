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
  currentPageEntry,
  isKnownPage,
} from "@features/csm-recent/currentPageEntry";
import { navNodeById } from "@config/csmNavItems";
import {
  CHANGE_REQUEST_LIST_PARAM_PREFIX,
  CHANGE_REQUESTS_VIEW_PARAM,
  STANDARD_CHANGE_PROPOSALS_PATH,
  STANDARD_CHANGES_LABELS,
  STANDARD_CHANGES_PATH,
  STANDARD_CHANGES_SHOW_PARAM,
} from "@features/csm-operations/utils/standardChanges";

describe("currentPageEntry — section landing routes", () => {
  it("pins a section under its own name", () => {
    expect(currentPageEntry("/cases", "")).toEqual({
      kind: "page",
      id: "/cases",
      title: "Support",
      href: "/cases",
    });
  });

  it("treats an empty query as no query", () => {
    expect(currentPageEntry("/cases", "?")).toMatchObject({ kind: "page" });
  });

  it("pins a filtered section view as a search", () => {
    expect(currentPageEntry("/cases", "?state=open")).toEqual({
      kind: "search",
      id: "/cases?state=open",
      title: "Support: 1 filter",
      href: "/cases?state=open",
    });
  });

  it("quotes a free-text query in the title", () => {
    expect(currentPageEntry("/cases", "?q=timeout").title).toBe(
      "Support: “timeout”",
    );
  });
});

describe("currentPageEntry — route-backed tabs", () => {
  it("pins a tab under its own name, not its section's", () => {
    expect(currentPageEntry("/customers/accounts", "")).toEqual({
      kind: "page",
      id: "/customers/accounts",
      title: "Accounts",
      href: "/customers/accounts",
    });
  });
});

describe("currentPageEntry — query-backed tabs", () => {
  it("pins ?tab= as the tab itself rather than a filter of its section", () => {
    expect(currentPageEntry("/operations", "?tab=incidents")).toEqual({
      kind: "page",
      id: "/operations?tab=incidents",
      title: "Incidents",
      href: "/operations?tab=incidents",
    });
  });

  it("gives each tab a distinct pin", () => {
    const incidents = currentPageEntry("/operations", "?tab=incidents");
    const problems = currentPageEntry("/operations", "?tab=problems");
    expect(problems.title).toBe("Problem management");
    expect(problems.id).not.toBe(incidents.id);
  });

  it("counts the remaining parameters as filters, excluding the tab", () => {
    expect(currentPageEntry("/operations", "?tab=incidents&state=open")).toEqual(
      {
        kind: "search",
        id: "/operations?tab=incidents&state=open",
        title: "Incidents: 1 filter",
        href: "/operations?tab=incidents&state=open",
      },
    );
  });

  it("falls back to the section for an unrecognised tab value", () => {
    expect(currentPageEntry("/operations", "?tab=nope").title).toBe(
      "Operations: 1 filter",
    );
  });

  it("still treats a non-tab query on the section as a filter", () => {
    expect(currentPageEntry("/operations", "?state=open").title).toBe(
      "Operations: 1 filter",
    );
  });
});

describe("currentPageEntry — a named place inside a page (Standard changes and its proposals)", () => {
  it.each([
    ["?view=standard", "Standard changes"],
    ["?view=standard&show=proposals", "Standard change proposals"],
  ])("pins %s as the place itself, not a filter of Change requests", (search, title) => {
    expect(currentPageEntry("/operations/change-requests", search)).toEqual({
      kind: "page",
      id: `/operations/change-requests${search}`,
      title,
      href: `/operations/change-requests${search}`,
    });
  });

  it("gives Standard changes, its proposals and the list each a distinct pin", () => {
    const ids = [
      currentPageEntry("/operations/change-requests", "").id,
      currentPageEntry("/operations/change-requests", "?view=standard").id,
      currentPageEntry("/operations/change-requests", "?view=standard&show=proposals").id,
    ];
    expect(new Set(ids).size).toBe(3);
    expect(currentPageEntry("/operations/change-requests", "").title).toBe("Change requests");
  });

  it("names the place first and counts only the other parameters as filters", () => {
    expect(
      currentPageEntry("/operations/change-requests", "?view=standard&show=proposals&scope=awaiting"),
    ).toEqual({
      kind: "search",
      id: "/operations/change-requests?view=standard&show=proposals&scope=awaiting",
      title: "Standard change proposals: 1 filter",
      href: "/operations/change-requests?view=standard&show=proposals&scope=awaiting",
    });
    expect(
      currentPageEntry("/operations/change-requests", "?view=standard&show=proposals&scope=mine&state=draft").title,
    ).toBe("Standard change proposals: 2 filters");
  });

  it("ignores the change request list's cr* filters kept in the URL on Standard changes", () => {
    expect(currentPageEntry("/operations/change-requests", "?view=standard&crQ=gateway&crStates=new")).toEqual({
      kind: "page",
      id: "/operations/change-requests?view=standard",
      title: "Standard changes",
      href: "/operations/change-requests?view=standard",
    });
    expect(currentPageEntry("/operations/change-requests", "?crQ=gateway&view=standard&show=proposals").title).toBe(
      "Standard change proposals",
    );
    // Its own filters still count, and the pin's href leaves the list's out.
    expect(
      currentPageEntry("/operations/change-requests", "?view=standard&show=proposals&scope=awaiting&crQ=gateway"),
    ).toEqual({
      kind: "search",
      id: "/operations/change-requests?view=standard&show=proposals&scope=awaiting",
      title: "Standard change proposals: 1 filter",
      href: "/operations/change-requests?view=standard&show=proposals&scope=awaiting",
    });
    expect(currentPageEntry("/operations", "?tab=change_requests&view=standard&crQ=x").title).toBe("Standard changes");
  });

  it("names the old sub-view links after the places they now land on", () => {
    expect(currentPageEntry("/operations/change-requests", "?view=catalogue").title).toBe("Standard changes");
    expect(currentPageEntry("/operations/change-requests", "?view=proposals").title).toBe(
      "Standard change proposals",
    );
  });

  it("still reads the change request list's own filters as filters", () => {
    expect(currentPageEntry("/operations/change-requests", "?crQ=gateway").title).toBe(
      "Change requests: 1 filter",
    );
  });

  it("reads a sub-view it does not know as a filter, as the page shows the list for it", () => {
    expect(currentPageEntry("/operations/change-requests", "?view=nonsense").title).toBe(
      "Change requests: 1 filter",
    );
  });

  it("resolves the place on the legacy ?tab= form too", () => {
    expect(currentPageEntry("/operations", "?tab=change_requests&view=standard")).toEqual({
      kind: "page",
      id: "/operations?tab=change_requests&view=standard",
      title: "Standard changes",
      href: "/operations?tab=change_requests&view=standard",
    });
  });

  it("does not treat `view` as a place on a page that declares none", () => {
    expect(currentPageEntry("/operations/incidents", "?view=standard").title).toBe(
      "Incidents: 1 filter",
    );
  });

  it("uses the same parameters and labels as the Change requests sub-navigation", () => {
    expect(CHANGE_REQUESTS_VIEW_PARAM).toBe("view");
    expect(STANDARD_CHANGES_SHOW_PARAM).toBe("show");
    expect(navNodeById("operations.change-requests")?.views?.ignoreParamPrefix).toBe(CHANGE_REQUEST_LIST_PARAM_PREFIX);
    expect(currentPageEntry("/operations/change-requests", new URL(STANDARD_CHANGES_PATH, "http://x").search).title).toBe(
      STANDARD_CHANGES_LABELS.templates,
    );
    expect(
      currentPageEntry("/operations/change-requests", new URL(STANDARD_CHANGE_PROPOSALS_PATH, "http://x").search).title,
    ).toBe(STANDARD_CHANGES_LABELS.proposals);
  });
});

describe("currentPageEntry — unknown routes", () => {
  it("labels from the first path segment", () => {
    expect(currentPageEntry("/security-center/reports/new", "")).toEqual({
      kind: "page",
      id: "/security-center/reports/new",
      title: "Security center",
      href: "/security-center/reports/new",
    });
  });

  it("falls back to a generic title at the root", () => {
    expect(currentPageEntry("/nothing-here", "").title).toBe("Nothing here");
  });
});

describe("isKnownPage", () => {
  it("accepts section landing routes and tab routes alike", () => {
    expect(isKnownPage("/cases")).toBe(true);
    expect(isKnownPage("/customers/accounts")).toBe(true);
    expect(isKnownPage("/operations/incidents")).toBe(true);
  });

  it("rejects a detail route and an unknown path", () => {
    expect(isKnownPage("/operations/incidents/INC0001")).toBe(false);
    expect(isKnownPage("/nothing-here")).toBe(false);
  });
});
