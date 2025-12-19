// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import React, { useState, useEffect } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  Box,
  TextField,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  Button,
  Typography,
  Chip,
  Card,
  Divider,
  Accordion,
  AccordionSummary,
  AccordionDetails,
} from "@mui/material";
import {
  CheckCircle,
  AutoAwesome,
  ArrowBack,
  Edit,
  Message,
  ExpandMore,
} from "@mui/icons-material";
import { Endpoints } from "@/services/endpoints";
import apiClient from "@/services/apiClient";
import { useGet } from "@/services/useApi";
import { useProject } from "@/context/ProjectContext";

interface FilterOptionsResponse {
  statuses: string[];
  severities: string[];
  categories: string[];
  products: string[];
  environments: string[];
}

interface FormData {
  product: string;
  deployment: string;
  title: string;
  description: string;
  issueType: string;
  severity: string;
}

const CreateCasePage: React.FC = () => {
  const { sysId } = useParams<{ sysId: string }>();
  const navigate = useNavigate();
  const { currentProject } = useProject();
  const projectName = currentProject?.name || "";

  const [isSubmitting, setIsSubmitting] = useState(false);

  const [formData, setFormData] = useState<FormData>({
    product: "",
    deployment: "",
    title: "",
    description: "",
    issueType: "",
    severity: "S2 - Medium",
  });

  // Fetch filter options
  const { data: filterOptions } = useGet<FilterOptionsResponse>(
    ["getCaseFilterOptions", sysId],
    Endpoints.getCaseFilterOptions(sysId || ""),
    {
      enabled: !!sysId,
    }
  );

  useEffect(() => {
    window.scrollTo(0, 0);
  }, []);

  const handleChange = (
    e:
      | React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>
      | { target: { name?: string; value: unknown } }
  ) => {
    const { name, value } = e.target;
    if (name) {
      setFormData((prev) => ({ ...prev, [name]: value as string }));
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    setError(null);

    try {
      const requestBody = {
        short_description: formData.title,
        description: formData.description,
        category: formData.issueType,
        priority: formData.severity,
        product: formData.product,
        u_project_deployment: formData.deployment,
      };

      const endpoint = Endpoints.postCase(sysId || "");
      const url = `${endpoint.baseUrl}${endpoint.path}`;

      const response = await apiClient.post<{
        caseId: string;
        projectId: string;
      }>(url, requestBody);

      if (response.data?.caseId && response.data?.projectId) {
        navigate(
          `/${response.data.projectId}/support/cases/${response.data.caseId}`
        );
      }
    } catch (err) {
      console.error("Error creating case:", err);
      setError("Failed to create case. Please try again.");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Box sx={{ p: 4, maxWidth: "1200px", mx: "auto" }}>
      {/* Header */}
      <Box sx={{ mb: 4 }}>
        <Button
          startIcon={<ArrowBack />}
          onClick={() => navigate(`/${sysId}/support`)}
          sx={{
            textTransform: "none",
            color: "text.secondary",
            mb: 2,
            ml: -1,
            "&:hover": { backgroundColor: "action.hover" },
          }}
        >
          Back to Chat
        </Button>

        <Box
          display="flex"
          justifyContent="space-between"
          alignItems="flex-start"
        >
          <Box>
            <Typography
              variant="h5"
              sx={{ color: "#111827", fontWeight: 600, mb: 0.5 }}
            >
              Review Case Details
            </Typography>
            <Typography variant="body2" sx={{ color: "#6B7280" }}>
              Please review and edit the auto-populated information before
              submitting
            </Typography>
          </Box>
          <Chip
            icon={<AutoAwesome sx={{ fontSize: 12 }} />}
            label="AI Generated"
            size="small"
            sx={{
              bgcolor: "#FFEDD5",
              color: "#C2410C",
              borderColor: "#FED7AA",
              border: "1px solid",
              height: 24,
              fontSize: "0.75rem",
            }}
          />
        </Box>
      </Box>

      {/* Grid Layout */}
      <Box sx={{ display: "grid", gridTemplateColumns: "2fr 1fr", gap: 3 }}>
        {/* Form Column */}
        <Box>
          <form onSubmit={handleSubmit}>
            <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
              {/* Info Banner */}
              <Card
                sx={{
                  bgcolor: "#FFF7ED",
                  border: "1px solid #FED7AA",
                  p: 2,
                  boxShadow: 0,
                }}
              >
                <Box display="flex" gap={1.5}>
                  <CheckCircle
                    sx={{ color: "#EA580C", fontSize: 20, mt: 0.25 }}
                  />
                  <Box>
                    <Typography
                      sx={{ fontSize: "0.875rem", color: "#111827", mb: 0.5 }}
                    >
                      Case details auto-populated from your conversation
                    </Typography>
                    <Typography sx={{ fontSize: "0.75rem", color: "#6B7280" }}>
                      All fields below have been filled based on your chat with
                      Novera. Please review and edit as needed.
                    </Typography>
                  </Box>
                </Box>
              </Card>

              {/* Basic Information */}
              <Card sx={{ p: 3, boxShadow: 0, border: "1px solid #E5E7EB" }}>
                <Box
                  display="flex"
                  justifyContent="space-between"
                  alignItems="center"
                  mb={3}
                >
                  <Typography
                    variant="h6"
                    sx={{ color: "#111827", fontSize: "1rem" }}
                  >
                    Basic Information
                  </Typography>
                  <Edit sx={{ fontSize: 16, color: "#9CA3AF" }} />
                </Box>

                <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
                  {/* Project */}
                  <FormControl fullWidth size="small">
                    <InputLabel>Project *</InputLabel>
                    <Select
                      value={projectName}
                      label="Project *"
                      disabled
                      sx={{ bgcolor: "#F9FAFB" }}
                    >
                      <MenuItem value={projectName}>{projectName}</MenuItem>
                    </Select>
                  </FormControl>

                  {/* Product & Version */}
                  <Box>
                    <Box display="flex" alignItems="center" gap={1} mb={1}>
                      <Typography
                        component="label"
                        sx={{ fontSize: "0.875rem", fontWeight: 500 }}
                      >
                        Product & Version{" "}
                        <span style={{ color: "#EF4444" }}>*</span>
                      </Typography>
                      <Chip
                        icon={<AutoAwesome sx={{ fontSize: 12 }} />}
                        label="Auto-detected"
                        size="small"
                        sx={{
                          bgcolor: "#F3F4F6",
                          color: "text.secondary",
                          height: 20,
                          fontSize: "0.75rem",
                        }}
                      />
                    </Box>
                    <FormControl fullWidth size="small">
                      <Select
                        name="product"
                        value={formData.product}
                        onChange={handleChange}
                        required
                      >
                        {filterOptions?.products.map((product) => (
                          <MenuItem key={product} value={product}>
                            {product}
                          </MenuItem>
                        ))}
                      </Select>
                    </FormControl>
                  </Box>

                  {/* Deployment Type */}
                  <Box>
                    <Box display="flex" alignItems="center" gap={1} mb={1}>
                      <Typography
                        component="label"
                        sx={{ fontSize: "0.875rem", fontWeight: 500 }}
                      >
                        Deployment Type{" "}
                        <span style={{ color: "#EF4444" }}>*</span>
                      </Typography>
                      <Chip
                        icon={<AutoAwesome sx={{ fontSize: 12 }} />}
                        label="Auto-detected"
                        size="small"
                        sx={{
                          bgcolor: "#F3F4F6",
                          color: "text.secondary",
                          height: 20,
                          fontSize: "0.75rem",
                        }}
                      />
                    </Box>
                    <FormControl fullWidth size="small">
                      <Select
                        name="deployment"
                        value={formData.deployment}
                        onChange={handleChange}
                        required
                      >
                        {filterOptions?.environments.map((env) => (
                          <MenuItem key={env} value={env}>
                            {env}
                          </MenuItem>
                        ))}
                      </Select>
                    </FormControl>
                  </Box>
                </Box>
              </Card>

              {/* Case Details */}
              <Card sx={{ p: 3, boxShadow: 0, border: "1px solid #E5E7EB" }}>
                <Box
                  display="flex"
                  justifyContent="space-between"
                  alignItems="center"
                  mb={3}
                >
                  <Typography
                    variant="h6"
                    sx={{ color: "#111827", fontSize: "1rem" }}
                  >
                    Case Details
                  </Typography>
                  <Edit sx={{ fontSize: 16, color: "#9CA3AF" }} />
                </Box>

                <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
                  {/* Issue Title */}
                  <Box>
                    <Box display="flex" alignItems="center" gap={1} mb={1}>
                      <Typography
                        component="label"
                        sx={{ fontSize: "0.875rem", fontWeight: 500 }}
                      >
                        Issue Title <span style={{ color: "#EF4444" }}>*</span>
                      </Typography>
                      <Chip
                        icon={<AutoAwesome sx={{ fontSize: 12 }} />}
                        label="Generated from chat"
                        size="small"
                        sx={{
                          bgcolor: "#F3F4F6",
                          color: "text.secondary",
                          height: 20,
                          fontSize: "0.75rem",
                        }}
                      />
                    </Box>
                    <TextField
                      fullWidth
                      size="small"
                      name="title"
                      value={formData.title}
                      onChange={handleChange}
                      required
                      placeholder="Issue title"
                    />
                  </Box>

                  {/* Case Description */}
                  <Box>
                    <Box display="flex" alignItems="center" gap={1} mb={1}>
                      <Typography
                        component="label"
                        sx={{ fontSize: "0.875rem", fontWeight: 500 }}
                      >
                        Case Description{" "}
                        <span style={{ color: "#EF4444" }}>*</span>
                      </Typography>
                      <Chip
                        icon={<AutoAwesome sx={{ fontSize: 12 }} />}
                        label="From conversation"
                        size="small"
                        sx={{
                          bgcolor: "#F3F4F6",
                          color: "text.secondary",
                          height: 20,
                          fontSize: "0.75rem",
                        }}
                      />
                    </Box>
                    <TextField
                      fullWidth
                      name="description"
                      value={formData.description}
                      onChange={handleChange}
                      required
                      multiline
                      rows={4}
                      placeholder="Case description"
                    />
                    <Typography
                      sx={{ fontSize: "0.75rem", color: "#6B7280", mt: 1 }}
                    >
                      This includes all the information you shared with Novera
                    </Typography>
                  </Box>

                  {/* Issue Type */}
                  <Box>
                    <Box display="flex" alignItems="center" gap={1} mb={1}>
                      <Typography
                        component="label"
                        sx={{ fontSize: "0.875rem", fontWeight: 500 }}
                      >
                        Issue Type <span style={{ color: "#EF4444" }}>*</span>
                      </Typography>
                      <Chip
                        icon={<AutoAwesome sx={{ fontSize: 12 }} />}
                        label="AI classified"
                        size="small"
                        sx={{
                          bgcolor: "#F3F4F6",
                          color: "text.secondary",
                          height: 20,
                          fontSize: "0.75rem",
                        }}
                      />
                    </Box>
                    <FormControl fullWidth size="small">
                      <Select
                        name="issueType"
                        value={formData.issueType}
                        onChange={handleChange}
                        required
                      >
                        {filterOptions?.categories.map((category) => (
                          <MenuItem key={category} value={category}>
                            {category}
                          </MenuItem>
                        ))}
                      </Select>
                    </FormControl>
                  </Box>

                  {/* Severity Level */}
                  <Box>
                    <Box display="flex" alignItems="center" gap={1} mb={1}>
                      <Typography
                        component="label"
                        sx={{ fontSize: "0.875rem", fontWeight: 500 }}
                      >
                        Severity Level{" "}
                        <span style={{ color: "#EF4444" }}>*</span>
                      </Typography>
                      <Chip
                        icon={<AutoAwesome sx={{ fontSize: 12 }} />}
                        label="AI assessed"
                        size="small"
                        sx={{
                          bgcolor: "#F3F4F6",
                          color: "text.secondary",
                          height: 20,
                          fontSize: "0.75rem",
                        }}
                      />
                    </Box>
                    <Card
                      sx={{
                        p: 2,
                        bgcolor: "#F9FAFB",
                        border: "1px solid #E5E7EB",
                        boxShadow: 0,
                      }}
                    >
                      <Box display="flex" alignItems="center" gap={1.5}>
                        <Box
                          sx={{
                            width: 12,
                            height: 12,
                            borderRadius: "50%",
                            bgcolor: "#EAB308",
                          }}
                        />
                        <Box flex={1}>
                          <Typography
                            sx={{ color: "#111827", fontSize: "0.875rem" }}
                          >
                            {formData.severity}
                          </Typography>
                          <Typography
                            sx={{ color: "#6B7280", fontSize: "0.875rem" }}
                          >
                            Important features affected
                          </Typography>
                        </Box>
                      </Box>
                    </Card>
                    <Typography
                      sx={{ fontSize: "0.75rem", color: "#6B7280", mt: 1 }}
                    >
                      Based on deployment type and issue description
                    </Typography>
                  </Box>
                </Box>
              </Card>

              {/* Action Buttons */}
              <Box
                display="flex"
                justifyContent="space-between"
                pt={2}
                borderTop="1px solid #E5E7EB"
              >
                <Button
                  onClick={() => navigate(`/${sysId}/support`)}
                  variant="outlined"
                  sx={{
                    textTransform: "none",
                    color: "#374151",
                    borderColor: "#D1D5DB",
                  }}
                >
                  Back to Chat
                </Button>
                <Button
                  type="submit"
                  variant="contained"
                  startIcon={<CheckCircle />}
                  disabled={isSubmitting}
                  sx={{
                    textTransform: "none",
                    bgcolor: "#EA580C",
                    "&:hover": { bgcolor: "#C2410C" },
                    "&:disabled": { bgcolor: "#D1D5DB" },
                  }}
                >
                  {isSubmitting ? "Creating..." : "Create Support Case"}
                </Button>
              </Box>
            </Box>
          </form>
        </Box>

        {/* Sidebar */}
        <Box>
          <Card
            sx={{
              p: 3,
              boxShadow: 0,
              border: "1px solid #E5E7EB",
              position: "sticky",
              top: 32,
            }}
          >
            <Box display="flex" alignItems="center" gap={1} mb={2}>
              <Message sx={{ fontSize: 20, color: "#EA580C" }} />
              <Typography
                variant="h6"
                sx={{ color: "#111827", fontSize: "1rem" }}
              >
                Conversation Summary
              </Typography>
            </Box>

            <Divider sx={{ mb: 2 }} />

            <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
              <Box>
                <Typography
                  sx={{ fontSize: "0.75rem", color: "#6B7280", mb: 0.5 }}
                >
                  Messages exchanged
                </Typography>
                <Typography sx={{ color: "#111827", fontSize: "0.875rem" }}>
                  6
                </Typography>
              </Box>

              <Box>
                <Typography
                  sx={{ fontSize: "0.75rem", color: "#6B7280", mb: 0.5 }}
                >
                  Troubleshooting attempts
                </Typography>
                <Typography sx={{ color: "#111827", fontSize: "0.875rem" }}>
                  2 steps completed
                </Typography>
              </Box>

              <Box>
                <Typography
                  sx={{ fontSize: "0.75rem", color: "#6B7280", mb: 0.5 }}
                >
                  KB articles reviewed
                </Typography>
                <Typography sx={{ color: "#111827", fontSize: "0.875rem" }}>
                  3 articles suggested
                </Typography>
              </Box>
            </Box>

            <Divider sx={{ my: 2 }} />

            <Accordion
              elevation={0}
              sx={{
                "&:before": { display: "none" },
                border: 0,
              }}
            >
              <AccordionSummary
                expandIcon={<ExpandMore />}
                sx={{
                  px: 0,
                  minHeight: "auto",
                  "&.Mui-expanded": { minHeight: "auto" },
                  "& .MuiAccordionSummary-content": {
                    margin: "8px 0",
                  },
                }}
              >
                <Typography sx={{ fontSize: "0.875rem", fontWeight: 500 }}>
                  View full conversation
                </Typography>
              </AccordionSummary>
              <AccordionDetails sx={{ px: 0 }}>
                <Typography sx={{ fontSize: "0.875rem", color: "#6B7280" }}>
                  Conversation details will appear here
                </Typography>
              </AccordionDetails>
            </Accordion>

            <Card
              sx={{
                mt: 2,
                p: 1.5,
                bgcolor: "#EFF6FF",
                border: "1px solid #BFDBFE",
                boxShadow: 0,
              }}
            >
              <Typography sx={{ fontSize: "0.75rem", color: "#1E40AF" }}>
                💡 All conversation details will be attached to your case for
                the support team's reference.
              </Typography>
            </Card>
          </Card>
        </Box>
      </Box>
    </Box>
  );
};

export default CreateCasePage;
