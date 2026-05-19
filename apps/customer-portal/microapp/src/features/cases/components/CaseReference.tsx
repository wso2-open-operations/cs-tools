import { Box, Grid } from "@wso2/oxygen-ui";
import DOMPurify from "dompurify";

import type { Case } from "@features/cases/types";
import { InfoField } from "@features/detail/components";

import { RichText, SectionCard } from "@shared/components/common";

export function CaseReference({ data }: { data: Case }) {
  return (
    <Box mb={2}>
      <SectionCard>
        <Grid spacing={1.5} container>
          <Grid size={12}>
            <InfoField label="Related Case ID" value={data.id} />
          </Grid>
          <Grid size={12}>
            <InfoField label="Title" value={data.title} />
          </Grid>
          <Grid size={12}>
            <InfoField
              label="Description"
              value={<RichText dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(data.description) }} />}
            />
          </Grid>
        </Grid>
      </SectionCard>
    </Box>
  );
}
