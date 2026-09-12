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
import {
  containsCallRequestLink,
  replaceCallRequestLinks,
} from "@features/csm-cases/utils/callRequestLinks";

const SYSID = "7a43e2d43b2a4b5091404c6aa5e45a41";
const UUID = "7a43e2d4-3b2a-4b50-9140-4c6aa5e45a41";

describe("containsCallRequestLink", () => {
  it("detects a bare call-request URL", () => {
    expect(
      containsCallRequestLink(
        `See https://sn-dev.example.com/sn_customerservice_customer_call.do?sys_id=${SYSID}`,
      ),
    ).toBe(true);
  });

  it("returns false when there is no call-request URL", () => {
    expect(containsCallRequestLink("<p>Hello there</p>")).toBe(false);
  });
});

describe("replaceCallRequestLinks", () => {
  it("replaces a bare URL with the in-app marker carrying the converted uuid", () => {
    const html = `See https://sn-dev.example.com/sn_customerservice_customer_call.do?sys_id=${SYSID} for details`;
    const result = replaceCallRequestLinks(html);
    expect(result).toContain(`data-call-request-sysid="${UUID}"`);
    expect(result).toContain("View call request");
    expect(result).not.toContain(SYSID);
    expect(result).not.toContain("sn_customerservice_customer_call.do");
  });

  it("replaces an already-wrapped <a href> anchor with the marker, preserving the original visible text (the task number) as the clickable label", () => {
    const html = `<p>Case Task <a href="https://sn-dev.example.com/sn_customerservice_customer_call.do?sys_id=${SYSID}">CTASK0012345</a> has been created</p>`;
    const result = replaceCallRequestLinks(html);
    expect(result).toContain(`data-call-request-sysid="${UUID}"`);
    // The task number is the label now, not the generic fallback — this is
    // the fix: rendering must read "Case Task CTASK0012345 has been created",
    // not "Case Task View call request has been created".
    expect(result).toContain(">CTASK0012345</span>");
    expect(result).not.toContain("View call request");
    expect(result).not.toContain("sn_customerservice_customer_call.do");
  });

  it("falls back to the generic label when the anchor has no visible text", () => {
    const html = `<a href="https://sn-dev.example.com/sn_customerservice_customer_call.do?sys_id=${SYSID}"></a>`;
    const result = replaceCallRequestLinks(html);
    expect(result).toContain(`data-call-request-sysid="${UUID}"`);
    expect(result).toContain("View call request");
  });

  it("handles multiple occurrences independently", () => {
    const otherSysid = "00000000000000000000000000000001";
    const otherUuid = "00000000-0000-0000-0000-000000000001";
    const html =
      `https://host/sn_customerservice_customer_call.do?sys_id=${SYSID} ` +
      `and https://host/sn_customerservice_customer_call.do?sys_id=${otherSysid}`;
    const result = replaceCallRequestLinks(html);
    expect(result).toContain(`data-call-request-sysid="${UUID}"`);
    expect(result).toContain(`data-call-request-sysid="${otherUuid}"`);
  });

  it("passes through unrelated HTML unchanged", () => {
    const html = "<p>Hello there</p>";
    expect(replaceCallRequestLinks(html)).toBe(html);
  });

  it("passes through a bare URL that is not a call-request link", () => {
    const html = "See https://example.com/doc for details";
    expect(replaceCallRequestLinks(html)).toBe(html);
  });

  it("handles empty/undefined input without throwing", () => {
    expect(replaceCallRequestLinks("")).toBe("");
  });
});
