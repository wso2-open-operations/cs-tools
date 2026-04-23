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

import type { DeploymentProductItem } from "@features/project-details/types/deployments";
import type { CaseClassificationResponse } from "@features/support/types/cases";
import type {
  DeploymentOption,
  IssueTypeOption,
  ProductVersionOption,
  ProjectDeploymentOption,
} from "@features/support/types/caseCreationOptions";
import { ChatSender } from "@features/support/types/conversations";

export type {
  DeploymentOption,
  IssueTypeOption,
  ProjectDeploymentOption,
  ProductVersionOption,
} from "@features/support/types/caseCreationOptions";

/**
 * Normalizes a product label for comparison.
 * Replaces hyphens with spaces and collapses multiple spaces.
 *
 * @param {string | undefined} label - Raw product label.
 * @returns {string} Normalized label for deduplication and matching.
 */
export function normalizeProductLabel(label: string | undefined): string {
  if (!label || typeof label !== "string") return "";
  return label
    .trim()
    .replace(/\s*-\s*/g, " ")
    .replace(/\s+/g, " ")
    .toLowerCase()
    .trim();
}

/**
 * True when the combined product/version label is empty or only placeholder
 * "Unknown" tokens from the API (e.g. "Unknown Unknown").
 *
 * @param {string} label - Combined display label.
 * @returns {boolean} Whether the label should be treated as missing.
 */
export function isUnknownPlaceholderProductLabel(label: string): boolean {
  const parts = label.trim().toLowerCase().split(/\s+/).filter(Boolean);
  if (parts.length === 0) {
    return true;
  }
  return parts.every((p) => p === "unknown");
}

/**
 * Builds the combined product label from case classification response.
 *
 * @param {Partial<Pick<CaseClassificationResponse["caseInfo"], "productName" | "productVersion">> | undefined} caseInfo - caseInfo (or subset) from classification response.
 * @returns {string} Combined label or empty string.
 */
export function buildClassificationProductLabel(
  caseInfo:
    | Partial<
        Pick<
          CaseClassificationResponse["caseInfo"],
          "productName" | "productVersion"
        >
      >
    | undefined,
): string {
  if (!caseInfo?.productName?.trim()) return "";
  const name = caseInfo.productName.trim();
  const version = caseInfo.productVersion?.trim();
  return version ? `${name} ${version}` : name;
}

/**
 * Finds a base deployment option matching the given label (case-insensitive).
 * Returns the actual option string so form state uses the API's exact value.
 *
 * @param {string} deploymentLabel - Classification or selected deployment label.
 * @param {string[]} baseDeploymentOptions - Deployment names from project deployments.
 * @returns {string | undefined} Matching option or undefined.
 */
export function findMatchingDeploymentLabel(
  deploymentLabel: string,
  baseDeploymentOptions: string[],
): string | undefined {
  if (!deploymentLabel?.trim()) return undefined;
  const labelLower = deploymentLabel.trim().toLowerCase();
  return baseDeploymentOptions.find(
    (opt) => opt?.trim().toLowerCase() === labelLower,
  );
}

/**
 * Resolves classification environment (e.g. "Production") to the deployment display label
 * by matching against project deployments' type.label or name. Use to auto-select deployment.
 *
 * @param {string} environmentLabel - caseInfo.environment from classification.
 * @param {ProjectDeploymentOption[]} projectDeployments - Deployments from useGetProjectDeployments.
 * @returns {string | undefined} Display label (name ?? type.label) for the matching deployment.
 */
export function getDeploymentDisplayLabelForEnvironment(
  environmentLabel: string,
  projectDeployments: ProjectDeploymentOption[] | undefined,
): string | undefined {
  if (!environmentLabel?.trim() || !projectDeployments?.length)
    return undefined;
  const envLower = environmentLabel.trim().toLowerCase();
  const dep = projectDeployments.find((d) => {
    const typeLabel = d.type?.label?.trim();
    const name = d.name?.trim();
    return (
      typeLabel?.toLowerCase() === envLower || name?.toLowerCase() === envLower
    );
  });
  if (!dep) return undefined;
  return dep.name || dep.type?.label;
}

/**
 * Resolves deployment ID from the selected deployment label by matching against project deployments.
 *
 * @param {string} deploymentLabel - Selected deployment name/label from the form.
 * @param {ProjectDeploymentOption[]} projectDeployments - Deployments from useGetProjectDeployments.
 * @param {DeploymentOption[]} filterDeployments - Deployments from filters.
 * @returns {{ id: string } | null} Deployment with id, or null if no match.
 */
export function resolveDeploymentMatch(
  deploymentLabel: string,
  projectDeployments: ProjectDeploymentOption[] | undefined,
  filterDeployments: DeploymentOption[] | undefined,
): { id: string } | null {
  const label = deploymentLabel?.trim();
  if (!label) return null;

  const labelLower = label.toLowerCase();

  const fromProjectExact = projectDeployments?.find((d) => {
    const typeLabel = d.type?.label?.trim();
    const depName = d.name?.trim();
    return typeLabel === label || depName === label;
  });
  if (fromProjectExact) return { id: fromProjectExact.id };

  const fromProjectCaseInsensitive = projectDeployments?.find((d) => {
    const typeLabel = d.type?.label?.trim();
    const depName = d.name?.trim();
    return (
      typeLabel?.toLowerCase() === labelLower ||
      depName?.toLowerCase() === labelLower
    );
  });
  if (fromProjectCaseInsensitive) return { id: fromProjectCaseInsensitive.id };

  const fromFiltersExact = filterDeployments?.find(
    (d) => d.id === label || d.label === label,
  );
  if (fromFiltersExact) return { id: fromFiltersExact.id };

  const fromFiltersCaseInsensitive = filterDeployments?.find(
    (d) =>
      d.id?.toLowerCase() === labelLower ||
      d.label?.toLowerCase() === labelLower,
  );
  if (fromFiltersCaseInsensitive) return { id: fromFiltersCaseInsensitive.id };

  return null;
}

/**
 * Resolves product ID from the selected value (id or combined label) against deployment products.
 *
 * @param {string} productLabelOrId - Selected deployment product id or "Product Version" label.
 * @param {DeploymentProductItem[]} allDeploymentProducts - Flat list of deployment products.
 * @returns {string} Deployment product item id or empty string if no match.
 */
export function resolveProductId(
  productLabelOrId: string,
  allDeploymentProducts: DeploymentProductItem[],
): string {
  if (!productLabelOrId?.trim()) return "";
  const input = productLabelOrId.trim();
  const byId = allDeploymentProducts.find((item) => item.id === input);
  if (byId) return byId.id;
  const normalized = normalizeProductLabel(input);
  if (!normalized) return "";
  const match = allDeploymentProducts.find(
    (item) =>
      normalizeProductLabel(getDeploymentProductDisplayLabel(item)) ===
      normalized,
  );
  return match?.id ?? "";
}

/**
 * Resolves issue type key (numeric id) from the selected issue type label.
 *
 * @param {string} issueTypeLabel - Selected issue type (id or label) from the form.
 * @param {IssueTypeOption[]} issueTypes - Issue types from filters.
 * @returns {number} Issue type key (parsed id) or 0.
 */
export function resolveIssueTypeKey(
  issueTypeLabel: string,
  issueTypes: IssueTypeOption[] | undefined,
): number {
  if (!issueTypeLabel?.trim()) return 0;
  const item = issueTypes?.find(
    (t) => t.id === issueTypeLabel || t.label === issueTypeLabel,
  );
  return parseInt(item?.id ?? issueTypeLabel, 10) || 0;
}

/**
 * Finds a base product option that matches the given label (normalized comparison).
 *
 * @param {string} productLabel - Selected or classification product label (e.g. "WSO2 API Manager 3.2.0").
 * @param {string[]} baseProductOptions - Product labels from deployment products (legacy).
 * @returns {string | undefined} Matching base option label, or undefined if no match.
 */
export function findMatchingProductLabel(
  productLabel: string,
  baseProductOptions: string[],
): string | undefined {
  if (!productLabel?.trim()) return undefined;
  const normalized = normalizeProductLabel(productLabel);
  if (!normalized) return undefined;
  return baseProductOptions.find(
    (opt) => normalizeProductLabel(opt) === normalized,
  );
}

/**
 * Finds deployment product option id that matches the given combined label (e.g. "WSO2 API Manager 1.8.0").
 *
 * @param {string} productLabel - Classification or display label.
 * @param {ProductVersionOption[]} baseProductOptions - Options from getBaseProductOptions.
 * @returns {string | undefined} Matching option id, or undefined if no match.
 */
export function findMatchingProductId(
  productLabel: string,
  baseProductOptions: ProductVersionOption[],
): string | undefined {
  if (!productLabel?.trim() || !baseProductOptions?.length) return undefined;
  const normalized = normalizeProductLabel(productLabel);
  if (!normalized) return undefined;
  const opt = baseProductOptions.find(
    (o) => normalizeProductLabel(o.label) === normalized,
  );
  return opt?.id;
}

/**
 * Returns whether the classification product should be added as an extra dropdown option.
 * When using ProductVersionOption[], returns false if a matching option already exists.
 *
 * @param {string} classificationProduct - Combined product label from classification.
 * @param {ProductVersionOption[]} baseProductOptions - Options from getBaseProductOptions.
 * @returns {boolean} True if classification product should be added as extra option.
 */
export function shouldAddClassificationProductToOptions(
  classificationProduct: string,
  baseProductOptions: ProductVersionOption[],
): boolean {
  if (!classificationProduct?.trim()) return false;
  return (
    findMatchingProductId(classificationProduct, baseProductOptions) == null
  );
}

/**
 * Gets display label for a deployment product (product.label + version.label).
 *
 * @param {DeploymentProductItem} item - Deployment product item.
 * @returns {string} Combined label, e.g. "WSO2 API Manager 1.8.0".
 */
export function getDeploymentProductDisplayLabel(
  item: DeploymentProductItem,
): string {
  const productLabel = item.product?.label?.trim() ?? "";
  let versionLabel = "";
  if (
    typeof item.version === "object" &&
    item.version !== null &&
    "label" in item.version
  ) {
    versionLabel =
      (item.version as { id: string; label: string }).label?.trim() ?? "";
  } else if (typeof item.version === "string") {
    versionLabel = item.version.trim();
  }
  if (
    productLabel &&
    versionLabel &&
    productLabel.toLowerCase().includes(versionLabel.toLowerCase())
  ) {
    return productLabel;
  }
  const combined = [productLabel, versionLabel].filter(Boolean).join(" ");
  return combined.trim();
}

/**
 * Builds Product Version options from deployment products: one option per item,
 * label = "product.label version.label", value = item id.
 *
 * @param {DeploymentProductItem[]} allDeploymentProducts - Flat list of deployment products.
 * @returns {ProductVersionOption[]} Options with id and display label.
 */
export function getBaseProductOptions(
  allDeploymentProducts: DeploymentProductItem[],
): ProductVersionOption[] {
  return allDeploymentProducts
    .map((item) => {
      const label = getDeploymentProductDisplayLabel(item);
      if (!label || isUnknownPlaceholderProductLabel(label)) {
        return null;
      }
      return { id: item.id, label };
    })
    .filter((opt): opt is ProductVersionOption => opt !== null);
}

/**
 * Formats chat messages for the case classification API.
 *
 * @param {Array<{ text: string; sender: string }>} messages - Chat messages with text and sender.
 * @returns {string} Formatted string: "User: ...\nAssistant: ..."
 */
export function formatChatHistoryForClassification(
  messages: Array<{ text: string; sender: ChatSender }>,
): string {
  return messages
    .map((m) => {
      const text = (m.text || "").trim();
      if (!text) return "";
      const role = m.sender === ChatSender.USER ? "User" : "Assistant";
      return `${role}: ${text}`;
    })
    .filter((line) => line.length > 0)
    .join("\n");
}

/**
 * Builds envProducts for classification from deployment products map.
 * Each value is "product.label version.label" (e.g. "WSO2 API Manager 1.8.0") so the classification API receives product and version.
 *
 * @param {Record<string, DeploymentProductItem[]>} productsByDeploymentId - Products keyed by deployment id.
 * @param {ProjectDeploymentOption[]} projectDeployments - Deployments with id and name.
 * @returns {Record<string, string[]>} envProducts: { [deploymentName]: [productVersionLabel, ...] }
 */
export function buildEnvProducts(
  productsByDeploymentId: Record<string, DeploymentProductItem[]>,
  projectDeployments: ProjectDeploymentOption[] | undefined,
): Record<string, string[]> {
  if (!projectDeployments?.length) return {};

  const result: Record<string, string[]> = {};

  for (const dep of projectDeployments) {
    const products = productsByDeploymentId[dep.id] ?? [];
    const labels = Array.from(
      new Set(
        products
          .map((p) => getDeploymentProductDisplayLabel(p))
          .filter(
            (l): l is string =>
              Boolean(l) && !isUnknownPlaceholderProductLabel(l),
          ),
      ),
    );
    const key = dep.name ?? dep.type?.label ?? "";
    if (key) {
      result[key] = labels;
    }
  }

  return result;
}

/**
 * Builds the list of deployment names/labels from project deployments (for dropdown).
 *
 * @param {ProjectDeploymentOption[]} projectDeployments - Deployments from useGetProjectDeployments.
 * @returns {string[]} Deployment names or type labels.
 */
export function getBaseDeploymentOptions(
  projectDeployments: ProjectDeploymentOption[] | undefined,
): string[] {
  return (
    projectDeployments?.map((d) => d.name ?? d.type?.label).filter(Boolean) ??
    []
  );
}
