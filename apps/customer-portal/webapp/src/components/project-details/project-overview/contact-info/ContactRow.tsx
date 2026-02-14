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

import { Box, Typography } from "@wso2/oxygen-ui";
import { Mail } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { Contact } from "@constants/projectDetailsConstants";

interface ContactRowProps {
  contact: Contact;
}

const ContactRow = ({ contact }: ContactRowProps): JSX.Element => {
  const Icon = contact.icon;
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Typography variant="caption" color="text.secondary">
        {contact.role}
      </Typography>
      <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
        <Box
          sx={{
            width: 40,
            height: 40,
            borderRadius: "50%",
            background: contact.bgColor,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            color: "white",
          }}
        >
          <Icon size={20} />
        </Box>
      <Box>
        <a
          href={`mailto:${contact.email}`}
          style={{
            textDecoration: "none",
            color: "inherit",
          }}
        >
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 1,
              color: "text.primary",
              "&:hover": { color: "primary.main" },
            }}
          >
            <Mail size={16} />
            <Typography variant="body2" color="inherit">
              {contact.email}
            </Typography>
          </Box>
        </a>
      </Box>
    </Box>
  </Box>
  );
};

export default ContactRow;
