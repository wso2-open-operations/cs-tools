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
  CatalogCarbon,
  CatalogChangePayload,
  CatalogReleaseLine,
  CatalogSupportStatus,
  CatalogVersion,
} from "@features/csm-products/types/productCatalog";

/**
 * Pure helpers behind the catalogue's edit dialogs: the form shape, the wire
 * value of each field, and the payload a save sends. A change payload carries
 * only keys the user changed — a present key sets the field and null clears
 * it, an absent key leaves it alone — so an edit never overwrites a field
 * someone else changed in the meantime.
 */

export const CARBON_KEYS: readonly (keyof CatalogCarbon)[] = [
  "platformRelease",
  "platformVersion",
  "carbonKernelVersion",
  "releaseChunk",
  "kernelPatchNo",
];

/** Every product_version key an UPDATE may set, in form order. */
export const VERSION_UPDATE_KEYS = [
  "deploymentProfile",
  "publicId",
  "currentSupportStatus",
  "releaseDate",
  "earliestPossibleSupportEolDate",
  "supportEolAnnouncementDate",
  "supportEolDate",
  "publicName",
  "publicAbbreviation",
  "description",
  "releaseLinks",
  "isPublic",
  "showInSupportMatrix",
  "tags",
  "legacyStatus",
  "releaseLineId",
  "carbon",
] as const;

export type VersionUpdateKey = (typeof VERSION_UPDATE_KEYS)[number];

/**
 * The fields the Edit version dialog offers, in form order: the support
 * status and EOL dates up front, then the release date and the publishing
 * settings under Advanced. All are listing fields, which entity-service
 * applies to every live row of the version; the row-specific ones
 * (deployment profile, public id) and the legacy status are not edited here.
 */
export const VERSION_EDIT_KEYS = [
  "currentSupportStatus",
  "earliestPossibleSupportEolDate",
  "supportEolAnnouncementDate",
  "supportEolDate",
  "releaseDate",
  "publicName",
  "publicAbbreviation",
  "description",
  "showInSupportMatrix",
  "isPublic",
  "releaseLineId",
  "tags",
  "releaseLinks",
  "carbon",
] as const satisfies readonly VersionUpdateKey[];

/** The edit dialog's state: text inputs hold strings, "" meaning empty. */
export interface VersionFormValues {
  version: string;
  deploymentProfile: string;
  publicId: string;
  currentSupportStatus: CatalogSupportStatus | "";
  releaseDate: string;
  earliestPossibleSupportEolDate: string;
  supportEolDate: string;
  supportEolAnnouncementDate: string;
  publicName: string;
  publicAbbreviation: string;
  description: string;
  releaseLinks: string;
  isPublic: boolean;
  showInSupportMatrix: boolean;
  /** Comma separated. */
  tags: string;
  legacyStatus: "" | "true" | "false";
  releaseLineId: string;
  carbon: Record<keyof CatalogCarbon, string>;
}

export function emptyVersionForm(): VersionFormValues {
  return {
    version: "",
    deploymentProfile: "",
    publicId: "",
    currentSupportStatus: "",
    releaseDate: "",
    earliestPossibleSupportEolDate: "",
    supportEolDate: "",
    supportEolAnnouncementDate: "",
    publicName: "",
    publicAbbreviation: "",
    description: "",
    releaseLinks: "",
    isPublic: false,
    showInSupportMatrix: false,
    tags: "",
    legacyStatus: "",
    releaseLineId: "",
    carbon: {
      platformRelease: "",
      platformVersion: "",
      carbonKernelVersion: "",
      releaseChunk: "",
      kernelPatchNo: "",
    },
  };
}

/** A live row as form values. */
export function versionToForm(version: CatalogVersion): VersionFormValues {
  const merged: Record<string, unknown> = { ...version };
  const text = (key: string): string => {
    const value = merged[key];
    return value === null || value === undefined ? "" : String(value);
  };
  const carbon = (merged.carbon ?? null) as Partial<CatalogCarbon> | null;
  const legacy = merged.legacyStatus;
  return {
    version: text("version"),
    deploymentProfile: text("deploymentProfile"),
    publicId: text("publicId"),
    currentSupportStatus: text("currentSupportStatus") as VersionFormValues["currentSupportStatus"],
    releaseDate: text("releaseDate"),
    earliestPossibleSupportEolDate: text("earliestPossibleSupportEolDate"),
    supportEolDate: text("supportEolDate"),
    supportEolAnnouncementDate: text("supportEolAnnouncementDate"),
    publicName: text("publicName"),
    publicAbbreviation: text("publicAbbreviation"),
    description: text("description"),
    releaseLinks: text("releaseLinks"),
    isPublic: Boolean(merged.isPublic),
    showInSupportMatrix: Boolean(merged.showInSupportMatrix),
    tags: Array.isArray(merged.tags) ? (merged.tags as unknown[]).map(String).join(", ") : "",
    legacyStatus: legacy === true ? "true" : legacy === false ? "false" : "",
    releaseLineId: text("releaseLineId"),
    carbon: {
      platformRelease: carbon?.platformRelease ?? "",
      platformVersion: carbon?.platformVersion ?? "",
      carbonKernelVersion: carbon?.carbonKernelVersion ?? "",
      releaseChunk: carbon?.releaseChunk ?? "",
      kernelPatchNo: carbon?.kernelPatchNo ?? "",
    },
  };
}

function textOrNull(value: string): string | null {
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

/** Comma-separated tags as the wire array: trimmed, blanks dropped, de-duplicated. */
export function parseTags(value: string): string[] {
  return Array.from(new Set(value.split(",").map((t) => t.trim()).filter(Boolean)));
}

/** A Carbon object with every empty sub-field as null, or null when all are empty. */
export function normalizeCarbon(carbon: Partial<Record<keyof CatalogCarbon, string | null>> | null): CatalogCarbon | null {
  if (!carbon) return null;
  const out = {} as CatalogCarbon;
  let any = false;
  for (const key of CARBON_KEYS) {
    const value = carbon[key];
    const normalized = typeof value === "string" ? textOrNull(value) : null;
    out[key] = normalized;
    if (normalized !== null) any = true;
  }
  return any ? out : null;
}

/** The wire value the form holds for one key. */
export function formWireValue(form: VersionFormValues, key: VersionUpdateKey): unknown {
  switch (key) {
    case "publicId": {
      const trimmed = form.publicId.trim();
      return trimmed === "" ? null : Number(trimmed);
    }
    case "currentSupportStatus":
      return form.currentSupportStatus || null;
    case "isPublic":
      return form.isPublic;
    case "showInSupportMatrix":
      return form.showInSupportMatrix;
    case "tags":
      return parseTags(form.tags);
    case "legacyStatus":
      return form.legacyStatus === "" ? null : form.legacyStatus === "true";
    case "carbon":
      return normalizeCarbon(form.carbon);
    default:
      return textOrNull(form[key]);
  }
}

/** The wire value a live row holds for one key, normalized like the form's. */
export function liveWireValue(version: CatalogVersion, key: VersionUpdateKey): unknown {
  switch (key) {
    case "tags":
      return version.tags ?? [];
    case "carbon":
      return normalizeCarbon(version.carbon);
    case "isPublic":
    case "showInSupportMatrix":
      return Boolean(version[key]);
    default: {
      const value = version[key];
      return value === undefined || value === "" ? null : value;
    }
  }
}

/** Structural equality for wire values (scalars, arrays, flat objects). */
export function sameWireValue(a: unknown, b: unknown): boolean {
  return JSON.stringify(a ?? null) === JSON.stringify(b ?? null);
}

/**
 * The Edit dialog's PATCH body: only the fields it shows (VERSION_EDIT_KEYS)
 * that differ from the live row. entity-service applies them to every live
 * row of the version, so nothing row-specific is sent.
 */
export function buildVersionEditPayload(live: CatalogVersion, form: VersionFormValues): CatalogChangePayload {
  const payload: CatalogChangePayload = {};
  for (const key of VERSION_EDIT_KEYS) {
    const next = formWireValue(form, key);
    if (!sameWireValue(next, liveWireValue(live, key))) payload[key] = next;
  }
  return payload;
}

/** An UPDATE payload holding only the keys whose value differs from the live row. */
export function buildVersionUpdatePayload(live: CatalogVersion, form: VersionFormValues): CatalogChangePayload {
  const payload: CatalogChangePayload = {};
  for (const key of VERSION_UPDATE_KEYS) {
    const next = formWireValue(form, key);
    if (!sameWireValue(next, liveWireValue(live, key))) payload[key] = next;
  }
  return payload;
}

/**
 * The body of POST /products/{id}/versions without its `profiles`: the
 * version plus every field that is set. Unset fields are left out so
 * entity-service applies its defaults. A `publicId` is sent only when joining
 * an existing listing; otherwise entity-service takes the next value.
 */
export function buildVersionCreatePayload(form: VersionFormValues, joinPublicId?: number | null): CatalogChangePayload {
  const payload: CatalogChangePayload = { version: form.version.trim() };
  for (const key of VERSION_UPDATE_KEYS) {
    if (key === "publicId" || key === "deploymentProfile") continue;
    const value = formWireValue(form, key);
    if (value === null || value === false || (Array.isArray(value) && value.length === 0)) continue;
    payload[key] = value;
  }
  if (joinPublicId !== undefined && joinPublicId !== null) payload.publicId = joinPublicId;
  return payload;
}

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

/**
 * Client-side checks that save a round trip; entity-service repeats them (and
 * more) when it applies the change. Returns field key -> message for each problem.
 */
export function validateVersionForm(
  form: VersionFormValues,
  mode: "create" | "update",
  context: { productCode?: string | null } = {},
): Record<string, string> {
  const errors: Record<string, string> = {};
  if (mode === "create") {
    if (!form.version.trim()) errors.version = "Version is required.";
    else if (form.version.trim().length > 20) errors.version = "At most 20 characters.";
  }
  for (const key of ["releaseDate", "earliestPossibleSupportEolDate", "supportEolDate", "supportEolAnnouncementDate"] as const) {
    if (form[key] && !DATE_RE.test(form[key])) errors[key] = "Use YYYY-MM-DD.";
  }
  // Date order, as entity-service enforces it: release <= earliest EOL <= EOL,
  // and the announcement no later than the EOL. Each rule needs both dates.
  const ok = (key: keyof VersionFormValues) => Boolean(form[key]) && !errors[key];
  if (ok("releaseDate") && ok("earliestPossibleSupportEolDate") && form.earliestPossibleSupportEolDate < form.releaseDate) {
    errors.earliestPossibleSupportEolDate = "Earliest possible EOL cannot be before the release date.";
  }
  if (ok("releaseDate") && ok("supportEolDate") && form.supportEolDate < form.releaseDate) {
    errors.supportEolDate = "EOL date cannot be before the release date.";
  } else if (
    ok("earliestPossibleSupportEolDate") &&
    ok("supportEolDate") &&
    form.supportEolDate < form.earliestPossibleSupportEolDate
  ) {
    errors.supportEolDate = "EOL date cannot be before the earliest possible EOL.";
  }
  if (ok("supportEolAnnouncementDate") && ok("supportEolDate") && form.supportEolAnnouncementDate > form.supportEolDate) {
    errors.supportEolAnnouncementDate = "EOL announcement cannot be after the EOL date.";
  }
  if (form.publicId.trim() && !/^\d+$/.test(form.publicId.trim())) errors.publicId = "A whole number.";
  if (form.publicName.trim().length > 150) errors.publicName = "At most 150 characters.";
  if (form.publicAbbreviation.trim().length > 64) errors.publicAbbreviation = "At most 64 characters.";
  else if (
    context.productCode !== undefined &&
    (form.isPublic || form.showInSupportMatrix) &&
    !form.publicAbbreviation.trim() &&
    !context.productCode?.trim()
  ) {
    // The public feeds sort by abbreviation; it falls back to the product code.
    errors.publicAbbreviation = "Needed on the public feeds: this product has no code.";
  }
  if (form.releaseLinks.trim().length > 255) errors.releaseLinks = "At most 255 characters.";
  return errors;
}

/** The release line dialog's state. */
export interface ReleaseLineFormValues {
  name: string;
  startDate: string;
  status: string;
}

export function releaseLineToForm(line: CatalogReleaseLine | null): ReleaseLineFormValues {
  return { name: line?.name ?? "", startDate: line?.startDate ?? "", status: line?.status ?? "" };
}

/**
 * A release line CREATE (`live` null: every set field) or UPDATE (only the
 * changed fields) payload.
 */
export function buildReleaseLinePayload(live: CatalogReleaseLine | null, form: ReleaseLineFormValues): CatalogChangePayload {
  const next: Record<keyof ReleaseLineFormValues, string | null> = {
    name: textOrNull(form.name),
    startDate: textOrNull(form.startDate),
    status: textOrNull(form.status),
  };
  const payload: CatalogChangePayload = {};
  for (const key of Object.keys(next) as (keyof ReleaseLineFormValues)[]) {
    if (live === null) {
      if (next[key] !== null) payload[key] = next[key];
    } else if (!sameWireValue(next[key], live[key] ?? null)) {
      payload[key] = next[key];
    }
  }
  return payload;
}

export function validateReleaseLineForm(form: ReleaseLineFormValues): Record<string, string> {
  const errors: Record<string, string> = {};
  if (!form.name.trim()) errors.name = "Name is required.";
  else if (form.name.trim().length > 100) errors.name = "At most 100 characters.";
  if (form.startDate && !DATE_RE.test(form.startDate)) errors.startDate = "Use YYYY-MM-DD.";
  if (form.status.trim().length > 50) errors.status = "At most 50 characters.";
  return errors;
}
