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
  useTheme,
} from "@wso2/oxygen-ui";
import { Calendar, Layers, Package, Users } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { NULL_PLACEHOLDER } from "@constants/common";
import CaseCardDescriptionClamp from "@components/list-view/CaseCardDescriptionClamp";
import {
  formatDateTime,
  getAssignedEngineerLabel,
  getStatusColor,
  resolveColorFromTheme,
} from "@features/support/utils/support";
import ServiceRequestsListSkeleton from "@features/operations/components/service-requests/ServiceRequestsListSkeleton";
import EmptyIcon from "@components/empty-state/EmptyIcon";
import SearchNoResultsIcon from "@components/empty-state/SearchNoResultsIcon";
import type { ServiceRequestsListProps } from "@features/operations/types/serviceRequests";
import {
  OPERATIONS_LIST_EMPTY_CONTAINER_PY,
  OPERATIONS_LIST_EMPTY_ICON_MARGIN_BOTTOM_PX,
  OPERATIONS_LIST_EMPTY_ILLUSTRATION_WIDTH_PX,
  SERVICE_REQUESTS_LIST_EMPTY_DEFAULT_MESSAGE,
  SERVICE_REQUESTS_LIST_EMPTY_REFINED_MESSAGE,
} from "@features/operations/constants/operationsConstants";

/**
 * Component to display service requests as cards.
 *
 * @param props - Service requests array and loading state.
 * @returns {JSX.Element} The rendered service request cards list.
 */
export default function ServiceRequestsList({
  serviceRequests,
  isLoading,
  hasListRefinement = false,
  onServiceRequestClick,
}: ServiceRequestsListProps): JSX.Element {
  const theme = useTheme();

  if (isLoading) {
    return <ServiceRequestsListSkeleton />;
  }

  const emptyIconStyle = {
    width: OPERATIONS_LIST_EMPTY_ILLUSTRATION_WIDTH_PX,
    maxWidth: "100%",
    height: "auto",
    marginBottom: OPERATIONS_LIST_EMPTY_ICON_MARGIN_BOTTOM_PX,
  };

  if (serviceRequests.length === 0) {
    if (hasListRefinement) {
      return (
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            py: OPERATIONS_LIST_EMPTY_CONTAINER_PY,
          }}
        >
          <SearchNoResultsIcon style={emptyIconStyle} />
          <Typography variant="body1" color="text.secondary">
            {SERVICE_REQUESTS_LIST_EMPTY_REFINED_MESSAGE}
          </Typography>
        </Box>
      );
    }
    return (
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          py: OPERATIONS_LIST_EMPTY_CONTAINER_PY,
        }}
      >
        <EmptyIcon style={emptyIconStyle} />
        <Typography variant="body1" color="text.secondary">
          {SERVICE_REQUESTS_LIST_EMPTY_DEFAULT_MESSAGE}
        </Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {serviceRequests.map((sr) => {
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
              minHeight: 172,
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
                    {sr.number || NULL_PLACEHOLDER}
                  </Typography>
                  <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
                    <Box
                      sx={{
                        width: 8,
                        height: 8,
                        borderRadius: "50%",
                        bgcolor: resolvedColor,
                        flexShrink: 0,
                      }}
                    />
                    <Typography variant="caption" sx={{ color: resolvedColor }}>
                      {sr.status?.label || NULL_PLACEHOLDER}
                    </Typography>
                  </Box>
                  {sr.issueType?.label && (
                    <Chip
                      size="small"
                      label={sr.issueType.label || NULL_PLACEHOLDER}
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
                {sr.title || NULL_PLACEHOLDER}
              </Typography>

              <CaseCardDescriptionClamp
                description={sr.description}
                emptyLabel={NULL_PLACEHOLDER}
              />
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
                    {formatDateTime(sr.createdOn) || NULL_PLACEHOLDER}
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
