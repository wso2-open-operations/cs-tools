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
import type { CaseCommentDto } from "./case.dto";
import { toComment } from "./case.model";

const dto = (createdBy: CaseCommentDto["createdBy"]): CaseCommentDto => ({
  id: "c-1",
  caseId: "case-1",
  type: "comment",
  content: "hello",
  createdBy,
  createdOn: "2026-11-10T08:00:00Z",
});

describe("toComment", () => {
  it("flags a comment whose author the backend resolved as internal", () => {
    expect(
      toComment(dto({ id: "u1", email: "staff@example.com", name: "First Last", isInternal: true })),
    ).toMatchObject({
      createdBy: "First Last",
      createdByIsInternal: true,
    });
  });

  it("does not flag a customer, an unresolved author, or a plain-string author", () => {
    expect(
      toComment(dto({ id: "u2", email: "customer@example.com", name: "Customer User", isInternal: false }))
        .createdByIsInternal,
    ).toBe(false);
    expect(toComment(dto({ id: null, email: "unresolved@example.com", name: "Unresolved" })).createdByIsInternal).toBe(
      false,
    );
    expect(toComment(dto("Plain String Author"))).toMatchObject({
      createdBy: "Plain String Author",
      createdByIsInternal: false,
    });
  });
});
