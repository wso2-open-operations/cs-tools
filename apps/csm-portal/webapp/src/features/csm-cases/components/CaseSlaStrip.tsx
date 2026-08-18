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

import { alpha, Box, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { slaProgressColor } from "@features/csm-cases/utils/caseSlaMapping";
import type { CaseSla } from "@features/csm-cases/types/csmCases";

interface CaseSlaStripProps {
  slas: CaseSla[] | undefined;
}

/**
 * Compact, always-visible SLA summary shown on the case Overview band — one
 * line per SLA that still needs attention, with the same colour-fill
 * treatment as the full SLA tab (see {@link CaseSlaTable}), so a breach reads
 * pre-attentively without opening that tab. `Completed`/`Cancelled` SLAs are
 * dropped here on purpose: those no longer need attention, and the full
 * history stays available on the SLA tab.
 */
export default function CaseSlaStrip({ slas }: CaseSlaStripProps): JSX.Element | null {
  const active = (slas ?? []).filter(
    (s) => s.stage !== "completed" && s.stage !== "cancelled",
  );
  if (active.length === 0) return null;

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 0.5,
        px: 2,
        py: 1,
        borderTop: 1,
        borderColor: "divider",
      }}
    >
      {active.map((sla) => {
        const elapsedPct = Math.min(Math.max(sla.businessElapsedPercent, 0), 100);
        return (
          <Box
            key={sla.id}
            aria-label={`${sla.definition} ${Math.round(elapsedPct)}% elapsed`}
            sx={(theme) => ({
              display: "flex",
              alignItems: "center",
              gap: 1,
              px: 1,
              py: 0.5,
              borderRadius: 1,
              background: `linear-gradient(to right, ${alpha(
                theme.palette[slaProgressColor(sla)].main,
                0.14,
              )} ${elapsedPct}%, transparent ${elapsedPct}%)`,
            })}
          >
            <Typography variant="body2" noWrap sx={{ flexShrink: 0 }}>
              {sla.definition}
            </Typography>
            <Typography variant="body2" color="text.secondary" noWrap>
              {sla.stageLabel}
            </Typography>
            <Typography variant="body2" color="text.secondary" noWrap>
              {sla.businessTimeLeftLabel ? `${sla.businessTimeLeftLabel} left` : null}
            </Typography>
            <Typography
              variant="body2"
              sx={{ ml: "auto", fontWeight: 600, flexShrink: 0 }}
              color={`${slaProgressColor(sla)}.main`}
            >
              {`${Math.round(sla.businessElapsedPercent)}%`}
            </Typography>
          </Box>
        );
      })}
    </Box>
  );
}
