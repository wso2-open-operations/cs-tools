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
  Form,
  Stack,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import {
  Calendar,
  Layers,
  Package,
  Users,
} from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { CaseListItem } from "@models/responses";
import {
  formatDateTime,
  getAssignedEngineerLabel,
  getSeverityColor,
  getStatusColor,
  getStatusIcon,
  mapSeverityToDisplay,
  resolveColorFromTheme,
  stripHtml,
} from "@utils/support";
import ServiceRequestsListSkeleton from "./ServiceRequestsListSkeleton";

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
        const assignedLabel = getAssignedEngineerLabel(sr.assignedEngineer);
        const environmentLabel = sr.deployment?.label;
        const productLabel = sr.deployedProduct?.label;

        return (
          <Form.CardButton
            key={sr.id}
            onClick={() => onServiceRequestClick?.(sr)}
            sx={{
              p: 3,
              display: "flex",
              flexDirection: "column",
              alignItems: "stretch",
              gap: 1,
            }}
          >
            <Form.CardHeader
              sx={{ p: 0 }}
              title={
                <Stack
                  direction="row"
                  spacing={1.5}
                  alignItems="center"
                  sx={{ mb: 1, flexWrap: "wrap" }}
                >
                  <Typography
                    variant="body2"
                    fontWeight={500}
                    color="text.primary"
                  >
                    {sr.number || "--"}
                  </Typography>
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: 0.5,
                    }}
                  >
                    <Box
                      sx={{
                        width: 8,
                        height: 8,
                        bgcolor: getSeverityColor(sr.severity?.label),
                      }}
                    />
                    <Typography variant="caption" color="text.secondary">
                      {mapSeverityToDisplay(sr.severity?.label)}
                    </Typography>
                  </Box>
                  <Chip
                    size="small"
                    variant="outlined"
                    label={sr.status?.label || "--"}
                    icon={<StatusIcon size={12} />}
                    sx={{
                      bgcolor: alpha(resolvedColor, 0.1),
                      color: resolvedColor,
                      height: 20,
                      fontSize: "0.75rem",
                      px: 0,
                      "& .MuiChip-icon": {
                        color: "inherit",
                        ml: "6px",
                        mr: "6px",
                      },
                      "& .MuiChip-label": {
                        pl: 0,
                        pr: "6px",
                      },
                    }}
                  />
                  {sr.issueType?.label && (
                    <Chip
                      size="small"
                      label={sr.issueType.label || "--"}
                      variant="outlined"
                      sx={{
                        height: 20,
                        fontSize: "0.75rem",
                      }}
                    />
                  )}
                </Stack>
              }
            />

            <Form.CardContent sx={{ p: 0 }}>
              <Typography
                variant="h6"
                color="text.primary"
                sx={{ mb: 1, fontWeight: 500 }}
              >
                {sr.title || "--"}
              </Typography>

              <Typography
                variant="body2"
                color="text.secondary"
                sx={{
                  mb: 2,
                  display: "-webkit-box",
                  WebkitLineClamp: 2,
                  WebkitBoxOrient: "vertical",
                  overflow: "hidden",
                }}
              >
                {stripHtml(sr.description) || "--"}
              </Typography>
            </Form.CardContent>

            <Form.CardActions
              sx={{
                p: 0,
                justifyContent: "space-between",
                flexWrap: "wrap",
                gap: 2,
              }}
            >
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 3,
                  flexWrap: "wrap",
                }}
              >
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 0.5,
                    flexShrink: 0,
                  }}
                >
                  <Calendar size={14} />
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    sx={{ lineHeight: 1 }}
                  >
                    {formatDateTime(sr.createdOn) || "--"}
                  </Typography>
                </Box>
                {environmentLabel && (
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: 0.5,
                      flexShrink: 0,
                    }}
                  >
                    <Layers size={14} />
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      sx={{ lineHeight: 1 }}
                    >
                      {environmentLabel}
                    </Typography>
                  </Box>
                )}
                {productLabel && (
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: 0.5,
                      flexShrink: 0,
                    }}
                  >
                    <Package size={14} />
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      sx={{ lineHeight: 1 }}
                    >
                      {productLabel}
                    </Typography>
                  </Box>
                )}
                {assignedLabel && (
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: 0.5,
                      flexShrink: 0,
                    }}
                  >
                    <Users size={14} />
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      sx={{ lineHeight: 1 }}
                    >
                      {assignedLabel}
                    </Typography>
                  </Box>
                )}
              </Box>
            </Form.CardActions>
          </Form.CardButton>
        );
      })}
    </Box>
  );
}
