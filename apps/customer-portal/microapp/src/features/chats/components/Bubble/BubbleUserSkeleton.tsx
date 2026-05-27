import { Card, Skeleton, Stack } from "@wso2/oxygen-ui";

export function BubbleUserSkeleton() {
  return (
    <Stack direction="row" justifyContent="end" width="100%">
      <Card
        component={Stack}
        sx={{
          p: 1.5,
          ml: 10,
          wdith: "fit-content",
          bgcolor: "background.paper",
          borderStyle: "dashed",
          borderWidth: 1,
          borderColor: "divider",
        }}
      >
        <Stack gap={1}>
          <Skeleton variant="text" width={150} height={20} />
          <Skeleton variant="text" width={100} height={20} />
        </Stack>

        <Stack direction="row" justifyContent="end" mt={1}>
          <Skeleton variant="text" width={50} height={20} />
        </Stack>
      </Card>
    </Stack>
  );
}
