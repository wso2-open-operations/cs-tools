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
import { buildAudienceCsv } from "@features/csm-announcements/utils/audienceCsv";

describe("buildAudienceCsv", () => {
  it("emits a header row plus one row per project", () => {
    const csv = buildAudienceCsv([
      { id: "1", key: "WSO2-1", name: "Project One", accountName: "Acme" },
      { id: "2", key: "WSO2-2", name: "Project Two", accountName: "Globex" },
    ]);
    const lines = csv.split("\r\n");
    expect(lines[0]).toBe('"Project key","Project name","Account"');
    expect(lines[1]).toBe('"WSO2-1","Project One","Acme"');
    expect(lines[2]).toBe('"WSO2-2","Project Two","Globex"');
    expect(lines).toHaveLength(3);
  });

  it("emits an empty Account field rather than the literal string null", () => {
    const csv = buildAudienceCsv([
      { id: "1", key: "WSO2-1", name: "No account project", accountName: null },
    ]);
    expect(csv.split("\r\n")[1]).toBe('"WSO2-1","No account project",""');
  });

  it("escapes an embedded double quote per RFC 4180 instead of corrupting the row", () => {
    const csv = buildAudienceCsv([
      { id: "1", key: "WSO2-1", name: 'Project "Alpha"', accountName: "Acme" },
    ]);
    expect(csv.split("\r\n")[1]).toBe('"WSO2-1","Project ""Alpha""","Acme"');
  });

  it("produces only the header row for an empty audience", () => {
    const csv = buildAudienceCsv([]);
    expect(csv).toBe('"Project key","Project name","Account"');
  });

  it.each(["=SUM(A1)", "+1+1", "-1+1", "@SUM(A1)", "\tSUM", "\rSUM", "\nSUM"])(
    "neutralizes a formula-trigger prefix (%j) so a spreadsheet app doesn't evaluate it",
    (dangerous) => {
      const csv = buildAudienceCsv([
        { id: "1", key: dangerous, name: "Project One", accountName: "Acme" },
      ]);
      const row = csv.split("\r\n")[1];
      // Quoted and prefixed with a leading single quote, not the raw formula-triggering value.
      expect(row).toBe(`"'${dangerous}","Project One","Acme"`);
    },
  );

  it("does not alter a value that merely contains, but doesn't start with, a formula-trigger character", () => {
    const csv = buildAudienceCsv([
      { id: "1", key: "WSO2-1", name: "Project = Alpha", accountName: "Acme" },
    ]);
    expect(csv.split("\r\n")[1]).toBe('"WSO2-1","Project = Alpha","Acme"');
  });
});
