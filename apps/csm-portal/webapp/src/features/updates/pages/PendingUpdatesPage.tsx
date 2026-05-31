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

import { Box, IconButton, Paper, Stack, Typography, alpha, useTheme } from "@wso2/oxygen-ui";
import { ArrowLeft, CircleCheck, Clock } from "@wso2/oxygen-ui-icons-react";
import { useParams, useSearchParams, useNavigate } from "react-router";
import { useCallback, useMemo, type JSX } from "react";
import { usePostUpdateLevelsSearch } from "@features/updates/api/usePostUpdateLevelsSearch";
import { PendingUpdatesList } from "@features/updates/components/pending-updates/PendingUpdatesList";
import PendingUpdatesListSkeleton from "@features/updates/components/pending-updates/PendingUpdatesListSkeleton";
import { ROUTE_PREVIOUS_PAGE } from "@features/project-hub/constants/navigationConstants";

/**
 * PendingUpdatesPage displays pending update level descriptions for a product.
 * Reads productName, productBaseVersion, startingUpdateLevel and endingUpdateLevel
 * from URL search params and calls POST /updates/levels/search.
 *
 * @returns {JSX.Element} The rendered Pending Updates page.
 */
export default function PendingUpdatesPage(): JSX.Element {
  const navigate = useNavigate();
  const theme = useTheme();
  const { projectId } = useParams<{ projectId: string }>();
  const [searchParams] = useSearchParams();

  const productName = searchParams.get("productName") ?? "";
  const productBaseVersion = searchParams.get("productBaseVersion") ?? "";
  const mode = searchParams.get("mode");
  const startParam = searchParams.get("startingUpdateLevel");
  const endParam = searchParams.get("endingUpdateLevel");
  const startingUpdateLevel = Number(startParam ?? "0");
  const endingUpdateLevel = Number(endParam ?? "0");

  const searchRequest = useMemo(() => {
    if (
      !productName ||
      !productBaseVersion ||
      startParam === null ||
      endParam === null ||
      Number.isNaN(startingUpdateLevel) ||
      Number.isNaN(endingUpdateLevel)
    ) {
      return null;
    }
    return {
      productName,
      productVersion: productBaseVersion,
      startingUpdateLevel,
      endingUpdateLevel,
    };
  }, [
    productName,
    productBaseVersion,
    startParam,
    endParam,
    startingUpdateLevel,
    endingUpdateLevel,
  ]);

  const { data, isLoading, isError } = usePostUpdateLevelsSearch(searchRequest);

  const displayTitle = useMemo(() => {
    const name = productName || "Product";
    const ver = productBaseVersion || "";
    return ver ? `${name} ${ver}` : name;
  }, [productName, productBaseVersion]);

  const levelRange = useMemo(() => {
    if (!data) return null;
    const levels = Object.keys(data)
      .map(Number)
      .sort((a, b) => a - b);
    if (levels.length === 0) return null;
    return levels.length === 1
      ? `Update level ${levels[0]}`
      : `Update levels ${levels[0]} to ${levels[levels.length - 1]}`;
  }, [data]);

  const handleBack = () => {
    if (projectId) {
      navigate(`/projects/${projectId}/updates`);
    } else {
      navigate(ROUTE_PREVIOUS_PAGE);
    }
  };

  const handleView = useCallback(
    (levelKey: string) => {
      if (!projectId) return;
      const params = new URLSearchParams({
        productName,
        productBaseVersion,
        startingUpdateLevel: String(startingUpdateLevel),
        endingUpdateLevel: String(endingUpdateLevel),
      });
      navigate(
        `/projects/${projectId}/updates/pending/level/${levelKey}?${params}`,
      );
    },
    [
      navigate,
      projectId,
      productName,
      productBaseVersion,
      startingUpdateLevel,
      endingUpdateLevel,
    ],
  );

  if (!productName || !productBaseVersion) {
    return (
      <Box sx={{ py: 4 }}>
        <Stack direction="row" alignItems="center" gap={2} sx={{ mb: 2 }}>
          <IconButton onClick={handleBack} size="small" aria-label="Back">
            <ArrowLeft size={20} />
          </IconButton>
        </Stack>
        <Typography variant="body1" color="text.secondary">
          Missing product parameters. Use the Updates page to open pending
          updates.
        </Typography>
      </Box>
    );
  }

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        flex: 1,
        minHeight: 0,
        overflow: "hidden",
        width: "100%",
      }}
    >
      <Paper
        variant="outlined"
        sx={{
          p: 2,
          flexShrink: 0,
          zIndex: 10,
          borderRadius: 0,
        }}
      >
        <Stack direction="row" alignItems="flex-start" gap={2}>
          <IconButton onClick={handleBack} size="small" aria-label="Back" sx={{ mt: 0.5 }}>
            <ArrowLeft size={20} />
          </IconButton>
          {mode === "installed" ? (
            <>
              <Box
                sx={{
                  p: 1,
                  borderRadius: "50%",
                  bgcolor: alpha(theme.palette.success.light, 0.1),
                  color: theme.palette.success.light,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                }}
              >
                <CircleCheck size={24} />
              </Box>
              <Box>
                <Typography variant="h5" color="text.primary" fontWeight={600}>
                  Installed Updates
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {displayTitle}
                </Typography>
              </Box>
            </>
          ) : mode === "pending" ? (
            <>
              <Box
                sx={{
                  p: 1,
                  borderRadius: "50%",
                  bgcolor: alpha(theme.palette.warning.light, 0.1),
                  color: theme.palette.warning.light,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                }}
              >
                <Clock size={24} />
              </Box>
              <Box>
                <Typography variant="h5" color="text.primary" fontWeight={600}>
                  Pending Updates
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {displayTitle}
                </Typography>
              </Box>
            </>
          ) : (
            <Box>
              <Typography variant="h5" color="text.primary" fontWeight={600}>
                {displayTitle} — Pending Updates
              </Typography>
              {levelRange && (
                <Typography variant="body2" color="text.secondary">
                  {levelRange}
                </Typography>
              )}
            </Box>
          )}
        </Stack>
      </Paper>

      <Box
        sx={{
          flex: 1,
          minHeight: 0,
          overflow: "auto",
          p: 3,
          pt: 2,
        }}
      >
        {isLoading ? (
          <PendingUpdatesListSkeleton />
        ) : (
          <PendingUpdatesList
            data={data ?? null}
            isError={isError}
            onView={handleView}
          />
        )}
      </Box>
    </Box>
  );
}
