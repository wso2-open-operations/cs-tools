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
import { CircularProgress, Grid } from "@wso2/oxygen-ui";
import DOMPurify from "dompurify";

import { InfoField, Layout } from "@features/detail/components";
import { useCase } from "@features/detail/hooks";

import { RichText, SectionCard } from "@shared/components/common";
import { StatusChip } from "@shared/components/support";

import { CASE_TYPES } from "@shared/constants";
import { useDateTime } from "@shared/hooks";

export function AnnouncementItemView() {
  const type = CASE_TYPES.ANNOUNCEMENT;

  const { format, fromNow } = useDateTime();
  const { data, isLoading } = useCase();

  return (
    <Layout type={type} title={data?.title} id={data?.number}>
      {isLoading ? (
        <CircularProgress size={20} />
      ) : (
        <RichText dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(data?.description ?? "") }} />
      )}

      <SectionCard>
        <Grid spacing={1.5} container>
          <Grid size={6}>
            <InfoField label="Created" value={data?.createdOn && format(data.createdOn)} loading={isLoading} />
          </Grid>

          <Grid size={6}>
            <InfoField label="Status" value={<StatusChip type={type} id={data?.statusId} size="small" />} />
          </Grid>

          <Grid size={6}>
            <InfoField label="Last Updated" value={data?.updatedOn && fromNow(data.updatedOn)} loading={isLoading} />
          </Grid>
        </Grid>
      </SectionCard>
    </Layout>
  );
}
