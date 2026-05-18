import { Grid } from "@wso2/oxygen-ui";
import { User, Users } from "@wso2/oxygen-ui-icons-react";

import { AttachmentsList, CommentsList, InfoField, Layout } from "@features/detail/components";
import { useActions, useCase } from "@features/detail/hooks";

import { SectionCard } from "@shared/components/common";
import { PriorityChip, StatusChip } from "@shared/components/support";

import { CASE_TYPES } from "@shared/constants";
import { useDateTime } from "@shared/hooks";

export function CaseDetailView() {
  const type = CASE_TYPES.DEFAULT;

  const { format } = useDateTime();
  const { data, isLoading } = useCase();
  const actions = useActions();

  return (
    <Layout type={type} title={data?.title} id={data?.id} actions={actions}>
      <SectionCard title="Case Information">
        <Grid spacing={1.5} container>
          <Grid size={6}>
            <InfoField label="Status" value={<StatusChip type={type} id={data?.statusId} size="small" />} />
          </Grid>

          <Grid size={6}>
            <InfoField label="Priority" value={<PriorityChip id={data?.severityId} size="small" />} />
          </Grid>

          <Grid size={6}>
            <InfoField label="Assignee" value={data?.assigned || "N/A"} icon={Users} loading={isLoading} />
          </Grid>

          <Grid size={6}>
            <InfoField label="Reporter" value={data?.reporter || "N/A"} icon={User} loading={isLoading} />
          </Grid>

          <Grid size={6}>
            <InfoField label="Category" value={data?.issueType || "N/A"} loading={isLoading} />
          </Grid>

          <Grid size={6}>
            <InfoField label="Last Updated" value={data?.updatedOn && format(data.updatedOn)} loading={isLoading} />
          </Grid>

          <Grid size={6}>
            <InfoField label="Created" value={data?.createdOn && format(data.createdOn)} loading={isLoading} />
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

      <SectionCard title="Attachments">
        <AttachmentsList />
      </SectionCard>

      <SectionCard title="Activity Timeline">
        <CommentsList />
      </SectionCard>
    </Layout>
  );
}
