// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import {
  Box,
  Chip,
  Link,
  Stack,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import {
  Calendar,
  ChevronRight,
  Layers,
  Package,
  Users,
} from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { CaseListItem } from "@models/responses";
import {
  formatRelativeTime,
  getStatusColor,
  getStatusIcon,
  resolveColorFromTheme,
  stripHtml,
} from "@utils/support";
import ServiceRequestsListSkeleton from "@components/support/service-requests/ServiceRequestsListSkeleton";

export interface ServiceRequestsListProps {
  serviceRequests: CaseListItem[];
  isLoading: boolean;
  onServiceRequestClick?: (sr: CaseListItem) => void;
}

/**
 * Component to display service requests as cards.
 *
 * @param {ServiceRequestsListProps} props - Service requests array and loading state.
 * @returns {JSX.Element} The rendered service request cards list.
 */
export default function ServiceRequestsList({
  serviceRequests,
  isLoading,
  onServiceRequestClick,
}: ServiceRequestsListProps): JSX.Element {
  const theme = useTheme();

  if (isLoading) {
    return <ServiceRequestsListSkeleton />;
  }

  if (serviceRequests.length === 0) {
    return (
      <Box sx={{ textAlign: "center", py: 6 }}>
        <Typography variant="body1" color="text.secondary">
          No service requests found.
        </Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {serviceRequests.map((sr) => {
        const StatusIcon = getStatusIcon(sr.status?.label);
        const colorPath = getStatusColor(sr.status?.label);
        const resolvedColor = resolveColorFromTheme(colorPath, theme);

        return (
          <Box
            key={sr.id}
            sx={{
              p: 3,
              bgcolor: "background.paper",
              display: "flex",
              flexDirection: "column",
              gap: 1.5,
              cursor: "pointer",
              "&:hover": {
                bgcolor: "action.hover",
              },
            }}
            onClick={() => onServiceRequestClick?.(sr)}
          >
            <Box
              sx={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "flex-start",
              }}
            >
              <Stack direction="row" spacing={1.5} alignItems="center">
                <Typography
                  variant="body2"
                  fontWeight={600}
                  color="text.primary"
                >
                  {sr.number || "--"}
                </Typography>
                <Chip
                  size="small"
                  variant="filled"
                  label={sr.status?.label || "--"}
                  icon={<StatusIcon size={12} />}
                  sx={{
                    bgcolor: alpha(resolvedColor, 0.1),
                    color: resolvedColor,
                    height: 22,
                    fontSize: "0.75rem",
                    "& .MuiChip-icon": {
                      color: "inherit",
                      ml: "6px",
                      mr: "-2px",
                    },
                    "& .MuiChip-label": {
                      px: 1,
                    },
                  }}
                />
                {sr.issueType?.label && (
                  <Chip
                    size="small"
                    label={sr.issueType.label}
                    variant="outlined"
                    sx={{
                      height: 22,
                      fontSize: "0.75rem",
                      bgcolor: "action.hover",
                      borderColor: "transparent",
                    }}
                  />
                )}
              </Stack>
              <Typography variant="caption" color="text.secondary">
                {formatRelativeTime(sr.createdOn)}
              </Typography>
            </Box>

            <Typography
              variant="subtitle1"
              color="text.primary"
              sx={{ fontWeight: 500 }}
            >
              {sr.title || "--"}
            </Typography>

            <Typography
              variant="body2"
              color="text.secondary"
              sx={{
                display: "-webkit-box",
                WebkitLineClamp: 2,
                WebkitBoxOrient: "vertical",
                overflow: "hidden",
              }}
            >
              {stripHtml(sr.description) || "--"}
            </Typography>

            <Box
              sx={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
                mt: 1,
              }}
            >
              <Stack
                direction="row"
                spacing={3}
                sx={{ flexWrap: "wrap", gap: 1.5 }}
              >
                {sr.deployment?.label && (
                  <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                    <Layers size={14} color={theme.palette.text.secondary} />
                    <Typography variant="caption" color="text.secondary">
                      {sr.deployment.label}
                    </Typography>
                  </Box>
                )}
                {sr.deployedProduct?.label && (
                  <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                    <Package size={14} color={theme.palette.text.secondary} />
                    <Typography variant="caption" color="text.secondary">
                      {sr.deployedProduct.label}
                    </Typography>
                  </Box>
                )}
                <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                  <Calendar size={14} color={theme.palette.text.secondary} />
                  <Typography variant="caption" color="text.secondary">
                    {sr.createdOn
                      ? new Date(sr.createdOn).toLocaleDateString("en-US", {
                          month: "short",
                          day: "numeric",
                          year: "numeric",
                        })
                      : "--"}
                  </Typography>
                </Box>
                {sr.assignedEngineer && (
                  <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                    <Users size={14} color={theme.palette.text.secondary} />
                    <Typography variant="caption" color="text.secondary">
                      {typeof sr.assignedEngineer === "string"
                        ? sr.assignedEngineer
                        : sr.assignedEngineer.label ||
                          sr.assignedEngineer.name ||
                          "--"}
                    </Typography>
                  </Box>
                )}
              </Stack>
              <Link
                component="button"
                variant="body2"
                underline="none"
                onClick={(e) => {
                  e.stopPropagation();
                  onServiceRequestClick?.(sr);
                }}
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 0.5,
                  color: "primary.main",
                  fontWeight: 500,
                }}
              >
                View Details
                <ChevronRight size={16} />
              </Link>
            </Box>
          </Box>
        );
      })}
    </Box>
  );
}
