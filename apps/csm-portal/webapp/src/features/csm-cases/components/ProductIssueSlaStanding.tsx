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

import { Box, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import SemanticChip from "@components/SemanticChip";
import { formatAbsoluteForUser } from "@utils/dateTime";
import type { ProductIssueSla } from "@features/csm-cases/types/productWorkItems";
import {
  productSlaConsumedText,
  productSlaLabel,
  productSlaRole,
} from "@features/csm-cases/utils/productWorkItems";

interface ProductIssueSlaChipProps {
  sla: ProductIssueSla | null | undefined;
}

/** Compact SLA status chip: label and colour by state. */
export function ProductIssueSlaChip({ sla }: ProductIssueSlaChipProps): JSX.Element {
  return <SemanticChip role={productSlaRole(sla?.state)} label={productSlaLabel(sla?.state)} />;
}

interface ProductIssueSlaStandingProps extends ProductIssueSlaChipProps {
  /** Show the "consumed of budget" and "waiting on CS since" lines. */
  showDetails?: boolean;
}

/** SLA chip with, below it, consumed vs budget and (when paused) since when. A state
 * of NONE shows the chip only, never numbers. */
export function ProductIssueSlaStanding({
  sla,
  showDetails = true,
}: ProductIssueSlaStandingProps): JSX.Element {
  const consumed = showDetails ? productSlaConsumedText(sla) : null;
  const paused = showDetails && sla?.state === "PAUSED" && sla.pausedSince ? sla.pausedSince : null;
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, alignItems: "flex-start" }}>
      <ProductIssueSlaChip sla={sla} />
      {consumed && (
        <Typography variant="caption" color="text.secondary">
          {consumed}
        </Typography>
      )}
      {paused && (
        <Typography variant="caption" color="text.secondary">
          {`Waiting on CS since ${formatAbsoluteForUser(paused) ?? "—"}`}
        </Typography>
      )}
    </Box>
  );
}

export default ProductIssueSlaStanding;

/** Hint for an issue that sits in a board column outside the standard set. */
export function NonStandardColumnHint({
  item,
}: {
  item: { nonStandardColumn?: boolean; boardStatusRaw?: string | null };
}): JSX.Element | null {
  if (!item.nonStandardColumn) return null;
  return (
    <Typography variant="caption" color="warning.main">
      {`Non-standard board column: ${item.boardStatusRaw?.trim() || "unknown"}`}
    </Typography>
  );
}
