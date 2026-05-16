import { Card, pxToRem, Skeleton, Stack } from "@wso2/oxygen-ui";

export function BubbleAgentSkeleton() {
  return (
    <Stack direction="row" justifyContent="start" width="100%">
      <Card
        component={Stack}
        sx={{
          p: 1.5,
          width: "100%",
          bgcolor: "background.paper",
          borderStyle: "dashed",
          borderWidth: 1,
          borderColor: "divider",
        }}
      >
        <Stack direction="row" justifyContent="start" gap={1} mb={1.5}>
          <Skeleton variant="circular" width={pxToRem(18)} height={pxToRem(18)} />
          <Skeleton variant="text" width={60} height={20} />
        </Stack>

        <Stack gap={1}>
          <Skeleton variant="text" width="90%" height={20} />
          <Skeleton variant="text" width="75%" height={20} />
        </Stack>
      </Card>
    </Stack>
  );
}
