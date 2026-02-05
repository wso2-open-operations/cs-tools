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

import { Button, Form, Stack } from "@wso2/oxygen-ui";
import { ArrowRight } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";

interface ProjectCardActionsProps {
  onViewDashboard?: () => void;
}

/**
 * Component to render the action buttons for the Project Card.
 *
 * @param {ProjectCardActionsProps} props - The component props.
 * @returns {JSX.Element} The rendered actions.
 */
export default function ProjectCardActions({
  onViewDashboard,
}: ProjectCardActionsProps): JSX.Element {
  return (
    <Form.CardActions sx={{ width: "100%", mt: "auto", pt: 1.5, pb: 2 }}>
      <Stack spacing={2} sx={{ width: "100%" }}>
        <Button
          variant="outlined"
          size="medium"
          color="secondary"
          fullWidth
          endIcon={<ArrowRight size={16} />}
          onClick={(e) => {
            e.stopPropagation();
            onViewDashboard?.();
          }}
        >
          View Dashboard
        </Button>
      </Stack>
    </Form.CardActions>
  );
}
