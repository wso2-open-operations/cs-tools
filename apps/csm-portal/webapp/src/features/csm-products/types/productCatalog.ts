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

/**
 * Types for the Products section, read from entity-service's products API
 * through the backend (`/products`, `/product-versions`, `/release-lines`).
 *
 * `CatalogVersion` and `CatalogReleaseLine` are the shapes the UI works with:
 * dates `"YYYY-MM-DD"`, support status and type lower case. The API sends
 * dates as RFC 3339 timestamps and the enums upper case (`*Wire` below);
 * `utils/catalogWire.ts` converts, and every hook returns the UI shapes.
 * Timestamps (`createdOn`, `retiredOn`, ...) stay RFC 3339.
 */

export type CatalogSupportStatus = "available" | "extended" | "deprecated" | "discontinued";

export interface CatalogProduct {
  id: string;
  name: string;
  code: string | null;
  class: "software" | "service";
  unit: string | null;
  businessUnit: string | null;
  manufacturer: string | null;
  description: string | null;
  /** Live (not retired) versions. */
  versionCount: number;
  /** Retired versions. */
  retiredVersionCount: number;
  /**
   * Live versions that are not discontinued (a version with no status counts).
   * A product with versions but none of these is end of life and not listed.
   */
  activeVersionCount: number;
  createdOn: string;
  updatedOn: string;
}

export interface CatalogCarbon {
  platformRelease: string | null;
  platformVersion: string | null;
  carbonKernelVersion: string | null;
  releaseChunk: string | null;
  kernelPatchNo: string | null;
}

/**
 * One deployment profile of a version. Rows sharing a `publicId` are one
 * listing on the public feeds.
 */
export interface CatalogVersion {
  id: string;
  productId: string;
  productName: string;
  productCode: string | null;
  version: string;
  deploymentProfile: string | null;
  serialNumber: string | null;
  currentSupportStatus: CatalogSupportStatus | null;
  releaseDate: string | null;
  earliestPossibleSupportEolDate: string | null;
  supportEolDate: string | null;
  supportEolAnnouncementDate: string | null;
  publicId: number | null;
  publicName: string | null;
  publicAbbreviation: string | null;
  description: string | null;
  isPublic: boolean;
  showInSupportMatrix: boolean;
  tags: string[];
  releaseLinks: string | null;
  legacyStatus: boolean | null;
  releaseLineId: string | null;
  releaseLineName: string | null;
  carbon: CatalogCarbon | null;
  retiredOn: string | null;
  createdOn: string;
  updatedOn: string;
}

/** A version as the API sends it: see the file comment. */
export type CatalogVersionWire = Omit<CatalogVersion, "currentSupportStatus"> & {
  currentSupportStatus: string | null;
};

export interface CatalogReleaseLine {
  id: string;
  name: string;
  startDate: string | null;
  status: string | null;
  legacyId: number | null;
  versionCount: number;
  retiredOn: string | null;
  createdOn: string;
  updatedOn: string;
}

/** A write body: a present key sets the field, null clears it, an absent key is left alone. */
export type CatalogChangePayload = Record<string, unknown>;
