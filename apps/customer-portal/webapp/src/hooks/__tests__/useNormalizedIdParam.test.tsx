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

import { type JSX } from "react";
import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import useNormalizedIdParam from "@hooks/useNormalizedIdParam";

// The same synthetic ids in both shapes: 32 hex characters, and the dashed
// 8-4-4-4-12 form of the same value.
const DASHLESS_PROJECT = "a1b2c3d4e5f60718293a4b5c6d7e8f90";
const DASHED_PROJECT = "a1b2c3d4-e5f6-0718-293a-4b5c6d7e8f90";
const DASHLESS_CASE = "0f1e2d3c4b5a69788796a5b4c3d2e1f0";
const DASHED_CASE = "0f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0";

/**
 * Renders the ids the hook resolved plus the live URL, so a test can assert on
 * both halves: what the page would fetch with, and what the address bar shows.
 * Asserting against the real router (rather than a mocked useNavigate) is what
 * makes the redirect itself observable.
 *
 * @param {(string | undefined)[]} values - Resolved ids, in order.
 * @returns {JSX.Element} The probe output.
 */
function Output({ values }: { values: (string | undefined)[] }): JSX.Element {
  const location = useLocation();
  return (
    <>
      <div data-testid="ids">{values.filter(Boolean).join(",")}</div>
      <div data-testid="url">
        {location.pathname}
        {location.search}
        {location.hash}
      </div>
    </>
  );
}

/** Probe for a route with a single id param. */
function OneIdProbe({ name }: { name: string }): JSX.Element {
  const value = useNormalizedIdParam(name);
  return <Output values={[value]} />;
}

/** Probe for a nested route carrying two id params. */
function TwoIdProbe({
  outer,
  inner,
}: {
  outer: string;
  inner: string;
}): JSX.Element {
  const outerValue = useNormalizedIdParam(outer);
  const innerValue = useNormalizedIdParam(inner);
  return <Output values={[outerValue, innerValue]} />;
}

function renderAt(
  initialUrl: string,
  routePath: string,
  params: string[],
): void {
  const element =
    params.length === 2 ? (
      <TwoIdProbe outer={params[0]} inner={params[1]} />
    ) : (
      <OneIdProbe name={params[0]} />
    );
  render(
    <MemoryRouter initialEntries={[initialUrl]}>
      <Routes>
        <Route path={routePath} element={element} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("useNormalizedIdParam", () => {
  it("returns a dashless id in dashed form on the very first render", () => {
    // The returned value matters more than the URL: it is what the page's data
    // hook fetches with, and it has to be correct before any redirect lands.
    renderAt(
      `/projects/${DASHLESS_PROJECT}`,
      "/projects/:projectId",
      ["projectId"],
    );

    expect(screen.getByTestId("ids").textContent).toBe(DASHED_PROJECT);
  });

  it("rewrites the URL to the dashed form", () => {
    renderAt(
      `/projects/${DASHLESS_PROJECT}`,
      "/projects/:projectId",
      ["projectId"],
    );

    expect(screen.getByTestId("url").textContent).toBe(
      `/projects/${DASHED_PROJECT}`,
    );
  });

  it("leaves an already-dashed id and its URL untouched", () => {
    renderAt(`/projects/${DASHED_PROJECT}`, "/projects/:projectId", [
      "projectId",
    ]);

    expect(screen.getByTestId("ids").textContent).toBe(DASHED_PROJECT);
    expect(screen.getByTestId("url").textContent).toBe(
      `/projects/${DASHED_PROJECT}`,
    );
  });

  it("normalizes two nested ids in one pass", () => {
    // The reason the rewrite is whole-path rather than per-param: a detail page
    // reads both projectId and its own entity id, so two instances of the hook
    // are live at once. Rewriting only its own value would let the last one to
    // run undo the other.
    renderAt(
      `/projects/${DASHLESS_PROJECT}/support/cases/${DASHLESS_CASE}`,
      "/projects/:projectId/support/cases/:caseId",
      ["projectId", "caseId"],
    );

    expect(screen.getByTestId("ids").textContent).toBe(
      `${DASHED_PROJECT},${DASHED_CASE}`,
    );
    expect(screen.getByTestId("url").textContent).toBe(
      `/projects/${DASHED_PROJECT}/support/cases/${DASHED_CASE}`,
    );
  });

  it("normalizes a nested id while the outer one is already dashed", () => {
    renderAt(
      `/projects/${DASHED_PROJECT}/support/cases/${DASHLESS_CASE}`,
      "/projects/:projectId/support/cases/:caseId",
      ["projectId", "caseId"],
    );

    expect(screen.getByTestId("ids").textContent).toBe(
      `${DASHED_PROJECT},${DASHED_CASE}`,
    );
    expect(screen.getByTestId("url").textContent).toBe(
      `/projects/${DASHED_PROJECT}/support/cases/${DASHED_CASE}`,
    );
  });

  it("preserves the query string and hash across the redirect", () => {
    renderAt(
      `/projects/${DASHLESS_PROJECT}/support/cases?tab=activity#comment-3`,
      "/projects/:projectId/support/cases",
      ["projectId"],
    );

    expect(screen.getByTestId("url").textContent).toBe(
      `/projects/${DASHED_PROJECT}/support/cases?tab=activity#comment-3`,
    );
  });

  it("returns undefined for an absent optional param without navigating", () => {
    // /support/chat has no :conversationId — the index route starts a new
    // conversation, so the hook must not invent a value or redirect.
    renderAt("/support/chat", "/support/chat", ["conversationId"]);

    expect(screen.getByTestId("ids").textContent).toBe("");
    expect(screen.getByTestId("url").textContent).toBe("/support/chat");
  });

  it.each([
    ["31 hex", "a1b2c3d4e5f60718293a4b5c6d7e8f9"],
    ["33 hex", "a1b2c3d4e5f60718293a4b5c6d7e8f900"],
    ["non-hex", "a1b2c3d4e5f60718293a4b5c6d7e8fzz"],
    ["a slug", "not-an-id"],
  ])("leaves a %s param alone", (_label, value) => {
    // Malformed ids are deliberately not repaired or rejected here — the page's
    // own error state still handles them.
    renderAt(`/projects/${value}`, "/projects/:projectId", ["projectId"]);

    expect(screen.getByTestId("ids").textContent).toBe(value);
    expect(screen.getByTestId("url").textContent).toBe(`/projects/${value}`);
  });
});
