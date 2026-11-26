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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import { describe, expect, it } from "vitest";
import { addKindRefusal, deleteKindRefusal } from "./tagRefusal";

describe("deleteKindRefusal", () => {
  it("shows the server's own reason for a 403 or a 409", () => {
    expect(
      deleteKindRefusal({ status: 403, message: "this tag is shared by every family; only a rota admin or a portal admin can delete it" }),
    ).toBe("This tag is shared by every family; only a rota admin or a portal admin can delete it.");
    expect(deleteKindRefusal({ status: 409, message: "the tag is still used by 3 leave or allocation entries; remove those first" })).toBe(
      "The tag is still used by 3 leave or allocation entries; remove those first.",
    );
  });

  it("keeps a plain line when there is no reason to show", () => {
    expect(deleteKindRefusal({ status: 409, message: "" })).toBe("That tag is still used on the rota. Remove those entries first.");
    expect(deleteKindRefusal({ status: 403, message: "" })).toBe("You cannot delete this tag. A built-in tag can never be deleted.");
    expect(deleteKindRefusal({ status: 500, message: "Failed to delete the tag." })).toBe("The tag was not deleted. Try again.");
    expect(deleteKindRefusal(new Error("network"))).toBe("The tag was not deleted. Try again.");
  });
});

describe("addKindRefusal", () => {
  it("shows the server's own reason for a refusal the lead can act on", () => {
    expect(
      addKindRefusal({ status: 400, message: "you run more than one family's rota: say which family the tag is for (family)" }),
    ).toBe("You run more than one family's rota: say which family the tag is for (family).");
    expect(addKindRefusal({ status: 409, message: 'a tag with the short code "SRE" already exists' })).toBe(
      'A tag with the short code "SRE" already exists.',
    );
  });

  it("keeps a plain line when there is no reason to show", () => {
    expect(addKindRefusal({ status: 500, message: "Failed to add the tag." })).toBe("The tag was not added. Try again.");
    expect(addKindRefusal({ status: 403, message: "" })).toBe("Only a team lead can add a tag.");
    expect(addKindRefusal(new Error("network"))).toBe("The tag was not added. Try again.");
  });
});
