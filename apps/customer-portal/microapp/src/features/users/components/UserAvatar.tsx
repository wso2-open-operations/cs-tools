import { getAvatarInitials } from "@root/src/shared/utils";
import { Avatar } from "@wso2/oxygen-ui";

export function UserAvatar({ size = "medium", children }: { size?: "medium" | "large"; children: string }) {
  return (
    <Avatar
      sx={(theme) => ({
        height: size === "large" ? 52 : 36,
        width: size === "large" ? 52 : 36,
        bgcolor: "primary.main",
        fontSize: size === "large" ? theme.typography.h3 : theme.typography.h5,
        fontWeight: "medium",
      })}
    >
      {getAvatarInitials(children)}
    </Avatar>
  );
}
