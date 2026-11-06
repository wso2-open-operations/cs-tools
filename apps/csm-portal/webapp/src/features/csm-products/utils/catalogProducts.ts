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


import type {
  CatalogChangePayload,
  CatalogProduct,
  CatalogSupportStatus,
  CatalogVersion,
} from "@features/csm-products/types/productCatalog";
import {
  emptyVersionForm,
  versionToForm,
  type VersionFormValues,
} from "@features/csm-products/utils/catalogChange";

/**
 * Pure helpers for the Products tab's table and its two Add dialogs:
 * which products and versions are listed (not SaaS, not end of life), one row
 * per listing, searching, filtering, sorting by a date column, the product create
 * payload, and the publishing settings a new version starts from (its
 * product's latest version) so it publishes like its siblings.
 */

/** Units whose products are WSO2's cloud (SaaS) offerings. */
const CLOUD_UNITS: ReadonlySet<string> = new Set(["choreo", "asgardeo", "devant", "bijira"]);

/**
 * True for a SaaS product (Choreo, Devant, Bijira, Asgardeo, OpenChoreo, ...):
 * a `service`-class product, one in the integration cloud business unit, or
 * one in a cloud unit. The catalogue never lists these; they are not on the
 * on-premise Support Matrix.
 */
/** The deployment profile a new version gets by default, and the one a listing is shown by. */
export const DEFAULT_PROFILE = "All In One";

export function isCloudProduct(product: CatalogProduct): boolean {
  return (
    product.class === "service" ||
    product.businessUnit === "integration_cloud" ||
    (product.unit !== null && CLOUD_UNITS.has(product.unit))
  );
}

/**
 * True for an end-of-life product: it has versions, but every one is retired
 * or discontinued. A new product with no versions at all is not end of life,
 * so its first version can be added.
 */
export function isEndOfLifeProduct(product: CatalogProduct): boolean {
  return product.activeVersionCount === 0 && product.versionCount + product.retiredVersionCount > 0;
}

/** Products the catalogue lists and a version can be added to: neither cloud nor end of life. */
export function isListedProduct(product: CatalogProduct): boolean {
  return !isCloudProduct(product) && !isEndOfLifeProduct(product);
}

/** What the catalogue shows for a product: its code, else its name. */
export function productDisplayCode(product: CatalogProduct): string {
  return product.code?.trim() || product.name;
}

/**
 * One row of the catalogue table: one public listing of a product version.
 * `version` is the row shown and edited (the "All In One" profile when there
 * is one); `rows` are all its live deployment-profile rows, `version` first.
 * `named` is set when the product has more than one listing of the same
 * version (API Manager 4.5.0 All In One, Control Plane, ...), so the table
 * shows the listing's public name to tell them apart.
 */
export interface CatalogListing {
  key: string;
  product: CatalogProduct;
  version: CatalogVersion;
  rows: CatalogVersion[];
  named: boolean;
}

/** The profile rows of one listing: same public listing, or same version when unpublished. */
function listingKey(version: CatalogVersion): string {
  return version.publicId !== null
    ? `public:${version.publicId}`
    : `version:${version.productId}:${version.version}`;
}

function compareDesc(a: string | null, b: string | null): number {
  if (a === b) return 0;
  if (a === null) return 1;
  if (b === null) return -1;
  return b.localeCompare(a, undefined, { numeric: true });
}

/**
 * The catalogue table's rows: every live, not discontinued version of a
 * listed product (neither cloud nor end of life), one row per listing rather
 * than per deployment profile. Sorted by product code, then newest release
 * first. A row with no status is kept.
 */
export function catalogListings(
  products: readonly CatalogProduct[],
  versions: readonly CatalogVersion[],
): CatalogListing[] {
  const byId = new Map(products.filter(isListedProduct).map((product) => [product.id, product]));
  const groups = new Map<string, CatalogVersion[]>();
  for (const version of versions) {
    if (version.retiredOn || version.currentSupportStatus === "discontinued" || !byId.has(version.productId)) continue;
    const key = listingKey(version);
    const rows = groups.get(key);
    if (rows) rows.push(version);
    else groups.set(key, [version]);
  }
  const perVersion = new Map<string, number>();
  for (const rows of groups.values()) {
    const key = `${rows[0].productId}:${rows[0].version}`;
    perVersion.set(key, (perVersion.get(key) ?? 0) + 1);
  }
  const listings: CatalogListing[] = [];
  for (const [key, rows] of groups) {
    const main = rows.find((row) => row.deploymentProfile === DEFAULT_PROFILE) ?? rows[0];
    listings.push({
      key,
      product: byId.get(main.productId) as CatalogProduct,
      version: main,
      rows: [main, ...rows.filter((row) => row !== main)],
      named: (perVersion.get(`${main.productId}:${main.version}`) ?? 0) > 1,
    });
  }
  return listings.sort(
    (a, b) =>
      productDisplayCode(a.product).localeCompare(productDisplayCode(b.product)) ||
      compareDesc(a.version.releaseDate, b.version.releaseDate) ||
      compareDesc(a.version.version, b.version.version) ||
      (a.version.publicName ?? "").localeCompare(b.version.publicName ?? ""),
  );
}

/** Listings matching `query` (product code or name, version or public name; case-insensitive). */
export function filterCatalogListings(listings: readonly CatalogListing[], query: string): CatalogListing[] {
  const q = query.trim().toLowerCase();
  if (!q) return [...listings];
  return listings.filter(({ product, version }) =>
    [product.code, product.name, version.version, version.publicName].some((text) => (text ?? "").toLowerCase().includes(q)),
  );
}

/** The EOL filter's choices: every version, past EOL, reaching EOL within one, three or six months, or with no EOL date. */
export type CatalogEolFilter = "all" | "past" | "within1Month" | "within3Months" | "within6Months" | "none";

/** How many months ahead each "EOL within" choice looks. */
const EOL_WINDOW_MONTHS: Partial<Record<CatalogEolFilter, number>> = { within1Month: 1, within3Months: 3, within6Months: 6 };

/** The products table's filters, beside the search box. Empty lists mean "any". */
export interface CatalogListingFilters {
  productIds: string[];
  statuses: CatalogSupportStatus[];
  eol: CatalogEolFilter;
}

export const NO_LISTING_FILTERS: CatalogListingFilters = { productIds: [], statuses: [], eol: "all" };

/** Whether any filter narrows the list. */
export function hasListingFilters(filters: CatalogListingFilters): boolean {
  return filters.productIds.length > 0 || filters.statuses.length > 0 || filters.eol !== "all";
}

/** `date` ("YYYY-MM-DD") moved by `months` calendar months, in UTC. */
function addMonths(date: string, months: number): string {
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCMonth(d.getUTCMonth() + months);
  return d.toISOString().slice(0, 10);
}

/**
 * The listings that pass every filter. `today` ("YYYY-MM-DD") decides the EOL
 * filter: past EOL is an EOL date before today; within N months is from
 * today up to (not including) N months from now.
 */
export function applyListingFilters(
  listings: readonly CatalogListing[],
  filters: CatalogListingFilters,
  today: string,
): CatalogListing[] {
  const window = EOL_WINDOW_MONTHS[filters.eol];
  const soon = window === undefined ? today : addMonths(today, window);
  return listings.filter(({ product, version }) => {
    if (filters.productIds.length > 0 && !filters.productIds.includes(product.id)) return false;
    if (filters.statuses.length > 0 && (!version.currentSupportStatus || !filters.statuses.includes(version.currentSupportStatus))) {
      return false;
    }
    const eol = version.supportEolDate;
    switch (filters.eol) {
      case "past":
        return Boolean(eol) && (eol as string) < today;
      case "within1Month":
      case "within3Months":
      case "within6Months":
        return Boolean(eol) && (eol as string) >= today && (eol as string) < soon;
      case "none":
        return !eol;
      default:
        return true;
    }
  });
}

/** The products table's sortable date columns. */
export type CatalogDateField = "releaseDate" | "earliestPossibleSupportEolDate" | "supportEolAnnouncementDate" | "supportEolDate";

/**
 * The listings sorted by one date column. Versions without that date go last
 * whichever way the column is sorted; ties keep their current order (product
 * code, newest release first). No sort keeps the order given.
 */
export function sortListings(
  listings: readonly CatalogListing[],
  sort: { field: CatalogDateField; order: "asc" | "desc" } | null,
): CatalogListing[] {
  if (!sort) return [...listings];
  const dir = sort.order === "asc" ? 1 : -1;
  return listings
    .map((listing, index) => ({ listing, index }))
    .sort((a, b) => {
      const x = a.listing.version[sort.field];
      const y = b.listing.version[sort.field];
      if (!x && !y) return a.index - b.index;
      if (!x) return 1;
      if (!y) return -1;
      return x === y ? a.index - b.index : (x < y ? -1 : 1) * dir;
    })
    .map(({ listing }) => listing);
}

/**
 * The product's most recent live version by release date (a version with no
 * date sorts last), or null when it has none.
 */
export function latestLiveVersion(versions: readonly CatalogVersion[]): CatalogVersion | null {
  let latest: CatalogVersion | null = null;
  for (const version of versions) {
    if (version.retiredOn) continue;
    if (!latest || isBetterSource(version, latest)) latest = version;
  }
  return latest;
}

/**
 * Whether `a` is a better row to copy settings from than `b`: the later
 * release date; on a tie the "All In One" profile, then the lowest public id,
 * then the id. That keeps a profile that is its own listing (with its own
 * public name, as APIM 4.5+ gateways are) from being copied onto a new version.
 */
function isBetterSource(a: CatalogVersion, b: CatalogVersion): boolean {
  const ra = a.releaseDate ?? "";
  const rb = b.releaseDate ?? "";
  if (ra !== rb) return ra > rb;
  const aio = (v: CatalogVersion) => (v.deploymentProfile ?? "").toLowerCase() === DEFAULT_PROFILE.toLowerCase();
  if (aio(a) !== aio(b)) return aio(a);
  const pa = a.publicId ?? Number.POSITIVE_INFINITY;
  const pb = b.publicId ?? Number.POSITIVE_INFINITY;
  if (pa !== pb) return pa < pb;
  return a.id < b.id;
}

/**
 * The publishing settings of a version — what the Add product version
 * dialog shows under "Publishing" and the Edit dialog under "Advanced": public name and abbreviation,
 * description, the two feed flags, release line, tags, release links and the
 * Carbon platform.
 */
export const LISTING_FORM_KEYS = [
  "publicName",
  "publicAbbreviation",
  "description",
  "showInSupportMatrix",
  "isPublic",
  "releaseLineId",
  "tags",
  "releaseLinks",
  "carbon",
] as const;

export type ListingFormValues = Pick<VersionFormValues, (typeof LISTING_FORM_KEYS)[number]>;

/**
 * The publishing settings a new version starts from: those of `source`, its
 * product's latest live version, so it publishes like its siblings. With no
 * source everything is empty and both feed flags are off (the backend's
 * defaults).
 */
export function listingFormFrom(source: CatalogVersion | null): ListingFormValues {
  const form = source ? versionToForm(source) : emptyVersionForm();
  const out = {} as Record<string, unknown>;
  for (const key of LISTING_FORM_KEYS) out[key] = key === "carbon" ? { ...form.carbon } : form[key];
  return out as unknown as ListingFormValues;
}

/** Profiles typed into the Add version dialog: trimmed, blanks dropped, de-duplicated. */
export function normalizeProfiles(profiles: readonly string[]): string[] {
  return Array.from(new Set(profiles.map((p) => p.trim()).filter(Boolean)));
}

/** The Add product dialog's state. */
export interface ProductFormValues {
  name: string;
  code: string;
  class: "software" | "service";
  unit: string;
  businessUnit: string;
  manufacturer: string;
  description: string;
}

export function emptyProductForm(): ProductFormValues {
  return {
    name: "",
    code: "",
    class: "software",
    unit: "",
    businessUnit: "",
    manufacturer: "wso2",
    description: "",
  };
}

/** A product CREATE payload: name and class, plus every other field that is set. */
export function buildProductCreatePayload(form: ProductFormValues): CatalogChangePayload {
  const payload: CatalogChangePayload = { name: form.name.trim(), class: form.class };
  for (const key of ["code", "unit", "businessUnit", "manufacturer", "description"] as const) {
    const value = form[key].trim();
    if (value) payload[key] = value;
  }
  return payload;
}

export function validateProductForm(form: ProductFormValues): Record<string, string> {
  const errors: Record<string, string> = {};
  if (!form.name.trim()) errors.name = "Name is required.";
  else if (form.name.trim().length > 100) errors.name = "At most 100 characters.";
  if (form.code.trim().length > 64) errors.code = "At most 64 characters.";
  return errors;
}
