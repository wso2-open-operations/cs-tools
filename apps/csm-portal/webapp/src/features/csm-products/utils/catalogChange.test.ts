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
  buildReleaseLinePayload,
  buildVersionCreatePayload,
  buildVersionEditPayload,
  isPublicAbbreviationRequired,
  buildVersionUpdatePayload,
  emptyVersionForm,
  parseTags,
  validateReleaseLineForm,
  validateVersionForm,
  versionToForm,
} from "@features/csm-products/utils/catalogChange";
import type { CatalogVersion } from "@features/csm-products/types/productCatalog";

const PRODUCT_ID = "11111111-1111-1111-1111-111111111111";
const VERSION_ID = "22222222-2222-2222-2222-222222222222";
const LINE_ID = "33333333-3333-3333-3333-333333333333";

function liveVersion(overrides: Partial<CatalogVersion> = {}): CatalogVersion {
  return {
    id: VERSION_ID,
    productId: PRODUCT_ID,
    productName: "Example Product",
    productCode: "ex",
    version: "1.0.0",
    deploymentProfile: "All In One",
    serialNumber: null,
    currentSupportStatus: "available",
    releaseDate: "2024-01-15",
    earliestPossibleSupportEolDate: "2027-01-15",
    supportEolDate: null,
    supportEolAnnouncementDate: null,
    publicId: 1001,
    publicName: null,
    publicAbbreviation: null,
    description: "First release.",
    isPublic: true,
    showInSupportMatrix: true,
    tags: [],
    releaseLinks: null,
    legacyStatus: true,
    releaseLineId: null,
    releaseLineName: null,
    carbon: null,
    retiredOn: null,
    createdOn: "2024-01-15T00:00:00Z",
    updatedOn: "2024-01-15T00:00:00Z",
    ...overrides,
  };
}

describe("buildVersionUpdatePayload", () => {
  it("is empty when nothing changed", () => {
    const live = liveVersion();
    expect(buildVersionUpdatePayload(live, versionToForm(live))).toEqual({});
  });

  it("carries only the changed keys, with a cleared field as null", () => {
    const live = liveVersion();
    const form = {
      ...versionToForm(live),
      supportEolDate: "2028-01-15",
      description: "  ",
      showInSupportMatrix: false,
    };
    expect(buildVersionUpdatePayload(live, form)).toEqual({
      supportEolDate: "2028-01-15",
      description: null,
      showInSupportMatrix: false,
    });
  });

  it("sends tags as an array and the public id as a number", () => {
    const live = liveVersion();
    const form = { ...versionToForm(live), tags: "OB, x , OB,", publicId: "1002" };
    expect(buildVersionUpdatePayload(live, form)).toEqual({ tags: ["OB", "x"], publicId: 1002 });
  });

  it("sends the whole carbon object when any part changes, and null when it is emptied", () => {
    const live = liveVersion({
      carbon: { platformRelease: "4.4", platformVersion: null, carbonKernelVersion: "4.4.9", releaseChunk: null, kernelPatchNo: null },
    });
    const changed = versionToForm(live);
    changed.carbon = { ...changed.carbon, releaseChunk: "Chunk 1" };
    expect(buildVersionUpdatePayload(live, changed)).toEqual({
      carbon: { platformRelease: "4.4", platformVersion: null, carbonKernelVersion: "4.4.9", releaseChunk: "Chunk 1", kernelPatchNo: null },
    });

    const emptied = versionToForm(live);
    emptied.carbon = { platformRelease: "", platformVersion: "", carbonKernelVersion: " ", releaseChunk: "", kernelPatchNo: "" };
    expect(buildVersionUpdatePayload(live, emptied)).toEqual({ carbon: null });
  });

  it("treats the legacy status as a tri-state", () => {
    const live = liveVersion({ legacyStatus: null });
    expect(buildVersionUpdatePayload(live, { ...versionToForm(live), legacyStatus: "false" })).toEqual({ legacyStatus: false });
  });
});

describe("buildVersionEditPayload", () => {
  it("sends only the fields that changed, support and publishing alike, never row-specific ones", () => {
    const live = liveVersion();
    const form = {
      ...versionToForm(live),
      supportEolDate: "2031-01-31",
      deploymentProfile: "Something else",
      publicId: "999",
      description: "changed",
    };
    expect(buildVersionEditPayload(live, form)).toEqual({ supportEolDate: "2031-01-31", description: "changed" });
    expect(buildVersionEditPayload(live, versionToForm(live))).toEqual({});
  });

  it("clears a publishing field emptied in the form", () => {
    const live = liveVersion({ publicName: "Acme Gateway 1.0" });
    expect(buildVersionEditPayload(live, { ...versionToForm(live), publicName: "  " })).toEqual({ publicName: null });
  });

  it("does not rewrite padded text or repeated tags the user left alone", () => {
    const live = liveVersion({ publicName: "Acme Gateway ", description: " First release.", tags: ["lts", "lts", " beta"] });
    const form = { ...versionToForm(live), supportEolDate: "2031-01-31" };
    expect(buildVersionEditPayload(live, form)).toEqual({ supportEolDate: "2031-01-31" });
    expect(buildVersionEditPayload(live, versionToForm(live))).toEqual({});
  });

  it("treats a tag reorder as no change, and a real tag change as one", () => {
    const live = liveVersion({ tags: ["a", "b"] });
    expect(buildVersionEditPayload(live, { ...versionToForm(live), tags: "b, a" })).toEqual({});
    expect(buildVersionEditPayload(live, { ...versionToForm(live), tags: "b, c" })).toEqual({ tags: ["b", "c"] });
  });
});

describe("buildVersionCreatePayload", () => {
  it("sends the version and only the fields that are set, never a profile", () => {
    const form = {
      ...emptyVersionForm(),
      version: " 2.0.0 ",
      deploymentProfile: "Gateway",
      releaseDate: "2026-05-01",
      currentSupportStatus: "available" as const,
    };
    expect(buildVersionCreatePayload(form)).toEqual({
      version: "2.0.0",
      releaseDate: "2026-05-01",
      currentSupportStatus: "available",
    });
  });

  it("sends the listing's public id when joining one, never the form's", () => {
    const form = { ...emptyVersionForm(), version: "2.0.0", publicId: "999" };
    expect(buildVersionCreatePayload(form, 1001).publicId).toBe(1001);
    expect(buildVersionCreatePayload(form)).not.toHaveProperty("publicId");
  });
});

describe("validateVersionForm", () => {
  it("requires a version on create only", () => {
    expect(validateVersionForm(emptyVersionForm(), "create").version).toBeTruthy();
    expect(validateVersionForm(emptyVersionForm(), "update").version).toBeUndefined();
  });

  it("refuses an EOL date before the release date and a non-numeric public id", () => {
    const errors = validateVersionForm(
      { ...emptyVersionForm(), releaseDate: "2026-01-01", supportEolDate: "2025-01-01", publicId: "abc" },
      "update",
    );
    expect(errors.supportEolDate).toMatch(/before the release date/);
    expect(errors.publicId).toBeTruthy();
  });

  it("applies every date-order rule", () => {
    const base = { ...emptyVersionForm(), releaseDate: "2026-01-01" };
    expect(validateVersionForm({ ...base, earliestPossibleSupportEolDate: "2025-12-31" }, "update").earliestPossibleSupportEolDate).toMatch(
      /before the release date/,
    );
    expect(
      validateVersionForm({ ...base, earliestPossibleSupportEolDate: "2030-01-01", supportEolDate: "2029-01-01" }, "update")
        .supportEolDate,
    ).toMatch(/before the earliest possible EOL/);
    expect(
      validateVersionForm({ ...base, supportEolDate: "2029-01-01", supportEolAnnouncementDate: "2029-06-01" }, "update")
        .supportEolAnnouncementDate,
    ).toMatch(/after the EOL date/);
    expect(
      validateVersionForm(
        { ...base, earliestPossibleSupportEolDate: "2028-01-01", supportEolDate: "2029-01-01", supportEolAnnouncementDate: "2028-06-01" },
        "update",
      ),
    ).toEqual({});
  });

  it("needs an abbreviation on the public feeds when the product has no code", () => {
    const form = { ...emptyVersionForm(), version: "1.0.0", isPublic: true };
    expect(validateVersionForm(form, "create", { productCode: null }).publicAbbreviation).toMatch(/no code/);
    expect(validateVersionForm(form, "create", { productCode: "ex" }).publicAbbreviation).toBeUndefined();
    expect(validateVersionForm({ ...form, publicAbbreviation: "ex" }, "create", { productCode: null })).toEqual({});
    expect(validateVersionForm({ ...form, isPublic: false }, "create", { productCode: null })).toEqual({});
  });

  describe("on an update (with the live row)", () => {
    // A legacy row: on the public feeds, no abbreviation, product has no code,
    // and a public name longer than the limit.
    const legacy = liveVersion({ publicAbbreviation: null, publicName: "x".repeat(160), isPublic: true });

    it("lets a change that leaves the publishing fields alone save", () => {
      const form = { ...versionToForm(legacy), supportEolDate: "2031-01-31" };
      expect(validateVersionForm(form, "update", { productCode: null, live: legacy })).toEqual({});
    });

    it("still checks the publishing fields the change touches", () => {
      const renamed = { ...versionToForm(legacy), publicName: "y".repeat(151) };
      expect(validateVersionForm(renamed, "update", { productCode: null, live: legacy }).publicName).toBeTruthy();
      const listed = { ...versionToForm(liveVersion({ isPublic: false, showInSupportMatrix: false })), isPublic: true };
      expect(
        validateVersionForm(listed, "update", { productCode: null, live: liveVersion({ isPublic: false, showInSupportMatrix: false }) })
          .publicAbbreviation,
      ).toMatch(/no code/);
    });

    it("always checks the date order of the row the change leaves", () => {
      const live = liveVersion({ releaseDate: "2026-01-01" });
      const form = { ...versionToForm(live), supportEolDate: "2025-01-01" };
      expect(validateVersionForm(form, "update", { live }).supportEolDate).toMatch(/before the release date/);
    });

    it("lets a release date be corrected or added, but not cleared", () => {
      const live = liveVersion({ releaseDate: "2024-01-15" });
      expect(validateVersionForm({ ...versionToForm(live), releaseDate: "2024-02-01" }, "update", { live })).toEqual({});
      expect(validateVersionForm({ ...versionToForm(live), releaseDate: "" }, "update", { live }).releaseDate).toMatch(/can't be cleared/);
      const undated = liveVersion({ releaseDate: null });
      expect(validateVersionForm(versionToForm(undated), "update", { live: undated })).toEqual({});
      expect(validateVersionForm({ ...versionToForm(undated), releaseDate: "2024-02-01" }, "update", { live: undated })).toEqual({});
    });
  });

  it("accepts a well-formed form", () => {
    expect(validateVersionForm({ ...emptyVersionForm(), version: "1.0.0", releaseDate: "2026-01-01", supportEolDate: "2029-01-01" }, "create")).toEqual({});
  });
});

describe("release line payloads", () => {
  const line = {
    id: LINE_ID,
    name: "Line A",
    startDate: "2024-01-01",
    status: "Active",
    legacyId: null,
    versionCount: 3,
    retiredOn: null,
    createdOn: "",
    updatedOn: "",
  };

  it("sends every set field on create and only the changed ones on update", () => {
    expect(buildReleaseLinePayload(null, { name: "Line B", startDate: "", status: "Active" })).toEqual({ name: "Line B", status: "Active" });
    expect(buildReleaseLinePayload(line, { name: "Line A", startDate: "", status: "Retiring" })).toEqual({ startDate: null, status: "Retiring" });
  });

  it("requires a name", () => {
    expect(validateReleaseLineForm({ name: " ", startDate: "", status: "" }).name).toBeTruthy();
  });
});

describe("parseTags", () => {
  it("trims, drops blanks and de-duplicates", () => {
    expect(parseTags(" a, ,b,a ")).toEqual(["a", "b"]);
  });
});

describe("isPublicAbbreviationRequired", () => {
  it("is required on create when the product has no code and a feed is on", () => {
    const form = { ...emptyVersionForm(), isPublic: true };
    expect(isPublicAbbreviationRequired(form, { productCode: null })).toBe(true);
    expect(isPublicAbbreviationRequired(form, { productCode: "ex" })).toBe(false);
    expect(isPublicAbbreviationRequired({ ...form, isPublic: false }, { productCode: null })).toBe(false);
    expect(isPublicAbbreviationRequired(form)).toBe(false); // no product context
  });

  it("on an update, only while a feed switch or the abbreviation changes", () => {
    const live = liveVersion({ isPublic: true, showInSupportMatrix: false, publicAbbreviation: null });
    const untouched = versionToForm(live);
    expect(isPublicAbbreviationRequired(untouched, { productCode: null, live })).toBe(false);
    expect(isPublicAbbreviationRequired({ ...untouched, showInSupportMatrix: true }, { productCode: null, live })).toBe(true);
    expect(isPublicAbbreviationRequired({ ...untouched, publicAbbreviation: "EX" }, { productCode: null, live })).toBe(true);
  });
});

describe("tags containing a comma", () => {
  it("are left as stored when the Tags box is untouched", () => {
    const live = liveVersion({ tags: ["alpha,beta", "gamma"] });
    const form = { ...versionToForm(live), supportEolDate: "2031-01-31" };
    expect(buildVersionEditPayload(live, form)).toEqual({ supportEolDate: "2031-01-31" });
    expect(buildVersionEditPayload(live, versionToForm(live))).toEqual({});
  });

  it("are re-parsed only when the user edits the Tags box", () => {
    const live = liveVersion({ tags: ["alpha,beta"] });
    expect(buildVersionEditPayload(live, { ...versionToForm(live), tags: "alpha,beta, delta" })).toEqual({
      tags: ["alpha", "beta", "delta"],
    });
  });
});

