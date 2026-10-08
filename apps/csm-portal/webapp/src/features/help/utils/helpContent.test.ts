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
import { navNodeById } from "@config/csmNavItems";
import { HELP_TOPIC_CONTENT } from "@features/help/utils/helpContent";
import { markdownToHtmlProse } from "@utils/renderMarkdown";

describe("HELP_TOPIC_CONTENT", () => {
  it("has a non-empty Markdown source for every topic declared in the nav tree", () => {
    const help = navNodeById("help");
    const topicIds = (help?.children ?? []).map((child) =>
      child.id.replace(/^help\./, ""),
    );
    expect(topicIds.length).toBeGreaterThan(0);

    for (const id of topicIds) {
      expect(HELP_TOPIC_CONTENT[id]).toBeTruthy();
    }
  });

  it("declares no orphaned entries beyond the nav tree's topic list", () => {
    const help = navNodeById("help");
    const topicIds = new Set(
      (help?.children ?? []).map((child) => child.id.replace(/^help\./, "")),
    );
    for (const key of Object.keys(HELP_TOPIC_CONTENT)) {
      expect(topicIds.has(key)).toBe(true);
    }
  });
});

/**
 * Pins the shape of the Operations topic's "A time the customer proposed"
 * bullet as the Help page renders it (`markdownToHtmlProse`, `breaks: false`).
 * Markdown lazily continues a paragraph, so an indented line that follows a
 * nested bullet without a blank line in between is rendered inside that nested
 * bullet; the paragraphs about a stored time nobody proposed, about a dialog the
 * proposal moves behind, and about where proposals are kept belong to the bullet
 * itself, after its list.
 */
describe("Operations help: the proposed-time bullet", () => {
  const proposalItem = (): HTMLElement => {
    const doc = new DOMParser().parseFromString(
      markdownToHtmlProse(HELP_TOPIC_CONTENT.operations),
      "text/html",
    );
    const matches = Array.from(doc.querySelectorAll("li")).filter((li) =>
      li.querySelector(":scope > p > strong, :scope > strong")?.textContent?.startsWith("A time the customer proposed"),
    );
    expect(matches).toHaveLength(1);
    return matches[0];
  };

  it("keeps the two answers as the only items of the nested list", () => {
    const answers = Array.from(proposalItem().querySelectorAll(":scope > ul > li"));
    expect(answers.map((li) => li.querySelector("strong")?.textContent)).toEqual([
      "Accept proposed time",
      "Propose a different time",
    ]);
  });

  it("renders the three caveats (nobody recorded, a dialog that moved on, PostgreSQL only) as paragraphs of the bullet, after its list", () => {
    const children = Array.from(proposalItem().children).map((child) => child.tagName);
    expect(children).toEqual(["P", "UL", "P", "P", "P"]);

    const [, , notRecorded, movedOn, postgresOnly] = Array.from(proposalItem().children);
    expect(notRecorded.textContent).toContain("A date a WSO2 user wrote in the previous system");
    expect(notRecorded.textContent).toContain("nobody is recorded as having proposed it");
    expect(notRecorded.textContent).toContain("no staff action stands in for the customer's own answer");
    // The only thing on record is the last writer: a genuine proposal reads "nobody is recorded" after a staff edit, and the help says so.
    const oneLine = (notRecorded.textContent ?? "").replace(/\s+/g, " ");
    expect(oneLine).toContain("who last changed the change request");
    expect(oneLine).toContain("once anyone at WSO2 has edited the change request since");
    // Nothing asks the engineer to confirm and accept it anyway.
    expect(notRecorded.textContent).not.toMatch(/confirm|check that/i);
    expect(movedOn.textContent).toContain("dialog closes");
    expect(postgresOnly.textContent).toContain("kept in PostgreSQL only");
  });

  it("keeps the caveats out of the Propose a different time answer", () => {
    const answers = Array.from(proposalItem().querySelectorAll(":scope > ul > li"));
    const propose = answers[1];
    expect(propose.textContent).toContain("The loop repeats with their next proposal.");
    expect(propose.textContent).not.toContain("nobody is recorded");
    expect(propose.textContent).not.toContain("PostgreSQL");
    expect(propose.textContent).not.toContain("previous system");
  });
});
