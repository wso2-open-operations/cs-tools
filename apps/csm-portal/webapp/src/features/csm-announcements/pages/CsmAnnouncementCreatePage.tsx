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

import { Box, Button, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import AnnouncementKindSelector, {
  type AnnouncementKind,
} from "@features/csm-announcements/components/AnnouncementKindSelector";
import CreateCustomerAnnouncementForm from "@features/csm-announcements/components/CreateCustomerAnnouncementForm";
import CreateEolAnnouncementForm from "@features/csm-announcements/components/CreateEolAnnouncementForm";
import { useNavTransition } from "@hooks/useNavTransition";

const BACK_TARGET = "/announcements";

/**
 * Entry point for the announcement create flow. Owns only the top-level
 * choice between the two announcement kinds (AnnouncementKindSelector) and
 * renders whichever form applies — each kind is fully self-contained
 * (CreateCustomerAnnouncementForm, CreateEolAnnouncementForm), so switching
 * kinds never carries state between them and this page has no form fields of
 * its own to reset.
 */
export default function CsmAnnouncementCreatePage(): JSX.Element {
  const navigate = useNavTransition();
  const [kind, setKind] = useState<AnnouncementKind>("customer");

  return (
    <Box sx={{ width: "100%", px: 3, py: 3 }}>
      <Button
        variant="text"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate(BACK_TARGET)}
        sx={{ mb: 1 }}
      >
        Back
      </Button>
      <Typography variant="h5" sx={{ mb: 2 }}>
        New announcement
      </Typography>

      <Box sx={{ mb: 2.5 }}>
        <AnnouncementKindSelector kind={kind} onKindChange={setKind} />
      </Box>

      {kind === "customer" ? <CreateCustomerAnnouncementForm /> : <CreateEolAnnouncementForm />}
    </Box>
  );
}
