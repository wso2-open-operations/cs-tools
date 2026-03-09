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
  Card,
  CardContent,
  Typography,
  Divider,
  Skeleton,
} from "@wso2/oxygen-ui";
import { Users, User, Shield } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { Contact } from "@constants/projectDetailsConstants";
import ContactRow from "@components/project-details/project-overview/contact-info/ContactRow";
import { colors } from "@wso2/oxygen-ui";
import type { ProjectDetails } from "@models/responses";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";

interface ContactInfoCardProps {
  project?: ProjectDetails;
  isLoading?: boolean;
  isError?: boolean;
}

const ContactInfoCard = ({
  project,
  isLoading,
  isError,
}: ContactInfoCardProps): JSX.Element => {
  // Build contacts array from project data
  const contacts: Contact[] = [];

  if (project?.account?.ownerEmail) {
    contacts.push({
      role: "Account Manager",
      email: project.account.ownerEmail,
      icon: User,
      bgColor: colors.blue[700],
    });
  }

  contacts.push({
    role: "Technical Owner",
    email: project?.account?.technicalOwnerEmail || null,
    icon: Shield,
    bgColor: colors.purple[400],
  });

  const renderContactSkeleton = () => (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Skeleton variant="text" width={120} height={16} />
      <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
        <Skeleton variant="circular" width={40} height={40} />
        <Skeleton variant="text" width={200} height={20} />
      </Box>
    </Box>
  );

  return (
    <Card sx={{ height: "100%" }}>
      <CardContent sx={{ p: 3 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Users size={20} />
          <Typography variant="h6">Contact Information</Typography>
        </Box>
        <Divider sx={{ mb: 2, pb: 2 }} />

        {isLoading ? (
          <Box sx={{ display: "flex", flexDirection: "column" }}>
            {renderContactSkeleton()}
            <Divider sx={{ my: 2 }} />
            {renderContactSkeleton()}
          </Box>
        ) : isError ? (
          <Box sx={{ textAlign: "center", py: 2 }}>
            <ErrorIndicator entityName="contact information" />
          </Box>
        ) : contacts.length > 0 ? (
          <Box sx={{ display: "flex", flexDirection: "column" }}>
            {contacts.map((contact, index) => (
              <Box key={index}>
                <ContactRow contact={contact} />

                {index < contacts.length - 1 && <Divider sx={{ my: 2 }} />}
              </Box>
            ))}
          </Box>
        ) : (
          <Box sx={{ textAlign: "center", py: 2 }}>
            <Typography variant="body2" color="text.secondary">
              No contact information available
            </Typography>
          </Box>
        )}
      </CardContent>
    </Card>
  );
};

export default ContactInfoCard;
