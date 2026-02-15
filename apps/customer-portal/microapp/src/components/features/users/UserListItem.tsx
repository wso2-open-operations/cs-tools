import { Link } from "react-router-dom";
import { Card, Stack, Avatar as MuiAvatar, Typography, Chip, useTheme, pxToRem } from "@wso2/oxygen-ui";
import { ShieldUser, ChevronRight, Mail } from "@wso2/oxygen-ui-icons-react";
import { capitalize, stringAvatar } from "@utils/others";
import type { RoleName } from "./RoleSelector";

export interface UserListItemProps {
  name: string;
  email: string;
  role: RoleName;
  lastActive: string;
}

export function UserListItem({ name, email, role, lastActive }: UserListItemProps) {
  const theme = useTheme();
  const admin = role === "admin";

  return (
    <Card
      component={Link}
      elevation={0}
      to="/users/edit"
      state={{ name, email, role: capitalize(role) }}
      sx={{ textDecoration: "none", p: 1 }}
    >
      <Stack direction="row" alignItems="center" justifyContent="space-between" gap={2}>
        <Stack direction="row" alignItems="center" gap={2}>
          <Avatar>{name}</Avatar>
          <Stack>
            <Stack direction="row" gap={1} alignItems="center">
              <Typography variant="subtitle1" fontWeight="medium">
                {name}
              </Typography>
              <Chip size="small" label={capitalize(role)} color={admin ? "primary" : "default"} />
            </Stack>
            <Stack direction="row" alignItems="center" gap={1}>
              <Mail color={theme.palette.text.secondary} size={pxToRem(13)} />
              <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                {email}
              </Typography>
            </Stack>
            <Typography variant="caption" fontWeight="regular" color="text.secondary" mt={0.5}>
              Last Active: {lastActive}
            </Typography>
          </Stack>
        </Stack>
        <Stack direction="row" alignItems="center" gap={2}>
          {admin && <ShieldUser color={theme.palette.primary.main} size={pxToRem(28)} />}

          <ChevronRight color={theme.palette.text.secondary} size={pxToRem(18)} />
        </Stack>
      </Stack>
    </Card>
  );
}

export function Avatar({ children }: { children: string }) {
  return (
    <MuiAvatar
      sx={(theme) => ({
        height: 40,
        width: 40,
        bgcolor: "primary.main",
        fontSize: theme.typography.h5,
        fontWeight: "medium",
      })}
    >
      {stringAvatar(children)}
    </MuiAvatar>
  );
}
