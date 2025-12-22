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

import { Article, Check, Send, Stars } from "@mui/icons-material";
import {
  Box,
  ButtonBase as Button,
  Card,
  Divider,
  IconButton,
  Stack,
  InputBase as TextField,
  Typography,
} from "@mui/material";
import { Link } from "react-router-dom";

export default function ChatPage() {
  return (
    <>
      <Stack p={2} mb={20} gap={2}>
        <Card component={Stack} p={1.5} elevation={0}>
          <Stack direction="row" justifyContent="start" gap={1} mb={1}>
            <Stars color="primary" sx={(theme) => ({ fontSize: theme.typography.pxToRem(22) })} />
            <Typography variant="subtitle2" fontWeight="regular" color="text.disabled">
              Just Now
            </Typography>
          </Stack>
          <Typography variant="body2">
            Hi! I'm Novera, your AI-powered support assistant. How can I help you today? Please describe the issue
            you're experiencing.
          </Typography>
        </Card>
        <Stack direction="row" justifyContent="end">
          <Card component={Stack} p={1.5} elevation={0} ml={10} width="fit-content">
            <Typography variant="body2">How can I help you today?</Typography>
            <Stack direction="row" justifyContent="end">
              <Typography variant="subtitle2" fontWeight="regular" color="text.disabled" mt={1}>
                Just Now
              </Typography>
            </Stack>
          </Card>
        </Stack>
        <Card component={Stack} gap={2} p={1.5} elevation={0}>
          <Box>
            <Stack direction="row" justifyContent="start" gap={1} mb={1}>
              <Stars color="primary" sx={(theme) => ({ fontSize: theme.typography.pxToRem(22) })} />
              <Typography variant="subtitle2" fontWeight="regular" color="text.disabled">
                Just Now
              </Typography>
            </Stack>
            <Typography variant="body2">
              Thanks for those details. Based on what you've shared, here are a few things to check:
            </Typography>
          </Box>
          <Stack gap={1}>
            <Stack direction="row" gap={2}>
              <Check color="success" />
              <Typography variant="body2">Verify your backend service timeout configurations</Typography>
            </Stack>
            <Stack direction="row" gap={2}>
              <Check color="success" />
              <Typography variant="body2">Check system resource utilization CPU, memory</Typography>
            </Stack>
            <Stack direction="row" gap={2}>
              <Check color="success" />
              <Typography variant="body2">Review recent deployment or configuration changes</Typography>
            </Stack>
          </Stack>
          <Divider />
          <Stack gap={1.5}>
            <Card
              component={Stack}
              direction="row"
              alignItems="center"
              gap={2}
              p={1}
              elevation={0}
              sx={{ bgcolor: "background.card" }}
            >
              <Article sx={{ color: "semantic.portal.accent.blue" }} />
              <Stack>
                <Typography variant="subtitle1">Troubleshooting API Gateway Timeouts</Typography>
                <Typography variant="caption" color="text.secondary">
                  KB-1234
                </Typography>
              </Stack>
            </Card>
            <Card
              component={Stack}
              direction="row"
              alignItems="center"
              gap={2}
              p={1}
              elevation={0}
              sx={{ bgcolor: "background.card" }}
            >
              <Article sx={{ color: "semantic.portal.accent.blue" }} />
              <Stack>
                <Typography variant="subtitle1">Troubleshooting API Gateway Timeouts</Typography>
                <Typography variant="caption" color="text.secondary">
                  KB-1234
                </Typography>
              </Stack>
            </Card>
          </Stack>
        </Card>
      </Stack>
      <Stack
        position="fixed"
        width="100%"
        p={2}
        bottom={100}
        gap={4}
        justifyContent="space-between"
        bgcolor="background.paper"
      >
        <Stack direction="row">
          <Typography variant="body2">I can create a support case with all the details we've discussed.</Typography>
          <Button
            component={Link}
            to="/create-case"
            variant="contained"
            sx={{ fontWeight: "bold", flexShrink: 0, height: 40 }}
          >
            Create Case
          </Button>
        </Stack>
        <Stack direction="row" gap={2}>
          <TextField placeholder="Type your message" fullWidth sx={{ alignSelf: "center" }} />
          <IconButton color="primary">
            <Send sx={(theme) => ({ color: "text.tertiary", fontSize: theme.typography.pxToRem(21) })} />
          </IconButton>
        </Stack>
      </Stack>
    </>
  );
}
