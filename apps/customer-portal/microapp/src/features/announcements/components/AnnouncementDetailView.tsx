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

import { useLayoutEffect } from "react";
import { Box, Grid, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { InfoField, OverlineSlot } from "@components/detail";
import { StatusChip } from "@components/support";
import { useLayout } from "@context/layout";
import DOMPurify from "dompurify";
import { RichText, SectionCard } from "@components/common";
import { useDateTime } from "@shared/hooks/useDateTime";
import { useOverlineVariant } from "@shared/hooks/useOverlineVariant";
import type { Case } from "@features/cases/types/case.model";

type AnnouncementDetailViewProps = {
  data: Case | undefined;
};

export function AnnouncementDetailView({ data }: AnnouncementDetailViewProps) {
  const layout = useLayout();
  const { fromNow, format } = useDateTime();
  const { ref, variant: overlineSlotVariant } = useOverlineVariant();

  useLayoutEffect(() => {
    layout.setLayoutOverrides({
      title: (
        <OverlineSlot
          variant={overlineSlotVariant}
          type="announcement"
          id={data?.number ? `${data.internalId} | ${data.number}` : undefined}
          title={data?.title}
        />
      ),
    });
    return () => {
      layout.setLayoutOverrides({ title: undefined });
    };
  }, [data, overlineSlotVariant]);

  return (
    <>
      <Stack gap={2} mb={10}>
        <Typography ref={ref} variant="h5" fontWeight="medium">
          {data?.title}
        </Typography>
        {data ? (
          <RichText dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(data.description) }} />
        ) : (
          <Box display="flex" flexDirection="column" gap={1}>
            <Skeleton variant="text" width="100%" />
            <Skeleton variant="text" width="100%" />
            <Skeleton variant="text" width="100%" />
            <Skeleton variant="text" width="100%" />
            <Skeleton variant="text" width="100%" />
          </Box>
        )}
        <SectionCard>
          <Grid spacing={1.5} container>
            <Grid size={6}>
              <InfoField label="Created" value={data?.createdOn ? format(data.createdOn) : undefined} />
            </Grid>
            <Grid size={6}>
              <InfoField
                label="Status"
                value={
                  data?.statusId ? (
                    <StatusChip type="service" id={data.statusId} size="small" />
                  ) : (
                    <Skeleton variant="text" width={50} height={30} />
                  )
                }
              />
            </Grid>
            <Grid size={6}>
              <InfoField label="Last Updated" value={data?.updatedOn ? fromNow(data.updatedOn) : undefined} />
            </Grid>
          </Grid>
        </SectionCard>
      </Stack>
    </>
  );
}
