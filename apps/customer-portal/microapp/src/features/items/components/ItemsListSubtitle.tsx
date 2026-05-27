import { Skeleton, Typography } from "@wso2/oxygen-ui";

export function ItemsListSubtitle({ count, total }: { count?: number; total?: number }) {
  if (!count || !total) return <Skeleton variant="text" width={50} height={20} />;
  return (
    <Typography component="div" variant="subtitle2" fontWeight="regular" color="text.secondary">
      {count} of {total}
    </Typography>
  );
}
