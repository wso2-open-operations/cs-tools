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
  shouldAddClassificationProductToOptions,
  getBaseProductOptions,
  getBaseDeploymentOptions,
} from "@utils/caseCreation";
import type { DeploymentProductItem } from "@models/responses";

describe("caseCreation utils", () => {
  describe("normalizeProductLabel", () => {
    it("returns empty string for undefined or empty input", () => {
      expect(normalizeProductLabel(undefined)).toBe("");
      expect(normalizeProductLabel("")).toBe("");
    });

    it("replaces hyphen with spaces with single space", () => {
      expect(normalizeProductLabel("WSO2 API Manager - 3.2.0")).toBe(
        "WSO2 API Manager 3.2.0",
      );
    });

    it("collapses multiple spaces", () => {
      expect(normalizeProductLabel("WSO2   Identity   Server")).toBe(
        "WSO2 Identity Server",
      );
    });

    it("trims leading and trailing whitespace", () => {
      expect(normalizeProductLabel("  WSO2 API Manager 3.2.0  ")).toBe(
        "WSO2 API Manager 3.2.0",
      );
    });
  });

  describe("buildClassificationProductLabel", () => {
    it("returns empty string when case_info is undefined", () => {
      expect(buildClassificationProductLabel(undefined)).toBe("");
    });

    it("returns empty string when productName is missing or blank", () => {
      expect(
        buildClassificationProductLabel({
          productName: "",
          productVersion: "3.2.0",
          description: "",
          shortDescription: "",
          environment: "",
          tier: "",
          region: "",
        }),
      ).toBe("");
    });

    it("returns productName only when productVersion is missing", () => {
      expect(
        buildClassificationProductLabel({
          productName: "WSO2 API Manager",
          productVersion: "",
          description: "",
          shortDescription: "",
          environment: "",
          tier: "",
          region: "",
        }),
      ).toBe("WSO2 API Manager");
    });

    it("returns productName and productVersion separated by space", () => {
      expect(
        buildClassificationProductLabel({
          productName: "WSO2 API Manager",
          productVersion: "3.2.0",
          description: "",
          shortDescription: "",
          environment: "",
          tier: "",
          region: "",
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

    it("matches by exact label", () => {
      expect(
        resolveProductId("WSO2 API Manager 3.2.0", allDeploymentProducts),
      ).toBe("pid-1");
    });

    it("matches by normalized label (hyphen vs space)", () => {
      expect(
        resolveProductId("WSO2 Identity Server 6.0.0", allDeploymentProducts),
      ).toBe("pid-2");
    });

    it("returns empty string when no match", () => {
      expect(resolveProductId("Unknown Product", allDeploymentProducts)).toBe(
        "",
      );
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
        shouldAddClassificationProductToOptions("", ["WSO2 API Manager 3.2.0"]),
      ).toBe(false);
    });

    it("returns false when classification product matches a base option (normalized)", () => {
      expect(
        shouldAddClassificationProductToOptions("WSO2 API Manager 3.2.0", [
          "WSO2 API Manager - 3.2.0",
        ]),
      ).toBe(false);
    });

    it("returns true when classification product is not in base options", () => {
      expect(
        shouldAddClassificationProductToOptions("WSO2 Choreo 1.0", [
          "WSO2 API Manager 3.2.0",
        ]),
      ).toBe(true);
    });
  });

  describe("getBaseProductOptions", () => {
    it("returns unique product labels from deployment products", () => {
      const products: DeploymentProductItem[] = [
        {
          id: "1",
          createdOn: "",
          updatedOn: "",
          description: null,
          product: { id: "p1", label: "WSO2 API Manager 3.2.0" },
          deployment: { id: "d1", label: "Dev" },
        },
        {
          id: "2",
          createdOn: "",
          updatedOn: "",
          description: null,
          product: { id: "p2", label: "WSO2 API Manager 3.2.0" },
          deployment: { id: "d2", label: "Staging" },
        },
      ];
      expect(getBaseProductOptions(products)).toEqual([
        "WSO2 API Manager 3.2.0",
      ]);
    });

    it("returns empty array for empty input", () => {
      expect(getBaseProductOptions([])).toEqual([]);
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
