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

import { useQuery } from "@tanstack/react-query";
import { Chip, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { deployments } from "@src/services/deployments";
import { formatDateOnly } from "@utils/customers";
import { ErrorState } from "@components/support/ErrorState";

/** SN sizing fields arrive as strings; treat null/blank uniformly as "—" — mirrors the webapp's
 * DeployedProductsPanel sizingValue. */
function sizingValue(value: string | null): string {
  return value?.trim() ? value : "—";
}

// Read-only list of a deployment's deployed products — the mobile card-list equivalent of the
// webapp's DeployedProductsPanel table (Product / Version / Support EOL / Cores / TPS / Category),
// minus its write actions (add/edit/deactivate are deliberately out of scope for this pass).
export function DeployedProductsList({ deploymentId }: { deploymentId: string }) {
  const { data, isLoading, isError, refetch } = useQuery(deployments.productsList(deploymentId));

  if (isLoading) {
    return (
      <Stack gap={1}>
        {[0, 1].map((i) => (
          <Skeleton key={i} variant="rounded" height={56} />
        ))}
      </Stack>
    );
  }

  if (isError) {
    return <ErrorState onRetry={() => void refetch()} />;
  }

  const products = data ?? [];

  if (products.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        No products deployed in this deployment.
      </Typography>
    );
  }

  return (
    <Stack gap={1.5}>
      {products.map((p) => (
        <Stack key={p.id} gap={0.5} sx={{ p: 1.5, border: 1, borderColor: "divider", borderRadius: 1 }}>
          <Stack direction="row" alignItems="center" justifyContent="space-between" gap={1}>
            <Typography variant="body2" noWrap sx={{ minWidth: 0 }}>
              {p.productName}
            </Typography>
            {p.versionName && <Chip size="small" variant="outlined" label={p.versionName} />}
          </Stack>
          <Typography variant="caption" color="text.secondary">
            Support EOL {formatDateOnly(p.supportEoLDate)}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            Cores {sizingValue(p.cores)} · TPS {sizingValue(p.tps)} · {p.category ?? "—"}
          </Typography>
        </Stack>
      ))}
    </Stack>
  );
}
