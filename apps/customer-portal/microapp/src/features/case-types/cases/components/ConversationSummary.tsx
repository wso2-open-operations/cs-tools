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
import { Accordion, AccordionDetails, AccordionSummary, Box, Card, Stack, Typography } from "@wso2/oxygen-ui";
import { ChevronDown, MessagesSquare } from "@wso2/oxygen-ui-icons-react";

import { useCreateCase } from "@features/case-types/cases/hooks";
import { Bubble } from "@features/case-types/conversations/components";

export function ConversationSummary() {
  const {
    state: { messages },
  } = useCreateCase();

  return (
    <Card sx={{ bgcolor: "background.paper" }} p={1.5} component={Stack} gap={1}>
      <Stack direction="row" alignItems="center" gap={1}>
        <Box color="primary.main">
          <MessagesSquare size={20} />
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
            <AccordionSummary expandIcon={<ChevronDown size={18} />} sx={{ p: 0 }}>
              <Typography variant="body2" color="text.secondary" component="span">
                View Full Conversation
              </Typography>
            </AccordionSummary>
            <AccordionDetails sx={{ p: 0 }}>
              <Stack gap={2}>
                {messages.map((message, index) => (
                  <Bubble key={index} {...message} />
                ))}
              </Stack>
            </AccordionDetails>
          </Accordion>
        )}
      </Stack>
    </Card>
  );
}
