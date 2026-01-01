import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { AppBar as MuiAppBar, ButtonBase as Button, Chip, IconButton, Stack, Typography } from "@mui/material";
import { ArrowBack, Cloud, ExpandMore, Folder, NotificationsOutlined, ThumbUpAlt } from "@mui/icons-material";

import { NotificationBadge } from "@components/ui";
import { ProjectSelector } from "@components/features/projects";
import { useLayout } from "@src/context/layout";

import { APP_BAR_CONFIG } from "@root/src/components/layout/config";

export function AppBar() {
  const navigate = useNavigate();
  const { title, appBarVariant, overlineSlot, subtitleSlot, startSlot, endSlot, appBarSlots, hasBackAction } =
    useLayout();
  const config = APP_BAR_CONFIG[appBarVariant];

  const [projectSelectorAnchor, setProjectSelectorAnchor] = useState<HTMLButtonElement | null>(null);
  const isProjectSelectorOpen = Boolean(projectSelectorAnchor);

  const navigateBack = () => navigate(-1);

  const openProjectSelector = (event: React.MouseEvent<HTMLButtonElement>) => {
    event.preventDefault();
    setProjectSelectorAnchor(event.currentTarget);
  };

  const closeProjectSelector = () => {
    setProjectSelectorAnchor(null);
  };

  return (
    <>
      <MuiAppBar
        position="relative"
        color="transparent"
        elevation={0}
        sx={{ backgroundColor: "background.paper", display: "flex", flexDirection: "col", gap: 1, p: 1.5, pt: 3 }}
      >
        {config.showNotifications && <NotificationButton to="/notifications" />}

        <Stack direction="row" justifyContent="space-between" alignItems="center" gap={1}>
          <Stack direction="row" alignItems="center" gap={1.5}>
            {hasBackAction && <BackButton onClick={navigateBack} />}
            {startSlot}
            <Stack>
              <Typography component="div" variant="body2" fontWeight="regular" color="text.secondary">
                {overlineSlot}
              </Typography>
              <Typography variant="h6" fontWeight="medium">
                {title}
              </Typography>
              <Typography component="div" variant="body2" fontWeight="regular" color="text.secondary">
                {subtitleSlot}
              </Typography>
            </Stack>
          </Stack>

          {endSlot}
        </Stack>

        {config.showProjectSelector && (
          <Button sx={{ justifyContent: "space-between", p: 0 }} onClick={openProjectSelector} disableRipple>
            <Stack direction="row" gap={1}>
              <Folder sx={{ color: "text.secondary" }} />
              <Typography variant="body2" color="text.secondary">
                Dreamsworks Inc
              </Typography>
            </Stack>
            <ExpandMore sx={{ color: "text.tertiary" }} />
          </Button>
        )}
        {config.showChips && (
          <Stack direction="row" gap={2} mt={1.5}>
            <Chip label="Managed Cloud" size="small" icon={<Cloud />} iconPosition="end" sx={{ borderRadius: 1 }} />
            <Chip label="All Good" size="small" color="success" iconPosition="end" icon={<ThumbUpAlt />} />
          </Stack>
        )}

        {/* Additional AppBar Content */}
        {appBarSlots}
      </MuiAppBar>

      {/* Popovers */}
      <ProjectSelector anchorEl={projectSelectorAnchor} open={isProjectSelectorOpen} onClose={closeProjectSelector} />
    </>
  );
}

function NotificationButton({ to }: { to: string }) {
  return (
    <IconButton
      component={Link}
      to={to}
      sx={{
        position: "absolute",
        right: 4,
        top: 10,
        p: 0,
      }}
    >
      <NotificationsOutlined
        sx={(theme) => ({
          fontSize: theme.typography.pxToRem(26),
          color: "text.secondary",
        })}
      />
      <NotificationBadge badgeContent={1} color="primary" overlap="circular" />
    </IconButton>
  );
}

function BackButton({ onClick }: { onClick: () => void }) {
  return (
    <IconButton onClick={onClick} sx={{ p: 0 }} disableRipple>
      <ArrowBack
        sx={(theme) => ({
          fontSize: theme.typography.pxToRem(26),
        })}
      />
    </IconButton>
  );
}
