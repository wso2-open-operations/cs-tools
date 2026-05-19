import { getAvatarInitials } from "@root/src/shared/utils";
import { Avatar } from "@wso2/oxygen-ui";

export function UserAvatar({ children }: { children: string }) {
  return (
    <Avatar
      sx={(theme) => ({
        height: 40,
        width: 40,
        bgcolor: "primary.main",
        fontSize: theme.typography.h5,
        fontWeight: "medium",
      })}
    >
      {getAvatarInitials(children)}
    </Avatar>
  );
}
