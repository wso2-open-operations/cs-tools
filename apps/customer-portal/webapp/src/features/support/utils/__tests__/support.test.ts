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

import { describe, expect, it, vi } from "vitest";
import {
  downloadSizeOf,
  getAvailableCaseActions,
  getInitials,
  splitStaffMark,
  collapseCodeBlockWhitespace,
  collapseCommentSourceWhitespace,
  collapseHtmlSourceWhitespace,
  compareByCreatedOnThenId,
  computeMinScheduleDatetimeLocalForTimeZone,
  convertCodeTagsToHtml,
  dedupeAdjacentDuplicateMessages,
  deriveFilterLabels,
  extractInlineImageRefId,
  isInlineImageRefSrc,
  isNoveraOrBotSender,
  withSystemAuthor,
  SYSTEM_AUTHOR_LABEL,
  hasSingleCodeWrapper,
  hasSubmittableEditorContent,
  linkifyBareUrls,
  normalizeCaseTypeOptions,
  plainTextCommentToHtml,
  replaceInlineImageSources,
  resolveActionStateKey,
  stripCodeWrapper,
  toUtcEndOfDay,
  toUtcStartOfDay,
} from "@features/support/utils/support";

describe("extractInlineImageRefId", () => {
  it("extracts 32-char id from service-now style .iix paths", () => {
    expect(
      extractInlineImageRefId(
        "https://host/path/1234567890abcdef1234567890abcdef.iix?x=1",
      ),
    ).toBe("1234567890abcdef1234567890abcdef");
  });
});

describe("replaceInlineImageSources", () => {
  it("replaces matching inline image with preview url and keeps unmatched", () => {
    const html =
      "<p>Images</p><img src='/1234567890abcdef1234567890abcdef.iix' /><img src='/no-match.iix' />";
    const output = replaceInlineImageSources(html, [
      {
        id: "1234567890abcdef1234567890abcdef",
        previewUrl: "data:image/png;base64,AAA",
      },
    ]);

    expect(output).toContain('src="data:image/png;base64,AAA"');
    expect(output).toContain('src="/no-match.iix"');
  });

  it("matches a dashless .iix reference against a dashed attachment id (HAR regression)", () => {
    // comment.inlineAttachments[].id is a dashed UUID (entity-service converts
    // the ServiceNow sys_id before sending it), but a .iix src always carries
    // the dashless 32-char form -- a real case had this comparison fail for
    // every inline image, falling through to an unconverted fetch that 404'd.
    const html = '<img src="/9eee0466eb378750fcf5f5dabad0cd32.iix" />';
    const output = replaceInlineImageSources(html, [
      {
        id: "9eee0466-eb37-8750-fcf5-f5dabad0cd32",
        downloadUrl: "https://example.test/attachments/9eee0466-eb37-8750-fcf5-f5dabad0cd32/content",
      },
    ]);

    expect(output).toContain(
      'src="https://example.test/attachments/9eee0466-eb37-8750-fcf5-f5dabad0cd32/content"',
    );
  });
});

describe("code-wrapper helpers", () => {
  it("detects and strips only a single [code] wrapper", () => {
    const content = "[code]Hello[/code]";
    expect(hasSingleCodeWrapper(content)).toBe(true);
    expect(stripCodeWrapper(content)).toBe("Hello");
  });

  it("converts escaped or repeated [code] tags to html code blocks", () => {
    const content = "[code]A[/code][code]B[/code]";
    expect(convertCodeTagsToHtml(content)).toContain("<code>A</code>");
    expect(convertCodeTagsToHtml(content)).toContain("<code>B</code>");
  });
});

describe("hasSubmittableEditorContent", () => {
  it("accepts image-only rich text as submit-worthy", () => {
    expect(hasSubmittableEditorContent("<p><img src='x.png' /></p>")).toBe(true);
  });

  it("rejects empty html wrappers", () => {
    expect(hasSubmittableEditorContent("<p>   </p>")).toBe(false);
  });
});

describe("normalizeCaseTypeOptions", () => {
  it("filters announcement and merges query+incident as Case option", () => {
    const normalized = normalizeCaseTypeOptions([
      { id: "1", label: "Announcement" },
      { id: "2", label: "Incident" },
      { id: "3", label: "Query" },
      { id: "4", label: "Task" },
    ]);

    expect(normalized).toEqual([
      { label: "Task", value: "4" },
      { label: "Case", value: "2,3" },
    ]);
  });
});

describe("isNoveraOrBotSender", () => {
  it("still recognizes the literal name 'Novera'", () => {
    expect(isNoveraOrBotSender("Novera", "comment")).toBe(true);
  });

  it("still recognizes an explicit bot type", () => {
    expect(isNoveraOrBotSender("", "bot")).toBe(true);
  });

  // Regression: the real GET /conversations/{id}/messages response sends
  // createdBy: "" for a Novera reply, not the literal name "Novera" -- every
  // real human message in this feed has a non-empty createdBy, so an empty
  // one is itself the bot signal.
  it("treats an empty createdBy as Novera, even with an ordinary 'comment' type", () => {
    expect(isNoveraOrBotSender("", "comment")).toBe(true);
    expect(isNoveraOrBotSender(null, "comment")).toBe(true);
    expect(isNoveraOrBotSender(undefined, undefined)).toBe(true);
  });

  it("does not flag a real person's message", () => {
    expect(isNoveraOrBotSender("Alice", "comment")).toBe(false);
  });
});

describe("compareByCreatedOnThenId", () => {
  it("orders human comment before bot when timestamps tie", () => {
    const rows = [
      { id: "2", createdOn: "2026-02-01 10:00:00", createdBy: "Novera", type: "bot" },
      { id: "1", createdOn: "2026-02-01 10:00:00", createdBy: "Alice", type: "human" },
    ];
    rows.sort(compareByCreatedOnThenId);

    expect(rows.map((r) => r.id)).toEqual(["1", "2"]);
  });

  // Regression: the real conversation transcript stores a Novera reply with
  // createdBy: "" (not the name "Novera") and only whole-second timestamp
  // precision, so a user's question and Novera's answer often share an
  // identical createdOn -- without isNoveraOrBotSender recognizing the empty
  // createdBy, this tie fell through to an effectively arbitrary id compare,
  // and the bot's reply could render above the question that caused it.
  it("orders the user's question before Novera's empty-createdBy reply when timestamps tie", () => {
    const rows = [
      { id: "reply-1", createdOn: "2026-06-24T16:19:34Z", createdBy: "", type: "comment" },
      {
        id: "question-1",
        createdOn: "2026-06-24T16:19:34Z",
        createdBy: "Jane Doe",
        type: "comment",
      },
    ];
    rows.sort(compareByCreatedOnThenId);

    expect(rows.map((r) => r.id)).toEqual(["question-1", "reply-1"]);
  });
});

// Regression: digiops-cs#3424 — a chat's history showed the same question
// and answer twice, with identical timestamps, for the first exchange of a
// new conversation. Fixed at the display layer: collapse a message that
// exactly repeats the one immediately before it (same content, author and
// createdOn), regardless of why the duplicate row exists.
describe("dedupeAdjacentDuplicateMessages", () => {
  it("collapses a back-to-back exact duplicate (same content/author/createdOn)", () => {
    const rows = [
      { id: "1", content: "First question", createdBy: "customer@example.test", createdOn: "2026-10-11 08:19:00" },
      { id: "2", content: "First question", createdBy: "customer@example.test", createdOn: "2026-10-11 08:19:00" },
      { id: "3", content: "First answer", createdBy: "Novera", createdOn: "2026-10-11 08:19:05" },
      { id: "4", content: "First answer", createdBy: "Novera", createdOn: "2026-10-11 08:19:05" },
    ];

    expect(dedupeAdjacentDuplicateMessages(rows).map((m) => m.id)).toEqual([
      "1",
      "3",
    ]);
  });

  it("keeps two messages with identical text sent at genuinely different times", () => {
    const rows = [
      { id: "1", content: "ok", createdBy: "customer@example.test", createdOn: "2026-10-11 08:19:00" },
      { id: "2", content: "ok", createdBy: "customer@example.test", createdOn: "2026-10-11 08:25:00" },
    ];

    expect(dedupeAdjacentDuplicateMessages(rows).map((m) => m.id)).toEqual([
      "1",
      "2",
    ]);
  });

  it("keeps two different authors' messages even if content and timestamp coincide", () => {
    const rows = [
      { id: "1", content: "test", createdBy: "customer@example.test", createdOn: "2026-10-11 08:19:00" },
      { id: "2", content: "test", createdBy: "Novera", createdOn: "2026-10-11 08:19:00" },
    ];

    expect(dedupeAdjacentDuplicateMessages(rows).map((m) => m.id)).toEqual([
      "1",
      "2",
    ]);
  });

  it("does not collapse a non-adjacent repeat (e.g. the same question asked again later)", () => {
    const rows = [
      { id: "1", content: "test", createdBy: "customer@example.test", createdOn: "2026-10-11 08:19:00" },
      { id: "2", content: "Hi there!", createdBy: "Novera", createdOn: "2026-10-11 08:19:05" },
      { id: "3", content: "test", createdBy: "customer@example.test", createdOn: "2026-10-11 08:20:00" },
    ];

    expect(dedupeAdjacentDuplicateMessages(rows).map((m) => m.id)).toEqual([
      "1",
      "2",
      "3",
    ]);
  });

  it("returns an empty array unchanged", () => {
    expect(dedupeAdjacentDuplicateMessages([])).toEqual([]);
  });

  // Regression for a CodeRabbit finding on an earlier version of this
  // function (verified before acting on it): since createdOn here is only
  // whole-second resolution, an exact-match check with no scope limit could
  // have silently dropped a real, independent repeat anywhere in a long
  // conversation if it coincidentally landed in the same second as an
  // earlier turn. Scoping collapsing to the opening exchange closes that —
  // this asserts a later, genuinely-adjacent exact match past the opening
  // exchange survives untouched.
  it("keeps an exact adjacent match past the opening exchange (not eligible for collapsing there)", () => {
    const rows = [
      { id: "1", content: "question one", createdBy: "customer@example.test", createdOn: "2026-10-11 08:19:00" },
      { id: "2", content: "answer one", createdBy: "Novera", createdOn: "2026-10-11 08:19:05" },
      { id: "3", content: "question two", createdBy: "customer@example.test", createdOn: "2026-10-11 08:25:00" },
      { id: "4", content: "question two", createdBy: "customer@example.test", createdOn: "2026-10-11 08:25:00" },
    ];

    expect(dedupeAdjacentDuplicateMessages(rows).map((m) => m.id)).toEqual([
      "1",
      "2",
      "3",
      "4",
    ]);
  });
});

describe("linkifyBareUrls", () => {
  it("linkifies plain urls but skips href values", () => {
    const html = '<a href="https://already.linked">ok</a> visit https://wso2.com/docs';
    const output = linkifyBareUrls(html);

    expect(output).toContain('<a href="https://already.linked">ok</a>');
    expect(output).toContain(
      '<a href="https://wso2.com/docs" target="_blank" rel="noopener noreferrer"',
    );
  });

  it("never rewrites a url inside a tag attribute (img src)", () => {
    const html =
      '<p><img src="https://wso2.service-now.com/d29f3f669773c350182338f11153af3f.iix"></p>';

    expect(linkifyBareUrls(html)).toBe(html);
  });

  it("does not wrap text that is already inside an anchor", () => {
    const html = '<a href="https://wso2.com">https://wso2.com</a> and https://wso2.com/x';
    const output = linkifyBareUrls(html);

    expect(output.startsWith('<a href="https://wso2.com">https://wso2.com</a> and ')).toBe(true);
    expect(output.match(/<a /g)).toHaveLength(2);
  });
});

// Regression tests: a Date with fewer than 4 digits in its year (reachable
// live from a partially-typed MUI DatePicker year section) used to
// serialize into a malformed, non-zero-padded RFC3339 string (e.g.
// "2-01-10T00:00:00Z") that entity-service's filter parser rejected with a
// 400. Month/day were already zero-padded; only the year was missed.
describe("toUtcStartOfDay", () => {
  it("zero-pads a short year to 4 digits", () => {
    // new Date(2, 0, 10) would NOT give year 2 -- the Date constructor
    // special-cases a 0-99 year argument as 1900+year. setFullYear has no
    // such special-casing, so it's the only way to construct a genuinely
    // short year for this test.
    const date = new Date(2026, 0, 10);
    date.setFullYear(2);
    expect(toUtcStartOfDay(date)).toBe("0002-01-10T00:00:00Z");
  });

  it("formats a normal 4-digit year unchanged", () => {
    const date = new Date(2026, 0, 10);
    expect(toUtcStartOfDay(date)).toBe("2026-01-10T00:00:00Z");
  });
});

// Regression test: a picker that doesn't know the real per-priority lead
// time must never be MORE permissive than the strictest real rule. Reported
// live: entity-service's severityBasedAllocationTime is currently always
// empty (no per-severity SLA-allocation-time table exists yet), so
// allocationMinutes is undefined for every case -- including a
// Catastrophic/P0 case, which ServiceNow's own CallRequestUtils requires at
// least 15 minutes' lead time for. The old "now + 1 minute" fallback let the
// picker accept a time the backend then rejected.
describe("computeMinScheduleDatetimeLocalForTimeZone", () => {
  const UTC = "UTC";

  it("falls back to the backend's own safe default (300 minutes) when allocationMinutes is undefined", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-11T09:00:00.000Z"));
    try {
      const result = computeMinScheduleDatetimeLocalForTimeZone(
        undefined,
        UTC,
      );
      // 09:00 + 300 minutes = 14:00, rounded up to the next 5-minute mark
      // (already on one).
      expect(result).toBe("2026-10-11T14:00");
    } finally {
      vi.useRealTimers();
    }
  });

  it("falls back to the same safe default for null/NaN/negative allocationMinutes", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-11T09:00:00.000Z"));
    try {
      const expected = "2026-10-11T14:00";
      expect(computeMinScheduleDatetimeLocalForTimeZone(null, UTC)).toBe(
        expected,
      );
      expect(computeMinScheduleDatetimeLocalForTimeZone(NaN, UTC)).toBe(
        expected,
      );
      expect(computeMinScheduleDatetimeLocalForTimeZone(-5, UTC)).toBe(
        expected,
      );
    } finally {
      vi.useRealTimers();
    }
  });

  it("uses the real allocation when one is actually known", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-11T09:00:00.000Z"));
    try {
      // ServiceNow's own Catastrophic/P0 offset.
      const result = computeMinScheduleDatetimeLocalForTimeZone(15, UTC);
      expect(result).toBe("2026-10-11T09:15");
    } finally {
      vi.useRealTimers();
    }
  });

  it("never returns a time earlier than now + 1 minute, even with a real allocation", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-11T09:00:00.000Z"));
    try {
      const result = computeMinScheduleDatetimeLocalForTimeZone(0, UTC);
      expect(result >= "2026-10-11T09:01").toBe(true);
    } finally {
      vi.useRealTimers();
    }
  });
});

describe("toUtcEndOfDay", () => {
  it("zero-pads a short year to 4 digits", () => {
    const date = new Date(2026, 0, 10);
    date.setFullYear(2);
    expect(toUtcEndOfDay(date)).toBe("0002-01-11T00:00:00Z");
  });

  it("formats a normal 4-digit year unchanged", () => {
    const date = new Date(2026, 0, 10);
    expect(toUtcEndOfDay(date)).toBe("2026-01-11T00:00:00Z");
  });
});

// Content migrated from the legacy data source carries inline images as a bare
// attachment id (`<img src="/<uuid>">`) with no `.iix` suffix.
describe("bare attachment-id srcs (migrated content)", () => {
  const UUID = "0f15cbcc-c36b-8310-af2f-404599013196";
  const HEX = UUID.replace(/-/g, "");

  it.each([
    ["hyphenated uuid with leading slash", `/${UUID}`, HEX],
    ["hyphenated uuid without leading slash", UUID, HEX],
    ["uppercase hyphenated uuid", `/${UUID.toUpperCase()}`, HEX],
    ["hyphenated uuid with .iix", `/${UUID}.iix`, HEX],
    ["32-hex id with leading slash", `/${HEX}`, HEX],
    ["32-hex id without leading slash", HEX, HEX],
    ["surrounding whitespace", `  /${UUID} `, HEX],
  ])("treats %s as an attachment reference", (_name, src, id) => {
    expect(isInlineImageRefSrc(src)).toBe(true);
    expect(extractInlineImageRefId(src)).toBe(id);
  });

  it("keeps existing .iix behaviour unchanged", () => {
    expect(isInlineImageRefSrc(`/${HEX}.iix`)).toBe(true);
    expect(extractInlineImageRefId(`/${HEX}.iix`)).toBe(HEX);
    expect(extractInlineImageRefId(`https://host/${HEX}.iix`)).toBe(HEX);
    expect(isInlineImageRefSrc("/no-match.iix")).toBe(true);
  });

  it.each([
    ["query string", `/${UUID}?x=1`],
    ["extra path segment", `/images/${UUID}`],
    ["absolute url", `https://host/${UUID}`],
    ["protocol-relative", `//${UUID}`],
    ["double leading slash", `//${HEX}`],
    ["other extension", `/${UUID}.png`],
    ["data uri", "data:image/png;base64,AAAA"],
    ["short hex", "/abc123"],
    ["malformed uuid", "/0f15cbcc-c36b-8310-af2f-40459901319"],
  ])("does not treat %s as an attachment reference", (_name, src) => {
    expect(isInlineImageRefSrc(src)).toBe(false);
  });

  it("replaces a bare-uuid src when the attachment id is the 32-hex form", () => {
    const out = replaceInlineImageSources(`<p><img src="/${UUID}"><br></p>`, [
      { id: HEX, previewUrl: "data:image/png;base64,AAA" },
    ]);
    expect(out).toContain('src="data:image/png;base64,AAA"');
  });

  it("leaves a bare-uuid src untouched when no attachment matches", () => {
    const out = replaceInlineImageSources(`<img src="/${UUID}">`, [
      { id: "ffffffffffffffffffffffffffffffff", previewUrl: "data:x" },
    ]);
    expect(out).toContain(`src="/${UUID}"`);
  });
});

describe("deriveFilterLabels", () => {
  it("words the state filter as Status, like every other list page", () => {
    expect(deriveFilterLabels("state")).toEqual({
      label: "Status",
      allLabel: "All Statuses",
    });
  });

  it("matches the label and all-option of the status filter", () => {
    expect(deriveFilterLabels("state")).toEqual(deriveFilterLabels("status"));
  });

  it("keeps capitalising and pluralising other ids", () => {
    expect(deriveFilterLabels("severity")).toEqual({
      label: "Severity",
      allLabel: "All Severities",
    });
    expect(deriveFilterLabels("impact")).toEqual({
      label: "Impact",
      allLabel: "All Impacts",
    });
    expect(deriveFilterLabels("caseType")).toEqual({
      label: "Case Type",
      allLabel: "All Case Types",
    });
    expect(deriveFilterLabels("createdBy")).toEqual({
      label: "Created By",
      allLabel: "All Users",
    });
    expect(deriveFilterLabels("status")).toEqual({
      label: "Status",
      allLabel: "All Statuses",
    });
  });
});

describe("collapseHtmlSourceWhitespace", () => {
  it("drops the newlines between laid-out block elements, \\r\\n included", () => {
    expect(collapseHtmlSourceWhitespace("<p>One</p>\r\n<p>Two</p>\r\n<p>Three</p>")).toBe(
      "<p>One</p><p>Two</p><p>Three</p>",
    );
  });

  it("joins a hand-wrapped paragraph and drops the indentation", () => {
    expect(
      collapseHtmlSourceWhitespace("<p>Wrapped by hand\r\n   at a fixed width.</p>\r\n<p>Next.</p>"),
    ).toBe("<p>Wrapped by hand at a fixed width.</p><p>Next.</p>");
  });

  it("joins hard-wrapped list items and drops the indentation", () => {
    const source =
      "<ul>\n  <li>First point\n    continues here.<br>\n    Second line.\n  </li>\n</ul>";
    expect(collapseHtmlSourceWhitespace(source)).toBe(
      "<ul><li>First point continues here.<br>Second line.</li></ul>",
    );
  });

  it("keeps a single space between inline elements that were on separate lines", () => {
    expect(collapseHtmlSourceWhitespace("<p>\n  Use <b>one</b>\n  <i>two</i> now\n</p>")).toBe(
      "<p>Use <b>one</b> <i>two</i> now</p>",
    );
  });

  it("leaves a <pre> block and a <code> snippet exactly as written", () => {
    const source =
      "<p>Run:</p>\n<pre>line one\n  line two</pre>\n<p>Then <code>a\nb</code> done</p>";
    expect(collapseHtmlSourceWhitespace(source)).toBe(
      "<p>Run:</p><pre>line one\n  line two</pre><p>Then <code>a\nb</code> done</p>",
    );
  });

  it("keeps a space either side of inline code that sat on its own lines", () => {
    expect(collapseHtmlSourceWhitespace("<p>\n  Set <code>x=1</code>\n  then restart\n</p>")).toBe(
      "<p>Set <code>x=1</code> then restart</p>",
    );
  });

  it("leaves plain text and editor output alone", () => {
    const text = "Line one\nLine two\n\nLine four";
    expect(collapseHtmlSourceWhitespace(text)).toBe(text);
    const editor = "<p>Two  spaces</p><p>Next<br>line</p>";
    expect(collapseHtmlSourceWhitespace(editor)).toBe(editor);
  });

  it("keeps the line breaks of a note that only mixes in a stray <br>", () => {
    const note = "Line one\nLine two<br>Line three";
    expect(collapseHtmlSourceWhitespace(note)).toBe(note);
  });

  it("trims a newline at the very start and end of the body", () => {
    expect(collapseHtmlSourceWhitespace("\r\n<p>One</p>\r\n")).toBe("<p>One</p>");
  });
});

describe("plainTextCommentToHtml", () => {
  it("converts a plain-text comment's real line breaks into <br>", () => {
    expect(
      plainTextCommentToHtml(
        "Hello,\n\nPlease let us know whether you need further support for this case. Please note that if you are unable to respond, the case will be closed off after one week.\n\nThanks,\nWSO2 Team",
      ),
    ).toBe(
      "Hello,<br><br>Please let us know whether you need further support for this case. Please note that if you are unable to respond, the case will be closed off after one week.<br><br>Thanks,<br>WSO2 Team",
    );
  });

  it("handles \\r\\n the same as \\n", () => {
    expect(plainTextCommentToHtml("Line one\r\nLine two")).toBe("Line one<br>Line two");
  });

  it("inserts a blank line after a greeting and before a sign-off even with only a single Enter", () => {
    expect(
      plainTextCommentToHtml(
        "Hello,\nPlease let us know whether you need further support for this case. Please note that if you are unable to respond, the case will be closed off after one week.\nThanks,\nWSO2 Team",
      ),
    ).toBe(
      "Hello,<br><br>Please let us know whether you need further support for this case. Please note that if you are unable to respond, the case will be closed off after one week.<br><br>Thanks,<br>WSO2 Team",
    );
  });

  it("does not add a second blank line when the author already left one", () => {
    expect(plainTextCommentToHtml("Hello,\n\nBody\n\nThanks,")).toBe("Hello,<br><br>Body<br><br>Thanks,");
  });

  it("does not add spacing around a sentence that merely starts with thanks", () => {
    expect(plainTextCommentToHtml("Line one\nThanks for your patience\nLine three")).toBe(
      "Line one<br>Thanks for your patience<br>Line three",
    );
  });

  it("does not treat a greeting-worded sentence with its own comma as an opening greeting", () => {
    expect(plainTextCommentToHtml("Hi, I checked the configuration,\nand it looks good.")).toBe(
      "Hi, I checked the configuration,<br>and it looks good.",
    );
  });

  it("adds a blank line before a sign-off with an escaped ampersand", () => {
    expect(plainTextCommentToHtml("Body\nThanks & Regards,")).toBe("Body<br><br>Thanks &amp; Regards,");
  });

  it("HTML-escapes a literal <, >, &, quote the author typed as plain text", () => {
    expect(plainTextCommentToHtml("Use < and > and & and \"quotes\" and it's")).toBe(
      "Use &lt; and &gt; and &amp; and &quot;quotes&quot; and it&#39;s",
    );
  });

  it("is a no-op for content that already contains real HTML markup", () => {
    const html = "<p>Already HTML\nwith a newline</p>";
    expect(plainTextCommentToHtml(html)).toBe(html);
  });

  it("is a no-op for content carrying a ServiceNow [code] marker, even with no HTML tags", () => {
    const withCode = "Intro\n[code]raw markup[/code]\nTail";
    expect(plainTextCommentToHtml(withCode)).toBe(withCode);
  });

  it("returns an empty string for empty/non-string input", () => {
    expect(plainTextCommentToHtml("")).toBe("");
    // @ts-expect-error -- exercising the runtime guard for non-string input
    expect(plainTextCommentToHtml(null)).toBe("");
  });
});

describe("collapseCodeBlockWhitespace / collapseCommentSourceWhitespace", () => {
  it("collapses only inside the [code] block and leaves the text around it alone", () => {
    const source =
      "Intro line\nnext line\n[code]<ul>\n  <li>One</li>\n</ul>[/code]\nTail text\nmore";
    expect(collapseCodeBlockWhitespace(source)).toBe(
      "Intro line\nnext line\n[code]<ul><li>One</li></ul>[/code]\nTail text\nmore",
    );
  });

  it("keeps the legacy escaped markers as written", () => {
    expect(collapseCodeBlockWhitespace("[\\code]<p>a</p>\n<p>b</p>[\\/code]")).toBe(
      "[\\code]<p>a</p><p>b</p>[\\/code]",
    );
  });

  it("leaves an inline [code] snippet with no markup, and its newlines, alone", () => {
    const source = "Run [code]line one\nline two[/code] now";
    expect(collapseCodeBlockWhitespace(source)).toBe(source);
  });

  it("also cleans laid-out HTML written outside the [code] blocks", () => {
    const source =
      "<div>\r\n  <p>Intro</p>\r\n</div>\r\n[code]<ul>\r\n  <li>One</li>\r\n</ul>[/code]\r\n<div>\r\n  <p>Outro</p>\r\n</div>";
    expect(collapseCodeBlockWhitespace(source)).toBe(
      "<div><p>Intro</p></div>[code]<ul><li>One</li></ul>[/code]<div><p>Outro</p></div>",
    );
  });

  it("judges a newline beside a block against its neighbours, so inline spacing survives", () => {
    const source =
      "<p>\r\n  See <b>this</b>\r\n[code]<b>that</b>[/code]\r\n  then stop\r\n</p>";
    expect(collapseCodeBlockWhitespace(source)).toBe(
      "<p>See <b>this</b> [code]<b>that</b>[/code] then stop</p>",
    );
  });

  it("does not let markup inside a block turn the plain text around it into laid-out HTML", () => {
    const source = "Line one\nLine two\n[code]<p>a</p>\n<p>b</p>[/code]\nLine three\nLine four";
    expect(collapseCodeBlockWhitespace(source)).toBe(
      "Line one\nLine two\n[code]<p>a</p><p>b</p>[/code]\nLine three\nLine four",
    );
  });

  it("still cleans each block when the text already holds the placeholder characters", () => {
    const source = "\uE000 note\n[code]<p>a</p>\n<p>b</p>[/code]";
    expect(collapseCodeBlockWhitespace(source)).toBe("\uE000 note\n[code]<p>a</p><p>b</p>[/code]");
  });

  it("picks the per-block or whole-body form by whether the body has [code] markers", () => {
    expect(collapseCommentSourceWhitespace("[code]<p>a</p>\n<p>b</p>[/code]\nplain\ntext")).toBe(
      "[code]<p>a</p><p>b</p>[/code]\nplain\ntext",
    );
    expect(collapseCommentSourceWhitespace("<p>a</p>\n<p>b</p>")).toBe("<p>a</p><p>b</p>");
    expect(collapseCommentSourceWhitespace("")).toBe("");
  });
});

describe("withSystemAuthor", () => {
  it("labels an author-less comment as System", () => {
    expect(withSystemAuthor({ createdBy: null }).createdBy).toBe(
      SYSTEM_AUTHOR_LABEL,
    );
    expect(withSystemAuthor({ createdBy: "  " }).createdBy).toBe(
      "System",
    );
    expect(withSystemAuthor<{ createdBy?: string | null }>({}).createdBy).toBe("System");
  });

  it("keeps real authors, full names and bot rows untouched", () => {
    const real = { createdBy: "jane.doe@example.com" };
    expect(withSystemAuthor(real)).toBe(real);
    const named = { createdBy: null, createdByFullName: "Jane Doe" };
    expect(withSystemAuthor(named)).toBe(named);
    const bot = { createdBy: "", type: "bot" };
    expect(withSystemAuthor(bot)).toBe(bot);
  });

  it("does not mutate its input", () => {
    const input = { createdBy: null };
    withSystemAuthor(input);
    expect(input.createdBy).toBeNull();
  });
});

describe("downloadSizeOf", () => {
  it("reads a number or a numeric string of bytes", () => {
    expect(downloadSizeOf(2400000)).toBe(2400000);
    expect(downloadSizeOf("2400000")).toBe(2400000);
  });

  it("is undefined when the size is not known", () => {
    for (const v of [undefined, null, "", "abc", 0, -5, Number.NaN]) {
      expect(downloadSizeOf(v)).toBeUndefined();
    }
  });
});

describe("resolveActionStateKey", () => {
  const caseStates = [
    { id: "3", label: "Closed" },
    { id: "10", label: "In Progress" },
  ];

  it("looks the state up by label when the project lists it", () => {
    expect(resolveActionStateKey("Closed", caseStates)).toBe(3);
  });

  it("sends the Waiting On WSO2 key even though the list no longer holds it", () => {
    expect(resolveActionStateKey("Waiting on WSO2", caseStates)).toBe(1003);
    expect(resolveActionStateKey("Reject Solution", caseStates)).toBe(1003);
    expect(resolveActionStateKey("Waiting on WSO2", undefined)).toBe(1003);
  });

  it("returns undefined for an action with no state", () => {
    expect(resolveActionStateKey("Open Related Case", caseStates)).toBeUndefined();
  });
});

describe("getAvailableCaseActions", () => {
  it("offers only Close on an Awaiting Info case by default", () => {
    expect(getAvailableCaseActions("Awaiting Info")).toEqual(["Closed"]);
  });

  it("adds Waiting on WSO2 on an Awaiting Info case for an internal user", () => {
    expect(getAvailableCaseActions("Awaiting Info", true)).toEqual([
      "Closed",
      "Waiting on WSO2",
    ]);
  });

  it("leaves every other status the same for an internal user", () => {
    for (const status of ["Open", "Work in Progress", "Reopened", "Solution Proposed", "Closed"]) {
      expect(getAvailableCaseActions(status, true)).toEqual(getAvailableCaseActions(status));
    }
  });
});

describe("splitStaffMark", () => {
  it("strips the circled W after a name and reports it", () => {
    expect(splitStaffMark("Jane Doe \u24E6")).toEqual({ name: "Jane Doe", isStaff: true });
    expect(splitStaffMark("Jane Doe\u24E6")).toEqual({ name: "Jane Doe", isStaff: true });
    expect(splitStaffMark("Jane Doe \u24CC")).toEqual({ name: "Jane Doe", isStaff: true });
  });

  it("leaves other names alone", () => {
    expect(splitStaffMark("Jane Doe")).toEqual({ name: "Jane Doe", isStaff: false });
    expect(splitStaffMark("Will Smith")).toEqual({ name: "Will Smith", isStaff: false });
    expect(splitStaffMark(null)).toEqual({ name: "", isStaff: false });
  });

  it("keeps the mark out of initials", () => {
    expect(getInitials("Jane Doe \u24E6")).toBe("JD");
  });
});
