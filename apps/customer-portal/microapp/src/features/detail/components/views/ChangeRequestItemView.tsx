import { Chip, Grid, Skeleton } from "@wso2/oxygen-ui";
import { User, Users } from "@wso2/oxygen-ui-icons-react";

import { InfoField, Layout, ProgressTimeline, StakeholdersList } from "@features/detail/components";
import { useChangeRequest } from "@features/detail/hooks";

import { SectionCard } from "@shared/components/common";
import { PriorityChip, StatusChip } from "@shared/components/support";

import { CASE_TYPES } from "@shared/constants";
import { stripHtmlTags } from "@shared/utils";

export function ChangeRequestItemView() {
  const type = CASE_TYPES.CHANGE_REQUEST;

  const { data, isLoading } = useChangeRequest();

  return (
    <Layout
      type={type}
      title={data?.title}
      id={data?.internalId && data?.number ? `${data.internalId} | ${data.number}` : undefined}
    >
      <SectionCard title="Change Information">
        <Grid spacing={1.5} container>
          <Grid size={12}>
            <InfoField
              loading={isLoading}
              label="Description"
              value={data?.description ? stripHtmlTags(data.description) : "No Description"}
            />
          </Grid>
          <Grid size={6}>
            <InfoField label="Change Owner" value={data?.createdBy} icon={User} loading={isLoading} />
          </Grid>
          <Grid size={6}>
            <InfoField label="Requested By" value={data?.assignedTeam} icon={Users} loading={isLoading} />
          </Grid>
          <Grid size={6}>
            <InfoField label="Status" value={<StatusChip type={type} id={data?.statusId} size="small" />} />
          </Grid>
          <Grid size={6}>
            <InfoField label="Impact" value={<PriorityChip type={type} size="small" id={data?.impactId} />} />
          </Grid>
          <Grid size={6}>
            <InfoField
              label="Request Type"
              value={
                data?.statusId ? (
                  <Chip label={data.requestType ?? "N/A"} size="small" />
                ) : (
                  <Skeleton variant="text" width={50} height={30} />
                )
              }
            />
          </Grid>
          <Grid size={6}>
            <InfoField label="Estimated Duration" value={data?.duration ?? "N/A"} loading={isLoading} />
          </Grid>
          <Grid size={6}>
            <InfoField
              label="Approval Status"
              value={
                data ? (
                  <Chip
                    size="small"
                    color={data.hasCustomerApproved ? "success" : "default"}
                    label={data.hasCustomerApproved ? "Approved" : "Pending"}
                  />
                ) : (
                  <Skeleton variant="rounded" width={70} height={22} sx={{ borderRadius: "16px" }} />
                )
              }
            />
          </Grid>
          <Grid size={6}>
            <InfoField label="Approved By" value={data?.approvedBy ?? "N/A"} loading={isLoading} />
          </Grid>
        </Grid>
      </SectionCard>

      <SectionCard title="Impact Assessment">
        <Grid spacing={1.5} container>
          <Grid size={12}>
            <InfoField
              loading={isLoading}
              label="Communication Plan"
              value={
                data?.communicationPlan ? stripHtmlTags(data.communicationPlan) : "No communication plan available"
              }
            />
          </Grid>
          <Grid size={12}>
            <InfoField
              loading={isLoading}
              label="Rollback Plan"
              value={data?.rollbackPlan ? stripHtmlTags(data.rollbackPlan) : "No rollback plan available"}
            />
          </Grid>
          <Grid size={12}>
            <InfoField
              loading={isLoading}
              label="Test Plan"
              value={data?.testPlan ? stripHtmlTags(data.testPlan) : "No test plan available"}
            />
          </Grid>
        </Grid>
      </SectionCard>

      <SectionCard title="Progress Timeline">
        <ProgressTimeline />
      </SectionCard>

      <SectionCard title="Stakeholders">
        <StakeholdersList />
      </SectionCard>
    </Layout>
  );
}
