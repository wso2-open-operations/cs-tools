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
import type { jsPDF } from "jspdf";
import {
  condenseBlankLines,
  htmlToPdfPlainText,
  toPdfSafeText,
  writeDivider,
  writeHeading,
  writeWrapped,
  type PdfCursor,
} from "./pdfReportKit";

type RecordedCall =
  | { type: "text"; y: number }
  | { type: "line"; y: number }
  | { type: "setDrawColor"; args: unknown[] }
  | { type: "setLineWidth"; width: number };

/** Minimal fake jsPDF recording each drawing call, for asserting on vertical
 * spacing/styling without needing a real rendering engine. */
function fakeDocRecordingYPositions(): { doc: jsPDF; calls: RecordedCall[] } {
  const calls: RecordedCall[] = [];
  const doc = {
    setFontSize() {
      return this;
    },
    setFont() {
      return this;
    },
    setTextColor() {
      return this;
    },
    setDrawColor(...args: unknown[]) {
      calls.push({ type: "setDrawColor", args });
      return this;
    },
    setLineWidth(width: number) {
      calls.push({ type: "setLineWidth", width });
      return this;
    },
    splitTextToSize(text: string) {
      return [text];
    },
    text(_value: string, _x: number, y: number) {
      calls.push({ type: "text", y });
      return this;
    },
    line(_x1: number, y1: number) {
      calls.push({ type: "line", y: y1 });
      return this;
    },
    addPage() {
      return this;
    },
  };
  return { doc: doc as unknown as jsPDF, calls };
}

function isTextCall(c: RecordedCall): c is Extract<RecordedCall, { type: "text" }> {
  return c.type === "text";
}
function isLineCall(c: RecordedCall): c is Extract<RecordedCall, { type: "line" }> {
  return c.type === "line";
}
function isSetDrawColorCall(c: RecordedCall): c is Extract<RecordedCall, { type: "setDrawColor" }> {
  return c.type === "setDrawColor";
}
function isSetLineWidthCall(c: RecordedCall): c is Extract<RecordedCall, { type: "setLineWidth" }> {
  return c.type === "setLineWidth";
}

describe("toPdfSafeText", () => {
  it("keeps plain ASCII and accented Latin-1 names untouched", () => {
    expect(toPdfSafeText("Kasun Perera")).toBe("Kasun Perera");
    expect(toPdfSafeText("Jürgen Müller")).toBe("Jürgen Müller");
  });

  it("maps smart/typographic punctuation to its plain-ASCII equivalent", () => {
    expect(toPdfSafeText("It’s a “test” — really…")).toBe(
      'It\'s a "test" - really...',
    );
    expect(toPdfSafeText("a b")).toBe("a b");
  });

  it("drops emoji and other characters outside jsPDF's standard-font range instead of rendering as garbage", () => {
    // A flag emoji (two astral "regional indicator" code points) is exactly
    // the kind of source-data character reported live as rendering like
    // garbled, widely-spaced text ("...$æ") when drawn with jsPDF's default
    // WinAnsi-encoded font.
    expect(toPdfSafeText("Kasun Perera 🇱🇰")).toBe("Kasun Perera ");
    // Zero-width joiners/spaces interspersed between letters.
    expect(toPdfSafeText("S​a​m")).toBe("Sam");
  });

  it("preserves newlines and tabs", () => {
    expect(toPdfSafeText("line one\nline two\tindented")).toBe("line one\nline two\tindented");
  });

  it("maps a unicode arrow to plain ASCII instead of silently dropping it", () => {
    // Reported live: a state-change description ("Open → Work In Progress")
    // rendered with the arrow simply missing ("Open  Work In Progress") —
    // the arrow was outside jsPDF's standard-font range and, before this
    // mapping existed, fell through the 0x20-0xFF filter with nothing to
    // replace it.
    expect(toPdfSafeText("Open → Work In Progress")).toBe("Open -> Work In Progress");
  });
});

describe("writeHeading", () => {
  it("leaves enough clearance after a level-2 heading's underline that the next line of body text doesn't overlap it", () => {
    // Reported live: "Description" (and every other level-2-headed section)
    // rendered with a stray horizontal line struck through the first line of
    // body text underneath it — the underline sat too close to the next
    // line's own baseline, so a normal 10pt line's ascenders reached back up
    // past it.
    const { doc, calls } = fakeDocRecordingYPositions();
    const cur: PdfCursor = { doc, y: 20 };
    writeHeading(cur, "Description", 2);
    writeWrapped(cur, "Testing purpose", { size: 10 });

    const lineY = calls.find(isLineCall)?.y;
    const bodyTextY = calls.filter(isTextCall).at(-1)?.y;
    expect(lineY).toBeDefined();
    expect(bodyTextY).toBeDefined();
    // A 10pt font's ascent is roughly 2.5mm above its own baseline — the gap
    // between the underline and the next baseline must clear that.
    expect(bodyTextY! - lineY!).toBeGreaterThanOrEqual(3);
  });

  it("draws no underline for level-1/3 headings", () => {
    const { doc, calls } = fakeDocRecordingYPositions();
    const cur: PdfCursor = { doc, y: 20 };
    writeHeading(cur, "Case Report", 1);
    expect(calls.some(isLineCall)).toBe(false);
  });
});

describe("writeDivider", () => {
  it("draws the rule in the more prominent divider color/width, not the subtle table-grid one", () => {
    // Reported live: the divider between activity-list entries was too
    // faint to notice — this asserts it's drawn with its own distinct
    // (heavier) styling rather than reusing the table's subtle grid color.
    const { doc, calls } = fakeDocRecordingYPositions();
    const cur: PdfCursor = { doc, y: 20 };
    writeDivider(cur);

    const drawColorCall = calls.find(isSetDrawColorCall);
    expect(drawColorCall?.args).toEqual([165, 171, 181]);
    const lineWidthCalls = calls.filter(isSetLineWidthCall);
    // Set to the heavier width for the rule itself, then restored afterward.
    expect(lineWidthCalls.some((c) => c.width === 0.4)).toBe(true);
  });

  it("leaves a real margin after the rule before the next content", () => {
    const { doc, calls } = fakeDocRecordingYPositions();
    const cur: PdfCursor = { doc, y: 20 };
    const yBefore = cur.y;
    writeDivider(cur);
    const lineY = calls.find(isLineCall)?.y;
    expect(lineY).toBeDefined();
    expect(lineY!).toBeGreaterThan(yBefore);
    // The cursor itself must end up meaningfully below the drawn rule, not
    // immediately flush against it.
    expect(cur.y - lineY!).toBeGreaterThanOrEqual(4);
  });
});

describe("htmlToPdfPlainText", () => {
  it("puts each paragraph on its own line, instead of running them together", () => {
    // Reported live: real multi-paragraph rich-text content rendered in the
    // PDF as one run-on wall of text ("First paragraph.Second paragraph.") —
    // identical content rendered correctly, with paragraph breaks, on screen.
    const html = "<p>First paragraph.</p><p>Second paragraph.</p><p>Third paragraph.</p>";
    const text = htmlToPdfPlainText(html);
    expect(text).toBe("First paragraph.\nSecond paragraph.\nThird paragraph.\n");
  });

  it("renders a bulleted list as one '• item' line per <li>", () => {
    const html = "<ul><li>First point</li><li>Second point</li></ul>";
    const text = htmlToPdfPlainText(html);
    expect(text).toContain("• First point");
    expect(text).toContain("• Second point");
    // Each bullet is its own line, not concatenated onto the previous one.
    expect(text).not.toContain("First point• Second point");
  });

  it("does not break the line for inline formatting (bold/italic/links)", () => {
    const html = "<p><strong>Label:</strong> Some value</p>";
    expect(htmlToPdfPlainText(html)).toBe("Label: Some value\n");
  });

  it("treats <br> as a single line break", () => {
    expect(htmlToPdfPlainText("Line one<br>Line two")).toBe("Line one\nLine two");
  });

  it("returns an empty string for empty/falsy input", () => {
    expect(htmlToPdfPlainText("")).toBe("");
  });
});

describe("condenseBlankLines", () => {
  it("collapses 3+ consecutive blank lines down to a single blank line", () => {
    expect(condenseBlankLines("first\n\n\n\n\nsecond")).toBe("first\n\nsecond");
  });

  it("trims leading and trailing whitespace", () => {
    expect(condenseBlankLines("\n\n  hello  \n\n")).toBe("hello");
  });

  it("leaves normal single/double line breaks alone", () => {
    expect(condenseBlankLines("a\nb\n\nc")).toBe("a\nb\n\nc");
  });
});
