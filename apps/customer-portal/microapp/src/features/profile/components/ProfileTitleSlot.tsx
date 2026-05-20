import { Skeleton, Stack, Typography } from "@wso2/oxygen-ui";

import { useMe } from "@features/profile/hooks";
import { UserAvatar } from "@features/users/components";

export function ProfileTitleSlot() {
  const {
    data: { name },
  } = useMe();

  return (
    <Stack direction="row" alignItems="center" gap={1.5} mt={-2}>
      {name ? (
        <UserAvatar>{name}</UserAvatar>
      ) : (
        <Skeleton variant="circular" width={40} height={40} sx={{ flexShrink: 0 }} />
      )}
      <Typography variant="h6" fontWeight="medium" sx={{ flex: 1 }}>
        {name ?? <Skeleton variant="text" width="100%" height={30} />}
      </Typography>
    </Stack>
  );
}
