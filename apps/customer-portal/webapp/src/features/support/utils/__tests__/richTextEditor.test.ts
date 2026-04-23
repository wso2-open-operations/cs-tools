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

import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { render } from "@testing-library/react";
import { createTheme } from "@wso2/oxygen-ui";
import {
  deriveAltFromFilename,
  escapeHtml,
  sanitizeUrl,
  getFileIcon,
  scrollElement,
  INSERT_IMAGE_COMMAND,
} from "@features/support/utils/richTextEditor";

describe("richTextEditor utils", () => {
  describe("deriveAltFromFilename", () => {
    it("derives alt from absolute URL path", () => {
      expect(
        deriveAltFromFilename("https://example.com/images/photo.jpg"),
      ).toBe("photo");
    });

    it("derives alt from relative path", () => {
      expect(deriveAltFromFilename("/assets/image.png")).toBe("image");
    });

    it("derives alt from plain filename", () => {
      expect(deriveAltFromFilename("image.png")).toBe("image");
    });

    it("returns basename when path has no extension", () => {
      expect(deriveAltFromFilename("https://example.com/photo")).toBe("photo");
    });

    it("strips query and hash when URL parsing fails", () => {
      expect(deriveAltFromFilename("file.png?q=1#hash")).toBe("file");
    });
  });

  describe("escapeHtml", () => {
    it("escapes ampersand", () => {
      expect(escapeHtml("a & b")).toBe("a &amp; b");
    });

    it("escapes less-than and greater-than", () => {
      expect(escapeHtml("<script>")).toBe("&lt;script&gt;");
    });

    it("escapes double and single quotes", () => {
      expect(escapeHtml('"test"')).toBe("&quot;test&quot;");
      expect(escapeHtml("'test'")).toBe("&#039;test&#039;");
    });

    it("handles multiple entities", () => {
      expect(escapeHtml('<a href="x">')).toBe("&lt;a href=&quot;x&quot;&gt;");
    });
  });

  describe("sanitizeUrl", () => {
    it("returns url for http/https", () => {
      expect(sanitizeUrl("https://example.com")).toBe("https://example.com");
      expect(sanitizeUrl("http://example.com")).toBe("http://example.com");
    });

    it("returns url for mailto and tel", () => {
      expect(sanitizeUrl("mailto:test@example.com")).toBe(
        "mailto:test@example.com",
      );
      expect(sanitizeUrl("tel:+1234567890")).toBe("tel:+1234567890");
    });

    it("returns url for relative and hash", () => {
      expect(sanitizeUrl("/path")).toBe("/path");
      expect(sanitizeUrl("#anchor")).toBe("#anchor");
    });

    it("returns empty string for javascript and data", () => {
      expect(sanitizeUrl("javascript:alert(1)")).toBe("");
      expect(sanitizeUrl("data:text/html,<script>")).toBe("");
    });

    it("returns empty string for protocol-relative URLs", () => {
      expect(sanitizeUrl("//evil.com")).toBe("");
      expect(sanitizeUrl("//example.com/path")).toBe("");
    });

    it("returns decoded and trimmed value for valid URLs", () => {
      expect(sanitizeUrl("  https://example.com  ")).toBe(
        "https://example.com",
      );
      expect(sanitizeUrl("  /path  ")).toBe("/path");
    });
  });

  describe("getFileIcon", () => {
    const theme = createTheme();

    it("returns FileImage for image extensions", () => {
      const file = new File([], "photo.png", { type: "image/png" });
      const { container } = render(getFileIcon(file, theme));
      const svg = container.querySelector("svg");
      expect(svg).toBeInTheDocument();
    });

    it("returns FileImage for image mime type", () => {
      const file = new File([], "unknown", { type: "image/gif" });
      const { container } = render(getFileIcon(file, theme));
      expect(container.querySelector("svg")).toBeInTheDocument();
    });

    it("returns FileText for pdf", () => {
      const file = new File([], "doc.pdf", { type: "application/pdf" });
      const { container } = render(getFileIcon(file, theme));
      expect(container.querySelector("svg")).toBeInTheDocument();
    });

    it("returns FileArchive for zip", () => {
      const file = new File([], "archive.zip", { type: "application/zip" });
      const { container } = render(getFileIcon(file, theme));
      expect(container.querySelector("svg")).toBeInTheDocument();
    });

    it("returns FileCode for code extensions", () => {
      const file = new File([], "script.ts", { type: "text/plain" });
      const { container } = render(getFileIcon(file, theme));
      expect(container.querySelector("svg")).toBeInTheDocument();
    });

    it("returns FileIcon for unknown type", () => {
      const file = new File([], "unknown.xyz", {
        type: "application/octet-stream",
      });
      const { container } = render(getFileIcon(file, theme));
      expect(container.querySelector("svg")).toBeInTheDocument();
    });
  });

  describe("scrollElement", () => {
    let mockScrollBy: ReturnType<typeof vi.fn>;

    beforeEach(() => {
      mockScrollBy = vi.fn();
      vi.spyOn(document, "getElementById").mockReturnValue({
        scrollBy: mockScrollBy,
      } as unknown as HTMLElement);
    });

    afterEach(() => {
      vi.restoreAllMocks();
    });

    it("scrolls right when direction is right", () => {
      scrollElement("toolbar", "right", 200);
      expect(mockScrollBy).toHaveBeenCalledWith({
        left: 200,
        behavior: "smooth",
      });
    });

    it("scrolls left when direction is left", () => {
      scrollElement("toolbar", "left", 200);
      expect(mockScrollBy).toHaveBeenCalledWith({
        left: -200,
        behavior: "smooth",
      });
    });

    it("does nothing when element is not found", () => {
      vi.mocked(document.getElementById).mockReturnValue(null);
      scrollElement("missing", "right");
      expect(mockScrollBy).not.toHaveBeenCalled();
    });
  });

  describe("INSERT_IMAGE_COMMAND", () => {
    it("is defined as a Lexical command", () => {
      expect(INSERT_IMAGE_COMMAND).toBeDefined();
      expect(INSERT_IMAGE_COMMAND).toBeTruthy();
    });
  });
});
