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
  CURRENT_USER_PLACEHOLDER,
  resolveCurrentUserPlaceholder,
} from "./currentUserFilterPlaceholder";

const CURRENT_USER_ID = "11111111-aaaa-bbbb-cccc-000000000001";

describe("resolveCurrentUserPlaceholder", () => {
  describe("case-search DSL shape ({ filters: [...] })", () => {
    it("substitutes the placeholder with the signed-in user's own id when one is available", () => {
      const filters = {
        filters: [
          { field: "state", op: "in", values: ["open"] },
          { field: "assignedUserId", op: "in", values: [CURRENT_USER_PLACEHOLDER] },
        ],
      };

      const resolved = resolveCurrentUserPlaceholder(filters, CURRENT_USER_ID);

      expect(resolved).toEqual({
        filters: [
          { field: "state", op: "in", values: ["open"] },
          { field: "assignedUserId", op: "in", values: [CURRENT_USER_ID] },
        ],
      });
    });

    it("drops the entry entirely when no signed-in user id is available yet", () => {
      const filters = {
        filters: [
          { field: "state", op: "in", values: ["open"] },
          { field: "assignedUserId", op: "in", values: [CURRENT_USER_PLACEHOLDER] },
        ],
      };

      const resolved = resolveCurrentUserPlaceholder(filters, undefined);

      expect(resolved).toEqual({
        filters: [{ field: "state", op: "in", values: ["open"] }],
      });
    });

    it("only substitutes the placeholder entry within a values array, leaving other literal values alone", () => {
      const filters = {
        filters: [
          {
            field: "assignedUserId",
            op: "in",
            values: ["some-literal-user-id", CURRENT_USER_PLACEHOLDER],
          },
        ],
      };

      const resolved = resolveCurrentUserPlaceholder(filters, CURRENT_USER_ID);

      expect(resolved).toEqual({
        filters: [
          {
            field: "assignedUserId",
            op: "in",
            values: ["some-literal-user-id", CURRENT_USER_ID],
          },
        ],
      });
    });

    it("returns filters unchanged when there's no entry carrying the placeholder at all", () => {
      const filters = { filters: [{ field: "state", op: "in", values: ["open"] }] };

      expect(resolveCurrentUserPlaceholder(filters, CURRENT_USER_ID)).toEqual(filters);
      expect(resolveCurrentUserPlaceholder(filters, undefined)).toEqual(filters);
    });

    it("passes through an empty filters array unchanged", () => {
      const filters = { filters: [] };

      expect(resolveCurrentUserPlaceholder(filters, CURRENT_USER_ID)).toBe(filters);
    });
  });

  describe("flat { fieldName: string[] | string } shape", () => {
    it("substitutes the placeholder inside a flat string-array field", () => {
      const filters = { assignedUserIds: [CURRENT_USER_PLACEHOLDER], states: ["open"] };

      const resolved = resolveCurrentUserPlaceholder(filters, CURRENT_USER_ID);

      expect(resolved).toEqual({ assignedUserIds: [CURRENT_USER_ID], states: ["open"] });
    });

    it("drops the placeholder from a flat string-array field, dropping the whole key if the array becomes empty", () => {
      const filters = { assignedUserIds: [CURRENT_USER_PLACEHOLDER], states: ["open"] };

      const resolved = resolveCurrentUserPlaceholder(filters, undefined);

      expect(resolved).toEqual({ states: ["open"] });
    });

    it("drops only the placeholder entry from a flat array carrying other literal values too", () => {
      const filters = { assignedUserIds: ["some-literal-user-id", CURRENT_USER_PLACEHOLDER] };

      const resolved = resolveCurrentUserPlaceholder(filters, undefined);

      expect(resolved).toEqual({ assignedUserIds: ["some-literal-user-id"] });
    });

    it("substitutes a bare placeholder string value directly (not nested in an array)", () => {
      const filters = { createdBy: CURRENT_USER_PLACEHOLDER };

      expect(resolveCurrentUserPlaceholder(filters, CURRENT_USER_ID)).toEqual({
        createdBy: CURRENT_USER_ID,
      });
    });

    it("drops the key entirely for a bare placeholder string value when unresolved", () => {
      const filters = { createdBy: CURRENT_USER_PLACEHOLDER, states: ["open"] };

      expect(resolveCurrentUserPlaceholder(filters, undefined)).toEqual({ states: ["open"] });
    });

    it("returns filters unchanged (same reference) when no field carries the placeholder", () => {
      const filters = { states: ["open"], severities: ["critical"] };

      expect(resolveCurrentUserPlaceholder(filters, CURRENT_USER_ID)).toBe(filters);
    });
  });
});
