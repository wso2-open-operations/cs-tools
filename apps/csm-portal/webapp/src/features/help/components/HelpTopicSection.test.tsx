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
import "@testing-library/jest-dom/vitest";
import { describe, expect, it } from "vitest";
import { markdownToHtml } from "@utils/renderMarkdown";
import { sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import { resolveHelpImages } from "@features/help/utils/helpImages";
import HelpTopicSection from "./HelpTopicSection";

describe("HelpTopicSection", () => {
  it("renders a known topic's sanitized Markdown content from a plain topicId prop", () => {
    render(<HelpTopicSection topicId="overview" />);
    expect(screen.getByRole("heading", { name: "Overview" })).toBeVisible();
  });

  it("shows a not-found message for an unknown topic id rather than throwing", () => {
    render(<HelpTopicSection topicId="not-a-real-topic" />);
    expect(
      screen.getByText(/couldn.?t be found/i),
    ).toBeVisible();
  });

  it("never lets a raw <script> tag survive the render pipeline as real markup", () => {
    const hostile = "# Heading\n\n<script>window.__pwned = true;</script>\n\nSafe text.";
    const html = sanitizeRichTextHtml(resolveHelpImages(markdownToHtml(hostile)));
    const container = document.createElement("div");
    container.innerHTML = html;
    expect(container.querySelector("script")).toBeNull();
    expect(container.textContent).toContain("Safe text.");
  });

  it("never lets a raw onerror handler survive the render pipeline as a real attribute", () => {
    // A raw HTML img tag with an inline event handler, embedded in Markdown
    // source. `markdownToHtml` runs with `html: false`, so this is escaped to
    // inert text rather than parsed as an element -- confirmed here by
    // checking no live <img> with an "onerror" attribute exists afterwards.
    const hostile = '<img src="x" onerror="window.__pwned = true">';
    const html = sanitizeRichTextHtml(resolveHelpImages(markdownToHtml(hostile)));
    const container = document.createElement("div");
    container.innerHTML = html;
    const img = container.querySelector("img");
    expect(img?.getAttribute("onerror") ?? null).toBeNull();
    expect(container.querySelector("[onerror]")).toBeNull();
  });
});
