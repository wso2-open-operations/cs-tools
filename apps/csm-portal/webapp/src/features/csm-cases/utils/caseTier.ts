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

import type { CustomerTier } from "@features/csm-cases/types/csmCases";
import type { SemanticRole } from "@components/SemanticChip";

// Single source of truth for how a customer tier is labelled and coloured.
// Lives in its own module (not a component file) so the header chip-row, the
// meta band, and the Customer widget can all share it without tripping the
// react-refresh "components-only export" rule.
//
// Tier is free-form (PG `basic|enterprise` for native cases, raw ServiceNow
// support tiers like "Enterprise" for SN-sourced ones), so these tolerate any
// string and fall back instead of assuming a closed set.

/** Title-case a raw tier token, e.g. "managed_cloud" → "Managed Cloud". */
function titleCase(s: string): string {
  return s
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((w) => w[0].toUpperCase() + w.slice(1).toLowerCase())
    .join(" ");
}

/** Human label for a tier; "—" when unset, otherwise the title-cased value. */
export function tierLabel(tier: CustomerTier | undefined): string {
  const t = (tier ?? "").trim();
  return t ? titleCase(t) : "—";
}

// Tier is metadata, not an action, so it must not wear the brand accent
// (`primary`/orange) — that fails WCAG contrast (white-on-#F87643 ~ 2.7:1) and
// would paint case headers orange. Enterprise takes a contrast-safe semantic
// role; everything else (incl. unknown tiers) renders neutral.
export function tierColor(tier: CustomerTier | undefined): SemanticRole {
  return (tier ?? "").trim().toLowerCase() === "enterprise" ? "info" : "default";
}
