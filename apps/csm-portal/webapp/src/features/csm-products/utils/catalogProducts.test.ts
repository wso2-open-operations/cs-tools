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
  applyListingFilters,
  sortListings,
  NO_LISTING_FILTERS,
  type CatalogListing,
  buildProductCreatePayload,
  emptyProductForm,
  catalogListings,
  filterCatalogListings,
  isCloudProduct,
  isEndOfLifeProduct,
  isListedProduct,
  latestLiveVersion,
  listingFormFrom,
  normalizeProfiles,
  productDisplayCode,
  validateProductForm,
} from "@features/csm-products/utils/catalogProducts";
import { buildVersionCreatePayload, emptyVersionForm } from "@features/csm-products/utils/catalogChange";
import type { CatalogProduct, CatalogVersion } from "@features/csm-products/types/productCatalog";

function product(overrides: Partial<CatalogProduct> = {}): CatalogProduct {
  return {
    id: "p1",
    name: "Example Server",
    code: "exs",
    class: "software",
    unit: "integration",
    businessUnit: "integration_software",
    manufacturer: "wso2",
    description: null,
    versionCount: 2,
    retiredVersionCount: 0,
    activeVersionCount: 2,
    createdOn: "",
    updatedOn: "",
    ...overrides,
  };
}

function version(overrides: Partial<CatalogVersion> = {}): CatalogVersion {
  return {
    id: "v1",
    productId: "p1",
    productName: "Example Server",
    productCode: "exs",
    version: "1.0.0",
    deploymentProfile: "All In One",
    serialNumber: null,
    currentSupportStatus: "available",
    releaseDate: "2024-01-15",
    earliestPossibleSupportEolDate: null,
    supportEolDate: null,
    supportEolAnnouncementDate: null,
    publicId: 1001,
    publicName: "Example",
    publicAbbreviation: "ex",
    description: "Old notes.",
    isPublic: true,
    showInSupportMatrix: true,
    tags: ["OB"],
    releaseLinks: null,
    legacyStatus: true,
    releaseLineId: "line-1",
    releaseLineName: "Line A",
    carbon: null,
    retiredOn: null,
    createdOn: "",
    updatedOn: "",
    ...overrides,
  };
}

describe("isCloudProduct", () => {
  it("flags service-class, integration-cloud and cloud-unit products", () => {
    expect(isCloudProduct(product())).toBe(false);
    expect(isCloudProduct(product({ class: "service" }))).toBe(true);
    expect(isCloudProduct(product({ businessUnit: "integration_cloud" }))).toBe(true);
    for (const unit of ["choreo", "asgardeo", "devant", "bijira"]) {
      expect(isCloudProduct(product({ unit }))).toBe(true);
    }
    expect(isCloudProduct(product({ unit: null, businessUnit: null }))).toBe(false);
  });
});

describe("isEndOfLifeProduct", () => {
  it("treats a product whose versions are all retired or discontinued as end of life", () => {
    expect(isEndOfLifeProduct(product({ versionCount: 0, retiredVersionCount: 3, activeVersionCount: 0 }))).toBe(true);
    expect(isEndOfLifeProduct(product({ versionCount: 2, retiredVersionCount: 0, activeVersionCount: 0 }))).toBe(true);
    expect(isEndOfLifeProduct(product({ versionCount: 2, retiredVersionCount: 1, activeVersionCount: 1 }))).toBe(false);
  });

  it("keeps a brand-new product with no versions at all", () => {
    expect(isEndOfLifeProduct(product({ versionCount: 0, retiredVersionCount: 0, activeVersionCount: 0 }))).toBe(false);
  });

  it("lists only products that are neither cloud nor end of life", () => {
    expect(isListedProduct(product())).toBe(true);
    expect(isListedProduct(product({ unit: "devant" }))).toBe(false);
    expect(isListedProduct(product({ versionCount: 1, retiredVersionCount: 1, activeVersionCount: 0 }))).toBe(false);
  });
});

describe("catalogListings", () => {
  const exs = product({ id: "p1", name: "Example Server", code: "exs" });
  const abc = product({ id: "b", name: "Another Server", code: "abc" });
  const cloud = product({ id: "c", name: "Cloud Thing", code: "ct", unit: "choreo" });
  const eol = product({ id: "d", name: "Old Thing", code: "old", versionCount: 1, activeVersionCount: 0 });
  const products = [exs, abc, cloud, eol];

  it("folds a listing's profiles into one row, the All In One profile first", () => {
    const rows = [
      version({ id: "gw", publicId: 1, deploymentProfile: "Gateway" }),
      version({ id: "aio", publicId: 1, deploymentProfile: "All In One" }),
    ];
    const [listing] = catalogListings(products, rows);
    expect(listing.version.id).toBe("aio");
    expect(listing.rows.map((v) => v.id)).toEqual(["aio", "gw"]);
    expect(listing.named).toBe(false);
  });

  it("folds an unpublished version's profiles by version", () => {
    const rows = [
      version({ id: "x", publicId: null, deploymentProfile: "All In One" }),
      version({ id: "y", publicId: null, deploymentProfile: "Business Process" }),
      version({ id: "z", publicId: null, version: "2.0.0" }),
    ];
    expect(catalogListings(products, rows).map((l) => l.rows.map((v) => v.id))).toEqual([["z"], ["x", "y"]]);
  });

  it("names the listings of a version that has several", () => {
    const rows = [
      version({ id: "aio", publicId: 1, publicName: "Example" }),
      version({ id: "cp", publicId: 2, publicName: "Control Plane", deploymentProfile: "Control Plane" }),
    ];
    expect(catalogListings(products, rows).map((l) => [l.version.id, l.named])).toEqual([
      ["cp", true],
      ["aio", true],
    ]);
  });

  it("leaves out cloud and end-of-life products and discontinued or retired rows", () => {
    const rows = [
      version({ id: "ok", publicId: 1 }),
      version({ id: "none", publicId: 2, version: "0.9.0", currentSupportStatus: null }),
      version({ id: "disc", publicId: 3, version: "0.8.0", currentSupportStatus: "discontinued" }),
      version({ id: "ret", publicId: 4, version: "0.7.0", retiredOn: "2026-01-01T00:00:00Z" }),
      version({ id: "cloud", publicId: 5, productId: "c" }),
      version({ id: "eol", publicId: 6, productId: "d" }),
    ];
    expect(catalogListings(products, rows).map((l) => l.version.id)).toEqual(["ok", "none"]);
  });

  it("sorts by product code, then newest release first", () => {
    const rows = [
      version({ id: "exs-old", publicId: 1, version: "1.0.0", releaseDate: "2020-01-01" }),
      version({ id: "exs-new", publicId: 2, version: "2.0.0", releaseDate: "2024-01-01" }),
      version({ id: "abc", publicId: 3, productId: "b", releaseDate: "2019-01-01" }),
      version({ id: "exs-undated", publicId: 4, version: "3.0.0", releaseDate: null }),
    ];
    expect(catalogListings(products, rows).map((l) => l.version.id)).toEqual([
      "abc",
      "exs-new",
      "exs-old",
      "exs-undated",
    ]);
  });

  it("searches product code or name, version and public name", () => {
    const listings = catalogListings(products, [
      version({ id: "one", publicId: 1, version: "1.0.0", publicName: "Example" }),
      version({ id: "two", publicId: 2, productId: "b", version: "4.5.0", publicName: "Control Plane" }),
    ]);
    const ids = (q: string): string[] => filterCatalogListings(listings, q).map((l) => l.version.id);
    expect(ids("")).toEqual(["two", "one"]);
    expect(ids("EXS")).toEqual(["one"]);
    expect(ids("another")).toEqual(["two"]);
    expect(ids("4.5")).toEqual(["two"]);
    expect(ids("control")).toEqual(["two"]);
  });

  it("shows the code, falling back to the name", () => {
    expect(productDisplayCode(exs)).toBe("exs");
    expect(productDisplayCode(product({ code: null, name: "Other Thing" }))).toBe("Other Thing");
  });
});

describe("latestLiveVersion", () => {
  it("picks the latest release date among live rows", () => {
    const rows = [
      version({ id: "old", releaseDate: "2023-01-01" }),
      version({ id: "new", releaseDate: "2025-06-01" }),
      version({ id: "retired", releaseDate: "2026-01-01", retiredOn: "2026-02-01T00:00:00Z" }),
      version({ id: "undated", releaseDate: null }),
    ];
    expect(latestLiveVersion(rows)?.id).toBe("new");
    expect(latestLiveVersion([])).toBeNull();
  });

  it("breaks a release-date tie by All In One, then the lowest public id", () => {
    const day = "2025-06-01";
    const gateway = version({ id: "gw", releaseDate: day, deploymentProfile: "Universal Gateway", publicId: 1070 });
    const aio = version({ id: "aio", releaseDate: day, deploymentProfile: "All In One", publicId: 1080 });
    const other = version({ id: "km", releaseDate: day, deploymentProfile: "Key Manager", publicId: 1060 });
    expect(latestLiveVersion([gateway, aio, other])?.id).toBe("aio");
    expect(latestLiveVersion([gateway, other])?.id).toBe("km");
  });
});

describe("copying the publishing settings from the latest version", () => {
  it("pre-fills the advanced fields from the source version", () => {
    expect(listingFormFrom(version())).toMatchObject({
      publicName: "Example",
      publicAbbreviation: "ex",
      description: "Old notes.",
      isPublic: true,
      showInSupportMatrix: true,
      releaseLineId: "line-1",
      tags: "OB",
      releaseLinks: "",
    });
  });

  it("leaves everything unset with no source", () => {
    expect(listingFormFrom(null)).toMatchObject({ isPublic: false, showInSupportMatrix: false, publicName: "", releaseLineId: "" });
  });

  it("sends the copied settings with the new version's own support fields, never the source's", () => {
    const form = { ...emptyVersionForm(), version: "2.0.0", releaseDate: "2026-05-01" };
    const payload = buildVersionCreatePayload({ ...form, ...listingFormFrom(version()) });
    expect(payload).toEqual({
      version: "2.0.0",
      releaseDate: "2026-05-01",
      publicName: "Example",
      publicAbbreviation: "ex",
      description: "Old notes.",
      isPublic: true,
      showInSupportMatrix: true,
      tags: ["OB"],
      releaseLineId: "line-1",
    });
  });

  it("copies nothing from a product with no versions", () => {
    const form = { ...emptyVersionForm(), version: "1.0.0" };
    expect(buildVersionCreatePayload({ ...form, ...listingFormFrom(null) })).toEqual({ version: "1.0.0" });
  });
});

describe("product form", () => {
  it("sends name and class plus the fields that are set", () => {
    expect(buildProductCreatePayload({ ...emptyProductForm(), name: " New Product ", code: "np", unit: "iam" })).toEqual({
      name: "New Product",
      class: "software",
      code: "np",
      unit: "iam",
      manufacturer: "wso2",
    });
  });

  it("requires a name", () => {
    expect(validateProductForm(emptyProductForm()).name).toBeTruthy();
    expect(validateProductForm({ ...emptyProductForm(), name: "x" })).toEqual({});
  });
});

describe("normalizeProfiles", () => {
  it("trims, drops blanks and de-duplicates", () => {
    expect(normalizeProfiles([" All In One", "", "Gateway", "All In One"])).toEqual(["All In One", "Gateway"]);
  });
});

describe("listing filters and date sort", () => {
  const listing = (
    id: string,
    productId: string,
    status: CatalogVersion["currentSupportStatus"],
    eol: string | null,
    release: string | null,
  ): CatalogListing => {
    const row = version({ id, productId, currentSupportStatus: status, supportEolDate: eol, releaseDate: release });
    return { key: id, product: product({ id: productId }), version: row, rows: [row], named: false };
  };
  const all = [
    listing("v1", "p1", "available", "2030-01-01", "2024-01-01"),
    listing("v2", "p1", "extended", "2026-01-01", "2022-01-01"),
    listing("v3", "p2", "deprecated", "2027-03-01", null),
    listing("v4", "p2", "available", null, "2025-06-01"),
  ];
  const keys = (rows: CatalogListing[]): string[] => rows.map((r) => r.key);

  it("keeps everything with no filters", () => {
    expect(keys(applyListingFilters(all, NO_LISTING_FILTERS, "2026-11-06"))).toEqual(["v1", "v2", "v3", "v4"]);
  });

  it("filters by product, status and EOL window", () => {
    expect(keys(applyListingFilters(all, { ...NO_LISTING_FILTERS, productIds: ["p2"] }, "2026-11-06"))).toEqual(["v3", "v4"]);
    expect(keys(applyListingFilters(all, { ...NO_LISTING_FILTERS, statuses: ["available"] }, "2026-11-06"))).toEqual(["v1", "v4"]);
    expect(keys(applyListingFilters(all, { ...NO_LISTING_FILTERS, eol: "past" }, "2026-11-06"))).toEqual(["v2"]);
    expect(keys(applyListingFilters(all, { ...NO_LISTING_FILTERS, eol: "within6Months" }, "2026-11-06"))).toEqual(["v3"]);
    expect(keys(applyListingFilters(all, { ...NO_LISTING_FILTERS, eol: "none" }, "2026-11-06"))).toEqual(["v4"]);
  });

  it("looks one, three or six months ahead for the EOL within choices", () => {
    const soon = [
      listing("d20", "p1", "available", "2026-11-20", null),
      listing("j15", "p1", "available", "2027-01-15", null),
      listing("a01", "p1", "available", "2027-04-01", null),
      listing("j01", "p1", "available", "2027-06-01", null),
    ];
    const within = (eol: "within1Month" | "within3Months" | "within6Months") =>
      keys(applyListingFilters(soon, { ...NO_LISTING_FILTERS, eol }, "2026-11-06"));
    expect(within("within1Month")).toEqual(["d20"]);
    expect(within("within3Months")).toEqual(["d20", "j15"]);
    expect(within("within6Months")).toEqual(["d20", "j15", "a01"]);
  });

  it("sorts a date column either way, empty dates last", () => {
    expect(keys(sortListings(all, { field: "releaseDate", order: "desc" }))).toEqual(["v4", "v1", "v2", "v3"]);
    expect(keys(sortListings(all, { field: "releaseDate", order: "asc" }))).toEqual(["v2", "v1", "v4", "v3"]);
    expect(keys(sortListings(all, { field: "supportEolDate", order: "asc" }))).toEqual(["v2", "v3", "v1", "v4"]);
    expect(keys(sortListings(all, null))).toEqual(["v1", "v2", "v3", "v4"]);
  });
});
