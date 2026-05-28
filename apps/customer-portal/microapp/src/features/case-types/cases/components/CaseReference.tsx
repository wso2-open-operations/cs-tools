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
import { Box, Grid } from "@wso2/oxygen-ui";
import DOMPurify from "dompurify";

import { useCreateCase } from "@features/case-types/cases/hooks";
import { InfoField } from "@features/detail/components";

import { RichText, SectionCard } from "@shared/components/common";

export function CaseReference() {
  const {
    state: { case: reference },
  } = useCreateCase();

  if (!reference) return;

  return (
    <Box mb={2}>
      <SectionCard>
        <Grid spacing={1.5} container>
          <Grid size={12}>
            <InfoField label="Related Case ID" value={reference?.id} />
          </Grid>
          <Grid size={12}>
            <InfoField label="Title" value={reference?.title} />
          </Grid>
          <Grid size={12}>
            <InfoField
              label="Description"
              value={<RichText dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(reference.description) }} />}
            />
          </Grid>
        </Grid>
      </SectionCard>
    </Box>
  );
}
