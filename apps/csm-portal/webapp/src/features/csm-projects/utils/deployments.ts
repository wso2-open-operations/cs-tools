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

import type { BeDeploymentType } from "@api/backend/types";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";

/**
 * Format a backend timestamp as a date for the deployment views. Uses the
 * shared timezone-aware formatter (honours the user's preferred timezone and
 * the backend's space-separated timestamp format) and falls back to "—" for
 * missing/unparseable values. Shared by the deployments table, the details
 * dialog, and the deployed-products panel so the format stays consistent.
 */
export function formatDeploymentDate(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      year: "numeric",
      month: "short",
      day: "numeric",
    }) ?? "—"
  );
}

/**
 * Every deployment type, in the order the pickers offer them (the deployment
 * create/edit dialogs, a standard change template's types).
 */
export const DEPLOYMENT_TYPES: readonly BeDeploymentType[] = [
  "primary_production",
  "staging",
  "qa",
  "stress",
  "uat",
  "development",
];

/** Human label for each deployment-type enum value returned by the backend. */
const DEPLOYMENT_TYPE_LABEL: Record<BeDeploymentType, string> = {
  primary_production: "Primary Production",
  staging: "Staging",
  qa: "QA",
  stress: "Stress",
  uat: "UAT",
  development: "Development",
};

/**
 * Display label for a deployment type. Falls back to the raw value (with
 * underscores spaced out) for any type the backend adds before this map does.
 */
export function deploymentTypeLabel(type?: BeDeploymentType | string): string {
  if (!type) return "—";
  return (
    DEPLOYMENT_TYPE_LABEL[type as BeDeploymentType] ?? type.replace(/_/g, " ")
  );
}

/** "Primary Production, Staging or UAT": several deployment types, for a sentence. */
export function deploymentTypesText(
  types: readonly string[] | null | undefined,
): string {
  const labels = (types ?? []).map((t) => deploymentTypeLabel(t));
  if (labels.length <= 1) return labels.join("");
  return `${labels.slice(0, -1).join(", ")} or ${labels[labels.length - 1]}`;
}

/**
 * Valid deployed-product category codes (`BeDeployedProduct.category`,
 * `BeDeployedProductCreatePayload`/`BeDeployedProductDetailUpdatePayload`'s
 * own `category` field) -- Postgres-only, same opaque vocabulary as
 * entity-service's `ProductCategories` search filter and the BE's
 * `deployed_product_category_enum`. No confirmed full-name expansion exists
 * for any of these codes anywhere in this codebase, so
 * {@link deployedProductCategoryLabel} below only upper-cases the raw code
 * rather than inventing one.
 */
export const DEPLOYED_PRODUCT_CATEGORY_OPTIONS = ["pdp", "ms", "ps", "cl", "pc"] as const;

/** Display label for a deployed-product category code. "—" when unset. */
export function deployedProductCategoryLabel(category?: string | null): string {
  return category ? category.toUpperCase() : "—";
}
