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

import { Article, CheckBox, ExpandMore, Folder, Forum, Stars } from "@mui/icons-material";
import {
  InputBase as TextField,
  ButtonBase as Button,
  Stack,
  FormControl,
  Typography,
  Select,
  MenuItem,
  InputAdornment,
  Card,
  Accordion,
  AccordionSummary,
  AccordionDetails,
} from "@mui/material";
import { Link } from "react-router-dom";

export default function CreateCasePage() {
  return (
    <Stack p={2} pb={20} gap={5}>
      <Stack gap={2}>
        <FormControl component={Stack} gap={1} fullWidth>
          <Typography component="label" variant="subtitle2" fontWeight="regular">
            Project
          </Typography>
          <Select
            value={0}
            sx={{ bgcolor: "background.paper" }}
            startAdornment={
              <InputAdornment position="start">
                <Folder />
              </InputAdornment>
            }
          >
            <MenuItem value={0}>Dreamworks Inc</MenuItem>
            <MenuItem value={1}>Newsline Enterprise</MenuItem>
            <MenuItem value={2}>Goods Store Mart</MenuItem>
          </Select>
        </FormControl>
        <FormControl component={Stack} gap={1} fullWidth>
          <Typography component="label" variant="subtitle2" fontWeight="regular">
            Product & Version
          </Typography>
          <Select value={0} sx={{ bgcolor: "background.paper" }}>
            <MenuItem value={0}>WSO2 API Manager v4.2.0</MenuItem>
            <MenuItem value={1}>WSO2 Identity Access Manager v4.2.0</MenuItem>
          </Select>
        </FormControl>
        <FormControl component={Stack} gap={1} fullWidth>
          <Typography component="label" variant="subtitle2" fontWeight="regular">
            Deployment Type
          </Typography>
          <Select value={0} sx={{ bgcolor: "background.paper" }}>
            <MenuItem value={0}>Production</MenuItem>
            <MenuItem value={1}>Staging</MenuItem>
            <MenuItem value={2}>Development</MenuItem>
          </Select>
        </FormControl>
      </Stack>
      <Stack gap={2}>
        <Typography variant="body1" fontWeight="bold">
          Case Details
        </Typography>
        <FormControl component={Stack} gap={1} fullWidth>
          <Typography component="label" variant="subtitle2" fontWeight="regular">
            Issue Title
          </Typography>
          <TextField value="API Gateway timeout issues in production" sx={{ bgcolor: "background.paper" }} />
        </FormControl>
        <FormControl component={Stack} gap={1} fullWidth>
          <Typography component="label" variant="subtitle2" fontWeight="regular">
            Case Description
          </Typography>
          <TextField
            value="Novera: Hi! I'm Novera, your AI-powered support assistant. How can I help you today? Please describe the issue you're experiencing.

Customer: dfasdfasd

Novera: Thanks for those details. Based on what you've shared, here are a few things to check:"
            sx={{ bgcolor: "background.paper", lineHeight: 1.65 }}
            rows={10}
            multiline
          />
        </FormControl>
        <FormControl component={Stack} gap={1} fullWidth>
          <Typography component="label" variant="subtitle2" fontWeight="regular">
            Issue Type
          </Typography>
          <Select value={0} sx={{ bgcolor: "background.paper" }}>
            <MenuItem value={0}>Configuration Issue</MenuItem>
            <MenuItem value={1}>Query</MenuItem>
            <MenuItem value={2}>Security Vulnerability</MenuItem>
          </Select>
        </FormControl>
        <FormControl component={Stack} gap={1} fullWidth>
          <Typography component="label" variant="subtitle2" fontWeight="regular">
            Severity Level
          </Typography>
          <Select value={0} sx={{ bgcolor: "background.paper" }}>
            <MenuItem value={0}>S1 Critical</MenuItem>
            <MenuItem value={1}>S2 Medium</MenuItem>
            <MenuItem value={2}>S3 Low</MenuItem>
          </Select>
        </FormControl>
      </Stack>
      <Card p={1.5} component={Stack} gap={1} elevation={0}>
        <Stack direction="row" alignItems="center" gap={1}>
          <Forum color="primary" />
          <Typography variant="h6" fontWeight="medium">
            Conversation Summary
          </Typography>
        </Stack>
        <Stack gap={0.8}>
          <Stack direction="row" gap={2} alignItems="center">
            <Typography variant="body2" color="text.secondary">
              Messages Exchanged
            </Typography>
            <Typography variant="body2" fontWeight="medium">
              6
            </Typography>
          </Stack>
          <Stack direction="row" gap={2} alignItems="center">
            <Typography variant="body2" color="text.secondary">
              Troubleshooting attempts
            </Typography>
            <Stack direction="row" alignItems="center" gap={1}>
              <CheckBox color="success" />
              <Typography variant="body2" fontWeight="medium">
                2 Steps Completed
              </Typography>
            </Stack>
          </Stack>
          <Stack direction="row" gap={2} alignItems="center">
            <Typography variant="body2" color="text.secondary">
              Articles Reviewed
            </Typography>
            <Stack direction="row" alignItems="center" gap={1}>
              <Article sx={{ color: "semantic.portal.accent.blue" }} />
              <Typography variant="body2" fontWeight="medium">
                3 Articles Suggested
              </Typography>
            </Stack>
          </Stack>
          <Accordion
            elevation={0}
            sx={{
              "&:before": {
                display: "none",
              },
            }}
            disableGutters
          >
            <AccordionSummary expandIcon={<ExpandMore />} sx={{ p: 0 }}>
              <Typography variant="subtitle1" fontWeight="bold" color="text.secondary" component="span">
                View Full Conversation
              </Typography>
            </AccordionSummary>
            <AccordionDetails sx={{ p: 0 }}>
              <Stack gap={2}>
                <Card component={Stack} p={1.5} sx={{ bgcolor: "background.card" }} elevation={0}>
                  <Stack direction="row" justifyContent="start" gap={1} mb={1}>
                    <Stars color="primary" sx={(theme) => ({ fontSize: theme.typography.pxToRem(22) })} />
                    <Typography variant="subtitle2" fontWeight="regular" color="text.disabled">
                      15:25
                    </Typography>
                  </Stack>
                  <Typography variant="body2">
                    Hi! I'm Novera, your AI-powered support assistant. How can I help you today? Please describe the
                    issue you're experiencing.
                  </Typography>
                </Card>
                <Stack direction="row" justifyContent="end">
                  <Card
                    component={Stack}
                    p={1.5}
                    elevation={0}
                    ml={10}
                    width="fit-content"
                    sx={{ bgcolor: "background.card" }}
                  >
                    <Typography variant="body2">How can I help you today?</Typography>
                    <Stack direction="row" justifyContent="end">
                      <Typography variant="subtitle2" fontWeight="regular" color="text.disabled" mt={1}>
                        15:27
                      </Typography>
                    </Stack>
                  </Card>
                </Stack>
              </Stack>
            </AccordionDetails>
          </Accordion>
        </Stack>
      </Card>
      <Button component={Link} to="/support" variant="contained" sx={{ fontWeight: "bold" }}>
        Create Case
      </Button>
    </Stack>
  );
}
