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

import { Box, Skeleton } from "@wso2/oxygen-ui";
import { useEffect, type JSX } from "react";
import { useNavigate, useParams } from "react-router";
import useGetMetadata from "@api/useGetMetadata";
import UsageAndMetricsTabContent from "@features/usage-metrics/components/UsageAndMetricsTabContent";

/**
 * Standalone Usage & Metrics page (sidebar navigation).
 * Rendered only when portal metadata enables usage metrics.
 *
 * @returns {JSX.Element | null} Usage & Metrics view, or null while redirecting when disabled.
 */
export default function UsageMetricsPage(): JSX.Element | null {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();
  const { isLoading } = useGetMetadata();
  const usageMetricsEnabled = true;

  useEffect(() => {
    if (isLoading) {
      return;
    }
    if (!usageMetricsEnabled) {
      const fallback =
        projectId != null && projectId !== ""
          ? `/projects/${projectId}/dashboard`
          : "/";
      navigate(fallback, { replace: true });
    }
  }, [isLoading, usageMetricsEnabled, navigate, projectId]);

  if (isLoading) {
    return (
      <Box sx={{ mt: 1, display: "flex", flexDirection: "column", gap: 2 }}>
        <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <Skeleton variant="rounded" width={280} height={32} />
        </Box>
        <Box sx={{ display: "flex", gap: 3, pb: 1.5 }}>
          <Skeleton variant="rounded" width={90} height={28} />
          <Skeleton variant="rounded" width={120} height={28} />
          <Skeleton variant="rounded" width={100} height={28} />
        </Box>
        <Skeleton variant="rounded" height="70vh" />
      </Box>
    );
  }

  if (!usageMetricsEnabled) {
    return null;
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <UsageAndMetricsTabContent />
    </Box>
  );
}
