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

// Single source of truth for how a customer tier is labelled and coloured.
// Lives in its own module (not a component file) so the header chip-row, the
// meta band, and the Customer widget can all share it without tripping the
// react-refresh "components-only export" rule.
export const TIER_LABEL: Record<CustomerTier, string> = {
  basic: "Basic",
  enterprise: "Enterprise",
};

// Tier is metadata, not an action, so it must not wear the brand accent
// (`primary`/orange) — that fails WCAG contrast (white-on-#F87643 ~ 2.7:1) and
// would paint case headers orange. basic renders neutral; enterprise takes a
// contrast-safe semantic role (its contrastText is dark in this theme).
export const TIER_COLOR: Record<
  CustomerTier,
  "default" | "info" | "success" | "warning"
> = {
  basic: "default",
  enterprise: "info",
};
