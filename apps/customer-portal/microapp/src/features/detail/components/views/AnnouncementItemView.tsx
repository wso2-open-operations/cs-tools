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
