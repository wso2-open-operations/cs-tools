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
      month: "numeric",
      day: "numeric",
    }) ?? "—"
  );
}

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
