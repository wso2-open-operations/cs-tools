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
import { useState } from "react";

import { goToMyAppsScreen } from "@src/bridge";
import { IconButton, Typography } from "@wso2/oxygen-ui";
import { Grip } from "@wso2/oxygen-ui-icons-react";

import { ConfirmDialog } from "@shared/components/common";

export function ExitButton() {
  const [open, setOpen] = useState(false);

  return (
    <>
      <IconButton disableRipple color="error" sx={{ gap: 1, p: 0, m: 0.8 }} onClick={() => setOpen(true)}>
        <Grip size={20} />
        <Typography>Go to Apps</Typography>
      </IconButton>

      <ConfirmDialog
        open={open}
        title="Return to Apps"
        description="Are you sure you want to leave this application?"
        confirmColor="error"
        confirmLabel="Leave"
        onClose={() => setOpen(false)}
        onConfirm={goToMyAppsScreen}
      />
    </>
  );
}
