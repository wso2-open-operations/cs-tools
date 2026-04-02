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
  CircleX,
  Clock,
  MessageCircle,
  TriangleAlert,
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
  getIncidentAndQueryCaseTypeIds,
  getIncidentAndQueryIds,
  getStatusIcon,
  getAnnouncementCaseTypeId,
  getAvailableCaseActions,
  getAttachmentFileCategory,
  stripHtml,
  resolveColorFromTheme,
  getSupportOverviewChipSx,
  getPlainChipSx,
  mapSeverityToDisplay,
  getSeverityIcon,
  hasSingleCodeWrapper,
  stripCodeWrapper,
  stripAllCodeBlocks,
  trimLeadingBr,
  convertCodeTagsToHtml,
  replaceInlineImageSources,
  extractInlineImageRefId,
  formatCommentDate,
  toDatetimeLocalInputFromApiString,
  dateFromApiCreatedOn,
  compareByCreatedOnThenId,
  parseApiLocalDateTimeMs,
  formatUtcToLocal,
  formatDateOnly,
  getInitials,
  hasDisplayableContent,
  stripCustomerPrefixFromReason,
  formatCallRequestBackendDateTimeShort,
  formatCallRequestPromptScheduledTime,
  callRequestApiPreferredTimeToDatetimeLocal,
  callRequestPreferredTimeFromDatetimeLocal,
  datetimeLocalWallTimeToUtcMs,
  normalizeDatetimeLocalForCompare,
  resolveCallSchedulingTimeZone,
  wallClockToUtcMilliseconds,
  isWithinOpenRelatedCaseWindow,
  toPresentContinuousActionLabel,
  toPresentTenseActionLabel,
} from "@utils/support";
import type { CaseComment } from "@models/responses";
import { createTheme } from "@wso2/oxygen-ui";

describe("support utils", () => {
  describe("getChatStatusAction", () => {
    it("should return resume for Open", () => {
      expect(getChatStatusAction(ChatStatus.OPEN)).toBe(
        ChatAction.RESUME,
      );
      expect(getChatStatusAction(ChatStatus.OPEN.toLowerCase())).toBe(
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

    it("should return info.main for Open", () => {
      expect(getChatStatusColor(ChatStatus.OPEN)).toBe("info.main");
    });

    it("should return error.main for Abandoned", () => {
      expect(getChatStatusColor(ChatStatus.ABANDONED)).toBe("error.main");
    });

    it("should return secondary.main for others", () => {
      expect(getChatStatusColor("")).toBe("secondary.main");
    });
  });

  describe("isWithinOpenRelatedCaseWindow", () => {
    it("returns true when closedOn is null or undefined", () => {
      expect(isWithinOpenRelatedCaseWindow(null)).toBe(true);
      expect(isWithinOpenRelatedCaseWindow(undefined)).toBe(true);
    });

    it("returns true when closedOn is within 60 days", () => {
      const recent = new Date();
      recent.setMonth(recent.getMonth() - 1);
      const str = recent.toISOString().replace("T", " ").slice(0, 19);
      expect(isWithinOpenRelatedCaseWindow(str)).toBe(true);
    });

    it("returns false when closedOn is more than 60 days ago", () => {
      expect(isWithinOpenRelatedCaseWindow("2020-01-01 10:00:00")).toBe(false);
    });
  });

  describe("stripCustomerPrefixFromReason", () => {
    it("strips [Customer] prefix case-insensitively", () => {
      expect(stripCustomerPrefixFromReason("[Customer] Some notes")).toBe(
        "Some notes",
      );
      expect(stripCustomerPrefixFromReason("[CUSTOMER] Call request")).toBe(
        "Call request",
      );
    });

    it("returns trimmed string when no prefix", () => {
      expect(stripCustomerPrefixFromReason("Plain notes")).toBe("Plain notes");
      expect(stripCustomerPrefixFromReason("  no prefix  ")).toBe("no prefix");
    });

    it("returns empty string for empty input", () => {
      expect(stripCustomerPrefixFromReason("")).toBe("");
    });
  });

  describe("formatCallRequestBackendDateTimeShort", () => {
    it("formats literal Z ISO as wall clock (not UTC shift)", () => {
      expect(
        formatCallRequestBackendDateTimeShort("2026-04-01T16:55:00.000Z"),
      ).toBe("Apr 1, 4:55 PM");
    });

    it("formatCallRequestPromptScheduledTime prefers first preferred time over scheduleTime", () => {
      expect(
        formatCallRequestPromptScheduledTime(
          ["2026-04-02T08:45:00.000Z"],
          "2026-04-02T14:15:00.000Z",
        ),
      ).toBe("Apr 2, 8:45 AM");
    });

    it("formats YYYY-MM-DD HH:mm:ss as wall-clock without shifting timezone", () => {
      expect(formatCallRequestBackendDateTimeShort("2026-04-02 05:49:41")).toBe(
        "Apr 2, 5:49 AM",
      );
    });

    it("formats MM/DD/YYYY HH:mm:ss as wall-clock", () => {
      expect(formatCallRequestBackendDateTimeShort("04/02/2026 01:50:00")).toBe(
        "Apr 2, 1:50 AM",
      );
    });

    it("returns -- for empty or invalid input", () => {
      expect(formatCallRequestBackendDateTimeShort("")).toBe("--");
      expect(formatCallRequestBackendDateTimeShort(undefined)).toBe("--");
      expect(formatCallRequestBackendDateTimeShort("not a date")).toBe("--");
    });
  });

  describe("datetimeLocalWallTimeToUtcMs (profile / filter API time zone)", () => {
    it("interprets datetime-local as Asia/Colombo wall time for API ISO", () => {
      const ms = datetimeLocalWallTimeToUtcMs(
        "2026-04-03T06:17",
        "Asia/Colombo",
      );
      expect(ms).not.toBeNull();
      expect(new Date(ms!).toISOString()).toMatch(
        /^2026-04-03T00:47:00\.000Z$/,
      );
    });

    it("maps WSO2/Colombo filter API id to same UTC ms as Asia/Colombo", () => {
      const msIana = datetimeLocalWallTimeToUtcMs(
        "2026-04-03T06:17",
        "Asia/Colombo",
      );
      const msWso2 = datetimeLocalWallTimeToUtcMs(
        "2026-04-03T06:17",
        "WSO2/Colombo",
      );
      expect(msWso2).toBe(msIana);
    });

    it("resolves Colombo wall clock to consistent UTC ms", () => {
      const ms = wallClockToUtcMilliseconds(2026, 4, 3, 6, 17, "Asia/Colombo");
      expect(ms).toBe(Date.parse("2026-04-03T00:47:00.000Z"));
    });
  });

  describe("callRequestApiPreferredTimeToDatetimeLocal", () => {
    it("strips literal Z to datetime-local (same clock as POST)", () => {
      expect(
        callRequestApiPreferredTimeToDatetimeLocal("2026-04-01T16:55:00.000Z"),
      ).toBe("2026-04-01T16:55");
    });

    it("maps space-separated API time to datetime-local", () => {
      expect(
        callRequestApiPreferredTimeToDatetimeLocal("2024-10-29 14:00:00"),
      ).toBe("2024-10-29T14:00");
    });
  });

  describe("callRequestPreferredTimeFromDatetimeLocal", () => {
    it("echoes modal wall clock as Z-suffixed ISO (not true UTC offset)", () => {
      expect(callRequestPreferredTimeFromDatetimeLocal("2026-04-01T16:55")).toBe(
        "2026-04-01T16:55:00.000Z",
      );
    });

    it("returns empty for invalid input", () => {
      expect(callRequestPreferredTimeFromDatetimeLocal("")).toBe("");
      expect(callRequestPreferredTimeFromDatetimeLocal("bad")).toBe("");
    });
  });

  describe("normalizeDatetimeLocalForCompare", () => {
    it("orders lexicographically for same-width strings", () => {
      expect(normalizeDatetimeLocalForCompare("2026-04-01T08:00")).toBe(
        "2026-04-01T08:00",
      );
      expect(
        normalizeDatetimeLocalForCompare("2026-04-01T08:00")! <
          normalizeDatetimeLocalForCompare("2026-04-01T16:55")!,
      ).toBe(true);
    });

    it("returns null for invalid input", () => {
      expect(normalizeDatetimeLocalForCompare("")).toBeNull();
    });
  });

  describe("resolveCallSchedulingTimeZone", () => {
    it("maps WSO2/Colombo to Asia/Colombo", () => {
      expect(resolveCallSchedulingTimeZone("WSO2/Colombo")).toBe(
        "Asia/Colombo",
      );
    });

    it("falls back to a valid IANA id for empty input", () => {
      const tz = resolveCallSchedulingTimeZone("");
      expect(typeof tz).toBe("string");
      expect(tz.length).toBeGreaterThan(0);
    });

    it("falls back for invalid zone ids", () => {
      const tz = resolveCallSchedulingTimeZone("Not/A/Real/Zone");
      expect(typeof tz).toBe("string");
      expect(() =>
        new Intl.DateTimeFormat("en-US", { timeZone: tz }).format(new Date()),
      ).not.toThrow();
    });

    it("passes through Asia/Kolkata", () => {
      expect(resolveCallSchedulingTimeZone("Asia/Kolkata")).toBe("Asia/Kolkata");
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

  describe("getSupportOverviewChipSx", () => {
    const theme = createTheme();

    it("should return sx with bgcolor, color, height, fontSize", () => {
      const sx = getSupportOverviewChipSx("success.main", theme);
      expect(sx).toHaveProperty("bgcolor");
      expect(sx).toHaveProperty("color");
      expect(sx).toHaveProperty("height", 20);
      expect(sx).toHaveProperty("fontSize", "0.75rem");
    });

    it("should use resolved color from theme", () => {
      const sx = getSupportOverviewChipSx("primary.main", theme);
      expect(sx.color).toBe(theme.palette.primary.main);
    });
  });

  describe("getPlainChipSx", () => {
    it("should return sx with height and fontSize only", () => {
      const sx = getPlainChipSx();
      expect(sx).toHaveProperty("height", 20);
      expect(sx).toHaveProperty("fontSize", "0.75rem");
      expect(sx).not.toHaveProperty("bgcolor");
      expect(sx).not.toHaveProperty("color");
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

    it("should return label for { id, label } object (assigned engineer)", () => {
      expect(formatValue({ id: "eng-1", label: "Agzaiyenth Ganaraj" })).toBe(
        "Agzaiyenth Ganaraj",
      );
      expect(formatValue({ id: "12", label: "" })).toBe("--");
    });

    it("should return name for { id, name } object (case details API)", () => {
      expect(formatValue({ id: "eng-1", name: "John Doe" })).toBe("John Doe");
      expect(formatValue({ id: "12", name: "" })).toBe("--");
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

    it("should derive labels for caseType", () => {
      expect(deriveFilterLabels("caseType")).toEqual({
        label: "Case Type",
        allLabel: "All Case Types",
      });
    });
  });

  describe("getIncidentAndQueryCaseTypeIds", () => {
    it("should return Incident and Query IDs from caseTypes", () => {
      const caseTypes = [
        { id: "id-incident", label: "Incident" },
        { id: "id-query", label: "Query" },
        { id: "id-other", label: "Service Request" },
      ];
      expect(getIncidentAndQueryCaseTypeIds(caseTypes)).toEqual([
        "id-incident",
        "id-query",
      ]);
    });

    it("should return empty array when caseTypes is empty or undefined", () => {
      expect(getIncidentAndQueryCaseTypeIds([])).toEqual([]);
      expect(getIncidentAndQueryCaseTypeIds(undefined)).toEqual([]);
    });

    it("should match case-insensitively", () => {
      const caseTypes = [
        { id: "id1", label: "INCIDENT" },
        { id: "id2", label: "query" },
      ];
      expect(getIncidentAndQueryCaseTypeIds(caseTypes)).toEqual(["id1", "id2"]);
    });

    it("should accept Icident typo variant", () => {
      const caseTypes = [
        { id: "id-icident", label: "Icident" },
        { id: "id-query", label: "Query" },
      ];
      expect(getIncidentAndQueryCaseTypeIds(caseTypes)).toEqual([
        "id-icident",
        "id-query",
      ]);
    });
  });

  describe("getIncidentAndQueryIds", () => {
    it("should return incidentId and queryId separately", () => {
      const caseTypes = [
        { id: "id-incident", label: "Incident" },
        { id: "id-query", label: "Query" },
      ];
      expect(getIncidentAndQueryIds(caseTypes)).toEqual({
        incidentId: "id-incident",
        queryId: "id-query",
      });
    });

    it("should return empty object when caseTypes is empty or undefined", () => {
      expect(getIncidentAndQueryIds([])).toEqual({});
      expect(getIncidentAndQueryIds(undefined)).toEqual({});
    });

    it("should match labels case-insensitively", () => {
      const caseTypes = [
        { id: "id1", label: "INCIDENT" },
        { id: "id2", label: "query" },
      ];
      expect(getIncidentAndQueryIds(caseTypes)).toEqual({
        incidentId: "id1",
        queryId: "id2",
      });
    });

    it("should accept Icident typo variant", () => {
      const caseTypes = [
        { id: "id-icident", label: "Icident" },
        { id: "id-query", label: "Query" },
      ];
      expect(getIncidentAndQueryIds(caseTypes)).toEqual({
        incidentId: "id-icident",
        queryId: "id-query",
      });
    });
  });

  describe("toPresentTenseActionLabel", () => {
    it("should map Closed to Close", () => {
      expect(toPresentTenseActionLabel("Closed")).toBe("Close");
    });
    it("should map Reopened to Reopen", () => {
      expect(toPresentTenseActionLabel("Reopened")).toBe("Reopen");
    });
    it("should map Waiting on WSO2 to Wait on WSO2", () => {
      expect(toPresentTenseActionLabel("Waiting on WSO2")).toBe("Wait on WSO2");
    });
    it("should return unchanged for unmapped labels", () => {
      expect(toPresentTenseActionLabel("Accept Solution")).toBe(
        "Accept Solution",
      );
    });
  });

  describe("toPresentContinuousActionLabel", () => {
    it("should map Closed to Closing...", () => {
      expect(toPresentContinuousActionLabel("Closed")).toBe("Closing...");
    });
    it("should map Reopened to Reopening...", () => {
      expect(toPresentContinuousActionLabel("Reopened")).toBe("Reopening...");
    });
    it("should map Accept Solution to Accepting...", () => {
      expect(toPresentContinuousActionLabel("Accept Solution")).toBe(
        "Accepting...",
      );
    });
    it("should map Reject Solution to Rejecting...", () => {
      expect(toPresentContinuousActionLabel("Reject Solution")).toBe(
        "Rejecting...",
      );
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

    it("should return CircleCheck for Resolved", () => {
      expect(getStatusIcon("Resolved")).toBe(CircleCheck);
    });

    it("should return CircleX for Closed", () => {
      expect(getStatusIcon("Closed")).toBe(CircleX);
    });
  });

  describe("hasSingleCodeWrapper", () => {
    it("returns false for empty or multiple [code] blocks", () => {
      expect(hasSingleCodeWrapper("")).toBe(false);
      expect(hasSingleCodeWrapper("[code]a[/code][code]b[/code]")).toBe(false);
    });

    it("returns true for single [code]...[/code] wrapper", () => {
      expect(hasSingleCodeWrapper("[code]x[/code]")).toBe(true);
      expect(hasSingleCodeWrapper("[code]<p>Hello</p>[/code]")).toBe(true);
    });
  });

  describe("stripCodeWrapper", () => {
    it("should return empty string for empty or invalid input", () => {
      expect(stripCodeWrapper("")).toBe("");
      expect(stripCodeWrapper(null as unknown as string)).toBe("");
    });

    it("should return content unchanged when no [code] wrapper or multiple blocks", () => {
      expect(stripCodeWrapper("plain text")).toBe("plain text");
      expect(stripCodeWrapper("[code]only start")).toBe("[code]only start");
      expect(stripCodeWrapper("only end[/code]")).toBe("only end[/code]");
      expect(stripCodeWrapper("[code]a[/code][code]b[/code]")).toBe(
        "[code]a[/code][code]b[/code]",
      );
    });

    it("should strip [code]...[/code] wrapper when single block", () => {
      expect(stripCodeWrapper("[code]x[/code]")).toBe("x");
      expect(stripCodeWrapper("[code]  hello  [/code]")).toBe("hello");
    });
  });

  describe("convertCodeTagsToHtml", () => {
    it("should return empty string for empty or invalid input", () => {
      expect(convertCodeTagsToHtml("")).toBe("");
      expect(convertCodeTagsToHtml(null as unknown as string)).toBe("");
    });

    it("should convert [code]...[/code] to <code> elements", () => {
      expect(convertCodeTagsToHtml("[code]CSTASK0010746[/code]")).toBe(
        "<code>CSTASK0010746</code>",
      );
      expect(
        convertCodeTagsToHtml("Case Task [code]CSTASK0010746[/code] has been created"),
      ).toBe("Case Task <code>CSTASK0010746</code> has been created");
    });

    it("should convert multiple inline code tags", () => {
      expect(
        convertCodeTagsToHtml("Refs [code]A[/code] and [code]B[/code]"),
      ).toBe("Refs <code>A</code> and <code>B</code>");
    });
  });

  describe("stripAllCodeBlocks", () => {
    it("should strip all [code]...[/code] blocks and return inner HTML", () => {
      expect(
        stripAllCodeBlocks(
          "[code]<br><b>Title</b>[/code][code]<br><p>Desc</p>[/code]",
        ),
      ).toBe("<br><b>Title</b><br><p>Desc</p>");
    });
    it("should return empty for empty input", () => {
      expect(stripAllCodeBlocks("")).toBe("");
    });
  });

  describe("trimLeadingBr", () => {
    it("should remove leading br tags", () => {
      expect(trimLeadingBr("<br><b>Title</b>")).toBe("<b>Title</b>");
      expect(trimLeadingBr("<br/><br><b>Title</b>")).toBe("<b>Title</b>");
    });
    it("should return trimmed string for empty input", () => {
      expect(trimLeadingBr("")).toBe("");
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
    it("extractInlineImageRefId parses absolute ServiceNow .iix URLs", () => {
      expect(
        extractInlineImageRefId(
          "https://wso2sndev.wso2.com/a133c8cc1bff76900bb3da47b04bcb67.iix",
        ),
      ).toBe("a133c8cc1bff76900bb3da47b04bcb67");
    });

    it("should return empty string for empty html", () => {
      expect(replaceInlineImageSources("")).toBe("");
    });

    it("should return sanitized html when no attachments", () => {
      const html = '<img src="/abc.iix" alt="x">';
      expect(replaceInlineImageSources(html, null)).toContain("src");
    });

    it("should normalize JSON-escaped forward slashes so images render", () => {
      const html = '<img src=\"https:\\/\\/example.com\\/img.png\" alt=\"x\">';
      // The returned HTML should contain a usable URL (without the escaping backslashes)
      expect(replaceInlineImageSources(html, null)).toContain("https://example.com/img.png");
    });

    it("should replace src by sys_id when attachment matches", () => {
      const html = '<img src="/sys123.iix" alt="x">';
      const attachments = [
        { sys_id: "sys123", url: "https://example.com/img.png" },
      ];
      const result = replaceInlineImageSources(html, attachments);
      expect(result).toContain("https://example.com/img.png");
    });

    it("should replace src by id when attachment matches", () => {
      const html = '<img src="/att456.iix">';
      const attachments = [
        { id: "att456", downloadUrl: "https://cdn.example.com/att.png" },
      ];
      const result = replaceInlineImageSources(html, attachments);
      expect(result).toContain("https://cdn.example.com/att.png");
    });

    it("should use same-origin .iix as img src and keep download URL as fallback attr", () => {
      const html =
        '<img src="https://wso2sndev.wso2.com/att456.iix" alt="inline">';
      const attachments = [
        {
          id: "att456",
          downloadUrl: "https://wso2sndev.wso2.com/sys_attachment.do?sys_id=att456",
        },
      ];
      const result = replaceInlineImageSources(html, attachments);
      expect(result).toContain('src="https://wso2sndev.wso2.com/att456.iix"');
      expect(result).toContain("data-inline-download-url=");
      expect(result).toContain("sys_attachment.do");
    });

    it("should handle single-quoted src", () => {
      const html = "<img src='/sys99.iix' alt=''>";
      const attachments = [
        { sys_id: "sys99", url: "https://example.com/a.png" },
      ];
      const result = replaceInlineImageSources(html, attachments);
      expect(result).toContain("https://example.com/a.png");
    });
  });

  describe("getInitials", () => {
    it("should derive initials from name", () => {
      expect(getInitials("John Doe")).toBe("JD");
      expect(getInitials("Alice")).toBe("A");
    });

    it("should derive initials from { id, label } object (assigned engineer)", () => {
      expect(getInitials({ id: "eng-1", label: "Agzaiyenth Ganaraj" })).toBe(
        "AG",
      );
      expect(getInitials({ id: "eng-2", label: "Sarah Chen" })).toBe("SC");
    });

    it("should derive initials from { id, name } object (case details API)", () => {
      expect(getInitials({ id: "eng-1", name: "John Doe" })).toBe("JD");
    });

    it("should return -- for null, undefined, empty", () => {
      expect(getInitials(null)).toBe("--");
      expect(getInitials(undefined)).toBe("--");
      expect(getInitials("")).toBe("--");
    });
  });

  describe("formatUtcToLocal", () => {
    it("should return -- for null and undefined", () => {
      expect(formatUtcToLocal(null)).toBe("--");
      expect(formatUtcToLocal(undefined)).toBe("--");
    });

    it("should parse YYYY-MM-DD HH:mm:ss as UTC and format in local", () => {
      const result = formatUtcToLocal("2024-10-29 10:00:00");
      expect(result).toContain("Oct");
      expect(result).toContain("29");
      expect(result).toContain("2024");
    });

    it("should parse MM/DD/YYYY HH:mm:ss as UTC and format in local", () => {
      const result = formatUtcToLocal("10/29/2024 14:00:00");
      expect(result).toContain("Oct");
      expect(result).toContain("29");
      expect(result).toContain("2024");
    });

    it("should return -- for invalid date", () => {
      expect(formatUtcToLocal("not-a-date")).toBe("--");
    });

    it("should format with short variant without four-digit year in output", () => {
      const result = formatUtcToLocal("2024-10-29 14:00:00", "short");
      expect(result).toContain("Oct");
      expect(result).toContain("29");
      expect(result).not.toMatch(/\b2024\b/);
    });
  });

  describe("formatDateOnly", () => {
    it("should return -- for null and undefined", () => {
      expect(formatDateOnly(null)).toBe("--");
      expect(formatDateOnly(undefined)).toBe("--");
    });

    it("should return -- for invalid date string", () => {
      expect(formatDateOnly("not-a-date")).toBe("--");
      expect(formatDateOnly("")).toBe("--");
    });

    it("should format valid ISO datetime to date-only string", () => {
      const result = formatDateOnly("2024-10-29T14:30:00Z");
      expect(result).toContain("Oct");
      expect(result).toContain("29");
      expect(result).toContain("2024");
      expect(result).not.toMatch(/14:30/);
      expect(result).not.toMatch(/PM/);
    });

    it("should format ServiceNow-style YYYY-MM-DD HH:mm:ss to date-only string", () => {
      const result = formatDateOnly("2024-10-29 14:30:00");
      expect(result).toContain("Oct");
      expect(result).toContain("29");
      expect(result).toContain("2024");
      expect(result).not.toMatch(/14:30/);
      expect(result).not.toMatch(/PM/);
    });

    it("should correctly parse year 2026 from API format", () => {
      const result = formatDateOnly("2026-02-25 15:24:47");
      expect(result).toBe("Feb 25, 2026");
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

    it("should parse YYYY-MM-DD HH:MM:SS as local wall time (no UTC shift)", () => {
      const result = formatCommentDate("2026-02-13 15:45:00");
      expect(result).toMatch(/Feb/);
      expect(result).toMatch(/13/);
      expect(result).toMatch(/2026/);
    });
  });

  describe("toDatetimeLocalInputFromApiString", () => {
    it("should map YYYY-MM-DD HH:mm:ss to datetime-local without UTC shift", () => {
      expect(toDatetimeLocalInputFromApiString("2026-03-27 13:34:56")).toBe(
        "2026-03-27T13:34",
      );
    });

    it("should map T-separated local wall time without Z", () => {
      expect(toDatetimeLocalInputFromApiString("2026-03-27T13:34:56")).toBe(
        "2026-03-27T13:34",
      );
    });
  });

  describe("parseApiLocalDateTimeMs", () => {
    it("should parse T-separated unzoned as local wall", () => {
      const space = parseApiLocalDateTimeMs("2026-03-27 13:32:54");
      const tsep = parseApiLocalDateTimeMs("2026-03-27T13:32:54");
      expect(space).toBe(tsep);
    });
  });

  describe("compareByCreatedOnThenId", () => {
    it("should order human before Novera when createdOn matches", () => {
      const human = {
        createdOn: "2026-03-27 13:29:37",
        id: "b",
        createdBy: "user@wso2.com",
        type: "comments",
      };
      const novera = {
        createdOn: "2026-03-27 13:29:37",
        id: "a",
        createdBy: "Novera",
        type: "comments",
      };
      const sorted = [novera, human].sort(compareByCreatedOnThenId);
      expect(sorted[0]?.createdBy).toBe("user@wso2.com");
      expect(sorted[1]?.createdBy).toBe("Novera");
    });
  });

  describe("dateFromApiCreatedOn", () => {
    it("should order consecutive API local timestamps", () => {
      const a = dateFromApiCreatedOn("2026-03-27 13:32:54");
      const b = dateFromApiCreatedOn("2026-03-27 13:32:55");
      expect(a.getTime()).toBeLessThan(b.getTime());
    });
  });

  describe("stripHtml", () => {
    it("should remove html tags", () => {
      expect(stripHtml("<p>Hello</p>")).toBe("Hello");
      expect(stripHtml("<b>Bold</b> and <i>Italic</i>")).toBe(
        "Bold and Italic",
      );
    });

    it("should return empty string for null, undefined or non-string", () => {
      expect(stripHtml(null)).toBe("");
      expect(stripHtml(undefined)).toBe("");
      expect(stripHtml("" as any)).toBe("");
    });
  });

  describe("mapSeverityToDisplay", () => {
    it("should map P-format severity labels to S0-S4", () => {
      expect(mapSeverityToDisplay("Critical (P1)")).toBe("S1");
      expect(mapSeverityToDisplay("High (P2)")).toBe("S2");
    });

    it("should map announcement format severity labels (1 - Critical, etc.)", () => {
      expect(mapSeverityToDisplay("1 - Critical")).toBe("S1");
      expect(mapSeverityToDisplay("2 - High")).toBe("S2");
      expect(mapSeverityToDisplay("3 - Moderate")).toBe("S3");
      expect(mapSeverityToDisplay("4 - Low")).toBe("S4");
      expect(mapSeverityToDisplay("0 - Catastrophic")).toBe("S0");
    });

    it("should return original label when no match", () => {
      expect(mapSeverityToDisplay("Unknown")).toBe("Unknown");
    });
  });

  describe("getSeverityIcon", () => {
    it("should return TriangleAlert for S0/S1", () => {
      expect(getSeverityIcon("S0")).toBe(TriangleAlert);
      expect(getSeverityIcon("S1")).toBe(TriangleAlert);
      expect(getSeverityIcon("1 - Critical")).toBe(TriangleAlert);
    });

    it("should return CircleAlert for S2", () => {
      expect(getSeverityIcon("S2")).toBe(CircleAlert);
    });

    it("should return Clock for S3", () => {
      expect(getSeverityIcon("S3")).toBe(Clock);
      expect(getSeverityIcon("3 - Moderate")).toBe(Clock);
    });

    it("should return CircleCheck for S4", () => {
      expect(getSeverityIcon("S4")).toBe(CircleCheck);
      expect(getSeverityIcon("4 - Low")).toBe(CircleCheck);
    });

    it("should return CircleAlert for unknown or empty input", () => {
      expect(getSeverityIcon("bogus")).toBe(CircleAlert);
      expect(getSeverityIcon("")).toBe(CircleAlert);
      expect(getSeverityIcon(undefined)).toBe(CircleAlert);
    });
  });

  describe("getAnnouncementCaseTypeId", () => {
    it("should return Announcement id from caseTypes", () => {
      const caseTypes = [
        { id: "3b8b43311b58f010cb6898aebd4bcb8f", label: "Announcement" },
        { id: "8d4b87bd1b18f010cb6898aebd4bcb59", label: "Incident" },
      ];
      expect(getAnnouncementCaseTypeId(caseTypes)).toBe(
        "3b8b43311b58f010cb6898aebd4bcb8f",
      );
    });

    it("should return undefined when Announcement not found", () => {
      const caseTypes = [
        { id: "1", label: "Incident" },
        { id: "2", label: "Query" },
      ];
      expect(getAnnouncementCaseTypeId(caseTypes)).toBeUndefined();
    });

    it("should return undefined for empty or null input", () => {
      expect(getAnnouncementCaseTypeId([])).toBeUndefined();
      expect(getAnnouncementCaseTypeId(null)).toBeUndefined();
      expect(getAnnouncementCaseTypeId(undefined)).toBeUndefined();
    });
  });

  describe("getAvailableCaseActions", () => {
    it("should return Open Related Case for Closed", () => {
      expect(getAvailableCaseActions("Closed")).toEqual(["Open Related Case"]);
      expect(getAvailableCaseActions("closed")).toEqual(["Open Related Case"]);
    });

    it("should return all actions for Solution Proposed", () => {
      expect(getAvailableCaseActions("Solution Proposed")).toEqual([
        "Closed",
        "Accept Solution",
        "Reject Solution",
      ]);
    });

    it("should return subset for Awaiting Info", () => {
      expect(getAvailableCaseActions("Awaiting Info")).toEqual([
        "Closed",
        "Waiting on WSO2",
      ]);
    });

    it("should return only Closed for other states", () => {
      expect(getAvailableCaseActions("Open")).toEqual(["Closed"]);
      expect(getAvailableCaseActions("Work in Progress")).toEqual(["Closed"]);
      expect(getAvailableCaseActions("Reopened")).toEqual(["Closed"]);
      expect(getAvailableCaseActions(null)).toEqual(["Closed"]);
    });
  });
});
