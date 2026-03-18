import { useLayoutEffect } from "react";
import { Chip, Grid, Skeleton, Stack } from "@wso2/oxygen-ui";
import { InfoField, OverlineSlot, StakeholderItem, StakeholderItemSkeleton } from "@components/features/detail";
import { PriorityChip, StatusChip } from "@components/features/support";
import { User, Users } from "@wso2/oxygen-ui-icons-react";
import { SectionCard } from "@components/shared";
import { useLayout } from "@context/layout";
import { useQuery } from "@tanstack/react-query";
import { changeRequests } from "@src/services/changes";
import { useParams } from "react-router-dom";
import { stripHtmlTags } from "@utils/others";

export default function ChangeDetailPage() {
  const layout = useLayout();
  const { id } = useParams();
  const { data, isLoading } = useQuery(changeRequests.get(id!));

  const AppBarSlot = () =>
    data ? (
      <Stack direction="row" gap={1.5} mt={1}>
        <StatusChip id={data.statusId} size="small" />
        <PriorityChip prefix="Impact" id={data.impactId} size="small" />
        <Chip label={data.requestType ?? "N/A"} size="small" />
      </Stack>
    ) : (
      <Stack direction="row" gap={1.5} mt={1}>
        <Skeleton variant="rounded" width={70} height={24} sx={{ borderRadius: "16px" }} />
        <Skeleton variant="rounded" width={70} height={24} sx={{ borderRadius: "16px" }} />
        <Skeleton variant="rounded" width={70} height={24} sx={{ borderRadius: "16px" }} />
      </Stack>
    );

  useLayoutEffect(() => {
    layout.setTitleOverride(data?.title ?? <Skeleton variant="text" width="100%" height={35} />);
    layout.setOverlineSlotOverride(<OverlineSlot type="change" id={data?.number} />);
    layout.setAppBarSlotsOverride(<AppBarSlot />);

    return () => {
      layout.setTitleOverride(undefined);
      layout.setOverlineSlotOverride(undefined);
      layout.setAppBarSlotsOverride(undefined);
    };
  }, [data]);

  return (
    <>
      <Stack gap={2} mb={10}>
        <SectionCard title="Change Information">
          <Grid spacing={1.5} container>
            <Grid size={12}>
              <InfoField label="Description" value={stripHtmlTags(data?.description ?? "")} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Change Owner" value={data?.createdBy} icon={User} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Requested By" value={data?.assignedTeam} icon={Users} />
            </Grid>
            <Grid size={6}>
              <InfoField
                label="Impact"
                value={
                  data ? (
                    <PriorityChip size="small" id={data?.impactId} />
                  ) : (
                    <Skeleton variant="rounded" width={70} height={22} sx={{ borderRadius: "16px" }} />
                  )
                }
              />
            </Grid>
            <Grid size={6}>
              <InfoField label="Estimated Duration" value={!isLoading ? (data?.duration ?? "N/A") : undefined} />
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
              <InfoField label="Approved By" value={!isLoading ? (data?.approvedBy ?? "N/A") : undefined} />
            </Grid>
          </Grid>
        </SectionCard>
        <SectionCard title="Impact Assessment">
          <Grid spacing={1.5} container>
            <Grid size={12}>
              <InfoField
                label="Communication Plan"
                value={
                  !isLoading
                    ? data?.communicationPlan
                      ? stripHtmlTags(data.communicationPlan)
                      : "No communication plan available"
                    : undefined
                }
              />
            </Grid>
            <Grid size={12}>
              <InfoField
                label="Rollback Plan"
                value={
                  !isLoading
                    ? data?.rollbackPlan
                      ? stripHtmlTags(data.rollbackPlan)
                      : "No rollback plan available"
                    : undefined
                }
              />
            </Grid>
            <Grid size={12}>
              <InfoField
                label="Test Plan"
                value={
                  !isLoading ? (data?.testPlan ? stripHtmlTags(data.testPlan) : "No test plan available") : undefined
                }
              />
            </Grid>
          </Grid>
        </SectionCard>
        <SectionCard title="Stakeholders">
          <Stack gap={1.5}>
            {data ? (
              <>
                {data.createdBy && <StakeholderItem name={data.createdBy} role="owner" />}
                {data.approvedBy && <StakeholderItem name={data.approvedBy} role="approver" />}
                {data.assignedTeam && <StakeholderItem name={data.assignedTeam} role="requestor" />}
              </>
            ) : (
              <>
                {Array.from({ length: 3 }).map((_, index) => (
                  <StakeholderItemSkeleton key={index} />
                ))}
              </>
            )}
          </Stack>
        </SectionCard>
      </Stack>
    </>
  );
}
