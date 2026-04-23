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
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Card,
  colors,
  pxToRem,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { BookOpen, ChevronDown, CircleCheck, MessagesSquare } from "@wso2/oxygen-ui-icons-react";
import { MessageBubble, type ChatMessage } from "../chat";

export function ConversationSummary({ messages }: { messages: ChatMessage[] }) {
  return (
    <Card sx={{ bgcolor: "background.paper" }} p={1.5} component={Stack} gap={1}>
      <Stack direction="row" alignItems="center" gap={1}>
        <Box color="primary.main">
          <MessagesSquare size={pxToRem(18)} />
        </Box>
        <Typography variant="body1" fontWeight="medium">
          Conversation Summary
        </Typography>
      </Stack>
      <Stack gap={1}>
        <Stack>
          <Typography variant="caption" color="text.secondary">
            Messages Exchanged
          </Typography>
          <Typography variant="h6" fontWeight="medium">
            {messages.length}
          </Typography>
        </Stack>

        {/* TODO: Integrate real data for troubleshooting and articles.
            Currently hardcoded to N/A as these metrics are not yet 
            available in the parent data source.
        */}
        <Stack>
          <Typography variant="caption" color="text.secondary">
            Troubleshooting attempts
          </Typography>
          <Stack direction="row" alignItems="center" gap={1}>
            <Box color="success.main">
              <CircleCheck size={pxToRem(16)} />
            </Box>

            <Typography variant="body2" fontWeight="medium">
              N/A Steps Completed
            </Typography>
          </Stack>
        </Stack>
        <Stack>
          <Typography variant="caption" color="text.secondary">
            Articles Reviewed
          </Typography>
          <Stack direction="row" alignItems="center" gap={1}>
            <Box color={colors.blue[500]}>
              <BookOpen size={pxToRem(16)} />
            </Box>
            <Typography variant="body2" fontWeight="medium">
              N/A Articles Suggested
            </Typography>
          </Stack>
        </Stack>
        {messages.length > 0 && (
          <Accordion
            elevation={0}
            sx={{
              bgcolor: "transparent",
              border: "none",

              "&:before": {
                display: "none",
              },
            }}
            disableGutters
          >
            <AccordionSummary expandIcon={<ChevronDown size={pxToRem(20)} />} sx={{ p: 0 }}>
              <Typography variant="subtitle1" color="text.secondary" component="span">
                View Full Conversation
              </Typography>
            </AccordionSummary>
            <AccordionDetails sx={{ p: 0 }}>
              <Stack gap={2}>
                {messages.map((message, index) => (
                  <MessageBubble key={index} {...message} sx={{ bgcolor: "background.default" }} />
                ))}
              </Stack>
            </AccordionDetails>
          </Accordion>
        )}
      </Stack>
    </Card>
  );
}
