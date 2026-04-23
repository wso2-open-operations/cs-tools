// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
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
import {
  normalizeProductLabel,
  buildClassificationProductLabel,
  resolveDeploymentMatch,
  resolveProductId,
  resolveIssueTypeKey,
  findMatchingProductLabel,
  findMatchingDeploymentLabel,
  shouldAddClassificationProductToOptions,
  getBaseProductOptions,
  isUnknownPlaceholderProductLabel,
  getBaseDeploymentOptions,
} from "@features/support/utils/caseCreation";
import type { DeploymentProductItem } from "@features/project-details/types/deployments";

describe("caseCreation utils", () => {
  describe("normalizeProductLabel", () => {
    it("returns empty string for undefined or empty input", () => {
      expect(normalizeProductLabel(undefined)).toBe("");
      expect(normalizeProductLabel("")).toBe("");
    });

    it("replaces hyphen with spaces with single space", () => {
      expect(normalizeProductLabel("WSO2 API Manager - 3.2.0")).toBe(
        "wso2 api manager 3.2.0",
      );
    });

    it("collapses multiple spaces", () => {
      expect(normalizeProductLabel("WSO2   Identity   Server")).toBe(
        "wso2 identity server",
      );
    });

    it("trims leading and trailing whitespace", () => {
      expect(normalizeProductLabel("  WSO2 API Manager 3.2.0  ")).toBe(
        "wso2 api manager 3.2.0",
      );
    });
  });

  describe("buildClassificationProductLabel", () => {
    it("returns empty string when caseInfo is undefined", () => {
      expect(buildClassificationProductLabel(undefined)).toBe("");
    });

    it("returns empty string when productName is missing or blank", () => {
      expect(
        buildClassificationProductLabel({
          productName: "",
          productVersion: "3.2.0",
        }),
      ).toBe("");
    });

    it("returns productName only when productVersion is missing", () => {
      expect(
        buildClassificationProductLabel({
          productName: "WSO2 API Manager",
          productVersion: "",
        }),
      ).toBe("WSO2 API Manager");
    });

    it("returns productName and productVersion separated by space", () => {
      expect(
        buildClassificationProductLabel({
          productName: "WSO2 API Manager",
          productVersion: "3.2.0",
        }),
      ).toBe("WSO2 API Manager 3.2.0");
    });
  });

  describe("resolveDeploymentMatch", () => {
    const projectDeployments = [
      { id: "dep-1", name: "Development", type: { id: "t1", label: "Dev" } },
      { id: "dep-2", name: "Staging", type: { id: "t2", label: "Staging" } },
    ];
    const filterDeployments = [
      { id: "f1", label: "Production" },
      { id: "f2", label: "QA" },
    ];

    it("returns null for empty label", () => {
      expect(
        resolveDeploymentMatch("", projectDeployments, filterDeployments),
      ).toBeNull();
    });

    it("matches by project deployment name", () => {
      const result = resolveDeploymentMatch(
        "Development",
        projectDeployments,
        filterDeployments,
      );
      expect(result).toEqual({ id: "dep-1" });
    });

    it("matches by project deployment type label", () => {
      const result = resolveDeploymentMatch(
        "Staging",
        projectDeployments,
        filterDeployments,
      );
      expect(result).toEqual({ id: "dep-2" });
    });

    it("matches by filter deployment label", () => {
      const result = resolveDeploymentMatch(
        "Production",
        projectDeployments,
        filterDeployments,
      );
      expect(result).toEqual({ id: "f1" });
    });

    it("returns null when no match", () => {
      expect(
        resolveDeploymentMatch(
          "Unknown",
          projectDeployments,
          filterDeployments,
        ),
      ).toBeNull();
    });

    it("matches by project deployment name (case-insensitive)", () => {
      const result = resolveDeploymentMatch(
        "development",
        projectDeployments,
        filterDeployments,
      );
      expect(result).toEqual({ id: "dep-1" });
    });

    it("matches by filter deployment label (case-insensitive)", () => {
      const result = resolveDeploymentMatch(
        "production",
        projectDeployments,
        filterDeployments,
      );
      expect(result).toEqual({ id: "f1" });
    });
  });

  describe("resolveProductId", () => {
    const allDeploymentProducts: DeploymentProductItem[] = [
      {
        id: "1",
        createdOn: "",
        updatedOn: "",
        description: null,
        product: { id: "pid-1", label: "WSO2 API Manager 3.2.0" },
        deployment: { id: "d1", label: "Dev" },
      },
      {
        id: "2",
        createdOn: "",
        updatedOn: "",
        description: null,
        product: { id: "pid-2", label: "WSO2 Identity Server - 6.0.0" },
        deployment: { id: "d1", label: "Dev" },
      },
    ];

    it("returns empty string for empty label", () => {
      expect(resolveProductId("", allDeploymentProducts)).toBe("");
    });

    it("matches by exact label and returns DeploymentProductItem.id", () => {
      expect(
        resolveProductId("WSO2 API Manager 3.2.0", allDeploymentProducts),
      ).toBe("1");
    });

    it("matches by normalized label (hyphen vs space) and returns DeploymentProductItem.id", () => {
      expect(
        resolveProductId("WSO2 Identity Server 6.0.0", allDeploymentProducts),
      ).toBe("2");
    });

    it("matches by normalized label (case insensitive) and returns DeploymentProductItem.id", () => {
      expect(
        resolveProductId("wso2 api manager 3.2.0", allDeploymentProducts),
      ).toBe("1");
    });

    it("returns empty string when no match", () => {
      expect(resolveProductId("Unknown Product", allDeploymentProducts)).toBe(
        "",
      );
    });

    it("returns id when input is already a deployment product id", () => {
      expect(resolveProductId("1", allDeploymentProducts)).toBe("1");
    });
  });

  describe("resolveIssueTypeKey", () => {
    const issueTypes = [
      { id: "1", label: "Question" },
      { id: "2", label: "Incident" },
    ];

    it("returns 0 for empty label", () => {
      expect(resolveIssueTypeKey("", issueTypes)).toBe(0);
    });

    it("matches by id", () => {
      expect(resolveIssueTypeKey("1", issueTypes)).toBe(1);
    });

    it("matches by label", () => {
      expect(resolveIssueTypeKey("Incident", issueTypes)).toBe(2);
    });

    it("returns parsed number when no match but numeric string", () => {
      expect(resolveIssueTypeKey("5", issueTypes)).toBe(5);
    });
  });

  describe("shouldAddClassificationProductToOptions", () => {
    it("returns false for empty classification product", () => {
      expect(
        shouldAddClassificationProductToOptions("", [
          { id: "1", label: "WSO2 API Manager 3.2.0" },
        ]),
      ).toBe(false);
    });

    it("returns false when classification product matches a base option (normalized)", () => {
      expect(
        shouldAddClassificationProductToOptions("WSO2 API Manager 3.2.0", [
          { id: "1", label: "WSO2 API Manager - 3.2.0" },
        ]),
      ).toBe(false);
    });

    it("returns false when classification product matches a base option (case insensitive)", () => {
      expect(
        shouldAddClassificationProductToOptions("wso2 api manager 3.2.0", [
          { id: "1", label: "WSO2 API Manager 3.2.0" },
        ]),
      ).toBe(false);
    });

    it("returns true when classification product is not in base options", () => {
      expect(
        shouldAddClassificationProductToOptions("WSO2 Choreo 1.0", [
          { id: "1", label: "WSO2 API Manager 3.2.0" },
        ]),
      ).toBe(true);
    });
  });

  describe("findMatchingDeploymentLabel", () => {
    const baseOptions = ["Production", "Staging", "Development"];

    it("returns undefined for empty label", () => {
      expect(findMatchingDeploymentLabel("", baseOptions)).toBeUndefined();
    });

    it("returns undefined for whitespace-only label", () => {
      expect(findMatchingDeploymentLabel("   ", baseOptions)).toBeUndefined();
    });

    it("returns matching option (exact match)", () => {
      expect(findMatchingDeploymentLabel("Production", baseOptions)).toBe(
        "Production",
      );
    });

    it("returns matching option (case-insensitive)", () => {
      expect(findMatchingDeploymentLabel("production", baseOptions)).toBe(
        "Production",
      );
      expect(findMatchingDeploymentLabel("STAGING", baseOptions)).toBe(
        "Staging",
      );
    });

    it("returns undefined when no match", () => {
      expect(findMatchingDeploymentLabel("QA", baseOptions)).toBeUndefined();
    });
  });

  describe("findMatchingProductLabel", () => {
    const baseOptions = [
      "WSO2 API Manager 3.2.0",
      "WSO2 Identity Server - 6.0.0",
    ];

    it("returns undefined for empty label", () => {
      expect(findMatchingProductLabel("", baseOptions)).toBeUndefined();
    });

    it("returns undefined for whitespace-only label", () => {
      expect(findMatchingProductLabel("   ", baseOptions)).toBeUndefined();
    });

    it("returns matching base option (exact match)", () => {
      expect(
        findMatchingProductLabel("WSO2 API Manager 3.2.0", baseOptions),
      ).toBe("WSO2 API Manager 3.2.0");
    });

    it("returns matching base option when label format differs (hyphen vs space)", () => {
      expect(
        findMatchingProductLabel("WSO2 Identity Server 6.0.0", baseOptions),
      ).toBe("WSO2 Identity Server - 6.0.0");
    });

    it("returns undefined when no match", () => {
      expect(
        findMatchingProductLabel("WSO2 Choreo 1.0", baseOptions),
      ).toBeUndefined();
    });

    it("returns matching base option (case-insensitive)", () => {
      expect(
        findMatchingProductLabel("wso2 api manager 3.2.0", baseOptions),
      ).toBe("WSO2 API Manager 3.2.0");
    });
  });

  describe("isUnknownPlaceholderProductLabel", () => {
    it("returns true for empty or unknown-only labels", () => {
      expect(isUnknownPlaceholderProductLabel("")).toBe(true);
      expect(isUnknownPlaceholderProductLabel("   ")).toBe(true);
      expect(isUnknownPlaceholderProductLabel("Unknown")).toBe(true);
      expect(isUnknownPlaceholderProductLabel("unknown UNKNOWN")).toBe(true);
      expect(isUnknownPlaceholderProductLabel("Unknown Unknown")).toBe(true);
    });

    it("returns false for real product labels", () => {
      expect(isUnknownPlaceholderProductLabel("WSO2 API Manager")).toBe(false);
      expect(isUnknownPlaceholderProductLabel("Unknown Product")).toBe(false);
    });
  });

  describe("getBaseProductOptions", () => {
    it("returns one option per deployment product with id and combined product+version label", () => {
      const products: DeploymentProductItem[] = [
        {
          id: "1",
          createdOn: "",
          updatedOn: "",
          description: null,
          product: { id: "p1", label: "WSO2 API Manager" },
          deployment: { id: "d1", label: "Dev" },
          version: { id: "v1", label: "3.2.0" },
        },
        {
          id: "2",
          createdOn: "",
          updatedOn: "",
          description: null,
          product: { id: "p2", label: "WSO2 API Manager" },
          deployment: { id: "d2", label: "Staging" },
          version: { id: "v2", label: "1.8.0" },
        },
      ];
      expect(getBaseProductOptions(products)).toEqual([
        { id: "1", label: "WSO2 API Manager 3.2.0" },
        { id: "2", label: "WSO2 API Manager 1.8.0" },
      ]);
    });

    it("returns empty array for empty input", () => {
      expect(getBaseProductOptions([])).toEqual([]);
    });

    it("omits placeholder Unknown Unknown rows", () => {
      const products: DeploymentProductItem[] = [
        {
          id: "1",
          createdOn: "",
          updatedOn: "",
          description: null,
          product: { id: "p1", label: "Unknown" },
          deployment: { id: "d1", label: "Dev" },
          version: { id: "v1", label: "Unknown" },
        },
        {
          id: "2",
          createdOn: "",
          updatedOn: "",
          description: null,
          product: { id: "p2", label: "WSO2 API Manager" },
          deployment: { id: "d1", label: "Dev" },
          version: { id: "v2", label: "4.2.0" },
        },
      ];
      expect(getBaseProductOptions(products)).toEqual([
        { id: "2", label: "WSO2 API Manager 4.2.0" },
      ]);
    });
  });

  describe("getBaseDeploymentOptions", () => {
    it("returns deployment names or type labels", () => {
      const deployments = [
        { id: "1", name: "Dev", type: { id: "t1", label: "Development" } },
        { id: "2", name: "Staging", type: { id: "t2", label: "Staging" } },
      ];
      expect(getBaseDeploymentOptions(deployments)).toEqual(["Dev", "Staging"]);
    });

    it("returns empty array for undefined", () => {
      expect(getBaseDeploymentOptions(undefined)).toEqual([]);
    });
  });
});
