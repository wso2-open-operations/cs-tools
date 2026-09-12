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

import { fireEvent, render, screen, within } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { navNodeById } from "@config/csmNavItems";
import { resetFeatureStatesForTests } from "@config/featureFlags";
import HelpPage from "./HelpPage";

function setOverrides(value: unknown): void {
  window.config = {
    ...window.config,
    CSM_PORTAL_FEATURE_OVERRIDES: value,
  } as Window["config"];
  resetFeatureStatesForTests();
}

beforeEach(() => {
  vi.spyOn(console, "warn").mockImplementation(() => undefined);
  setOverrides(undefined);
  window.location.hash = "";
});

afterEach(() => {
  vi.restoreAllMocks();
  window.location.hash = "";
});

describe("HelpPage", () => {
  it("renders a sidebar link per enabled topic, in nav order, pointing at that topic's hash", () => {
    render(<HelpPage />);
    const nav = screen.getByRole("navigation", { name: "Help topics" });
    const help = navNodeById("help");
    const labels = (help?.children ?? []).map((child) => child.label);

    const links = within(nav).getAllByRole("link");
    expect(links.map((link) => link.textContent)).toEqual(labels);
    expect(screen.getByRole("link", { name: "Operations" })).toHaveAttribute(
      "href",
      "#operations",
    );
  });

  it("shows only the first topic's content on a bare /help load", () => {
    render(<HelpPage />);
    expect(screen.getByRole("heading", { name: "Overview" })).toBeVisible();
    expect(screen.queryByRole("heading", { name: "Operations" })).toBeNull();
  });

  it("corrects a bare /help load's empty hash in place, without adding a history entry", () => {
    const lengthBefore = window.history.length;
    render(<HelpPage />);

    expect(window.location.hash).toBe("#overview");
    expect(window.history.length).toBe(lengthBefore);
  });

  it("corrects the hash in place when it names a disabled topic, without adding a history entry", () => {
    window.location.hash = "#operations";
    setOverrides({ "help.operations": "hidden" });
    const lengthBefore = window.history.length;
    render(<HelpPage />);

    expect(window.location.hash).toBe("#overview");
    expect(window.history.length).toBe(lengthBefore);
  });

  it("pushes a real history entry for a genuine topic-to-topic navigation", () => {
    render(<HelpPage />);
    const lengthBefore = window.history.length;

    fireEvent.click(screen.getByRole("link", { name: /^Next topic:/ }));

    expect(window.location.hash).toBe("#workspace-basics");
    expect(window.history.length).toBe(lengthBefore + 1);
  });

  it("shows the topic named by the URL hash on a direct /help#<topic> load", () => {
    window.location.hash = "#operations";
    render(<HelpPage />);
    expect(screen.getByRole("heading", { name: "Operations" })).toBeVisible();
    expect(screen.queryByRole("heading", { name: "Overview" })).toBeNull();
  });

  it("switches the shown topic when a sidebar link is clicked, without changing the route", () => {
    render(<HelpPage />);
    fireEvent.click(screen.getByRole("link", { name: "Operations" }));

    expect(screen.getByRole("heading", { name: "Operations" })).toBeVisible();
    expect(screen.queryByRole("heading", { name: "Overview" })).toBeNull();
    expect(
      screen.getByRole("link", { name: "Operations" }),
    ).toHaveAttribute("aria-current", "page");
  });

  it("drops a topic from the sidebar once it's disabled", () => {
    setOverrides({ "help.operations": "hidden" });
    render(<HelpPage />);
    expect(screen.queryByRole("link", { name: "Operations" })).toBeNull();
  });

  it("falls back to the first enabled topic when the hash names a disabled one", () => {
    window.location.hash = "#operations";
    setOverrides({ "help.operations": "hidden" });
    render(<HelpPage />);
    expect(screen.getByRole("heading", { name: "Overview" })).toBeVisible();
  });

  it("offers a Next-topic link to the following topic, and follows it on click", () => {
    render(<HelpPage />);
    const nextLink = screen.getByRole("link", {
      name: "Next topic: Navigation & personalization",
    });
    fireEvent.click(nextLink);

    expect(
      screen.getByRole("heading", { name: "Navigation & personalization" }),
    ).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Previous topic: Overview" }),
    ).toBeVisible();
  });

  it("shows no Previous-topic link on the first topic and no Next-topic link on the last", () => {
    render(<HelpPage />);
    expect(screen.queryByRole("link", { name: /^Previous topic:/ })).toBeNull();
    expect(screen.getByRole("link", { name: /^Next topic:/ })).toBeVisible();

    const help = navNodeById("help");
    const lastLabel = (help?.children ?? []).at(-1)?.label as string;
    fireEvent.click(screen.getByRole("link", { name: lastLabel }));

    expect(screen.queryByRole("link", { name: /^Next topic:/ })).toBeNull();
    expect(screen.getByRole("link", { name: /^Previous topic:/ })).toBeVisible();
  });

  it("narrows the sidebar list to topics matching the search box, without changing the shown topic", () => {
    render(<HelpPage />);
    fireEvent.change(screen.getByPlaceholderText("Search topics…"), {
      target: { value: "security" },
    });

    expect(screen.getByRole("link", { name: "Security Center" })).toBeVisible();
    expect(screen.queryByRole("link", { name: "Operations" })).toBeNull();
    // The content pane still shows whatever was active before filtering.
    expect(screen.getByRole("heading", { name: "Overview" })).toBeVisible();
  });

  it("surfaces a topic whose content (not its title) matches, with a snippet explaining why", () => {
    // "Incidents" is a tab inside the Operations topic, not a Help topic of
    // its own — searching it should still surface Operations rather than
    // returning nothing.
    render(<HelpPage />);
    fireEvent.change(screen.getByPlaceholderText("Search topics…"), {
      target: { value: "incidents" },
    });

    const operationsLink = screen.getByRole("link", { name: /Operations/ });
    expect(operationsLink).toBeVisible();
    expect(within(operationsLink).getByText("incidents")).toBeVisible();
  });

  it("shows a no-match message when the search box matches no topic", () => {
    render(<HelpPage />);
    fireEvent.change(screen.getByPlaceholderText("Search topics…"), {
      target: { value: "nonexistent-topic" },
    });

    expect(screen.getByText(/No topics match .nonexistent-topic./)).toBeVisible();
  });

  it("updates the shown topic when the URL hash changes after mount (browser back/forward)", () => {
    render(<HelpPage />);
    window.location.hash = "#operations";
    fireEvent(window, new HashChangeEvent("hashchange"));

    expect(screen.getByRole("heading", { name: "Operations" })).toBeVisible();
  });
});
