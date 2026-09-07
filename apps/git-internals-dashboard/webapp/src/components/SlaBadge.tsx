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

// Port of v3's src/components/SlaBadge.tsx.
import { Chip } from "@mui/material";
import { SLA_BADGE_SX, SLA_STATE_LABEL } from "@lib/sla";
import type { SlaState } from "@api/types";

export function SlaBadge({ state }: { state: SlaState | null | undefined }) {
  if (!state) return <span style={{ color: "var(--sla-fg3)" }}>—</span>;
  const sx = SLA_BADGE_SX[state];
  return (
    <Chip
      label={SLA_STATE_LABEL[state]}
      variant="outlined"
      size="small"
      sx={{ fontWeight: 500, borderColor: sx.borderColor, backgroundColor: sx.backgroundColor, color: sx.color }}
    />
  );
}
