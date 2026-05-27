import { Card, pxToRem, Skeleton, Stack } from "@wso2/oxygen-ui";

export function ItemCardSkeleton() {
  return (
    <Card sx={{ p: 1 }}>
      <Stack gap={0.8}>
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Stack direction="row" alignItems="center" gap={1}>
            <Skeleton variant="circular" width={pxToRem(18)} height={pxToRem(18)} />
            <Skeleton variant="text" width={60} height={20} />
            <Skeleton variant="rounded" width={50} height={24} sx={{ borderRadius: 1 }} />
          </Stack>
          <Skeleton variant="circular" width={pxToRem(18)} height={pxToRem(18)} />
        </Stack>

        <Skeleton variant="text" width="90%" height={28} />

        <Stack direction="row" alignItems="center" gap={1}>
          <Skeleton variant="rounded" width={70} height={24} sx={{ borderRadius: 1 }} />
          <Skeleton variant="circular" width={4} height={4} />
          <Skeleton variant="text" width={80} height={20} />
        </Stack>

        <Stack gap={0.5} mt={1}>
          <Stack direction="row" alignItems="center" gap={1}>
            <Skeleton variant="circular" width={pxToRem(16)} height={pxToRem(16)} />
            <Skeleton variant="text" width={100} height={18} />
          </Stack>
        </Stack>
      </Stack>
    </Card>
  );
}
