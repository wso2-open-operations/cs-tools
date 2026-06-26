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

import {
  Alert,
  Box,
  Chip,
  CircularProgress,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { type JSX } from "react";
import { useSearchDeployedProducts } from "@features/csm-projects/api/useSearchDeployedProducts";
import { formatDeploymentDate } from "@features/csm-projects/utils/deployments";

interface DeployedProductsPanelProps {
  deploymentId: string;
}

/** SN sizing fields arrive as strings; treat null/blank uniformly as "—". */
function sizingValue(value?: string | null): string {
  return value?.trim() ? value : "—";
}

/**
 * Read-only list of the products deployed in a single deployment
 * (`POST /deployments/{id}/products/search`). Rendered inside an expanded
 * deployment row. Mounting it issues the search, so it loads lazily when the
 * row is expanded.
 *
 * Adding, re-versioning, or removing deployed products is not offered: the
 * backend exposes no write endpoints for deployed products yet.
 */
export default function DeployedProductsPanel({
  deploymentId,
}: DeployedProductsPanelProps): JSX.Element {
  const { data, isLoading, isError, error } = useSearchDeployedProducts(deploymentId);
  const products = data ?? [];

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", py: 3 }}>
        <CircularProgress size={20} />
      </Box>
    );
  }

  if (isError) {
    return (
      <Alert severity="error" sx={{ my: 1 }}>
        Failed to load deployed products:{" "}
        {error instanceof Error ? error.message : "unknown error"}
      </Alert>
    );
  }

  if (products.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary" sx={{ py: 2, px: 1 }}>
        No products deployed in this deployment.
      </Typography>
    );
  }

  return (
    <Box sx={{ py: 1 }}>
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ textTransform: "uppercase", letterSpacing: 0.4, px: 1 }}
      >
        Deployed products
      </Typography>
      <Table size="small" sx={{ mt: 0.5 }}>
        <TableHead>
          <TableRow>
            <TableCell>Product</TableCell>
            <TableCell>Version</TableCell>
            <TableCell>Support EOL</TableCell>
            <TableCell align="right">Cores</TableCell>
            <TableCell align="right">TPS</TableCell>
            <TableCell>Category</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {products.map((p) => (
            <TableRow key={p.id}>
              <TableCell>{p.product?.name || p.product?.id || "—"}</TableCell>
              <TableCell>
                {p.version?.name ? (
                  <Chip size="small" variant="outlined" label={p.version.name} />
                ) : (
                  "—"
                )}
              </TableCell>
              <TableCell>{formatDeploymentDate(p.version?.supportEoLDate)}</TableCell>
              <TableCell align="right">{sizingValue(p.cores)}</TableCell>
              <TableCell align="right">{sizingValue(p.tps)}</TableCell>
              <TableCell>{p.category ?? "—"}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Box>
  );
}
