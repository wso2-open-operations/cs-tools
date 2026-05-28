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
