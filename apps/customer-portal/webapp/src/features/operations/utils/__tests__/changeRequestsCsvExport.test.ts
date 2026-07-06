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
import type { ChangeRequestItem } from "@features/operations/types/changeRequests";
import {
  buildChangeRequestsExportCsv,
  CHANGE_REQUEST_EXPORT_CSV_HEADERS,
  mapChangeRequestsToCsvRows,
} from "@features/operations/utils/changeRequestsCsvExport";

describe("mapChangeRequestsToCsvRows", () => {
  it("maps change request fields to export columns", () => {
    const rows = mapChangeRequestsToCsvRows([
      {
        number: "CR-1",
        internalId: "INT-CR-1",
        title: "Deploy update",
        state: { id: "1", label: "Scheduled" },
        type: { id: "2", label: "Standard" },
        createdBy: "owner@example.com",
        assignedEngineer: { id: "3", label: "Engineer B" },
        createdOn: "2026-02-01 09:00:00",
        updatedOn: "2026-02-02 10:00:00",
      } as ChangeRequestItem,
    ]);

    expect(rows[0][0]).toBe("CR-1");
    expect(rows[0][1]).toBe("INT-CR-1");
    expect(rows[0][2]).toBe("Scheduled");
    expect(rows[0][3]).toBe("Deploy update");
    expect(rows[0][4]).toBe("Standard");
    expect(rows[0][5]).toBe("owner@example.com");
    expect(rows[0][6]).toBe("Engineer B");
  });
});

describe("buildChangeRequestsExportCsv", () => {
  it("includes required headers", () => {
    const csv = buildChangeRequestsExportCsv([]);
    expect(csv.startsWith(CHANGE_REQUEST_EXPORT_CSV_HEADERS.join(","))).toBe(
      true,
    );
  });
});
