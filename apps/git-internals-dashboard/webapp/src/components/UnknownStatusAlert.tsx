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

import { Alert, AlertTitle } from "@mui/material";
import type { UnknownStatus } from "@api/types";

/**
 * Warns when the most recent recompute tick found a board status absent
 * from taxonomy.statuses — it's either pausing or accruing the SLA clock
 * (config.unknownStatusPolicy) for every issue sitting on it, unnoticed
 * until this banner or a direct query of `unknown_statuses` surfaces it.
 * Not dismissible: it reflects live server state (refreshed with every
 * overview poll), not a one-time fetch failure, so a dismissal would just
 * reappear confusingly on the next refresh.
 */
export function UnknownStatusAlert({ statuses }: { statuses: UnknownStatus[] | undefined }) {
  if (!statuses || statuses.length === 0) return null;

  const names = statuses.map((s) => `"${s.status}"`).join(", ");
  const plural = statuses.length !== 1;
  return (
    <Alert severity="warning" sx={{ mb: 2 }}>
      <AlertTitle>{plural ? `${statuses.length} unrecognized board statuses` : "Unrecognized board status"}</AlertTitle>
      {names} {plural ? "are" : "is"} not in the SLA taxonomy — the SLA clock for {plural ? "their" : "its"} issues is
      being paused or accrued by a default policy until classified. Add {plural ? "them" : "it"} to
      config/sla-config.yaml's taxonomy.statuses.
    </Alert>
  );
}
