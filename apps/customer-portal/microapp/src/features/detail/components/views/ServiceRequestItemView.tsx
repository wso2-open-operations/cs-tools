import { Grid } from "@wso2/oxygen-ui";
import { User, Users } from "@wso2/oxygen-ui-icons-react";

import { CommentsList, InfoField, Layout } from "@features/detail/components";
import { useServiceRequest } from "@features/detail/hooks";

import { SectionCard } from "@shared/components/common";
import { StatusChip } from "@shared/components/support";

import { CASE_TYPES } from "@shared/constants";
import { useDateTime } from "@shared/hooks";
import { stripHtmlTags } from "@shared/utils";

export function ServiceRequestItemView() {
  const type = CASE_TYPES.SERVICE_REQUEST;

  const { format, fromNow } = useDateTime();
  const { data, isLoading } = useServiceRequest();

  return (
    <Layout
      type={type}
      title={data?.title}
      id={data?.internalId && data?.number ? `${data.internalId} | ${data.number}` : undefined}
    >
      <SectionCard title="Request Information">
        <Grid spacing={1.5} container>
          <Grid size={12}>
            <InfoField
              label="Description"
              value={data?.description && stripHtmlTags(data.description)}
              loading={isLoading}
            />
          </Grid>

          <Grid size={6}>
            <InfoField label="Requested By" value={data?.createdBy ?? "N/A"} icon={User} loading={isLoading} />
          </Grid>

          <Grid size={6}>
            <InfoField label="Status" value={<StatusChip type={type} id={data?.statusId} size="small" />} />
          </Grid>

          <Grid size={6}>
            <InfoField label="Assigned To" value={data?.assignee ?? "N/A"} icon={Users} loading={isLoading} />
          </Grid>

          <Grid size={6}>
            <InfoField label="Created" value={data?.createdOn && format(data.createdOn)} loading={isLoading} />
          </Grid>

          <Grid size={6}>
            <InfoField label="Last Updated" value={data?.updatedOn && fromNow(data.updatedOn)} loading={isLoading} />
          </Grid>
        </Grid>
      </SectionCard>

      <SectionCard title="Product & Environment">
        <Grid spacing={1.5} container>
          <Grid size={12}>
            <InfoField label="Product Name" value={data?.product || "N/A"} loading={isLoading} />
          </Grid>

          <Grid size={12}>
            <InfoField label="Deployment" value={data?.deployment || "N/A"} loading={isLoading} />
          </Grid>
        </Grid>
      </SectionCard>

      <SectionCard title="Updates">
        <CommentsList />
      </SectionCard>
    </Layout>
  );
}
