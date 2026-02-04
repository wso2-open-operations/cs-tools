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

import { Box, Card, CardContent, Typography, Divider } from "@wso2/oxygen-ui";
import { Users } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { contacts } from "@/constants/projectDetailsConstants";
import ContactRow from "./ContactRow";

const ContactInfoCard = (): JSX.Element => {
  return (
    <Card sx={{ height: "100%" }}>
      <CardContent sx={{ p: 3 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Users size={20} />
          <Typography variant="h6">Contact Information</Typography>
        </Box>
        <Divider sx={{ mb: 2, pb: 2 }} />

        <Box sx={{ display: "flex", flexDirection: "column" }}>
          {contacts.map((contact, index) => (
            <Box key={index}>
              <ContactRow contact={contact} />

              {index < contacts.length - 1 && <Divider sx={{ my: 2 }} />}
            </Box>
          ))}
        </Box>
      </CardContent>
    </Card>
  );
};

export default ContactInfoCard;
