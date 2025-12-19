import React, { useState } from "react";
import { Box, Button, IconButton, Typography } from "@mui/material";
import { ChevronLeft, ChevronRight } from "@mui/icons-material";
import { useRouter } from "../../hooks/useRouter";

import {
  SettingsIcon,
  DashboardIcon,
  SupportIcon,
  ProjectDetailsIcon,
  UpdatesIcon,
  SecurityCenterIcon,
  EngagementsIcon,
  LegalContractsIcon,
  CommunityIcon,
  AnnouncementsIcon,
  Wso2Logo,
} from "../../assets/icons/side-nav-bar-icons";
import type { SidebarProps } from ".";
import { FolderOpenIcon } from "../../assets/icons/common/folder-open-icon";
import { CrownIcon } from "../../assets/icons/common/crown-icon";

const Sidebar: React.FC<SidebarProps> = ({
  projectName,
  projectKey,
  projectId,
}) => {
  const router = useRouter();
  const [activeItem, setActiveItem] = useState("Dashboard");
  const [isCollapsed, setIsCollapsed] = useState(false);

  const menuItems = [
    {
      label: "Dashboard",
      icon: <DashboardIcon width={20} height={20} />,
      active: true,
    },
    { label: "Support", icon: <SupportIcon width={20} height={20} /> },
    {
      label: "Project Details",
      icon: <ProjectDetailsIcon width={20} height={20} />,
    },
    { label: "Updates", icon: <UpdatesIcon width={20} height={20} /> },
    {
      label: "Security center",
      icon: <SecurityCenterIcon width={20} height={20} />,
      badge: 4,
    },
    { label: "Engagements", icon: <EngagementsIcon width={20} height={20} /> },
    {
      label: "Legal contracts",
      icon: <LegalContractsIcon width={20} height={20} />,
    },
    { label: "Community", icon: <CommunityIcon width={20} height={20} /> },
    {
      label: "Announcements",
      icon: <AnnouncementsIcon width={20} height={20} />,
      badge: 3,
    },
  ];

  return (
    <Box
      component="aside"
      sx={{
        width: isCollapsed ? "80px" : "256px",
        height: "100vh",
        maxHeight: "100vh",
        bgcolor: "background.paper",
        borderRight: (theme) => `1px solid ${theme.palette.grey[200]}`,
        display: "flex",
        flexDirection: "column",
        flexShrink: 0,
        transition: "all 0.3s",
        overflow: "hidden",
      }}
    >
      {/* Header */}
      <Box
        sx={{
          p: 3,
          borderBottom: (theme) => `1px solid ${theme.palette.grey[200]}`,
          display: "flex",
          alignItems: "center",
          justifyContent: isCollapsed ? "center" : "space-between",
        }}
      >
        {!isCollapsed && (
          <Box display="flex" alignItems="center" gap={1}>
            <Wso2Logo width={80} height="auto" />
          </Box>
        )}

        <IconButton
          size="small"
          onClick={() => setIsCollapsed(!isCollapsed)}
          sx={{
            width: 32,
            height: 32,
            "&:hover": { bgcolor: (theme) => theme.palette.grey[100] },
          }}
        >
          {isCollapsed ? (
            <ChevronRight fontSize="small" />
          ) : (
            <ChevronLeft fontSize="small" />
          )}
        </IconButton>
      </Box>

      {/* Navigation */}
      <Box
        sx={{
          flex: 1,
          minHeight: 0,
          p: 2,
          overflowY: "auto",
          display: "flex",
          flexDirection: "column",
          gap: 0.5,
          "&::-webkit-scrollbar": {
            width: "6px",
          },
          "&::-webkit-scrollbar-track": {
            bgcolor: "transparent",
          },
          "&::-webkit-scrollbar-thumb": {
            bgcolor: "grey.300",
            borderRadius: "3px",
            "&:hover": {
              bgcolor: "grey.400",
            },
          },
        }}
      >
        {menuItems.map((item) => {
          const routeMap: Record<string, string> = {
            Dashboard: `/${projectId}/dashboard`,
            Support: `/${projectId}/support`,
            "Project Details": `/${projectId}/project-details`,
            Updates: `/${projectId}/updates`,
            "Security center": `/${projectId}/security-center`,
            Engagements: `/${projectId}/engagements`,
            "Legal contracts": `/${projectId}/legal-contracts`,
            Community: `/${projectId}/community`,
            Announcements: `/${projectId}/announcements`,
          };

          const handleClick = () => {
            setActiveItem(item.label);
            const route = routeMap[item.label];
            if (route) {
              router.push(route);
            }
          };

          return (
            <Button
              key={item.label}
              fullWidth
              onClick={handleClick}
              sx={{
                display: "flex",
                alignItems: "center",
                gap: isCollapsed ? 0 : 1.5,
                px: isCollapsed ? 0 : 1.5,
                py: "10px",
                borderRadius: "12px",
                justifyContent: isCollapsed ? "center" : "flex-start",
                textTransform: "none",
                position: "relative",
                overflow: isCollapsed ? "visible" : "hidden",
                transition: "background-color 0.2s, color 0.2s",
                minHeight: 0,
                minWidth: isCollapsed ? "44px" : "auto",
                bgcolor: activeItem === item.label ? "grey.900" : "transparent",
                color: activeItem === item.label ? "white" : "grey.700",
                "&:hover": {
                  bgcolor: activeItem === item.label ? "grey.900" : "grey.100",
                },
              }}
            >
              <Box
                sx={{
                  width: "20px",
                  height: "20px",
                  flexShrink: 0,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  "& svg": {
                    width: "20px",
                    height: "20px",
                  },
                }}
              >
                {item.icon}
              </Box>
              {!isCollapsed && (
                <Box
                  component="span"
                  sx={{
                    flex: 1,
                    textAlign: "left",
                    fontSize: "0.875rem",
                    whiteSpace: "nowrap",
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    minWidth: 0,
                  }}
                >
                  {item.label}
                </Box>
              )}
              {item.badge && !isCollapsed && (
                <Box
                  component="span"
                  sx={{
                    height: "20px",
                    minWidth: "20px",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    px: 0.5,
                    fontSize: "0.75rem",
                    fontWeight: 500,
                    bgcolor: "error.main",
                    color: "white",
                    borderRadius: "6px",
                    flexShrink: 0,
                  }}
                >
                  {item.badge}
                </Box>
              )}
              {item.badge && isCollapsed && (
                <Box
                  component="span"
                  sx={{
                    position: "absolute",
                    top: -4,
                    right: -4,
                    height: "16px",
                    minWidth: "16px",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    px: 0.5,
                    fontSize: "0.625rem",
                    fontWeight: 500,
                    bgcolor: "error.main",
                    color: "white",
                    borderRadius: "10px",
                    flexShrink: 0,
                    boxShadow: "0 2px 4px rgba(0,0,0,0.1)",
                  }}
                >
                  {item.badge}
                </Box>
              )}
            </Button>
          );
        })}
      </Box>

      {/* Current Project */}
      {!isCollapsed && (
        <Box
          sx={{
            px: 2,
            py: 1.5,
            borderTop: (theme) => `1px solid ${theme.palette.grey[200]}`,
            borderBottom: (theme) => `1px solid ${theme.palette.grey[200]}`,
            flexShrink: 0,
          }}
        >
          <Box display="flex" alignItems="center" gap={1} mb={0.5}>
            <FolderOpenIcon width="12px" height="12px" color="grey.500" />
            <Typography
              fontSize="0.75rem"
              color="grey.500"
              sx={{ letterSpacing: "0.05em" }}
            >
              CURRENT PROJECT
            </Typography>
          </Box>
          <Typography
            fontSize="0.875rem"
            color="grey.900"
            sx={{
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
            }}
            title={projectName}
          >
            {projectName}
          </Typography>
          <Typography fontSize="0.75rem" color="grey.500" mt={0.25}>
            {projectKey}
          </Typography>
        </Box>
      )}

      {/* Subscription Card */}
      {!isCollapsed && (
        <Box sx={{ p: 2, flexShrink: 0 }}>
          <Box
            sx={{
              background: "linear-gradient(to bottom right, #f97316, #ea580c)",
              borderRadius: "12px",
              p: 2.5,
              color: "white",
            }}
          >
            <Box display="flex" alignItems="center" gap={1} mb={1.5}>
              <CrownIcon width="20px" height="20px" />
              <Typography fontSize="1rem" fontWeight={600}>
                Subscription
              </Typography>
            </Box>
            <Typography
              fontSize="0.875rem"
              sx={{ color: "rgba(254, 215, 170, 1)", mb: 2 }}
            >
              Information about subscription
            </Typography>
            <Button
              fullWidth
              variant="outlined"
              size="small"
              sx={{
                bgcolor: "rgba(255, 255, 255, 0.2)",
                borderColor: "rgba(255, 255, 255, 0.3)",
                color: "white",
                textTransform: "none",
                "&:hover": {
                  bgcolor: "rgba(255, 255, 255, 0.3)",
                  borderColor: "rgba(255, 255, 255, 0.3)",
                },
              }}
            >
              Details
            </Button>
          </Box>
        </Box>
      )}

      {/* Settings Button */}
      <Box sx={{ p: 2, pt: 0, flexShrink: 0 }}>
        <Button
          fullWidth
          onClick={() => {
            setActiveItem("Settings");
            router.push(`/${projectId}/settings`);
          }}
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1.5,
            px: isCollapsed ? 0 : 1.5,
            py: "10px",
            borderRadius: "12px",
            justifyContent: isCollapsed ? "center" : "flex-start",
            textTransform: "none",
            position: "relative",
            overflow: isCollapsed ? "visible" : "hidden",
            transition: "background-color 0.2s, color 0.2s",
            minHeight: 0,
            minWidth: isCollapsed ? "44px" : "auto",
            bgcolor: activeItem === "Settings" ? "grey.900" : "transparent",
            color: activeItem === "Settings" ? "white" : "grey.700",
            "&:hover": {
              bgcolor: activeItem === "Settings" ? "grey.900" : "grey.100",
            },
          }}
        >
          <Box
            sx={{
              width: "20px",
              height: "20px",
              flexShrink: 0,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              "& svg": {
                width: "20px",
                height: "20px",
              },
            }}
          >
            <SettingsIcon width="20px" height="20px" />
          </Box>
          {!isCollapsed && (
            <Typography fontSize="0.875rem" sx={{ flex: 1, textAlign: "left" }}>
              Settings
            </Typography>
          )}
        </Button>
      </Box>
    </Box>
  );
};

export default Sidebar;
