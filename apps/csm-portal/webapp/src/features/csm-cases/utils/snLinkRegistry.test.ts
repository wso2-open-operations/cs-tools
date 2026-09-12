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
import { containsSnLink, replaceSnLinks } from "@features/csm-cases/utils/snLinkRegistry";

const ALERT_SYSID = "7a43e2d43b2a4b5091404c6aa5e45a41";
const ALERT_UUID = "7a43e2d4-3b2a-4b50-9140-4c6aa5e45a41";
const SMART_ALERT_SYSID = "00000000000000000000000000000001";
const SMART_ALERT_UUID = "00000000-0000-0000-0000-000000000001";

describe("containsSnLink", () => {
  it("detects a bare alert URL", () => {
    expect(
      containsSnLink(`See https://sn.example.com/u_custom_alert.do?sys_id=${ALERT_SYSID}`),
    ).toBe(true);
  });

  it("detects a bare smart-alert URL", () => {
    expect(
      containsSnLink(
        `https://sn.example.com/u_smart_alert_buffer.do?sys_id=${SMART_ALERT_SYSID}`,
      ),
    ).toBe(true);
  });

  it("detects an already-anchored alert URL", () => {
    expect(
      containsSnLink(
        `<a href="https://sn.example.com/u_custom_alert.do?sys_id=${ALERT_SYSID}">ALT-1</a>`,
      ),
    ).toBe(true);
  });

  it("detects an already-anchored smart-alert URL", () => {
    expect(
      containsSnLink(
        `<a href="https://sn.example.com/u_smart_alert_buffer.do?sys_id=${SMART_ALERT_SYSID}">SALERT-1</a>`,
      ),
    ).toBe(true);
  });

  it("returns false when there is no recognized link", () => {
    expect(containsSnLink("<p>Hello there</p>")).toBe(false);
  });

  it("is not stateful across repeated calls (global-regex lastIndex safety)", () => {
    const html = `https://sn.example.com/u_custom_alert.do?sys_id=${ALERT_SYSID}`;
    expect(containsSnLink(html)).toBe(true);
    expect(containsSnLink(html)).toBe(true);
    expect(containsSnLink(html)).toBe(true);
  });
});

describe("replaceSnLinks — alert (u_custom_alert)", () => {
  it("replaces a bare alert URL with a span marker carrying the converted uuid", () => {
    const html = `See https://sn.example.com/u_custom_alert.do?sys_id=${ALERT_SYSID} for details`;
    const result = replaceSnLinks(html);
    expect(result).toContain(`data-sn-link-type="alert"`);
    expect(result).toContain(`data-sn-link-id="${ALERT_UUID}"`);
    expect(result).toContain('role="button"');
    expect(result).toContain('tabindex="0"');
    expect(result).not.toContain("<a ");
    expect(result).not.toContain(ALERT_SYSID);
  });

  it("replaces an already-anchored alert URL with the marker, dropping the raw anchor", () => {
    const html = `<p>Alert: <a href="https://sn.example.com/u_custom_alert.do?sys_id=${ALERT_SYSID}">ALT-1</a></p>`;
    const result = replaceSnLinks(html);
    expect(result).toContain(`data-sn-link-type="alert"`);
    expect(result).toContain(`data-sn-link-id="${ALERT_UUID}"`);
    expect(result).not.toContain("<a ");
  });
});

describe("replaceSnLinks — smart alert (u_smart_alert_buffer)", () => {
  it("replaces a bare smart-alert URL with a span marker carrying the converted uuid", () => {
    const html = `https://sn.example.com/u_smart_alert_buffer.do?sys_id=${SMART_ALERT_SYSID}`;
    const result = replaceSnLinks(html);
    expect(result).toContain(`data-sn-link-type="smartAlert"`);
    expect(result).toContain(`data-sn-link-id="${SMART_ALERT_UUID}"`);
    expect(result).not.toContain(SMART_ALERT_SYSID);
  });

  it("replaces an already-anchored smart-alert URL with the marker, dropping the raw anchor", () => {
    const html = `<p>Smart alert: <a href="https://sn.example.com/u_smart_alert_buffer.do?sys_id=${SMART_ALERT_SYSID}">SALERT-1</a></p>`;
    const result = replaceSnLinks(html);
    expect(result).toContain(`data-sn-link-type="smartAlert"`);
    expect(result).toContain(`data-sn-link-id="${SMART_ALERT_UUID}"`);
    // The anchor must be fully replaced by the marker span, not left with a
    // <span> nested inside a malformed href attribute.
    expect(result).not.toContain("<a ");
    expect(result).not.toContain("href=");
    expect(result).not.toContain(SMART_ALERT_SYSID);
  });
});

describe("replaceSnLinks — mixed content and passthrough", () => {
  it("replaces both an alert and a smart-alert reference in the same html independently", () => {
    const html =
      `Alert: https://sn.example.com/u_custom_alert.do?sys_id=${ALERT_SYSID}\n` +
      `Smart alert: https://sn.example.com/u_smart_alert_buffer.do?sys_id=${SMART_ALERT_SYSID}`;
    const result = replaceSnLinks(html);
    expect(result).toContain(`data-sn-link-type="alert"`);
    expect(result).toContain(`data-sn-link-id="${ALERT_UUID}"`);
    expect(result).toContain(`data-sn-link-type="smartAlert"`);
    expect(result).toContain(`data-sn-link-id="${SMART_ALERT_UUID}"`);
  });

  it("passes through unrelated HTML unchanged", () => {
    const html = "<p>Hello there</p>";
    expect(replaceSnLinks(html)).toBe(html);
  });

  it("passes through a bare URL that is not a recognized backing-table link", () => {
    const html = "See https://example.com/doc for details";
    expect(replaceSnLinks(html)).toBe(html);
  });

  it("handles empty input without throwing", () => {
    expect(replaceSnLinks("")).toBe("");
  });
});
