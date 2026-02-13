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
import { ChatAction, ChatStatus } from "@constants/supportConstants";
import {
  CircleAlert,
  CircleCheck,
  CircleQuestionMark,
  Clock,
  MessageCircle,
} from "@wso2/oxygen-ui-icons-react";
import {
  getChatStatusAction,
  getChatStatusColor,
  getChatActionColor,
  formatRelativeTime,
  formatValue,
  formatFileSize,
  formatSlaResponseTime,
  deriveFilterLabels,
  getStatusIcon,
  getAttachmentFileCategory,
  resolveColorFromTheme,
  stripCodeWrapper,
  replaceInlineImageSources,
  formatCommentDate,
  getInitials,
  hasDisplayableContent,
} from "@utils/support";
import type { CaseComment } from "@models/responses";
import { createTheme } from "@wso2/oxygen-ui";

describe("support utils", () => {
  describe("getChatStatusAction", () => {
    it("should return resume for Still Open", () => {
      expect(getChatStatusAction(ChatStatus.STILL_OPEN)).toBe(
        ChatAction.RESUME,
      );
      expect(getChatStatusAction(ChatStatus.STILL_OPEN.toLowerCase())).toBe(
        ChatAction.RESUME,
      );
    });

    it("should return view for Resolved and Abandoned", () => {
      expect(getChatStatusAction(ChatStatus.RESOLVED)).toBe(ChatAction.VIEW);
      expect(getChatStatusAction(ChatStatus.ABANDONED)).toBe(ChatAction.VIEW);
      expect(getChatStatusAction("")).toBe(ChatAction.VIEW);
    });
  });

  describe("getChatActionColor", () => {
    it("should return info for RESUME", () => {
      expect(getChatActionColor(ChatAction.RESUME)).toBe("info");
    });

    it("should return primary for VIEW", () => {
      expect(getChatActionColor(ChatAction.VIEW)).toBe("primary");
    });
  });

  describe("getChatStatusColor", () => {
    it("should return success.main for Resolved", () => {
      expect(getChatStatusColor(ChatStatus.RESOLVED)).toBe("success.main");
    });

    it("should return info.main for Still Open", () => {
      expect(getChatStatusColor(ChatStatus.STILL_OPEN)).toBe("info.main");
    });

    it("should return error.main for Abandoned", () => {
      expect(getChatStatusColor(ChatStatus.ABANDONED)).toBe("error.main");
    });

    it("should return secondary.main for others", () => {
      expect(getChatStatusColor("")).toBe("secondary.main");
    });
  });

  describe("resolveColorFromTheme", () => {
    const theme = createTheme();

    it("should resolve primary.main", () => {
      expect(resolveColorFromTheme("primary.main", theme)).toBe(
        theme.palette.primary.main,
      );
    });

    it("should return the path itself if not found in theme", () => {
      expect(resolveColorFromTheme("non.existent.color", theme)).toBe(
        "non.existent.color",
      );
    });
  });

  describe("formatSlaResponseTime", () => {
    it("should return '--' for null and undefined", () => {
      expect(formatSlaResponseTime(null)).toBe("--");
      expect(formatSlaResponseTime(undefined)).toBe("--");
    });

    it("should format milliseconds to hours with singular/plural", () => {
      expect(formatSlaResponseTime(3600000)).toBe("1 hour");
      expect(formatSlaResponseTime(7200000)).toBe("2 hours");
    });

    it("should format milliseconds to minutes with singular/plural", () => {
      expect(formatSlaResponseTime(60000)).toBe("1 minute");
      expect(formatSlaResponseTime(120000)).toBe("2 minutes");
    });

    it("should use Math.floor so values just under threshold stay in current unit", () => {
      expect(formatSlaResponseTime(3599999)).toBe("59 minutes");
      expect(formatSlaResponseTime(3599999)).not.toBe("1 hour");
    });

    it("should format milliseconds to days when >= 24 hours", () => {
      // 129671000 ms ≈ 36 hours → Math.floor(36/24) = 1 day
      expect(formatSlaResponseTime("129671000")).toBe("1 day");
      expect(formatSlaResponseTime(172800000)).toBe("2 days");
    });

    it("should format milliseconds to days with singular/plural", () => {
      expect(formatSlaResponseTime(86400000)).toBe("1 day");
      expect(formatSlaResponseTime(172800000)).toBe("2 days");
    });
  });

  describe("formatValue", () => {
    it("should return '--' for null and undefined", () => {
      expect(formatValue(null)).toBe("--");
      expect(formatValue(undefined)).toBe("--");
    });

    it("should return '--' for empty string", () => {
      expect(formatValue("")).toBe("--");
    });

    it("should return string for non-empty string", () => {
      expect(formatValue("Choreo")).toBe("Choreo");
    });

    it("should return string for number", () => {
      expect(formatValue(42)).toBe("42");
    });
  });

  describe("formatRelativeTime", () => {
    it("should return '--' for undefined date", () => {
      expect(formatRelativeTime(undefined)).toBe("--");
    });

    it("should return 'just now' for very recent dates", () => {
      const now = new Date();
      expect(formatRelativeTime(now)).toBe("just now");
    });

    it("should format minutes ago", () => {
      const fiveMinsAgo = new Date(Date.now() - 5 * 60 * 1000);
      expect(formatRelativeTime(fiveMinsAgo)).toBe("5 minutes ago");
    });

    it("should format hours ago", () => {
      const twoHoursAgo = new Date(Date.now() - 2 * 60 * 60 * 1000);
      expect(formatRelativeTime(twoHoursAgo)).toBe("2 hours ago");
    });

    it("should format days ago", () => {
      const threeDaysAgo = new Date(Date.now() - 3 * 24 * 60 * 60 * 1000);
      expect(formatRelativeTime(threeDaysAgo)).toBe("3 days ago");
    });
  });

  describe("deriveFilterLabels", () => {
    it("should derive labels for status", () => {
      expect(deriveFilterLabels("status")).toEqual({
        label: "Status",
        allLabel: "All Statuses",
      });
    });

    it("should derive labels for category", () => {
      expect(deriveFilterLabels("category")).toEqual({
        label: "Category",
        allLabel: "All Categories",
      });
    });

    it("should derive labels for severity", () => {
      expect(deriveFilterLabels("severity")).toEqual({
        label: "Severity",
        allLabel: "All Severities",
      });
    });

    it("should derive labels for deployment", () => {
      expect(deriveFilterLabels("deployment")).toEqual({
        label: "Deployment",
        allLabel: "All Deployments",
      });
    });
  });

  describe("getAttachmentFileCategory", () => {
    it("should return image for png, jpg, gif, webp, svg", () => {
      expect(getAttachmentFileCategory("photo.png", "")).toBe("image");
      expect(getAttachmentFileCategory("photo.jpg", "")).toBe("image");
      expect(getAttachmentFileCategory("photo.jpeg", "")).toBe("image");
      expect(getAttachmentFileCategory("photo.gif", "")).toBe("image");
      expect(getAttachmentFileCategory("photo.webp", "")).toBe("image");
      expect(getAttachmentFileCategory("photo.svg", "")).toBe("image");
      expect(getAttachmentFileCategory("x", "image/png")).toBe("image");
    });

    it("should return pdf for pdf files", () => {
      expect(getAttachmentFileCategory("doc.pdf", "")).toBe("pdf");
      expect(getAttachmentFileCategory("x", "application/pdf")).toBe("pdf");
    });

    it("should return archive for zip, rar, 7z, tar, gz", () => {
      expect(getAttachmentFileCategory("data.zip", "")).toBe("archive");
      expect(getAttachmentFileCategory("data.rar", "")).toBe("archive");
      expect(getAttachmentFileCategory("data.7z", "")).toBe("archive");
      expect(getAttachmentFileCategory("data.tar", "")).toBe("archive");
      expect(getAttachmentFileCategory("data.gz", "")).toBe("archive");
      expect(getAttachmentFileCategory("x", "application/zip")).toBe("archive");
    });

    it("should return text for log, txt and text/*", () => {
      expect(getAttachmentFileCategory("app.log", "")).toBe("text");
      expect(getAttachmentFileCategory("readme.txt", "")).toBe("text");
      expect(getAttachmentFileCategory("x", "text/plain")).toBe("text");
    });

    it("should return file for unknown types", () => {
      expect(getAttachmentFileCategory("data.xml", "")).toBe("file");
    });
  });

  describe("formatFileSize", () => {
    it("should format bytes to KB and MB", () => {
      expect(formatFileSize(500)).toBe("500 B");
      expect(formatFileSize(1024)).toBe("1 KB");
      expect(formatFileSize(245760)).toBe("240 KB");
      expect(formatFileSize(1024 * 1024)).toBe("1 MB");
    });

    it("should return -- for null or invalid", () => {
      expect(formatFileSize(null)).toBe("--");
      expect(formatFileSize(undefined)).toBe("--");
    });
  });

  describe("getStatusIcon", () => {
    it("should return CircleAlert for Open", () => {
      expect(getStatusIcon("Open")).toBe(CircleAlert);
      expect(getStatusIcon("open")).toBe(CircleAlert);
    });

    it("should return Clock for In Progress", () => {
      expect(getStatusIcon("In Progress")).toBe(Clock);
    });

    it("should return MessageCircle for Awaiting Reply", () => {
      expect(getStatusIcon("Awaiting Customer Reply")).toBe(MessageCircle);
    });

    it("should return CircleQuestionMark for Waiting", () => {
      expect(getStatusIcon("Waiting for WSO2")).toBe(CircleQuestionMark);
    });

    it("should return CircleCheck for Resolved or Closed", () => {
      expect(getStatusIcon("Resolved")).toBe(CircleCheck);
      expect(getStatusIcon("Closed")).toBe(CircleCheck);
    });
  });

  describe("stripCodeWrapper", () => {
    it("should return empty string for empty or invalid input", () => {
      expect(stripCodeWrapper("")).toBe("");
      expect(stripCodeWrapper(null as unknown as string)).toBe("");
    });

    it("should return content unchanged when no [code] wrapper", () => {
      expect(stripCodeWrapper("plain text")).toBe("plain text");
      expect(stripCodeWrapper("[code]only start")).toBe("[code]only start");
      expect(stripCodeWrapper("only end[/code]")).toBe("only end[/code]");
    });

    it("should strip [code]...[/code] wrapper", () => {
      expect(stripCodeWrapper("[code]x[/code]")).toBe("x");
      expect(stripCodeWrapper("[code]  hello  [/code]")).toBe("hello");
    });
  });

  describe("hasDisplayableContent", () => {
    it("returns true for comment with code wrapper and meaningful content", () => {
      const comment: CaseComment = {
        id: "1",
        content: "[code]<p>Hello world</p>[/code]",
        type: "comments",
        createdOn: "2026-02-12 10:00:00",
        createdBy: "user@example.com",
        isEscalated: false,
      };
      expect(hasDisplayableContent(comment)).toBe(true);
    });

    it("returns true for comment with plain text", () => {
      const comment: CaseComment = {
        id: "2",
        content: "Some text here",
        type: "comments",
        createdOn: "2026-02-12 10:00:00",
        createdBy: "user@example.com",
        isEscalated: false,
      };
      expect(hasDisplayableContent(comment)).toBe(true);
    });

    it("returns false for comment with only code wrapper and Customer comment added", () => {
      const comment: CaseComment = {
        id: "3",
        content: "[code]<p>Customer comment added</p>[/code]",
        type: "comments",
        createdOn: "2026-02-12 10:00:00",
        createdBy: "user@example.com",
        isEscalated: false,
      };
      expect(hasDisplayableContent(comment)).toBe(false);
    });

    it("returns false for empty content", () => {
      const comment: CaseComment = {
        id: "4",
        content: "",
        type: "comments",
        createdOn: "2026-02-12 10:00:00",
        createdBy: "user@example.com",
        isEscalated: false,
      };
      expect(hasDisplayableContent(comment)).toBe(false);
    });

    it("returns false for null content", () => {
      const comment: CaseComment = {
        id: "5",
        content: undefined as unknown as string,
        type: "comments",
        createdOn: "2026-02-12 10:00:00",
        createdBy: "user@example.com",
        isEscalated: false,
      };
      expect(hasDisplayableContent(comment)).toBe(false);
    });
  });

  describe("replaceInlineImageSources", () => {
    it("should return empty string for empty html", () => {
      expect(replaceInlineImageSources("")).toBe("");
    });

    it("should return sanitized html when no attachments", () => {
      const html = '<img src="/abc.iix" alt="x">';
      expect(replaceInlineImageSources(html, null)).toContain("src");
    });

    it("should replace src by sys_id when attachment matches", () => {
      const html = '<img src="/sys123.iix" alt="x">';
      const attachments = [{ sys_id: "sys123", url: "https://example.com/img.png" }];
      const result = replaceInlineImageSources(html, attachments);
      expect(result).toContain("https://example.com/img.png");
    });

    it("should replace src by id when attachment matches", () => {
      const html = '<img src="/att456.iix">';
      const attachments = [{ id: "att456", downloadUrl: "https://cdn.example.com/att.png" }];
      const result = replaceInlineImageSources(html, attachments);
      expect(result).toContain("https://cdn.example.com/att.png");
    });

    it("should handle single-quoted src", () => {
      const html = "<img src='/sys99.iix' alt=''>";
      const attachments = [{ sys_id: "sys99", url: "https://example.com/a.png" }];
      const result = replaceInlineImageSources(html, attachments);
      expect(result).toContain("https://example.com/a.png");
    });
  });

  describe("getInitials", () => {
    it("should derive initials from name", () => {
      expect(getInitials("John Doe")).toBe("JD");
      expect(getInitials("Alice")).toBe("A");
    });

    it("should return -- for null, undefined, empty", () => {
      expect(getInitials(null)).toBe("--");
      expect(getInitials(undefined)).toBe("--");
      expect(getInitials("")).toBe("--");
    });
  });

  describe("formatCommentDate", () => {
    it("should return -- for null and undefined", () => {
      expect(formatCommentDate(null)).toBe("--");
      expect(formatCommentDate(undefined)).toBe("--");
    });

    it("should return -- for invalid date", () => {
      expect(formatCommentDate("not-a-date")).toBe("--");
    });

    it("should format valid date string", () => {
      const result = formatCommentDate("2026-02-13T15:45:00Z");
      expect(result).toMatch(/Feb/);
      expect(result).toMatch(/13/);
      expect(result).toMatch(/2026/);
    });

    it("should normalize ServiceNow timestamp YYYY-MM-DD HH:MM:SS as UTC", () => {
      const result = formatCommentDate("2026-02-13 15:45:00");
      expect(result).toMatch(/Feb/);
      expect(result).toMatch(/13/);
      expect(result).toMatch(/2026/);
    });
  });
});
