import { Badge, badgeClasses } from "@mui/material";
import { styled } from "@mui/system";

export const NotificationBadge = styled(Badge)`
  & .${badgeClasses.badge} {
    position: absolute;
    top: -8px;
    right: 12px;
  }
`;
