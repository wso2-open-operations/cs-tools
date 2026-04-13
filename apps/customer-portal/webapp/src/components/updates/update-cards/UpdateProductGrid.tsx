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

import { Box, Grid, Typography } from "@wso2/oxygen-ui";
import { useNavigate } from "react-router";
import type { JSX } from "react";
import Error500Page from "@components/common/error/Error500Page";
import EmptyState from "@components/common/empty-state/EmptyState";
import type { RecommendedUpdateLevelItem } from "@/types/updates";
import { UpdateProductCard } from "@update-cards/UpdateProductCard";
import { UpdateProductCardSkeleton } from "@update-cards/UpdateProductCardSkeleton";

export interface UpdateProductGridProps {
  data: RecommendedUpdateLevelItem[] | undefined;
  isLoading: boolean;
  isError: boolean;
  projectId?: string;
}

/**
 * Grid component to display a list of product update status cards.
 *
 * @param {UpdateProductGridProps} props - Component props.
 * @returns {JSX.Element} The rendered component.
 */
export function UpdateProductGrid({
  data,
  isLoading,
  isError,
  projectId,
}: UpdateProductGridProps): JSX.Element {
  const navigate = useNavigate();

  if (isError) {
    return (
      <Box
        sx={{
          mt: 4,
          display: "flex",
          justifyContent: "center",
          alignItems: "center",
          flexDirection: "column",
          py: 5,
        }}
      >
        <Error500Page style={{ width: 200, height: "auto" }} />
      </Box>
    );
  }

  const renderSkeletons = () => (
    <Grid container spacing={2}>
      {[1, 2, 3, 4].map((i) => (
        <Grid key={i} size={{ xs: 12, md: 6 }}>
          <UpdateProductCardSkeleton />
        </Grid>
      ))}
    </Grid>
  );

  return (
    <Box sx={{ mt: 4 }}>
      <Typography variant="h6" sx={{ mb: 2 }}>
        Product Update Status
      </Typography>
      {isLoading || !data ? (
        renderSkeletons()
      ) : data.length === 0 ? (
        <EmptyState description="No product updates found." />
      ) : (
        <Grid container spacing={2}>
          {data.map((item) => (
            <Grid
              key={`${item.productName}-${item.productBaseVersion}`}
              size={{ xs: 12, md: 6 }}
            >
              <UpdateProductCard
                item={item}
                onViewPendingUpdates={
                  projectId
                    ? () => {
                        const params = new URLSearchParams({
                          productName: item.productName,
                          productBaseVersion: item.productBaseVersion,
                          startingUpdateLevel: String(item.startingUpdateLevel),
                          endingUpdateLevel: String(item.recommendedUpdateLevel),
                        });
                        navigate(`/projects/${projectId}/updates/pending?${params}`);
                      }
                    : undefined
                }
              />
            </Grid>
          ))}
        </Grid>
      )}
    </Box>
  );
}
