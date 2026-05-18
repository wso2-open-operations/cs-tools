import { Stack, Typography } from "@wso2/oxygen-ui";

import { StakeholderItem, StakeholderItemSkeleton } from "@features/detail/components";
import { useChangeRequest } from "@features/detail/hooks";

export function StakeholdersList() {
  const { data, isLoading } = useChangeRequest();

  if (isLoading) return <StakeholdersListSkeleton />;

  if (!data) {
    return (
      <Typography variant="body2" color="text.secondary">
        No stakeholders.
      </Typography>
    );
  }

  return (
    <Stack gap={1.5}>
      {data.createdBy && <StakeholderItem name={data.createdBy} role="owner" />}
      {data.approvedBy && <StakeholderItem name={data.approvedBy} role="approver" />}
      {data.assignedTeam && <StakeholderItem name={data.assignedTeam} role="requestor" />}
    </Stack>
  );
}

function StakeholdersListSkeleton() {
  return (
    <Stack gap={1.5}>
      {Array.from({ length: 3 }).map((_, index) => (
        <StakeholderItemSkeleton key={index} />
      ))}
    </Stack>
  );
}
