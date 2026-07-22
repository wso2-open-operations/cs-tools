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

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Card, Chip, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { ChevronRight } from "@wso2/oxygen-ui-icons-react";
import { deployments } from "@src/services/deployments";
import type { Deployment } from "@src/types";
import { formatDateOnly, formatEnumLabel } from "@utils/customers";
import { ErrorState } from "@components/support/ErrorState";
import { DeploymentDetailDialog } from "@components/customers/DeploymentDetailDialog";

// A project's deployments — the mobile card-list equivalent of the webapp's DeploymentsTab table
// (Name / Type / Description / Created / Updated), minus its write actions/create button
// (deliberately out of scope for this pass). Tapping a card opens its detail + deployed products,
// same role the webapp's row-click / "View details" menu item plays.
export function DeploymentsTab({ projectId }: { projectId: string }) {
  const { data, isLoading, isError, refetch } = useQuery(deployments.list(projectId));
  const [viewing, setViewing] = useState<Deployment | null>(null);

  if (isLoading) {
    return (
      <Stack gap={1.5}>
        {[0, 1].map((i) => (
          <Skeleton key={i} variant="rounded" height={72} />
        ))}
      </Stack>
    );
  }

  if (isError) {
    return <ErrorState onRetry={() => void refetch()} />;
  }

  const items = data ?? [];

  if (items.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        No deployments for this project.
      </Typography>
    );
  }

  return (
    <Stack gap={1.5}>
      {items.map((d) => (
        <Card key={d.id} variant="outlined" sx={{ p: 2, cursor: "pointer" }} onClick={() => setViewing(d)}>
          <Stack direction="row" alignItems="center" justifyContent="space-between" gap={1}>
            <Stack gap={0.5} sx={{ minWidth: 0 }}>
              <Stack direction="row" alignItems="center" gap={1} flexWrap="wrap">
                <Typography variant="body2" noWrap>
                  {d.name || "—"}
                </Typography>
                <Chip size="small" variant="outlined" label={formatEnumLabel(d.type)} />
              </Stack>
              {d.description && (
                <Typography variant="caption" color="text.secondary" noWrap>
                  {d.description}
                </Typography>
              )}
              <Typography variant="caption" color="text.secondary">
                Updated {formatDateOnly(d.updatedOn)}
              </Typography>
            </Stack>
            <ChevronRight size={18} />
          </Stack>
        </Card>
      ))}

      <DeploymentDetailDialog deployment={viewing} onClose={() => setViewing(null)} />
    </Stack>
  );
}
